package bcrypt_test

import (
	"errors"
	"testing"

	xbcrypt "golang.org/x/crypto/bcrypt"

	"github.com/nexus-commerce/user-service/src/bcrypt"
	"github.com/nexus-commerce/user-service/src/user"
)

func mustPassword(t *testing.T, s string) user.Password {
	t.Helper()
	p, err := user.NewPassword(s)
	if err != nil {
		t.Fatalf("NewPassword(%q) error: %v", s, err)
	}
	return p
}

func TestHasherVerify(t *testing.T) {
	h := bcrypt.NewHasher(xbcrypt.MinCost)
	p := mustPassword(t, "s3cretpass")
	other := mustPassword(t, "otherpass")

	hash, err := h.Hash(p)
	if err != nil {
		t.Fatalf("Hash() error: %v", err)
	}
	if ok, err := h.Verify(hash, p); err != nil || !ok {
		t.Errorf("Verify(own hash) = %v, %v; want true, nil", ok, err)
	}
	if ok, err := h.Verify(hash, other); err != nil || ok {
		t.Errorf("Verify(foreign hash) = %v, %v; want false, nil", ok, err)
	}
}

func TestHasherVerifyMalformedHash(t *testing.T) {
	h := bcrypt.NewHasher(xbcrypt.MinCost)
	hash, err := user.NewPasswordHash("not-a-bcrypt-hash")
	if err != nil {
		t.Fatalf("NewPasswordHash() error: %v", err)
	}

	ok, err := h.Verify(hash, mustPassword(t, "s3cretpass"))
	if ok || !errors.Is(err, user.ErrMalformedPasswordHash) {
		t.Errorf("Verify(malformed) = %v, %v; want false, %v", ok, err, user.ErrMalformedPasswordHash)
	}
}
