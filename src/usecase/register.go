package usecase

import (
	"context"
	"fmt"

	"github.com/nexus-commerce/user-service/src/entity"
)

type registerUsers interface {
	CreateWithMessage(ctx context.Context, user *entity.User, msg Message) error
}

type registerEvents interface {
	UserRegistered(user *entity.User) (Message, error)
}

type Register struct {
	users  registerUsers
	events registerEvents
}

func NewRegister(users registerUsers, events registerEvents) *Register {
	return &Register{
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

	msg, err := r.events.UserRegistered(user)
	if err != nil {
		return nil, err
	}

	err = r.users.CreateWithMessage(ctx, user, msg)
	if err != nil {
		return nil, err
	}

	return user, nil
}
