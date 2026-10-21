package user

import (
	"slices"
	"time"
	"uuid"
)

const registrationTTL = 24 * time.Hour

type PendingRegistration struct {
	userID       uuid.UUID
	email        Email
	fullName     FullName
	passwordHash PasswordHash
	tokenHash    ConfirmationTokenHash
	expiresAt    time.Time

	events []Event
}

func RequestRegistration(email Email, name FullName, password Password, hasher Hasher) (*PendingRegistration, error) {
	hash, err := hasher.Hash(password)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	token := NewConfirmationToken()
	p := &PendingRegistration{
		userID:       uuid.New(),
		email:        email,
		fullName:     name,
		passwordHash: hash,
		tokenHash:    token.Hash(),
		expiresAt:    now.Add(registrationTTL),
	}
	p.events = append(p.events, RegistrationRequested{
		EventMeta: p.newEventMeta(now),
		Email:     p.email,
		FullName:  p.fullName,
		Token:     token,
		ExpiresAt: p.expiresAt,
	})
	return p, nil
}

func RestorePendingRegistration(userID uuid.UUID, email Email, name FullName, passwordHash PasswordHash, tokenHash ConfirmationTokenHash, expiresAt time.Time) *PendingRegistration {
	return &PendingRegistration{
		userID:       userID,
		email:        email,
		fullName:     name,
		passwordHash: passwordHash,
		tokenHash:    tokenHash,
		expiresAt:    expiresAt,
	}
}

func (p *PendingRegistration) UserID() uuid.UUID { return p.userID }

func (p *PendingRegistration) Email() Email { return p.email }

func (p *PendingRegistration) FullName() FullName { return p.fullName }

func (p *PendingRegistration) PasswordHash() PasswordHash { return p.passwordHash }

func (p *PendingRegistration) TokenHash() ConfirmationTokenHash { return p.tokenHash }

func (p *PendingRegistration) ExpiresAt() time.Time { return p.expiresAt }

func (p *PendingRegistration) newEventMeta(at time.Time) EventMeta {
	return newEventMeta(p.userID, at)
}

func (p *PendingRegistration) Events() []Event {
	return slices.Clone(p.events)
}

func (p *PendingRegistration) Confirm(token ConfirmationToken) (*User, error) {
	if token.Hash() != p.tokenHash {
		return nil, ErrInvalidConfirmationToken
	}
	now := time.Now().UTC()
	if now.After(p.expiresAt) {
		return nil, ErrConfirmationTokenExpired
	}
	u := &User{
		id:           p.userID,
		email:        p.email,
		fullName:     p.fullName,
		passwordHash: p.passwordHash,
		createdAt:    now,
	}
	u.events = append(u.events, Registered{
		EventMeta: u.newEventMeta(u.createdAt),
		Email:     u.email,
		FullName:  u.fullName,
	})
	return u, nil
}
