package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/user"
)

type emailChangeRequesterFunc func(context.Context, usecase.EmailChangeRequest) error

func (f emailChangeRequesterFunc) Execute(ctx context.Context, req usecase.EmailChangeRequest) error {
	return f(ctx, req)
}

func TestRequestEmailChangeServeHTTP(t *testing.T) {
	id := uuid.New()
	tests := []controllerTestCase{
		{name: "success", wantStatus: http.StatusAccepted},
		{name: "invalid email", ucErr: &usecase.FieldError{Field: usecase.FieldEmail, Err: user.ErrMalformedEmail}, wantStatus: http.StatusBadRequest, wantBody: `{"error":"newEmail: must be a valid email address"}`},
		{name: "unchanged email", ucErr: &usecase.FieldError{Field: usecase.FieldEmail, Err: user.ErrEmailUnchanged}, wantStatus: http.StatusBadRequest, wantBody: `{"error":"newEmail: must differ from the current email"}`},
		{name: "invalid credentials", ucErr: user.ErrInvalidCredentials, wantStatus: http.StatusForbidden, wantBody: `{"error":"invalid password"}`},
		{name: "wrapped invalid credentials", ucErr: fmt.Errorf("change email: %w", user.ErrInvalidCredentials), wantStatus: http.StatusForbidden, wantBody: `{"error":"invalid password"}`},
		{name: "not found", ucErr: user.ErrNotFound, wantStatus: http.StatusNotFound, wantBody: `{"error":"user not found"}`},
		{name: "unexpected error", ucErr: errors.New("private database details"), wantStatus: http.StatusInternalServerError, wantBody: `{"error":"internal server error"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(t.Context(), userIDKey{}, id)
			body := strings.NewReader(`{"newEmail":"new@example.com","password":"secretpass"}`)
			req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/users/me/email", body)
			calls := 0
			uc := emailChangeRequesterFunc(func(ctx context.Context, got usecase.EmailChangeRequest) error {
				calls++
				want := usecase.EmailChangeRequest{UserID: id, NewEmail: "new@example.com", Password: "secretpass"}
				if ctx != req.Context() || got != want {
					t.Errorf("Execute() got context %v, email change %+v; want request context, %+v", ctx, got, want)
				}
				return tt.ucErr
			})
			rec := httptest.NewRecorder()
			NewRequestEmailChange(uc).ServeHTTP(rec, req)
			if calls != 1 {
				t.Errorf("Execute() calls = %d, want 1", calls)
			}
			checkResponse(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}
