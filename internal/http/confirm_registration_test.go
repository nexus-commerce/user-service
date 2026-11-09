package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexus-commerce/user-service/internal/user"
)

type registrationConfirmerFunc func(context.Context, string) error

func (f registrationConfirmerFunc) Execute(ctx context.Context, token string) error {
	return f(ctx, token)
}

func TestConfirmRegistrationServeHTTP(t *testing.T) {
	tests := []controllerTestCase{
		{name: "success", wantStatus: http.StatusAccepted, wantBody: `{"status":"registration confirmed, account is being created"}`},
		{name: "invalid token", ucErr: user.ErrInvalidConfirmationToken, wantStatus: http.StatusBadRequest, wantBody: `{"error":"invalid or already used confirmation link"}`},
		{name: "expired token", ucErr: user.ErrConfirmationTokenExpired, wantStatus: http.StatusBadRequest, wantBody: `{"error":"confirmation link has expired"}`},
		{name: "wrapped expired token", ucErr: fmt.Errorf("confirm: %w", user.ErrConfirmationTokenExpired), wantStatus: http.StatusBadRequest, wantBody: `{"error":"confirmation link has expired"}`},
		{name: "unexpected error", ucErr: errors.New("private database details"), wantStatus: http.StatusInternalServerError, wantBody: `{"error":"internal server error"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/confirm", strings.NewReader(`{"token":"confirmation-token"}`))
			calls := 0
			uc := registrationConfirmerFunc(func(ctx context.Context, token string) error {
				calls++
				if ctx != req.Context() || token != "confirmation-token" {
					t.Errorf("Execute() got context %v, token %q; want request context, confirmation-token", ctx, token)
				}
				return tt.ucErr
			})
			rec := httptest.NewRecorder()
			NewConfirmRegistration(uc).ServeHTTP(rec, req)
			if calls != 1 {
				t.Errorf("Execute() calls = %d, want 1", calls)
			}
			checkResponse(t, "ConfirmRegistration.ServeHTTP()", rec, tt.wantStatus, tt.wantBody)
		})
	}
}
