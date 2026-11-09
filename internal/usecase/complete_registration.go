package usecase

import (
	"context"
	"errors"
	"uuid"

	"github.com/nexus-commerce/user-service/internal/user"
)

type completeRegistrationRegistrations interface {
	ByUserID(ctx context.Context, id uuid.UUID) (*user.PendingRegistration, error)
	Remove(ctx context.Context, p *user.PendingRegistration) error
}

type completeRegistrationUsers interface {
	Add(ctx context.Context, u *user.User) error
}

type CompleteRegistration struct {
	registrations completeRegistrationRegistrations
	users         completeRegistrationUsers
}

func NewCompleteRegistration(registrations completeRegistrationRegistrations, users completeRegistrationUsers) *CompleteRegistration {
	return &CompleteRegistration{
		registrations: registrations,
		users:         users,
	}
}

func (c *CompleteRegistration) Execute(ctx context.Context, userID uuid.UUID) error {
	pending, err := c.registrations.ByUserID(ctx, userID)
	if errors.Is(err, user.ErrRegistrationNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	u, err := pending.Complete()
	if err != nil {
		return err
	}

	err = c.users.Add(ctx, u)
	if errors.Is(err, user.ErrEmailTaken) {
		return c.reject(ctx, pending)
	}
	if err != nil {
		return err
	}
	return c.registrations.Remove(ctx, pending)
}

func (c *CompleteRegistration) reject(ctx context.Context, pending *user.PendingRegistration) error {
	if err := pending.Reject(); err != nil {
		return err
	}
	if err := c.registrations.Remove(ctx, pending); err != nil {
		return err
	}
	return user.ErrEmailTaken
}
