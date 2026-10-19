package messaging

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/nexus-commerce/user-service/src/entity"
)

const (
	topicUsers = "users"

	typeUserRegistered = "user.registered"
)

type destination struct {
	topic   string
	typ     string
	version string
}

type route struct {
	destination
	event   reflect.Type
	convert func(entity.Event) any
}

func newRoute[E entity.Event](dst destination, fn func(E) any) route {
	return route{
		destination: dst,
		event:       reflect.TypeFor[E](),
		convert:     func(e entity.Event) any { return fn(e.(E)) },
	}
}

type encoder struct {
	routes map[reflect.Type]route
}

func newEncoder() *encoder {
	enc := &encoder{routes: make(map[reflect.Type]route)}
	enc.register(newRoute(destination{
		topic:   topicUsers,
		typ:     typeUserRegistered,
		version: "1.0",
	}, convertUserRegistered))
	return enc
}

func (enc *encoder) register(r route) {
	if _, ok := enc.routes[r.event]; ok {
		panic(fmt.Sprintf("messaging: duplicate route for event %s", r.event))
	}
	enc.routes[r.event] = r
}

type envelope struct {
	ID        string    `json:"eventId"`
	Type      string    `json:"eventType"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Data      any       `json:"data"`
}

func (enc *encoder) encode(e entity.Event) (Message, error) {
	r, ok := enc.routes[reflect.TypeOf(e)]
	if !ok {
		return Message{}, fmt.Errorf("no route for event %T", e)
	}
	meta := e.Meta()
	value, err := json.Marshal(envelope{
		ID:        meta.ID.String(),
		Type:      r.typ,
		Timestamp: meta.OccurredAt,
		Version:   r.version,
		Data:      r.convert(e),
	})
	if err != nil {
		return Message{}, fmt.Errorf("marshal %s event: %w", r.typ, err)
	}
	return Message{
		ID:    meta.ID.String(),
		Topic: r.topic,
		Key:   meta.AggregateID.String(),
		Value: value,
	}, nil
}

type userRegistered struct {
	UserID    string    `json:"userId"`
	Email     string    `json:"email"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	CreatedAt time.Time `json:"createdAt"`
}

func convertUserRegistered(e entity.UserRegistered) any {
	return userRegistered{
		UserID:    e.AggregateID.String(),
		Email:     e.Profile.Email().String(),
		FirstName: e.Profile.FirstName().String(),
		LastName:  e.Profile.LastName().String(),
		CreatedAt: e.OccurredAt,
	}
}
