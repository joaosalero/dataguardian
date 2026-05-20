package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

type argon2Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

var defaultParams = argon2Params{
	memory:      65536,
	iterations:  3,
	parallelism: 4,
	saltLength:  16,
	keyLength:   32,
}

func HashPassword(password string) (string, error) {
	salt := make([]byte, defaultParams.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, defaultParams.iterations, defaultParams.memory, defaultParams.parallelism, defaultParams.keyLength)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		defaultParams.memory,
		defaultParams.iterations,
		defaultParams.parallelism,
		b64Salt,
		b64Hash,
	), nil
}

func VerifyPassword(password string, encodedHash string) bool {
	params, salt, hash, err := decodeHash(encodedHash)
	if err != nil {
		return false
	}

	candidate := argon2.IDKey([]byte(password), salt, params.iterations, params.memory, params.parallelism, uint32(len(hash)))
	return subtle.ConstantTimeCompare(hash, candidate) == 1
}

func ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return errors.New("Password must be at least 8 characters long")
	}
	if strings.ToLower(password) == password || strings.ToUpper(password) == password {
		return errors.New("Password must include mixed case characters")
	}
	for _, char := range password {
		if char >= '0' && char <= '9' {
			return nil
		}
	}
	return errors.New("Password must include a number")
}

func decodeHash(encodedHash string) (argon2Params, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return argon2Params{}, nil, nil, errors.New("invalid argon2 hash")
	}
	if parts[1] != "argon2id" {
		return argon2Params{}, nil, nil, errors.New("unsupported argon2 variant")
	}

	var params argon2Params
	for _, param := range strings.Split(parts[3], ",") {
		keyValue := strings.SplitN(param, "=", 2)
		if len(keyValue) != 2 {
			return argon2Params{}, nil, nil, errors.New("invalid argon2 parameters")
		}
		switch keyValue[0] {
		case "m":
			value, err := parseUint32Param(keyValue[1])
			if err != nil {
				return argon2Params{}, nil, nil, err
			}
			params.memory = value
		case "t":
			value, err := parseUint32Param(keyValue[1])
			if err != nil {
				return argon2Params{}, nil, nil, err
			}
			params.iterations = value
		case "p":
			value, err := parseUint8Param(keyValue[1])
			if err != nil {
				return argon2Params{}, nil, nil, err
			}
			params.parallelism = value
		}
	}
	if params.memory == 0 || params.iterations == 0 || params.parallelism == 0 {
		return argon2Params{}, nil, nil, errors.New("invalid argon2 parameters")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argon2Params{}, nil, nil, err
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argon2Params{}, nil, nil, err
	}
	return params, salt, hash, nil
}

func parseUint32Param(value string) (uint32, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, err
	}
	if parsed == 0 || parsed > math.MaxUint32 {
		return 0, errors.New("argon2 parameter out of range")
	}
	return uint32(parsed), nil
}

func parseUint8Param(value string) (uint8, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, err
	}
	if parsed == 0 || parsed > math.MaxUint8 {
		return 0, errors.New("argon2 parameter out of range")
	}
	return uint8(parsed), nil
}
