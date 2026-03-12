// Package reconciler handles policy reconciliation across connectors
package reconciler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gatewise/gatewise/internal/connector"
	"github.com/gatewise/gatewise/internal/store"
	"github.com/gatewise/gatewise/pkg/license"
	"github.com/gatewise/gatewise/pkg/models"
)

// ReconcilerConfig holds reconciler configuration
type ReconcilerConfig struct {
	ReconcileInterval time.Duration
	DriftCheckEnabled bool
	DriftCheckInterval time.Duration
}

// DefaultConfig returns default reconciler configuration
func DefaultConfig() *ReconcilerConfig {
	return &ReconcilerConfig{
		ReconcileInterval:  30 * time.Second,
		DriftCheckEnabled:  false,
		DriftCheckInterval: 5 * time.Minute,
	}
}

// Reconciler orchestrates policy reconciliation
type Reconciler struct {
	mu           sync.RWMutex
	config       *ReconcilerConfig
	factory      *connector.Factory
	store        *store.PostgresStore
	licenseMgr   *license.Manager
	stopCh       chan struct{}
	running      bool
}

// NewReconciler creates a new reconciler
func NewReconciler(config *ReconcilerConfig, factory *connector.Factory, store *store.PostgresStore, licenseMgr *license.Manager) *Reconciler {
	if config == nil {
		config = DefaultConfig()
	}
	return &Reconciler{
		config:     config,
		factory:    factory,
		store:      store,
		licenseMgr: licenseMgr,
		stopCh:     make(chan struct{}),
	}
}

// ReconcilePolicy reconciles a single policy
func (r *Reconciler) ReconcilePolicy(ctx context.Context, policy *models.Policy) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Update status to Reconciling
	if r.store != nil {
		r.store.UpdatePolicyStatus(ctx, policy.Metadata.Name, models.StateReconciling, "Starting reconciliation")
	}

	var allErrors []error
	var allCreated []string

	// Process each target
	for _, target := range policy.Spec.Targets {
		targetCopy := target // Create copy for pointer
		
		// Get the appropriate connector
		connType := r.targetTypeToConnectorType(target.Type)
		conn, exists := r.factory.GetConnector(connType)
		if !exists {
			// Try to create it
			var err error
			conn, err = r.factory.CreateConnector(connType)
			if err != nil {
				allErrors = append(allErrors, fmt.Errorf("target %s: %w", target.Name, err))
				continue
			}
			// Initialize
			if err := conn.Initialize(ctx); err != nil {
				allErrors = append(allErrors, fmt.Errorf("target %s init: %w", target.Name, err))
				continue
			}
		}

		// Reconcile the target
		result, err := conn.Reconcile(ctx, policy, &targetCopy)
		if err != nil {
			allErrors = append(allErrors, fmt.Errorf("target %s: %w", target.Name, err))
			continue
		}

		if !result.Success {
			for _, e := range result.Errors {
				allErrors = append(allErrors, fmt.Errorf("target %s: %w", target.Name, e))
			}
		}

		allCreated = append(allCreated, result.ResourcesCreated...)

		// Create audit log
		if r.store != nil {
			auditEntry := &store.AuditLogEntry{
				PolicyName: policy.Metadata.Name,
				Action:     "RECONCILE",
				TargetType: string(target.Type),
				TargetName: target.Name,
				Actor:      "reconciler",
				Details: map[string]interface{}{
					"resources_created": result.ResourcesCreated,
					"resources_updated": result.ResourcesUpdated,
				},
				Success: result.Success,
			}
			if !result.Success && len(result.Errors) > 0 {
				auditEntry.ErrorMessage = result.Errors[0].Error()
			}
			r.store.CreateAuditLog(ctx, auditEntry)
		}
	}

	// Update final status
	if r.store != nil {
		if len(allErrors) > 0 {
			r.store.UpdatePolicyStatus(ctx, policy.Metadata.Name, models.StateFailed, 
				fmt.Sprintf("%d errors during reconciliation", len(allErrors)))
		} else {
			r.store.UpdatePolicyStatus(ctx, policy.Metadata.Name, models.StateApplied,
				fmt.Sprintf("Created %d resources", len(allCreated)))
		}
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("reconciliation completed with %d errors: %v", len(allErrors), allErrors)
	}

	return nil
}

// targetTypeToConnectorType converts policy target type to connector type
func (r *Reconciler) targetTypeToConnectorType(targetType models.TargetType) connector.ConnectorType {
	switch targetType {
	case models.TargetKubernetes:
		return connector.TypeKubernetes
	case models.TargetKafka:
		return connector.TypeKafka
	case models.TargetKong:
		return connector.TypeKong
	default:
		return connector.ConnectorType(targetType)
	}
}

// Start begins the reconciliation loop
func (r *Reconciler) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("reconciler already running")
	}
	r.running = true
	r.mu.Unlock()

	ticker := time.NewTicker(r.config.ReconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.stopCh:
			return nil
		case <-ticker.C:
			r.reconcileAll(ctx)
		}
	}
}

// reconcileAll reconciles all pending policies
func (r *Reconciler) reconcileAll(ctx context.Context) {
	if r.store == nil {
		return
	}

	// Get all policies that need reconciliation
	policies, err := r.store.ListPolicies(ctx, "", "")
	if err != nil {
		return
	}

	for _, policy := range policies {
		// Skip already applied policies (unless drift check is enabled)
		// TODO: Add status filtering
		r.ReconcilePolicy(ctx, policy)
	}
}

// Stop stops the reconciliation loop
func (r *Reconciler) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		close(r.stopCh)
		r.running = false
	}
}

// DriftDetector handles anti-drift / self-healing
type DriftDetector struct {
	reconciler *Reconciler
	interval   time.Duration
	stopCh     chan struct{}
}

// NewDriftDetector creates a drift detector
func NewDriftDetector(reconciler *Reconciler, interval time.Duration) *DriftDetector {
	return &DriftDetector{
		reconciler: reconciler,
		interval:   interval,
		stopCh:     make(chan struct{}),
	}
}

// Start begins drift detection loop
func (d *DriftDetector) Start(ctx context.Context) error {
	// Check if self-healing is licensed
	if !d.reconciler.licenseMgr.IsFeatureEnabled(license.FeatureSelfHealing) {
		return fmt.Errorf("self-healing requires Enterprise license")
	}

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-d.stopCh:
			return nil
		case <-ticker.C:
			d.detectAndRepair(ctx)
		}
	}
}

// detectAndRepair checks for drift and triggers reconciliation
func (d *DriftDetector) detectAndRepair(ctx context.Context) {
	// TODO: Implement drift detection logic
	// 1. Compare current state with desired state
	// 2. If drift detected, trigger reconciliation
	// 3. Log drift events for audit
}

// Stop stops drift detection
func (d *DriftDetector) Stop() {
	close(d.stopCh)
}
