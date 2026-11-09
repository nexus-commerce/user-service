package http

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/user"
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

type controllerTestCase struct {
	name       string
	ucErr      error
	wantStatus int
	wantBody   string
}

func checkResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantBody string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Errorf("status = %d, want %d; body = %s", rec.Code, wantStatus, rec.Body.String())
	}
	if wantBody == "" {
		if rec.Body.Len() != 0 {
			t.Errorf("body = %q, want empty", rec.Body.String())
		}
		return
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var got, want any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if err := json.Unmarshal([]byte(wantBody), &want); err != nil {
		t.Fatalf("decode expected response: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("body = %s, want %s", rec.Body.String(), wantBody)
	}
}
