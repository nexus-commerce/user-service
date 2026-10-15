package relay

import (
	"context"
	"log"
	"time"

	"github.com/nexus-commerce/user-service/src/messaging"
)

type Outbox interface {
	ProcessBatch(ctx context.Context, limit int, fn func(ctx context.Context, msgs []messaging.Message) error) (int, error)
}

type Producer interface {
	Publish(ctx context.Context, msgs ...messaging.Message) error
}

type Options struct {
	Interval  time.Duration
	BatchSize int
}

type Relay struct {
	outbox   Outbox
	producer Producer
	opts     Options
}

func New(outbox Outbox, producer Producer, opts Options) *Relay {
	return &Relay{outbox: outbox, producer: producer, opts: opts}
}

func (r *Relay) Run(ctx context.Context) error {
	log.Println("Starting outbox relay")

	ticker := time.NewTicker(r.opts.Interval)
	defer ticker.Stop()

	for {
		r.flush(ctx)

		select {
		case <-ctx.Done():
			log.Println("Stopping outbox relay")
			return nil
		case <-ticker.C:
		}
	}
}

func (r *Relay) flush(ctx context.Context) {
	for ctx.Err() == nil {
		n, err := r.relayBatch(ctx)
		if err != nil {
			log.Printf("relay outbox messages: %v", err)
			return
		}
		if n < r.opts.BatchSize {
			return
		}
	}
}

func (r *Relay) relayBatch(ctx context.Context) (int, error) {
	return r.outbox.ProcessBatch(ctx, r.opts.BatchSize, func(ctx context.Context, msgs []messaging.Message) error {
		return r.producer.Publish(ctx, msgs...)
	})
}
