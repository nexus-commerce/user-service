package usecase_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/nexus-commerce/user-service/internal/user"
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
	hashes   int
	verifies int
}

func (c *countingHasher) Verify(h user.PasswordHash, p user.Password) (bool, error) {
	c.verifies++
	return c.fakeHasher.Verify(h, p)
}

func (c *countingHasher) Hash(p user.Password) (user.PasswordHash, error) {
	c.hashes++
	return c.fakeHasher.Hash(p)
}

type fakeUsers struct {
	byID                  map[uuid.UUID]*user.User
	addErr                error
	addCalls              int
	updateErr             error
	vanishOnUpdateByEmail bool
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
	before := *usr
	if err = f.apply(usr, updateFn); err != nil {
		*usr = before
		return nil, err
	}
	return usr, nil
}

func (f *fakeUsers) apply(usr *user.User, updateFn func(*user.User) error) error {
	if err := updateFn(usr); err != nil {
		return err
	}
	for _, other := range f.byID {
		if other.ID() != usr.ID() && other.Email() == usr.Email() {
			return user.ErrEmailTaken
		}
	}
	return f.updateErr
}

func (f *fakeUsers) UpdateByEmail(ctx context.Context, email user.Email, updateFn func(*user.User) error) (*user.User, error) {
	if f.vanishOnUpdateByEmail {
		return nil, user.ErrNotFound
	}
	usr, err := f.ByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	return f.UpdateByID(ctx, usr.ID(), updateFn)
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
	byEmail   map[user.Email]*user.PendingRegistration
	readErr   error
	updateErr error
	removeErr error
	published []user.Event
}

func copyPending(p *user.PendingRegistration) *user.PendingRegistration {
	var confirmedAt *time.Time
	if at, ok := p.ConfirmedAt(); ok {
		confirmedAt = &at
	}
	return user.RestorePendingRegistration(p.UserID(), p.Email(), p.FullName(), p.PasswordHash(), p.TokenHash(), p.ExpiresAt(), confirmedAt)
}

func (f *fakeRegistrations) Save(_ context.Context, p *user.PendingRegistration, supersede func(*user.PendingRegistration) error) error {
	if f.byEmail == nil {
		f.byEmail = make(map[user.Email]*user.PendingRegistration)
	}
	saved := p
	if existing, ok := f.byEmail[p.Email()]; ok {
		saved = copyPending(existing)
		if err := supersede(saved); err != nil {
			return err
		}
	}
	f.byEmail[saved.Email()] = saved
	f.published = append(f.published, saved.Events()...)
	return nil
}

func (f *fakeRegistrations) find(match func(*user.PendingRegistration) bool) (*user.PendingRegistration, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	for _, p := range f.byEmail {
		if match(p) {
			return copyPending(p), nil
		}
	}
	return nil, user.ErrRegistrationNotFound
}

func (f *fakeRegistrations) ByUserID(_ context.Context, id uuid.UUID) (*user.PendingRegistration, error) {
	return f.find(func(p *user.PendingRegistration) bool { return p.UserID() == id })
}

func (f *fakeRegistrations) UpdateByToken(_ context.Context, hash user.ConfirmationTokenHash, updateFn func(*user.PendingRegistration) error) (*user.PendingRegistration, error) {
	p, err := f.find(func(p *user.PendingRegistration) bool { return p.TokenHash() == hash })
	if err != nil {
		return nil, err
	}
	if err = updateFn(p); err != nil {
		return nil, err
	}
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.byEmail[p.Email()] = p
	f.published = append(f.published, p.Events()...)
	return p, nil
}

func (f *fakeRegistrations) Remove(_ context.Context, p *user.PendingRegistration) error {
	if f.removeErr != nil {
		return f.removeErr
	}
	stored, ok := f.byEmail[p.Email()]
	if !ok || stored.UserID() != p.UserID() {
		return nil
	}
	delete(f.byEmail, p.Email())
	f.published = append(f.published, p.Events()...)
	return nil
}

func (f *fakeUsers) Add(ctx context.Context, usr *user.User) error {
	f.addCalls++
	if f.addErr != nil {
		return f.addErr
	}
	if _, ok := f.byID[usr.ID()]; ok {
		return nil
	}
	taken, err := f.EmailTaken(ctx, usr.Email())
	if err != nil {
		return err
	}
	if taken {
		return user.ErrEmailTaken
	}
	f.add(usr)
	return nil
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
	if err := p.Confirm(token); err != nil {
		t.Fatalf("Confirm() error: %v", err)
	}
	usr, err := p.Complete()
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}
	users.add(usr)
	return usr
}

func newJane(t *testing.T) (*user.User, *fakeUsers) {
	t.Helper()
	users := &fakeUsers{}
	return newUser(t, users, "jane@example.com", "Jane", "Doe"), users
}
