package user

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

type Name struct {
	value string
}

func NewName(s string) (Name, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Name{}, ErrEmptyName
	}
	if utf8.RuneCountInString(s) > maxNameLength {
		return Name{}, ErrNameTooLong
	}
	return Name{value: s}, nil
}

func (n Name) String() string { return n.value }
