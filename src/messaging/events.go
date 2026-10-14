package messaging

import (
	"encoding/json"
	"fmt"
	"time"
	"uuid"

	"github.com/nexus-commerce/user-service/src/entity"
	"github.com/nexus-commerce/user-service/src/usecase"
)

// The topic and event type names are part of the public contract
// documented in src/docs/asyncapi.yaml.
const (
	topicUsers         = "users"
	typeUserRegistered = "user.registered"
)

type Events struct{}

func NewEvents() *Events {
	return &Events{}
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

func (*Events) UserRegistered(u *entity.User) (usecase.Message, error) {
	spec := messageSpec{
		topic:   topicUsers,
		typ:     typeUserRegistered,
		version: "1.0",
		key:     u.ID,
	}
	return encode(spec, userRegistered{
		UserID:    u.ID,
		Email:     string(u.Profile.Email),
		FirstName: string(u.Profile.FirstName),
		LastName:  string(u.Profile.LastName),
		CreatedAt: u.CreatedAt,
	})
}

type messageSpec struct {
	topic   string
	typ     string
	version string
	key     string
}

func encode(spec messageSpec, data any) (usecase.Message, error) {
	value, err := json.Marshal(envelope{
		ID:        uuid.New().String(),
		Type:      spec.typ,
		Timestamp: time.Now().UTC(),
		Version:   spec.version,
		Data:      data,
	})
	if err != nil {
		return usecase.Message{}, fmt.Errorf("marshal %s event: %w", spec.typ, err)
	}
	return usecase.Message{
		Topic: spec.topic,
		Key:   spec.key,
		Value: value,
	}, nil
}
