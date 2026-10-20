package nacl

import (
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/box"
)

var (
	ErrInvalidPrivateKey = errors.New("invalid private key")
	ErrCannotOpen        = errors.New("cannot open sealed box")
)

type Opener struct {
	publicKey  *[keySize]byte
	privateKey *[keySize]byte
}

func NewOpener(privateKey string) (*Opener, error) {
	raw, err := base64.StdEncoding.DecodeString(privateKey)
	if err != nil || len(raw) != keySize {
		return nil, fmt.Errorf("%w: want base64 of %d bytes", ErrInvalidPrivateKey, keySize)
	}
	rawPub, err := curve25519.X25519(raw, curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPrivateKey, err)
	}
	var priv, pub [keySize]byte
	copy(priv[:], raw)
	copy(pub[:], rawPub)
	return &Opener{publicKey: &pub, privateKey: &priv}, nil
}

func (o *Opener) Open(sealed []byte) ([]byte, error) {
	opened, ok := box.OpenAnonymous(nil, sealed, o.publicKey, o.privateKey)
	if !ok {
		return nil, ErrCannotOpen
	}
	return opened, nil
}
