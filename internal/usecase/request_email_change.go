package usecase

import (
	"context"
	"errors"
	"uuid"

	"github.com/nexus-commerce/user-service/internal/user"
)

type requestEmailChangeUsers interface {
	ByID(ctx context.Context, id uuid.UUID) (*user.User, error)
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

	usr, err := r.users.ByID(ctx, req.UserID)
	if err != nil {
		return err
	}
	proof, err := usr.Reauthenticate(password, r.hasher)
	if err != nil {
		return err
	}

	taken, err := r.users.EmailTaken(ctx, email)
	if err != nil {
		return err
	}

	_, err = r.users.UpdateByID(ctx, req.UserID, func(locked *user.User) error {
		if taken {
			return locked.AttemptEmailChangeToTakenAddress(email, proof)
		}
		return locked.RequestEmailChange(email, proof)
	})
	if errors.Is(err, user.ErrEmailUnchanged) {
		return &FieldError{Field: FieldEmail, Err: err}
	}
	return err
}
