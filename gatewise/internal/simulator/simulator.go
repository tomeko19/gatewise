// Package simulator provides policy simulation and dry-run capabilities
package simulator

import (
	"context"
	"fmt"

	"github.com/gatewise/gatewise/internal/connector"
	"github.com/gatewise/gatewise/pkg/models"
)

// SimulationResult represents the result of a policy simulation
type SimulationResult struct {
	PolicyName   string                 `json:"policyName"`
	WillCreate   []ResourceChange       `json:"willCreate"`
	WillModify   []ResourceChange       `json:"willModify"`
	WillDelete   []ResourceChange       `json:"willDelete"`
	AccessGrants []AccessChange         `json:"accessGrants"`
	AccessRevoke []AccessChange         `json:"accessRevoke"`
	Warnings     []string               `json:"warnings"`
	Errors       []string               `json:"errors"`
}

// ResourceChange represents a change to a resource
type ResourceChange struct {
	Type        string                 `json:"type"`        // "namespace", "topic", "route"
	Name        string                 `json:"name"`
	Action      string                 `json:"action"`      // "create", "update", "delete"
	CurrentSpec map[string]interface{} `json:"currentSpec,omitempty"`
	DesiredSpec map[string]interface{} `json:"desiredSpec,omitempty"`
	Diff        []string               `json:"diff,omitempty"`
}

// AccessChange represents a change in user/group access
type AccessChange struct {
	Principal  string   `json:"principal"`  // User or group name
	Resource   string   `json:"resource"`   // Namespace, topic, or route
	Operations []string `json:"operations"` // List of operations (read, write, admin)
	Reason     string   `json:"reason"`
}

// Simulator simulates policy application without making changes
type Simulator struct {
	factory *connector.Factory
}

// NewSimulator creates a new policy simulator
func NewSimulator(factory *connector.Factory) *Simulator {
	return &Simulator{
		factory: factory,
	}
}

// Simulate simulates policy application and returns predicted changes
func (s *Simulator) Simulate(ctx context.Context, policy *models.Policy) (*SimulationResult, error) {
	result := &SimulationResult{
		PolicyName:   policy.Metadata.Name,
		WillCreate:   []ResourceChange{},
		WillModify:   []ResourceChange{},
		WillDelete:   []ResourceChange{},
		AccessGrants: []AccessChange{},
		AccessRevoke: []AccessChange{},
		Warnings:     []string{},
		Errors:       []string{},
	}

	// Simulate each target
	for _, target := range policy.Spec.Targets {
		switch target.Type {
		case models.TargetKubernetes:
			if err := s.simulateKubernetes(ctx, &target, result); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Kubernetes simulation error: %v", err))
			}
		case models.TargetKafka:
			if err := s.simulateKafka(ctx, &target, result); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Kafka simulation error: %v", err))
			}
		case models.TargetKong:
			if err := s.simulateKong(ctx, &target, result); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Kong simulation error: %v", err))
			}
		}
	}

	return result, nil
}

// simulateKubernetes simulates Kubernetes resource changes
func (s *Simulator) simulateKubernetes(ctx context.Context, target *models.Target, result *SimulationResult) error {
	if target.K8s == nil {
		return nil
	}

	// Get Kubernetes connector
	conn, exists := s.factory.GetConnector(connector.TypeKubernetes)
	if !exists {
		var err error
		conn, err = s.factory.CreateConnector(connector.TypeKubernetes)
		if err != nil {
			return fmt.Errorf("failed to create Kubernetes connector: %w", err)
		}
	}

	k8sConn, ok := conn.(*connector.KubernetesConnector)
	if !ok {
		return fmt.Errorf("invalid connector type")
	}

	// Check if namespace exists
	namespace := target.K8s.Namespace
	exists, err := k8sConn.NamespaceExists(ctx, namespace)
	if err != nil {
		return err
	}

	if !exists {
		// Namespace will be created
		result.WillCreate = append(result.WillCreate, ResourceChange{
			Type:   "namespace",
			Name:   namespace,
			Action: "create",
			DesiredSpec: map[string]interface{}{
				"resources": target.K8s.Resources,
				"verbs":     target.K8s.Verbs,
			},
		})

		// All subjects will gain access
		for _, verb := range target.K8s.Verbs {
			result.AccessGrants = append(result.AccessGrants, AccessChange{
				Principal:  policy.Spec.Team,
				Resource:   namespace,
				Operations: []string{verb},
				Reason:     fmt.Sprintf("New namespace %s created with %s permission", namespace, verb),
			})
		}
	} else {
		// Namespace exists, will be modified
		result.WillModify = append(result.WillModify, ResourceChange{
			Type:   "namespace",
			Name:   namespace,
			Action: "update",
			Diff:   []string{"RBAC roles updated"},
		})

		result.Warnings = append(result.Warnings, 
			fmt.Sprintf("Namespace %s already exists - RBAC will be updated", namespace))
	}

	return nil
}

// simulateKafka simulates Kafka resource changes
func (s *Simulator) simulateKafka(ctx context.Context, target *models.Target, result *SimulationResult) error {
	if target.Kafka == nil {
		return nil
	}

	// Check if Kafka connector is available (Enterprise feature)
	_, exists := s.factory.GetConnector(connector.TypeKafka)
	if !exists {
		result.Warnings = append(result.Warnings, "Kafka connector requires Enterprise license")
		return nil
	}

	// Simulate topic creation
	for _, topic := range target.Kafka.Topics {
		result.WillCreate = append(result.WillCreate, ResourceChange{
			Type:   "kafka-topic",
			Name:   topic,
			Action: "create",
			DesiredSpec: map[string]interface{}{
				"operations": target.Kafka.Operations,
			},
		})

		// Simulate access grants
		for _, op := range target.Kafka.Operations {
			result.AccessGrants = append(result.AccessGrants, AccessChange{
				Principal:  policy.Spec.Team,
				Resource:   topic,
				Operations: []string{op},
				Reason:     fmt.Sprintf("Topic %s access with %s operation", topic, op),
			})
		}
	}

	return nil
}

// simulateKong simulates Kong resource changes
func (s *Simulator) simulateKong(ctx context.Context, target *models.Target, result *SimulationResult) error {
	if target.Kong == nil {
		return nil
	}

	// Check if Kong connector is available (Enterprise feature)
	_, exists := s.factory.GetConnector(connector.TypeKong)
	if !exists {
		result.Warnings = append(result.Warnings, "Kong connector requires Enterprise license")
		return nil
	}

	// Simulate route creation
	for _, route := range target.Kong.Routes {
		result.WillCreate = append(result.WillCreate, ResourceChange{
			Type:   "kong-route",
			Name:   route,
			Action: "create",
			DesiredSpec: map[string]interface{}{
				"route": route,
			},
		})

		// Simulate access grants
		result.AccessGrants = append(result.AccessGrants, AccessChange{
			Principal:  policy.Spec.Team,
			Resource:   route,
			Operations: []string{"access"},
			Reason:     fmt.Sprintf("Route %s configured for team access", route),
		})
	}

	return nil
}

// ComparePolicies compares two policies and returns the differences
func (s *Simulator) ComparePolicies(ctx context.Context, oldPolicy, newPolicy *models.Policy) (*SimulationResult, error) {
	result := &SimulationResult{
		PolicyName:   newPolicy.Metadata.Name,
		WillCreate:   []ResourceChange{},
		WillModify:   []ResourceChange{},
		WillDelete:   []ResourceChange{},
		AccessGrants: []AccessChange{},
		AccessRevoke: []AccessChange{},
		Warnings:     []string{},
		Errors:       []string{},
	}

	// Compare team changes
	if oldPolicy.Spec.Team != newPolicy.Spec.Team {
		result.Warnings = append(result.Warnings, 
			fmt.Sprintf("Team changed from %s to %s - access will be transferred", 
				oldPolicy.Spec.Team, newPolicy.Spec.Team))
		
		// All old team members lose access
		result.AccessRevoke = append(result.AccessRevoke, AccessChange{
			Principal: oldPolicy.Spec.Team,
			Resource:  "all",
			Reason:    "Team reassignment",
		})
		
		// New team members gain access
		result.AccessGrants = append(result.AccessGrants, AccessChange{
			Principal: newPolicy.Spec.Team,
			Resource:  "all",
			Reason:    "Team reassignment",
		})
	}

	// Simple diff - in production this would be more sophisticated
	result.WillModify = append(result.WillModify, ResourceChange{
		Type:   "policy",
		Name:   newPolicy.Metadata.Name,
		Action: "update",
		Diff:   []string{"Policy spec updated"},
	})

	return result, nil
}
