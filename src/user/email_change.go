package user

import "time"

const emailChangeTokenTTL = 24 * time.Hour

type PendingEmailChange struct {
	email     Email
	tokenHash ConfirmationTokenHash
	expiresAt time.Time
}

func RestorePendingEmailChange(email Email, tokenHash ConfirmationTokenHash, expiresAt time.Time) PendingEmailChange {
	return PendingEmailChange{
		email:     email,
		tokenHash: tokenHash,
		expiresAt: expiresAt,
	}
}

func (p PendingEmailChange) Email() Email { return p.email }

func (p PendingEmailChange) TokenHash() ConfirmationTokenHash { return p.tokenHash }

func (p PendingEmailChange) ExpiresAt() time.Time { return p.expiresAt }
