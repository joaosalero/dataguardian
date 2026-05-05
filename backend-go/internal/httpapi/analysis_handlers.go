package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"dataguardian/backend-go/internal/analysis"
	"dataguardian/backend-go/internal/db"
)

const maxAnalysisFileSize = 10 * 1024 * 1024

var analyzeURL = analysis.AnalyzeURL

func (s *server) createAnalysis(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		s.createFileAnalysis(w, r)
		return
	}

	var payload CreateAnalysisRequest
	if !readJSON(w, r, &payload) {
		return
	}
	if detail := validateCreateAnalysisRequest(payload); detail != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": detail})
		return
	}
	if payload.InputType == db.InputTypeURL {
		s.createURLAnalysis(w, r, payload)
		return
	}
	writeJSON(w, http.StatusNotImplemented, map[string]string{"detail": "Only FILE multipart analysis is implemented"})
}

func (s *server) createURLAnalysis(w http.ResponseWriter, r *http.Request, payload CreateAnalysisRequest) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "Invalid authentication credentials"})
		return
	}
	if _, err := s.store.ProjectByID(r.Context(), userID, payload.ProjectID); errors.Is(err, db.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Project not found"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}

	result, err := analyzeURL(r.Context(), payload.URL.OriginalURL)
	if errors.Is(err, analysis.ErrInvalidURL) || errors.Is(err, analysis.ErrUnsafeURL) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid or unsafe URL"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid or unsafe URL"})
		return
	}

	root, err := s.store.CreateAnalysis(r.Context(), db.Analysis{
		ProjectID: payload.ProjectID,
		InputType: db.InputTypeURL,
		Status:    db.AnalysisStatusProcessing,
		Summary:   "URL analysis is processing.",
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	result.Target.AnalysisID = root.ID
	createdTarget, err := s.store.CreateURLTarget(r.Context(), result.Target)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	result.Metadata.AnalysisID = root.ID
	metadata, err := s.store.SaveMetadata(r.Context(), result.Metadata)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	if err := s.store.SaveFindings(r.Context(), root.ID, result.Findings); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	result.RiskScore.AnalysisID = root.ID
	riskScore, err := s.store.SaveRiskScore(r.Context(), result.RiskScore)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	completed, err := s.store.CompleteAnalysis(r.Context(), root.ID, result.Summary)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	findings, err := s.store.FindingsByAnalysisID(r.Context(), root.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, buildURLAnalysisResponse(completed, createdTarget, metadata, findings, riskScore))
}

func (s *server) listAnalyses(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "Invalid authentication credentials"})
		return
	}
	projects, err := s.store.ProjectsByUser(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}

	items := make([]AnalysisListItem, 0)
	for _, project := range projects {
		analyses, err := s.store.ListAnalysesByProject(r.Context(), userID, project.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
			return
		}
		for _, analysis := range analyses {
			riskScore, err := s.store.RiskScoreByAnalysisID(r.Context(), analysis.ID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
				return
			}
			items = append(items, AnalysisListItem{
				AnalysisID: analysis.ID,
				ProjectID:  analysis.ProjectID,
				InputType:  analysis.InputType,
				Status:     analysis.Status,
				RiskLevel:  riskScore.Level,
				CreatedAt:  analysis.StartedAt,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	writeJSON(w, http.StatusOK, map[string]any{"analyses": items})
}

func (s *server) createFileAnalysis(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "Invalid authentication credentials"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAnalysisFileSize+1024*1024)
	if err := r.ParseMultipartForm(maxAnalysisFileSize + 1024); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid multipart request"})
		return
	}

	projectID, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("projectId")), 10, 64)
	if err != nil || projectID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Project id is required"})
		return
	}
	if inputType := strings.TrimSpace(r.FormValue("inputType")); inputType != string(db.InputTypeFile) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "inputType must be FILE"})
		return
	}
	if _, err := s.store.ProjectByID(r.Context(), userID, projectID); errors.Is(err, db.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Project not found"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "File is required"})
		return
	}
	defer file.Close()

	content, err := readLimitedFile(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	mimeType := http.DetectContentType(content)
	if !allowedAnalysisMimeType(mimeType) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Unsupported file type"})
		return
	}

	checksum := sha256.Sum256(content)
	checksumHex := hex.EncodeToString(checksum[:])
	originalFilename := safeOriginalFilename(header.Filename)
	extension := strings.ToLower(filepath.Ext(originalFilename))
	storedReference, err := storeAnalysisFile(s.cfg.StorageDir, checksumHex, extension, content)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}

	root, err := s.store.CreateAnalysis(r.Context(), db.Analysis{
		ProjectID: projectID,
		InputType: db.InputTypeFile,
		Status:    db.AnalysisStatusProcessing,
		Summary:   "File analysis is processing.",
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	createdFile, err := s.store.CreateFile(r.Context(), db.File{
		AnalysisID:       root.ID,
		OriginalFilename: originalFilename,
		StoredReference:  storedReference,
		MimeType:         mimeType,
		SizeBytes:        int64(len(content)),
		ChecksumSHA256:   checksumHex,
		Extension:        stringPtrOrNil(extension),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}

	result := analysis.AnalyzeFile(content, mimeType)
	metadata, err := s.store.SaveMetadata(r.Context(), fileMetadata(root.ID, createdFile, result.MetadataEntries))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	if err := s.store.SaveFindings(r.Context(), root.ID, result.Findings); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	cleanFile, err := s.createCleanFile(r.Context(), createdFile, originalFilename, extension, content, mimeType)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	result.RiskScore.AnalysisID = root.ID
	riskScore, err := s.store.SaveRiskScore(r.Context(), result.RiskScore)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	completed, err := s.store.CompleteAnalysis(r.Context(), root.ID, result.Summary)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	findings, err := s.store.FindingsByAnalysisID(r.Context(), root.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, buildFileAnalysisResponse(completed, createdFile, metadata, findings, riskScore, cleanFile))
}

func (s *server) getAnalysis(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "Invalid authentication credentials"})
		return
	}
	analysisID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || analysisID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid analysis id"})
		return
	}

	root, err := s.store.GetAnalysisByID(r.Context(), userID, analysisID)
	if errors.Is(err, db.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Analysis not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	switch root.InputType {
	case db.InputTypeFile:
		file, metadata, findings, riskScore, cleanFile, ok := s.loadFileAnalysisParts(w, r, root.ID)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, buildFileAnalysisResponse(root, file, metadata, findings, riskScore, cleanFile))
	case db.InputTypeURL:
		target, metadata, findings, riskScore, ok := s.loadURLAnalysisParts(w, r, root.ID)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, buildURLAnalysisResponse(root, target, metadata, findings, riskScore))
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
	}
}

func (s *server) loadFileAnalysisParts(w http.ResponseWriter, r *http.Request, analysisID int64) (db.File, db.Metadata, []db.Finding, db.RiskScore, *db.CleanFile, bool) {
	file, err := s.store.FileByAnalysisID(r.Context(), analysisID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return db.File{}, db.Metadata{}, nil, db.RiskScore{}, nil, false
	}
	metadata, err := s.store.MetadataByAnalysisID(r.Context(), analysisID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return db.File{}, db.Metadata{}, nil, db.RiskScore{}, nil, false
	}
	findings, err := s.store.FindingsByAnalysisID(r.Context(), analysisID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return db.File{}, db.Metadata{}, nil, db.RiskScore{}, nil, false
	}
	riskScore, err := s.store.RiskScoreByAnalysisID(r.Context(), analysisID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return db.File{}, db.Metadata{}, nil, db.RiskScore{}, nil, false
	}
	cleanFile, err := s.store.CleanFileByAnalysisID(r.Context(), analysisID)
	if errors.Is(err, db.ErrNotFound) {
		return file, metadata, findings, riskScore, nil, true
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return db.File{}, db.Metadata{}, nil, db.RiskScore{}, nil, false
	}
	return file, metadata, findings, riskScore, &cleanFile, true
}

func (s *server) loadURLAnalysisParts(w http.ResponseWriter, r *http.Request, analysisID int64) (db.URLTarget, db.Metadata, []db.Finding, db.RiskScore, bool) {
	target, err := s.store.URLTargetByAnalysisID(r.Context(), analysisID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return db.URLTarget{}, db.Metadata{}, nil, db.RiskScore{}, false
	}
	metadata, err := s.store.MetadataByAnalysisID(r.Context(), analysisID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return db.URLTarget{}, db.Metadata{}, nil, db.RiskScore{}, false
	}
	findings, err := s.store.FindingsByAnalysisID(r.Context(), analysisID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return db.URLTarget{}, db.Metadata{}, nil, db.RiskScore{}, false
	}
	riskScore, err := s.store.RiskScoreByAnalysisID(r.Context(), analysisID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return db.URLTarget{}, db.Metadata{}, nil, db.RiskScore{}, false
	}
	return target, metadata, findings, riskScore, true
}

// validateCreateAnalysisRequest supports the JSON contract while multipart handles real file upload.
func validateCreateAnalysisRequest(payload CreateAnalysisRequest) string {
	if payload.ProjectID <= 0 {
		return "Project id is required"
	}
	if payload.InputType != db.InputTypeFile && payload.InputType != db.InputTypeURL {
		return "inputType must be FILE or URL"
	}

	hasFile := payload.File != nil
	hasURL := payload.URL != nil
	if hasFile == hasURL {
		return "Provide exactly one analysis input"
	}
	if payload.InputType == db.InputTypeFile && !hasFile {
		return "File input is required for FILE analyses"
	}
	if payload.InputType == db.InputTypeURL && !hasURL {
		return "URL input is required for URL analyses"
	}
	if payload.InputType == db.InputTypeURL && strings.TrimSpace(payload.URL.OriginalURL) == "" {
		return "URL is required for URL analyses"
	}
	return ""
}

func readLimitedFile(file io.Reader) ([]byte, error) {
	content, err := io.ReadAll(io.LimitReader(file, maxAnalysisFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		return nil, errors.New("File is empty")
	}
	if len(content) > maxAnalysisFileSize {
		return nil, errors.New("File exceeds the 10MB limit")
	}
	return content, nil
}

func allowedAnalysisMimeType(mimeType string) bool {
	return mimeType == "application/pdf" || mimeType == "image/jpeg"
}

func safeOriginalFilename(filename string) string {
	filename = strings.ReplaceAll(filename, "\\", "/")
	filename = strings.TrimSpace(filepath.Base(filename))
	if filename == "." || filename == "/" || filename == "" {
		return "uploaded-file"
	}
	return filename
}

func storeAnalysisFile(storageDir string, checksum string, extension string, content []byte) (string, error) {
	if err := ensureStorageDir(storageDir); err != nil {
		return "", err
	}
	name, err := randomHex(16)
	if err != nil {
		return "", err
	}
	if extension != ".pdf" && extension != ".jpg" && extension != ".jpeg" {
		extension = ""
	}
	path := filepath.Join(storageDir, checksum+"-"+name+extension)
	if err := writeStoredFile(path, content); err != nil {
		return "", err
	}
	return path, nil
}

func (s *server) createCleanFile(ctx context.Context, originalFile db.File, originalFilename string, extension string, content []byte, mimeType string) (*db.CleanFile, error) {
	cleanResult, err := analysis.SanitizeFile(content, mimeType)
	if err != nil {
		return nil, err
	}
	checksum := sha256.Sum256(cleanResult.Content)
	checksumHex := hex.EncodeToString(checksum[:])
	storedReference, err := storeAnalysisFile(s.cfg.StorageDir, checksumHex, extension, cleanResult.Content)
	if err != nil {
		return saveFailedCleanFile(ctx, s.store, originalFile, originalFilename, mimeType, cleanResult.RemovedMetadataKeys)
	}
	cleanFile, err := s.store.SaveCleanFile(ctx, db.CleanFile{
		AnalysisID:          originalFile.AnalysisID,
		OriginalFileID:      originalFile.ID,
		StoredReference:     storedReference,
		Filename:            cleanFilename(originalFilename),
		MimeType:            mimeType,
		SizeBytes:           int64(len(cleanResult.Content)),
		ChecksumSHA256:      checksumHex,
		CleaningStatus:      db.CleaningStatusCompleted,
		RemovedMetadataKeys: cleanResult.RemovedMetadataKeys,
	})
	if err != nil {
		return nil, err
	}
	return &cleanFile, nil
}

func saveFailedCleanFile(ctx context.Context, store dataStore, originalFile db.File, originalFilename string, mimeType string, removedKeys []string) (*db.CleanFile, error) {
	cleanFile, err := store.SaveCleanFile(ctx, db.CleanFile{
		AnalysisID:          originalFile.AnalysisID,
		OriginalFileID:      originalFile.ID,
		StoredReference:     "",
		Filename:            cleanFilename(originalFilename),
		MimeType:            mimeType,
		SizeBytes:           0,
		ChecksumSHA256:      "",
		CleaningStatus:      db.CleaningStatusFailed,
		RemovedMetadataKeys: removedKeys,
	})
	if err != nil {
		return nil, err
	}
	return &cleanFile, nil
}

func cleanFilename(original string) string {
	extension := filepath.Ext(original)
	base := strings.TrimSuffix(original, extension)
	if strings.TrimSpace(base) == "" {
		base = "clean-file"
	}
	return base + "-clean" + extension
}

var ensureStorageDir = func(storageDir string) error {
	return mkdirAll(storageDir)
}

var writeStoredFile = func(path string, content []byte) error {
	return writeFile(path, content)
}

var mkdirAll = func(path string) error {
	return os.MkdirAll(path, 0o700)
}

var writeFile = func(path string, content []byte) error {
	return os.WriteFile(path, content, 0o600)
}

func randomHex(size int) (string, error) {
	token := make([]byte, size)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	return hex.EncodeToString(token), nil
}

func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
