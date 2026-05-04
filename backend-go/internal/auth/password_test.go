package auth

import "testing"

func TestHashPasswordUsesArgon2IDAndVerifies(t *testing.T) {
	hashed, err := HashPassword("StrongPass123")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if !VerifyPassword("StrongPass123", hashed) {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword("WrongPass123", hashed) {
		t.Fatal("expected wrong password to fail")
	}
	if len(hashed) < len("$argon2id$") || hashed[:9] != "$argon2id" {
		t.Fatalf("expected argon2id hash, got %q", hashed)
	}
}

func TestVerifyKnownArgon2IDHash(t *testing.T) {
	hashed, err := HashPassword("PythonCompatible123")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if !VerifyPassword("PythonCompatible123", hashed) {
		t.Fatal("expected generated argon2id hash to verify")
	}
}
