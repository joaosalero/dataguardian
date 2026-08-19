package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"dataguardian/backend-go/internal/analysis"
	"dataguardian/backend-go/internal/config"
	"dataguardian/backend-go/internal/db"
)

var errTestStorage = errors.New("test storage failure")

func TestAnalysisRoutesExistAndRequireAuth(t *testing.T) {
	router := NewRouter(config.Settings{AuthCookieName: "dataguardian_session"}, nil)

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/analyses"},
		{method: http.MethodPost, path: "/analyses", body: `{"projectId":1,"inputType":"URL","url":{"originalUrl":"https://example.com"}}`},
		{method: http.MethodGet, path: "/analyses/1"},
		{method: http.MethodDelete, path: "/analyses/1"},
		{method: http.MethodGet, path: "/analyses/1/file"},
		{method: http.MethodGet, path: "/analyses/1/clean-file"},
		{method: http.MethodGet, path: "/storage"},
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
	}, "sample.pdf", []byte("%PDF-1.7\n<< /Author (Alice) /OpenAction << /JS (JavaScript) >> >>"))
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
	if response.CleanFile == nil || response.CleanFile.CleaningStatus != db.CleaningStatusCompleted {
		t.Fatalf("expected completed cleanFile, got %#v", response.CleanFile)
	}
	if response.SafePreview == nil || !response.SafePreview.Available || response.SafePreview.Kind != "image" {
		t.Fatalf("expected static safe preview, got %#v", response.SafePreview)
	}
	if len(response.Metadata.Entries) == 0 {
		t.Fatal("expected metadata entries in response")
	}
	if !metadataEntryExists(response.Metadata.Entries, "author") {
		t.Fatalf("expected extracted author metadata, got %#v", response.Metadata.Entries)
	}
	if store.createdFile.StoredReference == "" {
		t.Fatal("expected stored file reference")
	}
	if !strings.HasPrefix(filepath.Clean(store.createdFile.StoredReference), filepath.Clean(srv.cfg.StorageDir)) {
		t.Fatalf("stored file escaped storage dir: %s", store.createdFile.StoredReference)
	}
}

func metadataEntryExists(entries []db.MetadataEntry, key string) bool {
	for _, entry := range entries {
		if entry.Key == key {
			return true
		}
	}
	return false
}

func TestDownloadOriginalFileSuccess(t *testing.T) {
	for _, inputType := range []db.InputType{db.InputTypeFile, db.InputTypeURL} {
		t.Run(string(inputType), func(t *testing.T) {
			storageDir := t.TempDir()
			content := []byte("%PDF-1.7\noriginal")
			path := filepath.Join(storageDir, "original.pdf")
			if err := os.WriteFile(path, content, 0o600); err != nil {
				t.Fatalf("WriteFile returned error: %v", err)
			}
			store := newFakeAnalysisStore()
			store.analysis = db.Analysis{ID: 99, ProjectID: 7, InputType: inputType, Status: db.AnalysisStatusCompleted}
			store.createdFile = db.File{
				ID:               100,
				AnalysisID:       99,
				OriginalFilename: "sample.pdf",
				StoredReference:  path,
				MimeType:         "application/pdf",
				SizeBytes:        int64(len(content)),
			}
			srv := &server{
				cfg:   config.Settings{StorageDir: storageDir},
				store: store,
			}
			req := httptest.NewRequest(http.MethodGet, "/analyses/99/file", nil)
			req.SetPathValue("id", "99")
			req = req.WithContext(withUserID(req.Context(), 42))
			rec := httptest.NewRecorder()

			srv.downloadOriginalFile(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("downloadOriginalFile returned %d with body %s", rec.Code, rec.Body.String())
			}
			if rec.Body.String() != string(content) {
				t.Fatalf("unexpected download body: %q", rec.Body.String())
			}
			if disposition := rec.Header().Get("Content-Disposition"); !strings.Contains(disposition, "attachment") || !strings.Contains(disposition, "sample.pdf") {
				t.Fatalf("expected attachment disposition, got %q", disposition)
			}
			if cacheControl := rec.Header().Get("Cache-Control"); cacheControl != "no-store" {
				t.Fatalf("expected no-store cache control, got %q", cacheControl)
			}
		})
	}
}

func TestDownloadCleanFileSuccess(t *testing.T) {
	for _, inputType := range []db.InputType{db.InputTypeFile, db.InputTypeURL} {
		t.Run(string(inputType), func(t *testing.T) {
			storageDir := t.TempDir()
			content := []byte("%PDF-1.7\nclean")
			path := filepath.Join(storageDir, "clean.pdf")
			if err := os.WriteFile(path, content, 0o600); err != nil {
				t.Fatalf("WriteFile returned error: %v", err)
			}
			store := newFakeAnalysisStore()
			store.analysis = db.Analysis{ID: 99, ProjectID: 7, InputType: inputType, Status: db.AnalysisStatusCompleted}
			store.cleanFile = db.CleanFile{
				ID:              104,
				AnalysisID:      99,
				StoredReference: path,
				Filename:        "sample-clean.pdf",
				MimeType:        "application/pdf",
				SizeBytes:       int64(len(content)),
				CleaningStatus:  db.CleaningStatusCompleted,
			}
			srv := &server{
				cfg:   config.Settings{StorageDir: storageDir},
				store: store,
			}
			req := httptest.NewRequest(http.MethodGet, "/analyses/99/clean-file", nil)
			req.SetPathValue("id", "99")
			req = req.WithContext(withUserID(req.Context(), 42))
			rec := httptest.NewRecorder()

			srv.downloadCleanFile(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("downloadCleanFile returned %d with body %s", rec.Code, rec.Body.String())
			}
			if rec.Body.String() != string(content) {
				t.Fatalf("unexpected download body: %q", rec.Body.String())
			}
			if disposition := rec.Header().Get("Content-Disposition"); !strings.Contains(disposition, "attachment") || !strings.Contains(disposition, "sample-clean.pdf") {
				t.Fatalf("expected attachment disposition, got %q", disposition)
			}
			if cacheControl := rec.Header().Get("Cache-Control"); cacheControl != "no-store" {
				t.Fatalf("expected no-store cache control, got %q", cacheControl)
			}
		})
	}
}

func TestDownloadCleanFileReturnsNotFoundWhenMissing(t *testing.T) {
	store := newFakeAnalysisStore()
	store.analysis = db.Analysis{ID: 99, ProjectID: 7, InputType: db.InputTypeFile, Status: db.AnalysisStatusCompleted}
	srv := &server{
		cfg:   config.Settings{StorageDir: t.TempDir()},
		store: store,
	}
	req := httptest.NewRequest(http.MethodGet, "/analyses/99/clean-file", nil)
	req.SetPathValue("id", "99")
	req = req.WithContext(withUserID(req.Context(), 42))
	rec := httptest.NewRecorder()

	srv.downloadCleanFile(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("downloadCleanFile returned %d, expected %d", rec.Code, http.StatusNotFound)
	}
}

func TestDownloadOriginalFileReturnsNotFoundForInvalidStoredReference(t *testing.T) {
	store := newFakeAnalysisStore()
	store.analysis = db.Analysis{ID: 99, ProjectID: 7, InputType: db.InputTypeFile, Status: db.AnalysisStatusCompleted}
	store.createdFile = db.File{
		ID:               100,
		AnalysisID:       99,
		OriginalFilename: "sample.pdf",
		StoredReference:  filepath.Join(t.TempDir(), "..", "outside.pdf"),
		MimeType:         "application/pdf",
	}
	srv := &server{
		cfg:   config.Settings{StorageDir: t.TempDir()},
		store: store,
	}
	req := httptest.NewRequest(http.MethodGet, "/analyses/99/file", nil)
	req.SetPathValue("id", "99")
	req = req.WithContext(withUserID(req.Context(), 42))
	rec := httptest.NewRecorder()

	srv.downloadOriginalFile(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("downloadOriginalFile returned %d, expected %d", rec.Code, http.StatusNotFound)
	}
	if strings.Contains(rec.Body.String(), "outside.pdf") {
		t.Fatalf("response leaked stored path: %s", rec.Body.String())
	}
}

func TestStoredFilePathForWriteRejectsUnsafeFilenames(t *testing.T) {
	storageDir := t.TempDir()
	path, ok := storedFilePathForWrite(storageDir, "checksum-token.pdf")
	if !ok {
		t.Fatal("expected generated filename to be accepted")
	}
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(storageDir)+string(filepath.Separator)) {
		t.Fatalf("stored write path escaped storage dir: %s", path)
	}
	for _, filename := range []string{"../escape.pdf", "nested/file.pdf", filepath.Join(t.TempDir(), "absolute.pdf"), ""} {
		if path, ok := storedFilePathForWrite(storageDir, filename); ok {
			t.Fatalf("expected unsafe filename %q to be rejected, got %s", filename, path)
		}
	}
}

func TestSampleCorpusRejectedExtensionMismatches(t *testing.T) {
	root := filepath.Join("..", "..", "..", "samples", "rejected")
	for _, name := range []string{"mismatched-extension.jpg", "malformed.pdf"} {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		mimeType := http.DetectContentType(content)
		if extensionMatchesMimeType(strings.ToLower(filepath.Ext(name)), mimeType) {
			t.Fatalf("expected %s (%s) to be rejected", name, mimeType)
		}
	}
}

func TestStorageSummaryRequiresAdmin(t *testing.T) {
	store := newFakeAnalysisStore()
	srv := &server{cfg: config.Settings{StorageDir: t.TempDir()}, store: store}
	req := httptest.NewRequest(http.MethodGet, "/storage", nil)
	req = req.WithContext(withUserID(req.Context(), 42))
	rec := httptest.NewRecorder()

	srv.requireAdmin(srv.storageSummary)(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("storageSummary returned %d, expected %d", rec.Code, http.StatusForbidden)
	}
}

func TestStorageSummaryAllowsAdmin(t *testing.T) {
	storageDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(storageDir, "artifact.txt"), []byte("artifact"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	store := newFakeAnalysisStore()
	store.isAdmin = true
	srv := &server{cfg: config.Settings{StorageDir: storageDir, StorageOrphanRetentionHours: 24}, store: store}
	req := httptest.NewRequest(http.MethodGet, "/storage", nil)
	req = req.WithContext(withUserID(req.Context(), 42))
	rec := httptest.NewRecorder()

	srv.requireAdmin(srv.storageSummary)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("storageSummary returned %d with body %s", rec.Code, rec.Body.String())
	}
	var payload StorageSummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response JSON did not decode: %v", err)
	}
	if payload.FileCount != 1 || payload.TotalBytes != int64(len("artifact")) {
		t.Fatalf("unexpected storage summary: %#v", payload)
	}
}

func TestCreateFileAnalysisCleanFileFailureIsRecorded(t *testing.T) {
	store := newFakeAnalysisStore()
	srv := &server{
		cfg:   config.Settings{StorageDir: t.TempDir()},
		store: store,
	}
	originalWriteStoredFile := writeStoredFile
	writeCount := 0
	writeStoredFile = func(storageDir string, filename string, content []byte) error {
		writeCount++
		if writeCount == 2 {
			return errTestStorage
		}
		return originalWriteStoredFile(storageDir, filename, content)
	}
	defer func() { writeStoredFile = originalWriteStoredFile }()

	body, contentType := multipartAnalysisBody(t, map[string]string{
		"projectId": "7",
		"inputType": string(db.InputTypeFile),
	}, "sample.pdf", []byte("%PDF-1.7\n<< /Author (Alice) >>"))
	req := httptest.NewRequest(http.MethodPost, "/analyses", body)
	req.Header.Set("Content-Type", contentType)
	req = req.WithContext(withUserID(req.Context(), 42))
	rec := httptest.NewRecorder()

	srv.createAnalysis(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("createAnalysis returned %d with body %s", rec.Code, rec.Body.String())
	}
	if store.cleanFile.CleaningStatus != db.CleaningStatusFailed {
		t.Fatalf("expected failed clean file recorded, got %#v", store.cleanFile)
	}
}

func TestCreateTextFileAnalysisReturnsSafePreviewWithoutCleanFile(t *testing.T) {
	store := newFakeAnalysisStore()
	srv := &server{
		cfg:   config.Settings{StorageDir: t.TempDir()},
		store: store,
	}
	body, contentType := multipartAnalysisBody(t, map[string]string{
		"projectId": "7",
		"inputType": string(db.InputTypeFile),
	}, "note.txt", []byte("Suspicious note\nDo not execute anything."))
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
	if response.CleanFile != nil {
		t.Fatalf("expected no clean file for text upload, got %#v", response.CleanFile)
	}
	if response.SafePreview == nil || !response.SafePreview.Available || !strings.Contains(response.SafePreview.Text, "Suspicious note") {
		t.Fatalf("expected text safe preview, got %#v", response.SafePreview)
	}
}

func TestCreateFileAnalysisRejectsInvalidProjectID(t *testing.T) {
	srv := &server{
		cfg:   config.Settings{StorageDir: t.TempDir()},
		store: newFakeAnalysisStore(),
	}
	body, contentType := multipartAnalysisBody(t, map[string]string{
		"projectId": "invalid",
		"inputType": string(db.InputTypeFile),
	}, "sample.pdf", []byte("%PDF-1.7"))
	req := httptest.NewRequest(http.MethodPost, "/analyses", body)
	req.Header.Set("Content-Type", contentType)
	req = req.WithContext(withUserID(req.Context(), 42))
	rec := httptest.NewRecorder()

	srv.createAnalysis(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("createAnalysis returned %d, expected %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "Project id is required") {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}

func TestReadLimitedFileRejectsOversizedFile(t *testing.T) {
	content := strings.NewReader(strings.Repeat("A", maxAnalysisFileSize+1))

	if _, err := readLimitedFile(content); err == nil || !strings.Contains(err.Error(), "10MB") {
		t.Fatalf("expected oversized file error, got %v", err)
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
	}, "sample.bin", []byte{0x00, 0x01, 0x02, 0x03, 0x04})
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

func TestCreateURLAnalysisSuccess(t *testing.T) {
	store := newFakeAnalysisStore()
	srv := &server{store: store}
	originalAnalyzeURL := analyzeURL
	defer func() { analyzeURL = originalAnalyzeURL }()
	analyzeURL = func(ctx context.Context, raw string) (analysis.URLResult, error) {
		contentType := "text/html"
		contentLength := int64(128)
		statusCode := http.StatusOK
		finalURL := "https://example.com/final"
		target := db.URLTarget{
			OriginalURL:        raw,
			FinalURL:           &finalURL,
			RedirectCount:      1,
			RedirectChain:      []string{finalURL},
			UsesHTTPS:          true,
			Host:               "example.com",
			ContentType:        &contentType,
			ContentLengthBytes: &contentLength,
			HTTPStatusCode:     &statusCode,
			FetchStatus:        db.FetchStatusSuccess,
		}
		findings := []db.Finding{
			{
				Type:        db.FindingTypeURL,
				Code:        "URL_REDIRECT_DETECTED",
				Title:       "URL redirect detected",
				Description: "The submitted URL redirected before returning final content.",
				Severity:    db.SeverityLow,
				Evidence: db.FindingEvidence{
					Source: db.FindingEvidenceSourceURL,
					RuleID: "URL_REDIRECT_DETECTED",
				},
			},
		}
		return analysis.URLResult{
			Target: target,
			Metadata: db.Metadata{
				SourceType: db.MetadataSourceTypeURLContent,
				Entries: []db.MetadataEntry{
					{
						Key:         "host",
						Value:       "example.com",
						Category:    db.MetadataCategoryURL,
						Sensitivity: db.MetadataSensitivityNonSensitive,
						Source:      "url",
						Confidence:  db.MetadataConfidenceHigh,
					},
					{
						Key:         "safe_preview_text",
						Value:       "Example preview",
						Category:    db.MetadataCategoryURL,
						Sensitivity: db.MetadataSensitivityNonSensitive,
						Source:      "safe_preview",
						Confidence:  db.MetadataConfidenceHigh,
					},
				},
			},
			Findings:  findings,
			RiskScore: analysis.ScoreFindings(findings),
			Summary:   "URL analysis completed with structured findings.",
		}, nil
	}
	req := httptest.NewRequest(http.MethodPost, "/analyses", strings.NewReader(`{"projectId":7,"inputType":"URL","url":{"originalUrl":"https://example.com"}}`))
	req.Header.Set("Content-Type", "application/json")
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
	if response.URLTarget == nil || response.File != nil || response.CleanFile != nil {
		t.Fatalf("unexpected URL response shape: %#v", response)
	}
	if response.URLTarget.HTTPStatusCode == nil || *response.URLTarget.HTTPStatusCode != http.StatusOK {
		t.Fatalf("expected http status in URL target: %#v", response.URLTarget)
	}
	if response.Metadata.SourceType != db.MetadataSourceTypeURLContent {
		t.Fatalf("expected URL metadata, got %s", response.Metadata.SourceType)
	}
	if len(response.Findings) != 1 || response.Findings[0].Code != "URL_REDIRECT_DETECTED" {
		t.Fatalf("expected redirect finding, got %#v", response.Findings)
	}
	if response.SafePreview == nil || !response.SafePreview.Available || response.SafePreview.Text != "Example preview" {
		t.Fatalf("expected URL safe preview, got %#v", response.SafePreview)
	}
}

func TestCreateURLAnalysisPersistsRemoteFileInspection(t *testing.T) {
	store := newFakeAnalysisStore()
	srv := &server{
		cfg:   config.Settings{StorageDir: t.TempDir()},
		store: store,
	}
	originalAnalyzeURL := analyzeURL
	defer func() { analyzeURL = originalAnalyzeURL }()
	analyzeURL = func(ctx context.Context, raw string) (analysis.URLResult, error) {
		contentType := "application/pdf"
		contentLength := int64(54)
		statusCode := http.StatusOK
		target := db.URLTarget{
			OriginalURL:        raw,
			FinalURL:           &raw,
			UsesHTTPS:          true,
			Host:               "example.com",
			ContentType:        &contentType,
			ContentLengthBytes: &contentLength,
			HTTPStatusCode:     &statusCode,
			FetchStatus:        db.FetchStatusSuccess,
		}
		findings := []db.Finding{
			{
				Type:        db.FindingTypeURL,
				Code:        "URL_REMOTE_FILE_DETECTED",
				Title:       "Remote downloadable file detected",
				Description: "The fetched URL returned a supported file type.",
				Severity:    db.SeverityLow,
				Evidence: db.FindingEvidence{
					Source: db.FindingEvidenceSourceURL,
					RuleID: "URL_REMOTE_FILE_DETECTED",
				},
			},
		}
		return analysis.URLResult{
			Target: target,
			Metadata: db.Metadata{
				SourceType: db.MetadataSourceTypeURLContent,
				Entries: []db.MetadataEntry{
					{
						Key:         "content_type",
						Value:       "application/pdf",
						Category:    db.MetadataCategoryURL,
						Sensitivity: db.MetadataSensitivityNonSensitive,
						Source:      "headers",
						Confidence:  db.MetadataConfidenceHigh,
					},
				},
			},
			Findings:  findings,
			RiskScore: analysis.ScoreFindings(findings),
			Summary:   "URL analysis completed with a remote file inspection candidate.",
			RemoteFile: &analysis.RemoteFile{
				Filename:  "remote.pdf",
				MimeType:  "application/pdf",
				SizeBytes: contentLength,
				Content:   []byte("%PDF-1.7\n<< /Author (Remote Sender) /JS (JavaScript) >>"),
			},
		}, nil
	}
	req := httptest.NewRequest(http.MethodPost, "/analyses", strings.NewReader(`{"projectId":7,"inputType":"URL","url":{"originalUrl":"https://example.com/remote.pdf"}}`))
	req.Header.Set("Content-Type", "application/json")
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
	if response.URLTarget == nil || response.File == nil {
		t.Fatalf("expected URL response with remote file, got %#v", response)
	}
	if response.File.OriginalFilename != "remote.pdf" || response.File.MimeType != "application/pdf" {
		t.Fatalf("unexpected remote file reference: %#v", response.File)
	}
	if response.CleanFile == nil || response.CleanFile.CleaningStatus != db.CleaningStatusCompleted {
		t.Fatalf("expected sanitized remote file, got %#v", response.CleanFile)
	}
	if response.SafePreview == nil || !response.SafePreview.Available {
		t.Fatalf("expected remote file safe preview, got %#v", response.SafePreview)
	}
	if !metadataEntryExists(response.Metadata.Entries, "author") {
		t.Fatalf("expected remote file metadata to be included, got %#v", response.Metadata.Entries)
	}
}

func TestCreateURLAnalysisRejectsInvalidURL(t *testing.T) {
	srv := &server{store: newFakeAnalysisStore()}
	originalAnalyzeURL := analyzeURL
	defer func() { analyzeURL = originalAnalyzeURL }()
	analyzeURL = func(ctx context.Context, raw string) (analysis.URLResult, error) {
		return analysis.URLResult{}, analysis.ErrInvalidURL
	}
	req := httptest.NewRequest(http.MethodPost, "/analyses", strings.NewReader(`{"projectId":7,"inputType":"URL","url":{"originalUrl":"file:///etc/passwd"}}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(withUserID(req.Context(), 42))
	rec := httptest.NewRecorder()

	srv.createAnalysis(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("createAnalysis returned %d, expected %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "Invalid or unsafe URL") {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}

func TestCreateURLAnalysisRejectsBlockedURL(t *testing.T) {
	srv := &server{store: newFakeAnalysisStore()}
	originalAnalyzeURL := analyzeURL
	defer func() { analyzeURL = originalAnalyzeURL }()
	analyzeURL = func(ctx context.Context, raw string) (analysis.URLResult, error) {
		return analysis.URLResult{}, analysis.ErrUnsafeURL
	}
	req := httptest.NewRequest(http.MethodPost, "/analyses", strings.NewReader(`{"projectId":7,"inputType":"URL","url":{"originalUrl":"http://127.0.0.1/admin"}}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(withUserID(req.Context(), 42))
	rec := httptest.NewRecorder()

	srv.createAnalysis(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("createAnalysis returned %d, expected %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "Invalid or unsafe URL") {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}

func TestListAnalysesReturnsUserScopedHistory(t *testing.T) {
	store := newFakeAnalysisStore()
	first := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	second := first.Add(time.Hour)
	store.projects = []db.Project{
		{ID: 7, UserID: 42, Name: "Project A", Target: "db-a"},
		{ID: 8, UserID: 42, Name: "Project B", Target: "db-b"},
	}
	store.analysesByProject = map[int64][]db.Analysis{
		7: {
			{ID: 99, ProjectID: 7, InputType: db.InputTypeFile, Status: db.AnalysisStatusCompleted, StartedAt: first},
		},
		8: {
			{ID: 100, ProjectID: 8, InputType: db.InputTypeURL, Status: db.AnalysisStatusCompleted, StartedAt: second},
		},
	}
	store.riskScores = map[int64]db.RiskScore{
		99:  {AnalysisID: 99, Level: db.RiskLevelMedium},
		100: {AnalysisID: 100, Level: db.RiskLevelHigh},
	}
	srv := &server{store: store}
	req := httptest.NewRequest(http.MethodGet, "/analyses", nil)
	req = req.WithContext(withUserID(req.Context(), 42))
	rec := httptest.NewRecorder()

	srv.listAnalyses(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("listAnalyses returned %d with body %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Analyses []AnalysisListItem `json:"analyses"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response JSON did not decode: %v", err)
	}
	if len(payload.Analyses) != 2 {
		t.Fatalf("expected two analyses, got %#v", payload.Analyses)
	}
	if payload.Analyses[0].AnalysisID != 100 || payload.Analyses[0].RiskLevel != db.RiskLevelHigh {
		t.Fatalf("expected newest high-risk URL first, got %#v", payload.Analyses[0])
	}
}

func TestListAnalysesAppliesFiltersAndPagination(t *testing.T) {
	store := newFakeAnalysisStore()
	store.projects = []db.Project{{ID: 7, UserID: 42}, {ID: 8, UserID: 42}}
	store.analysesByProject = map[int64][]db.Analysis{
		7: {
			{ID: 1, ProjectID: 7, InputType: db.InputTypeFile, Status: db.AnalysisStatusCompleted, StartedAt: time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)},
			{ID: 2, ProjectID: 7, InputType: db.InputTypeURL, Status: db.AnalysisStatusCompleted, StartedAt: time.Date(2026, 5, 5, 11, 0, 0, 0, time.UTC)},
		},
		8: {
			{ID: 3, ProjectID: 8, InputType: db.InputTypeFile, Status: db.AnalysisStatusFailed, StartedAt: time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC)},
		},
	}
	store.riskScores = map[int64]db.RiskScore{
		1: {Level: db.RiskLevelHigh},
		2: {Level: db.RiskLevelLow},
		3: {Level: db.RiskLevelHigh},
	}
	srv := &server{store: store}
	req := httptest.NewRequest(http.MethodGet, "/analyses?inputType=FILE&riskLevel=HIGH&page=1&pageSize=1", nil)
	req = req.WithContext(withUserID(req.Context(), 42))
	rec := httptest.NewRecorder()

	srv.listAnalyses(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("listAnalyses returned %d with body %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Analyses   []AnalysisListItem `json:"analyses"`
		Pagination AnalysisPagination `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response JSON did not decode: %v", err)
	}
	if len(payload.Analyses) != 1 || payload.Analyses[0].AnalysisID != 1 {
		t.Fatalf("unexpected filtered page: %#v", payload.Analyses)
	}
	if payload.Pagination.TotalItems != 2 || !payload.Pagination.HasNext {
		t.Fatalf("unexpected pagination: %#v", payload.Pagination)
	}
}

func TestListAnalysesRejectsMalformedPagination(t *testing.T) {
	srv := &server{store: newFakeAnalysisStore()}
	req := httptest.NewRequest(http.MethodGet, "/analyses?page=abc", nil)
	req = req.WithContext(withUserID(req.Context(), 42))
	rec := httptest.NewRecorder()

	srv.listAnalyses(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("listAnalyses returned %d, expected %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "Invalid page parameter") {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}

func TestListAnalysesOutOfRangePageIsEmptyWithPagination(t *testing.T) {
	store := newFakeAnalysisStore()
	store.projects = []db.Project{{ID: 7, UserID: 42}}
	store.analysesByProject = map[int64][]db.Analysis{
		7: {
			{ID: 1, ProjectID: 7, InputType: db.InputTypeFile, Status: db.AnalysisStatusCompleted, StartedAt: time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)},
		},
	}
	store.riskScores = map[int64]db.RiskScore{1: {Level: db.RiskLevelLow}}
	srv := &server{store: store}
	req := httptest.NewRequest(http.MethodGet, "/analyses?page=2&pageSize=10", nil)
	req = req.WithContext(withUserID(req.Context(), 42))
	rec := httptest.NewRecorder()

	srv.listAnalyses(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("listAnalyses returned %d with body %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Analyses   []AnalysisListItem `json:"analyses"`
		Pagination AnalysisPagination `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response JSON did not decode: %v", err)
	}
	if len(payload.Analyses) != 0 || payload.Pagination.TotalItems != 1 || payload.Pagination.TotalPages != 1 {
		t.Fatalf("unexpected out-of-range page response: %#v", payload)
	}
}

func TestDeleteAnalysisRemovesOwnedAnalysisAndArtifacts(t *testing.T) {
	storageDir := t.TempDir()
	originalPath := filepath.Join(storageDir, "original.pdf")
	cleanPath := filepath.Join(storageDir, "clean.pdf")
	if err := os.WriteFile(originalPath, []byte("original"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(cleanPath, []byte("clean"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	store := newFakeAnalysisStore()
	store.analysis = db.Analysis{ID: 99, ProjectID: 7, InputType: db.InputTypeFile}
	store.createdFile = db.File{ID: 100, AnalysisID: 99, StoredReference: originalPath}
	store.cleanFile = db.CleanFile{ID: 101, AnalysisID: 99, StoredReference: cleanPath}
	srv := &server{cfg: config.Settings{StorageDir: storageDir}, store: store}
	req := httptest.NewRequest(http.MethodDelete, "/analyses/99", nil)
	req.SetPathValue("id", "99")
	req = req.WithContext(withUserID(req.Context(), 42))
	rec := httptest.NewRecorder()

	srv.deleteAnalysis(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("deleteAnalysis returned %d with body %s", rec.Code, rec.Body.String())
	}
	if !store.deletedAnalysis {
		t.Fatal("expected analysis to be deleted from store")
	}
	if _, err := os.Stat(originalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected original artifact removed, got %v", err)
	}
	if _, err := os.Stat(cleanPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected clean artifact removed, got %v", err)
	}
}

func TestGetAnalysisReturnsNotFoundForUnauthorizedAnalysis(t *testing.T) {
	store := newFakeAnalysisStore()
	store.getAnalysisErr = db.ErrNotFound
	srv := &server{store: store}
	req := httptest.NewRequest(http.MethodGet, "/analyses/99", nil)
	req.SetPathValue("id", "99")
	req = req.WithContext(withUserID(req.Context(), 42))
	rec := httptest.NewRecorder()

	srv.getAnalysis(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("getAnalysis returned %d, expected %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetURLAnalysisSuccess(t *testing.T) {
	store := newFakeAnalysisStore()
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	store.analysis = db.Analysis{
		ID:          99,
		ProjectID:   7,
		InputType:   db.InputTypeURL,
		Status:      db.AnalysisStatusCompleted,
		Summary:     "URL analysis completed with structured findings.",
		StartedAt:   now,
		CompletedAt: &now,
	}
	finalURL := "https://example.com/final"
	store.urlTarget = db.URLTarget{
		ID:            103,
		AnalysisID:    99,
		OriginalURL:   "https://example.com",
		FinalURL:      &finalURL,
		RedirectCount: 1,
		RedirectChain: []string{finalURL},
		UsesHTTPS:     true,
		Host:          "example.com",
		FetchStatus:   db.FetchStatusSuccess,
	}
	store.metadata = db.Metadata{
		ID:         101,
		AnalysisID: 99,
		SourceType: db.MetadataSourceTypeURLContent,
		Entries: []db.MetadataEntry{
			{
				Key:         "host",
				Value:       "example.com",
				Category:    db.MetadataCategoryURL,
				Sensitivity: db.MetadataSensitivityNonSensitive,
				Source:      "url",
				Confidence:  db.MetadataConfidenceHigh,
			},
		},
	}
	store.findings = []db.Finding{
		{
			ID:         1,
			AnalysisID: 99,
			Type:       db.FindingTypeURL,
			Code:       "URL_REDIRECT_DETECTED",
			Title:      "URL redirect detected",
			Severity:   db.SeverityLow,
			Evidence: db.FindingEvidence{
				Source: db.FindingEvidenceSourceURL,
				RuleID: "URL_REDIRECT_DETECTED",
			},
		},
	}
	store.riskScore = db.RiskScore{
		ID:         102,
		AnalysisID: 99,
		Score:      10,
		Level:      db.RiskLevelLow,
		Drivers:    []db.RiskDriver{},
	}
	srv := &server{store: store}
	req := httptest.NewRequest(http.MethodGet, "/analyses/99", nil)
	req.SetPathValue("id", "99")
	req = req.WithContext(withUserID(req.Context(), 42))
	rec := httptest.NewRecorder()

	srv.getAnalysis(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getAnalysis returned %d with body %s", rec.Code, rec.Body.String())
	}
	var response AnalysisResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("response JSON did not decode: %v", err)
	}
	if response.URLTarget == nil || response.File != nil || response.CleanFile != nil {
		t.Fatalf("unexpected URL response shape: %#v", response)
	}
	if response.URLTarget.RedirectCount != 1 || response.URLTarget.RedirectChain[0] != finalURL {
		t.Fatalf("expected URL target redirect details, got %#v", response.URLTarget)
	}
	if response.Metadata.SourceType != db.MetadataSourceTypeURLContent {
		t.Fatalf("expected URL metadata, got %s", response.Metadata.SourceType)
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
	analysis          db.Analysis
	createdFile       db.File
	urlTarget         db.URLTarget
	metadata          db.Metadata
	findings          []db.Finding
	riskScore         db.RiskScore
	riskScores        map[int64]db.RiskScore
	cleanFile         db.CleanFile
	projects          []db.Project
	analysesByProject map[int64][]db.Analysis
	getAnalysisErr    error
	deletedAnalysis   bool
	isAdmin           bool
}

func newFakeAnalysisStore() *fakeAnalysisStore {
	return &fakeAnalysisStore{}
}

func (f *fakeAnalysisStore) UserByID(ctx context.Context, id int64) (db.User, error) {
	return db.User{ID: id, Email: "test@dataguardian.dev", IsAdmin: f.isAdmin}, nil
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
	if f.projects != nil {
		return f.projects, nil
	}
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
	if f.getAnalysisErr != nil {
		return db.Analysis{}, f.getAnalysisErr
	}
	return f.analysis, nil
}

func (f *fakeAnalysisStore) ListAnalysesByProject(ctx context.Context, userID int64, projectID int64) ([]db.Analysis, error) {
	if f.analysesByProject != nil {
		return f.analysesByProject[projectID], nil
	}
	return []db.Analysis{f.analysis}, nil
}

func (f *fakeAnalysisStore) ListAnalysesForUser(ctx context.Context, userID int64, filter db.AnalysisListFilter) ([]db.AnalysisListRow, int, error) {
	rows := []db.AnalysisListRow{}
	projects := f.projects
	if len(projects) == 0 {
		projects = []db.Project{{ID: f.analysis.ProjectID, UserID: userID}}
	}
	for _, project := range projects {
		analyses := f.analysesByProject[project.ID]
		if f.analysesByProject == nil && f.analysis.ID != 0 {
			analyses = []db.Analysis{f.analysis}
		}
		for _, analysis := range analyses {
			riskScore := f.riskScore
			if f.riskScores != nil {
				riskScore = f.riskScores[analysis.ID]
			}
			row := db.AnalysisListRow{
				AnalysisID: analysis.ID,
				ProjectID:  analysis.ProjectID,
				InputType:  analysis.InputType,
				Status:     analysis.Status,
				RiskLevel:  riskScore.Level,
				CreatedAt:  analysis.StartedAt,
			}
			if filter.InputType != nil && row.InputType != *filter.InputType {
				continue
			}
			if filter.RiskLevel != nil && row.RiskLevel != *filter.RiskLevel {
				continue
			}
			if filter.Status != nil && row.Status != *filter.Status {
				continue
			}
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].CreatedAt.After(rows[j].CreatedAt)
	})
	total := len(rows)
	start := (filter.Page - 1) * filter.PageSize
	if start >= total {
		return []db.AnalysisListRow{}, total, nil
	}
	end := start + filter.PageSize
	if end > total {
		end = total
	}
	return rows[start:end], total, nil
}

func (f *fakeAnalysisStore) DeleteAnalysis(ctx context.Context, userID int64, analysisID int64) error {
	if f.getAnalysisErr != nil {
		return f.getAnalysisErr
	}
	f.deletedAnalysis = true
	return nil
}

func (f *fakeAnalysisStore) CreateFile(ctx context.Context, file db.File) (db.File, error) {
	file.ID = 100
	file.CreatedAt = f.analysis.StartedAt
	f.createdFile = file
	return file, nil
}

func (f *fakeAnalysisStore) FileByAnalysisID(ctx context.Context, analysisID int64) (db.File, error) {
	if f.createdFile.ID == 0 {
		return db.File{}, db.ErrNotFound
	}
	return f.createdFile, nil
}

func (f *fakeAnalysisStore) CreateURLTarget(ctx context.Context, target db.URLTarget) (db.URLTarget, error) {
	target.ID = 103
	f.urlTarget = target
	return target, nil
}

func (f *fakeAnalysisStore) URLTargetByAnalysisID(ctx context.Context, analysisID int64) (db.URLTarget, error) {
	return f.urlTarget, nil
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
	if f.riskScores != nil {
		return f.riskScores[analysisID], nil
	}
	return f.riskScore, nil
}

func (f *fakeAnalysisStore) SaveCleanFile(ctx context.Context, cleanFile db.CleanFile) (db.CleanFile, error) {
	cleanFile.ID = 104
	cleanFile.CreatedAt = f.analysis.StartedAt
	f.cleanFile = cleanFile
	return cleanFile, nil
}

func (f *fakeAnalysisStore) CleanFileByAnalysisID(ctx context.Context, analysisID int64) (db.CleanFile, error) {
	if f.cleanFile.ID == 0 {
		return db.CleanFile{}, db.ErrNotFound
	}
	return f.cleanFile, nil
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
