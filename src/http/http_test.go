package http

import (
	"errors"
	"testing"

	"github.com/nexus-commerce/user-service/src/user"
)

func TestValidationMessage(t *testing.T) {
	fields := wireFields{user.FieldEmail: "newEmail"}
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "mapped field",
			err:  &user.FieldError{Field: user.FieldEmail, Err: user.ErrMalformedEmail},
			want: "newEmail: must be a valid email address",
		},
		{
			name: "unmapped field",
			err:  &user.FieldError{Field: user.FieldFirstName, Err: user.ErrEmptyName},
			want: "first name: must not be empty",
		},
		{
			name: "no field",
			err:  user.ErrEmailUnchanged,
			want: "must differ from the current email",
		},
		{
			name: "wrapped",
			err:  errors.Join(errors.New("ctx"), &user.FieldError{Field: user.FieldEmail, Err: user.ErrEmptyEmail}),
			want: "newEmail: must not be empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validationMessage(tt.err, fields); got != tt.want {
				t.Errorf("validationMessage(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}
