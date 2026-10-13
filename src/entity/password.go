package entity

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const maxPasswordBytes = 72

var (
	ErrEmptyPassword   = NewValidationError("must not be empty")
	ErrPasswordTooLong = NewValidationError(fmt.Sprintf("must be at most %d bytes", maxPasswordBytes))
)

type Password string

func (p Password) Validate() error {
	if p == "" {
		return ErrEmptyPassword
	}
	if len(p) > maxPasswordBytes {
		return ErrPasswordTooLong
	}
	return nil
}

func (p Password) Hash() (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func (p Password) Matches(hash string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(p))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword), errors.Is(err, bcrypt.ErrPasswordTooLong):
		return false, nil
	default:
		return false, fmt.Errorf("compare password hash: %w", err)
	}
}
