// Package connector provides factory for creating infrastructure connectors
package connector

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gatewise/gatewise/pkg/license"
)

// Factory creates and manages connectors
type Factory struct {
	mu           sync.RWMutex
	connectors   map[ConnectorType]Connector
	configs      map[ConnectorType]*ConnectorConfig
	licenseMgr   *license.Manager
}

// NewFactory creates a new connector factory
func NewFactory(licenseMgr *license.Manager) *Factory {
	return &Factory{
		connectors: make(map[ConnectorType]Connector),
		configs:    make(map[ConnectorType]*ConnectorConfig),
		licenseMgr: licenseMgr,
	}
}

// RegisterConfig adds a connector configuration
func (f *Factory) RegisterConfig(config *ConnectorConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check license for enterprise connectors
	if !f.isConnectorAllowed(config.Type) {
		return fmt.Errorf("connector %s requires Enterprise license", config.Type)
	}

	f.configs[config.Type] = config
	return nil
}

// isConnectorAllowed checks if the connector is allowed by license
func (f *Factory) isConnectorAllowed(connType ConnectorType) bool {
	switch connType {
	case TypeKubernetes:
		// K8s basic is always allowed (Community)
		return true
	case TypeKafka:
		return f.licenseMgr.IsFeatureEnabled(license.FeatureKafkaConnector)
	case TypeKong:
		return f.licenseMgr.IsFeatureEnabled(license.FeatureKongConnector)
	case TypeKeycloak:
		return f.licenseMgr.IsFeatureEnabled(license.FeatureKeycloakSSO)
	case TypeCerbos:
		return f.licenseMgr.IsFeatureEnabled(license.FeatureCerbosAuthz)
	default:
		return false
	}
}

// CreateConnector creates a connector based on configuration
func (f *Factory) CreateConnector(connType ConnectorType) (Connector, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if already created
	if conn, exists := f.connectors[connType]; exists {
		return conn, nil
	}

	// Get config
	config, exists := f.configs[connType]
	if !exists {
		return nil, fmt.Errorf("no configuration found for connector %s", connType)
	}

	if !config.Enabled {
		return nil, fmt.Errorf("connector %s is disabled", connType)
	}

	// Create connector based on type and mode
	var conn Connector
	var err error

	switch connType {
	case TypeKubernetes:
		conn, err = f.createKubernetesConnector(config)
	case TypeKafka:
		conn, err = f.createKafkaConnector(config)
	case TypeKong:
		conn, err = f.createKongConnector(config)
	default:
		return nil, fmt.Errorf("unknown connector type: %s", connType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create %s connector: %w", connType, err)
	}

	f.connectors[connType] = conn
	return conn, nil
}

// createKubernetesConnector creates K8s connector based on mode
func (f *Factory) createKubernetesConnector(config *ConnectorConfig) (Connector, error) {
	switch config.Mode {
	case ModeEmbedded:
		// In-cluster configuration
		return NewK8sConnector("", config.Mode)
	case ModeExternal:
		// External kubeconfig
		kubeconfig := config.Credentials["kubeconfig"]
		return NewK8sConnector(kubeconfig, config.Mode)
	default:
		return nil, fmt.Errorf("unknown mode: %s", config.Mode)
	}
}

// createKafkaConnector creates Kafka connector based on mode
func (f *Factory) createKafkaConnector(config *ConnectorConfig) (Connector, error) {
	brokers := []string{}
	if brokersStr, ok := config.Credentials["brokers"]; ok {
		brokers = strings.Split(brokersStr, ",")
	}
	if config.Endpoint != "" {
		brokers = append(brokers, config.Endpoint)
	}
	if len(brokers) == 0 {
		return nil, fmt.Errorf("no Kafka brokers configured")
	}
	return NewKafkaConnector(brokers, config.Mode)
}

// createKongConnector creates Kong connector based on mode  
func (f *Factory) createKongConnector(config *ConnectorConfig) (Connector, error) {
	adminURL := config.Endpoint
	if adminURL == "" {
		adminURL = config.Credentials["admin_url"]
	}
	if adminURL == "" {
		return nil, fmt.Errorf("no Kong admin URL configured")
	}
	apiKey := config.Credentials["api_key"]
	return NewKongConnector(adminURL, apiKey, config.Mode)
}

// GetConnector returns an existing connector
func (f *Factory) GetConnector(connType ConnectorType) (Connector, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	conn, exists := f.connectors[connType]
	return conn, exists
}

// CloseAll closes all connectors
func (f *Factory) CloseAll() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var lastErr error
	for _, conn := range f.connectors {
		if err := conn.Close(); err != nil {
			lastErr = err
		}
	}
	f.connectors = make(map[ConnectorType]Connector)
	return lastErr
}
