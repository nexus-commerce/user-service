package usecase

import (
	"context"
	"errors"

	"github.com/nexus-commerce/user-service/src/user"
)

type confirmRegistrationRegistrations interface {
	ConvertToUser(ctx context.Context, hash user.ConfirmationTokenHash, confirmFn func(*user.PendingRegistration) (*user.User, error)) (*user.User, error)
}

type ConfirmRegistration struct {
	registrations confirmRegistrationRegistrations
	tokens        tokenIssuer
}

func NewConfirmRegistration(registrations confirmRegistrationRegistrations, tokens tokenIssuer) *ConfirmRegistration {
	return &ConfirmRegistration{
		registrations: registrations,
		tokens:        tokens,
	}
}

func (c *ConfirmRegistration) Execute(ctx context.Context, rawToken string) (string, error) {
	token, err := user.ParseConfirmationToken(rawToken)
	if err != nil {
		return "", err
	}

	usr, err := c.registrations.ConvertToUser(ctx, token.Hash(), func(p *user.PendingRegistration) (*user.User, error) {
		return p.Confirm(token)
	})
	if errors.Is(err, user.ErrNotFound) {
		return "", user.ErrInvalidConfirmationToken
	}
	if err != nil {
		return "", err
	}
	return c.tokens.Issue(usr.ID())
}
