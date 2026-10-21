package postgres

import (
	"context"
	"fmt"
	"time"
)

type PendingRequests struct {
	db *Database
}

func NewPendingRequests(db *Database) *PendingRequests {
	return &PendingRequests{db: db}
}

func (p *PendingRequests) DeleteExpiredRegistrations(ctx context.Context, before time.Time) (int64, error) {
	query := `DELETE FROM pending_registrations WHERE expires_at < $1`

	res, err := p.db.pool.ExecContext(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("delete expired registrations: %w", err)
	}
	return res.RowsAffected()
}

func (p *PendingRequests) DeleteExpiredEmailChanges(ctx context.Context, before time.Time) (int64, error) {
	query := `DELETE FROM pending_email_changes WHERE expires_at < $1`

	res, err := p.db.pool.ExecContext(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("delete expired email changes: %w", err)
	}
	return res.RowsAffected()
}
