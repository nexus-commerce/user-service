package user

import (
	"errors"
	"slices"
	"time"
	"uuid"
)

type Profile struct {
	firstName Name
	lastName  Name
}

type ProfileFields struct {
	FirstName string
	LastName  string
}

func NewProfile(firstName, lastName Name) Profile {
	return Profile{firstName: firstName, lastName: lastName}
}

func RestoreProfile(f ProfileFields) Profile {
	return Profile{
		firstName: Name{value: f.FirstName},
		lastName:  Name{value: f.LastName},
	}
}

func (p Profile) FirstName() Name { return p.firstName }

func (p Profile) LastName() Name { return p.lastName }

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailTaken         = errors.New("user email already exists")
	ErrEmailUnchanged     = NewValidationError("must differ from the current email")
)

type User struct {
	id           uuid.UUID
	email        Email
	profile      Profile
	passwordHash PasswordHash
	createdAt    time.Time
	pending      *PendingEmailChange

	events []Event
}

func NewUser(email Email, p Profile, password Password, hasher Hasher) (*User, error) {
	hash, err := hasher.Hash(password)
	if err != nil {
		return nil, err
	}
	u := &User{
		id:           uuid.New(),
		email:        email,
		profile:      p,
		passwordHash: hash,
		createdAt:    time.Now().UTC(),
	}
	u.events = append(u.events, Registered{
		EventMeta: u.newEventMeta(u.createdAt),
		Email:     u.email,
		Profile:   u.profile,
	})
	return u, nil
}

func RestoreUser(id uuid.UUID, email string, p Profile, passwordHash PasswordHash, createdAt time.Time, pending *PendingEmailChange) *User {
	return &User{
		id:           id,
		email:        Email{value: email},
		profile:      p,
		passwordHash: passwordHash,
		createdAt:    createdAt,
		pending:      pending,
	}
}

func (u *User) ID() uuid.UUID { return u.id }

func (u *User) Email() Email { return u.email }

func (u *User) FirstName() Name { return u.profile.firstName }

func (u *User) LastName() Name { return u.profile.lastName }

func (u *User) PasswordHash() PasswordHash { return u.passwordHash }

func (u *User) CreatedAt() time.Time { return u.createdAt }

func (u *User) PendingEmailChange() (PendingEmailChange, bool) {
	if u.pending == nil {
		return PendingEmailChange{}, false
	}
	return *u.pending, true
}

func (u *User) Rename(first, last Name) {
	p := NewProfile(first, last)
	if p == u.profile {
		return
	}
	u.profile = p
	u.events = append(u.events, Renamed{
		EventMeta: u.newEventMeta(time.Now().UTC()),
		FirstName: first,
		LastName:  last,
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
