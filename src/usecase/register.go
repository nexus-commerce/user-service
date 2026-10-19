package usecase

import (
	"context"

	"github.com/nexus-commerce/user-service/src/entity"
)

type registerUsers interface {
	Add(ctx context.Context, user *entity.User) error
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
	FirstName            string
	LastName             string
	Email                string
	Password             string
	PasswordConfirmation string
}

func (r *Register) Execute(ctx context.Context, reg Registration) (string, error) {
	if reg.Password != reg.PasswordConfirmation {
		return "", &entity.FieldError{Field: "passwordConfirmation", Err: ErrPasswordMismatch}
	}

	profile, err := entity.NewProfile(entity.ProfileFields{
		Email:     reg.Email,
		FirstName: reg.FirstName,
		LastName:  reg.LastName,
	})
	if err != nil {
		return "", err
	}
	password, err := entity.NewPassword(reg.Password)
	if err != nil {
		return "", &entity.FieldError{Field: "password", Err: err}
	}
	user, err := entity.NewUser(profile, password)
	if err != nil {
		return "", err
	}

	err = r.users.Add(ctx, user)
	if err != nil {
		return "", err
	}

	return r.tokens.Issue(user.ID())
}
