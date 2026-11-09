package userevent

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"uuid"
)

var ErrMalformedMessage = errors.New("malformed message")

type incomingEnvelope struct {
	Type string         `json:"eventType"`
	Data jsontext.Value `json:"data"`
}

func DecodeRegistrationConfirmed(value []byte) (uuid.UUID, error) {
	var env incomingEnvelope
	if err := json.Unmarshal(value, &env); err != nil {
		return uuid.UUID{}, fmt.Errorf("%w: decode envelope: %w", ErrMalformedMessage, err)
	}
	if env.Type != TypeRegistrationConfirmed {
		return uuid.UUID{}, fmt.Errorf("%w: event type %q, want %q", ErrMalformedMessage, env.Type, TypeRegistrationConfirmed)
	}
	var payload registrationConfirmed
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		return uuid.UUID{}, fmt.Errorf("%w: decode %s data: %w", ErrMalformedMessage, TypeRegistrationConfirmed, err)
	}
	id, err := uuid.Parse(payload.UserID)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("%w: parse user id: %w", ErrMalformedMessage, err)
	}
	return id, nil
}
