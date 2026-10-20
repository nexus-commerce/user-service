package bcrypt

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/nexus-commerce/user-service/src/user"
)

const DefaultCost = bcrypt.DefaultCost

type Hasher struct {
	cost int
}

func NewHasher(cost int) *Hasher {
	return &Hasher{cost: cost}
}

func (h *Hasher) Hash(p user.Password) (user.PasswordHash, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(p.Plaintext()), h.cost)
	if err != nil {
		return user.PasswordHash{}, fmt.Errorf("hash password: %w", err)
	}
	return user.NewPasswordHash(string(hash))
}

func (h *Hasher) Verify(hash user.PasswordHash, p user.Password) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash.Encoded()), []byte(p.Plaintext()))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword), errors.Is(err, bcrypt.ErrPasswordTooLong):
		return false, nil
	default:
		return false, fmt.Errorf("%w: %v", user.ErrMalformedPasswordHash, err)
	}
}
