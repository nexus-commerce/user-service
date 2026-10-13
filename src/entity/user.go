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
}

func NewUser(p Profile, password Password) (*User, error) {
	hash, err := password.Hash()
	if err != nil {
		return nil, err
	}
	return &User{
		ID:           uuid.New().String(),
		Profile:      p,
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}, nil
}

func (u *User) CheckPassword(password Password) (bool, error) {
	return password.Matches(u.PasswordHash)
}
