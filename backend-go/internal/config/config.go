package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAddr                     = "127.0.0.1:8000"
	defaultDatabaseURL              = "postgresql://dataguardian:dataguardian@localhost:5434/dataguardian"
	defaultEnvironment              = "dev"
	defaultAccessTokenExpireMinutes = 30
	defaultAuthCookieName           = "dataguardian_session"
	defaultCookieSameSite           = "lax"
	defaultStorageDir               = "/tmp/dataguardian/uploads"
)

type Settings struct {
	Addr                     string
	DatabaseURL              string
	Environment              string
	AccessTokenExpireMinutes int
	AuthCookieName           string
	CookieSameSite           string
	StorageDir               string
	JWTPrivateKeyPEM         string
	JWTPublicKeyPEM          string
	FernetKey                string
	ReadTimeout              time.Duration
	WriteTimeout             time.Duration
}

func Load() (Settings, error) {
	cfg := Settings{
		Addr:                     env("GO_BACKEND_ADDR", defaultAddr),
		DatabaseURL:              env("DATABASE_URL", defaultDatabaseURL),
		Environment:              normalizeEnvironment(env("ENVIRONMENT", defaultEnvironment)),
		AccessTokenExpireMinutes: envInt("ACCESS_TOKEN_EXPIRE_MINUTES", defaultAccessTokenExpireMinutes),
		AuthCookieName:           env("AUTH_COOKIE_NAME", defaultAuthCookieName),
		CookieSameSite:           strings.ToLower(env("COOKIE_SAMESITE", defaultCookieSameSite)),
		StorageDir:               env("STORAGE_DIR", defaultStorageDir),
		JWTPrivateKeyPEM:         normalizePEM(os.Getenv("JWT_PRIVATE_KEY")),
		JWTPublicKeyPEM:          normalizePEM(os.Getenv("JWT_PUBLIC_KEY")),
		FernetKey:                strings.TrimSpace(os.Getenv("FERNET_KEY")),
		ReadTimeout:              10 * time.Second,
		WriteTimeout:             10 * time.Second,
	}

	if cfg.Environment == "prod" && strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		return Settings{}, errors.New("DATABASE_URL is required in production")
	}
	if cfg.Environment == "prod" && (cfg.JWTPrivateKeyPEM == "" || cfg.JWTPublicKeyPEM == "") {
		return Settings{}, errors.New("JWT_PRIVATE_KEY and JWT_PUBLIC_KEY are required in production")
	}
	if cfg.Environment == "prod" && cfg.FernetKey == "" {
		return Settings{}, errors.New("FERNET_KEY is required in production")
	}
	if cfg.JWTPrivateKeyPEM == "" || cfg.JWTPublicKeyPEM == "" {
		privatePEM, publicPEM, err := generateDevKeyPair()
		if err != nil {
			return Settings{}, err
		}
		cfg.JWTPrivateKeyPEM = privatePEM
		cfg.JWTPublicKeyPEM = publicPEM
	}

	return cfg, nil
}

func (s Settings) CookieSecure() bool {
	return s.Environment == "prod"
}

func (s Settings) IsDevOrTest() bool {
	return s.Environment == "dev" || s.Environment == "test"
}

func env(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func normalizeEnvironment(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "development", "dev", "local":
		return "dev"
	case "production", "prod":
		return "prod"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizePEM(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), `\n`, "\n")
}

func generateDevKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	privateBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", "", err
	}
	publicBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateBytes})
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicBytes})
	return string(privatePEM), string(publicPEM), nil
}
