package user

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
)

const (
	confirmationTokenBytes  = 32
	confirmationTokenLength = 43
)

var (
	ErrInvalidConfirmationToken = errors.New("invalid confirmation token")
	ErrConfirmationTokenExpired = errors.New("confirmation token expired")
)

type ConfirmationToken struct {
	value string
}

func NewConfirmationToken() ConfirmationToken {
	b := make([]byte, confirmationTokenBytes)
	_, _ = rand.Read(b)
	return ConfirmationToken{value: base64.RawURLEncoding.EncodeToString(b)}
}

func ParseConfirmationToken(s string) (ConfirmationToken, error) {
	if len(s) != confirmationTokenLength {
		return ConfirmationToken{}, ErrInvalidConfirmationToken
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(b) != confirmationTokenBytes {
		return ConfirmationToken{}, ErrInvalidConfirmationToken
	}
	return ConfirmationToken{value: s}, nil
}

func (ConfirmationToken) String() string { return "[REDACTED]" }

func (t ConfirmationToken) Plaintext() string { return t.value }

func (t ConfirmationToken) Hash() ConfirmationTokenHash {
	sum := sha256.Sum256([]byte(t.value))
	return ConfirmationTokenHash{value: hex.EncodeToString(sum[:])}
}

type ConfirmationTokenHash struct {
	value string
}

func RestoreConfirmationTokenHash(s string) ConfirmationTokenHash {
	return ConfirmationTokenHash{value: s}
}

func (h ConfirmationTokenHash) Encoded() string { return h.value }
