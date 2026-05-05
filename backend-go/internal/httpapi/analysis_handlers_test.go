package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

func TestCreateFileAnalysisUploadSuccess(t *testing.T) {
	store := newFakeAnalysisStore()
	srv := &server{
		cfg:   config.Settings{StorageDir: t.TempDir()},
		store: store,
	}
	body, contentType := multipartAnalysisBody(t, map[string]string{
		"projectId": "7",
		"inputType": string(db.InputTypeFile),
	}, "sample.pdf", []byte("%PDF-1.7\n/OpenAction << /JS (JavaScript) >>"))
	req := httptest.NewRequest(http.MethodPost, "/analyses", body)
	req.Header.Set("Content-Type", contentType)
	req = req.WithContext(withUserID(req.Context(), 42))
	rec := httptest.NewRecorder()

	srv.createAnalysis(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("createAnalysis returned %d with body %s", rec.Code, rec.Body.String())
	}
	var response AnalysisResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("response JSON did not decode: %v", err)
	}
	if response.AnalysisID == 0 || response.ProjectID != 7 || response.File == nil {
		t.Fatalf("unexpected response shape: %#v", response)
	}
	if response.RiskScore.Level != db.RiskLevelHigh {
		t.Fatalf("expected high risk, got %s", response.RiskScore.Level)
	}
	if len(response.Findings) == 0 {
		t.Fatal("expected findings in response")
	}
	if response.CleanFile != nil {
		t.Fatal("cleanFile must be null for the first file-analysis slice")
	}
	if store.createdFile.StoredReference == "" {
		t.Fatal("expected stored file reference")
	}
	if !strings.HasPrefix(filepath.Clean(store.createdFile.StoredReference), filepath.Clean(srv.cfg.StorageDir)) {
		t.Fatalf("stored file escaped storage dir: %s", store.createdFile.StoredReference)
	}
}

func TestCreateFileAnalysisRejectsInvalidFileType(t *testing.T) {
	srv := &server{
		cfg:   config.Settings{StorageDir: t.TempDir()},
		store: newFakeAnalysisStore(),
	}
	body, contentType := multipartAnalysisBody(t, map[string]string{
		"projectId": "7",
		"inputType": string(db.InputTypeFile),
	}, "notes.txt", []byte("plain text is not an allowed upload type"))
	req := httptest.NewRequest(http.MethodPost, "/analyses", body)
	req.Header.Set("Content-Type", contentType)
	req = req.WithContext(withUserID(req.Context(), 42))
	rec := httptest.NewRecorder()

	srv.createAnalysis(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("createAnalysis returned %d, expected %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "Unsupported file type") {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
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

func multipartAnalysisBody(t *testing.T, fields map[string]string, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("WriteField returned error: %v", err)
		}
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("part.Write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close returned error: %v", err)
	}
	return body, writer.FormDataContentType()
}

type fakeAnalysisStore struct {
	analysis    db.Analysis
	createdFile db.File
	metadata    db.Metadata
	findings    []db.Finding
	riskScore   db.RiskScore
}

func newFakeAnalysisStore() *fakeAnalysisStore {
	return &fakeAnalysisStore{}
}

func (f *fakeAnalysisStore) UserByID(ctx context.Context, id int64) (db.User, error) {
	return db.User{ID: id, Email: "test@dataguardian.dev"}, nil
}

func (f *fakeAnalysisStore) UserByEmail(ctx context.Context, email string) (db.User, error) {
	return db.User{ID: 42, Email: email}, nil
}

func (f *fakeAnalysisStore) CreateUser(ctx context.Context, email string, hashedPassword string, tenantID *string) (db.User, error) {
	return db.User{ID: 42, Email: email}, nil
}

func (f *fakeAnalysisStore) EnsureLocalUser(ctx context.Context, email string, hashedPassword string, isAdmin bool) error {
	return nil
}

func (f *fakeAnalysisStore) CreateProject(ctx context.Context, userID int64, name string, target string) (db.Project, error) {
	return db.Project{ID: 7, UserID: userID, Name: name, Target: target}, nil
}

func (f *fakeAnalysisStore) ProjectsByUser(ctx context.Context, userID int64) ([]db.Project, error) {
	return []db.Project{{ID: 7, UserID: userID, Name: "Project", Target: "Target"}}, nil
}

func (f *fakeAnalysisStore) ProjectByID(ctx context.Context, userID int64, projectID int64) (db.Project, error) {
	return db.Project{ID: projectID, UserID: userID, Name: "Project", Target: "Target"}, nil
}

func (f *fakeAnalysisStore) CreateAudit(ctx context.Context, projectID int64, status string, summary string, findings []string) (db.Audit, error) {
	return db.Audit{ID: 1, ProjectID: projectID, Status: status, Summary: summary, Findings: findings}, nil
}

func (f *fakeAnalysisStore) AuditsByProject(ctx context.Context, projectID int64) ([]db.Audit, error) {
	return nil, nil
}

func (f *fakeAnalysisStore) CreateAnalysis(ctx context.Context, analysis db.Analysis) (db.Analysis, error) {
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	analysis.ID = 99
	analysis.StartedAt = now
	f.analysis = analysis
	return analysis, nil
}

func (f *fakeAnalysisStore) CompleteAnalysis(ctx context.Context, analysisID int64, summary string) (db.Analysis, error) {
	completed := f.analysis.StartedAt.Add(time.Second)
	f.analysis.ID = analysisID
	f.analysis.Status = db.AnalysisStatusCompleted
	f.analysis.Summary = summary
	f.analysis.CompletedAt = &completed
	return f.analysis, nil
}

func (f *fakeAnalysisStore) GetAnalysisByID(ctx context.Context, userID int64, analysisID int64) (db.Analysis, error) {
	return f.analysis, nil
}

func (f *fakeAnalysisStore) ListAnalysesByProject(ctx context.Context, userID int64, projectID int64) ([]db.Analysis, error) {
	return []db.Analysis{f.analysis}, nil
}

func (f *fakeAnalysisStore) CreateFile(ctx context.Context, file db.File) (db.File, error) {
	file.ID = 100
	file.CreatedAt = f.analysis.StartedAt
	f.createdFile = file
	return file, nil
}

func (f *fakeAnalysisStore) FileByAnalysisID(ctx context.Context, analysisID int64) (db.File, error) {
	return f.createdFile, nil
}

func (f *fakeAnalysisStore) CreateURLTarget(ctx context.Context, target db.URLTarget) (db.URLTarget, error) {
	return db.URLTarget{}, db.ErrNotImplemented
}

func (f *fakeAnalysisStore) SaveMetadata(ctx context.Context, metadata db.Metadata) (db.Metadata, error) {
	metadata.ID = 101
	metadata.CreatedAt = f.analysis.StartedAt
	f.metadata = metadata
	return metadata, nil
}

func (f *fakeAnalysisStore) MetadataByAnalysisID(ctx context.Context, analysisID int64) (db.Metadata, error) {
	return f.metadata, nil
}

func (f *fakeAnalysisStore) SaveFindings(ctx context.Context, analysisID int64, findings []db.Finding) error {
	f.findings = make([]db.Finding, 0, len(findings))
	for index, finding := range findings {
		finding.ID = int64(index + 1)
		finding.AnalysisID = analysisID
		f.findings = append(f.findings, finding)
	}
	return nil
}

func (f *fakeAnalysisStore) FindingsByAnalysisID(ctx context.Context, analysisID int64) ([]db.Finding, error) {
	return f.findings, nil
}

func (f *fakeAnalysisStore) SaveRiskScore(ctx context.Context, score db.RiskScore) (db.RiskScore, error) {
	score.ID = 102
	f.riskScore = score
	return score, nil
}

func (f *fakeAnalysisStore) RiskScoreByAnalysisID(ctx context.Context, analysisID int64) (db.RiskScore, error) {
	return f.riskScore, nil
}

func (f *fakeAnalysisStore) SaveCleanFile(ctx context.Context, cleanFile db.CleanFile) (db.CleanFile, error) {
	return db.CleanFile{}, db.ErrNotImplemented
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
