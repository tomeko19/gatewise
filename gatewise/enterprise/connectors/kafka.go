// Package connector provides the Kafka connector implementation
package connector

import (
	"context"
	"fmt"
	"strings"

	"github.com/IBM/sarama"
	"github.com/gatewise/gatewise/pkg/models"
)

// KafkaConnector implements the Connector interface for Kafka
type KafkaConnector struct {
	admin      sarama.ClusterAdmin
	config     *sarama.Config
	brokers    []string
	mode       ConnectorMode
	connected  bool
}

// KafkaConnectorConfig holds Kafka-specific configuration
type KafkaConnectorConfig struct {
	Brokers       []string
	SASLEnabled   bool
	SASLMechanism string
	SASLUser      string
	SASLPassword  string
	TLSEnabled    bool
}

// NewKafkaConnector creates a new Kafka connector
func NewKafkaConnector(brokers []string, mode ConnectorMode) (*KafkaConnector, error) {
	return &KafkaConnector{
		brokers: brokers,
		mode:    mode,
	}, nil
}

// Type returns the connector type
func (k *KafkaConnector) Type() ConnectorType {
	return TypeKafka
}

// Mode returns the connector mode
func (k *KafkaConnector) Mode() ConnectorMode {
	return k.mode
}

// Initialize sets up the Kafka admin client
func (k *KafkaConnector) Initialize(ctx context.Context) error {
	config := sarama.NewConfig()
	config.Version = sarama.V2_8_0_0
	config.Admin.Timeout = 10000 // 10 seconds

	// Create admin client
	admin, err := sarama.NewClusterAdmin(k.brokers, config)
	if err != nil {
		return fmt.Errorf("failed to create Kafka admin client: %w", err)
	}

	k.admin = admin
	k.config = config
	k.connected = true
	return nil
}

// HealthCheck verifies connectivity to Kafka
func (k *KafkaConnector) HealthCheck(ctx context.Context) error {
	if k.admin == nil {
		return fmt.Errorf("connector not initialized")
	}

	// Try to list topics as a health check
	_, err := k.admin.ListTopics()
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka: %w", err)
	}
	return nil
}

// Reconcile applies the Kafka resources defined in the policy
func (k *KafkaConnector) Reconcile(ctx context.Context, policy *models.Policy, target *models.Target) (*ReconcileResult, error) {
	if k.admin == nil {
		return nil, fmt.Errorf("connector not initialized")
	}

	if target.Kafka == nil {
		return nil, fmt.Errorf("kafka configuration missing in target")
	}

	result := &ReconcileResult{Success: true}

	// Step 1: Create/Verify Topics
	for _, topicName := range target.Kafka.Topics {
		err := k.reconcileTopic(ctx, topicName, policy)
		if err != nil {
			result.Errors = append(result.Errors, err)
			result.Success = false
		} else {
			result.ResourcesCreated = append(result.ResourcesCreated, fmt.Sprintf("Topic/%s", topicName))
		}
	}

	// Step 2: Create ACLs for team members
	for _, member := range policy.Spec.Members {
		for _, topicName := range target.Kafka.Topics {
			err := k.reconcileACL(ctx, topicName, member, target.Kafka.Operations, policy)
			if err != nil {
				result.Errors = append(result.Errors, err)
				result.Success = false
			} else {
				result.ResourcesCreated = append(result.ResourcesCreated, 
					fmt.Sprintf("ACL/%s->%s", member, topicName))
			}
		}
	}

	// Step 3: Create Consumer Group ACL if specified
	if target.Kafka.ConsumerGroup != "" {
		for _, member := range policy.Spec.Members {
			err := k.reconcileConsumerGroupACL(ctx, target.Kafka.ConsumerGroup, member, policy)
			if err != nil {
				result.Errors = append(result.Errors, err)
				result.Success = false
			} else {
				result.ResourcesCreated = append(result.ResourcesCreated,
					fmt.Sprintf("ACL/ConsumerGroup/%s->%s", member, target.Kafka.ConsumerGroup))
			}
		}
	}

	if result.Success {
		result.Message = fmt.Sprintf("Successfully reconciled %d Kafka resources", len(result.ResourcesCreated))
	} else {
		result.Message = fmt.Sprintf("Reconciliation completed with %d errors", len(result.Errors))
	}

	return result, nil
}

// reconcileTopic creates a topic if it doesn't exist
func (k *KafkaConnector) reconcileTopic(ctx context.Context, topicName string, policy *models.Policy) error {
	// Check if topic exists
	topics, err := k.admin.ListTopics()
	if err != nil {
		return fmt.Errorf("failed to list topics: %w", err)
	}

	if _, exists := topics[topicName]; exists {
		return nil // Topic already exists
	}

	// Create topic with default configuration
	topicDetail := &sarama.TopicDetail{
		NumPartitions:     3,
		ReplicationFactor: 1, // Should be configurable
		ConfigEntries: map[string]*string{
			// Add Gatewise metadata as topic config
		},
	}

	err = k.admin.CreateTopic(topicName, topicDetail, false)
	if err != nil {
		// Ignore "topic already exists" error
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return fmt.Errorf("failed to create topic %s: %w", topicName, err)
	}

	return nil
}

// reconcileACL creates ACLs for a user on a topic
func (k *KafkaConnector) reconcileACL(ctx context.Context, topicName, principal string, operations []string, policy *models.Policy) error {
	for _, op := range operations {
		acl := &sarama.ResourceAcls{
			Resource: sarama.Resource{
				ResourceType:        sarama.AclResourceTopic,
				ResourceName:        topicName,
				ResourcePatternType: sarama.AclPatternLiteral,
			},
			Acls: []*sarama.Acl{
				{
					Principal:      fmt.Sprintf("User:%s", principal),
					Host:           "*",
					Operation:      k.parseOperation(op),
					PermissionType: sarama.AclPermissionAllow,
				},
			},
		}

		err := k.admin.CreateACLs([]*sarama.ResourceAcls{acl})
		if err != nil {
			return fmt.Errorf("failed to create ACL for %s on %s: %w", principal, topicName, err)
		}
	}

	return nil
}

// reconcileConsumerGroupACL creates ACLs for consumer group access
func (k *KafkaConnector) reconcileConsumerGroupACL(ctx context.Context, groupName, principal string, policy *models.Policy) error {
	acl := &sarama.ResourceAcls{
		Resource: sarama.Resource{
			ResourceType:        sarama.AclResourceGroup,
			ResourceName:        groupName,
			ResourcePatternType: sarama.AclPatternLiteral,
		},
		Acls: []*sarama.Acl{
			{
				Principal:      fmt.Sprintf("User:%s", principal),
				Host:           "*",
				Operation:      sarama.AclOperationRead,
				PermissionType: sarama.AclPermissionAllow,
			},
		},
	}

	err := k.admin.CreateACLs([]*sarama.ResourceAcls{acl})
	if err != nil {
		return fmt.Errorf("failed to create consumer group ACL: %w", err)
	}

	return nil
}

// parseOperation converts string operation to sarama.AclOperation
func (k *KafkaConnector) parseOperation(op string) sarama.AclOperation {
	switch strings.ToUpper(op) {
	case "READ":
		return sarama.AclOperationRead
	case "WRITE":
		return sarama.AclOperationWrite
	case "CREATE":
		return sarama.AclOperationCreate
	case "DELETE":
		return sarama.AclOperationDelete
	case "ALTER":
		return sarama.AclOperationAlter
	case "DESCRIBE":
		return sarama.AclOperationDescribe
	case "ALL":
		return sarama.AclOperationAll
	default:
		return sarama.AclOperationRead
	}
}

// Delete removes Kafka resources created by a policy
func (k *KafkaConnector) Delete(ctx context.Context, policy *models.Policy, target *models.Target) error {
	if k.admin == nil {
		return fmt.Errorf("connector not initialized")
	}

	var errs []error

	// Delete ACLs for each member
	for _, member := range policy.Spec.Members {
		filter := sarama.AclFilter{
			ResourceType:              sarama.AclResourceTopic,
			ResourcePatternTypeFilter: sarama.AclPatternLiteral,
			Principal:                 stringPtr(fmt.Sprintf("User:%s", member)),
			PermissionType:            sarama.AclPermissionAllow,
			Operation:                 sarama.AclOperationAny,
		}

		_, err := k.admin.DeleteACL(filter, false)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to delete ACLs for %s: %w", member, err))
		}
	}

	// Note: We don't delete topics by default to prevent data loss

	if len(errs) > 0 {
		return fmt.Errorf("deletion completed with errors: %v", errs)
	}
	return nil
}

// Close closes the Kafka admin client
func (k *KafkaConnector) Close() error {
	if k.admin != nil {
		return k.admin.Close()
	}
	return nil
}

// Discover implements CapabilityDiscovery
func (k *KafkaConnector) Discover(ctx context.Context) (bool, error) {
	if k.admin == nil {
		if err := k.Initialize(ctx); err != nil {
			return false, nil
		}
	}

	if err := k.HealthCheck(ctx); err != nil {
		return false, nil
	}
	return true, nil
}

// GetCapabilities returns available Kafka features
func (k *KafkaConnector) GetCapabilities(ctx context.Context) ([]string, error) {
	return []string{
		"topics",
		"acls",
		"consumer_groups",
	}, nil
}

func stringPtr(s string) *string {
	return &s
}
