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
	if err != nil && !errors.Is(err, user.ErrNotFound) {
		return nil, fmt.Errorf("select user by email: %w", err)
	}
	return usr, err
}

func (u *Users) EmailTaken(ctx context.Context, email user.Email) (bool, error) {
	query := `SELECT EXISTS (SELECT 1 FROM users WHERE email = $1)`

	var taken bool
	if err := u.db.pool.QueryRowContext(ctx, query, email.String()).Scan(&taken); err != nil {
		return false, fmt.Errorf("check email taken: %w", err)
	}
	return taken, nil
}

func (u *Users) ByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	query := selectUser + ` WHERE u.id = $1`

	usr, err := scanUser(u.db.pool.QueryRowContext(ctx, query, id.String()))
	if err != nil && !errors.Is(err, user.ErrNotFound) {
		return nil, fmt.Errorf("select user by id: %w", err)
	}
	return usr, err
}

func (u *Users) UpdateByID(
	ctx context.Context,
	id uuid.UUID,
	updateFn func(*user.User) error,
) (*user.User, error) {
	return u.update(ctx, selectUser+` WHERE u.id = $1 FOR UPDATE OF u`, id.String(), updateFn)
}

func (u *Users) UpdateByEmailChangeToken(
	ctx context.Context,
	hash user.EmailChangeTokenHash,
	updateFn func(*user.User) error,
) (*user.User, error) {
	return u.update(ctx, selectUser+` WHERE p.token_hash = $1 FOR UPDATE OF u`, hash.Encoded(), updateFn)
}

func (u *Users) update(
	ctx context.Context,
	selectQuery string,
	arg string,
	updateFn func(*user.User) error,
) (*user.User, error) {
	updateQuery := `UPDATE users SET first_name = $1, last_name = $2, email = $3, password_hash = $4 WHERE id = $5`

	var usr *user.User
	err := u.db.runInTx(ctx, func(tx *sql.Tx) error {
		var err error
		usr, err = scanUser(tx.QueryRowContext(ctx, selectQuery, arg))
		if err != nil {
			if errors.Is(err, user.ErrNotFound) {
				return err
			}
			return fmt.Errorf("select user for update: %w", err)
		}

		if err = updateFn(usr); err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, updateQuery,
			usr.FullName().FirstName().String(),
			usr.FullName().LastName().String(),
			usr.Email().String(),
			usr.PasswordHash().Encoded(),
			usr.ID().String(),
		)
		if err != nil {
			if isEmailUniqueViolation(err) {
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
			if isEmailUniqueViolation(err) {
				return user.ErrEmailTaken
			}
			return fmt.Errorf("insert user: %w", err)
		}

		return u.events.Publish(ctx, outboxProducer{tx: tx}, usr.Events())
	})
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
			return nil, user.ErrNotFound
		}
		return nil, err
	}

	id, err := uuid.Parse(rawID)
	if err != nil {
		return nil, fmt.Errorf("restore user: %w", err)
	}
	fullName := user.NewFullName(user.RestoreName(firstName), user.RestoreName(lastName))
	var pending *user.PendingEmailChange
	if pendingEmail.Valid {
		p := user.RestorePendingEmailChange(
			user.RestoreEmail(pendingEmail.String),
			user.RestoreEmailChangeTokenHash(pendingTokenHash.String),
			pendingExpiresAt.Time,
		)
		pending = &p
	}
	return user.Restore(id, user.RestoreEmail(email), fullName, user.RestorePasswordHash(passwordHash), createdAt, pending), nil
}

const (
	uniqueViolation       = "23505"
	emailUniqueConstraint = "users_email_key"
)

func isEmailUniqueViolation(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	if !ok {
		return false
	}

	isUniqueViolation := pgErr.Code == uniqueViolation
	isEmailConstraint := pgErr.ConstraintName == emailUniqueConstraint
	return isUniqueViolation && isEmailConstraint
}
