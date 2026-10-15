package usecase

import "github.com/nexus-commerce/user-service/src/entity"

var ErrPasswordMismatch = entity.NewValidationError("passwords do not match")
