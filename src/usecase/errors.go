package usecase

import (
	"strconv"

	"github.com/nexus-commerce/user-service/src/user"
)

var ErrPasswordMismatch = user.NewValidationError("passwords do not match")

type Field int

const (
	FieldFirstName Field = iota + 1
	FieldLastName
	FieldEmail
	FieldPassword
	FieldPasswordConfirmation
)

func (f Field) String() string {
	switch f {
	case FieldFirstName:
		return "FirstName"
	case FieldLastName:
		return "LastName"
	case FieldEmail:
		return "Email"
	case FieldPassword:
		return "Password"
	case FieldPasswordConfirmation:
		return "PasswordConfirmation"
	}
	return "Field(" + strconv.Itoa(int(f)) + ")"
}

type FieldError struct {
	Field Field
	Err   error
}

func (e *FieldError) Error() string { return e.Field.String() + ": " + e.Err.Error() }

func (e *FieldError) Unwrap() error { return e.Err }
