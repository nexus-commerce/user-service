package user

import (
	"errors"
	"slices"
	"time"
	"uuid"
)

var (
	ErrUserNotFound       = errors.New("user not found")
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

func NewUser(email Email, name FullName, password Password, hasher Hasher) (*User, error) {
	hash, err := hasher.Hash(password)
	if err != nil {
		return nil, err
	}
	u := &User{
		id:           uuid.New(),
		email:        email,
		fullName:     name,
		passwordHash: hash,
		createdAt:    time.Now().UTC(),
	}
	u.events = append(u.events, Registered{
		EventMeta: u.newEventMeta(u.createdAt),
		Email:     u.email,
		FullName:  u.fullName,
	})
	return u, nil
}

func RestoreUser(id uuid.UUID, email Email, name FullName, passwordHash PasswordHash, createdAt time.Time, pending *PendingEmailChange) *User {
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

func (u *User) RequestEmailChange(newEmail Email, password Password, hasher Hasher) error {
	if err := u.Authenticate(password, hasher); err != nil {
		return err
	}
	if newEmail == u.email {
		return ErrEmailUnchanged
	}
	now := time.Now().UTC()
	token := NewEmailChangeToken()
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

func (u *User) ConfirmEmailChange(token EmailChangeToken) error {
	if u.pending == nil || token.Hash() != u.pending.tokenHash {
		return ErrInvalidEmailChangeToken
	}
	now := time.Now().UTC()
	if now.After(u.pending.expiresAt) {
		return ErrEmailChangeTokenExpired
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
	return EventMeta{ID: uuid.New(), AggregateID: u.id, OccurredAt: at}
}

func (u *User) Events() []Event {
	return slices.Clone(u.events)
}
