package userevent

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/nexus-commerce/user-service/internal/messaging"
	"github.com/nexus-commerce/user-service/internal/user"
)

const (
	TopicUsers                = "users"
	TopicUsersInternal        = "users.internal"
	TypeRegistrationConfirmed = "user.registration_confirmed"
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

func newEncoder(sealer Sealer) (*encoder, error) {
	enc := &encoder{routes: make(map[reflect.Type]route), sealer: sealer}
	routes := []route{
		newRoute(destination{
			topic:   TopicUsers,
			typ:     "user.registered",
			version: "1.0",
		}, convertRegistered),
		newRoute(destination{
			topic:   TopicUsers,
			typ:     "user.registration_requested",
			version: "1.0",
		}, enc.convertRegistrationRequested),
		newRoute(destination{
			topic:   TopicUsersInternal,
			typ:     TypeRegistrationConfirmed,
			version: "1.0",
		}, convertRegistrationConfirmed),
		newRoute(destination{
			topic:   TopicUsers,
			typ:     "user.registration_rejected",
			version: "1.0",
		}, convertRegistrationRejected),
		newRoute(destination{
			topic:   TopicUsers,
			typ:     "user.duplicate_registration_attempted",
			version: "1.0",
		}, convertDuplicateRegistrationAttempted),
		newRoute(destination{
			topic:   TopicUsers,
			typ:     "user.renamed",
			version: "1.0",
		}, convertRenamed),
		newRoute(destination{
			topic:   TopicUsers,
			typ:     "user.email_change_requested",
			version: "1.0",
		}, enc.convertEmailChangeRequested),
		newRoute(destination{
			topic:   TopicUsers,
			typ:     "user.email_change_to_taken_address_attempted",
			version: "1.0",
		}, convertEmailChangeToTakenAddressAttempted),
		newRoute(destination{
			topic:   TopicUsers,
			typ:     "user.email_changed",
			version: "1.0",
		}, convertEmailChanged),
		newRoute(destination{
			topic:   TopicUsers,
			typ:     "user.email_change_rejected",
			version: "1.0",
		}, convertEmailChangeRejected),
	}
	for _, r := range routes {
		if err := enc.register(r); err != nil {
			return nil, err
		}
	}
	return enc, nil
}

func (enc *encoder) register(r route) error {
	if _, ok := enc.routes[r.event]; ok {
		return fmt.Errorf("duplicate route for event %s", r.event)
	}
	enc.routes[r.event] = r
	return nil
}

type envelope struct {
	ID        string    `json:"eventId"`
	Type      string    `json:"eventType"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Data      any       `json:"data"`
}

func (enc *encoder) encode(e user.Event) (messaging.Message, error) {
	r, ok := enc.routes[reflect.TypeOf(e)]
	if !ok {
		return messaging.Message{}, fmt.Errorf("no route for event %T", e)
	}
	meta := e.Meta()
	data, err := r.convert(e)
	if err != nil {
		return messaging.Message{}, fmt.Errorf("convert %s event: %w", r.typ, err)
	}
	value, err := json.Marshal(envelope{
		ID:        meta.ID.String(),
		Type:      r.typ,
		Timestamp: meta.OccurredAt,
		Version:   r.version,
		Data:      data,
	})
	if err != nil {
		return messaging.Message{}, fmt.Errorf("marshal %s event: %w", r.typ, err)
	}
	return messaging.Message{
		ID:    meta.ID.String(),
		Type:  r.typ,
		Topic: r.topic,
		Key:   meta.AggregateID.String(),
		Value: value,
	}, nil
}

type registered struct {
	UserID    string    `json:"userId"`
	Email     string    `json:"email"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	CreatedAt time.Time `json:"createdAt"`
}

func convertRegistered(e user.Registered) (any, error) {
	return registered{
		UserID:    e.AggregateID.String(),
		Email:     e.Email.String(),
		FirstName: e.FullName.FirstName().String(),
		LastName:  e.FullName.LastName().String(),
		CreatedAt: e.OccurredAt,
	}, nil
}

type registrationRequested struct {
	UserID         string    `json:"userId"`
	Email          string    `json:"email"`
	FirstName      string    `json:"firstName"`
	LastName       string    `json:"lastName"`
	EncryptedToken []byte    `json:"encryptedToken"`
	KeyID          string    `json:"keyId"`
	ExpiresAt      time.Time `json:"expiresAt"`
	RequestedAt    time.Time `json:"requestedAt"`
}

func (enc *encoder) convertRegistrationRequested(e user.RegistrationRequested) (any, error) {
	sealed, err := enc.sealer.Seal([]byte(e.Token.Plaintext()))
	if err != nil {
		return nil, fmt.Errorf("seal registration token: %w", err)
	}
	return registrationRequested{
		UserID:         e.AggregateID.String(),
		Email:          e.Email.String(),
		FirstName:      e.FullName.FirstName().String(),
		LastName:       e.FullName.LastName().String(),
		EncryptedToken: sealed,
		KeyID:          enc.sealer.KeyID(),
		ExpiresAt:      e.ExpiresAt,
		RequestedAt:    e.OccurredAt,
	}, nil
}

type registrationConfirmed struct {
	UserID      string    `json:"userId"`
	ConfirmedAt time.Time `json:"confirmedAt"`
}

func convertRegistrationConfirmed(e user.RegistrationConfirmed) (any, error) {
	return registrationConfirmed{
		UserID:      e.AggregateID.String(),
		ConfirmedAt: e.OccurredAt,
	}, nil
}

type registrationRejected struct {
	UserID     string    `json:"userId"`
	Email      string    `json:"email"`
	RejectedAt time.Time `json:"rejectedAt"`
}

func convertRegistrationRejected(e user.RegistrationRejected) (any, error) {
	return registrationRejected{
		UserID:     e.AggregateID.String(),
		Email:      e.Email.String(),
		RejectedAt: e.OccurredAt,
	}, nil
}

type duplicateRegistrationAttempted struct {
	UserID      string    `json:"userId"`
	FirstName   string    `json:"firstName"`
	LastName    string    `json:"lastName"`
	Email       string    `json:"email"`
	AttemptedAt time.Time `json:"attemptedAt"`
}

func convertDuplicateRegistrationAttempted(e user.DuplicateRegistrationAttempted) (any, error) {
	return duplicateRegistrationAttempted{
		UserID:      e.AggregateID.String(),
		FirstName:   e.FullName.FirstName().String(),
		LastName:    e.FullName.LastName().String(),
		Email:       e.Email.String(),
		AttemptedAt: e.OccurredAt,
	}, nil
}

type renamed struct {
	UserID    string    `json:"userId"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	RenamedAt time.Time `json:"renamedAt"`
}

func convertRenamed(e user.Renamed) (any, error) {
	return renamed{
		UserID:    e.AggregateID.String(),
		FirstName: e.FullName.FirstName().String(),
		LastName:  e.FullName.LastName().String(),
		RenamedAt: e.OccurredAt,
	}, nil
}

type emailChangeRequested struct {
	UserID         string    `json:"userId"`
	FirstName      string    `json:"firstName"`
	LastName       string    `json:"lastName"`
	NewEmail       string    `json:"newEmail"`
	EncryptedToken []byte    `json:"encryptedToken"`
	KeyID          string    `json:"keyId"`
	ExpiresAt      time.Time `json:"expiresAt"`
	RequestedAt    time.Time `json:"requestedAt"`
}

func (enc *encoder) convertEmailChangeRequested(e user.EmailChangeRequested) (any, error) {
	sealed, err := enc.sealer.Seal([]byte(e.Token.Plaintext()))
	if err != nil {
		return nil, fmt.Errorf("seal email change token: %w", err)
	}
	return emailChangeRequested{
		UserID:         e.AggregateID.String(),
		FirstName:      e.FullName.FirstName().String(),
		LastName:       e.FullName.LastName().String(),
		NewEmail:       e.NewEmail.String(),
		EncryptedToken: sealed,
		KeyID:          enc.sealer.KeyID(),
		ExpiresAt:      e.ExpiresAt,
		RequestedAt:    e.OccurredAt,
	}, nil
}

type emailChangeToTakenAddressAttempted struct {
	UserID      string    `json:"userId"`
	NewEmail    string    `json:"newEmail"`
	AttemptedAt time.Time `json:"attemptedAt"`
}

func convertEmailChangeToTakenAddressAttempted(e user.EmailChangeToTakenAddressAttempted) (any, error) {
	return emailChangeToTakenAddressAttempted{
		UserID:      e.AggregateID.String(),
		NewEmail:    e.NewEmail.String(),
		AttemptedAt: e.OccurredAt,
	}, nil
}

type emailChanged struct {
	UserID    string    `json:"userId"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	OldEmail  string    `json:"oldEmail"`
	NewEmail  string    `json:"newEmail"`
	ChangedAt time.Time `json:"changedAt"`
}

func convertEmailChanged(e user.EmailChanged) (any, error) {
	return emailChanged{
		UserID:    e.AggregateID.String(),
		FirstName: e.FullName.FirstName().String(),
		LastName:  e.FullName.LastName().String(),
		OldEmail:  e.OldEmail.String(),
		NewEmail:  e.NewEmail.String(),
		ChangedAt: e.OccurredAt,
	}, nil
}

type emailChangeRejected struct {
	UserID     string    `json:"userId"`
	NewEmail   string    `json:"newEmail"`
	RejectedAt time.Time `json:"rejectedAt"`
}

func convertEmailChangeRejected(e user.EmailChangeRejected) (any, error) {
	return emailChangeRejected{
		UserID:     e.AggregateID.String(),
		NewEmail:   e.NewEmail.String(),
		RejectedAt: e.OccurredAt,
	}, nil
}
