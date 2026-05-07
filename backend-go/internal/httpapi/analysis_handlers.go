package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"html"
	"io"
	"mime"
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
const maxInlinePreviewAssetSize = 1024 * 1024
const maxTextPreviewBytes = 4096

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

	safePreview := safeFilePreview(content, mimeType, originalFilename)
	writeJSON(w, http.StatusCreated, buildFileAnalysisResponse(completed, createdFile, metadata, findings, riskScore, cleanFile, safePreview))
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
		safePreview := s.safeFilePreviewFromStoredFile(file)
		writeJSON(w, http.StatusOK, buildFileAnalysisResponse(root, file, metadata, findings, riskScore, cleanFile, safePreview))
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

func (s *server) downloadCleanFile(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Clean file not available"})
		return
	}

	cleanFile, err := s.store.CleanFileByAnalysisID(r.Context(), analysisID)
	if errors.Is(err, db.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Clean file not available"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	if cleanFile.CleaningStatus != db.CleaningStatusCompleted || strings.TrimSpace(cleanFile.StoredReference) == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Clean file not available"})
		return
	}

	path, ok := storedFilePath(s.cfg.StorageDir, cleanFile.StoredReference)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Clean file not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil || stat.IsDir() {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}

	w.Header().Set("Content-Type", cleanFile.MimeType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": cleanFile.Filename}))
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
	http.ServeContent(w, r, cleanFile.Filename, stat.ModTime(), file)
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
	return mimeType == "application/pdf" ||
		mimeType == "image/jpeg" ||
		mimeType == "image/png" ||
		strings.HasPrefix(mimeType, "text/plain")
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
	if extension != ".pdf" && extension != ".jpg" && extension != ".jpeg" && extension != ".png" && extension != ".txt" {
		extension = ""
	}
	path := filepath.Join(storageDir, checksum+"-"+name+extension)
	if err := writeStoredFile(path, content); err != nil {
		return "", err
	}
	return path, nil
}

func storedFilePath(storageDir string, storedReference string) (string, bool) {
	storageRoot, err := filepath.Abs(storageDir)
	if err != nil {
		return "", false
	}
	cleanPath, err := filepath.Abs(storedReference)
	if err != nil {
		return "", false
	}
	relative, err := filepath.Rel(storageRoot, cleanPath)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
		return "", false
	}
	return cleanPath, true
}

func (s *server) safeFilePreviewFromStoredFile(file db.File) *AnalysisSafePreview {
	path, ok := storedFilePath(s.cfg.StorageDir, file.StoredReference)
	if !ok {
		return unavailableSafePreview("Preview is unavailable for this stored file.")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return unavailableSafePreview("Preview content could not be loaded.")
	}
	return safeFilePreview(content, file.MimeType, file.OriginalFilename)
}

func safeFilePreview(content []byte, mimeType string, filename string) *AnalysisSafePreview {
	switch {
	case mimeType == "image/jpeg" || mimeType == "image/png":
		if len(content) > maxInlinePreviewAssetSize {
			return unavailableSafePreview("Image preview is unavailable because the file is larger than the inline preview limit.")
		}
		return &AnalysisSafePreview{
			Available: true,
			Kind:      "image",
			MimeType:  mimeType,
			DataURL:   "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(content),
		}
	case strings.HasPrefix(mimeType, "text/plain"):
		return &AnalysisSafePreview{
			Available: true,
			Kind:      "text",
			MimeType:  "text/plain; charset=utf-8",
			Text:      plainTextPreview(content),
		}
	case mimeType == "application/pdf":
		if !strings.HasPrefix(string(content[:min(len(content), 8)]), "%PDF") {
			return unavailableSafePreview("PDF preview is unavailable because the file is malformed.")
		}
		svg := pdfStaticPreviewSVG(filename, plainTextPreview(content))
		return &AnalysisSafePreview{
			Available: true,
			Kind:      "image",
			MimeType:  "image/svg+xml",
			DataURL:   "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg)),
		}
	default:
		return unavailableSafePreview("Safe preview is not supported for this file type.")
	}
}

func plainTextPreview(content []byte) string {
	if len(content) > maxTextPreviewBytes {
		content = content[:maxTextPreviewBytes]
	}
	var builder strings.Builder
	for _, r := range string(content) {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			builder.WriteRune(r)
		case r >= 32 && r != 127:
			builder.WriteRune(r)
		}
	}
	text := strings.TrimSpace(builder.String())
	if text == "" {
		return "No readable text was found in the safe preview."
	}
	return text
}

func pdfStaticPreviewSVG(filename string, text string) string {
	lines := previewLines(text, 12, 72)
	var body strings.Builder
	y := 116
	for _, line := range lines {
		body.WriteString(`<text x="44" y="`)
		body.WriteString(strconv.Itoa(y))
		body.WriteString(`" font-size="14" fill="#374151">`)
		body.WriteString(html.EscapeString(line))
		body.WriteString(`</text>`)
		y += 24
	}
	if len(lines) == 0 {
		body.WriteString(`<text x="44" y="116" font-size="14" fill="#6b7280">No readable first-page text was extracted.</text>`)
	}
	return `<svg xmlns="http://www.w3.org/2000/svg" width="640" height="840" viewBox="0 0 640 840">` +
		`<rect width="640" height="840" fill="#f3f4f6"/>` +
		`<rect x="28" y="28" width="584" height="784" rx="4" fill="#ffffff" stroke="#d1d5db"/>` +
		`<text x="44" y="70" font-size="20" font-weight="700" fill="#111827">PDF static safe preview</text>` +
		`<text x="44" y="94" font-size="12" fill="#6b7280">` + html.EscapeString(filename) + `</text>` +
		body.String() +
		`<text x="44" y="780" font-size="12" fill="#92400e">Active PDF content is not executed or embedded.</text>` +
		`</svg>`
}

func previewLines(text string, maxLines int, maxLen int) []string {
	lines := make([]string, 0, maxLines)
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if len(line) > maxLen {
			line = line[:maxLen] + "..."
		}
		lines = append(lines, line)
		if len(lines) == maxLines {
			break
		}
	}
	return lines
}

func unavailableSafePreview(message string) *AnalysisSafePreview {
	return &AnalysisSafePreview{
		Available: false,
		Kind:      "unavailable",
		Message:   message,
	}
}

func (s *server) createCleanFile(ctx context.Context, originalFile db.File, originalFilename string, extension string, content []byte, mimeType string) (*db.CleanFile, error) {
	cleanResult, err := analysis.SanitizeFile(content, mimeType)
	if errors.Is(err, analysis.ErrUnsupportedFileType) {
		return nil, nil
	}
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
