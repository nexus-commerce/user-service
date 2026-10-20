package nacl

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"

	"golang.org/x/crypto/nacl/box"
)

func TestSealerRoundTrip(t *testing.T) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	s, err := NewSealer(base64.StdEncoding.EncodeToString(pub[:]), "k1")
	if err != nil {
		t.Fatalf("NewSealer() error: %v", err)
	}
	sealed, err := s.Seal([]byte("token"))
	if err != nil {
		t.Fatalf("Seal() error: %v", err)
	}
	opened, ok := box.OpenAnonymous(nil, sealed, pub, priv)
	if !ok || string(opened) != "token" || s.KeyID() != "k1" {
		t.Errorf("OpenAnonymous = %q, %v; KeyID = %q; want token, true, k1", opened, ok, s.KeyID())
	}
}

func TestNewSealerRejectsInvalidInput(t *testing.T) {
	valid := base64.StdEncoding.EncodeToString(make([]byte, 32))
	tests := []struct{ name, key, keyID string }{
		{"empty key", "", "k1"},
		{"not base64", "%%%", "k1"},
		{"wrong length", base64.StdEncoding.EncodeToString(make([]byte, 31)), "k1"},
		{"empty key id", valid, ""},
	}
	for _, tt := range tests {
		if _, err := NewSealer(tt.key, tt.keyID); err == nil {
			t.Errorf("NewSealer(%s) = _, nil; want error", tt.name)
		}
	}
}

func TestOpenerOpensSealed(t *testing.T) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	s, err := NewSealer(base64.StdEncoding.EncodeToString(pub[:]), "k1")
	if err != nil {
		t.Fatalf("NewSealer() error: %v", err)
	}
	sealed, err := s.Seal([]byte("token"))
	if err != nil {
		t.Fatalf("Seal() error: %v", err)
	}

	o, err := NewOpener(base64.StdEncoding.EncodeToString(priv[:]))
	if err != nil {
		t.Fatalf("NewOpener() error: %v", err)
	}
	opened, err := o.Open(sealed)
	if err != nil || string(opened) != "token" {
		t.Errorf("Open() = %q, %v; want token, nil", opened, err)
	}
}

func TestOpenerRejectsForeignBox(t *testing.T) {
	pub, _, _ := box.GenerateKey(rand.Reader)
	_, otherPriv, _ := box.GenerateKey(rand.Reader)
	s, _ := NewSealer(base64.StdEncoding.EncodeToString(pub[:]), "k1")
	sealed, _ := s.Seal([]byte("token"))

	o, err := NewOpener(base64.StdEncoding.EncodeToString(otherPriv[:]))
	if err != nil {
		t.Fatalf("NewOpener() error: %v", err)
	}
	if _, err := o.Open(sealed); !errors.Is(err, ErrCannotOpen) {
		t.Errorf("Open() error = %v, want %v", err, ErrCannotOpen)
	}
}

func TestNewOpenerRejectsInvalidKey(t *testing.T) {
	for _, key := range []string{"", "%%%", base64.StdEncoding.EncodeToString(make([]byte, 31))} {
		if _, err := NewOpener(key); !errors.Is(err, ErrInvalidPrivateKey) {
			t.Errorf("NewOpener(%q) error = %v, want %v", key, err, ErrInvalidPrivateKey)
		}
	}
}
