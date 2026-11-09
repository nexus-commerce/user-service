package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/nexus-commerce/user-service/internal/messaging"
	"github.com/nexus-commerce/user-service/internal/user"
)

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

func (us *Users) ByEmail(ctx context.Context, email user.Email) (*user.User, error) {
	query := `SELECT u.id, u.first_name, u.last_name, u.email, u.password_hash, u.created_at,
			p.email, p.token_hash, p.expires_at
		FROM users u
		LEFT JOIN pending_email_changes p ON p.user_id = u.id
		WHERE u.email = $1`

	u, err := scanUser(us.db.pool.QueryRowContext(ctx, query, email.String()))
	if err != nil && !errors.Is(err, user.ErrNotFound) {
		return nil, fmt.Errorf("select user by email: %w", err)
	}
	return u, err
}

func (us *Users) EmailTaken(ctx context.Context, email user.Email) (bool, error) {
	query := `SELECT EXISTS (SELECT 1 FROM users WHERE email = $1)`

	var taken bool
	if err := us.db.pool.QueryRowContext(ctx, query, email.String()).Scan(&taken); err != nil {
		return false, fmt.Errorf("check email taken: %w", err)
	}
	return taken, nil
}

func (us *Users) ByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	query := `SELECT u.id, u.first_name, u.last_name, u.email, u.password_hash, u.created_at,
			p.email, p.token_hash, p.expires_at
		FROM users u
		LEFT JOIN pending_email_changes p ON p.user_id = u.id
		WHERE u.id = $1`

	u, err := scanUser(us.db.pool.QueryRowContext(ctx, query, id.String()))
	if err != nil && !errors.Is(err, user.ErrNotFound) {
		return nil, fmt.Errorf("select user by id: %w", err)
	}
	return u, err
}

func (us *Users) UpdateByID(ctx context.Context, id uuid.UUID, update func(*user.User) error) (*user.User, error) {
	lockQuery := `SELECT id FROM users WHERE id = $1 FOR UPDATE`

	return us.update(ctx, lockQuery, id.String(), update)
}

func (us *Users) UpdateByEmail(ctx context.Context, email user.Email, update func(*user.User) error) (*user.User, error) {
	lockQuery := `SELECT id FROM users WHERE email = $1 FOR UPDATE`

	return us.update(ctx, lockQuery, email.String(), update)
}

func (us *Users) UpdateByEmailChangeToken(ctx context.Context, hash user.ConfirmationTokenHash, update func(*user.User) error) (*user.User, error) {
	lockQuery := `SELECT u.id FROM users u
		JOIN pending_email_changes p ON p.user_id = u.id
		WHERE p.token_hash = $1
		FOR UPDATE OF u`

	return us.update(ctx, lockQuery, hash.Encoded(), update)
}

func (us *Users) update(ctx context.Context, lockQuery, arg string, update func(*user.User) error) (*user.User, error) {
	selectQuery := `SELECT u.id, u.first_name, u.last_name, u.email, u.password_hash, u.created_at,
			p.email, p.token_hash, p.expires_at
		FROM users u
		LEFT JOIN pending_email_changes p ON p.user_id = u.id
		WHERE u.id = $1`
	updateQuery := `UPDATE users SET first_name = $1, last_name = $2, email = $3, password_hash = $4 WHERE id = $5`

	var u *user.User
	err := us.db.runInTx(ctx, func(tx *sql.Tx) error {
		var id string
		err := tx.QueryRowContext(ctx, lockQuery, arg).Scan(&id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return user.ErrNotFound
			}
			return fmt.Errorf("lock user for update: %w", err)
		}

		u, err = scanUser(tx.QueryRowContext(ctx, selectQuery, id))
		if err != nil {
			if errors.Is(err, user.ErrNotFound) {
				return err
			}
			return fmt.Errorf("select user for update: %w", err)
		}

		if err = update(u); err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, updateQuery,
			u.FullName().FirstName().String(),
			u.FullName().LastName().String(),
			u.Email().String(),
			u.PasswordHash().Encoded(),
			u.ID().String(),
		)
		if err != nil {
			if isEmailUniqueViolation(err) {
				return user.ErrEmailTaken
			}
			return fmt.Errorf("update user: %w", err)
		}

		if err = savePendingEmailChange(ctx, tx, u); err != nil {
			return err
		}

		return us.events.Publish(ctx, outboxProducer{tx: tx}, u.Events())
	})
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (us *Users) Add(ctx context.Context, u *user.User) error {
	query := `INSERT INTO users (id, first_name, last_name, email, password_hash, created_at) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO NOTHING`

	return us.db.runInTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, query,
			u.ID().String(),
			u.FullName().FirstName().String(),
			u.FullName().LastName().String(),
			u.Email().String(),
			u.PasswordHash().Encoded(),
			u.CreatedAt(),
		)
		if err != nil {
			if isEmailUniqueViolation(err) {
				return user.ErrEmailTaken
			}
			return fmt.Errorf("insert user: %w", err)
		}
		count, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("count inserted users: %w", err)
		}
		if count == 0 {
			return nil
		}

		return us.events.Publish(ctx, outboxProducer{tx: tx}, u.Events())
	})
}

func savePendingEmailChange(ctx context.Context, tx *sql.Tx, u *user.User) error {
	upsertQuery := `INSERT INTO pending_email_changes (user_id, email, token_hash, expires_at) VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET email = EXCLUDED.email, token_hash = EXCLUDED.token_hash, expires_at = EXCLUDED.expires_at`
	deleteQuery := `DELETE FROM pending_email_changes WHERE user_id = $1`

	p, ok := u.PendingEmailChange()
	if !ok {
		if _, err := tx.ExecContext(ctx, deleteQuery, u.ID().String()); err != nil {
			return fmt.Errorf("delete pending email change: %w", err)
		}
		return nil
	}

	_, err := tx.ExecContext(ctx, upsertQuery,
		u.ID().String(),
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
			user.RestoreConfirmationTokenHash(pendingTokenHash.String),
			pendingExpiresAt.Time,
		)
		pending = &p
	}
	return user.Restore(user.Snapshot{
		ID:                 id,
		Email:              user.RestoreEmail(email),
		FullName:           fullName,
		PasswordHash:       user.RestorePasswordHash(passwordHash),
		CreatedAt:          createdAt,
		PendingEmailChange: pending,
	}), nil
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
