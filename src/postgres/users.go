package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/nexus-commerce/user-service/src/entity"
	"github.com/nexus-commerce/user-service/src/usecase"
)

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

type Users struct {
	db *DB
}

func NewUsers(db *DB) *Users {
	return &Users{db: db}
}

func (u *Users) ByEmail(ctx context.Context, email entity.Email) (*entity.User, error) {
	query := `SELECT id, first_name, last_name, email, password_hash, created_at FROM users WHERE email = $1`

	user, err := scanUser(u.db.pool.QueryRowContext(ctx, query, email))
	if err != nil && !errors.Is(err, usecase.ErrUserNotFound) {
		return nil, fmt.Errorf("select user by email: %w", err)
	}
	return user, err
}

func (u *Users) ByID(ctx context.Context, id string) (*entity.User, error) {
	query := `SELECT id, first_name, last_name, email, password_hash, created_at FROM users WHERE id = $1`

	user, err := scanUser(u.db.pool.QueryRowContext(ctx, query, id))
	if err != nil && !errors.Is(err, usecase.ErrUserNotFound) {
		return nil, fmt.Errorf("select user by id: %w", err)
	}
	return user, err
}

func (u *Users) CreateWithMessage(ctx context.Context, user *entity.User, msg usecase.Message) error {
	userQuery := `INSERT INTO users (id, first_name, last_name, email, password_hash, created_at) VALUES ($1, $2, $3, $4, $5, $6)`
	outboxQuery := `INSERT INTO outbox (topic, key, value) VALUES ($1, $2, $3)`

	return u.db.runInTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, userQuery,
			user.ID,
			user.FirstName,
			user.LastName,
			user.Email,
			user.PasswordHash,
			user.CreatedAt,
		)
		if err != nil {
			if isEmailTaken(err) {
				return usecase.ErrEmailTaken
			}
			return fmt.Errorf("insert user: %w", err)
		}

		_, err = tx.ExecContext(ctx, outboxQuery, msg.Topic, []byte(msg.Key), msg.Value)
		if err != nil {
			return fmt.Errorf("insert outbox message: %w", err)
		}
		return nil
	})
}

func (u *Users) UpdateByID(ctx context.Context, id string, updateFn func(user *entity.User) (bool, error)) (*entity.User, error) {
	selectQuery := `SELECT id, first_name, last_name, email, password_hash, created_at FROM users WHERE id = $1 FOR UPDATE`
	updateQuery := `UPDATE users SET first_name = $1, last_name = $2, email = $3, password_hash = $4 WHERE id = $5`

	var user *entity.User
	err := u.db.runInTx(ctx, func(tx *sql.Tx) error {
		var err error
		user, err = scanUser(tx.QueryRowContext(ctx, selectQuery, id))
		if err != nil {
			if errors.Is(err, usecase.ErrUserNotFound) {
				return err
			}
			return fmt.Errorf("select user for update: %w", err)
		}

		updated, err := updateFn(user)
		if err != nil || !updated {
			return err
		}

		_, err = tx.ExecContext(ctx, updateQuery,
			user.FirstName,
			user.LastName,
			user.Email,
			user.PasswordHash,
			user.ID,
		)
		if err != nil {
			if isEmailTaken(err) {
				return usecase.ErrEmailTaken
			}
			return fmt.Errorf("update user: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}

func scanUser(row *sql.Row) (*entity.User, error) {
	user := new(entity.User)
	err := row.Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, usecase.ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}
