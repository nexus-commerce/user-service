package usecase

import (
	"context"
	"errors"

	"github.com/nexus-commerce/user-service/src/user"
)

type registerUsers interface {
	EmailTaken(ctx context.Context, email user.Email) (bool, error)
	RecordByEmail(ctx context.Context, email user.Email, recordFn func(*user.User)) error
}

type registerRegistrations interface {
	Save(ctx context.Context, p *user.PendingRegistration) error
}

type Register struct {
	users         registerUsers
	registrations registerRegistrations
	hasher        user.Hasher
}

func NewRegister(users registerUsers, registrations registerRegistrations, hasher user.Hasher) *Register {
	return &Register{
		users:         users,
		registrations: registrations,
		hasher:        hasher,
	}
}

type Registration struct {
	FirstName            string
	LastName             string
	Email                string
	Password             string
	PasswordConfirmation string
}

func (r *Register) Execute(ctx context.Context, reg Registration) error {
	if reg.Password != reg.PasswordConfirmation {
		return &FieldError{Field: FieldPasswordConfirmation, Err: ErrPasswordMismatch}
	}

	first, err := user.NewName(reg.FirstName)
	if err != nil {
		return &FieldError{Field: FieldFirstName, Err: err}
	}
	last, err := user.NewName(reg.LastName)
	if err != nil {
		return &FieldError{Field: FieldLastName, Err: err}
	}
	email, err := user.NewEmail(reg.Email)
	if err != nil {
		return &FieldError{Field: FieldEmail, Err: err}
	}
	password, err := user.NewPassword(reg.Password)
	if err != nil {
		return &FieldError{Field: FieldPassword, Err: err}
	}
	pending, err := user.RequestRegistration(email, user.NewFullName(first, last), password, r.hasher)
	if err != nil {
		return err
	}

	taken, err := r.users.EmailTaken(ctx, email)
	if err != nil {
		return err
	}
	if !taken {
		return r.registrations.Save(ctx, pending)
	}

	err = r.users.RecordByEmail(ctx, email, (*user.User).RecordDuplicateRegistration)
	if errors.Is(err, user.ErrNotFound) {
		return nil
	}
	return err
}
