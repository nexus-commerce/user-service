package entity

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const maxNameLength = 50

var (
	ErrEmptyName   = NewValidationError("must not be empty")
	ErrNameTooLong = NewValidationError(fmt.Sprintf("must be at most %d characters", maxNameLength))
)

type Name string

func (n Name) Validate() error {
	if strings.TrimSpace(string(n)) == "" {
		return ErrEmptyName
	}
	if utf8.RuneCountInString(string(n)) > maxNameLength {
		return ErrNameTooLong
	}
	return nil
}
