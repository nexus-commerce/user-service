package user

import (
	"errors"
	"slices"
	"time"
	"uuid"
)

var (
	ErrNotFound           = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailTaken         = errors.New("user email already exists")
	ErrEmailUnchanged     = NewValidationError("must differ from the current email")
)

type User struct {
	id           uuid.UUID
	email        Email
	fullName     FullName
	passwordHash PasswordHash
	createdAt    time.Time
	pending      *PendingEmailChange

	events []Event
}

func Restore(id uuid.UUID, email Email, name FullName, passwordHash PasswordHash, createdAt time.Time, pending *PendingEmailChange) *User {
	return &User{
		id:           id,
		email:        email,
		fullName:     name,
		passwordHash: passwordHash,
		createdAt:    createdAt,
		pending:      pending,
	}
}

func (u *User) ID() uuid.UUID { return u.id }

func (u *User) Email() Email { return u.email }

func (u *User) FullName() FullName { return u.fullName }

func (u *User) PasswordHash() PasswordHash { return u.passwordHash }

func (u *User) CreatedAt() time.Time { return u.createdAt }

func (u *User) PendingEmailChange() (PendingEmailChange, bool) {
	if u.pending == nil {
		return PendingEmailChange{}, false
	}
	return *u.pending, true
}

func (u *User) Rename(name FullName) {
	if name == u.fullName {
		return
	}
	u.fullName = name
	u.events = append(u.events, Renamed{
		EventMeta: u.newEventMeta(time.Now().UTC()),
		FullName:  name,
	})
}

func (u *User) RecordDuplicateRegistration() {
	u.events = append(u.events, DuplicateRegistrationAttempted{
		EventMeta: u.newEventMeta(time.Now().UTC()),
		Email:     u.email,
	})
}

func (u *User) RequestEmailChange(newEmail Email, password Password, hasher Hasher) error {
	if err := u.authorizeEmailChange(newEmail, password, hasher); err != nil {
		return err
	}
	now := time.Now().UTC()
	token := NewConfirmationToken()
	u.pending = &PendingEmailChange{
		email:     newEmail,
		tokenHash: token.Hash(),
		expiresAt: now.Add(emailChangeTokenTTL),
	}
	u.events = append(u.events, EmailChangeRequested{
		EventMeta: u.newEventMeta(now),
		NewEmail:  newEmail,
		Token:     token,
		ExpiresAt: u.pending.expiresAt,
	})
	return nil
}

func (u *User) AttemptEmailChangeToTakenAddress(newEmail Email, password Password, hasher Hasher) error {
	if err := u.authorizeEmailChange(newEmail, password, hasher); err != nil {
		return err
	}
	u.events = append(u.events, EmailChangeToTakenAddressAttempted{
		EventMeta: u.newEventMeta(time.Now().UTC()),
		NewEmail:  newEmail,
	})
	return nil
}

func (u *User) authorizeEmailChange(newEmail Email, password Password, hasher Hasher) error {
	if err := u.Authenticate(password, hasher); err != nil {
		return err
	}
	if newEmail == u.email {
		return ErrEmailUnchanged
	}
	return nil
}

func (u *User) ConfirmEmailChange(token ConfirmationToken) error {
	if u.pending == nil || token.Hash() != u.pending.tokenHash {
		return ErrInvalidConfirmationToken
	}
	now := time.Now().UTC()
	if now.After(u.pending.expiresAt) {
		return ErrConfirmationTokenExpired
	}
	old := u.email
	u.email = u.pending.email
	u.pending = nil
	u.events = append(u.events, EmailChanged{
		EventMeta: u.newEventMeta(now),
		OldEmail:  old,
		NewEmail:  u.email,
	})
	return nil
}

func (u *User) Authenticate(password Password, hasher Hasher) error {
	ok, err := hasher.Verify(u.passwordHash, password)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidCredentials
	}
	return nil
}

func (u *User) newEventMeta(at time.Time) EventMeta {
	return newEventMeta(u.id, at)
}

func (u *User) Events() []Event {
	return slices.Clone(u.events)
}
