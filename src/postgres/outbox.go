package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nexus-commerce/user-service/src/entity"
	"github.com/nexus-commerce/user-service/src/messaging"
)

type eventEncoder interface {
	Encode(e entity.Event) (messaging.Message, bool, error)
}

type OutboxWriter struct {
	encoder eventEncoder
}

func NewOutboxWriter(encoder eventEncoder) *OutboxWriter {
	return &OutboxWriter{encoder: encoder}
}

func (w *OutboxWriter) Write(ctx context.Context, tx *sql.Tx, events []entity.Event) error {
	msgs := make([]messaging.Message, 0, len(events))
	for _, e := range events {
		msg, ok, err := w.encoder.Encode(e)
		if err != nil {
			return fmt.Errorf("encode event: %w", err)
		}
		if ok {
			msgs = append(msgs, msg)
		}
	}
	return insertOutboxMessages(ctx, tx, msgs)
}

func insertOutboxMessages(ctx context.Context, tx *sql.Tx, msgs []messaging.Message) error {
	query := `INSERT INTO outbox (event_id, topic, key, value)
		SELECT event_id, topic, key, value
		FROM unnest($1::uuid[], $2::text[], $3::bytea[], $4::bytea[])
			WITH ORDINALITY AS m(event_id, topic, key, value, ord)
		ORDER BY ord
		ON CONFLICT (event_id) DO NOTHING`

	if len(msgs) == 0 {
		return nil
	}

	ids := make([]string, len(msgs))
	topics := make([]string, len(msgs))
	keys := make([][]byte, len(msgs))
	values := make([][]byte, len(msgs))
	for i, m := range msgs {
		ids[i] = m.ID
		topics[i] = m.Topic
		keys[i] = []byte(m.Key)
		values[i] = m.Value
	}

	if _, err := tx.ExecContext(ctx, query, ids, topics, keys, values); err != nil {
		return fmt.Errorf("insert %d outbox messages: %w", len(msgs), err)
	}
	return nil
}

type Outbox struct {
	db *DB
}

func NewOutbox(db *DB) *Outbox {
	return &Outbox{db: db}
}

func (o *Outbox) ProcessBatch(ctx context.Context, limit int, fn func(ctx context.Context, msgs []messaging.Message) error) (int, error) {
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

func selectOutboxMessages(ctx context.Context, tx *sql.Tx, limit int) ([]int64, []messaging.Message, error) {
	query := `SELECT id, event_id, topic, key, value FROM outbox ORDER BY id LIMIT $1 FOR UPDATE`

	rows, err := tx.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("select outbox messages: %w", err)
	}
	defer rows.Close()

	var (
		ids  []int64
		msgs []messaging.Message
	)
	for rows.Next() {
		var (
			id  int64
			m   messaging.Message
			key []byte
		)
		if err = rows.Scan(&id, &m.ID, &m.Topic, &key, &m.Value); err != nil {
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
