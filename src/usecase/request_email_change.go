package usecase

import (
	"context"
	"errors"
	"uuid"

	"github.com/nexus-commerce/user-service/src/user"
)

type requestEmailChangeUsers interface {
	ByEmail(ctx context.Context, email user.Email) (*user.User, error)
	Update(ctx context.Context, id uuid.UUID, updateFn func(*user.User) error) (*user.User, error)
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
		return &user.FieldError{Field: "newEmail", Err: err}
	}
	password, err := user.NewPassword(req.Password)
	if err != nil {
		return user.ErrInvalidCredentials
	}

	owner, err := r.users.ByEmail(ctx, email)
	switch {
	case err == nil && owner.ID() != req.UserID:
		return user.ErrEmailTaken
	case err != nil && !errors.Is(err, user.ErrUserNotFound):
		return err
	}

	_, err = r.users.Update(ctx, req.UserID, func(usr *user.User) error {
		err := usr.RequestEmailChange(email, password, r.hasher)
		if errors.Is(err, user.ErrEmailUnchanged) {
			return &user.FieldError{Field: "newEmail", Err: err}
		}
		return err
	})
	return err
}
