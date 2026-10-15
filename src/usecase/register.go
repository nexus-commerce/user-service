package usecase

import (
	"context"

	"github.com/nexus-commerce/user-service/src/entity"
)

type registerUsers interface {
	Create(ctx context.Context, user *entity.User) error
}

type Register struct {
	users  registerUsers
	tokens tokenIssuer
}

func NewRegister(users registerUsers, tokens tokenIssuer) *Register {
	return &Register{
		users:  users,
		tokens: tokens,
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

func (r *Register) Execute(ctx context.Context, reg Registration) (string, error) {
	if reg.Password != reg.PasswordConfirmation {
		return "", ErrPasswordMismatch
	}

	user, err := entity.NewUser(reg.profile(), reg.Password)
	if err != nil {
		return "", err
	}

	err = r.users.Create(ctx, user)
	if err != nil {
		return "", err
	}

	return r.tokens.Issue(user.ID)
}
