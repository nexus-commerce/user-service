package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/user"
)

type registrationRequesterFunc func(context.Context, usecase.RegistrationRequest) error

func (f registrationRequesterFunc) Execute(ctx context.Context, req usecase.RegistrationRequest) error {
	return f(ctx, req)
}

func TestRequestRegistrationServeHTTP(t *testing.T) {
	tests := []controllerTestCase{
		{name: "success", wantStatus: http.StatusAccepted},
		{name: "invalid first name", ucErr: &usecase.FieldError{Field: usecase.FieldFirstName, Err: user.ErrEmptyName}, wantStatus: http.StatusBadRequest, wantBody: `{"error":"firstName: must not be empty"}`},
		{name: "invalid last name", ucErr: &usecase.FieldError{Field: usecase.FieldLastName, Err: user.ErrEmptyName}, wantStatus: http.StatusBadRequest, wantBody: `{"error":"lastName: must not be empty"}`},
		{name: "invalid email", ucErr: &usecase.FieldError{Field: usecase.FieldEmail, Err: user.ErrMalformedEmail}, wantStatus: http.StatusBadRequest, wantBody: `{"error":"email: must be a valid email address"}`},
		{name: "invalid password", ucErr: &usecase.FieldError{Field: usecase.FieldPassword, Err: user.ErrEmptyPassword}, wantStatus: http.StatusBadRequest, wantBody: `{"error":"password: must not be empty"}`},
		{name: "password mismatch", ucErr: &usecase.FieldError{Field: usecase.FieldPasswordConfirmation, Err: usecase.ErrPasswordMismatch}, wantStatus: http.StatusBadRequest, wantBody: `{"error":"passwordConfirmation: passwords do not match"}`},
		{name: "unexpected error", ucErr: errors.New("private database details"), wantStatus: http.StatusInternalServerError, wantBody: `{"error":"internal server error"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register", strings.NewReader(`{"firstName":"Jane","lastName":"Doe","email":"jane@example.com","password":"secretpass","passwordConfirmation":"secretpass"}`))
			calls := 0
			uc := registrationRequesterFunc(func(ctx context.Context, got usecase.RegistrationRequest) error {
				calls++
				want := usecase.RegistrationRequest{FirstName: "Jane", LastName: "Doe", Email: "jane@example.com", Password: "secretpass", PasswordConfirmation: "secretpass"}
				if ctx != req.Context() || got != want {
					t.Errorf("Execute() got context %v, registration %+v; want request context, %+v", ctx, got, want)
				}
				return tt.ucErr
			})
			rec := httptest.NewRecorder()
			NewRequestRegistration(uc).ServeHTTP(rec, req)
			if calls != 1 {
				t.Errorf("Execute() calls = %d, want 1", calls)
			}
			checkResponse(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}
