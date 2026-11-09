package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/user"
)

type userFetcherFunc func(context.Context, uuid.UUID) (usecase.UserView, error)

func (f userFetcherFunc) Execute(ctx context.Context, id uuid.UUID) (usecase.UserView, error) {
	return f(ctx, id)
}

func TestFetchUserServeHTTP(t *testing.T) {
	id := uuid.New()
	view := usecase.UserView{ID: id, Email: "jane@example.com", FirstName: "Jane", LastName: "Doe"}
	tests := []controllerTestCase{
		{name: "success", wantStatus: http.StatusOK, wantBody: `{"user":{"id":"` + id.String() + `","email":"jane@example.com","firstName":"Jane","lastName":"Doe"}}`},
		{name: "not found", ucErr: user.ErrNotFound, wantStatus: http.StatusNotFound, wantBody: `{"error":"user not found"}`},
		{name: "wrapped not found", ucErr: fmt.Errorf("fetch: %w", user.ErrNotFound), wantStatus: http.StatusNotFound, wantBody: `{"error":"user not found"}`},
		{name: "unexpected error", ucErr: errors.New("private database details"), wantStatus: http.StatusInternalServerError, wantBody: `{"error":"internal server error"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.WithValue(t.Context(), userIDKey{}, id), http.MethodGet, "/users/me", nil)
			var calls int
			uc := userFetcherFunc(func(ctx context.Context, got uuid.UUID) (usecase.UserView, error) {
				calls++
				if ctx != req.Context() || got != id {
					t.Errorf("Execute() got context %v, user %v; want request context, %v", ctx, got, id)
				}
				return view, tt.ucErr
			})
			rec := httptest.NewRecorder()
			NewFetchUser(uc).ServeHTTP(rec, req)
			if calls != 1 {
				t.Errorf("Execute() calls = %d, want 1", calls)
			}
			checkResponse(t, "FetchUser.ServeHTTP()", rec, tt.wantStatus, tt.wantBody)
		})
	}
}
