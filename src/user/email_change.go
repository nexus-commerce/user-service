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
	emailChangeTokenBytes  = 32
	emailChangeTokenLength = 43
	emailChangeTokenTTL    = 24 * time.Hour
)

var (
	ErrInvalidEmailChangeToken = errors.New("invalid email change token")
	ErrEmailChangeTokenExpired = errors.New("email change token expired")
)

type EmailChangeToken struct {
	value string
}

func NewEmailChangeToken() EmailChangeToken {
	b := make([]byte, emailChangeTokenBytes)
	_, _ = rand.Read(b)
	return EmailChangeToken{value: base64.RawURLEncoding.EncodeToString(b)}
}

func ParseEmailChangeToken(s string) (EmailChangeToken, error) {
	if len(s) != emailChangeTokenLength {
		return EmailChangeToken{}, ErrInvalidEmailChangeToken
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(b) != emailChangeTokenBytes {
		return EmailChangeToken{}, ErrInvalidEmailChangeToken
	}
	return EmailChangeToken{value: s}, nil
}

func (EmailChangeToken) String() string { return "[REDACTED]" }

func (t EmailChangeToken) Plaintext() string { return t.value }

func (t EmailChangeToken) Hash() EmailChangeTokenHash {
	sum := sha256.Sum256([]byte(t.value))
	return EmailChangeTokenHash{value: hex.EncodeToString(sum[:])}
}

type EmailChangeTokenHash struct {
	value string
}

func RestoreEmailChangeTokenHash(s string) EmailChangeTokenHash {
	return EmailChangeTokenHash{value: s}
}

func (h EmailChangeTokenHash) Encoded() string { return h.value }

type PendingEmailChange struct {
	email     Email
	tokenHash EmailChangeTokenHash
	expiresAt time.Time
}

func RestorePendingEmailChange(email Email, tokenHash EmailChangeTokenHash, expiresAt time.Time) PendingEmailChange {
	return PendingEmailChange{
		email:     email,
		tokenHash: tokenHash,
		expiresAt: expiresAt,
	}
}

func (p PendingEmailChange) Email() Email { return p.email }

func (p PendingEmailChange) TokenHash() EmailChangeTokenHash { return p.tokenHash }

func (p PendingEmailChange) ExpiresAt() time.Time { return p.expiresAt }
