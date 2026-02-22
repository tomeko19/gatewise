package mem

import (
	"sync"

	"github.com/tomeko19/gatewise/internal/policy/model"
)

type Store struct {
	mu       sync.RWMutex
	policies map[string]*model.Policy
}

func New() *Store {
	return &Store{
		policies: make(map[string]*model.Policy),
	}
}

func key(tenant, role string) string {
	return tenant + ":" + role
}

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
