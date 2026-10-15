package messaging

import (
	"encoding/json"
	"fmt"
	"time"
	"uuid"

	"github.com/nexus-commerce/user-service/src/entity"
	"github.com/nexus-commerce/user-service/src/usecase"
)

const typeUserRegistered = "user.registered"

type Topics struct {
	Users string
}

type Encoder struct {
	topics Topics
}

func NewEncoder(topics Topics) *Encoder {
	return &Encoder{topics: topics}
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

func (enc *Encoder) Encode(e entity.Event) (usecase.Message, error) {
	switch e := e.(type) {
	case entity.UserRegistered:
		spec := messageSpec{
			topic:     enc.topics.Users,
			typ:       typeUserRegistered,
			version:   "1.0",
			key:       e.UserID,
			timestamp: e.OccurredAt,
		}
		return encode(spec, userRegistered{
			UserID:    e.UserID,
			Email:     string(e.Profile.Email),
			FirstName: string(e.Profile.FirstName),
			LastName:  string(e.Profile.LastName),
			CreatedAt: e.OccurredAt,
		})
	default:
		return usecase.Message{}, fmt.Errorf("unknown event %T", e)
	}
}

type messageSpec struct {
	topic     string
	typ       string
	version   string
	key       string
	timestamp time.Time
}

func encode(spec messageSpec, data any) (usecase.Message, error) {
	value, err := json.Marshal(envelope{
		ID:        uuid.New().String(),
		Type:      spec.typ,
		Timestamp: spec.timestamp,
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
