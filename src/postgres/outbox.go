package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nexus-commerce/user-service/src/usecase"
)

type Outbox struct {
	db *DB
}

func NewOutbox(db *DB) *Outbox {
	return &Outbox{db: db}
}

func (o *Outbox) ProcessBatch(ctx context.Context, limit int, fn func(ctx context.Context, msgs []usecase.Message) error) (int, error) {
	query := `DELETE FROM outbox WHERE id = ANY($1)`

	var count int
	err := o.db.runInTx(ctx, func(tx *sql.Tx) error {
		ids, msgs, err := selectOutboxMessages(ctx, tx, limit)
		if err != nil {
			return err
		}
		if len(msgs) == 0 {
			return nil
		}

		if err = fn(ctx, msgs); err != nil {
			return err
		}

		if _, err = tx.ExecContext(ctx, query, ids); err != nil {
			return fmt.Errorf("delete outbox messages: %w", err)
		}
		count = len(msgs)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func selectOutboxMessages(ctx context.Context, tx *sql.Tx, limit int) ([]int64, []usecase.Message, error) {
	query := `SELECT id, topic, key, value FROM outbox ORDER BY id LIMIT $1 FOR UPDATE`

	rows, err := tx.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("select outbox messages: %w", err)
	}
	defer rows.Close()

	var (
		ids  []int64
		msgs []usecase.Message
	)
	for rows.Next() {
		var (
			id  int64
			m   usecase.Message
			key []byte
		)
		if err = rows.Scan(&id, &m.Topic, &key, &m.Value); err != nil {
			return nil, nil, fmt.Errorf("scan outbox message: %w", err)
		}
		m.Key = string(key)
		ids = append(ids, id)
		msgs = append(msgs, m)
	}
	if err = rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate outbox messages: %w", err)
	}
	return ids, msgs, nil
}
