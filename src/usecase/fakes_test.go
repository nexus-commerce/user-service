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

type fakeUsers struct {
	byID      map[uuid.UUID]*user.User
	updateErr error
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
		return nil, user.ErrUserNotFound
	}
	return usr, nil
}

func (f *fakeUsers) ByEmail(_ context.Context, email user.Email) (*user.User, error) {
	for _, usr := range f.byID {
		if usr.Email() == email {
			return usr, nil
		}
	}
	return nil, user.ErrUserNotFound
}

func (f *fakeUsers) Update(ctx context.Context, id uuid.UUID, updateFn func(*user.User) error) (*user.User, error) {
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

func (f *fakeUsers) IDByEmailChangeToken(_ context.Context, hash user.EmailChangeTokenHash) (uuid.UUID, error) {
	for id, usr := range f.byID {
		if p, ok := usr.PendingEmailChange(); ok && p.TokenHash() == hash {
			return id, nil
		}
	}
	return uuid.Nil(), user.ErrUserNotFound
}

func newUser(t *testing.T, users *fakeUsers, email, first, last string) *user.User {
	t.Helper()
	e, err := user.NewEmail(email)
	if err != nil {
		t.Fatalf("NewEmail(%q) error: %v", email, err)
	}
	fn, err := user.NewName(first)
	if err != nil {
		t.Fatalf("NewName(%q) error: %v", first, err)
	}
	ln, err := user.NewName(last)
	if err != nil {
		t.Fatalf("NewName(%q) error: %v", last, err)
	}
	password, err := user.NewPassword("s3cretpass")
	if err != nil {
		t.Fatalf("NewPassword() error: %v", err)
	}
	usr, err := user.NewUser(e, user.NewProfile(fn, ln), password, fakeHasher{})
	if err != nil {
		t.Fatalf("NewUser() error: %v", err)
	}
	users.add(usr)
	return usr
}

func newJane(t *testing.T) (*user.User, *fakeUsers) {
	t.Helper()
	users := &fakeUsers{}
	return newUser(t, users, "jane@example.com", "Jane", "Doe"), users
}
