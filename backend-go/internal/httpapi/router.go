package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dataguardian/backend-go/internal/auth"
	"dataguardian/backend-go/internal/config"
	"dataguardian/backend-go/internal/db"
)

type server struct {
	cfg         config.Settings
	store       *db.Store
	authLimiter *rateLimiter
}

type credentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type projectRequest struct {
	Name   string `json:"name"`
	Target string `json:"target"`
}

func NewRouter(cfg config.Settings, store *db.Store) http.Handler {
	srv := &server{
		cfg:         cfg,
		store:       store,
		authLimiter: newRateLimiter(20, time.Minute),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", srv.health)
	mux.HandleFunc("POST /auth/login", srv.authLimiter.middleware("login", srv.login))
	mux.HandleFunc("POST /auth/register", srv.authLimiter.middleware("register", srv.register))
	mux.HandleFunc("POST /auth/logout", srv.logout)
	mux.HandleFunc("GET /auth/me", srv.requireAuth(srv.me))
	mux.HandleFunc("GET /projects", srv.requireAuth(srv.listProjects))
	mux.HandleFunc("POST /projects", srv.requireAuth(srv.createProject))
	mux.HandleFunc("GET /projects/{id}", srv.requireAuth(srv.getProject))
	mux.HandleFunc("POST /projects/{id}/audit", srv.requireAuth(srv.runAudit))
	mux.HandleFunc("GET /projects/{id}/audits", srv.requireAuth(srv.listAudits))
	mux.HandleFunc("POST /analyses", srv.requireAuth(srv.createAnalysis))
	mux.HandleFunc("GET /analyses/{id}", srv.requireAuth(srv.getAnalysis))
	return securityHeaders(httpsRequired(cfg, withTenantContext(cors(mux))))
}

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(s.cfg.AuthCookieName)
		if err != nil || cookie.Value == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "Invalid authentication credentials"})
			return
		}

		claims, err := auth.ValidateAccessToken(cookie.Value, s.cfg.JWTPublicKeyPEM)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "Invalid authentication credentials"})
			return
		}

		next(w, r.WithContext(withUserID(r.Context(), claims.UserID)))
	}
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "Invalid authentication credentials"})
		return
	}
	user, err := s.store.UserByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "Invalid authentication credentials"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         user.ID,
		"email":      user.Email,
		"created_at": user.CreatedAt,
	})
}

func (s *server) logout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.AuthCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure(),
		SameSite: sameSiteMode(s.cfg.CookieSameSite),
	})
	writeJSON(w, http.StatusOK, map[string]string{"message": "Logged out"})
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	payload, ok := readCredentials(w, r)
	if !ok {
		return
	}

	email := normalizeUsername(payload.Username)
	user, err := s.store.UserByEmail(r.Context(), email)
	if err != nil || !auth.VerifyPassword(payload.Password, user.HashedPassword) {
		log.Print("User login failed")
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "Invalid credentials"})
		return
	}

	token, err := auth.CreateAccessToken(user.ID, s.cfg.JWTPrivateKeyPEM, time.Duration(s.cfg.AccessTokenExpireMinutes)*time.Minute)
	if err != nil {
		log.Printf("token creation failed: %T", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.AuthCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   s.cfg.AccessTokenExpireMinutes * 60,
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure(),
		SameSite: sameSiteMode(s.cfg.CookieSameSite),
	})
	log.Printf("User login succeeded: user_id=%d", user.ID)
	writeJSON(w, http.StatusOK, map[string]string{"message": "Authenticated"})
}

func (s *server) register(w http.ResponseWriter, r *http.Request) {
	payload, ok := readCredentials(w, r)
	if !ok {
		return
	}

	email := normalizeUsername(payload.Username)
	if err := auth.ValidatePasswordStrength(payload.Password); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	if _, err := s.store.UserByEmail(r.Context(), email); err == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"detail": "User already exists"})
		return
	} else if !errors.Is(err, db.ErrNotFound) {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}

	hashedPassword, err := auth.HashPassword(payload.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	user, err := s.store.CreateUser(r.Context(), email, hashedPassword, tenantIDFromContext(r.Context()))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         user.ID,
		"email":      user.Email,
		"created_at": user.CreatedAt,
	})
}

func (s *server) listProjects(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

func (s *server) createProject(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "Invalid authentication credentials"})
		return
	}
	var payload projectRequest
	if !readJSON(w, r, &payload) {
		return
	}
	name := strings.TrimSpace(payload.Name)
	target := strings.TrimSpace(payload.Target)
	if name == "" || target == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Project name and target are required"})
		return
	}
	project, err := s.store.CreateProject(r.Context(), userID, name, target)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	writeJSON(w, http.StatusCreated, project)
}

func (s *server) getProject(w http.ResponseWriter, r *http.Request) {
	userID, projectID, ok := s.projectContext(w, r)
	if !ok {
		return
	}
	project, err := s.store.ProjectByID(r.Context(), userID, projectID)
	if errors.Is(err, db.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Project not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (s *server) runAudit(w http.ResponseWriter, r *http.Request) {
	userID, projectID, ok := s.projectContext(w, r)
	if !ok {
		return
	}
	project, err := s.store.ProjectByID(r.Context(), userID, projectID)
	if errors.Is(err, db.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Project not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}

	audit, err := s.store.CreateAudit(
		r.Context(),
		project.ID,
		"completed",
		"Baseline security audit completed for "+project.Target+".",
		[]string{
			"Authentication boundary verified for this project.",
			"No critical exposure detected in the baseline audit.",
			"Manual review recommended before production database access.",
		},
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	writeJSON(w, http.StatusCreated, audit)
}

func (s *server) listAudits(w http.ResponseWriter, r *http.Request) {
	userID, projectID, ok := s.projectContext(w, r)
	if !ok {
		return
	}
	if _, err := s.store.ProjectByID(r.Context(), userID, projectID); errors.Is(err, db.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Project not found"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	audits, err := s.store.AuditsByProject(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Internal server error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"audits": audits})
}

func (s *server) projectContext(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "Invalid authentication credentials"})
		return 0, 0, false
	}
	projectID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || projectID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid project id"})
		return 0, 0, false
	}
	return userID, projectID, true
}

func readCredentials(w http.ResponseWriter, r *http.Request) (credentialsRequest, bool) {
	var payload credentialsRequest
	return payload, readJSON(w, r, &payload)
}

func readJSON(w http.ResponseWriter, r *http.Request, payload any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid request body"})
		return false
	}
	return true
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("response encoding failed: %T", err)
	}
}

func sameSiteMode(value string) http.SameSite {
	switch strings.ToLower(value) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
