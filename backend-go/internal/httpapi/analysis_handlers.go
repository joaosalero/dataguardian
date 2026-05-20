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
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dataguardian/backend-go/internal/analysis"
	"dataguardian/backend-go/internal/db"
)

const maxAnalysisFileSize = 10 * 1024 * 1024
const maxInlinePreviewAssetSize = 1024 * 1024
const maxTextPreviewBytes = 4096
const defaultAnalysisPageSize = 10
const maxAnalysisPageSize = 50

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
		log.Print("URL analysis rejected by URL safety validation")
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid or unsafe URL"})
		return
	}
	if err != nil {
		log.Printf("URL analysis failed before persistence: %T", err)
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
	var createdFile *db.File
	var cleanFile *db.CleanFile
	var safePreview *AnalysisSafePreview
	if result.RemoteFile != nil {
		file, generatedCleanFile, generatedPreview, ok := s.persistRemoteFileInspection(w, r, root.ID, result.RemoteFile, &result)
		if !ok {
			return
		}
		createdFile = file
		cleanFile = generatedCleanFile
		safePreview = generatedPreview
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

	writeJSON(w, http.StatusCreated, buildURLAnalysisResponse(completed, createdTarget, metadata, findings, riskScore, createdFile, cleanFile, safePreview))
}

func (s *server) listAnalyses(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "Invalid authentication credentials"})
		return
	}
	filter, ok := analysisListFilterFromRequest(w, r)
	if !ok {
		return
	}
	rows, totalItems, err := s.store.ListAnalysesForUser(r.Context(), userID, db.AnalysisListFilter{
		Page:      filter.Page,
		PageSize:  filter.PageSize,
		InputType: filter.InputType,
		RiskLevel: filter.RiskLevel,
		Status:    filter.Status,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}

	items := make([]AnalysisListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, AnalysisListItem{
			AnalysisID: row.AnalysisID,
			ProjectID:  row.ProjectID,
			InputType:  row.InputType,
			Status:     row.Status,
			RiskLevel:  row.RiskLevel,
			CreatedAt:  row.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"analyses": items, "pagination": analysisPagination(totalItems, filter.Page, filter.PageSize)})
}

func (s *server) deleteAnalysis(w http.ResponseWriter, r *http.Request) {
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

	if _, err := s.store.GetAnalysisByID(r.Context(), userID, analysisID); errors.Is(err, db.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Analysis not found"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}

	paths, ok := s.analysisArtifactPaths(w, r, analysisID)
	if !ok {
		return
	}
	if err := s.store.DeleteAnalysis(r.Context(), userID, analysisID); errors.Is(err, db.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Analysis not found"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("analysis artifact cleanup skipped one file: %T", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) storageSummary(w http.ResponseWriter, _ *http.Request) {
	summary, err := storageSummary(s.cfg.StorageDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	summary.OrphanRetentionHours = s.cfg.StorageOrphanRetentionHours
	writeJSON(w, http.StatusOK, summary)
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
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
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
		log.Printf("clean file generation failed: %T", err)
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
		file, cleanFile, safePreview, ok := s.loadOptionalURLFileParts(w, r, root.ID)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, buildURLAnalysisResponse(root, target, metadata, findings, riskScore, file, cleanFile, safePreview))
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
	if root.InputType != db.InputTypeFile && root.InputType != db.InputTypeURL {
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
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Clean file not available"})
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
	if errors.Is(err, os.ErrNotExist) || (err == nil && stat.IsDir()) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Clean file not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}

	w.Header().Set("Content-Type", cleanFile.MimeType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": cleanFile.Filename}))
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, cleanFile.Filename, stat.ModTime(), file)
}

func (s *server) downloadOriginalFile(w http.ResponseWriter, r *http.Request) {
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
	if root.InputType != db.InputTypeFile && root.InputType != db.InputTypeURL {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Original file not available"})
		return
	}

	originalFile, err := s.store.FileByAnalysisID(r.Context(), analysisID)
	if errors.Is(err, db.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Original file not available"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}

	path, ok := storedFilePath(s.cfg.StorageDir, originalFile.StoredReference)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Original file not available"})
		return
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Original file not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if errors.Is(err, os.ErrNotExist) || (err == nil && stat.IsDir()) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Original file not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}

	w.Header().Set("Content-Type", originalFile.MimeType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": originalFile.OriginalFilename}))
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, originalFile.OriginalFilename, stat.ModTime(), file)
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

func (s *server) persistRemoteFileInspection(w http.ResponseWriter, r *http.Request, analysisID int64, remoteFile *analysis.RemoteFile, result *analysis.URLResult) (*db.File, *db.CleanFile, *AnalysisSafePreview, bool) {
	mimeType := remoteFile.MimeType
	if !allowedAnalysisMimeType(mimeType) {
		return nil, nil, unavailableSafePreview("Remote file type is not supported for safe preview."), true
	}
	checksum := sha256.Sum256(remoteFile.Content)
	checksumHex := hex.EncodeToString(checksum[:])
	originalFilename := safeOriginalFilename(remoteFile.Filename)
	extension := strings.ToLower(filepath.Ext(originalFilename))
	storedReference, err := storeAnalysisFile(s.cfg.StorageDir, checksumHex, extension, remoteFile.Content)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return nil, nil, nil, false
	}
	createdFile, err := s.store.CreateFile(r.Context(), db.File{
		AnalysisID:       analysisID,
		OriginalFilename: originalFilename,
		StoredReference:  storedReference,
		MimeType:         mimeType,
		SizeBytes:        int64(len(remoteFile.Content)),
		ChecksumSHA256:   checksumHex,
		Extension:        stringPtrOrNil(extension),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return nil, nil, nil, false
	}

	fileResult := analysis.AnalyzeFile(remoteFile.Content, mimeType)
	result.Findings = append(result.Findings, fileResult.Findings...)
	result.Metadata.Entries = append(result.Metadata.Entries, fileMetadata(analysisID, createdFile, fileResult.MetadataEntries).Entries...)
	result.RiskScore = analysis.ScoreFindings(result.Findings)
	result.Summary = "URL analysis completed with remote file inspection."
	if len(result.Findings) > 0 {
		result.Summary = "URL analysis completed with remote file inspection and structured findings."
	}

	cleanFile, err := s.createCleanFile(r.Context(), createdFile, originalFilename, extension, remoteFile.Content, mimeType)
	if err != nil {
		log.Printf("remote clean file generation failed: %T", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return nil, nil, nil, false
	}
	safePreview := safeFilePreview(remoteFile.Content, mimeType, originalFilename)
	return &createdFile, cleanFile, safePreview, true
}

func (s *server) loadOptionalURLFileParts(w http.ResponseWriter, r *http.Request, analysisID int64) (*db.File, *db.CleanFile, *AnalysisSafePreview, bool) {
	file, err := s.store.FileByAnalysisID(r.Context(), analysisID)
	if errors.Is(err, db.ErrNotFound) {
		return nil, nil, nil, true
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return nil, nil, nil, false
	}
	cleanFile, err := s.store.CleanFileByAnalysisID(r.Context(), analysisID)
	if errors.Is(err, db.ErrNotFound) {
		safePreview := s.safeFilePreviewFromStoredFile(file)
		return &file, nil, safePreview, true
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return nil, nil, nil, false
	}
	safePreview := s.safeFilePreviewFromStoredFile(file)
	return &file, &cleanFile, safePreview, true
}

type analysisListFilter struct {
	Page      int
	PageSize  int
	InputType *db.InputType
	RiskLevel *db.RiskLevel
	Status    *db.AnalysisStatus
}

func analysisListFilterFromRequest(w http.ResponseWriter, r *http.Request) (analysisListFilter, bool) {
	query := r.URL.Query()
	page, ok := queryInt(w, query.Get("page"), 1, "page")
	if !ok {
		return analysisListFilter{}, false
	}
	pageSize, ok := queryInt(w, query.Get("pageSize"), defaultAnalysisPageSize, "pageSize")
	if !ok {
		return analysisListFilter{}, false
	}
	filter := analysisListFilter{
		Page:     page,
		PageSize: pageSize,
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = defaultAnalysisPageSize
	}
	if filter.PageSize > maxAnalysisPageSize {
		filter.PageSize = maxAnalysisPageSize
	}
	if value := strings.ToUpper(strings.TrimSpace(query.Get("inputType"))); value != "" {
		inputType := db.InputType(value)
		if inputType != db.InputTypeFile && inputType != db.InputTypeURL {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid inputType filter"})
			return analysisListFilter{}, false
		}
		filter.InputType = &inputType
	}
	if value := strings.ToUpper(strings.TrimSpace(query.Get("riskLevel"))); value != "" {
		riskLevel := db.RiskLevel(value)
		if riskLevel != db.RiskLevelLow && riskLevel != db.RiskLevelMedium && riskLevel != db.RiskLevelHigh {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid riskLevel filter"})
			return analysisListFilter{}, false
		}
		filter.RiskLevel = &riskLevel
	}
	if value := strings.ToUpper(strings.TrimSpace(query.Get("status"))); value != "" {
		status := db.AnalysisStatus(value)
		if status != db.AnalysisStatusPending &&
			status != db.AnalysisStatusProcessing &&
			status != db.AnalysisStatusCompleted &&
			status != db.AnalysisStatusFailed {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid status filter"})
			return analysisListFilter{}, false
		}
		filter.Status = &status
	}
	return filter, true
}

func analysisPagination(totalItems int, page int, pageSize int) AnalysisPagination {
	totalPages := 0
	if totalItems > 0 {
		totalPages = (totalItems + pageSize - 1) / pageSize
	}
	return AnalysisPagination{
		Page:        page,
		PageSize:    pageSize,
		TotalItems:  totalItems,
		TotalPages:  totalPages,
		HasNext:     totalPages > 0 && page < totalPages,
		HasPrevious: page > 1 && totalPages > 0,
	}
}

func queryInt(w http.ResponseWriter, value string, fallback int, name string) (int, bool) {
	if strings.TrimSpace(value) == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid " + name + " parameter"})
		return 0, false
	}
	return parsed, true
}

func (s *server) analysisArtifactPaths(w http.ResponseWriter, r *http.Request, analysisID int64) ([]string, bool) {
	paths := []string{}
	seen := map[string]bool{}
	if file, err := s.store.FileByAnalysisID(r.Context(), analysisID); err == nil {
		path, ok := storedFilePath(s.cfg.StorageDir, file.StoredReference)
		if !ok {
			log.Print("analysis artifact cleanup skipped unsafe original file reference")
		} else if !seen[path] {
			paths = append(paths, path)
			seen[path] = true
		}
	} else if err != nil && !errors.Is(err, db.ErrNotFound) {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return nil, false
	}
	if cleanFile, err := s.store.CleanFileByAnalysisID(r.Context(), analysisID); err == nil && strings.TrimSpace(cleanFile.StoredReference) != "" {
		path, ok := storedFilePath(s.cfg.StorageDir, cleanFile.StoredReference)
		if !ok {
			log.Print("analysis artifact cleanup skipped unsafe sanitized file reference")
		} else if !seen[path] {
			paths = append(paths, path)
			seen[path] = true
		}
	} else if err != nil && !errors.Is(err, db.ErrNotFound) {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return nil, false
	}
	return paths, true
}

func storageSummary(storageDir string) (StorageSummaryResponse, error) {
	var summary StorageSummaryResponse
	root, err := filepath.Abs(storageDir)
	if err != nil {
		return summary, err
	}
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return summary, nil
	} else if err != nil {
		return summary, err
	}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return err
		}
		summary.FileCount++
		summary.TotalBytes += info.Size()
		return nil
	})
	return summary, err
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
	filename := checksum + "-" + name + extension
	path, ok := storedFilePathForWrite(storageDir, filename)
	if !ok {
		return "", errors.New("invalid storage path")
	}
	if err := writeStoredFile(storageDir, filename, content); err != nil {
		return "", err
	}
	return path, nil
}

func storedFilePathForWrite(storageDir string, filename string) (string, bool) {
	if strings.TrimSpace(filename) == "" || filepath.Base(filename) != filename {
		return "", false
	}
	storageRoot, err := filepath.Abs(storageDir)
	if err != nil {
		return "", false
	}
	cleanPath, err := filepath.Abs(filepath.Join(storageRoot, filename))
	if err != nil {
		return "", false
	}
	relative, err := filepath.Rel(storageRoot, cleanPath)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
		return "", false
	}
	return cleanPath, true
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
	stat, err := os.Stat(path)
	if err != nil {
		return unavailableSafePreview("Preview content is no longer available.")
	}
	if stat.IsDir() {
		return unavailableSafePreview("Preview is unavailable for this stored file.")
	}
	if stat.Size() > maxAnalysisFileSize {
		return unavailableSafePreview("Preview is unavailable because the stored file is larger than the preview read limit.")
	}
	stored, err := os.Open(path)
	if err != nil {
		return unavailableSafePreview("Preview content is no longer available.")
	}
	defer stored.Close()
	content, err := io.ReadAll(io.LimitReader(stored, maxAnalysisFileSize+1))
	if err != nil || len(content) > maxAnalysisFileSize {
		return unavailableSafePreview("Preview content could not be loaded safely.")
	}
	return safeFilePreview(content, file.MimeType, file.OriginalFilename)
}

func safeFilePreview(content []byte, mimeType string, filename string) *AnalysisSafePreview {
	switch {
	case mimeType == "image/jpeg" || mimeType == "image/png":
		if !validPreviewImage(content, mimeType) {
			return unavailableSafePreview("Image preview is unavailable because the file appears malformed.")
		}
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

func validPreviewImage(content []byte, mimeType string) bool {
	switch mimeType {
	case "image/jpeg":
		return len(content) >= 3 && content[0] == 0xff && content[1] == 0xd8 && content[2] == 0xff
	case "image/png":
		return len(content) >= 8 && string(content[:8]) == "\x89PNG\r\n\x1a\n"
	default:
		return false
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

var writeStoredFile = func(storageDir string, filename string, content []byte) error {
	path, ok := storedFilePathForWrite(storageDir, filename)
	if !ok {
		return errors.New("invalid storage path")
	}
	return os.WriteFile(path, content, 0o600)
}

var mkdirAll = func(path string) error {
	return os.MkdirAll(path, 0o700)
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
