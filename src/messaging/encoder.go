package messaging

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nexus-commerce/user-service/src/entity"
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

func (enc *Encoder) Encode(e entity.Event) (Message, bool, error) {
	var (
		spec messageSpec
		data any
	)
	switch e := e.(type) {
	case entity.UserRegistered:
		spec = messageSpec{
			id:        e.ID.String(),
			topic:     enc.topics.Users,
			typ:       typeUserRegistered,
			version:   "1.0",
			key:       e.UserID.String(),
			timestamp: e.OccurredAt,
		}
		data = userRegistered{
			UserID:    e.UserID.String(),
			Email:     e.Profile.Email().String(),
			FirstName: e.Profile.FirstName().String(),
			LastName:  e.Profile.LastName().String(),
			CreatedAt: e.OccurredAt,
		}
	default:
		return Message{}, false, nil
	}

	msg, err := encode(spec, data)
	if err != nil {
		return Message{}, false, err
	}
	return msg, true, nil
}

type messageSpec struct {
	id        string
	topic     string
	typ       string
	version   string
	key       string
	timestamp time.Time
}

func encode(spec messageSpec, data any) (Message, error) {
	value, err := json.Marshal(envelope{
		ID:        spec.id,
		Type:      spec.typ,
		Timestamp: spec.timestamp,
		Version:   spec.version,
		Data:      data,
	})
	if err != nil {
		return Message{}, fmt.Errorf("marshal %s event: %w", spec.typ, err)
	}
	return Message{
		ID:    spec.id,
		Topic: spec.topic,
		Key:   spec.key,
		Value: value,
	}, nil
}
