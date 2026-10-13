package publisher

import (
	"context"
	"log"
	"time"
)

type Outbox interface {
	Dispatch(ctx context.Context, limit int, send func(ctx context.Context, msgs []Message) error) (int, error)
}

type RelayOptions struct {
	Interval  time.Duration
	BatchSize int
}

// Relay moves messages from the outbox to the producer. Delivery is
// at-least-once: a message is removed from the outbox only after the
// producer accepted it, so consumers must deduplicate by event ID.
type Relay struct {
	outbox   Outbox
	producer Producer
	opts     RelayOptions
}

func NewRelay(outbox Outbox, producer Producer, opts RelayOptions) *Relay {
	return &Relay{outbox: outbox, producer: producer, opts: opts}
}

func (r *Relay) Run(ctx context.Context) error {
	log.Println("Starting outbox relay")

	ticker := time.NewTicker(r.opts.Interval)
	defer ticker.Stop()

	for {
		r.drain(ctx)

		select {
		case <-ctx.Done():
			log.Println("Stopping outbox relay")
			return nil
		case <-ticker.C:
		}
	}
}

func (r *Relay) drain(ctx context.Context) {
	for ctx.Err() == nil {
		n, err := r.outbox.Dispatch(ctx, r.opts.BatchSize, func(ctx context.Context, msgs []Message) error {
			return r.producer.Publish(ctx, msgs...)
		})
		if err != nil {
			log.Printf("relay outbox messages: %v", err)
			return
		}
		if n < r.opts.BatchSize {
			return
		}
	}
}
