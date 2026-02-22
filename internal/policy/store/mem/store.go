// Package model provides data memories for policy management.
package mem

import (
	"sync"

	"github.com/tomeko19/gatewise/internal/policy/model"
)

// Store manages policies in memory.
type Store struct {
	mu       sync.RWMutex
	policies map[string]*model.Policy
}

// New returns a new Store instance.
func New() *Store {
	return &Store{
		policies: make(map[string]*model.Policy),
	}
}

// Upsert adds or updates a policy in the store.
func key(tenant, role string) string {
	return tenant + ":" + role
}

// List returns all policies stored.
func (s *Store) Upsert(p *model.Policy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policies[key(p.Tenant, p.Role)] = p
}

func (s *Store) List() []*model.Policy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*model.Policy, 0, len(s.policies))
	for _, v := range s.policies {
		out = append(out, v)
	}
	return out
}
