package usecase

import (
	"context"
	"errors"
	"uuid"

	"github.com/nexus-commerce/user-service/src/user"
)

type confirmEmailChangeUsers interface {
	IDByEmailChangeToken(ctx context.Context, hash user.EmailChangeTokenHash) (uuid.UUID, error)
	Update(ctx context.Context, id uuid.UUID, updateFn func(*user.User) error) (*user.User, error)
}

type ConfirmEmailChange struct {
	users confirmEmailChangeUsers
}

func NewConfirmEmailChange(users confirmEmailChangeUsers) *ConfirmEmailChange {
	return &ConfirmEmailChange{
		users: users,
	}
}

func (c *ConfirmEmailChange) Execute(ctx context.Context, rawToken string) error {
	token, err := user.ParseEmailChangeToken(rawToken)
	if err != nil {
		return err
	}

	id, err := c.users.IDByEmailChangeToken(ctx, token.Hash())
	if errors.Is(err, user.ErrUserNotFound) {
		return user.ErrInvalidEmailChangeToken
	}
	if err != nil {
		return err
	}

	_, err = c.users.Update(ctx, id, func(usr *user.User) error {
		return usr.ConfirmEmailChange(token)
	})
	if errors.Is(err, user.ErrUserNotFound) {
		return user.ErrInvalidEmailChangeToken
	}
	return err
}
