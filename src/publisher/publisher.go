package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nexus-commerce/user-service/src/entity"
)

const userRegistered eventType = "user.registered"

type Topics struct {
	Users string
}

type Events struct {
	producer Producer
	topics   Topics
}

func NewEvents(p Producer, topics Topics) *Events {
	return &Events{producer: p, topics: topics}
}

type registeredUser struct {
	ID        string    `json:"userId"`
	Email     string    `json:"email"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	CreatedAt time.Time `json:"createdAt"`
}

func toRegisteredUser(u *entity.User) registeredUser {
	return registeredUser{
		ID:        u.ID,
		Email:     string(u.Email),
		FirstName: string(u.FirstName),
		LastName:  string(u.LastName),
		CreatedAt: u.CreatedAt,
	}
}

func (e *Events) UserRegistered(ctx context.Context, user *entity.User) error {
	b, err := json.Marshal(newEvent(userRegistered, "1.0", toRegisteredUser(user)))
	if err != nil {
		return fmt.Errorf("marshal %s event: %w", userRegistered, err)
	}
	return e.producer.Publish(ctx, Message{
		Topic: e.topics.Users,
		Key:   []byte(user.ID),
		Value: b,
	})
}
