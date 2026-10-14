package usecase

import (
	"errors"

	"github.com/nexus-commerce/user-service/src/entity"
)

var ErrValidation = entity.ErrValidation

var (
	ErrUserNotFound  = errors.New("user not found")
	ErrWrongPassword = errors.New("wrong password")
	ErrEmailTaken    = errors.New("user email already exists")
)

var (
	ErrPasswordMismatch = entity.NewValidationError("passwords do not match")
)
