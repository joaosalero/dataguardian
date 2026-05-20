package analysis

import (
	"strings"
	"testing"

	"dataguardian/backend-go/internal/db"
)

func TestAnalyzeFileDetectsPDFFindings(t *testing.T) {
	result := AnalyzeFile([]byte("%PDF-1.7\n/OpenAction << /JS (JavaScript) >>"), "application/pdf")

	if len(result.Findings) != 2 {
		t.Fatalf("expected 2 PDF findings, got %d", len(result.Findings))
	}
	if result.RiskScore.Level != db.RiskLevelHigh {
		t.Fatalf("expected high risk, got %s", result.RiskScore.Level)
	}
}

func TestAnalyzeFileDetectsGenericPatterns(t *testing.T) {
	content := []byte("safe prefix eval(alert) " + strings.Repeat("A", 90))
	result := AnalyzeFile(content, "image/jpeg")

	if len(result.Findings) != 2 {
		t.Fatalf("expected 2 generic findings, got %d", len(result.Findings))
	}
	if result.RiskScore.Level != db.RiskLevelMedium {
		t.Fatalf("expected medium risk, got %s", result.RiskScore.Level)
	}
}

func TestScoreFindingsReturnsLowWithoutFindings(t *testing.T) {
	score := ScoreFindings(nil)

	if score.Level != db.RiskLevelLow {
		t.Fatalf("expected low risk, got %s", score.Level)
	}
	if score.Score < 0 || score.Score > 30 {
		t.Fatalf("expected low numeric score, got %d", score.Score)
	}
}
