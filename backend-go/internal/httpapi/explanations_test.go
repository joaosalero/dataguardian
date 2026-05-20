package httpapi

import (
	"errors"
	"testing"

	"dataguardian/backend-go/internal/db"
)

type failingExplanationProvider struct{}

func (failingExplanationProvider) ExplainFinding(finding db.Finding) (FindingExplanation, error) {
	return FindingExplanation{}, errors.New("explanation unavailable")
}

func TestTemplateExplanationProviderMapsKnownFinding(t *testing.T) {
	result, err := templateExplanationProvider{}.ExplainFinding(db.Finding{Code: "PDF_JS_DETECTED"})
	if err != nil {
		t.Fatalf("ExplainFinding returned error: %v", err)
	}
	if result.Explanation == "" || result.Recommendation == "" {
		t.Fatalf("expected explanation and recommendation, got %#v", result)
	}
}

func TestTemplateExplanationProviderUsesFallbackForUnknownFinding(t *testing.T) {
	result, err := templateExplanationProvider{}.ExplainFinding(db.Finding{Code: "UNKNOWN_RULE"})
	if err != nil {
		t.Fatalf("ExplainFinding returned error: %v", err)
	}
	if result.Explanation == "" || result.Recommendation == "" {
		t.Fatalf("expected fallback explanation and recommendation, got %#v", result)
	}
}

func TestAnalysisFindingsAddsExplanationsAndKeepsFallbackSafe(t *testing.T) {
	findings := analysisFindings([]db.Finding{
		{
			ID:          1,
			Type:        db.FindingTypePDF,
			Code:        "PDF_JS_DETECTED",
			Title:       "PDF JavaScript marker detected",
			Description: "The PDF contains a JavaScript marker in its raw bytes.",
			Severity:    db.SeverityHigh,
		},
	})
	if len(findings) != 1 || findings[0].Explanation == "" || findings[0].Recommendation == nil {
		t.Fatalf("expected decorated finding, got %#v", findings)
	}

	originalProvider := explanationProvider
	explanationProvider = failingExplanationProvider{}
	t.Cleanup(func() { explanationProvider = originalProvider })

	findings = analysisFindings([]db.Finding{{Code: "PDF_JS_DETECTED"}})
	if len(findings) != 1 || findings[0].Explanation != "" {
		t.Fatalf("expected response without AI decoration on provider failure, got %#v", findings)
	}
}
