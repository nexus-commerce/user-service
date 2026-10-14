package relay

import (
	"context"
	"log"
	"time"
)

type Messages interface {
	Execute(ctx context.Context, limit int) (int, error)
}

type Options struct {
	Interval  time.Duration
	BatchSize int
}

type Relay struct {
	messages Messages
	opts     Options
}

func New(messages Messages, opts Options) *Relay {
	return &Relay{messages: messages, opts: opts}
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
		n, err := r.messages.Execute(ctx, r.opts.BatchSize)
		if err != nil {
			log.Printf("relay outbox messages: %v", err)
			return
		}
		if n < r.opts.BatchSize {
			return
		}
	}
}
