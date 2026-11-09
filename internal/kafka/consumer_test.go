package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"testing/synctest"

	"github.com/google/go-cmp/cmp"
	"github.com/segmentio/kafka-go"

	"github.com/nexus-commerce/user-service/internal/messaging"
)

var discardLogger = slog.New(slog.DiscardHandler)

func TestToMessageReadsEventType(t *testing.T) {
	msg := toMessage(kafka.Message{
		Topic:   "users",
		Key:     []byte("key"),
		Value:   []byte("value"),
		Headers: []kafka.Header{{Key: "other", Value: []byte("x")}, {Key: messaging.HeaderEventType, Value: []byte("user.renamed")}},
	})
	want := messaging.Message{Type: "user.renamed", Topic: "users", Key: "key", Value: []byte("value")}
	if diff := cmp.Diff(want, msg); diff != "" {
		t.Errorf("toMessage() mismatch (-want +got):\n%s", diff)
	}
}

type fetchResult struct {
	msg kafka.Message
	err error
}

type fakeReader struct {
	fetches   []fetchResult
	commitErr []error
	committed []kafka.Message
	done      func()
}

func (r *fakeReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	if len(r.fetches) == 0 {
		r.done()
		<-ctx.Done()
		return kafka.Message{}, ctx.Err()
	}
	f := r.fetches[0]
	r.fetches = r.fetches[1:]
	return f.msg, f.err
}

func (r *fakeReader) CommitMessages(_ context.Context, msgs ...kafka.Message) error {
	if len(r.commitErr) > 0 {
		err := r.commitErr[0]
		r.commitErr = r.commitErr[1:]
		return err
	}
	r.committed = append(r.committed, msgs...)
	return nil
}

func (r *fakeReader) Close() error { return nil }

func confirmedKafkaMessage(offset int64) kafka.Message {
	return kafka.Message{
		Offset:  offset,
		Headers: []kafka.Header{{Key: messaging.HeaderEventType, Value: []byte("known")}},
	}
}

func TestRunRetriesTemporaryKafkaErrors(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		h := &fakeHandler{typ: "known"}
		reader := &fakeReader{
			fetches: []fetchResult{
				{err: kafka.NotCoordinatorForGroup},
				{err: fmt.Errorf("fetch: %w", kafka.GroupCoordinatorNotAvailable)},
				{msg: confirmedKafkaMessage(7)},
			},
			commitErr: []error{kafka.NotCoordinatorForGroup},
			done:      cancel,
		}
		c := &Consumer{
			reader: reader,
			router: mustRouter(t, h),
			retry:  retrier{logger: discardLogger, retryable: temporary},
			logger: discardLogger,
		}

		if err := c.Run(ctx); err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if h.calls != 1 {
			t.Errorf("handler called %d times, want 1", h.calls)
		}
		var offsets []int64
		for _, m := range reader.committed {
			offsets = append(offsets, m.Offset)
		}
		if diff := cmp.Diff([]int64{7}, offsets); diff != "" {
			t.Errorf("Run() committed offsets mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestRunStopsOnNonTemporaryFetchError(t *testing.T) {
	want := errors.New("broken reader")
	reader := &fakeReader{fetches: []fetchResult{{err: want}}, done: func() {}}
	c := &Consumer{
		reader: reader,
		router: mustRouter(t),
		retry:  retrier{logger: discardLogger, retryable: temporary},
		logger: discardLogger,
	}

	if err := c.Run(t.Context()); !errors.Is(err, want) {
		t.Fatalf("Run() error = %v, want %v", err, want)
	}
}
