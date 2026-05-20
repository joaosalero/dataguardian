package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
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
	defaultStorageOrphanRetention   = 24
)

type Settings struct {
	Addr                        string
	DatabaseURL                 string
	Environment                 string
	AccessTokenExpireMinutes    int
	AuthCookieName              string
	CookieSameSite              string
	StorageDir                  string
	StorageOrphanRetentionHours int
	JWTPrivateKeyPEM            string
	JWTPublicKeyPEM             string
	FernetKey                   string
	ReadTimeout                 time.Duration
	WriteTimeout                time.Duration
}

func Load() (Settings, error) {
	accessTokenExpireMinutes, err := envInt("ACCESS_TOKEN_EXPIRE_MINUTES", defaultAccessTokenExpireMinutes)
	if err != nil {
		return Settings{}, err
	}
	storageOrphanRetentionHours, err := envInt("STORAGE_ORPHAN_RETENTION_HOURS", defaultStorageOrphanRetention)
	if err != nil {
		return Settings{}, err
	}
	cfg := Settings{
		Addr:                        env("GO_BACKEND_ADDR", defaultAddr),
		DatabaseURL:                 env("DATABASE_URL", defaultDatabaseURL),
		Environment:                 normalizeEnvironment(env("ENVIRONMENT", defaultEnvironment)),
		AccessTokenExpireMinutes:    accessTokenExpireMinutes,
		AuthCookieName:              env("AUTH_COOKIE_NAME", defaultAuthCookieName),
		CookieSameSite:              normalizeCookieSameSite(env("COOKIE_SAMESITE", defaultCookieSameSite)),
		StorageDir:                  env("STORAGE_DIR", defaultStorageDir),
		StorageOrphanRetentionHours: storageOrphanRetentionHours,
		JWTPrivateKeyPEM:            normalizePEM(os.Getenv("JWT_PRIVATE_KEY")),
		JWTPublicKeyPEM:             normalizePEM(os.Getenv("JWT_PUBLIC_KEY")),
		FernetKey:                   strings.TrimSpace(os.Getenv("FERNET_KEY")),
		ReadTimeout:                 10 * time.Second,
		WriteTimeout:                10 * time.Second,
	}

	if cfg.Environment == "prod" && strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		return Settings{}, errors.New("DATABASE_URL is required in production")
	}
	if cfg.Environment != "dev" && cfg.Environment != "test" && cfg.Environment != "prod" {
		return Settings{}, errors.New("ENVIRONMENT must be dev, test, or prod")
	}
	if _, _, err := net.SplitHostPort(cfg.Addr); err != nil {
		return Settings{}, errors.New("GO_BACKEND_ADDR must be in host:port format")
	}
	if strings.TrimSpace(cfg.AuthCookieName) == "" {
		return Settings{}, errors.New("AUTH_COOKIE_NAME is required")
	}
	if cfg.CookieSameSite == "none" && cfg.Environment != "prod" {
		return Settings{}, errors.New("COOKIE_SAMESITE=none requires ENVIRONMENT=prod")
	}
	if cfg.Environment == "prod" && (cfg.JWTPrivateKeyPEM == "" || cfg.JWTPublicKeyPEM == "") {
		return Settings{}, errors.New("JWT_PRIVATE_KEY and JWT_PUBLIC_KEY are required in production")
	}
	if cfg.Environment == "prod" && cfg.FernetKey == "" {
		return Settings{}, errors.New("FERNET_KEY is required in production")
	}
	if cfg.AccessTokenExpireMinutes <= 0 {
		return Settings{}, errors.New("ACCESS_TOKEN_EXPIRE_MINUTES must be greater than zero")
	}
	if cfg.StorageOrphanRetentionHours < 0 {
		return Settings{}, errors.New("STORAGE_ORPHAN_RETENTION_HOURS must be zero or greater")
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

func (s Settings) StorageOrphanRetention() time.Duration {
	return time.Duration(s.StorageOrphanRetentionHours) * time.Hour
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

func envInt(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, errors.New(name + " must be an integer")
	}
	return parsed, nil
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

func normalizeCookieSameSite(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return "strict"
	case "none":
		return "none"
	default:
		return "lax"
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
