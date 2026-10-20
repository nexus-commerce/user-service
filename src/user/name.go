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

type FullName struct {
	firstName Name
	lastName  Name
}

type FullNameFields struct {
	FirstName string
	LastName  string
}

func NewFullName(firstName, lastName Name) FullName {
	return FullName{firstName: firstName, lastName: lastName}
}

func RestoreFullName(f FullNameFields) FullName {
	return FullName{
		firstName: Name{value: f.FirstName},
		lastName:  Name{value: f.LastName},
	}
}

func (n FullName) FirstName() Name { return n.firstName }

func (n FullName) LastName() Name { return n.lastName }

func (n FullName) WithFirstName(first Name) FullName {
	return FullName{firstName: first, lastName: n.lastName}
}

func (n FullName) WithLastName(last Name) FullName {
	return FullName{firstName: n.firstName, lastName: last}
}
