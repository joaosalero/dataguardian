package httpapi

import (
	"net/http"
	"strings"

	"dataguardian/backend-go/internal/config"
)

func csrfProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if strings.EqualFold(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")), "cross-site") {
			writeJSON(w, http.StatusForbidden, map[string]string{"detail": "Cross-site request rejected"})
			return
		}
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" && origin != "http://localhost:3000" && origin != "http://127.0.0.1:3000" {
			writeJSON(w, http.StatusForbidden, map[string]string{"detail": "Untrusted request origin"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func httpsRequired(cfg config.Settings, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.Environment == "prod" && r.Header.Get("X-Forwarded-Proto") != "https" && r.TLS == nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"detail": "HTTPS is required"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://localhost:3000" || origin == "http://127.0.0.1:3000" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "content-type, authorization, x-tenant-id")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
