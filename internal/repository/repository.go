package repository

import (
	"context"
	"database/sql"
	"user-service/internal/model"
)

type Repository struct {
	conn *sql.DB
}

func New(conn *sql.DB) *Repository {
	return &Repository{conn: conn}
}

func (u *Repository) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `SELECT * FROM users WHERE email = $1`

	user := new(model.User)

	err := u.conn.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (u *Repository) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	query := `SELECT * FROM users WHERE id = $1`

	user := new(model.User)

	err := u.conn.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (u *Repository) CreateUser(ctx context.Context, user *model.User) (int64, error) {
	query := `INSERT INTO users (first_name, last_name, email, password_hash, created_at) VALUES ($1, $2, $3, $4, $5) RETURNING id`

	err := u.conn.QueryRowContext(ctx, query,
		user.FirstName,
		user.LastName,
		user.Email,
		user.PasswordHash,
		user.CreatedAt,
	).Scan(&user.ID)
	if err != nil {
		return 0, err
	}

	return user.ID, nil
}

func (u *Repository) UpdateUser(ctx context.Context, user *model.User) error {
	query := `UPDATE users SET first_name = $1, last_name = $2, email = $3, password_hash = $4 WHERE id = $5`

	_, err := u.conn.ExecContext(ctx, query,
		user.FirstName,
		user.LastName,
		user.Email,
		user.PasswordHash,
		user.ID,
	)
	return err
}
