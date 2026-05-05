package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dataguardian/backend-go/internal/config"
	"dataguardian/backend-go/internal/db"
)

func TestAnalysisRoutesExistAndRequireAuth(t *testing.T) {
	router := NewRouter(config.Settings{AuthCookieName: "dataguardian_session"}, nil)

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/analyses", body: `{"projectId":1,"inputType":"URL","url":{"originalUrl":"https://example.com"}}`},
		{method: http.MethodGet, path: "/analyses/1"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s returned %d, expected auth-protected route status %d", tc.method, tc.path, rec.Code, http.StatusUnauthorized)
		}
	}
}

func TestValidateCreateAnalysisRequest(t *testing.T) {
	validFile := CreateAnalysisRequest{
		ProjectID: 1,
		InputType: db.InputTypeFile,
		File: &AnalysisFileInput{
			OriginalFilename: "report.pdf",
			StoredReference:  "uploads/report.pdf",
			MimeType:         "application/pdf",
			SizeBytes:        100,
			ChecksumSHA256:   strings.Repeat("a", 64),
		},
	}
	validURL := CreateAnalysisRequest{
		ProjectID: 1,
		InputType: db.InputTypeURL,
		URL:       &AnalysisURLInput{OriginalURL: "https://example.com"},
	}

	for _, tc := range []struct {
		name    string
		payload CreateAnalysisRequest
		wantErr bool
	}{
		{name: "valid file", payload: validFile},
		{name: "valid url", payload: validURL},
		{name: "missing project", payload: CreateAnalysisRequest{InputType: db.InputTypeURL, URL: validURL.URL}, wantErr: true},
		{name: "invalid input type", payload: CreateAnalysisRequest{ProjectID: 1, InputType: "OTHER", URL: validURL.URL}, wantErr: true},
		{name: "missing input", payload: CreateAnalysisRequest{ProjectID: 1, InputType: db.InputTypeURL}, wantErr: true},
		{name: "both inputs", payload: CreateAnalysisRequest{ProjectID: 1, InputType: db.InputTypeURL, File: validFile.File, URL: validURL.URL}, wantErr: true},
		{name: "mismatched file type", payload: CreateAnalysisRequest{ProjectID: 1, InputType: db.InputTypeFile, URL: validURL.URL}, wantErr: true},
		{name: "mismatched url type", payload: CreateAnalysisRequest{ProjectID: 1, InputType: db.InputTypeURL, File: validFile.File}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errDetail := validateCreateAnalysisRequest(tc.payload)
			if tc.wantErr && errDetail == "" {
				t.Fatal("expected validation error")
			}
			if !tc.wantErr && errDetail != "" {
				t.Fatalf("expected valid request, got %q", errDetail)
			}
		})
	}
}

func TestAnalysisDTOJSONRoundTrip(t *testing.T) {
	completedAt := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	response := AnalysisResponse{
		AnalysisID:  10,
		ProjectID:   20,
		InputType:   db.InputTypeURL,
		Status:      db.AnalysisStatusCompleted,
		Summary:     "Analysis completed.",
		StartedAt:   completedAt.Add(-time.Minute),
		CompletedAt: &completedAt,
		URLTarget: &AnalysisURLTarget{
			ID:            30,
			OriginalURL:   "https://example.com",
			RedirectChain: []string{},
			UsesHTTPS:     true,
			Host:          "example.com",
			FetchStatus:   db.FetchStatusSuccess,
		},
		Findings: []AnalysisFinding{},
		Metadata: AnalysisMetadata{
			ID:         40,
			SourceType: db.MetadataSourceTypeURLContent,
			Entries: []db.MetadataEntry{
				{
					Key:         "content_type",
					Value:       "text/html",
					Category:    db.MetadataCategoryURL,
					Sensitivity: db.MetadataSensitivityNonSensitive,
					Source:      "headers",
					Confidence:  db.MetadataConfidenceHigh,
				},
			},
		},
		RiskScore: AnalysisRiskScore{
			Score:   5,
			Level:   db.RiskLevelLow,
			Drivers: []db.RiskDriver{},
		},
		CleanFile: nil,
	}

	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if !bytes.Contains(encoded, []byte(`"analysisId":10`)) {
		t.Fatalf("expected analysisId in JSON, got %s", encoded)
	}
	if !bytes.Contains(encoded, []byte(`"urlTarget"`)) {
		t.Fatalf("expected urlTarget in JSON, got %s", encoded)
	}

	var decoded AnalysisResponse
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if decoded.AnalysisID != response.AnalysisID || decoded.InputType != db.InputTypeURL {
		t.Fatalf("decoded response mismatch: %#v", decoded)
	}
}
