// Package policyapi provides HTTP handlers for policy management.
package policyapi

import (
	"encoding/json"
	"io"
	nethttp "net/http"

	"github.com/tomeko19/gatewise/internal/policy/dsl"
	"github.com/tomeko19/gatewise/internal/policy/store/mem"
)

// Handler serves policy-related HTTP endpoints.
type Handler struct {
	store *mem.Store
}

// New creates a new policy HTTP handler.
func New(store *mem.Store) *Handler {
	return &Handler{store: store}
}

// Register registers policy routes under /v1.
func (h *Handler) Register(mux *nethttp.ServeMux) {
	mux.HandleFunc("/v1/policies", h.policies)
}

func (h *Handler) policies(w nethttp.ResponseWriter, r *nethttp.Request) {
	switch r.Method {
	case nethttp.MethodPost:
		h.createPolicy(w, r)
	case nethttp.MethodGet:
		h.listPolicies(w, r)
	default:
		w.WriteHeader(nethttp.StatusMethodNotAllowed)
	}
}

func (h *Handler) createPolicy(w nethttp.ResponseWriter, r *nethttp.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(nethttp.StatusBadRequest)
		return
	}

	p, err := dsl.ParsePolicyYAML(body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(nethttp.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	h.store.Upsert(p)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(nethttp.StatusCreated)
	_ = json.NewEncoder(w).Encode(p)
}

func (h *Handler) listPolicies(w nethttp.ResponseWriter, _ *nethttp.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.store.List())
}
