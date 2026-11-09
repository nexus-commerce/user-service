package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/nexus-commerce/user-service/internal/messaging"
)

var discardLogger = slog.New(slog.DiscardHandler)

type fakeHandler struct {
	typ   string
	errs  []error
	calls int
}

func (h *fakeHandler) EventType() string { return h.typ }

func (h *fakeHandler) Handle(context.Context, messaging.Message) error {
	h.calls++
	if len(h.errs) == 0 {
		return nil
	}
	err := h.errs[0]
	h.errs = h.errs[1:]
	return err
}

func mustRouter(t *testing.T, handlers ...Handler) router {
	t.Helper()
	r, err := newRouter(discardLogger, handlers...)
	if err != nil {
		t.Fatalf("newRouter() error: %v", err)
	}
	return r
}

func TestNewRouterRejectsDuplicateHandlers(t *testing.T) {
	if _, err := newRouter(discardLogger, &fakeHandler{typ: "a"}, &fakeHandler{typ: "a"}); err == nil {
		t.Error("newRouter(duplicate) error = nil, want error")
	}
}

func TestRouteSkipsUnknownType(t *testing.T) {
	h := &fakeHandler{typ: "known"}
	if err := mustRouter(t, h).route(t.Context(), messaging.Message{Type: "unknown"}); err != nil {
		t.Fatalf("route() error: %v", err)
	}
	if h.calls != 0 {
		t.Errorf("handler called %d times, want 0", h.calls)
	}
}

func TestRouteSkipsPermanentFailure(t *testing.T) {
	h := &fakeHandler{typ: "known", errs: []error{fmt.Errorf("%w: bad payload", ErrPermanent)}}
	if err := mustRouter(t, h).route(t.Context(), messaging.Message{Type: "known"}); err != nil {
		t.Fatalf("route() error: %v", err)
	}
	if h.calls != 1 {
		t.Errorf("handler called %d times, want 1", h.calls)
	}
}

func TestRouteRetriesWithBackoff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		transient := errors.New("db unavailable")
		h := &fakeHandler{typ: "known", errs: []error{transient, transient, transient}}
		start := time.Now()

		if err := mustRouter(t, h).route(t.Context(), messaging.Message{Type: "known"}); err != nil {
			t.Fatalf("route() error: %v", err)
		}
		if h.calls != 4 {
			t.Errorf("handler called %d times, want 4", h.calls)
		}
		if got, want := time.Since(start), 700*time.Millisecond; got != want {
			t.Errorf("retried for %v, want %v", got, want)
		}
	})
}

func TestRouteCapsBackoff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		transient := errors.New("db unavailable")
		errs := make([]error, 12)
		for i := range errs {
			errs[i] = transient
		}
		h := &fakeHandler{typ: "known", errs: errs}
		start := time.Now()

		if err := mustRouter(t, h).route(t.Context(), messaging.Message{Type: "known"}); err != nil {
			t.Fatalf("route() error: %v", err)
		}
		want := time.Duration(0)
		for delay, i := minRetryDelay, 0; i < len(errs); i++ {
			want += delay
			delay = min(delay*2, maxRetryDelay)
		}
		if got := time.Since(start); got != want {
			t.Errorf("retried for %v, want %v", got, want)
		}
	})
}

func TestRouteStopsOnCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		h := &fakeHandler{typ: "known", errs: []error{errors.New("db unavailable")}}
		time.AfterFunc(50*time.Millisecond, cancel)

		if err := mustRouter(t, h).route(ctx, messaging.Message{Type: "known"}); !errors.Is(err, context.Canceled) {
			t.Fatalf("route() error = %v, want %v", err, context.Canceled)
		}
		if h.calls != 1 {
			t.Errorf("handler called %d times, want 1", h.calls)
		}
	})
}

func TestToMessageReadsEventType(t *testing.T) {
	msg := toMessage(kafka.Message{
		Topic:   "users",
		Key:     []byte("key"),
		Value:   []byte("value"),
		Headers: []kafka.Header{{Key: "other", Value: []byte("x")}, {Key: messaging.HeaderEventType, Value: []byte("user.renamed")}},
	})
	want := messaging.Message{Type: "user.renamed", Topic: "users", Key: "key", Value: []byte("value")}
	if msg.Type != want.Type || msg.Topic != want.Topic || msg.Key != want.Key || string(msg.Value) != string(want.Value) {
		t.Errorf("toMessage() = %+v, want %+v", msg, want)
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
		c := &Consumer{reader: reader, router: mustRouter(t, h), logger: discardLogger}

		if err := c.Run(ctx); err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if h.calls != 1 {
			t.Errorf("handler called %d times, want 1", h.calls)
		}
		if len(reader.committed) != 1 || reader.committed[0].Offset != 7 {
			t.Errorf("committed %+v, want offset 7", reader.committed)
		}
	})
}

func TestRunStopsOnNonTemporaryFetchError(t *testing.T) {
	want := errors.New("broken reader")
	reader := &fakeReader{fetches: []fetchResult{{err: want}}, done: func() {}}
	c := &Consumer{reader: reader, router: mustRouter(t), logger: discardLogger}

	if err := c.Run(t.Context()); !errors.Is(err, want) {
		t.Fatalf("Run() error = %v, want %v", err, want)
	}
}
