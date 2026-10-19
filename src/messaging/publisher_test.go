package messaging

import (
	"context"
	"testing"

	"github.com/nexus-commerce/user-service/src/entity"
)

type fakeProducer struct {
	calls [][]Message
}

func (p *fakeProducer) Produce(_ context.Context, msgs ...Message) error {
	p.calls = append(p.calls, msgs)
	return nil
}

func TestPublishProducesEncodedMessages(t *testing.T) {
	pub := NewPublisher()
	producer := &fakeProducer{}
	first, second := newUserRegistered(t), newUserRegistered(t)

	if err := pub.Publish(context.Background(), producer, []entity.Event{first, second}); err != nil {
		t.Fatalf("Publish = %v; want nil", err)
	}
	if len(producer.calls) != 1 {
		t.Fatalf("Produce called %d times; want 1", len(producer.calls))
	}
	msgs := producer.calls[0]
	if len(msgs) != 2 || msgs[0].ID != first.ID.String() || msgs[1].ID != second.ID.String() {
		t.Errorf("messages = %+v", msgs)
	}
}

func TestPublishSkipsEmptyEvents(t *testing.T) {
	pub := NewPublisher()
	producer := &fakeProducer{}

	if err := pub.Publish(context.Background(), producer, nil); err != nil {
		t.Fatalf("Publish = %v; want nil", err)
	}
	if len(producer.calls) != 0 {
		t.Errorf("Produce called %d times; want 0", len(producer.calls))
	}
}

func TestPublishReturnsEncodeError(t *testing.T) {
	pub := NewPublisher()
	producer := &fakeProducer{}

	err := pub.Publish(context.Background(), producer, []entity.Event{newUserRegistered(t), unknownEvent{}})
	if err == nil {
		t.Fatal("Publish = nil; want error")
	}
	if len(producer.calls) != 0 {
		t.Errorf("Produce called %d times; want 0", len(producer.calls))
	}
}
