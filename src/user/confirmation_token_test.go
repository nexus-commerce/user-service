package user_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/nexus-commerce/user-service/src/user"
)

func TestNewConfirmationTokenRoundTrip(t *testing.T) {
	tok := user.NewConfirmationToken()
	if n := len(tok.Plaintext()); n != 43 {
		t.Fatalf("len(Plaintext()) = %d, want 43", n)
	}
	parsed, err := user.ParseConfirmationToken(tok.Plaintext())
	if err != nil || parsed.Hash() != tok.Hash() {
		t.Errorf("ParseConfirmationToken(Plaintext()) = %v, %v; want same hash, nil", parsed.Hash().Encoded(), err)
	}
	if user.NewConfirmationToken().Hash() == tok.Hash() {
		t.Error("two generated tokens have the same hash")
	}
}

func TestParseConfirmationTokenRejectsGarbage(t *testing.T) {
	for _, s := range []string{"", "short", strings.Repeat("a", 44), strings.Repeat("!", 43)} {
		if _, err := user.ParseConfirmationToken(s); !errors.Is(err, user.ErrInvalidConfirmationToken) {
			t.Errorf("ParseConfirmationToken(%q) error = %v, want %v", s, err, user.ErrInvalidConfirmationToken)
		}
	}
}

func TestConfirmationTokenHash(t *testing.T) {
	tok := mustToken(t, strings.Repeat("A", 43))
	h := tok.Hash().Encoded()
	if len(h) != 64 || strings.ToLower(h) != h {
		t.Errorf("Hash().Encoded() = %q, want 64 lowercase hex chars", h)
	}
	if restored := user.RestoreConfirmationTokenHash(h); restored != tok.Hash() {
		t.Errorf("RestoreConfirmationTokenHash(%q) = %q, want equal hash", h, restored.Encoded())
	}
}

func TestConfirmationTokenIsRedacted(t *testing.T) {
	tok := user.NewConfirmationToken()
	if got := fmt.Sprintf("%v %s", tok, tok); strings.Contains(got, tok.Plaintext()) {
		t.Errorf("formatted token %q leaks plaintext", got)
	}
}

func mustToken(t *testing.T, s string) user.ConfirmationToken {
	t.Helper()
	tok, err := user.ParseConfirmationToken(s)
	if err != nil {
		t.Fatalf("ParseConfirmationToken(%q) error: %v", s, err)
	}
	return tok
}
