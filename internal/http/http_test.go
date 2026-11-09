package http

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"

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

func checkResponse(t *testing.T, handler string, rec *httptest.ResponseRecorder, wantStatus int, wantBody string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Errorf("%s status = %d, want %d; body = %s", handler, rec.Code, wantStatus, rec.Body.String())
	}
	if wantBody == "" {
		if rec.Body.Len() != 0 {
			t.Errorf("%s body = %q, want empty", handler, rec.Body.String())
		}
		return
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("%s Content-Type = %q, want application/json", handler, got)
	}
	var got, want any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("%s body %q: decode: %v", handler, rec.Body.String(), err)
	}
	if err := json.Unmarshal([]byte(wantBody), &want); err != nil {
		t.Fatalf("decode expected body %q: %v", wantBody, err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("%s body mismatch (-want +got):\n%s", handler, diff)
	}
}
