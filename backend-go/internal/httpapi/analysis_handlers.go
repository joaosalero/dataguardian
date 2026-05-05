package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dataguardian/backend-go/internal/analysis"
	"dataguardian/backend-go/internal/db"
)

const maxAnalysisFileSize = 10 * 1024 * 1024

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
	writeJSON(w, http.StatusNotImplemented, map[string]string{"detail": "Only FILE multipart analysis is implemented"})
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
	metadata, err := s.store.SaveMetadata(r.Context(), minimalFileMetadata(root.ID, createdFile))
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

	writeJSON(w, http.StatusCreated, buildFileAnalysisResponse(completed, createdFile, metadata, findings, riskScore))
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
	if root.InputType != db.InputTypeFile {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"detail": "Only FILE analysis retrieval is implemented"})
		return
	}

	file, metadata, findings, riskScore, ok := s.loadFileAnalysisParts(w, r, root.ID)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, buildFileAnalysisResponse(root, file, metadata, findings, riskScore))
}

func (s *server) loadFileAnalysisParts(w http.ResponseWriter, r *http.Request, analysisID int64) (db.File, db.Metadata, []db.Finding, db.RiskScore, bool) {
	file, err := s.store.FileByAnalysisID(r.Context(), analysisID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return db.File{}, db.Metadata{}, nil, db.RiskScore{}, false
	}
	metadata, err := s.store.MetadataByAnalysisID(r.Context(), analysisID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return db.File{}, db.Metadata{}, nil, db.RiskScore{}, false
	}
	findings, err := s.store.FindingsByAnalysisID(r.Context(), analysisID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return db.File{}, db.Metadata{}, nil, db.RiskScore{}, false
	}
	riskScore, err := s.store.RiskScoreByAnalysisID(r.Context(), analysisID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return db.File{}, db.Metadata{}, nil, db.RiskScore{}, false
	}
	return file, metadata, findings, riskScore, true
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
