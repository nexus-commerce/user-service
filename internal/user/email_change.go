package user

import "time"

const emailChangeTTL = 24 * time.Hour

type PendingEmailChange struct {
	email        Email
	confirmation Confirmation
}

func RestorePendingEmailChange(email Email, tokenHash ConfirmationTokenHash, expiresAt time.Time) PendingEmailChange {
	return PendingEmailChange{
		email:        email,
		confirmation: Confirmation{tokenHash: tokenHash, expiresAt: expiresAt},
	}
}

func (p PendingEmailChange) Email() Email { return p.email }

func (p PendingEmailChange) TokenHash() ConfirmationTokenHash { return p.confirmation.tokenHash }

func (p PendingEmailChange) ExpiresAt() time.Time { return p.confirmation.expiresAt }
