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
		want    string
		wantErr error
	}{
		{name: "valid", in: "Jane", want: "Jane"},
		{name: "max length multibyte", in: strings.Repeat("ї", 50), want: strings.Repeat("ї", 50)},
		{name: "surrounding whitespace trimmed", in: " \tJane \n", want: "Jane"},
		{name: "inner whitespace kept", in: "Mary Jane", want: "Mary Jane"},
		{name: "padding not counted toward length", in: "  " + strings.Repeat("a", 50) + "  ", want: strings.Repeat("a", 50)},
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
			if got.String() != tt.want {
				t.Errorf("NewName(%q).String() = %q, want %q", tt.in, got.String(), tt.want)
			}
		})
	}
}

func TestFullNameWith(t *testing.T) {
	jane, _ := user.NewName("Jane")
	doe, _ := user.NewName("Doe")
	janet, _ := user.NewName("Janet")
	smith, _ := user.NewName("Smith")
	orig := user.NewFullName(jane, doe)

	if got := orig.WithFirstName(janet); got != user.NewFullName(janet, doe) {
		t.Errorf("WithFirstName() = %s %s, want Janet Doe", got.FirstName(), got.LastName())
	}
	if got := orig.WithLastName(smith); got != user.NewFullName(jane, smith) {
		t.Errorf("WithLastName() = %s %s, want Jane Smith", got.FirstName(), got.LastName())
	}
	if orig != user.NewFullName(jane, doe) {
		t.Errorf("original mutated: %s %s", orig.FirstName(), orig.LastName())
	}
}
