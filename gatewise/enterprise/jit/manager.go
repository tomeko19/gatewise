// Package jit provides Just-In-Time access management
package jit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gatewise/gatewise/internal/store"
	"github.com/gatewise/gatewise/pkg/license"
)

// Grant represents a JIT access grant
type Grant struct {
	ID              string    `json:"id"`
	PolicyName      string    `json:"policyName"`
	Grantee         string    `json:"grantee"`
	GrantedBy       string    `json:"grantedBy"`
	Reason          string    `json:"reason"`
	Duration        time.Duration `json:"duration"`
	CreatedAt       time.Time `json:"createdAt"`
	ExpiresAt       time.Time `json:"expiresAt"`
	Renewals        int       `json:"renewals"`
	MaxRenewals     int       `json:"maxRenewals"`
	Status          GrantStatus `json:"status"`
	RevokedAt       *time.Time `json:"revokedAt,omitempty"`
	RevokedBy       string    `json:"revokedBy,omitempty"`
	ApprovalRequired bool     `json:"approvalRequired"`
	ApprovedBy      string    `json:"approvedBy,omitempty"`
	ApprovedAt      *time.Time `json:"approvedAt,omitempty"`
}

// GrantStatus represents the status of a grant
type GrantStatus string

const (
	GrantStatusPending   GrantStatus = "pending"
	GrantStatusActive    GrantStatus = "active"
	GrantStatusExpired   GrantStatus = "expired"
	GrantStatusRevoked   GrantStatus = "revoked"
	GrantStatusRejected  GrantStatus = "rejected"
)

// GrantRequest represents a request for JIT access
type GrantRequest struct {
	PolicyName string        `json:"policyName"`
	Grantee    string        `json:"grantee"`
	Reason     string        `json:"reason"`
	Duration   time.Duration `json:"duration"`
}

// Manager manages JIT access grants
type Manager struct {
	mu         sync.RWMutex
	grants     map[string]*Grant
	store      *store.PostgresStore
	licenseMgr *license.Manager
	stopCh     chan struct{}
	eventCh    chan GrantEvent
	running    bool
}

// GrantEvent represents a JIT grant event
type GrantEvent struct {
	Type    string `json:"type"`
	Grant   *Grant `json:"grant"`
	Message string `json:"message"`
}

// NewManager creates a new JIT manager
func NewManager(store *store.PostgresStore, licenseMgr *license.Manager) *Manager {
	return &Manager{
		grants:     make(map[string]*Grant),
		store:      store,
		licenseMgr: licenseMgr,
		stopCh:     make(chan struct{}),
		eventCh:    make(chan GrantEvent, 100),
	}
}

// RequestAccess creates a new JIT access request
func (m *Manager) RequestAccess(ctx context.Context, req *GrantRequest, requestedBy string) (*Grant, error) {
	// Check license
	if !m.licenseMgr.IsFeatureEnabled(license.FeatureJITAccess) {
		return nil, fmt.Errorf("JIT access requires Enterprise license")
	}

	// Validate request
	if req.PolicyName == "" {
		return nil, fmt.Errorf("policy name is required")
	}
	if req.Grantee == "" {
		return nil, fmt.Errorf("grantee is required")
	}
	if req.Duration <= 0 {
		return nil, fmt.Errorf("duration must be positive")
	}
	if req.Duration > 24*time.Hour {
		return nil, fmt.Errorf("maximum duration is 24 hours")
	}

	now := time.Now()
	grant := &Grant{
		ID:              fmt.Sprintf("jit-%d", now.UnixNano()),
		PolicyName:      req.PolicyName,
		Grantee:         req.Grantee,
		GrantedBy:       requestedBy,
		Reason:          req.Reason,
		Duration:        req.Duration,
		CreatedAt:       now,
		ExpiresAt:       now.Add(req.Duration),
		Renewals:        0,
		MaxRenewals:     2,
		Status:          GrantStatusActive, // Auto-approve for now
		ApprovalRequired: false,
	}

	m.mu.Lock()
	m.grants[grant.ID] = grant
	m.mu.Unlock()

	// Send event
	m.sendEvent(GrantEvent{
		Type:    "grant_created",
		Grant:   grant,
		Message: fmt.Sprintf("JIT access granted to %s for policy %s", grant.Grantee, grant.PolicyName),
	})

	return grant, nil
}

// RenewGrant renews an existing grant
func (m *Manager) RenewGrant(ctx context.Context, grantID string, renewedBy string) (*Grant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	grant, exists := m.grants[grantID]
	if !exists {
		return nil, fmt.Errorf("grant not found")
	}

	if grant.Status != GrantStatusActive {
		return nil, fmt.Errorf("grant is not active")
	}

	if grant.Renewals >= grant.MaxRenewals {
		return nil, fmt.Errorf("maximum renewals reached")
	}

	grant.Renewals++
	grant.ExpiresAt = time.Now().Add(grant.Duration)

	m.sendEvent(GrantEvent{
		Type:    "grant_renewed",
		Grant:   grant,
		Message: fmt.Sprintf("JIT access renewed for %s (renewal %d/%d)", grant.Grantee, grant.Renewals, grant.MaxRenewals),
	})

	return grant, nil
}

// RevokeGrant revokes an active grant
func (m *Manager) RevokeGrant(ctx context.Context, grantID string, revokedBy string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	grant, exists := m.grants[grantID]
	if !exists {
		return fmt.Errorf("grant not found")
	}

	if grant.Status != GrantStatusActive && grant.Status != GrantStatusPending {
		return fmt.Errorf("grant cannot be revoked")
	}

	now := time.Now()
	grant.Status = GrantStatusRevoked
	grant.RevokedAt = &now
	grant.RevokedBy = revokedBy

	m.sendEvent(GrantEvent{
		Type:    "grant_revoked",
		Grant:   grant,
		Message: fmt.Sprintf("JIT access revoked for %s by %s: %s", grant.Grantee, revokedBy, reason),
	})

	return nil
}

// GetGrant retrieves a grant by ID
func (m *Manager) GetGrant(grantID string) (*Grant, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	grant, exists := m.grants[grantID]
	return grant, exists
}

// ListGrants returns all grants with optional filtering
func (m *Manager) ListGrants(status GrantStatus, policyName string) []*Grant {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Grant
	for _, grant := range m.grants {
		if status != "" && grant.Status != status {
			continue
		}
		if policyName != "" && grant.PolicyName != policyName {
			continue
		}
		result = append(result, grant)
	}
	return result
}

// ListActiveGrants returns grants for a specific grantee
func (m *Manager) ListActiveGrants(grantee string) []*Grant {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Grant
	for _, grant := range m.grants {
		if grant.Grantee == grantee && grant.Status == GrantStatusActive {
			result = append(result, grant)
		}
	}
	return result
}

// Start begins the expiration checker loop
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("JIT manager already running")
	}
	m.running = true
	m.mu.Unlock()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-m.stopCh:
			return nil
		case <-ticker.C:
			m.checkExpirations()
		}
	}
}

// Stop stops the JIT manager
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		close(m.stopCh)
		m.running = false
	}
}

// GetEventChannel returns the event channel
func (m *Manager) GetEventChannel() <-chan GrantEvent {
	return m.eventCh
}

// checkExpirations checks and expires grants
func (m *Manager) checkExpirations() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, grant := range m.grants {
		if grant.Status == GrantStatusActive && now.After(grant.ExpiresAt) {
			grant.Status = GrantStatusExpired
			m.sendEvent(GrantEvent{
				Type:    "grant_expired",
				Grant:   grant,
				Message: fmt.Sprintf("JIT access expired for %s on policy %s", grant.Grantee, grant.PolicyName),
			})
		}
	}
}

func (m *Manager) sendEvent(event GrantEvent) {
	select {
	case m.eventCh <- event:
	default:
	}
}
