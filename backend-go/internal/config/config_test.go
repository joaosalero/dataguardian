package config

import (
	"strings"
	"testing"
)

func TestLoadRejectsInvalidIntegerConfig(t *testing.T) {
	t.Setenv("ACCESS_TOKEN_EXPIRE_MINUTES", "soon")

	_, err := Load()

	if err == nil || !strings.Contains(err.Error(), "ACCESS_TOKEN_EXPIRE_MINUTES") {
		t.Fatalf("expected ACCESS_TOKEN_EXPIRE_MINUTES error, got %v", err)
	}
}

func TestLoadRejectsInvalidBackendAddress(t *testing.T) {
	t.Setenv("GO_BACKEND_ADDR", "localhost")

	_, err := Load()

	if err == nil || !strings.Contains(err.Error(), "GO_BACKEND_ADDR") {
		t.Fatalf("expected GO_BACKEND_ADDR error, got %v", err)
	}
}

func TestLoadRejectsSameSiteNoneOutsideProduction(t *testing.T) {
	t.Setenv("ENVIRONMENT", "dev")
	t.Setenv("COOKIE_SAMESITE", "none")

	_, err := Load()

	if err == nil || !strings.Contains(err.Error(), "COOKIE_SAMESITE") {
		t.Fatalf("expected COOKIE_SAMESITE error, got %v", err)
	}
}
