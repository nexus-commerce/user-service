package usecase_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/nexus-commerce/user-service/internal/user"
)

var (
	valueObjects    = cmp.AllowUnexported(user.Email{}, user.Name{}, user.FullName{}, user.ConfirmationToken{}, user.ConfirmationTokenHash{}, user.PasswordHash{})
	ignoreEventMeta = cmpopts.IgnoreFields(user.EventMeta{}, "ID", "OccurredAt")
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

func (f *fakeUsers) ByID(_ context.Context, id uuid.UUID) (*user.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, user.ErrNotFound
	}
	return u, nil
}

func (f *fakeUsers) ByEmail(_ context.Context, email user.Email) (*user.User, error) {
	for _, u := range f.byID {
		if u.Email() == email {
			return u, nil
		}
	}
	return nil, user.ErrNotFound
}

func (f *fakeUsers) EmailTaken(ctx context.Context, email user.Email) (bool, error) {
	_, err := f.ByEmail(ctx, email)
	return err == nil, nil
}

func (f *fakeUsers) UpdateByID(ctx context.Context, id uuid.UUID, update user.Update) (*user.User, error) {
	u, err := f.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	before := *u
	if err = f.apply(u, update); err != nil {
		*u = before
		return nil, err
	}
	return u, nil
}

func (f *fakeUsers) UpdateByEmail(ctx context.Context, email user.Email, update user.Update) (*user.User, error) {
	if f.vanishOnUpdateByEmail {
		return nil, user.ErrNotFound
	}
	u, err := f.ByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	return f.UpdateByID(ctx, u.ID(), update)
}

func (f *fakeUsers) UpdateByEmailChangeToken(ctx context.Context, hash user.ConfirmationTokenHash, update user.Update) (*user.User, error) {
	for id, u := range f.byID {
		if p, ok := u.PendingEmailChange(); ok && p.TokenHash() == hash {
			return f.UpdateByID(ctx, id, update)
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
	return user.RestorePendingRegistration(user.PendingRegistrationSnapshot{
		UserID:       p.UserID(),
		Email:        p.Email(),
		FullName:     p.FullName(),
		PasswordHash: p.PasswordHash(),
		TokenHash:    p.TokenHash(),
		ExpiresAt:    p.ExpiresAt(),
		ConfirmedAt:  confirmedAt,
	})
}

func (f *fakeRegistrations) Save(_ context.Context, p *user.PendingRegistration, supersede user.PendingRegistrationUpdate) error {
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

func (f *fakeRegistrations) ByUserID(_ context.Context, id uuid.UUID) (*user.PendingRegistration, error) {
	return f.find(func(p *user.PendingRegistration) bool { return p.UserID() == id })
}

func (f *fakeRegistrations) UpdateByToken(_ context.Context, hash user.ConfirmationTokenHash, update user.PendingRegistrationUpdate) (*user.PendingRegistration, error) {
	p, err := f.find(func(p *user.PendingRegistration) bool { return p.TokenHash() == hash })
	if err != nil {
		return nil, err
	}
	if err = update(p); err != nil {
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

func (f *fakeUsers) Add(ctx context.Context, u *user.User) error {
	f.addCalls++
	if f.addErr != nil {
		return f.addErr
	}
	if _, ok := f.byID[u.ID()]; ok {
		return nil
	}
	taken, err := f.EmailTaken(ctx, u.Email())
	if err != nil {
		return err
	}
	if taken {
		return user.ErrEmailTaken
	}
	f.add(u)
	return nil
}

func (f *fakeUsers) add(u *user.User) {
	if f.byID == nil {
		f.byID = make(map[uuid.UUID]*user.User)
	}
	f.byID[u.ID()] = u
}

func (f *fakeUsers) apply(u *user.User, update user.Update) error {
	if err := update(u); err != nil {
		return err
	}
	for _, other := range f.byID {
		if other.ID() != u.ID() && other.Email() == u.Email() {
			return user.ErrEmailTaken
		}
	}
	return f.updateErr
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
	u, err := p.Complete()
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}
	users.add(u)
	return u
}

func newJane(t *testing.T) (*user.User, *fakeUsers) {
	t.Helper()
	users := &fakeUsers{}
	return newUser(t, users, "jane@example.com", "Jane", "Doe"), users
}
