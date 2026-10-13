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

type Email string

func (e Email) Validate() error {
	if e == "" {
		return ErrEmptyEmail
	}
	if utf8.RuneCountInString(string(e)) > maxEmailLength {
		return ErrEmailTooLong
	}

	addr, err := mail.ParseAddress(string(e))
	if err != nil || addr.Address != string(e) {
		return ErrMalformedEmail
	}
	return nil
}
