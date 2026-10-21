package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/nexus-commerce/user-service/src/user"
)

type Registrations struct {
	db     *Database
	events eventPublisher
}

func NewRegistrations(db *Database, events eventPublisher) *Registrations {
	return &Registrations{db: db, events: events}
}

func (r *Registrations) Save(ctx context.Context, p *user.PendingRegistration) error {
	query := `INSERT INTO pending_registrations (email, user_id, first_name, last_name, password_hash, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (email) DO UPDATE SET user_id = EXCLUDED.user_id, first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name, password_hash = EXCLUDED.password_hash,
			token_hash = EXCLUDED.token_hash, expires_at = EXCLUDED.expires_at`

	return r.db.runInTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, query,
			p.Email().String(),
			p.UserID().String(),
			p.FullName().FirstName().String(),
			p.FullName().LastName().String(),
			p.PasswordHash().Encoded(),
			p.TokenHash().Encoded(),
			p.ExpiresAt(),
		)
		if err != nil {
			return fmt.Errorf("upsert pending registration: %w", err)
		}

		return r.events.Publish(ctx, outboxProducer{tx: tx}, p.Events())
	})
}

func (r *Registrations) ConvertToUser(
	ctx context.Context,
	hash user.ConfirmationTokenHash,
	confirmFn func(*user.PendingRegistration) (*user.User, error),
) (*user.User, error) {
	selectQuery := `SELECT email, user_id, first_name, last_name, password_hash, token_hash, expires_at
		FROM pending_registrations WHERE token_hash = $1 FOR UPDATE`
	deleteQuery := `DELETE FROM pending_registrations WHERE email = $1`

	var usr *user.User
	err := r.db.runInTx(ctx, func(tx *sql.Tx) error {
		p, err := scanPendingRegistration(tx.QueryRowContext(ctx, selectQuery, hash.Encoded()))
		if err != nil {
			if errors.Is(err, user.ErrNotFound) {
				return err
			}
			return fmt.Errorf("select pending registration for update: %w", err)
		}

		usr, err = confirmFn(p)
		if err != nil {
			return err
		}

		if err = insertUser(ctx, tx, usr); err != nil {
			return err
		}

		if _, err = tx.ExecContext(ctx, deleteQuery, p.Email().String()); err != nil {
			return fmt.Errorf("delete pending registration: %w", err)
		}

		return r.events.Publish(ctx, outboxProducer{tx: tx}, usr.Events())
	})
	if err != nil {
		return nil, err
	}
	return usr, nil
}

func scanPendingRegistration(row *sql.Row) (*user.PendingRegistration, error) {
	var (
		email, rawUserID, firstName, lastName, passwordHash, tokenHash string
		expiresAt                                                      time.Time
	)
	err := row.Scan(
		&email,
		&rawUserID,
		&firstName,
		&lastName,
		&passwordHash,
		&tokenHash,
		&expiresAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, user.ErrNotFound
		}
		return nil, err
	}

	userID, err := uuid.Parse(rawUserID)
	if err != nil {
		return nil, fmt.Errorf("restore pending registration: %w", err)
	}
	fullName := user.NewFullName(user.RestoreName(firstName), user.RestoreName(lastName))
	return user.RestorePendingRegistration(
		userID,
		user.RestoreEmail(email),
		fullName,
		user.RestorePasswordHash(passwordHash),
		user.RestoreConfirmationTokenHash(tokenHash),
		expiresAt,
	), nil
}
