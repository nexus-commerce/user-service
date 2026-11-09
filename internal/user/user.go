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

type Update func(*User) error

type Reauthentication struct {
	userID       uuid.UUID
	passwordHash PasswordHash
}

type Snapshot struct {
	ID                 uuid.UUID
	Email              Email
	FullName           FullName
	PasswordHash       PasswordHash
	CreatedAt          time.Time
	PendingEmailChange *PendingEmailChange
}

func Restore(s Snapshot) *User {
	return &User{
		id:           s.ID,
		email:        s.Email,
		fullName:     s.FullName,
		passwordHash: s.PasswordHash,
		createdAt:    s.CreatedAt,
		pending:      s.PendingEmailChange,
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
		FullName:  u.fullName,
		Email:     u.email,
	})
}

func (u *User) RequestEmailChange(newEmail Email, taken bool, proof Reauthentication) error {
	if proof.userID != u.id || proof.passwordHash != u.passwordHash {
		return ErrInvalidCredentials
	}
	if newEmail == u.email {
		return ErrEmailUnchanged
	}
	now := time.Now().UTC()
	if taken {
		u.pending = nil
		u.events = append(u.events, EmailChangeToTakenAddressAttempted{
			EventMeta: u.newEventMeta(now),
			NewEmail:  newEmail,
		})
		return nil
	}
	confirmation, token := issueConfirmation(now, emailChangeTTL)
	u.pending = &PendingEmailChange{email: newEmail, confirmation: confirmation}
	u.events = append(u.events, EmailChangeRequested{
		EventMeta: u.newEventMeta(now),
		FullName:  u.fullName,
		NewEmail:  newEmail,
		Token:     token,
		ExpiresAt: confirmation.expiresAt,
	})
	return nil
}

func (u *User) ConfirmEmailChange(token ConfirmationToken) error {
	if u.pending == nil {
		return ErrInvalidConfirmationToken
	}
	now := time.Now().UTC()
	if err := u.pending.confirmation.verify(token, now); err != nil {
		return err
	}
	old := u.email
	u.email = u.pending.email
	u.pending = nil
	u.events = append(u.events, EmailChanged{
		EventMeta: u.newEventMeta(now),
		FullName:  u.fullName,
		OldEmail:  old,
		NewEmail:  u.email,
	})
	return nil
}

func (u *User) RejectEmailChange(token ConfirmationToken) error {
	if u.pending == nil || !u.pending.confirmation.matches(token) {
		return ErrInvalidConfirmationToken
	}
	rejected := u.pending.email
	u.pending = nil
	u.events = append(u.events, EmailChangeRejected{
		EventMeta: u.newEventMeta(time.Now().UTC()),
		NewEmail:  rejected,
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

func (u *User) Reauthenticate(password Password, hasher Hasher) (Reauthentication, error) {
	if err := u.Authenticate(password, hasher); err != nil {
		return Reauthentication{}, err
	}
	return Reauthentication{userID: u.id, passwordHash: u.passwordHash}, nil
}

func (u *User) Events() []Event {
	return slices.Clone(u.events)
}

func (u *User) newEventMeta(at time.Time) EventMeta {
	return newEventMeta(u.id, at)
}
