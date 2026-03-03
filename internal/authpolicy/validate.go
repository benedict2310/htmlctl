package authpolicy

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	MaxUsernameLength = 128
	MinBcryptCost     = 10
	MaxBcryptCost     = 14
)

func ValidateUsername(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("username is required")
	}
	if len(value) > MaxUsernameLength {
		return "", fmt.Errorf("username exceeds maximum length of %d characters", MaxUsernameLength)
	}
	if strings.ContainsAny(value, "{}#\"'\\") {
		return "", fmt.Errorf("username contains unsupported Caddyfile characters")
	}
	for _, r := range value {
		if r < 0x21 || r > 0x7e {
			return "", fmt.Errorf("username must contain printable ASCII characters only")
		}
		if r == ':' {
			return "", fmt.Errorf("username must not contain ':'")
		}
	}
	return value, nil
}

func ValidatePasswordHash(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("password hash is required")
	}
	cost, err := bcrypt.Cost([]byte(value))
	if err != nil {
		return "", fmt.Errorf("password hash must be valid bcrypt: %w", err)
	}
	if cost < MinBcryptCost || cost > MaxBcryptCost {
		return "", fmt.Errorf("password hash bcrypt cost must be between %d and %d", MinBcryptCost, MaxBcryptCost)
	}
	return value, nil
}
