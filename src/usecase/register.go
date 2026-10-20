package usecase

import (
	"context"

	"github.com/nexus-commerce/user-service/src/user"
)

type registerUsers interface {
	Add(ctx context.Context, user *user.User) error
}

type Register struct {
	users  registerUsers
	tokens tokenIssuer
	hasher user.Hasher
}

func NewRegister(users registerUsers, tokens tokenIssuer, hasher user.Hasher) *Register {
	return &Register{
		users:  users,
		tokens: tokens,
		hasher: hasher,
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
		return "", &user.FieldError{Field: "passwordConfirmation", Err: ErrPasswordMismatch}
	}

	profile, err := user.NewProfile(user.ProfileFields{
		Email:     reg.Email,
		FirstName: reg.FirstName,
		LastName:  reg.LastName,
	})
	if err != nil {
		return "", err
	}
	password, err := user.NewPassword(reg.Password)
	if err != nil {
		return "", &user.FieldError{Field: "password", Err: err}
	}
	user, err := user.NewUser(profile, password, r.hasher)
	if err != nil {
		return "", err
	}

	err = r.users.Add(ctx, user)
	if err != nil {
		return "", err
	}

	return r.tokens.Issue(user.ID())
}
