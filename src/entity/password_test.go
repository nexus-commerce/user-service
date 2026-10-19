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

func TestPasswordHashMatches(t *testing.T) {
	p := mustPassword(t, "s3cret")
	other := mustPassword(t, "other")

	hash, err := p.Hash()
	if err != nil {
		t.Fatalf("Hash() error: %v", err)
	}
	if ok, err := p.Matches(hash); err != nil || !ok {
		t.Errorf("Matches(own hash) = %v, %v; want true, nil", ok, err)
	}
	if ok, err := other.Matches(hash); err != nil || ok {
		t.Errorf("Matches(foreign hash) = %v, %v; want false, nil", ok, err)
	}
}
