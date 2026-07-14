package analysis

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"dataguardian/backend-go/internal/db"
)

func TestSafeSampleCorpusExpectedRisk(t *testing.T) {
	root := filepath.Join("..", "..", "..", "samples")
	tests := []struct {
		name string
		path string
		want db.RiskLevel
	}{
		{"clean PDF", "clean/clean.pdf", db.RiskLevelLow},
		{"clean PNG", "clean/clean.png", db.RiskLevelLow},
		{"clean JPEG", "clean/clean.jpg", db.RiskLevelLow},
		{"clean text", "clean/clean.txt", db.RiskLevelLow},
		{"inert PDF markers", "suspicious-inert/pdf-js-markers.pdf", db.RiskLevelHigh},
		{"inert PNG markers", "suspicious-inert/png-encoded-marker.png", db.RiskLevelMedium},
		{"inert JPEG markers", "suspicious-inert/jpeg-encoded-marker.jpg", db.RiskLevelMedium},
		{"fictional JPEG GPS", "suspicious-inert/jpeg-exif-gps.jpg", db.RiskLevelHigh},
		{"inert text markers", "suspicious-inert/text-eval-marker.txt", db.RiskLevelMedium},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(tc.path)))
			if err != nil {
				t.Fatal(err)
			}
			mimeType := http.DetectContentType(content)
			result := AnalyzeFile(content, mimeType)
			if result.RiskScore.Level != tc.want {
				t.Fatalf("%s: got %s risk for %s, want %s", tc.path, result.RiskScore.Level, mimeType, tc.want)
			}
		})
	}
}
