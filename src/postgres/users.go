package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/nexus-commerce/user-service/src/messaging"
	"github.com/nexus-commerce/user-service/src/user"
)

const selectUser = `SELECT u.id, u.first_name, u.last_name, u.email, u.password_hash, u.created_at,
		p.email, p.token_hash, p.expires_at
	FROM users u
	LEFT JOIN pending_email_changes p ON p.user_id = u.id`

type eventPublisher interface {
	Publish(ctx context.Context, pr messaging.Producer, events []user.Event) error
}

type Users struct {
	db     *Database
	events eventPublisher
}

func NewUsers(db *Database, events eventPublisher) *Users {
	return &Users{db: db, events: events}
}

func (u *Users) ByEmail(ctx context.Context, email user.Email) (*user.User, error) {
	query := selectUser + ` WHERE u.email = $1`

	usr, err := scanUser(u.db.pool.QueryRowContext(ctx, query, email.String()))
	if err != nil && !errors.Is(err, user.ErrUserNotFound) {
		return nil, fmt.Errorf("select user by email: %w", err)
	}
	return usr, err
}

func (u *Users) ByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	query := selectUser + ` WHERE u.id = $1`

	usr, err := scanUser(u.db.pool.QueryRowContext(ctx, query, id.String()))
	if err != nil && !errors.Is(err, user.ErrUserNotFound) {
		return nil, fmt.Errorf("select user by id: %w", err)
	}
	return usr, err
}

func (u *Users) Update(
	ctx context.Context,
	id uuid.UUID,
	updateFn func(*user.User) error,
) (*user.User, error) {
	selectQuery := selectUser + ` WHERE u.id = $1 FOR UPDATE OF u`
	updateQuery := `UPDATE users SET first_name = $1, last_name = $2, email = $3, password_hash = $4 WHERE id = $5`

	var usr *user.User
	err := u.db.runInTx(ctx, func(tx *sql.Tx) error {
		var err error
		usr, err = scanUser(tx.QueryRowContext(ctx, selectQuery, id.String()))
		if err != nil {
			if errors.Is(err, user.ErrUserNotFound) {
				return err
			}
			return fmt.Errorf("select user for update: %w", err)
		}

		if err = updateFn(usr); err != nil {
			return err
		}
		if len(usr.Events()) == 0 {
			return nil
		}

		_, err = tx.ExecContext(ctx, updateQuery,
			usr.FullName().FirstName().String(),
			usr.FullName().LastName().String(),
			usr.Email().String(),
			usr.PasswordHash().Encoded(),
			usr.ID().String(),
		)
		if err != nil {
			if isEmailTaken(err) {
				return user.ErrEmailTaken
			}
			return fmt.Errorf("update user: %w", err)
		}

		if err = savePendingEmailChange(ctx, tx, usr); err != nil {
			return err
		}

		return u.events.Publish(ctx, outboxProducer{tx: tx}, usr.Events())
	})
	if err != nil {
		return nil, err
	}
	return usr, nil
}

func (u *Users) Add(ctx context.Context, usr *user.User) error {
	query := `INSERT INTO users (id, first_name, last_name, email, password_hash, created_at) VALUES ($1, $2, $3, $4, $5, $6)`

	return u.db.runInTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, query,
			usr.ID().String(),
			usr.FullName().FirstName().String(),
			usr.FullName().LastName().String(),
			usr.Email().String(),
			usr.PasswordHash().Encoded(),
			usr.CreatedAt(),
		)
		if err != nil {
			if isEmailTaken(err) {
				return user.ErrEmailTaken
			}
			return fmt.Errorf("insert user: %w", err)
		}

		return u.events.Publish(ctx, outboxProducer{tx: tx}, usr.Events())
	})
}

func (u *Users) IDByEmailChangeToken(ctx context.Context, hash user.EmailChangeTokenHash) (uuid.UUID, error) {
	query := `SELECT user_id FROM pending_email_changes WHERE token_hash = $1`

	var rawID string
	err := u.db.pool.QueryRowContext(ctx, query, hash.Encoded()).Scan(&rawID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil(), user.ErrUserNotFound
		}
		return uuid.Nil(), fmt.Errorf("select user id by email change token: %w", err)
	}
	id, err := uuid.Parse(rawID)
	if err != nil {
		return uuid.Nil(), fmt.Errorf("parse user id: %w", err)
	}
	return id, nil
}

func savePendingEmailChange(ctx context.Context, tx *sql.Tx, usr *user.User) error {
	upsertQuery := `INSERT INTO pending_email_changes (user_id, email, token_hash, expires_at) VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET email = EXCLUDED.email, token_hash = EXCLUDED.token_hash, expires_at = EXCLUDED.expires_at`
	deleteQuery := `DELETE FROM pending_email_changes WHERE user_id = $1`

	p, ok := usr.PendingEmailChange()
	if !ok {
		if _, err := tx.ExecContext(ctx, deleteQuery, usr.ID().String()); err != nil {
			return fmt.Errorf("delete pending email change: %w", err)
		}
		return nil
	}

	_, err := tx.ExecContext(ctx, upsertQuery,
		usr.ID().String(),
		p.Email().String(),
		p.TokenHash().Encoded(),
		p.ExpiresAt(),
	)
	if err != nil {
		return fmt.Errorf("upsert pending email change: %w", err)
	}
	return nil
}

func scanUser(row *sql.Row) (*user.User, error) {
	var (
		rawID, firstName, lastName, email, passwordHash string
		createdAt                                       time.Time
		pendingEmail, pendingTokenHash                  sql.NullString
		pendingExpiresAt                                sql.NullTime
	)
	err := row.Scan(
		&rawID,
		&firstName,
		&lastName,
		&email,
		&passwordHash,
		&createdAt,
		&pendingEmail,
		&pendingTokenHash,
		&pendingExpiresAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, user.ErrUserNotFound
		}
		return nil, err
	}

	id, err := uuid.Parse(rawID)
	if err != nil {
		return nil, fmt.Errorf("restore user: %w", err)
	}
	fullName := user.RestoreFullName(user.FullNameFields{
		FirstName: firstName,
		LastName:  lastName,
	})
	hash, err := user.NewPasswordHash(passwordHash)
	if err != nil {
		return nil, fmt.Errorf("restore user %s: %w", id, err)
	}
	var pending *user.PendingEmailChange
	if pendingEmail.Valid {
		tokenHash, err := user.NewEmailChangeTokenHash(pendingTokenHash.String)
		if err != nil {
			return nil, fmt.Errorf("restore user %s: %w", id, err)
		}
		p := user.RestorePendingEmailChange(pendingEmail.String, tokenHash, pendingExpiresAt.Time)
		pending = &p
	}
	return user.RestoreUser(id, email, fullName, hash, createdAt, pending), nil
}

const (
	uniqueViolation       = "23505"
	emailUniqueConstraint = "users_email_key"
)

func isEmailTaken(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	if !ok {
		return false
	}

	isUniqueViolation := pgErr.Code == uniqueViolation
	isEmailConstraint := pgErr.ConstraintName == emailUniqueConstraint
	return isUniqueViolation && isEmailConstraint
}
