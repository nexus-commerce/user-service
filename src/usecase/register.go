package usecase

import (
	"context"
	"fmt"

	"github.com/nexus-commerce/user-service/src/entity"
)

type RegisterUsers interface {
	Create(ctx context.Context, user *entity.User) error
}

type Events interface {
	UserRegistered(ctx context.Context, user *entity.User) error
}

type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type Register struct {
	tx     Transactor
	users  RegisterUsers
	events Events
}

func NewRegister(tx Transactor, users RegisterUsers, events Events) *Register {
	return &Register{
		tx:     tx,
		users:  users,
		events: events,
	}
}

type Registration struct {
	FirstName            entity.Name
	LastName             entity.Name
	Email                entity.Email
	Password             entity.Password
	PasswordConfirmation entity.Password
}

func (r Registration) profile() entity.Profile {
	return entity.Profile{
		Email:     r.Email,
		FirstName: r.FirstName,
		LastName:  r.LastName,
	}
}

func (r Registration) validate() error {
	if err := r.profile().Validate(); err != nil {
		return err
	}

	if err := r.Password.Validate(); err != nil {
		return fmt.Errorf("password: %w", err)
	}

	if r.Password != r.PasswordConfirmation {
		return ErrPasswordMismatch
	}

	return nil
}

func (r *Register) Execute(ctx context.Context, reg Registration) (*entity.User, error) {
	err := reg.validate()
	if err != nil {
		return nil, err
	}

	user, err := entity.NewUser(reg.profile(), reg.Password)
	if err != nil {
		return nil, err
	}

	err = r.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := r.users.Create(ctx, user); err != nil {
			return err
		}
		if err := r.events.UserRegistered(ctx, user); err != nil {
			return fmt.Errorf("%w: %v", ErrPublishEvent, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return user, nil
}
