package entity

import (
	"errors"
	"slices"
	"time"
	"uuid"
)

type Profile struct {
	email     Email
	firstName Name
	lastName  Name
}

type ProfileFields struct {
	Email     string
	FirstName string
	LastName  string
}

func NewProfile(f ProfileFields) (Profile, error) {
	fn, err := NewName(f.FirstName)
	if err != nil {
		return Profile{}, &FieldError{Field: "firstName", Err: err}
	}
	ln, err := NewName(f.LastName)
	if err != nil {
		return Profile{}, &FieldError{Field: "lastName", Err: err}
	}
	e, err := NewEmail(f.Email)
	if err != nil {
		return Profile{}, &FieldError{Field: "email", Err: err}
	}
	return Profile{email: e, firstName: fn, lastName: ln}, nil
}

func RestoreProfile(f ProfileFields) Profile {
	return Profile{
		email:     Email{value: f.Email},
		firstName: Name{value: f.FirstName},
		lastName:  Name{value: f.LastName},
	}
}

func (p Profile) Email() Email { return p.email }

func (p Profile) FirstName() Name { return p.firstName }

func (p Profile) LastName() Name { return p.lastName }

type ProfileChange struct {
	Email     *string
	FirstName *string
	LastName  *string
}

func (p Profile) Apply(c ProfileChange) (Profile, error) {
	if err := applyField(&p.firstName, c.FirstName, "firstName", NewName); err != nil {
		return Profile{}, err
	}
	if err := applyField(&p.lastName, c.LastName, "lastName", NewName); err != nil {
		return Profile{}, err
	}
	if err := applyField(&p.email, c.Email, "email", NewEmail); err != nil {
		return Profile{}, err
	}
	return p, nil
}

func applyField[T any](dst *T, v *string, field string, parse func(string) (T, error)) error {
	if v == nil {
		return nil
	}
	parsed, err := parse(*v)
	if err != nil {
		return &FieldError{Field: field, Err: err}
	}
	*dst = parsed
	return nil
}

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailTaken         = errors.New("user email already exists")
)

type User struct {
	id           uuid.UUID
	profile      Profile
	passwordHash PasswordHash
	createdAt    time.Time

	events []Event
}

func NewUser(p Profile, password Password, hasher Hasher) (*User, error) {
	hash, err := hasher.Hash(password)
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
		EventMeta: u.newEventMeta(u.createdAt),
		Profile:   u.profile,
	})
	return u, nil
}

func RestoreUser(id uuid.UUID, p Profile, passwordHash PasswordHash, createdAt time.Time) *User {
	return &User{
		id:           id,
		profile:      p,
		passwordHash: passwordHash,
		createdAt:    createdAt,
	}
}

func (u *User) ID() uuid.UUID { return u.id }

func (u *User) Email() Email { return u.profile.email }

func (u *User) FirstName() Name { return u.profile.firstName }

func (u *User) LastName() Name { return u.profile.lastName }

func (u *User) PasswordHash() PasswordHash { return u.passwordHash }

func (u *User) CreatedAt() time.Time { return u.createdAt }

func (u *User) ChangeProfile(c ProfileChange) (bool, error) {
	p, err := u.profile.Apply(c)
	if err != nil {
		return false, err
	}
	if p == u.profile {
		return false, nil
	}
	u.profile = p
	return true, nil
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
