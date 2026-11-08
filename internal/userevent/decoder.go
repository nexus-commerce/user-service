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

func decodeData[T any](value []byte, typ string) (T, error) {
	var data T
	var env incomingEnvelope
	if err := json.Unmarshal(value, &env); err != nil {
		return data, fmt.Errorf("%w: decode envelope: %w", ErrMalformedMessage, err)
	}
	if env.Type != typ {
		return data, fmt.Errorf("%w: event type %q, want %q", ErrMalformedMessage, env.Type, typ)
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return data, fmt.Errorf("%w: decode %s data: %w", ErrMalformedMessage, typ, err)
	}
	return data, nil
}

func DecodeRegistrationConfirmed(value []byte) (uuid.UUID, error) {
	data, err := decodeData[userRegistrationConfirmed](value, TypeRegistrationConfirmed)
	if err != nil {
		return uuid.UUID{}, err
	}
	id, err := uuid.Parse(data.UserID)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("%w: parse user id: %w", ErrMalformedMessage, err)
	}
	return id, nil
}
