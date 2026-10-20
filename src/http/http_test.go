package http

import (
	"errors"
	"testing"

	"github.com/nexus-commerce/user-service/src/usecase"
	"github.com/nexus-commerce/user-service/src/user"
)

func TestValidationMessage(t *testing.T) {
	fields := wireFields{usecase.FieldEmail: "newEmail"}
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "mapped field",
			err:  &usecase.FieldError{Field: usecase.FieldEmail, Err: user.ErrMalformedEmail},
			want: "newEmail: must be a valid email address",
		},
		{
			name: "unmapped field",
			err:  &usecase.FieldError{Field: usecase.FieldFirstName, Err: user.ErrEmptyName},
			want: "FirstName: must not be empty",
		},
		{
			name: "no field",
			err:  user.ErrEmailUnchanged,
			want: "must differ from the current email",
		},
		{
			name: "wrapped",
			err:  errors.Join(errors.New("ctx"), &usecase.FieldError{Field: usecase.FieldEmail, Err: user.ErrEmptyEmail}),
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
