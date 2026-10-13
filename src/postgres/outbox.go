package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nexus-commerce/user-service/src/publisher"
)

type Outbox struct {
	db *DB
}

func NewOutbox(db *DB) *Outbox {
	return &Outbox{db: db}
}

func (o *Outbox) Publish(ctx context.Context, msgs ...publisher.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	query := `INSERT INTO outbox (topic, key, value, headers)
		SELECT * FROM unnest($1::varchar[], $2::bytea[], $3::bytea[], $4::jsonb[])`

	var (
		topics  = make([]string, len(msgs))
		keys    = make([][]byte, len(msgs))
		values  = make([][]byte, len(msgs))
		headers = make([]string, len(msgs))
	)
	for i, m := range msgs {
		h, err := json.Marshal(m.Headers)
		if err != nil {
			return fmt.Errorf("marshal outbox headers: %w", err)
		}
		topics[i], keys[i], values[i], headers[i] = m.Topic, m.Key, m.Value, string(h)
	}

	_, err := o.db.querier(ctx).ExecContext(ctx, query, topics, keys, values, headers)
	if err != nil {
		return fmt.Errorf("insert outbox messages: %w", err)
	}
	return nil
}

func (o *Outbox) Dispatch(ctx context.Context, limit int, send func(ctx context.Context, msgs []publisher.Message) error) (int, error) {
	selectQuery := `SELECT id, topic, key, value, headers FROM outbox ORDER BY id LIMIT $1 FOR UPDATE SKIP LOCKED`
	deleteQuery := `DELETE FROM outbox WHERE id = ANY($1)`

	var n int
	err := o.db.WithinTx(ctx, func(ctx context.Context) error {
		q := o.db.querier(ctx)

		rows, err := q.QueryContext(ctx, selectQuery, limit)
		if err != nil {
			return fmt.Errorf("select outbox messages: %w", err)
		}
		defer rows.Close()

		var (
			ids  []int64
			msgs []publisher.Message
		)
		for rows.Next() {
			var (
				id      int64
				m       publisher.Message
				headers []byte
			)
			if err = rows.Scan(&id, &m.Topic, &m.Key, &m.Value, &headers); err != nil {
				return fmt.Errorf("scan outbox message: %w", err)
			}
			if len(headers) > 0 {
				if err = json.Unmarshal(headers, &m.Headers); err != nil {
					return fmt.Errorf("unmarshal outbox headers: %w", err)
				}
			}
			ids = append(ids, id)
			msgs = append(msgs, m)
		}
		if err = rows.Err(); err != nil {
			return fmt.Errorf("iterate outbox messages: %w", err)
		}
		if len(msgs) == 0 {
			return nil
		}

		if err = send(ctx, msgs); err != nil {
			return err
		}

		if _, err = q.ExecContext(ctx, deleteQuery, ids); err != nil {
			return fmt.Errorf("delete outbox messages: %w", err)
		}
		n = len(msgs)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}
