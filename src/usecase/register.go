package usecase

import (
	"context"

	"github.com/nexus-commerce/user-service/src/user"
)

type registerUsers interface {
	Add(ctx context.Context, usr *user.User) error
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
		return "", &FieldError{Field: FieldPasswordConfirmation, Err: ErrPasswordMismatch}
	}

	first, err := user.NewName(reg.FirstName)
	if err != nil {
		return "", &FieldError{Field: FieldFirstName, Err: err}
	}
	last, err := user.NewName(reg.LastName)
	if err != nil {
		return "", &FieldError{Field: FieldLastName, Err: err}
	}
	email, err := user.NewEmail(reg.Email)
	if err != nil {
		return "", &FieldError{Field: FieldEmail, Err: err}
	}
	password, err := user.NewPassword(reg.Password)
	if err != nil {
		return "", &FieldError{Field: FieldPassword, Err: err}
	}
	usr, err := user.NewUser(email, user.NewFullName(first, last), password, r.hasher)
	if err != nil {
		return "", err
	}

	err = r.users.Add(ctx, usr)
	if err != nil {
		return "", err
	}

	return r.tokens.Issue(usr.ID())
}
