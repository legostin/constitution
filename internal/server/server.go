package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/legostin/constitution/internal/check"
	"github.com/legostin/constitution/internal/server/store"
	"github.com/legostin/constitution/pkg/types"
)

// Server is the constitutiond HTTP server.
type Server struct {
	policy   *types.Policy
	registry *check.Registry
	store    store.Store
	mux      *http.ServeMux
	token    string
}

// Config holds server configuration.
type Config struct {
	Addr     string
	Policy   *types.Policy
	DBPath   string
	Token    string
}

// New creates a new Server.
func New(cfg Config) (*Server, error) {
	st, err := store.NewSQLiteStore(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("server: store init failed: %w", err)
	}

	s := &Server{
		policy:   cfg.Policy,
		registry: check.NewRegistry(),
		store:    st,
		mux:      http.NewServeMux(),
		token:    cfg.Token,
	}

	s.registerRoutes()
	return s, nil
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe(addr string) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.loggingMiddleware(s.authMiddleware(s.mux)),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	slog.Info("constitutiond starting", "addr", addr)
	return srv.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.store.Close()
}
