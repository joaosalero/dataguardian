package auth

import (
	"crypto/rsa"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func CreateAccessToken(userID int64, privateKeyPEM string, ttl time.Duration) (string, error) {
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(privateKeyPEM))
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub": strconvFormatInt(userID),
		"iat": now.Unix(),
		"exp": now.Add(ttl).Unix(),
	}

	return jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(privateKey)
}

func ParsePublicKey(publicKeyPEM string) (*rsa.PublicKey, error) {
	return jwt.ParseRSAPublicKeyFromPEM([]byte(publicKeyPEM))
}

func strconvFormatInt(value int64) string {
	return strconv.FormatInt(value, 10)
}
