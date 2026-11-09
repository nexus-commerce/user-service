package usecase

import (
	"context"
	"errors"

	"github.com/nexus-commerce/user-service/internal/user"
)

type requestRegistrationUsers interface {
	EmailTaken(ctx context.Context, email user.Email) (bool, error)
	UpdateByEmail(ctx context.Context, email user.Email, update func(*user.User) error) (*user.User, error)
}

type requestRegistrationRegistrations interface {
	Save(ctx context.Context, p *user.PendingRegistration, supersede func(existing *user.PendingRegistration) error) error
}

type RequestRegistration struct {
	users         requestRegistrationUsers
	registrations requestRegistrationRegistrations
	hasher        user.Hasher
}

func NewRequestRegistration(users requestRegistrationUsers, registrations requestRegistrationRegistrations, hasher user.Hasher) *RequestRegistration {
	return &RequestRegistration{
		users:         users,
		registrations: registrations,
		hasher:        hasher,
	}
}

type RegistrationRequest struct {
	FirstName            string
	LastName             string
	Email                string
	Password             string
	PasswordConfirmation string
}

func (r *RequestRegistration) Execute(ctx context.Context, req RegistrationRequest) error {
	if req.Password != req.PasswordConfirmation {
		return &FieldError{Field: FieldPasswordConfirmation, Err: ErrPasswordMismatch}
	}

	first, err := user.NewName(req.FirstName)
	if err != nil {
		return &FieldError{Field: FieldFirstName, Err: err}
	}
	last, err := user.NewName(req.LastName)
	if err != nil {
		return &FieldError{Field: FieldLastName, Err: err}
	}
	email, err := user.NewEmail(req.Email)
	if err != nil {
		return &FieldError{Field: FieldEmail, Err: err}
	}
	password, err := user.NewPassword(req.Password)
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
		err = r.registrations.Save(ctx, pending, func(existing *user.PendingRegistration) error {
			return existing.Supersede(pending)
		})
		if errors.Is(err, user.ErrRegistrationAlreadyConfirmed) {
			return nil
		}
		return err
	}

	_, err = r.users.UpdateByEmail(ctx, email, func(u *user.User) error {
		u.RecordDuplicateRegistration()
		return nil
	})
	if errors.Is(err, user.ErrNotFound) {
		return nil
	}
	return err
}
