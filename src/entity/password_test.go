package entity_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/nexus-commerce/user-service/src/entity"
)

func TestNewPassword(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr error
	}{
		{name: "valid", in: "s3cret"},
		{name: "max bytes", in: strings.Repeat("a", 72)},
		{name: "empty", in: "", wantErr: entity.ErrEmptyPassword},
		{name: "too long", in: strings.Repeat("a", 73), wantErr: entity.ErrPasswordTooLong},
		{name: "too long multibyte", in: strings.Repeat("ї", 37), wantErr: entity.ErrPasswordTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := entity.NewPassword(tt.in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) || !errors.Is(err, entity.ErrValidation) {
					t.Fatalf("NewPassword error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewPassword unexpected error: %v", err)
			}
		})
	}
}

func TestPasswordIsRedacted(t *testing.T) {
	p, err := entity.NewPassword("s3cret")
	if err != nil {
		t.Fatalf("NewPassword(%q) error: %v", "s3cret", err)
	}
	for _, verb := range []string{"%s", "%v", "%+v"} {
		if out := fmt.Sprintf(verb, p); strings.Contains(out, "s3cret") {
			t.Errorf("Sprintf(%q) leaks password: %s", verb, out)
		}
	}
}

func TestNewPasswordHash(t *testing.T) {
	const encoded = "$2a$10$abcdefghijklmnopqrstuuabcdefghijklmnopqrstuvwxyz01234"
	h, err := entity.NewPasswordHash(encoded)
	if err != nil {
		t.Fatalf("NewPasswordHash(%q) error: %v", encoded, err)
	}
	if got := h.Encoded(); got != encoded {
		t.Errorf("Encoded() = %q, want %q", got, encoded)
	}

	_, err = entity.NewPasswordHash("")
	if !errors.Is(err, entity.ErrMalformedPasswordHash) {
		t.Fatalf("NewPasswordHash(\"\") error = %v, want %v", err, entity.ErrMalformedPasswordHash)
	}
	if errors.Is(err, entity.ErrValidation) {
		t.Error("malformed hash must not be a validation error")
	}
}

func TestPasswordHashIsRedacted(t *testing.T) {
	const encoded = "$2a$10$secret"
	h, err := entity.NewPasswordHash(encoded)
	if err != nil {
		t.Fatalf("NewPasswordHash(%q) error: %v", encoded, err)
	}
	for _, verb := range []string{"%s", "%v", "%+v"} {
		if out := fmt.Sprintf(verb, h); strings.Contains(out, encoded) {
			t.Errorf("Sprintf(%q) leaks password hash: %s", verb, out)
		}
	}
}
