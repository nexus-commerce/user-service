package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/nexus-commerce/user-service/src/entity"
)

type eventWriter interface {
	Write(ctx context.Context, tx *sql.Tx, events []entity.Event) error
}

type Users struct {
	db     *DB
	events eventWriter
}

func NewUsers(db *DB, events eventWriter) *Users {
	return &Users{db: db, events: events}
}

func (u *Users) ByEmail(ctx context.Context, email entity.Email) (*entity.User, error) {
	query := `SELECT id, first_name, last_name, email, password_hash, created_at FROM users WHERE email = $1`

	user, err := scanUser(u.db.pool.QueryRowContext(ctx, query, email.String()))
	if err != nil && !errors.Is(err, entity.ErrUserNotFound) {
		return nil, fmt.Errorf("select user by email: %w", err)
	}
	return user, err
}

func (u *Users) ByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	query := `SELECT id, first_name, last_name, email, password_hash, created_at FROM users WHERE id = $1`

	user, err := scanUser(u.db.pool.QueryRowContext(ctx, query, id.String()))
	if err != nil && !errors.Is(err, entity.ErrUserNotFound) {
		return nil, fmt.Errorf("select user by id: %w", err)
	}
	return user, err
}

func (u *Users) Update(ctx context.Context, id uuid.UUID, updateFn func(user *entity.User) (bool, error)) (*entity.User, error) {
	selectQuery := `SELECT id, first_name, last_name, email, password_hash, created_at FROM users WHERE id = $1 FOR UPDATE`
	updateQuery := `UPDATE users SET first_name = $1, last_name = $2, email = $3, password_hash = $4 WHERE id = $5`

	var user *entity.User
	err := u.db.runInTx(ctx, func(tx *sql.Tx) error {
		var err error
		user, err = scanUser(tx.QueryRowContext(ctx, selectQuery, id.String()))
		if err != nil {
			if errors.Is(err, entity.ErrUserNotFound) {
				return err
			}
			return fmt.Errorf("select user for update: %w", err)
		}

		updated, err := updateFn(user)
		if err != nil || !updated {
			return err
		}

		_, err = tx.ExecContext(ctx, updateQuery,
			user.FirstName().String(),
			user.LastName().String(),
			user.Email().String(),
			user.PasswordHash(),
			user.ID().String(),
		)
		if err != nil {
			if isEmailTaken(err) {
				return entity.ErrEmailTaken
			}
			return fmt.Errorf("update user: %w", err)
		}

		return u.events.Write(ctx, tx, user.Events())
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (u *Users) Add(ctx context.Context, user *entity.User) error {
	query := `INSERT INTO users (id, first_name, last_name, email, password_hash, created_at) VALUES ($1, $2, $3, $4, $5, $6)`

	return u.db.runInTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, query,
			user.ID().String(),
			user.FirstName().String(),
			user.LastName().String(),
			user.Email().String(),
			user.PasswordHash(),
			user.CreatedAt(),
		)
		if err != nil {
			if isEmailTaken(err) {
				return entity.ErrEmailTaken
			}
			return fmt.Errorf("insert user: %w", err)
		}

		return u.events.Write(ctx, tx, user.Events())
	})
}

func scanUser(row *sql.Row) (*entity.User, error) {
	var (
		rawID, firstName, lastName, email, passwordHash string
		createdAt                                       time.Time
	)
	err := row.Scan(
		&rawID,
		&firstName,
		&lastName,
		&email,
		&passwordHash,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, entity.ErrUserNotFound
		}
		return nil, err
	}

	id, err := uuid.Parse(rawID)
	if err != nil {
		return nil, fmt.Errorf("restore user: %w", err)
	}
	profile := entity.RestoreProfile(email, firstName, lastName)
	user, err := entity.RestoreUser(id, profile, passwordHash, createdAt)
	if err != nil {
		return nil, fmt.Errorf("restore user %s: %w", id, err)
	}
	return user, nil
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
