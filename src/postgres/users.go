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

type eventEncoder interface {
	Encode(e entity.Event) (usecase.Message, error)
}

type Users struct {
	db      *DB
	encoder eventEncoder
}

func NewUsers(db *DB, encoder eventEncoder) *Users {
	return &Users{db: db, encoder: encoder}
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

		msgs, err := u.pullMessages(user)
		if err != nil {
			return err
		}
		return insertOutboxMessages(ctx, tx, msgs)
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (u *Users) Create(ctx context.Context, user *entity.User) error {
	query := `INSERT INTO users (id, first_name, last_name, email, password_hash, created_at) VALUES ($1, $2, $3, $4, $5, $6)`

	msgs, err := u.pullMessages(user)
	if err != nil {
		return err
	}

	return u.db.runInTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, query,
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

		return insertOutboxMessages(ctx, tx, msgs)
	})
}

func (u *Users) pullMessages(user *entity.User) ([]usecase.Message, error) {
	events := user.PullEvents()
	msgs := make([]usecase.Message, len(events))
	for i, e := range events {
		msg, err := u.encoder.Encode(e)
		if err != nil {
			return nil, fmt.Errorf("encode event: %w", err)
		}
		msgs[i] = msg
	}
	return msgs, nil
}

func insertOutboxMessages(ctx context.Context, tx *sql.Tx, msgs []usecase.Message) error {
	query := `INSERT INTO outbox (topic, key, value)
		SELECT topic, key, value
		FROM unnest($1::text[], $2::bytea[], $3::bytea[]) WITH ORDINALITY AS m(topic, key, value, ord)
		ORDER BY ord`

	if len(msgs) == 0 {
		return nil
	}

	topics := make([]string, len(msgs))
	keys := make([][]byte, len(msgs))
	values := make([][]byte, len(msgs))
	for i, m := range msgs {
		topics[i] = m.Topic
		keys[i] = []byte(m.Key)
		values[i] = m.Value
	}

	if _, err := tx.ExecContext(ctx, query, topics, keys, values); err != nil {
		return fmt.Errorf("insert %d outbox messages: %w", len(msgs), err)
	}
	return nil
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
