package auth

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// ValidatePasswordSetting checks trimmed length for bcrypt (max 72 bytes UTF-8).
func ValidatePasswordSetting(pw string) error {
	pw = strings.TrimSpace(pw)
	if len(pw) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	if len(pw) > 72 {
		return errors.New("password must be at most 72 characters")
	}
	return nil
}

const bcryptCost = bcrypt.DefaultCost

// HashPassword returns a bcrypt hash of plain (UTF-8). Plain must be at most 72 bytes for bcrypt.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// PasswordMatches compares bcrypt hash with plain text.
func PasswordMatches(hashed, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}
