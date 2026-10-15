package entity

import (
	"fmt"
	"net/mail"
	"unicode/utf8"
)

const maxEmailLength = 100

var (
	ErrEmptyEmail     = NewValidationError("must not be empty")
	ErrEmailTooLong   = NewValidationError(fmt.Sprintf("must be at most %d characters", maxEmailLength))
	ErrMalformedEmail = NewValidationError("must be a valid email address")
)

type Email struct {
	value string
}

func NewEmail(s string) (Email, error) {
	if s == "" {
		return Email{}, ErrEmptyEmail
	}
	if utf8.RuneCountInString(s) > maxEmailLength {
		return Email{}, ErrEmailTooLong
	}

	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		return Email{}, ErrMalformedEmail
	}
	return Email{value: s}, nil
}

func (e Email) String() string { return e.value }
