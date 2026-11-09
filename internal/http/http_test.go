package http

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
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

type response struct {
	Status      int
	ContentType string
	Body        any
}

func responseOf(t *testing.T, rec *httptest.ResponseRecorder) response {
	t.Helper()
	return response{
		Status:      rec.Code,
		ContentType: rec.Header().Get("Content-Type"),
		Body:        decodeJSON(t, rec.Body.String()),
	}
}

func jsonResponse(t *testing.T, status int, body string) response {
	t.Helper()
	if body == "" {
		return response{Status: status}
	}
	return response{Status: status, ContentType: "application/json", Body: decodeJSON(t, body)}
}

func decodeJSON(t *testing.T, s string) any {
	t.Helper()
	if s == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("json.Unmarshal(%q) error: %v", s, err)
	}
	return v
}
