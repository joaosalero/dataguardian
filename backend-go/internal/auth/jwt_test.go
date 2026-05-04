package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestCreateAccessTokenUsesRS256Claims(t *testing.T) {
	privatePEM, publicPEM := generateTestKeyPair(t)

	tokenString, err := CreateAccessToken(123, privatePEM, 30*time.Minute)
	if err != nil {
		t.Fatalf("CreateAccessToken returned error: %v", err)
	}

	publicKey, err := ParsePublicKey(publicPEM)
	if err != nil {
		t.Fatalf("ParsePublicKey returned error: %v", err)
	}
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			t.Fatalf("unexpected signing method: %s", token.Method.Alg())
		}
		return publicKey, nil
	})
	if err != nil {
		t.Fatalf("jwt parse failed: %v", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		t.Fatal("expected valid map claims")
	}
	if claims["sub"] != "123" {
		t.Fatalf("expected sub claim to be 123, got %#v", claims["sub"])
	}
	if _, ok := claims["iat"]; !ok {
		t.Fatal("expected iat claim")
	}
	if _, ok := claims["exp"]; !ok {
		t.Fatal("expected exp claim")
	}
}

func TestValidateAccessTokenRejectsInvalidSignature(t *testing.T) {
	privatePEM, _ := generateTestKeyPair(t)
	_, wrongPublicPEM := generateTestKeyPair(t)

	tokenString, err := CreateAccessToken(123, privatePEM, time.Minute)
	if err != nil {
		t.Fatalf("CreateAccessToken returned error: %v", err)
	}

	if _, err := ValidateAccessToken(tokenString, wrongPublicPEM); err == nil {
		t.Fatal("expected invalid signature to be rejected")
	}
}

func TestValidateAccessTokenRejectsExpiredToken(t *testing.T) {
	privatePEM, publicPEM := generateTestKeyPair(t)

	tokenString, err := CreateAccessToken(123, privatePEM, -time.Minute)
	if err != nil {
		t.Fatalf("CreateAccessToken returned error: %v", err)
	}

	if _, err := ValidateAccessToken(tokenString, publicPEM); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestValidateAccessTokenRejectsMissingSubject(t *testing.T) {
	privatePEM, publicPEM := generateTestKeyPair(t)
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(privatePEM))
	if err != nil {
		t.Fatalf("ParseRSAPrivateKeyFromPEM returned error: %v", err)
	}

	now := time.Now().UTC()
	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": now.Unix(),
		"exp": now.Add(time.Minute).Unix(),
	}).SignedString(privateKey)
	if err != nil {
		t.Fatalf("SignedString returned error: %v", err)
	}

	if _, err := ValidateAccessToken(tokenString, publicPEM); err == nil {
		t.Fatal("expected missing subject to be rejected")
	}
}

func generateTestKeyPair(t *testing.T) (string, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}

	privateBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey returned error: %v", err)
	}
	publicBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey returned error: %v", err)
	}

	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateBytes})
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicBytes})

	return string(privatePEM), string(publicPEM)
}
