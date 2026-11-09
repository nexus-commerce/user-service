package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/user"
)

type authenticatorFunc func(context.Context, usecase.Credentials) (string, error)

func (f authenticatorFunc) Execute(ctx context.Context, creds usecase.Credentials) (string, error) {
	return f(ctx, creds)
}

func TestLoginServeHTTP(t *testing.T) {
	tests := []controllerTestCase{
		{name: "success", wantStatus: http.StatusOK, wantBody: `{"token":"access-token"}`},
		{name: "invalid credentials", ucErr: user.ErrInvalidCredentials, wantStatus: http.StatusUnauthorized, wantBody: `{"error":"invalid credentials"}`},
		{name: "wrapped invalid credentials", ucErr: fmt.Errorf("login: %w", user.ErrInvalidCredentials), wantStatus: http.StatusUnauthorized, wantBody: `{"error":"invalid credentials"}`},
		{name: "unexpected error", ucErr: errors.New("private database details"), wantStatus: http.StatusInternalServerError, wantBody: `{"error":"internal server error"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/login", strings.NewReader(`{"email":"jane@example.com","password":"secretpass"}`))
			var calls int
			uc := authenticatorFunc(func(ctx context.Context, creds usecase.Credentials) (string, error) {
				calls++
				want := usecase.Credentials{Email: "jane@example.com", Password: "secretpass"}
				if ctx != req.Context() || creds != want {
					t.Errorf("Execute() got context %v, credentials %+v; want request context, %+v", ctx, creds, want)
				}
				return "access-token", tt.ucErr
			})
			rec := httptest.NewRecorder()
			NewLogin(uc).ServeHTTP(rec, req)
			if calls != 1 {
				t.Errorf("Execute() calls = %d, want 1", calls)
			}
			checkResponse(t, "Login.ServeHTTP()", rec, tt.wantStatus, tt.wantBody)
		})
	}
}
