// Package server provides the HTTP server for Gatewise.
package server

import (
	"context"
	"encoding/json"
	"log"
	nethttp "net/http"
	"time"

	policyapi "github.com/tomeko19/gatewise/internal/policy/http"
	"github.com/tomeko19/gatewise/internal/policy/store/mem"
)

// Server wraps the HTTP server implementation for Gatewise.
type Server struct {
	httpServer *nethttp.Server
}

// HealthResponse is returned by the /healthz endpoint.
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// New creates a new Gatewise HTTP server listening on addr.
func New(addr string) *Server {
	s := &nethttp.Server{
		Addr:              addr,
		Handler:           NewHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return &Server{httpServer: s}
}

// NewHandler builds the HTTP handler (routes) for Gatewise.
func NewHandler() nethttp.Handler {
	mux := nethttp.NewServeMux()

	// health endpoint
	mux.HandleFunc("/healthz", func(w nethttp.ResponseWriter, _ *nethttp.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := HealthResponse{
			Status:    "ok",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	// ✅ register policy routes
	pstore := mem.New()
	ph := policyapi.New(pstore)
	ph.Register(mux)

	// root endpoint (strict)
	mux.HandleFunc("/", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if r.URL.Path != "/" {
			nethttp.NotFound(w, r)
			return
		}
		w.WriteHeader(nethttp.StatusOK)
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
