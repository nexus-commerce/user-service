package http

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"uuid"
)

var ErrNoUserID = errors.New("user id not found in context")

type Middleware func(http.Handler) http.Handler

func BasicAuth(username, password string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			userOK := subtle.ConstantTimeCompare([]byte(user), []byte(username)) == 1
			passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(password)) == 1
			if !ok || !userOK || !passOK {
				rw.Header().Set("WWW-Authenticate", `Basic realm="User Service"`)
				writeError(rw, r, http.StatusUnauthorized, "unauthorized")
				return
			}
			next.ServeHTTP(rw, r)
		})
	}
}

type TokenParser interface {
	Parse(token string) (uuid.UUID, error)
}

type userIDKey struct{}

func BearerAuth(p TokenParser) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || raw == "" {
				writeError(rw, r, http.StatusUnauthorized, "bearer token not found")
				return
			}

			userID, err := p.Parse(raw)
			if err != nil {
				writeError(rw, r, http.StatusUnauthorized, "invalid token")
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey{}, userID)
			next.ServeHTTP(rw, r.WithContext(ctx))
		})
	}
}

func userIDFromContext(ctx context.Context) (uuid.UUID, error) {
	id, ok := ctx.Value(userIDKey{}).(uuid.UUID)
	if !ok || id == uuid.Nil() {
		return uuid.Nil(), ErrNoUserID
	}
	return id, nil
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(status int) {
	rec.status = status
	rec.ResponseWriter.WriteHeader(status)
}

func (rec *statusRecorder) Unwrap() http.ResponseWriter {
	return rec.ResponseWriter
}

type loggerKey struct{}

var discardLogger = slog.New(slog.DiscardHandler)

func Logging(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: rw, status: http.StatusOK}

			next.ServeHTTP(rec, r.WithContext(context.WithValue(r.Context(), loggerKey{}, logger)))

			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration", time.Since(start),
			)
		})
	}
}

func loggerFrom(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
		return l
	}
	return discardLogger
}

func logRequestError(r *http.Request, msg string, err error) {
	loggerFrom(r.Context()).Error(msg, "method", r.Method, "path", r.URL.Path, "err", err)
}
