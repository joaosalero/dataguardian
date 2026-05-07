package analysis

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"dataguardian/backend-go/internal/db"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func withURLTestHooks(t *testing.T, transport http.RoundTripper) {
	t.Helper()
	originalLookup := lookupURLIPAddrs
	originalTransport := urlHTTPTransport
	lookupURLIPAddrs = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	}
	urlHTTPTransport = transport
	t.Cleanup(func() {
		lookupURLIPAddrs = originalLookup
		urlHTTPTransport = originalTransport
	})
}

func TestAnalyzeURLDetectsFindingsAndResponseFields(t *testing.T) {
	withURLTestHooks(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `prefix eval("x") ` + strings.Repeat("A", 96)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type":   []string{"text/html; charset=utf-8"},
				"Content-Length": []string{"120"},
			},
			Body: io.NopCloser(strings.NewReader(body)),
		}, nil
	}))

	result, err := AnalyzeURL(context.Background(), "http://example.test/page")
	if err != nil {
		t.Fatalf("AnalyzeURL returned error: %v", err)
	}
	if result.Target.FetchStatus != db.FetchStatusSuccess || result.Target.HTTPStatusCode == nil || *result.Target.HTTPStatusCode != http.StatusOK {
		t.Fatalf("unexpected target: %#v", result.Target)
	}
	if result.RiskScore.Level != db.RiskLevelMedium {
		t.Fatalf("expected medium risk, got %s", result.RiskScore.Level)
	}
	codes := map[string]bool{}
	for _, finding := range result.Findings {
		codes[finding.Code] = true
	}
	if !codes["URL_NO_HTTPS"] || !codes["URL_SUSPICIOUS_CONTENT"] {
		t.Fatalf("missing expected URL findings: %#v", result.Findings)
	}
}

func TestAnalyzeURLBlocksSSRFHosts(t *testing.T) {
	for _, rawURL := range []string{
		"http://127.0.0.1/admin",
		"http://[::1]/admin",
		"http://10.0.0.2/admin",
		"http://172.16.0.2/admin",
		"http://172.31.0.2/admin",
		"http://192.168.1.2/admin",
	} {
		if _, err := AnalyzeURL(context.Background(), rawURL); !errors.Is(err, ErrUnsafeURL) {
			t.Fatalf("expected unsafe URL error for %s, got %v", rawURL, err)
		}
	}
}

func TestAnalyzeURLRecordsTimeoutAsFetchFailure(t *testing.T) {
	withURLTestHooks(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	}))

	result, err := AnalyzeURL(context.Background(), "https://example.test/slow")
	if err != nil {
		t.Fatalf("AnalyzeURL returned error: %v", err)
	}
	if result.Target.FetchStatus != db.FetchStatusFailed || result.Target.FailureReason == nil || *result.Target.FailureReason != "timeout" {
		t.Fatalf("expected timeout fetch failure, got %#v", result.Target)
	}
	if len(result.Findings) != 1 || result.Findings[0].Code != "URL_FETCH_FAILED" {
		t.Fatalf("expected fetch failed finding, got %#v", result.Findings)
	}
}

func TestAnalyzeURLRejectsOversizedResponses(t *testing.T) {
	withURLTestHooks(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Header:        http.Header{},
			ContentLength: int64(maxURLResponseSize + 1),
			Body:          io.NopCloser(strings.NewReader("")),
		}, nil
	}))

	result, err := AnalyzeURL(context.Background(), "https://example.test/large")
	if err != nil {
		t.Fatalf("AnalyzeURL returned error: %v", err)
	}
	if result.Target.FetchStatus != db.FetchStatusFailed || result.Target.FailureReason == nil || *result.Target.FailureReason != "response too large" {
		t.Fatalf("expected oversized fetch failure, got %#v", result.Target)
	}
}

func TestAnalyzeURLCapturesRedirectChain(t *testing.T) {
	withURLTestHooks(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/start" {
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{"https://example.test/final"}},
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/html"}},
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	}))

	result, err := AnalyzeURL(context.Background(), "https://example.test/start")
	if err != nil {
		t.Fatalf("AnalyzeURL returned error: %v", err)
	}
	if result.Target.RedirectCount != 1 || result.Target.RedirectChain[0] != "https://example.test/final" {
		t.Fatalf("expected redirect chain, got %#v", result.Target)
	}
	if len(result.Findings) != 1 || result.Findings[0].Code != "URL_REDIRECT_DETECTED" {
		t.Fatalf("expected redirect finding, got %#v", result.Findings)
	}
}
