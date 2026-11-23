package analysis

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"dataguardian/backend-go/internal/db"
)

func TestInspectionEvidence(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		input := []byte("%PDF-1.7\n<< /Author (Fixture Author) /Title (Public fixture) >>\n%%EOF\n")
		original := bytes.Clone(input)
		inspected := AnalyzeFile(input, "application/pdf")
		clean, err := SanitizeFile(input, "application/pdf")
		if err != nil {
			t.Fatal(err)
		}
		if len(inspected.Findings) != 1 || inspected.Findings[0].Code != "METADATA_AUTHOR_PRESENT" ||
			inspected.RiskScore.Score != 55 || inspected.RiskScore.Level != db.RiskLevelMedium {
			t.Fatalf("unexpected findings or risk: %+v, %+v", inspected.Findings, inspected.RiskScore)
		}
		if len(clean.RemovedMetadataKeys) != 1 || clean.RemovedMetadataKeys[0] != "author" ||
			bytes.Contains(clean.Content, []byte("Fixture Author")) || !bytes.Contains(clean.Content, []byte("Public fixture")) ||
			len(input) != len(clean.Content) || !bytes.Equal(input, original) {
			t.Fatal("metadata-cleaning contract changed")
		}
		t.Logf("file finding=%s score=%d level=%s removed=%v original_bytes=%d cleaned_bytes=%d title_retained=true",
			inspected.Findings[0].Code, inspected.RiskScore.Score, inspected.RiskScore.Level,
			clean.RemovedMetadataKeys, len(input), len(clean.Content))
	})

	t.Run("url", func(t *testing.T) {
		withURLTestHooks(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != "http://fixture.test/overview" {
				t.Fatalf("unexpected fixture request: %s", r.URL)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"text/html"}},
				Body:       io.NopCloser(strings.NewReader("<html><p>Public fixture</p></html>")),
			}, nil
		}))
		result, err := AnalyzeURL(context.Background(), "http://fixture.test/overview")
		if err != nil {
			t.Fatal(err)
		}
		if result.Target.HTTPStatusCode == nil || *result.Target.HTTPStatusCode != http.StatusOK ||
			result.Target.FetchStatus != db.FetchStatusSuccess || len(result.Findings) != 1 ||
			result.Findings[0].Code != "URL_NO_HTTPS" || result.RiskScore.Score != 10 ||
			result.RiskScore.Level != db.RiskLevelLow {
			t.Fatalf("unexpected controlled response: %+v", result)
		}
		t.Logf("url status=%d finding=%s score=%d level=%s transport=in-memory",
			*result.Target.HTTPStatusCode, result.Findings[0].Code, result.RiskScore.Score, result.RiskScore.Level)
	})
}

func TestPDFMetadataKeepsFirstNonemptySupportedValues(t *testing.T) {
	input := []byte("/Title (Untouched) /Author ( ) /Author (First) /Producer (Tool) /Author (Second) /CreationDate (Today)")
	values := pdfLiteralMetadata(input)
	if len(values) != 3 || values["Author"] != "First" || values["Producer"] != "Tool" || values["CreationDate"] != "Today" {
		t.Fatalf("unexpected metadata values: %v", values)
	}
}
