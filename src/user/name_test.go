package user_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/nexus-commerce/user-service/src/user"
)

func TestNewName(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr error
	}{
		{name: "valid", in: "Jane"},
		{name: "max length multibyte", in: strings.Repeat("ї", 50)},
		{name: "empty", in: "", wantErr: user.ErrEmptyName},
		{name: "whitespace only", in: " \t ", wantErr: user.ErrEmptyName},
		{name: "too long", in: strings.Repeat("a", 51), wantErr: user.ErrNameTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := user.NewName(tt.in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) || !errors.Is(err, user.ErrValidation) {
					t.Fatalf("NewName(%q) error = %v, want %v", tt.in, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewName(%q) unexpected error: %v", tt.in, err)
			}
			if got.String() != tt.in {
				t.Errorf("NewName(%q).String() = %q", tt.in, got.String())
			}
		})
	}
}
