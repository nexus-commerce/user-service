package postgres

import (
	"context"
	"fmt"
	"time"
)

type Retention struct {
	db *Database
}

func NewRetention(db *Database) *Retention {
	return &Retention{db: db}
}

func (r *Retention) DeleteAbandonedRegistrations(ctx context.Context, before time.Time) (int64, error) {
	query := `DELETE FROM pending_registrations WHERE expires_at < $1 AND confirmed_at IS NULL`

	res, err := r.db.pool.ExecContext(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("delete abandoned registrations: %w", err)
	}
	return res.RowsAffected()
}

func (r *Retention) DeleteExpiredEmailChanges(ctx context.Context, before time.Time) (int64, error) {
	query := `DELETE FROM pending_email_changes WHERE expires_at < $1`

	res, err := r.db.pool.ExecContext(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("delete expired email changes: %w", err)
	}
	return res.RowsAffected()
}
