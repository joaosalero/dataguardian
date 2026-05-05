package analysis

import (
	"bytes"
	"regexp"

	"dataguardian/backend-go/internal/db"
)

var base64Pattern = regexp.MustCompile(`[A-Za-z0-9+/]{80,}={0,2}`)

// FileResult contains deterministic findings and risk generated from raw file bytes.
type FileResult struct {
	Findings  []db.Finding
	RiskScore db.RiskScore
	Summary   string
}

// AnalyzeFile inspects bytes only; it never executes, renders, or interprets active content.
func AnalyzeFile(content []byte, mimeType string) FileResult {
	findings := make([]db.Finding, 0)

	if mimeType == "application/pdf" {
		findings = append(findings, pdfFindings(content)...)
	}
	findings = append(findings, genericFindings(content)...)

	score := ScoreFindings(findings)
	summary := "File analysis completed with no findings."
	if len(findings) > 0 {
		summary = "File analysis completed with structured findings."
	}

	return FileResult{
		Findings:  findings,
		RiskScore: score,
		Summary:   summary,
	}
}

// ScoreFindings applies the first deterministic risk model for file analysis.
func ScoreFindings(findings []db.Finding) db.RiskScore {
	score := 10
	level := db.RiskLevelLow
	drivers := make([]db.RiskDriver, 0, len(findings))

	for _, finding := range findings {
		drivers = append(drivers, db.RiskDriver{
			FindingCode: finding.Code,
			Severity:    finding.Severity,
			Reason:      finding.Title,
		})
		if finding.Severity == db.SeverityHigh {
			score = 85
			level = db.RiskLevelHigh
		} else if finding.Severity == db.SeverityMedium && level != db.RiskLevelHigh {
			score = 55
			level = db.RiskLevelMedium
		}
	}

	return db.RiskScore{
		Score:   score,
		Level:   level,
		Drivers: drivers,
	}
}

func pdfFindings(content []byte) []db.Finding {
	checks := []struct {
		pattern     []byte
		code        string
		title       string
		description string
		severity    db.Severity
	}{
		{
			pattern:     []byte("JavaScript"),
			code:        "PDF_JS_DETECTED",
			title:       "PDF JavaScript marker detected",
			description: "The PDF contains a JavaScript marker in its raw bytes.",
			severity:    db.SeverityHigh,
		},
		{
			pattern:     []byte("/JS"),
			code:        "PDF_JS_DETECTED",
			title:       "PDF JavaScript action marker detected",
			description: "The PDF contains a /JS action marker in its raw bytes.",
			severity:    db.SeverityHigh,
		},
		{
			pattern:     []byte("/OpenAction"),
			code:        "PDF_OPENACTION_DETECTED",
			title:       "PDF OpenAction marker detected",
			description: "The PDF contains an /OpenAction marker in its raw bytes.",
			severity:    db.SeverityMedium,
		},
	}

	findings := make([]db.Finding, 0)
	seen := map[string]bool{}
	for _, check := range checks {
		if seen[check.code] || !bytes.Contains(content, check.pattern) {
			continue
		}
		seen[check.code] = true
		findings = append(findings, newFinding(
			db.FindingTypePDF,
			check.code,
			check.title,
			check.description,
			check.severity,
			string(check.pattern),
			check.code,
		))
	}
	return findings
}

func genericFindings(content []byte) []db.Finding {
	findings := make([]db.Finding, 0)
	if match := findBase64Pattern(content); match != "" {
		findings = append(findings, newFinding(
			db.FindingTypeGeneric,
			"GENERIC_BASE64_PATTERN",
			"Long base64-like string detected",
			"The file contains a long base64-like string in its raw bytes.",
			db.SeverityMedium,
			match,
			"GENERIC_BASE64_PATTERN",
		))
	}
	if hasEvalPattern(content) {
		findings = append(findings, newFinding(
			db.FindingTypeGeneric,
			"GENERIC_EVAL_PATTERN",
			"eval pattern detected",
			"The file contains an eval( string pattern in its raw bytes.",
			db.SeverityMedium,
			"eval(",
			"GENERIC_EVAL_PATTERN",
		))
	}
	return findings
}

func findBase64Pattern(content []byte) string {
	if match := base64Pattern.Find(content); len(match) > 0 {
		return snippet(match)
	}
	return ""
}

func hasEvalPattern(content []byte) bool {
	return bytes.Contains(bytes.ToLower(content), []byte("eval("))
}

func newFinding(
	findingType db.FindingType,
	code string,
	title string,
	description string,
	severity db.Severity,
	matchedValue string,
	ruleID string,
) db.Finding {
	return db.Finding{
		Type:        findingType,
		Code:        code,
		Title:       title,
		Description: description,
		Severity:    severity,
		Evidence: db.FindingEvidence{
			Source:       db.FindingEvidenceSourceContent,
			MatchedValue: &matchedValue,
			RuleID:       ruleID,
		},
	}
}

func snippet(value []byte) string {
	if len(value) > 80 {
		value = value[:80]
	}
	return string(value)
}
