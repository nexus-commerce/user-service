package user

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
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

type Confirmation struct {
	tokenHash ConfirmationTokenHash
	expiresAt time.Time
}

func issueConfirmation(now time.Time, ttl time.Duration) (Confirmation, ConfirmationToken) {
	token := NewConfirmationToken()
	return Confirmation{tokenHash: token.Hash(), expiresAt: now.Add(ttl)}, token
}

func (c Confirmation) matches(token ConfirmationToken) bool {
	return token.Hash() == c.tokenHash
}

func (c Confirmation) verify(token ConfirmationToken, now time.Time) error {
	if !c.matches(token) {
		return ErrInvalidConfirmationToken
	}
	if now.After(c.expiresAt) {
		return ErrConfirmationTokenExpired
	}
	return nil
}
