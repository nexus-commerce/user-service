package relay_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/nexus-commerce/user-service/internal/messaging"
	"github.com/nexus-commerce/user-service/internal/relay"
)

type fakeOutbox struct {
	pending []messaging.Message
	calls   int
	err     error
}

func (f *fakeOutbox) ProcessBatch(ctx context.Context, limit int, fn messaging.BatchFunc) (int, error) {
	f.calls++
	if f.err != nil {
		return 0, f.err
	}
	n := min(limit, len(f.pending))
	if n == 0 {
		return 0, nil
	}
	if err := fn(ctx, f.pending[:n]); err != nil {
		return 0, err
	}
	f.pending = f.pending[n:]
	return n, nil
}

type fakeProducer struct {
	produced []messaging.Message
	err      error
}

func (f *fakeProducer) Produce(_ context.Context, msgs ...messaging.Message) error {
	if f.err != nil {
		return f.err
	}
	f.produced = append(f.produced, msgs...)
	return nil
}

func newMessages(n int) []messaging.Message {
	msgs := make([]messaging.Message, n)
	for i := range msgs {
		id := strconv.Itoa(i)
		msgs[i] = messaging.Message{ID: id, Type: "user.test", Topic: "users", Key: id, Value: []byte(id)}
	}
	return msgs
}

func TestRunFlushesOnStartAndEveryInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		outbox, producer := &fakeOutbox{}, &fakeProducer{}
		opts := relay.Options{Interval: time.Second, BatchSize: 10}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error)

		go func() { done <- relay.NewWorker(outbox, producer, opts).Run(ctx) }()
		synctest.Wait()
		time.Sleep(opts.Interval)
		synctest.Wait()
		cancel()

		if err := <-done; err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if outbox.calls != 2 {
			t.Errorf("Run() processed batches %d times, want 2", outbox.calls)
		}
	})
}

func TestRunProducesPendingMessages(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		want := newMessages(3)
		outbox, producer := &fakeOutbox{pending: want}, &fakeProducer{}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error)

		go func() {
			done <- relay.NewWorker(outbox, producer, relay.Options{Interval: time.Hour, BatchSize: 10}).Run(ctx)
		}()
		synctest.Wait()
		cancel()

		if err := <-done; err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if diff := cmp.Diff(want, producer.produced); diff != "" {
			t.Errorf("Run() produced messages mismatch (-want +got):\n%s", diff)
		}
		if len(outbox.pending) != 0 {
			t.Errorf("Run() left %d messages in outbox, want 0", len(outbox.pending))
		}
	})
}

func TestRunDrainsFullBatchesWithoutWaiting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		want := newMessages(5)
		outbox, producer := &fakeOutbox{pending: want}, &fakeProducer{}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error)

		go func() {
			done <- relay.NewWorker(outbox, producer, relay.Options{Interval: time.Hour, BatchSize: 2}).Run(ctx)
		}()
		synctest.Wait()
		cancel()

		if err := <-done; err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if diff := cmp.Diff(want, producer.produced); diff != "" {
			t.Errorf("Run() produced messages mismatch (-want +got):\n%s", diff)
		}
		if outbox.calls != 3 {
			t.Errorf("Run() processed batches %d times, want 3 (2+2+1)", outbox.calls)
		}
	})
}

func TestRunContinuesAfterOutboxError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		outbox, producer := &fakeOutbox{err: errors.New("boom")}, &fakeProducer{}
		opts := relay.Options{Interval: time.Second, BatchSize: 10}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error)

		go func() { done <- relay.NewWorker(outbox, producer, opts).Run(ctx) }()
		synctest.Wait()
		time.Sleep(opts.Interval)
		synctest.Wait()
		cancel()

		if err := <-done; err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if outbox.calls != 2 {
			t.Errorf("Run() processed batches %d times, want 2 (one per tick, no retry loop)", outbox.calls)
		}
	})
}

func TestRunKeepsMessagesWhenProducerFails(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		outbox, producer := &fakeOutbox{pending: newMessages(3)}, &fakeProducer{err: errors.New("kafka down")}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error)

		go func() {
			done <- relay.NewWorker(outbox, producer, relay.Options{Interval: time.Hour, BatchSize: 10}).Run(ctx)
		}()
		synctest.Wait()
		cancel()

		if err := <-done; err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if len(outbox.pending) != 3 {
			t.Errorf("Run() left %d messages in outbox, want 3", len(outbox.pending))
		}
		if outbox.calls != 1 {
			t.Errorf("Run() processed batches %d times, want 1", outbox.calls)
		}
	})
}

func TestRunStopsOnCancelledContext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		outbox, producer := &fakeOutbox{pending: newMessages(5)}, &fakeProducer{}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := relay.NewWorker(outbox, producer, relay.Options{Interval: time.Hour, BatchSize: 1}).Run(ctx)

		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if outbox.calls != 0 {
			t.Errorf("Run() processed batches %d times, want 0", outbox.calls)
		}
	})
}
