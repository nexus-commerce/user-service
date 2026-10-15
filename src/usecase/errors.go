package usecase

import (
	"errors"

	"github.com/nexus-commerce/user-service/src/entity"
)

var (
	ErrWrongPassword    = errors.New("wrong password")
	ErrPasswordMismatch = entity.NewValidationError("passwords do not match")
)
