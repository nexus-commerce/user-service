package entity_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/nexus-commerce/user-service/src/entity"
)

func TestNewEmail(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr error
	}{
		{name: "valid", in: "jane.doe@example.com"},
		{name: "max length", in: strings.Repeat("a", 88) + "@example.com"},
		{name: "empty", in: "", wantErr: entity.ErrEmptyEmail},
		{name: "too long", in: strings.Repeat("a", 89) + "@example.com", wantErr: entity.ErrEmailTooLong},
		{name: "no at sign", in: "x", wantErr: entity.ErrMalformedEmail},
		{name: "display name", in: "Jane <jane@example.com>", wantErr: entity.ErrMalformedEmail},
		{name: "surrounding spaces", in: " jane@example.com ", wantErr: entity.ErrMalformedEmail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := entity.NewEmail(tt.in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) || !errors.Is(err, entity.ErrValidation) {
					t.Fatalf("NewEmail(%q) error = %v, want %v", tt.in, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewEmail(%q) unexpected error: %v", tt.in, err)
			}
			if got.String() != tt.in {
				t.Errorf("NewEmail(%q).String() = %q", tt.in, got.String())
			}
		})
	}
}
