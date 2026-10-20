package bcrypt_test

import (
	"errors"
	"testing"

	xbcrypt "golang.org/x/crypto/bcrypt"

	"github.com/nexus-commerce/user-service/src/bcrypt"
	"github.com/nexus-commerce/user-service/src/entity"
)

func mustPassword(t *testing.T, s string) entity.Password {
	t.Helper()
	p, err := entity.NewPassword(s)
	if err != nil {
		t.Fatalf("NewPassword(%q) error: %v", s, err)
	}
	return p
}

func TestHasherVerify(t *testing.T) {
	h := bcrypt.NewHasher(xbcrypt.MinCost)
	p := mustPassword(t, "s3cret")
	other := mustPassword(t, "other")

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
	hash, err := entity.NewPasswordHash("not-a-bcrypt-hash")
	if err != nil {
		t.Fatalf("NewPasswordHash() error: %v", err)
	}

	ok, err := h.Verify(hash, mustPassword(t, "s3cret"))
	if ok || !errors.Is(err, entity.ErrMalformedPasswordHash) {
		t.Errorf("Verify(malformed) = %v, %v; want false, %v", ok, err, entity.ErrMalformedPasswordHash)
	}
}
