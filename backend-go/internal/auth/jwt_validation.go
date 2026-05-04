package auth

import (
	"crypto/rsa"
	"errors"
	"strconv"

	"github.com/golang-jwt/jwt/v5"
)

type TokenClaims struct {
	UserID int64
}

func ValidateAccessToken(tokenString string, publicKeyPEM string) (TokenClaims, error) {
	publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(publicKeyPEM))
	if err != nil {
		return TokenClaims{}, err
	}
	return ValidateAccessTokenWithKey(tokenString, publicKey)
}

func ValidateAccessTokenWithKey(tokenString string, publicKey *rsa.PublicKey) (TokenClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, errors.New("unexpected JWT signing method")
		}
		return publicKey, nil
	}, jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil {
		return TokenClaims{}, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return TokenClaims{}, errors.New("invalid token claims")
	}

	subject, err := claims.GetSubject()
	if err != nil || subject == "" {
		return TokenClaims{}, errors.New("missing token subject")
	}

	userID, err := strconv.ParseInt(subject, 10, 64)
	if err != nil || userID <= 0 {
		return TokenClaims{}, errors.New("invalid token subject")
	}
	if _, err := claims.GetIssuedAt(); err != nil {
		return TokenClaims{}, errors.New("missing issued-at claim")
	}
	if _, err := claims.GetExpirationTime(); err != nil {
		return TokenClaims{}, errors.New("missing expiration claim")
	}

	return TokenClaims{UserID: userID}, nil
}
