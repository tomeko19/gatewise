// Package license manages Gatewise licensing tiers
package license

import (
	"sync"
)

// Tier represents the license tier
type Tier string

const (
	TierCommunity  Tier = "community"
	TierEnterprise Tier = "enterprise"
)

// Feature represents a licensable feature
type Feature string

const (
	// Community Features
	FeatureK8sBasicRBAC    Feature = "k8s_basic_rbac"
	FeatureK8sNamespace    Feature = "k8s_namespace"
	FeatureK8sQuota        Feature = "k8s_quota"
	FeaturePolicyParsing   Feature = "policy_parsing"
	FeatureAuditLog        Feature = "audit_log"

	// Enterprise Features
	FeatureK8sAdvancedRBAC Feature = "k8s_advanced_rbac"
	FeatureKafkaConnector  Feature = "kafka_connector"
	FeatureKongConnector   Feature = "kong_connector"
	FeatureKeycloakSSO     Feature = "keycloak_sso"
	FeatureCerbosAuthz     Feature = "cerbos_authz"
	FeatureJITAccess       Feature = "jit_access"
	FeatureSelfHealing     Feature = "self_healing"
	FeatureMultiTenant     Feature = "multi_tenant"
	FeatureExternalConnect Feature = "external_connect"
)

// Manager handles license validation
type Manager struct {
	mu           sync.RWMutex
	currentTier  Tier
	licenseKey   string
	features     map[Feature]bool
}

// communityFeatures lists features available in Community tier
var communityFeatures = []Feature{
	FeatureK8sBasicRBAC,
	FeatureK8sNamespace,
	FeatureK8sQuota,
	FeaturePolicyParsing,
	FeatureAuditLog,
}

// enterpriseFeatures lists features available in Enterprise tier
var enterpriseFeatures = []Feature{
	FeatureK8sAdvancedRBAC,
	FeatureKafkaConnector,
	FeatureKongConnector,
	FeatureKeycloakSSO,
	FeatureCerbosAuthz,
	FeatureJITAccess,
	FeatureSelfHealing,
	FeatureMultiTenant,
	FeatureExternalConnect,
}

// NewManager creates a new license manager
func NewManager() *Manager {
	m := &Manager{
		currentTier: TierCommunity,
		features:    make(map[Feature]bool),
	}
	m.loadFeatures()
	return m
}

// loadFeatures initializes available features based on tier
func (m *Manager) loadFeatures() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear features
	m.features = make(map[Feature]bool)

	// Add community features
	for _, f := range communityFeatures {
		m.features[f] = true
	}

	// Add enterprise features if licensed
	if m.currentTier == TierEnterprise {
		for _, f := range enterpriseFeatures {
			m.features[f] = true
		}
	}
}

// SetLicenseKey validates and sets the license key
func (m *Manager) SetLicenseKey(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// TODO: Implement actual license validation
	// For now, any key starting with "ENT-" enables enterprise
	if len(key) > 4 && key[:4] == "ENT-" {
		m.currentTier = TierEnterprise
		m.licenseKey = key
	} else {
		m.currentTier = TierCommunity
		m.licenseKey = ""
	}

	m.loadFeatures()
	return nil
}

// IsFeatureEnabled checks if a feature is available
func (m *Manager) IsFeatureEnabled(feature Feature) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.features[feature]
}

// GetTier returns the current license tier
func (m *Manager) GetTier() Tier {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentTier
}

// ListEnabledFeatures returns all enabled features
func (m *Manager) ListEnabledFeatures() []Feature {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var features []Feature
	for f, enabled := range m.features {
		if enabled {
			features = append(features, f)
		}
	}
	return features
}
