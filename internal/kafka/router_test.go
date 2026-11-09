package kafka

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/nexus-commerce/user-service/internal/messaging"
)

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
		t.Errorf("route() called the handler %d times, want 0", h.calls)
	}
}

func TestRouteSkipsPermanentFailure(t *testing.T) {
	h := &fakeHandler{typ: "known", errs: []error{fmt.Errorf("%w: bad payload", ErrPermanent)}}
	if err := mustRouter(t, h).route(t.Context(), messaging.Message{Type: "known"}); err != nil {
		t.Fatalf("route() error: %v", err)
	}
	if h.calls != 1 {
		t.Errorf("route() called the handler %d times, want 1", h.calls)
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
			t.Errorf("route() called the handler %d times, want 4", h.calls)
		}
		if got, want := time.Since(start), 700*time.Millisecond; got != want {
			t.Errorf("route() retried for %v, want %v", got, want)
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
			t.Errorf("route() retried for %v, want %v", got, want)
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
			t.Errorf("route() called the handler %d times, want 1", h.calls)
		}
	})
}
