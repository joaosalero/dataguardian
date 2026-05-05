package httpapi

import (
	"net/http"
	"strconv"

	"dataguardian/backend-go/internal/db"
)

func (s *server) createAnalysis(w http.ResponseWriter, r *http.Request) {
	var payload CreateAnalysisRequest
	if !readJSON(w, r, &payload) {
		return
	}
	if detail := validateCreateAnalysisRequest(payload); detail != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": detail})
		return
	}

	writeJSON(w, http.StatusNotImplemented, map[string]string{"detail": "Analysis pipeline is not implemented"})
}

func (s *server) getAnalysis(w http.ResponseWriter, r *http.Request) {
	analysisID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || analysisID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid analysis id"})
		return
	}

	writeJSON(w, http.StatusNotImplemented, map[string]string{"detail": "Analysis retrieval is not implemented"})
}

// validateCreateAnalysisRequest performs only the light contract checks for route skeletons.
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
