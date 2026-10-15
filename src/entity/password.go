package entity

import (
	"errors"
	"fmt"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

const maxPasswordBytes = 72

var (
	ErrEmptyPassword   = NewValidationError("must not be empty")
	ErrPasswordTooLong = NewValidationError(fmt.Sprintf("must be at most %d bytes", maxPasswordBytes))
)

var dummyHash = sync.OnceValue(func() string {
	hash, err := bcrypt.GenerateFromPassword([]byte("dummy"), bcrypt.DefaultCost)
	if err != nil {
		panic(fmt.Sprintf("generate dummy password hash: %v", err))
	}
	return string(hash)
})

type Password struct {
	value string
}

func NewPassword(s string) (Password, error) {
	if s == "" {
		return Password{}, ErrEmptyPassword
	}
	if len(s) > maxPasswordBytes {
		return Password{}, ErrPasswordTooLong
	}
	return Password{value: s}, nil
}

func (p Password) String() string { return "[REDACTED]" }

func (p Password) Hash() (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(p.value), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func (p Password) Matches(hash string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(p.value))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword), errors.Is(err, bcrypt.ErrPasswordTooLong):
		return false, nil
	default:
		return false, fmt.Errorf("compare password hash: %w", err)
	}
}

func validPasswordHash(h string) bool {
	_, err := bcrypt.Cost([]byte(h))
	return err == nil
}

func SpendPasswordCheck() {
	_ = bcrypt.CompareHashAndPassword([]byte(dummyHash()), []byte("dummy"))
}
