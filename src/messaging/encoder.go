package messaging

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nexus-commerce/user-service/src/entity"
)

const topicUsers = "users"

type Encoder struct {
	routes map[entity.EventType]route
}

type route struct {
	topic   string
	typ     string
	version string
	convert func(entity.Event) (meta, any, error)
}

type meta struct {
	id        string
	key       string
	timestamp time.Time
}

func NewEncoder() *Encoder {
	enc := &Encoder{routes: make(map[entity.EventType]route)}
	register(enc, topicUsers, "user.registered", "1.0", convertUserRegistered)
	return enc
}

func register[E entity.Event](enc *Encoder, topic, typ, version string, fn func(E) (meta, any)) {
	var zero E
	t := zero.EventType()
	if _, ok := enc.routes[t]; ok {
		panic(fmt.Sprintf("messaging: duplicate route for event %s", t))
	}
	enc.routes[t] = route{
		topic:   topic,
		typ:     typ,
		version: version,
		convert: func(e entity.Event) (meta, any, error) {
			ev, ok := e.(E)
			if !ok {
				return meta{}, nil, fmt.Errorf("event %s: unexpected type %T", t, e)
			}
			m, data := fn(ev)
			return m, data, nil
		},
	}
}

type envelope struct {
	ID        string    `json:"eventId"`
	Type      string    `json:"eventType"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Data      any       `json:"data"`
}

type userRegistered struct {
	UserID    string    `json:"userId"`
	Email     string    `json:"email"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	CreatedAt time.Time `json:"createdAt"`
}

func convertUserRegistered(e entity.UserRegistered) (meta, any) {
	m := meta{
		id:        e.ID.String(),
		key:       e.UserID.String(),
		timestamp: e.OccurredAt,
	}
	return m, userRegistered{
		UserID:    e.UserID.String(),
		Email:     e.Profile.Email().String(),
		FirstName: e.Profile.FirstName().String(),
		LastName:  e.Profile.LastName().String(),
		CreatedAt: e.OccurredAt,
	}
}

func (enc *Encoder) Encode(e entity.Event) (Message, error) {
	r, ok := enc.routes[e.EventType()]
	if !ok {
		return Message{}, fmt.Errorf("no route for event %s", e.EventType())
	}
	m, data, err := r.convert(e)
	if err != nil {
		return Message{}, err
	}
	value, err := json.Marshal(envelope{
		ID:        m.id,
		Type:      r.typ,
		Timestamp: m.timestamp,
		Version:   r.version,
		Data:      data,
	})
	if err != nil {
		return Message{}, fmt.Errorf("marshal %s event: %w", r.typ, err)
	}
	return Message{
		ID:    m.id,
		Topic: r.topic,
		Key:   m.key,
		Value: value,
	}, nil
}
