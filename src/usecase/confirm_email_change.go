package usecase

import (
	"context"
	"errors"

	"github.com/nexus-commerce/user-service/src/user"
)

type confirmEmailChangeUsers interface {
	UpdateByEmailChangeToken(ctx context.Context, hash user.EmailChangeTokenHash, updateFn func(*user.User) error) (*user.User, error)
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

	_, err = c.users.UpdateByEmailChangeToken(ctx, token.Hash(), func(usr *user.User) error {
		return usr.ConfirmEmailChange(token)
	})
	if errors.Is(err, user.ErrUserNotFound) {
		return user.ErrInvalidEmailChangeToken
	}
	return err
}
