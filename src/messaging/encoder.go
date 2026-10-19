package messaging

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nexus-commerce/user-service/src/entity"
)

const (
	topicUsers = "users"

	typeUserRegistered = "user.registered"
)

type route struct {
	event   entity.EventType
	topic   string
	typ     string
	version string
	convert func(entity.Event) (any, error)
}

func newRoute[E entity.Event](topic, typ, version string, fn func(E) any) route {
	var zero E
	t := zero.Type()
	return route{
		event:   t,
		topic:   topic,
		typ:     typ,
		version: version,
		convert: func(e entity.Event) (any, error) {
			ev, ok := e.(E)
			if !ok {
				return nil, fmt.Errorf("event %s: unexpected type %T", t, e)
			}
			return fn(ev), nil
		},
	}
}

type encoder struct {
	routes map[entity.EventType]route
}

func newEncoder() *encoder {
	enc := &encoder{routes: make(map[entity.EventType]route)}
	enc.register(newRoute(topicUsers, typeUserRegistered, "1.0", convertUserRegistered))
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
	r, ok := enc.routes[e.Type()]
	if !ok {
		return Message{}, fmt.Errorf("no route for event %s", e.Type())
	}
	data, err := r.convert(e)
	if err != nil {
		return Message{}, err
	}
	meta := e.Meta()
	value, err := json.Marshal(envelope{
		ID:        meta.ID.String(),
		Type:      r.typ,
		Timestamp: meta.OccurredAt,
		Version:   r.version,
		Data:      data,
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
