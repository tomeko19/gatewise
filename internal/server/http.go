// Package server provides the HTTP server for Gatewise.
package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// Server wraps the HTTP server implementation for Gatewise.
type Server struct {
	httpServer *http.Server
}

// HealthResponse is returned by the /healthz endpoint.
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// New creates a new Gatewise HTTP server listening on addr.
func New(addr string) *Server {
	s := &http.Server{
		Addr:              addr,
		Handler:           NewHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &Server{httpServer: s}
}

// NewHandler builds the HTTP handler (routes) for Gatewise.
func NewHandler() http.Handler {
	mux := http.NewServeMux()

	// health endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := HealthResponse{
			Status:    "ok",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	// root endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("gatewise api\n"))
	})

	return mux
}

// Start runs the HTTP server (blocking call).
func (s *Server) Start() error {
	log.Printf("[gatewise] listening on %s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
