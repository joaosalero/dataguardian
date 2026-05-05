package analysis

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dataguardian/backend-go/internal/db"
)

const (
	maxURLResponseSize = 2 * 1024 * 1024
	maxURLRedirects    = 3
	urlFetchTimeout    = 5 * time.Second
)

var (
	ErrInvalidURL = errors.New("invalid URL")
	ErrUnsafeURL  = errors.New("unsafe URL")
)

var lookupURLIPAddrs = net.DefaultResolver.LookupIPAddr
var urlHTTPTransport http.RoundTripper = http.DefaultTransport

// URLResult contains the persisted URL target details and deterministic findings.
type URLResult struct {
	Target    db.URLTarget
	Metadata  db.Metadata
	Findings  []db.Finding
	RiskScore db.RiskScore
	Summary   string
}

// AnalyzeURL fetches URL content passively and inspects raw bytes only.
func AnalyzeURL(ctx context.Context, originalURL string) (URLResult, error) {
	parsed, err := validateSafeURL(ctx, originalURL)
	if err != nil {
		return URLResult{}, err
	}

	target := db.URLTarget{
		OriginalURL:   parsed.String(),
		RedirectChain: []string{},
		UsesHTTPS:     parsed.Scheme == "https",
		Host:          parsed.Hostname(),
		FetchStatus:   db.FetchStatusNotFetched,
	}
	findings := urlTransportFindings(target)

	fetchedTarget, content, fetchErr := fetchURL(ctx, parsed, target)
	target = fetchedTarget
	if fetchErr != nil {
		reason := safeFailureReason(fetchErr)
		target.FetchStatus = db.FetchStatusFailed
		target.FailureReason = &reason
		findings = append(findings, urlFinding(
			"URL_FETCH_FAILED",
			"URL fetch failed",
			"The URL could not be fetched within the configured safety limits.",
			db.SeverityMedium,
			reason,
			"URL_FETCH_FAILED",
			db.FindingEvidenceSourceURL,
		))
	} else {
		if target.RedirectCount > 0 {
			findings = append(findings, urlFinding(
				"URL_REDIRECT_DETECTED",
				"URL redirect detected",
				"The submitted URL redirected before returning final content.",
				db.SeverityLow,
				strings.Join(target.RedirectChain, " -> "),
				"URL_REDIRECT_DETECTED",
				db.FindingEvidenceSourceURL,
			))
		}
		findings = append(findings, inspectURLContent(content)...)
	}

	metadata := urlMetadata(0, target)
	score := ScoreFindings(findings)
	summary := "URL analysis completed with no findings."
	if len(findings) > 0 {
		summary = "URL analysis completed with structured findings."
	}
	return URLResult{
		Target:    target,
		Metadata:  metadata,
		Findings:  findings,
		RiskScore: score,
		Summary:   summary,
	}, nil
}

func validateSafeURL(ctx context.Context, raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Hostname() == "" {
		return nil, ErrInvalidURL
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, ErrInvalidURL
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return nil, ErrInvalidURL
	}
	if err := ensureSafeHost(ctx, parsed.Hostname()); err != nil {
		return nil, err
	}
	return parsed, nil
}

func ensureSafeHost(ctx context.Context, host string) error {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return ErrUnsafeURL
	}
	if ip := net.ParseIP(host); ip != nil {
		if isUnsafeIP(ip) {
			return ErrUnsafeURL
		}
		return nil
	}
	addrs, err := lookupURLIPAddrs(ctx, host)
	if err != nil || len(addrs) == 0 {
		return ErrInvalidURL
	}
	// Every resolved address must be public; mixed public/private DNS answers are blocked.
	for _, addr := range addrs {
		if isUnsafeIP(addr.IP) {
			return ErrUnsafeURL
		}
	}
	return nil
}

func isUnsafeIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}

func fetchURL(ctx context.Context, start *url.URL, target db.URLTarget) (db.URLTarget, []byte, error) {
	client := &http.Client{
		Timeout:   urlFetchTimeout,
		Transport: urlHTTPTransport,
		// Redirects are validated and followed manually so every hop gets SSRF checks.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	current := start

	for redirects := 0; ; redirects++ {
		if err := ensureSafeHost(ctx, current.Hostname()); err != nil {
			return target, nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, current.String(), nil)
		if err != nil {
			return target, nil, ErrInvalidURL
		}
		resp, err := client.Do(req)
		if err != nil {
			return target, nil, err
		}

		target.FinalURL = stringPtr(current.String())
		target.Host = current.Hostname()
		target.UsesHTTPS = current.Scheme == "https"
		target.FetchStatus = db.FetchStatusSuccess
		target.ContentType = stringPtrOrNil(strings.TrimSpace(resp.Header.Get("Content-Type")))
		target.ContentLengthBytes = parseContentLength(resp.Header.Get("Content-Length"))
		statusCode := resp.StatusCode
		target.HTTPStatusCode = &statusCode

		if isRedirect(resp.StatusCode) && resp.Header.Get("Location") != "" {
			_ = resp.Body.Close()
			if redirects >= maxURLRedirects {
				return target, nil, errors.New("redirect limit exceeded")
			}
			next, err := current.Parse(resp.Header.Get("Location"))
			if err != nil {
				return target, nil, ErrInvalidURL
			}
			if _, err := validateSafeURL(ctx, next.String()); err != nil {
				return target, nil, err
			}
			target.RedirectChain = append(target.RedirectChain, next.String())
			target.RedirectCount = len(target.RedirectChain)
			current = next
			continue
		}

		content, err := readLimitedURLBody(resp)
		if err != nil {
			return target, nil, err
		}
		if target.ContentLengthBytes == nil {
			length := int64(len(content))
			target.ContentLengthBytes = &length
		}
		now := time.Now().UTC()
		target.FetchedAt = &now
		return target, content, nil
	}
}

func readLimitedURLBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	if resp.ContentLength > maxURLResponseSize {
		return nil, errors.New("response exceeds size limit")
	}
	// LimitReader prevents untrusted servers from streaming beyond the accepted cap.
	content, err := io.ReadAll(io.LimitReader(resp.Body, maxURLResponseSize+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxURLResponseSize {
		return nil, errors.New("response exceeds size limit")
	}
	return content, nil
}

func isRedirect(status int) bool {
	return status == http.StatusMovedPermanently ||
		status == http.StatusFound ||
		status == http.StatusSeeOther ||
		status == http.StatusTemporaryRedirect ||
		status == http.StatusPermanentRedirect
}

func parseContentLength(value string) *int64 {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return nil
	}
	return &parsed
}

func urlTransportFindings(target db.URLTarget) []db.Finding {
	findings := make([]db.Finding, 0)
	if !target.UsesHTTPS {
		findings = append(findings, urlFinding(
			"URL_NO_HTTPS",
			"URL does not use HTTPS",
			"The submitted URL uses HTTP instead of HTTPS.",
			db.SeverityLow,
			target.OriginalURL,
			"URL_NO_HTTPS",
			db.FindingEvidenceSourceURL,
		))
	}
	return findings
}

func inspectURLContent(content []byte) []db.Finding {
	findings := make([]db.Finding, 0)
	if match := findBase64Pattern(content); match != "" {
		findings = append(findings, urlSuspiciousContentFinding("Long base64-like string detected", match, "URL_CONTENT_BASE64_PATTERN"))
	}
	if hasEvalPattern(content) {
		findings = append(findings, urlSuspiciousContentFinding("eval pattern detected", "eval(", "URL_CONTENT_EVAL_PATTERN"))
	}
	if match := longEncodedString(content); match != "" {
		findings = append(findings, urlSuspiciousContentFinding("Suspicious long encoded string detected", match, "URL_CONTENT_LONG_ENCODED_PATTERN"))
	}
	return findings
}

func longEncodedString(content []byte) string {
	fields := bytes.FieldsFunc(content, func(r rune) bool {
		return !(r >= 'A' && r <= 'Z') && !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '%' && r != '_' && r != '-'
	})
	for _, field := range fields {
		if len(field) >= 160 && (bytes.Contains(field, []byte("%")) || bytes.Contains(field, []byte("_")) || bytes.Contains(field, []byte("-"))) {
			return snippet(field)
		}
	}
	return ""
}

func urlSuspiciousContentFinding(title string, matchedValue string, ruleID string) db.Finding {
	return urlFinding(
		"URL_SUSPICIOUS_CONTENT",
		title,
		"The fetched URL content contains suspicious string patterns in raw bytes.",
		db.SeverityMedium,
		matchedValue,
		ruleID,
		db.FindingEvidenceSourceContent,
	)
}

func urlFinding(code string, title string, description string, severity db.Severity, matchedValue string, ruleID string, source db.FindingEvidenceSource) db.Finding {
	return db.Finding{
		Type:        db.FindingTypeURL,
		Code:        code,
		Title:       title,
		Description: description,
		Severity:    severity,
		Evidence: db.FindingEvidence{
			Source:       source,
			MatchedValue: stringPtr(matchedValue),
			RuleID:       ruleID,
		},
	}
}

func urlMetadata(analysisID int64, target db.URLTarget) db.Metadata {
	return db.Metadata{
		AnalysisID: analysisID,
		SourceType: db.MetadataSourceTypeURLContent,
		Entries: []db.MetadataEntry{
			urlMetadataEntry("content_type", nullableString(target.ContentType), "headers"),
			urlMetadataEntry("content_length_bytes", nullableInt64(target.ContentLengthBytes), "headers"),
			urlMetadataEntry("host", target.Host, "url"),
			urlMetadataEntry("protocol", protocolFromURL(target), "url"),
			urlMetadataEntry("redirect_count", target.RedirectCount, "redirects"),
			urlMetadataEntry("redirect_chain", target.RedirectChain, "redirects"),
			urlMetadataEntry("http_status_code", nullableInt(target.HTTPStatusCode), "headers"),
			urlMetadataEntry("fetch_status", target.FetchStatus, "fetch"),
		},
	}
}

func urlMetadataEntry(key string, value any, source string) db.MetadataEntry {
	return db.MetadataEntry{
		Key:         key,
		Value:       value,
		Category:    db.MetadataCategoryURL,
		Sensitivity: db.MetadataSensitivityNonSensitive,
		Source:      source,
		Confidence:  db.MetadataConfidenceHigh,
	}
}

func protocolFromURL(target db.URLTarget) string {
	if target.UsesHTTPS {
		return "https"
	}
	return "http"
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func safeFailureReason(err error) string {
	if errors.Is(err, ErrInvalidURL) {
		return "invalid URL"
	}
	if errors.Is(err, ErrUnsafeURL) {
		return "unsafe URL"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if strings.Contains(strings.ToLower(err.Error()), "timeout") {
		return "timeout"
	}
	if strings.Contains(strings.ToLower(err.Error()), "size limit") {
		return "response too large"
	}
	if strings.Contains(strings.ToLower(err.Error()), "redirect limit") {
		return "redirect limit exceeded"
	}
	return "network failure"
}

func stringPtr(value string) *string {
	return &value
}

func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
