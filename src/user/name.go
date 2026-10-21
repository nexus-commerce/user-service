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

func RestoreName(s string) Name { return Name{value: s} }

func (n Name) String() string { return n.value }

type FullName struct {
	firstName Name
	lastName  Name
}

func NewFullName(firstName, lastName Name) FullName {
	return FullName{firstName: firstName, lastName: lastName}
}

func (f FullName) FirstName() Name { return f.firstName }

func (f FullName) LastName() Name { return f.lastName }

func (f FullName) WithFirstName(first Name) FullName {
	return FullName{firstName: first, lastName: f.lastName}
}

func (f FullName) WithLastName(last Name) FullName {
	return FullName{firstName: f.firstName, lastName: last}
}
