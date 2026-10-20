package user

import "errors"

var ErrValidation = errors.New("validation failed")

type ValidationError struct{ msg string }

func NewValidationError(msg string) error {
	return &ValidationError{msg: msg}
}

func (e *ValidationError) Error() string { return e.msg }

func (e *ValidationError) Is(target error) bool { return target == ErrValidation }

type Field int

const (
	FieldFirstName Field = iota + 1
	FieldLastName
	FieldEmail
	FieldPassword
)

func (f Field) String() string {
	switch f {
	case FieldFirstName:
		return "first name"
	case FieldLastName:
		return "last name"
	case FieldEmail:
		return "email"
	case FieldPassword:
		return "password"
	}
	return "unknown field"
}

type FieldError struct {
	Field Field
	Err   error
}

func (e *FieldError) Error() string { return e.Field.String() + ": " + e.Err.Error() }

func (e *FieldError) Unwrap() error { return e.Err }
