package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/nexus-commerce/user-service/internal/user"
)

type emailChangeConfirmerFunc func(context.Context, string) error

func (f emailChangeConfirmerFunc) Execute(ctx context.Context, token string) error {
	return f(ctx, token)
}

func TestConfirmEmailChangeServeHTTP(t *testing.T) {
	tests := []controllerTestCase{
		{name: "success", wantStatus: http.StatusNoContent},
		{name: "invalid token", ucErr: user.ErrInvalidConfirmationToken, wantStatus: http.StatusBadRequest, wantBody: `{"error":"invalid or already used confirmation link"}`},
		{name: "expired token", ucErr: user.ErrConfirmationTokenExpired, wantStatus: http.StatusBadRequest, wantBody: `{"error":"confirmation link has expired"}`},
		{name: "email taken", ucErr: user.ErrEmailTaken, wantStatus: http.StatusConflict, wantBody: `{"error":"user email already exists"}`},
		{name: "wrapped email taken", ucErr: fmt.Errorf("confirm: %w", user.ErrEmailTaken), wantStatus: http.StatusConflict, wantBody: `{"error":"user email already exists"}`},
		{name: "unexpected error", ucErr: errors.New("private database details"), wantStatus: http.StatusInternalServerError, wantBody: `{"error":"internal server error"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/users/email/confirm", strings.NewReader(`{"token":"confirmation-token"}`))
			var calls int
			uc := emailChangeConfirmerFunc(func(ctx context.Context, token string) error {
				calls++
				if ctx != req.Context() || token != "confirmation-token" {
					t.Errorf("Execute() got context %v, token %q; want request context, confirmation-token", ctx, token)
				}
				return tt.ucErr
			})
			rec := httptest.NewRecorder()
			NewConfirmEmailChange(uc).ServeHTTP(rec, req)
			if calls != 1 {
				t.Errorf("Execute() calls = %d, want 1", calls)
			}
			if diff := cmp.Diff(jsonResponse(t, tt.wantStatus, tt.wantBody), responseOf(t, rec)); diff != "" {
				t.Errorf("ConfirmEmailChange.ServeHTTP() response mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
