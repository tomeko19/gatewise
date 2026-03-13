// Package reconciler provides drift detection and self-healing capabilities
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

// DriftEvent represents a detected drift
type DriftEvent struct {
	ID           string                 `json:"id"`
	PolicyName   string                 `json:"policyName"`
	TargetType   string                 `json:"targetType"`
	TargetName   string                 `json:"targetName"`
	ResourceType string                 `json:"resourceType"`
	ResourceName string                 `json:"resourceName"`
	DriftType    DriftType              `json:"driftType"`
	Expected     map[string]interface{} `json:"expected"`
	Actual       map[string]interface{} `json:"actual"`
	DetectedAt   time.Time              `json:"detectedAt"`
	RepairedAt   *time.Time             `json:"repairedAt,omitempty"`
	Status       DriftStatus            `json:"status"`
}

// DriftType represents the type of drift
type DriftType string

const (
	DriftTypeModified DriftType = "modified"
	DriftTypeDeleted  DriftType = "deleted"
	DriftTypeExtra    DriftType = "extra"
)

// DriftStatus represents the status of a drift event
type DriftStatus string

const (
	DriftStatusDetected DriftStatus = "detected"
	DriftStatusRepairing DriftStatus = "repairing"
	DriftStatusRepaired  DriftStatus = "repaired"
	DriftStatusFailed    DriftStatus = "failed"
)

// DriftDetector handles drift detection and self-healing
type DriftDetector struct {
	mu           sync.RWMutex
	reconciler   *Reconciler
	licenseMgr   *license.Manager
	store        *store.PostgresStore
	interval     time.Duration
	stopCh       chan struct{}
	running      bool
	events       []DriftEvent
	eventCh      chan DriftEvent
	autoRepair   bool
}

// NewDriftDetector creates a new drift detector
func NewDriftDetector(reconciler *Reconciler, licenseMgr *license.Manager, store *store.PostgresStore, interval time.Duration) *DriftDetector {
	return &DriftDetector{
		reconciler: reconciler,
		licenseMgr: licenseMgr,
		store:      store,
		interval:   interval,
		stopCh:     make(chan struct{}),
		events:     make([]DriftEvent, 0),
		eventCh:    make(chan DriftEvent, 100),
		autoRepair: true,
	}
}

// SetAutoRepair enables or disables automatic repair
func (d *DriftDetector) SetAutoRepair(enabled bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.autoRepair = enabled
}

// Start begins the drift detection loop
func (d *DriftDetector) Start(ctx context.Context) error {
	// Check license
	if !d.licenseMgr.IsFeatureEnabled(license.FeatureSelfHealing) {
		return fmt.Errorf("self-healing requires Enterprise license")
	}

	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return fmt.Errorf("drift detector already running")
	}
	d.running = true
	d.mu.Unlock()

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	// Initial scan
	d.detectDrifts(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-d.stopCh:
			return nil
		case <-ticker.C:
			d.detectDrifts(ctx)
		}
	}
}

// Stop stops the drift detector
func (d *DriftDetector) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.running {
		close(d.stopCh)
		d.running = false
	}
}

// GetEvents returns all drift events
func (d *DriftDetector) GetEvents() []DriftEvent {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return append([]DriftEvent{}, d.events...)
}

// GetEventChannel returns the event channel for real-time updates
func (d *DriftDetector) GetEventChannel() <-chan DriftEvent {
	return d.eventCh
}

// detectDrifts checks all policies for drift
func (d *DriftDetector) detectDrifts(ctx context.Context) {
	if d.store == nil {
		return
	}

	policies, err := d.store.ListPolicies(ctx, "", "")
	if err != nil {
		return
	}

	for _, policy := range policies {
		for _, target := range policy.Spec.Targets {
			targetCopy := target
			d.detectTargetDrift(ctx, policy, &targetCopy)
		}
	}
}

// detectTargetDrift checks a specific target for drift
func (d *DriftDetector) detectTargetDrift(ctx context.Context, policy *models.Policy, target *models.Target) {
	connType := d.targetTypeToConnectorType(target.Type)
	conn, exists := d.reconciler.factory.GetConnector(connType)
	if !exists {
		return
	}

	// For K8s connector, check if resources match expected state
	if k8sConn, ok := conn.(*connector.K8sConnector); ok {
		d.detectK8sDrift(ctx, k8sConn, policy, target)
	}
}

// detectK8sDrift detects drift in Kubernetes resources
func (d *DriftDetector) detectK8sDrift(ctx context.Context, k8sConn *connector.K8sConnector, policy *models.Policy, target *models.Target) {
	if target.K8s == nil {
		return
	}

	// Check namespace labels
	nsName := target.K8s.Namespace
	expectedLabels := map[string]string{
		"gatewise.io/managed": "true",
		"gatewise.io/policy":  policy.Metadata.Name,
	}

	// This would normally use the K8s client to check actual state
	// For now, we simulate drift detection
	// In production, this would compare actual vs expected state

	// Example drift event (would be generated from actual comparison)
	/*
	event := DriftEvent{
		ID:           fmt.Sprintf("%s-%s-%d", policy.Metadata.Name, nsName, time.Now().Unix()),
		PolicyName:   policy.Metadata.Name,
		TargetType:   string(target.Type),
		TargetName:   target.Name,
		ResourceType: "Namespace",
		ResourceName: nsName,
		DriftType:    DriftTypeModified,
		Expected:     map[string]interface{}{"labels": expectedLabels},
		Actual:       map[string]interface{}{"labels": actualLabels},
		DetectedAt:   time.Now(),
		Status:       DriftStatusDetected,
	}

	d.addEvent(event)

	if d.autoRepair {
		d.repairDrift(ctx, &event, policy, target)
	}
	*/
	_ = nsName
	_ = expectedLabels
}

// addEvent adds a drift event
func (d *DriftDetector) addEvent(event DriftEvent) {
	d.mu.Lock()
	d.events = append(d.events, event)
	// Keep only last 1000 events
	if len(d.events) > 1000 {
		d.events = d.events[len(d.events)-1000:]
	}
	d.mu.Unlock()

	// Send to channel (non-blocking)
	select {
	case d.eventCh <- event:
	default:
	}
}

// repairDrift repairs a detected drift
func (d *DriftDetector) repairDrift(ctx context.Context, event *DriftEvent, policy *models.Policy, target *models.Target) {
	event.Status = DriftStatusRepairing

	// Re-reconcile the policy target
	err := d.reconciler.ReconcilePolicy(ctx, policy)
	if err != nil {
		event.Status = DriftStatusFailed
		return
	}

	now := time.Now()
	event.RepairedAt = &now
	event.Status = DriftStatusRepaired

	// Update policy status
	if d.store != nil {
		d.store.UpdatePolicyStatus(ctx, policy.Metadata.Name, models.StateApplied, "Drift repaired by self-healing")
	}
}

func (d *DriftDetector) targetTypeToConnectorType(targetType models.TargetType) connector.ConnectorType {
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
