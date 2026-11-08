package user

import (
	"errors"
	"slices"
	"time"
	"uuid"
)

const registrationTTL = 24 * time.Hour

var (
	ErrRegistrationNotFound         = errors.New("registration not found")
	ErrRegistrationAlreadyConfirmed = errors.New("registration already confirmed")
	ErrRegistrationNotConfirmed     = errors.New("registration not confirmed")
	ErrRegistrationEmailMismatch    = errors.New("registration is for another email")
)

type PendingRegistration struct {
	userID       uuid.UUID
	email        Email
	fullName     FullName
	passwordHash PasswordHash
	confirmation Confirmation
	confirmedAt  *time.Time

	events []Event
}

func RequestRegistration(email Email, name FullName, password Password, hasher Hasher) (*PendingRegistration, error) {
	hash, err := hasher.Hash(password)
	if err != nil {
		return nil, err
	}
	p := &PendingRegistration{
		userID:       uuid.New(),
		email:        email,
		fullName:     name,
		passwordHash: hash,
	}
	p.issueConfirmationToken(time.Now().UTC())
	return p, nil
}

func RestorePendingRegistration(
	userID uuid.UUID,
	email Email,
	name FullName,
	passwordHash PasswordHash,
	tokenHash ConfirmationTokenHash,
	expiresAt time.Time,
	confirmedAt *time.Time,
) *PendingRegistration {
	return &PendingRegistration{
		userID:       userID,
		email:        email,
		fullName:     name,
		passwordHash: passwordHash,
		confirmation: Confirmation{tokenHash: tokenHash, expiresAt: expiresAt},
		confirmedAt:  confirmedAt,
	}
}

func (p *PendingRegistration) UserID() uuid.UUID { return p.userID }

func (p *PendingRegistration) Email() Email { return p.email }

func (p *PendingRegistration) FullName() FullName { return p.fullName }

func (p *PendingRegistration) PasswordHash() PasswordHash { return p.passwordHash }

func (p *PendingRegistration) TokenHash() ConfirmationTokenHash { return p.confirmation.tokenHash }

func (p *PendingRegistration) ExpiresAt() time.Time { return p.confirmation.expiresAt }

func (p *PendingRegistration) ConfirmedAt() (time.Time, bool) {
	if p.confirmedAt == nil {
		return time.Time{}, false
	}
	return *p.confirmedAt, true
}

func (p *PendingRegistration) Confirmed() bool { return p.confirmedAt != nil }

func (p *PendingRegistration) newEventMeta(at time.Time) EventMeta {
	return newEventMeta(p.userID, at)
}

func (p *PendingRegistration) Events() []Event {
	return slices.Clone(p.events)
}

func (p *PendingRegistration) Confirm(token ConfirmationToken) error {
	if p.Confirmed() {
		return ErrInvalidConfirmationToken
	}
	now := time.Now().UTC()
	if err := p.confirmation.verify(token, now); err != nil {
		return err
	}
	p.confirmedAt = &now
	p.events = append(p.events, RegistrationConfirmed{
		EventMeta: p.newEventMeta(now),
	})
	return nil
}

func (p *PendingRegistration) Supersede(newer *PendingRegistration) error {
	if p.Confirmed() {
		return ErrRegistrationAlreadyConfirmed
	}
	if newer.email != p.email {
		return ErrRegistrationEmailMismatch
	}
	p.fullName = newer.fullName
	p.passwordHash = newer.passwordHash
	p.issueConfirmationToken(time.Now().UTC())
	return nil
}

func (p *PendingRegistration) issueConfirmationToken(now time.Time) {
	confirmation, token := issueConfirmation(now, registrationTTL)
	p.confirmation = confirmation
	p.events = append(p.events, RegistrationRequested{
		EventMeta: p.newEventMeta(now),
		Email:     p.email,
		FullName:  p.fullName,
		Token:     token,
		ExpiresAt: confirmation.expiresAt,
	})
}

func (p *PendingRegistration) Reject() error {
	if !p.Confirmed() {
		return ErrRegistrationNotConfirmed
	}
	p.events = append(p.events, RegistrationRejected{
		EventMeta: p.newEventMeta(time.Now().UTC()),
		Email:     p.email,
	})
	return nil
}

func (p *PendingRegistration) Complete() (*User, error) {
	if !p.Confirmed() {
		return nil, ErrRegistrationNotConfirmed
	}
	u := &User{
		id:           p.userID,
		email:        p.email,
		fullName:     p.fullName,
		passwordHash: p.passwordHash,
		createdAt:    time.Now().UTC(),
	}
	u.events = append(u.events, Registered{
		EventMeta: u.newEventMeta(u.createdAt),
		Email:     u.email,
		FullName:  u.fullName,
	})
	return u, nil
}
