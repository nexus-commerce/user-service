package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"uuid"

	"github.com/google/go-cmp/cmp"

	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/user"
)

type loginUsersFunc func(context.Context, user.Email) (*user.User, error)

func (f loginUsersFunc) ByEmail(ctx context.Context, email user.Email) (*user.User, error) {
	return f(ctx, email)
}

type tokenIssuerFunc func(uuid.UUID) (string, error)

func (f tokenIssuerFunc) Issue(id uuid.UUID) (string, error) { return f(id) }

type loginVerification struct {
	Hash     string
	Password string
}

type loginHasher struct {
	hashErr       error
	verifyErr     error
	hashCalls     int
	dummy         loginVerification
	verifications []loginVerification
}

func (h *loginHasher) Hash(p user.Password) (user.PasswordHash, error) {
	h.hashCalls++
	if h.hashErr != nil {
		return user.PasswordHash{}, h.hashErr
	}
	hash, err := (fakeHasher{}).Hash(p)
	h.dummy = loginVerification{Hash: hash.Encoded(), Password: p.Plaintext()}
	return hash, err
}

func (h *loginHasher) Verify(hash user.PasswordHash, p user.Password) (bool, error) {
	h.verifications = append(h.verifications, loginVerification{Hash: hash.Encoded(), Password: p.Plaintext()})
	if h.verifyErr != nil {
		return false, h.verifyErr
	}
	return (fakeHasher{}).Verify(hash, p)
}

func TestLoginIssuesToken(t *testing.T) {
	for _, email := range []string{"jane@example.com", "JANE@EXAMPLE.COM"} {
		t.Run(email, func(t *testing.T) {
			u, users := newJane(t)
			hasher := &loginHasher{}
			var readCalls, issueCalls int
			repo := loginUsersFunc(func(ctx context.Context, email user.Email) (*user.User, error) {
				readCalls++
				if ctx != t.Context() || email != u.Email() {
					t.Errorf("ByEmail() got context %v, email %v; want test context, %v", ctx, email, u.Email())
				}
				return users.ByEmail(ctx, email)
			})
			tokens := tokenIssuerFunc(func(id uuid.UUID) (string, error) {
				issueCalls++
				if id != u.ID() {
					t.Errorf("Issue() user ID = %v, want %v", id, u.ID())
				}
				if len(hasher.verifications) != 1 {
					t.Error("token issued before password verification")
				}
				return "access-token", nil
			})
			login, err := usecase.NewLogin(repo, tokens, hasher)
			if err != nil {
				t.Fatalf("NewLogin() error: %v", err)
			}

			token, err := login.Execute(t.Context(), usecase.Credentials{Email: email, Password: "s3cretpass"})
			if err != nil || token != "access-token" {
				t.Errorf("Execute() = %q, %v; want access-token, nil", token, err)
			}
			if readCalls != 1 || issueCalls != 1 {
				t.Errorf("Execute() ByEmail/Issue calls = %d/%d, want 1/1", readCalls, issueCalls)
			}
			want := []loginVerification{{Hash: u.PasswordHash().Encoded(), Password: "s3cretpass"}}
			if diff := cmp.Diff(want, hasher.verifications); diff != "" {
				t.Errorf("Execute() Verify calls mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	tests := []struct {
		name          string
		email         string
		password      string
		readErr       error
		verifyErr     error
		wantReadCalls int
		realPassword  bool
	}{
		{name: "empty email", password: "s3cretpass"},
		{name: "malformed email", email: "invalid", password: "s3cretpass"},
		{name: "empty password", email: "jane@example.com"},
		{name: "short password", email: "jane@example.com", password: "short"},
		{name: "long password", email: "jane@example.com", password: strings.Repeat("x", 73)},
		{name: "unknown email", email: "unknown@example.com", password: "s3cretpass", readErr: user.ErrNotFound, wantReadCalls: 1},
		{name: "wrapped not found", email: "unknown@example.com", password: "s3cretpass", readErr: fmt.Errorf("lookup: %w", user.ErrNotFound), wantReadCalls: 1},
		{name: "dummy verification fails", email: "unknown@example.com", password: "s3cretpass", readErr: user.ErrNotFound, verifyErr: errors.New("verify failed"), wantReadCalls: 1},
		{name: "wrong password", email: "jane@example.com", password: "wrongpass", wantReadCalls: 1, realPassword: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, users := newJane(t)
			hasher := &loginHasher{verifyErr: tt.verifyErr}
			var readCalls int
			repo := loginUsersFunc(func(ctx context.Context, email user.Email) (*user.User, error) {
				readCalls++
				if tt.readErr != nil {
					return nil, tt.readErr
				}
				return users.ByEmail(ctx, email)
			})
			tokens := tokenIssuerFunc(func(uuid.UUID) (string, error) {
				t.Fatal("Issue() called for invalid credentials")
				return "", nil
			})
			login, err := usecase.NewLogin(repo, tokens, hasher)
			if err != nil {
				t.Fatalf("NewLogin() error: %v", err)
			}

			token, err := login.Execute(t.Context(), usecase.Credentials{Email: tt.email, Password: tt.password})
			if token != "" || !errors.Is(err, user.ErrInvalidCredentials) {
				t.Errorf("Execute() = %q, %v; want empty token, %v", token, err, user.ErrInvalidCredentials)
			}
			if readCalls != tt.wantReadCalls {
				t.Errorf("Execute() ByEmail calls = %d, want %d", readCalls, tt.wantReadCalls)
			}
			if hasher.hashCalls != 1 {
				t.Errorf("Hash() calls = %d, want one at initialization", hasher.hashCalls)
			}
			want := hasher.dummy
			if tt.realPassword {
				want = loginVerification{Hash: u.PasswordHash().Encoded(), Password: tt.password}
			}
			if diff := cmp.Diff([]loginVerification{want}, hasher.verifications); diff != "" {
				t.Errorf("Execute() Verify calls mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLoginPropagatesDependencyErrors(t *testing.T) {
	want := errors.New("dependency failed")
	tests := []struct {
		name            string
		readErr         error
		verifyErr       error
		issueErr        error
		wantVerifyCalls int
		wantIssueCalls  int
	}{
		{name: "repository", readErr: fmt.Errorf("lookup: %w", want)},
		{name: "hasher", verifyErr: want, wantVerifyCalls: 1},
		{name: "token issuer", issueErr: want, wantVerifyCalls: 1, wantIssueCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, users := newJane(t)
			hasher := &loginHasher{verifyErr: tt.verifyErr}
			repo := loginUsersFunc(func(ctx context.Context, email user.Email) (*user.User, error) {
				if tt.readErr != nil {
					return nil, tt.readErr
				}
				return users.ByEmail(ctx, email)
			})
			var issueCalls int
			tokens := tokenIssuerFunc(func(uuid.UUID) (string, error) {
				issueCalls++
				return "", tt.issueErr
			})
			login, err := usecase.NewLogin(repo, tokens, hasher)
			if err != nil {
				t.Fatalf("NewLogin() error: %v", err)
			}

			token, err := login.Execute(t.Context(), usecase.Credentials{Email: "jane@example.com", Password: "s3cretpass"})
			if token != "" || !errors.Is(err, want) {
				t.Errorf("Execute() = %q, %v; want empty token, %v", token, err, want)
			}
			if len(hasher.verifications) != tt.wantVerifyCalls || issueCalls != tt.wantIssueCalls {
				t.Errorf("Execute() Verify/Issue calls = %d/%d, want %d/%d", len(hasher.verifications), issueCalls, tt.wantVerifyCalls, tt.wantIssueCalls)
			}
		})
	}
}

func TestNewLoginReturnsDummyHashError(t *testing.T) {
	want := errors.New("hash failed")
	hasher := &loginHasher{hashErr: want}
	repo := loginUsersFunc(func(context.Context, user.Email) (*user.User, error) {
		t.Fatal("ByEmail() called during initialization")
		return nil, nil
	})
	tokens := tokenIssuerFunc(func(uuid.UUID) (string, error) {
		t.Fatal("Issue() called during initialization")
		return "", nil
	})

	login, err := usecase.NewLogin(repo, tokens, hasher)
	if login != nil || !errors.Is(err, want) {
		t.Errorf("NewLogin() = %v, %v; want nil, %v", login, err, want)
	}
	if hasher.hashCalls != 1 || len(hasher.verifications) != 0 {
		t.Errorf("NewLogin() Hash/Verify calls = %d/%d, want 1/0", hasher.hashCalls, len(hasher.verifications))
	}
}
