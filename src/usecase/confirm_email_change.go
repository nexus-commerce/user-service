package usecase

import (
	"context"
	"errors"

	"github.com/nexus-commerce/user-service/src/user"
)

type confirmEmailChangeUsers interface {
	UpdateByEmailChangeToken(ctx context.Context, hash user.ConfirmationTokenHash, updateFn func(*user.User) error) (*user.User, error)
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
	token, err := user.ParseConfirmationToken(rawToken)
	if err != nil {
		return err
	}

	_, err = c.users.UpdateByEmailChangeToken(ctx, token.Hash(), func(usr *user.User) error {
		return usr.ConfirmEmailChange(token)
	})
	if errors.Is(err, user.ErrNotFound) {
		return user.ErrInvalidConfirmationToken
	}
	return err
}
