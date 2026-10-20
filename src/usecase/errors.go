package usecase

import "github.com/nexus-commerce/user-service/src/user"

var ErrPasswordMismatch = user.NewValidationError("passwords do not match")
