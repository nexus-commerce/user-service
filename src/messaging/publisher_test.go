package messaging

import (
	"context"
	"slices"
	"testing"

	"github.com/nexus-commerce/user-service/src/user"
)

type fakeProducer struct {
	calls [][]Message
}

func (p *fakeProducer) Produce(_ context.Context, msgs ...Message) error {
	p.calls = append(p.calls, msgs)
	return nil
}

func TestPublishProducesEncodedMessages(t *testing.T) {
	pub := NewPublisher(fakeSealer{})
	fake := &fakeProducer{}
	first, second := newUserRegistered(t), newUserRegistered(t)

	if err := pub.Publish(t.Context(), fake, []user.Event{first, second}); err != nil {
		t.Fatalf("Publish = %v; want nil", err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("Produce called %d times; want 1", len(fake.calls))
	}
	msgs := fake.calls[0]
	got := make([]string, len(msgs))
	for i, m := range msgs {
		got[i] = m.ID
	}
	want := []string{first.ID.String(), second.ID.String()}
	if !slices.Equal(got, want) {
		t.Errorf("Publish produced message IDs %v, want %v", got, want)
	}
}

func TestPublishSkipsEmptyEvents(t *testing.T) {
	pub := NewPublisher(fakeSealer{})
	fake := &fakeProducer{}

	if err := pub.Publish(t.Context(), fake, nil); err != nil {
		t.Fatalf("Publish = %v; want nil", err)
	}
	if len(fake.calls) != 0 {
		t.Errorf("Produce called %d times; want 0", len(fake.calls))
	}
}

func TestPublishReturnsEncodeError(t *testing.T) {
	pub := NewPublisher(fakeSealer{})
	fake := &fakeProducer{}

	err := pub.Publish(t.Context(), fake, []user.Event{newUserRegistered(t), unroutedEvent{newUserRegistered(t)}})
	if err == nil {
		t.Fatal("Publish = nil; want error")
	}
	if len(fake.calls) != 0 {
		t.Errorf("Produce called %d times; want 0", len(fake.calls))
	}
}
