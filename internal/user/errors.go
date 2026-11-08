package user

import "errors"

var ErrValidation = errors.New("validation failed")

type ValidationError struct{ msg string }

func NewValidationError(msg string) error {
	return &ValidationError{msg: msg}
}

func (e *ValidationError) Error() string { return e.msg }

func (e *ValidationError) Is(target error) bool { return target == ErrValidation }
