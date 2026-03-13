// Package connector provides stub connectors for Community Edition
// Enterprise connectors (Kafka, Kong) are available in Gatewise Enterprise
package connector

import (
	"fmt"

	"github.com/gatewise/gatewise/pkg/models"
)

// KafkaConnector stub for Community Edition
type KafkaConnector struct{}

// NewKafkaConnector returns an error in Community Edition
func NewKafkaConnector(config map[string]string) (*KafkaConnector, error) {
	return nil, fmt.Errorf("kafka connector requires Gatewise Enterprise license")
}

// Apply returns an error
func (k *KafkaConnector) Apply(target *models.Target) ([]string, error) {
	return nil, fmt.Errorf("kafka connector requires Enterprise license")
}

// Validate returns an error
func (k *KafkaConnector) Validate(target *models.Target) error {
	return fmt.Errorf("kafka connector requires Enterprise license")
}

// KongConnector stub for Community Edition
type KongConnector struct{}

// NewKongConnector returns an error in Community Edition
func NewKongConnector(config map[string]string) (*KongConnector, error) {
	return nil, fmt.Errorf("kong connector requires Gatewise Enterprise license")
}

// Apply returns an error
func (k *KongConnector) Apply(target *models.Target) ([]string, error) {
	return nil, fmt.Errorf("kong connector requires Enterprise license")
}

// Validate returns an error
func (k *KongConnector) Validate(target *models.Target) error {
	return fmt.Errorf("kong connector requires Enterprise license")
}
