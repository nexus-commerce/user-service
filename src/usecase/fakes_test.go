package usecase_test

import (
	"context"
	"testing"
	"uuid"

	"github.com/nexus-commerce/user-service/src/user"
)

type fakeHasher struct{}

func (fakeHasher) Hash(p user.Password) (user.PasswordHash, error) {
	return user.NewPasswordHash("hashed:" + p.Plaintext())
}

func (fakeHasher) Verify(h user.PasswordHash, p user.Password) (bool, error) {
	return h.Encoded() == "hashed:"+p.Plaintext(), nil
}

type countingHasher struct {
	fakeHasher
	hashes int
}

func (c *countingHasher) Hash(p user.Password) (user.PasswordHash, error) {
	c.hashes++
	return c.fakeHasher.Hash(p)
}

type fakeUsers struct {
	byID           map[uuid.UUID]*user.User
	updateErr      error
	vanishOnRecord bool
}

func (f *fakeUsers) add(usr *user.User) {
	if f.byID == nil {
		f.byID = make(map[uuid.UUID]*user.User)
	}
	f.byID[usr.ID()] = usr
}

func (f *fakeUsers) ByID(_ context.Context, id uuid.UUID) (*user.User, error) {
	usr, ok := f.byID[id]
	if !ok {
		return nil, user.ErrNotFound
	}
	return usr, nil
}

func (f *fakeUsers) ByEmail(_ context.Context, email user.Email) (*user.User, error) {
	for _, usr := range f.byID {
		if usr.Email() == email {
			return usr, nil
		}
	}
	return nil, user.ErrNotFound
}

func (f *fakeUsers) EmailTaken(ctx context.Context, email user.Email) (bool, error) {
	_, err := f.ByEmail(ctx, email)
	return err == nil, nil
}

func (f *fakeUsers) UpdateByID(ctx context.Context, id uuid.UUID, updateFn func(*user.User) error) (*user.User, error) {
	usr, err := f.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err = updateFn(usr); err != nil {
		return nil, err
	}
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return usr, nil
}

func (f *fakeUsers) RecordByEmail(ctx context.Context, email user.Email, recordFn func(*user.User)) error {
	if f.vanishOnRecord {
		return user.ErrNotFound
	}
	usr, err := f.ByEmail(ctx, email)
	if err != nil {
		return err
	}
	recordFn(usr)
	return nil
}

func (f *fakeUsers) UpdateByEmailChangeToken(ctx context.Context, hash user.ConfirmationTokenHash, updateFn func(*user.User) error) (*user.User, error) {
	for id, usr := range f.byID {
		if p, ok := usr.PendingEmailChange(); ok && p.TokenHash() == hash {
			return f.UpdateByID(ctx, id, updateFn)
		}
	}
	return nil, user.ErrNotFound
}

type fakeRegistrations struct {
	byEmail    map[user.Email]*user.PendingRegistration
	users      *fakeUsers
	confirmErr error
}

func (f *fakeRegistrations) Save(_ context.Context, p *user.PendingRegistration) error {
	if f.byEmail == nil {
		f.byEmail = make(map[user.Email]*user.PendingRegistration)
	}
	f.byEmail[p.Email()] = p
	return nil
}

func (f *fakeRegistrations) ConvertToUser(_ context.Context, hash user.ConfirmationTokenHash, confirmFn func(*user.PendingRegistration) (*user.User, error)) (*user.User, error) {
	for email, p := range f.byEmail {
		if p.TokenHash() != hash {
			continue
		}
		usr, err := confirmFn(p)
		if err != nil {
			return nil, err
		}
		if f.confirmErr != nil {
			return nil, f.confirmErr
		}
		delete(f.byEmail, email)
		f.users.add(usr)
		return usr, nil
	}
	return nil, user.ErrNotFound
}

func tokenFor(t *testing.T, regs *fakeRegistrations, email string) user.ConfirmationToken {
	t.Helper()
	e, err := user.NewEmail(email)
	if err != nil {
		t.Fatalf("NewEmail(%q) error: %v", email, err)
	}
	p, ok := regs.byEmail[e]
	if !ok {
		t.Fatalf("no pending registration for %q", email)
	}
	for _, ev := range p.Events() {
		if r, ok := ev.(user.RegistrationRequested); ok {
			return r.Token
		}
	}
	t.Fatalf("no RegistrationRequested event for %q", email)
	return user.ConfirmationToken{}
}

func newUser(t *testing.T, users *fakeUsers, email, first, last string) *user.User {
	t.Helper()
	e, err := user.NewEmail(email)
	if err != nil {
		t.Fatalf("NewEmail(%q) error: %v", email, err)
	}
	firstName, err := user.NewName(first)
	if err != nil {
		t.Fatalf("NewName(%q) error: %v", first, err)
	}
	lastName, err := user.NewName(last)
	if err != nil {
		t.Fatalf("NewName(%q) error: %v", last, err)
	}
	password, err := user.NewPassword("s3cretpass")
	if err != nil {
		t.Fatalf("NewPassword() error: %v", err)
	}
	p, err := user.RequestRegistration(e, user.NewFullName(firstName, lastName), password, fakeHasher{})
	if err != nil {
		t.Fatalf("RequestRegistration() error: %v", err)
	}
	var token user.ConfirmationToken
	for _, ev := range p.Events() {
		if r, ok := ev.(user.RegistrationRequested); ok {
			token = r.Token
		}
	}
	usr, err := p.Confirm(token)
	if err != nil {
		t.Fatalf("Confirm() error: %v", err)
	}
	users.add(usr)
	return usr
}

func newJane(t *testing.T) (*user.User, *fakeUsers) {
	t.Helper()
	users := &fakeUsers{}
	return newUser(t, users, "jane@example.com", "Jane", "Doe"), users
}
