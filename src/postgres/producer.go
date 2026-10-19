package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nexus-commerce/user-service/src/messaging"
)

type outboxProducer struct {
	tx *sql.Tx
}

func (p outboxProducer) Produce(ctx context.Context, msgs ...messaging.Message) error {
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

	if _, err := p.tx.ExecContext(ctx, query, ids, topics, keys, values); err != nil {
		return fmt.Errorf("insert %d outbox messages: %w", len(msgs), err)
	}
	return nil
}
