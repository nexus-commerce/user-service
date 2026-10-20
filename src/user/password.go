package user

import (
	"errors"
	"fmt"
)

const maxPasswordBytes = 72

var (
	ErrEmptyPassword         = NewValidationError("must not be empty")
	ErrPasswordTooLong       = NewValidationError(fmt.Sprintf("must be at most %d bytes", maxPasswordBytes))
	ErrMalformedPasswordHash = errors.New("malformed password hash")
)

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

func (Password) String() string { return "[REDACTED]" }

func (p Password) Plaintext() string { return p.value }

type PasswordHash struct {
	value string
}

func NewPasswordHash(s string) (PasswordHash, error) {
	if s == "" {
		return PasswordHash{}, ErrMalformedPasswordHash
	}
	return PasswordHash{value: s}, nil
}

func (PasswordHash) String() string { return "[REDACTED]" }

func (h PasswordHash) Encoded() string { return h.value }

type Hasher interface {
	Hash(Password) (PasswordHash, error)
	Verify(PasswordHash, Password) (bool, error)
}
