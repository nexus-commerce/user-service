package usecase

import (
	"context"
	"errors"
	"uuid"

	"github.com/nexus-commerce/user-service/src/user"
)

type requestEmailChangeUsers interface {
	EmailTaken(ctx context.Context, email user.Email) (bool, error)
	UpdateByID(ctx context.Context, id uuid.UUID, updateFn func(*user.User) error) (*user.User, error)
}

type RequestEmailChange struct {
	users  requestEmailChangeUsers
	hasher user.Hasher
}

func NewRequestEmailChange(users requestEmailChangeUsers, hasher user.Hasher) *RequestEmailChange {
	return &RequestEmailChange{
		users:  users,
		hasher: hasher,
	}
}

type EmailChangeRequest struct {
	UserID   uuid.UUID
	NewEmail string
	Password string
}

func (r *RequestEmailChange) Execute(ctx context.Context, req EmailChangeRequest) error {
	email, err := user.NewEmail(req.NewEmail)
	if err != nil {
		return &FieldError{Field: FieldEmail, Err: err}
	}
	password, err := user.NewPassword(req.Password)
	if err != nil {
		return user.ErrInvalidCredentials
	}

	taken, err := r.users.EmailTaken(ctx, email)
	if err != nil {
		return err
	}

	_, err = r.users.UpdateByID(ctx, req.UserID, func(usr *user.User) error {
		if err := usr.RequestEmailChange(email, password, r.hasher); err != nil {
			if errors.Is(err, user.ErrEmailUnchanged) {
				return &FieldError{Field: FieldEmail, Err: err}
			}
			return err
		}
		if taken {
			return user.ErrEmailTaken
		}
		return nil
	})
	return err
}
