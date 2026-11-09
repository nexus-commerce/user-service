package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/nexus-commerce/user-service/internal/user"
)

const saveAttempts = 3

var errConcurrentRegistration = errors.New("concurrent registration for the same email")

type Registrations struct {
	db     *Database
	events eventPublisher
}

func NewRegistrations(db *Database, events eventPublisher) *Registrations {
	return &Registrations{db: db, events: events}
}

func (r *Registrations) Save(ctx context.Context, p *user.PendingRegistration, supersede func(existing *user.PendingRegistration) error) error {
	var err error
	for range saveAttempts {
		err = r.db.runInTx(ctx, func(tx *sql.Tx) error {
			return r.save(ctx, tx, p, supersede)
		})
		if !errors.Is(err, errConcurrentRegistration) {
			return err
		}
	}
	return err
}

func insertPendingRegistration(ctx context.Context, tx *sql.Tx, p *user.PendingRegistration) error {
	query := `INSERT INTO pending_registrations (email, user_id, first_name, last_name, password_hash, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (email) DO NOTHING`

	res, err := tx.ExecContext(ctx, query,
		p.Email().String(),
		p.UserID().String(),
		p.FullName().FirstName().String(),
		p.FullName().LastName().String(),
		p.PasswordHash().Encoded(),
		p.TokenHash().Encoded(),
		p.ExpiresAt(),
	)
	if err != nil {
		return fmt.Errorf("insert pending registration: %w", err)
	}
	count, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("count inserted pending registrations: %w", err)
	}
	if count == 0 {
		return errConcurrentRegistration
	}
	return nil
}

func updatePendingRegistration(ctx context.Context, tx *sql.Tx, p *user.PendingRegistration) error {
	query := `UPDATE pending_registrations SET first_name = $2, last_name = $3,
			password_hash = $4, token_hash = $5, expires_at = $6, confirmed_at = $7
		WHERE user_id = $1`

	var confirmedAt sql.NullTime
	confirmedAt.Time, confirmedAt.Valid = p.ConfirmedAt()
	_, err := tx.ExecContext(ctx, query,
		p.UserID().String(),
		p.FullName().FirstName().String(),
		p.FullName().LastName().String(),
		p.PasswordHash().Encoded(),
		p.TokenHash().Encoded(),
		p.ExpiresAt(),
		confirmedAt,
	)
	if err != nil {
		return fmt.Errorf("update pending registration: %w", err)
	}
	return nil
}

func (r *Registrations) ByUserID(ctx context.Context, id uuid.UUID) (*user.PendingRegistration, error) {
	query := `SELECT email, user_id, first_name, last_name, password_hash, token_hash, expires_at, confirmed_at
		FROM pending_registrations
		WHERE user_id = $1`

	p, err := scanPendingRegistration(r.db.pool.QueryRowContext(ctx, query, id.String()))
	if err != nil && !errors.Is(err, user.ErrRegistrationNotFound) {
		return nil, fmt.Errorf("select pending registration by user id: %w", err)
	}
	return p, err
}

func (r *Registrations) UpdateByToken(ctx context.Context, hash user.ConfirmationTokenHash, update func(*user.PendingRegistration) error) (*user.PendingRegistration, error) {
	selectQuery := `SELECT email, user_id, first_name, last_name, password_hash, token_hash, expires_at, confirmed_at
		FROM pending_registrations
		WHERE token_hash = $1 FOR UPDATE`

	var p *user.PendingRegistration
	err := r.db.runInTx(ctx, func(tx *sql.Tx) error {
		var err error
		p, err = scanPendingRegistration(tx.QueryRowContext(ctx, selectQuery, hash.Encoded()))
		if err != nil {
			if errors.Is(err, user.ErrRegistrationNotFound) {
				return err
			}
			return fmt.Errorf("select pending registration for update: %w", err)
		}

		if err = update(p); err != nil {
			return err
		}

		if err = updatePendingRegistration(ctx, tx, p); err != nil {
			return err
		}

		return r.events.Publish(ctx, outboxProducer{tx: tx}, p.Events())
	})
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *Registrations) Remove(ctx context.Context, p *user.PendingRegistration) error {
	query := `DELETE FROM pending_registrations WHERE user_id = $1`

	return r.db.runInTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, query, p.UserID().String())
		if err != nil {
			return fmt.Errorf("delete pending registration: %w", err)
		}
		count, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("count deleted pending registrations: %w", err)
		}
		if count == 0 {
			return nil
		}

		return r.events.Publish(ctx, outboxProducer{tx: tx}, p.Events())
	})
}

func (r *Registrations) save(ctx context.Context, tx *sql.Tx, p *user.PendingRegistration, supersede func(existing *user.PendingRegistration) error) error {
	selectQuery := `SELECT email, user_id, first_name, last_name, password_hash, token_hash, expires_at, confirmed_at
		FROM pending_registrations
		WHERE email = $1 FOR UPDATE`

	saved := p
	existing, err := scanPendingRegistration(tx.QueryRowContext(ctx, selectQuery, p.Email().String()))
	switch {
	case errors.Is(err, user.ErrRegistrationNotFound):
		err = insertPendingRegistration(ctx, tx, p)
	case err != nil:
		return fmt.Errorf("select pending registration by email: %w", err)
	default:
		if err = supersede(existing); err != nil {
			return err
		}
		saved = existing
		err = updatePendingRegistration(ctx, tx, existing)
	}
	if err != nil {
		return err
	}

	return r.events.Publish(ctx, outboxProducer{tx: tx}, saved.Events())
}

func scanPendingRegistration(row *sql.Row) (*user.PendingRegistration, error) {
	var (
		email, rawUserID, firstName, lastName, passwordHash, tokenHash string
		expiresAt                                                      time.Time
		confirmedAt                                                    sql.NullTime
	)
	err := row.Scan(
		&email,
		&rawUserID,
		&firstName,
		&lastName,
		&passwordHash,
		&tokenHash,
		&expiresAt,
		&confirmedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, user.ErrRegistrationNotFound
		}
		return nil, err
	}

	userID, err := uuid.Parse(rawUserID)
	if err != nil {
		return nil, fmt.Errorf("restore pending registration: %w", err)
	}
	var confirmed *time.Time
	if confirmedAt.Valid {
		confirmed = new(confirmedAt.Time)
	}
	fullName := user.NewFullName(user.RestoreName(firstName), user.RestoreName(lastName))
	return user.RestorePendingRegistration(user.PendingRegistrationSnapshot{
		UserID:       userID,
		Email:        user.RestoreEmail(email),
		FullName:     fullName,
		PasswordHash: user.RestorePasswordHash(passwordHash),
		TokenHash:    user.RestoreConfirmationTokenHash(tokenHash),
		ExpiresAt:    expiresAt,
		ConfirmedAt:  confirmed,
	}), nil
}
