package entity

import "errors"

var ErrValidation = errors.New("validation failed")

type ValidationError struct{ msg string }

func NewValidationError(msg string) error {
	return &ValidationError{msg: msg}
}

func (e *ValidationError) Error() string { return e.msg }

func (e *ValidationError) Is(target error) bool { return target == ErrValidation }

type FieldError struct {
	Field string
	Err   error
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Err.Error() }

func (e *FieldError) Unwrap() error { return e.Err }
