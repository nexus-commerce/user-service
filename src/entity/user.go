package entity

import (
	"fmt"
	"time"
	"uuid"
)

type Profile struct {
	Email     Email
	FirstName Name
	LastName  Name
}

func (p Profile) Validate() error {
	if err := p.FirstName.Validate(); err != nil {
		return fmt.Errorf("first name: %w", err)
	}
	if err := p.LastName.Validate(); err != nil {
		return fmt.Errorf("last name: %w", err)
	}
	if err := p.Email.Validate(); err != nil {
		return fmt.Errorf("email: %w", err)
	}
	return nil
}

type User struct {
	ID string
	Profile
	PasswordHash string
	CreatedAt    time.Time

	events []Event
}

func NewUser(p Profile, password Password) (*User, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := password.Validate(); err != nil {
		return nil, fmt.Errorf("password: %w", err)
	}

	hash, err := password.Hash()
	if err != nil {
		return nil, err
	}
	u := &User{
		ID:           uuid.New().String(),
		Profile:      p,
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}
	u.events = append(u.events, UserRegistered{
		UserID:     u.ID,
		Profile:    u.Profile,
		OccurredAt: u.CreatedAt,
	})
	return u, nil
}

func (u *User) ChangeProfile(p Profile) error {
	if err := p.Validate(); err != nil {
		return err
	}
	u.Profile = p
	return nil
}

func (u *User) CheckPassword(password Password) (bool, error) {
	return password.Matches(u.PasswordHash)
}

func (u *User) PullEvents() []Event {
	events := u.events
	u.events = nil
	return events
}
