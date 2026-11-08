package nacl

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/nacl/box"
)

const keySize = 32

var (
	ErrInvalidPublicKey = errors.New("invalid public key")
	ErrEmptyKeyID       = errors.New("empty key id")
)

type Sealer struct {
	publicKey *[keySize]byte
	keyID     string
}

func NewSealer(publicKey, keyID string) (*Sealer, error) {
	raw, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil || len(raw) != keySize {
		return nil, fmt.Errorf("%w: want base64 of %d bytes", ErrInvalidPublicKey, keySize)
	}
	if keyID == "" {
		return nil, ErrEmptyKeyID
	}
	var key [keySize]byte
	copy(key[:], raw)
	return &Sealer{publicKey: &key, keyID: keyID}, nil
}

func (s *Sealer) Seal(plaintext []byte) ([]byte, error) {
	sealed, err := box.SealAnonymous(nil, plaintext, s.publicKey, rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("seal: %w", err)
	}
	return sealed, nil
}

func (s *Sealer) KeyID() string { return s.keyID }
