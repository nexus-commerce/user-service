package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

const shutdownTimeout = 10 * time.Second

type Controller interface {
	Route() Route
}

type Server struct {
	srv        *http.Server
	api        *http.ServeMux
	basicAuth  Middleware
	bearerAuth Middleware
	logger     *slog.Logger
}

type Auth int

const (
	AuthNone Auth = iota
	AuthBasic
	AuthBearer
)

type Route struct {
	Pattern string
	Handler http.Handler
	Auth    Auth
}

type Options struct {
	Port       string
	BasicAuth  Middleware
	BearerAuth Middleware
	Logger     *slog.Logger
}

func New(opts Options) *Server {
	logger := opts.Logger
	if logger == nil {
		logger = discardLogger
	}
	api := http.NewServeMux()
	root := http.NewServeMux()
	root.Handle("/api/", http.StripPrefix("/api", api))

	return &Server{
		api: api,
		srv: &http.Server{
			Addr:         ":" + opts.Port,
			Handler:      Logging(logger)(root),
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 120 * time.Second,
		},
		basicAuth:  opts.BasicAuth,
		bearerAuth: opts.BearerAuth,
		logger:     logger,
	}
}

func (s *Server) Register(controllers ...Controller) {
	for _, c := range controllers {
		r := c.Route()
		h := r.Handler
		switch r.Auth {
		case AuthBasic:
			h = s.basicAuth(h)
		case AuthBearer:
			h = s.bearerAuth(h)
		}
		s.api.Handle(r.Pattern, h)
	}
}

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("starting server", "addr", s.srv.Addr)
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.logger.Info("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		if err := s.srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}
		return nil
	}
}
