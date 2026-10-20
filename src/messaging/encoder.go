package messaging

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/nexus-commerce/user-service/src/user"
)

const (
	topicUsers = "users"

	typeUserRegistered           = "user.registered"
	typeUserRenamed              = "user.renamed"
	typeUserEmailChangeRequested = "user.email_change_requested"
	typeUserEmailChanged         = "user.email_changed"
)

type Sealer interface {
	Seal(plaintext []byte) ([]byte, error)
	KeyID() string
}

type destination struct {
	topic   string
	typ     string
	version string
}

type route struct {
	destination
	event   reflect.Type
	convert func(user.Event) (any, error)
}

func newRoute[E user.Event](dst destination, fn func(E) (any, error)) route {
	return route{
		destination: dst,
		event:       reflect.TypeFor[E](),
		convert:     func(e user.Event) (any, error) { return fn(e.(E)) },
	}
}

type encoder struct {
	routes map[reflect.Type]route
	sealer Sealer
}

func newEncoder(sealer Sealer) *encoder {
	enc := &encoder{routes: make(map[reflect.Type]route), sealer: sealer}
	enc.register(newRoute(destination{
		topic:   topicUsers,
		typ:     typeUserRegistered,
		version: "1.0",
	}, convertRegistered))
	enc.register(newRoute(destination{
		topic:   topicUsers,
		typ:     typeUserRenamed,
		version: "1.0",
	}, convertRenamed))
	enc.register(newRoute(destination{
		topic:   topicUsers,
		typ:     typeUserEmailChangeRequested,
		version: "1.0",
	}, enc.convertUserEmailChangeRequested))
	enc.register(newRoute(destination{
		topic:   topicUsers,
		typ:     typeUserEmailChanged,
		version: "1.0",
	}, convertEmailChanged))
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

func (enc *encoder) encode(e user.Event) (Message, error) {
	r, ok := enc.routes[reflect.TypeOf(e)]
	if !ok {
		return Message{}, fmt.Errorf("no route for event %T", e)
	}
	meta := e.Meta()
	data, err := r.convert(e)
	if err != nil {
		return Message{}, fmt.Errorf("convert %s event: %w", r.typ, err)
	}
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

func convertRegistered(e user.Registered) (any, error) {
	return userRegistered{
		UserID:    e.AggregateID.String(),
		Email:     e.Profile.Email().String(),
		FirstName: e.Profile.FirstName().String(),
		LastName:  e.Profile.LastName().String(),
		CreatedAt: e.OccurredAt,
	}, nil
}

type userRenamed struct {
	UserID    string    `json:"userId"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	RenamedAt time.Time `json:"renamedAt"`
}

func convertRenamed(e user.Renamed) (any, error) {
	return userRenamed{
		UserID:    e.AggregateID.String(),
		FirstName: e.FirstName.String(),
		LastName:  e.LastName.String(),
		RenamedAt: e.OccurredAt,
	}, nil
}

type userEmailChangeRequested struct {
	UserID         string    `json:"userId"`
	NewEmail       string    `json:"newEmail"`
	EncryptedToken []byte    `json:"encryptedToken"`
	KeyID          string    `json:"keyId"`
	ExpiresAt      time.Time `json:"expiresAt"`
	RequestedAt    time.Time `json:"requestedAt"`
}

func (enc *encoder) convertUserEmailChangeRequested(e user.EmailChangeRequested) (any, error) {
	sealed, err := enc.sealer.Seal([]byte(e.Token.Plaintext()))
	if err != nil {
		return nil, fmt.Errorf("seal email change token: %w", err)
	}
	return userEmailChangeRequested{
		UserID:         e.AggregateID.String(),
		NewEmail:       e.NewEmail.String(),
		EncryptedToken: sealed,
		KeyID:          enc.sealer.KeyID(),
		ExpiresAt:      e.ExpiresAt,
		RequestedAt:    e.OccurredAt,
	}, nil
}

type userEmailChanged struct {
	UserID    string    `json:"userId"`
	OldEmail  string    `json:"oldEmail"`
	NewEmail  string    `json:"newEmail"`
	ChangedAt time.Time `json:"changedAt"`
}

func convertEmailChanged(e user.EmailChanged) (any, error) {
	return userEmailChanged{
		UserID:    e.AggregateID.String(),
		OldEmail:  e.OldEmail.String(),
		NewEmail:  e.NewEmail.String(),
		ChangedAt: e.OccurredAt,
	}, nil
}
