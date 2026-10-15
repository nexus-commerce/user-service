package entity

import (
	"errors"
	"fmt"
	"slices"
	"time"
	"uuid"
)

type Profile struct {
	email     Email
	firstName Name
	lastName  Name
}

func NewProfile(email, firstName, lastName string) (Profile, error) {
	fn, err := NewName(firstName)
	if err != nil {
		return Profile{}, fmt.Errorf("first name: %w", err)
	}
	ln, err := NewName(lastName)
	if err != nil {
		return Profile{}, fmt.Errorf("last name: %w", err)
	}
	e, err := NewEmail(email)
	if err != nil {
		return Profile{}, fmt.Errorf("email: %w", err)
	}
	return Profile{email: e, firstName: fn, lastName: ln}, nil
}

func RestoreProfile(email, firstName, lastName string) Profile {
	return Profile{
		email:     Email{value: email},
		firstName: Name{value: firstName},
		lastName:  Name{value: lastName},
	}
}

func (p Profile) Email() Email { return p.email }

func (p Profile) FirstName() Name { return p.firstName }

func (p Profile) LastName() Name { return p.lastName }

var (
	ErrUserNotFound          = errors.New("user not found")
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrEmailTaken            = errors.New("user email already exists")
	ErrMalformedPasswordHash = errors.New("malformed password hash")
)

type User struct {
	id           uuid.UUID
	profile      Profile
	passwordHash string
	createdAt    time.Time

	events []Event
}

func NewUser(p Profile, password Password) (*User, error) {
	hash, err := password.Hash()
	if err != nil {
		return nil, err
	}
	u := &User{
		id:           uuid.New(),
		profile:      p,
		passwordHash: hash,
		createdAt:    time.Now().UTC(),
	}
	u.events = append(u.events, UserRegistered{
		ID:         uuid.New(),
		UserID:     u.id,
		Profile:    u.profile,
		OccurredAt: u.createdAt,
	})
	return u, nil
}

func RestoreUser(id uuid.UUID, p Profile, passwordHash string, createdAt time.Time) (*User, error) {
	if !validPasswordHash(passwordHash) {
		return nil, ErrMalformedPasswordHash
	}
	return &User{
		id:           id,
		profile:      p,
		passwordHash: passwordHash,
		createdAt:    createdAt,
	}, nil
}

func (u *User) ID() uuid.UUID { return u.id }

func (u *User) Email() Email { return u.profile.email }

func (u *User) FirstName() Name { return u.profile.firstName }

func (u *User) LastName() Name { return u.profile.lastName }

func (u *User) PasswordHash() string { return u.passwordHash }

func (u *User) CreatedAt() time.Time { return u.createdAt }

func (u *User) ChangeProfile(p Profile) bool {
	if u.profile == p {
		return false
	}
	u.profile = p
	return true
}

func (u *User) Authenticate(password Password) error {
	ok, err := password.Matches(u.passwordHash)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidCredentials
	}
	return nil
}

func (u *User) Events() []Event {
	return slices.Clone(u.events)
}
