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
	query := `SELECT * FROM users WHERE email = $1`

	user := new(entity.User)

	err := u.db.querier(ctx).QueryRowContext(ctx, query, email).Scan(
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
		return nil, fmt.Errorf("select user by email: %w", err)
	}

	return user, nil
}

func (u *Users) ByID(ctx context.Context, id string) (*entity.User, error) {
	query := `SELECT * FROM users WHERE id = $1`

	user := new(entity.User)

	err := u.db.querier(ctx).QueryRowContext(ctx, query, id).Scan(
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
		return nil, fmt.Errorf("select user by id: %w", err)
	}

	return user, nil
}

func (u *Users) Create(ctx context.Context, user *entity.User) error {
	query := `INSERT INTO users (id, first_name, last_name, email, password_hash, created_at) VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := u.db.querier(ctx).ExecContext(ctx, query,
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

	return nil
}

func (u *Users) Update(ctx context.Context, user *entity.User) error {
	query := `UPDATE users SET first_name = $1, last_name = $2, email = $3, password_hash = $4 WHERE id = $5`

	res, err := u.db.querier(ctx).ExecContext(ctx, query,
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

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update user: rows affected: %w", err)
	}
	if n == 0 {
		return usecase.ErrUserNotFound
	}

	return nil
}
