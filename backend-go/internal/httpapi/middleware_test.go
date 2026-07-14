package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFProtectionRejectsCrossSiteMutation(t *testing.T) {
	handler := csrfProtection(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected cross-site mutation rejection, got %d", rec.Code)
	}
}

func TestCSRFProtectionAllowsTrustedLocalOrigin(t *testing.T) {
	handler := csrfProtection(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected trusted origin to pass, got %d", rec.Code)
	}
}
