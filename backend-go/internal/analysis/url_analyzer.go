package analysis

import (
	"bytes"
	"context"
	"errors"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dataguardian/backend-go/internal/db"
)

const (
	maxURLResponseSize = 2 * 1024 * 1024
	maxURLRedirects    = 3
	urlFetchTimeout    = 5 * time.Second
	maxURLPreviewChars = 2000
)

var (
	ErrInvalidURL = errors.New("invalid URL")
	ErrUnsafeURL  = errors.New("unsafe URL")
)

var lookupURLIPAddrs = net.DefaultResolver.LookupIPAddr
var urlHTTPTransport http.RoundTripper

// URLResult contains the persisted URL target details and deterministic findings.
type URLResult struct {
	Target     db.URLTarget
	Metadata   db.Metadata
	Findings   []db.Finding
	RiskScore  db.RiskScore
	Summary    string
	RemoteFile *RemoteFile
}

// RemoteFile is a supported downloadable URL response kept for isolated file inspection.
type RemoteFile struct {
	Filename  string
	MimeType  string
	SizeBytes int64
	Content   []byte
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
		targetPreview := extractURLPlainTextPreview(content, nullableString(target.ContentType))
		if targetPreview != "" {
			target.MetadataPreview = targetPreview
		}
		if remoteFile := detectRemoteFile(parsed, target, content); remoteFile != nil {
			findings = append(findings, urlFinding(
				"URL_REMOTE_FILE_DETECTED",
				"Remote downloadable file detected",
				"The fetched URL returned a supported file type that can be inspected before local download.",
				db.SeverityLow,
				remoteFile.MimeType,
				"URL_REMOTE_FILE_DETECTED",
				db.FindingEvidenceSourceURL,
			))
			target.MetadataPreview = ""
			return URLResult{
				Target:     target,
				Metadata:   urlMetadata(0, target),
				Findings:   findings,
				RiskScore:  ScoreFindings(findings),
				Summary:    "URL analysis completed with a remote file inspection candidate.",
				RemoteFile: remoteFile,
			}, nil
		}
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

func detectRemoteFile(requestURL *url.URL, target db.URLTarget, content []byte) *RemoteFile {
	if len(content) == 0 || target.HTTPStatusCode == nil || *target.HTTPStatusCode < 200 || *target.HTTPStatusCode >= 300 {
		return nil
	}
	headerType := strings.ToLower(strings.TrimSpace(stringFromPtr(target.ContentType)))
	sniffedType := strings.ToLower(http.DetectContentType(content))
	mimeType := supportedRemoteFileMimeType(headerType, sniffedType, content)
	if mimeType == "" {
		return nil
	}
	filename := remoteFileName(requestURL, stringFromPtr(target.FinalURL), mimeType)
	return &RemoteFile{
		Filename:  filename,
		MimeType:  mimeType,
		SizeBytes: int64(len(content)),
		Content:   append([]byte(nil), content...),
	}
}

func supportedRemoteFileMimeType(headerType string, sniffedType string, content []byte) string {
	switch {
	case strings.Contains(headerType, "text/html"):
		return ""
	case (strings.Contains(headerType, "application/pdf") || sniffedType == "application/pdf") && looksLikePDF(content):
		return "application/pdf"
	case (strings.Contains(headerType, "image/jpeg") || sniffedType == "image/jpeg") && looksLikeJPEG(content):
		return "image/jpeg"
	case (strings.Contains(headerType, "image/png") || sniffedType == "image/png") && looksLikePNG(content):
		return "image/png"
	case strings.HasPrefix(headerType, "text/plain") && looksLikeText(content):
		return "text/plain; charset=utf-8"
	case headerType == "" && strings.HasPrefix(sniffedType, "text/plain") && looksLikeText(content) && !bytes.Contains(bytes.ToLower(content[:min(len(content), 1024)]), []byte("<html")):
		return "text/plain; charset=utf-8"
	default:
		return ""
	}
}

func looksLikePDF(content []byte) bool {
	return len(content) >= 4 && string(content[:4]) == "%PDF"
}

func looksLikeJPEG(content []byte) bool {
	return len(content) >= 3 && content[0] == 0xff && content[1] == 0xd8 && content[2] == 0xff
}

func looksLikePNG(content []byte) bool {
	return len(content) >= 8 && string(content[:8]) == "\x89PNG\r\n\x1a\n"
}

func looksLikeText(content []byte) bool {
	if len(content) == 0 {
		return false
	}
	sample := content[:min(len(content), 1024)]
	return !bytes.Contains(sample, []byte{0})
}

func remoteFileName(requestURL *url.URL, finalURL string, mimeType string) string {
	source := requestURL
	if parsed, err := url.Parse(finalURL); err == nil && parsed.Path != "" {
		source = parsed
	}
	name := strings.TrimSpace(path.Base(source.Path))
	if name == "." || name == "/" || name == "" {
		name = "remote-file"
	}
	if path.Ext(name) == "" {
		name += remoteFileExtension(mimeType)
	}
	return name
}

func remoteFileExtension(mimeType string) string {
	switch {
	case mimeType == "application/pdf":
		return ".pdf"
	case mimeType == "image/jpeg":
		return ".jpg"
	case mimeType == "image/png":
		return ".png"
	case strings.HasPrefix(mimeType, "text/plain"):
		return ".txt"
	default:
		return ""
	}
}

func stringFromPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

var (
	scriptOrStyleBlockPattern = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	htmlTagPattern            = regexp.MustCompile(`(?s)<[^>]+>`)
	whitespacePattern         = regexp.MustCompile(`[ \t\r\n]+`)
)

func validateSafeURL(ctx context.Context, raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Hostname() == "" {
		return nil, ErrInvalidURL
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, ErrInvalidURL
	}
	if port := parsed.Port(); port != "" && !((parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443")) {
		return nil, ErrUnsafeURL
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return nil, ErrInvalidURL
	}
	if _, err := resolveSafeHost(ctx, parsed.Hostname()); err != nil {
		return nil, err
	}
	return parsed, nil
}

func ensureSafeHost(ctx context.Context, host string) error {
	_, err := resolveSafeHost(ctx, host)
	return err
}

func resolveSafeHost(ctx context.Context, host string) ([]net.IP, error) {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return nil, ErrUnsafeURL
	}
	if ip := net.ParseIP(host); ip != nil {
		if isUnsafeIP(ip) {
			return nil, ErrUnsafeURL
		}
		return []net.IP{ip}, nil
	}
	addrs, err := lookupURLIPAddrs(ctx, host)
	if err != nil || len(addrs) == 0 {
		return nil, ErrInvalidURL
	}
	// Every resolved address must be public; mixed public/private DNS answers are blocked.
	ips := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		if isUnsafeIP(addr.IP) {
			return nil, ErrUnsafeURL
		}
		ips = append(ips, addr.IP)
	}
	return ips, nil
}

func isUnsafeIP(ip net.IP) bool {
	return !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}

func fetchURL(ctx context.Context, start *url.URL, target db.URLTarget) (db.URLTarget, []byte, error) {
	transport := urlHTTPTransport
	if transport == nil {
		transport = safeURLTransport(ctx)
	}
	client := &http.Client{
		Timeout:   urlFetchTimeout,
		Transport: transport,
		// Redirects are validated and followed manually so every hop gets SSRF checks.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	current := start

	for redirects := 0; ; redirects++ {
		validatedCurrent, err := validateSafeURL(ctx, current.String())
		if err != nil {
			return target, nil, err
		}
		current = validatedCurrent
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, current.String(), nil)
		if err != nil {
			return target, nil, err
		}
		current = validatedCurrent
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
			validatedNext, err := validateSafeURL(ctx, next.String())
			if err != nil {
				return target, nil, err
			}
			target.RedirectChain = append(target.RedirectChain, validatedNext.String())
			target.RedirectCount = len(target.RedirectChain)
			current = validatedNext
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

func safeURLRequest(ctx context.Context, method string, candidate *url.URL) (*http.Request, *url.URL, error) {
	if candidate == nil {
		return nil, nil, ErrInvalidURL
	}
	validated, err := validateSafeURL(ctx, candidate.String())
	if err != nil {
		return nil, nil, err
	}
	if ips, err := resolveSafeHost(ctx, validated.Hostname()); err != nil {
		return nil, nil, err
	} else if len(ips) == 0 {
		return nil, nil, ErrInvalidURL
	}
	requestURL := *validated
	requestURL.User = nil
	requestURL.Fragment = ""
	req := (&http.Request{
		Method: method,
		URL:    &requestURL,
		Header: make(http.Header),
	}).WithContext(ctx)
	return req, &requestURL, nil
}

func safeURLTransport(ctx context.Context) http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = safeURLDialContext(ctx)
	return transport
}

func safeURLDialContext(parent context.Context) func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: urlFetchTimeout}
	return func(ctx context.Context, network string, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, ErrInvalidURL
		}
		ips, err := resolveSafeHost(parent, host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, ErrInvalidURL
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
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
	entries := []db.MetadataEntry{
		urlMetadataEntry("content_type", nullableString(target.ContentType), "headers"),
		urlMetadataEntry("content_length_bytes", nullableInt64(target.ContentLengthBytes), "headers"),
		urlMetadataEntry("host", target.Host, "url"),
		urlMetadataEntry("protocol", protocolFromURL(target), "url"),
		urlMetadataEntry("redirect_count", target.RedirectCount, "redirects"),
		urlMetadataEntry("redirect_chain", target.RedirectChain, "redirects"),
		urlMetadataEntry("http_status_code", nullableInt(target.HTTPStatusCode), "headers"),
		urlMetadataEntry("fetch_status", target.FetchStatus, "fetch"),
	}
	if strings.TrimSpace(target.MetadataPreview) != "" {
		entries = append(entries, urlMetadataEntry("safe_preview_text", target.MetadataPreview, "safe_preview"))
	}
	return db.Metadata{
		AnalysisID: analysisID,
		SourceType: db.MetadataSourceTypeURLContent,
		Entries:    entries,
	}
}

func extractURLPlainTextPreview(content []byte, contentType any) string {
	contentTypeText, _ := contentType.(string)
	lowerType := strings.ToLower(contentTypeText)
	if lowerType != "" &&
		!strings.Contains(lowerType, "text/") &&
		!strings.Contains(lowerType, "application/json") &&
		!strings.Contains(lowerType, "application/xml") &&
		!strings.Contains(lowerType, "application/xhtml") {
		return ""
	}
	text := string(content)
	if strings.Contains(lowerType, "html") || bytes.Contains(bytes.ToLower(content[:min(len(content), 1024)]), []byte("<html")) {
		text = scriptOrStyleBlockPattern.ReplaceAllString(text, " ")
		text = htmlTagPattern.ReplaceAllString(text, " ")
		text = html.UnescapeString(text)
	}
	text = whitespacePattern.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if len(text) > maxURLPreviewChars {
		text = text[:maxURLPreviewChars] + "..."
	}
	return text
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
