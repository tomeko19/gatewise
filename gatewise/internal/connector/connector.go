// Package connector defines the interface for infrastructure connectors
package connector

import (
	"context"

	"github.com/gatewise/gatewise/pkg/models"
)

// ConnectorMode represents how the connector connects to infrastructure
type ConnectorMode string

const (
	// ModeEmbedded - Gatewise manages the infrastructure (deployed via Helm)
	ModeEmbedded ConnectorMode = "embedded"
	// ModeExternal - Connect to existing customer infrastructure (BYOI)
	ModeExternal ConnectorMode = "external"
)

// ConnectorType represents the type of infrastructure
type ConnectorType string

const (
	TypeKubernetes ConnectorType = "kubernetes"
	TypeKafka      ConnectorType = "kafka"
	TypeKong       ConnectorType = "kong"
	TypeKeycloak   ConnectorType = "keycloak"
	TypeCerbos     ConnectorType = "cerbos"
)

// ConnectorConfig holds configuration for a connector
type ConnectorConfig struct {
	Type       ConnectorType         `yaml:"type" json:"type"`
	Mode       ConnectorMode         `yaml:"mode" json:"mode"`
	Enabled    bool                  `yaml:"enabled" json:"enabled"`
	Endpoint   string                `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Credentials map[string]string    `yaml:"credentials,omitempty" json:"credentials,omitempty"`
	Options    map[string]interface{} `yaml:"options,omitempty" json:"options,omitempty"`
}

// ReconcileResult represents the result of a reconciliation
type ReconcileResult struct {
	Success     bool
	Message     string
	ResourcesCreated []string
	ResourcesUpdated []string
	ResourcesDeleted []string
	Errors      []error
}

// Connector is the interface all infrastructure connectors must implement
type Connector interface {
	// Type returns the connector type
	Type() ConnectorType

	// Mode returns whether this is embedded or external
	Mode() ConnectorMode

	// Initialize sets up the connector
	Initialize(ctx context.Context) error

	// Reconcile applies the policy target to the infrastructure
	Reconcile(ctx context.Context, policy *models.Policy, target *models.Target) (*ReconcileResult, error)

	// Delete removes resources created by a policy
	Delete(ctx context.Context, policy *models.Policy, target *models.Target) error

	// HealthCheck verifies connectivity
	HealthCheck(ctx context.Context) error

	// Close cleans up resources
	Close() error
}

// CapabilityDiscovery checks if infrastructure is available
type CapabilityDiscovery interface {
	// Discover checks if the target infrastructure exists
	Discover(ctx context.Context) (bool, error)

	// GetCapabilities returns available features
	GetCapabilities(ctx context.Context) ([]string, error)
}
