package userevent

import (
	"errors"
	"testing"

	"github.com/nexus-commerce/user-service/internal/user"
)

func TestDecodeRegistrationConfirmedRoundTrip(t *testing.T) {
	e := user.RegistrationConfirmed{EventMeta: newMeta()}
	msg, err := mustEncoder(t, fakeSealer{}).encode(e)
	if err != nil {
		t.Fatalf("encode() error: %v", err)
	}

	id, err := DecodeRegistrationConfirmed(msg.Value)
	if err != nil {
		t.Fatalf("DecodeRegistrationConfirmed() error: %v", err)
	}
	if id != e.AggregateID {
		t.Errorf("DecodeRegistrationConfirmed() = %v, want %v", id, e.AggregateID)
	}
}

func TestDecodeRegistrationConfirmedRejectsMalformed(t *testing.T) {
	renamed, err := mustEncoder(t, fakeSealer{}).encode(user.Renamed{
		EventMeta: newMeta(),
		FullName:  user.NewFullName(mustName(t, "Jane"), mustName(t, "Doe")),
	})
	if err != nil {
		t.Fatalf("encode() error: %v", err)
	}
	for name, value := range map[string][]byte{
		"not json":   []byte("{"),
		"other type": renamed.Value,
		"bad data":   []byte(`{"eventType":"user.registration_confirmed","data":42}`),
		"bad id":     []byte(`{"eventType":"user.registration_confirmed","data":{"userId":"nope"}}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeRegistrationConfirmed(value); !errors.Is(err, ErrMalformedMessage) {
				t.Errorf("DecodeRegistrationConfirmed() error = %v, want %v", err, ErrMalformedMessage)
			}
		})
	}
}
