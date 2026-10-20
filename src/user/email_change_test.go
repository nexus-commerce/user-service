package user_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/nexus-commerce/user-service/src/user"
)

func TestNewEmailChangeTokenRoundTrip(t *testing.T) {
	tok := user.NewEmailChangeToken()
	if n := len(tok.Plaintext()); n != 43 {
		t.Fatalf("len(Plaintext()) = %d, want 43", n)
	}
	parsed, err := user.ParseEmailChangeToken(tok.Plaintext())
	if err != nil || parsed.Hash() != tok.Hash() {
		t.Errorf("ParseEmailChangeToken(Plaintext()) = %v, %v; want same hash, nil", parsed.Hash().Encoded(), err)
	}
	if user.NewEmailChangeToken().Hash() == tok.Hash() {
		t.Error("two generated tokens have the same hash")
	}
}

func TestParseEmailChangeTokenRejectsGarbage(t *testing.T) {
	for _, s := range []string{"", "short", strings.Repeat("a", 44), strings.Repeat("!", 43)} {
		if _, err := user.ParseEmailChangeToken(s); !errors.Is(err, user.ErrInvalidEmailChangeToken) {
			t.Errorf("ParseEmailChangeToken(%q) error = %v, want %v", s, err, user.ErrInvalidEmailChangeToken)
		}
	}
}

func TestEmailChangeTokenHash(t *testing.T) {
	tok := mustToken(t, strings.Repeat("A", 43))
	h := tok.Hash().Encoded()
	if len(h) != 64 || strings.ToLower(h) != h {
		t.Errorf("Hash().Encoded() = %q, want 64 lowercase hex chars", h)
	}
	if restored := user.RestoreEmailChangeTokenHash(h); restored != tok.Hash() {
		t.Errorf("RestoreEmailChangeTokenHash(%q) = %q, want equal hash", h, restored.Encoded())
	}
}

func TestEmailChangeTokenIsRedacted(t *testing.T) {
	tok := user.NewEmailChangeToken()
	if got := fmt.Sprintf("%v %s", tok, tok); strings.Contains(got, tok.Plaintext()) {
		t.Errorf("formatted token %q leaks plaintext", got)
	}
}

func mustToken(t *testing.T, s string) user.EmailChangeToken {
	t.Helper()
	tok, err := user.ParseEmailChangeToken(s)
	if err != nil {
		t.Fatalf("ParseEmailChangeToken(%q) error: %v", s, err)
	}
	return tok
}
