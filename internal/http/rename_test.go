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

	"github.com/google/go-cmp/cmp"

	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/user"
)

type userRenamerFunc func(context.Context, usecase.NameChange) (usecase.UserView, error)

func (f userRenamerFunc) Execute(ctx context.Context, change usecase.NameChange) (usecase.UserView, error) {
	return f(ctx, change)
}

func TestRenameServeHTTP(t *testing.T) {
	id := uuid.New()
	view := usecase.UserView{ID: id, Email: "jane@example.com", FirstName: "Janet", LastName: "Doe"}
	tests := []controllerTestCase{
		{name: "success", wantStatus: http.StatusOK, wantBody: `{"user":{"id":"` + id.String() + `","email":"jane@example.com","firstName":"Janet","lastName":"Doe"}}`},
		{name: "invalid first name", ucErr: &usecase.FieldError{Field: usecase.FieldFirstName, Err: user.ErrEmptyName}, wantStatus: http.StatusBadRequest, wantBody: `{"error":"firstName: must not be empty"}`},
		{name: "wrapped invalid last name", ucErr: fmt.Errorf("rename: %w", &usecase.FieldError{Field: usecase.FieldLastName, Err: user.ErrEmptyName}), wantStatus: http.StatusBadRequest, wantBody: `{"error":"lastName: must not be empty"}`},
		{name: "not found", ucErr: user.ErrNotFound, wantStatus: http.StatusNotFound, wantBody: `{"error":"user not found"}`},
		{name: "unexpected error", ucErr: errors.New("private database details"), wantStatus: http.StatusInternalServerError, wantBody: `{"error":"internal server error"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(t.Context(), userIDKey{}, id)
			body := strings.NewReader(`{"firstName":"Janet"}`)
			req := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/users/me/name", body)
			var calls int
			uc := userRenamerFunc(func(ctx context.Context, got usecase.NameChange) (usecase.UserView, error) {
				calls++
				if ctx != req.Context() {
					t.Errorf("Execute() got context %v, want request context", ctx)
				}
				firstName := "Janet"
				want := usecase.NameChange{UserID: id, FirstName: &firstName}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("Execute() change mismatch (-want +got):\n%s", diff)
				}
				return view, tt.ucErr
			})
			rec := httptest.NewRecorder()
			NewRename(uc).ServeHTTP(rec, req)
			if calls != 1 {
				t.Errorf("Execute() calls = %d, want 1", calls)
			}
			if diff := cmp.Diff(jsonResponse(t, tt.wantStatus, tt.wantBody), responseOf(t, rec)); diff != "" {
				t.Errorf("Rename.ServeHTTP() response mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
