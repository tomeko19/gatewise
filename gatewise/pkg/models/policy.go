// Package models defines the core data structures for Gatewise policies
package models

import (
	"time"
)

// PolicyKind represents the type of policy
type PolicyKind string

const (
	KindAccessPolicy  PolicyKind = "AccessPolicy"
	KindTeamBinding   PolicyKind = "TeamBinding"
	KindResourceQuota PolicyKind = "ResourceQuota"
)

// Policy is the root structure for all Gatewise policies
type Policy struct {
	APIVersion string            `yaml:"apiVersion" json:"apiVersion"`
	Kind       PolicyKind        `yaml:"kind" json:"kind"`
	Metadata   PolicyMetadata    `yaml:"metadata" json:"metadata"`
	Spec       PolicySpec        `yaml:"spec" json:"spec"`
}

// PolicyMetadata contains policy identification information
type PolicyMetadata struct {
	Name        string            `yaml:"name" json:"name"`
	Namespace   string            `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty" json:"annotations,omitempty"`
	CreatedAt   time.Time         `yaml:"-" json:"createdAt,omitempty"`
	UpdatedAt   time.Time         `yaml:"-" json:"updatedAt,omitempty"`
}

// PolicySpec contains the policy specification
type PolicySpec struct {
	// Team information
	Team       string   `yaml:"team,omitempty" json:"team,omitempty"`
	Members    []string `yaml:"members,omitempty" json:"members,omitempty"`
	
	// Access targets
	Targets    []Target `yaml:"targets,omitempty" json:"targets,omitempty"`
	
	// Just-In-Time access configuration
	JIT        *JITConfig `yaml:"jit,omitempty" json:"jit,omitempty"`
	
	// Resource quotas for Kubernetes
	Quotas     *QuotaSpec `yaml:"quotas,omitempty" json:"quotas,omitempty"`
}

// Target defines access to a specific infrastructure component
type Target struct {
	Type       TargetType  `yaml:"type" json:"type"`
	Name       string      `yaml:"name" json:"name"`
	Actions    []string    `yaml:"actions" json:"actions"`
	K8s        *K8sConfig  `yaml:"kubernetes,omitempty" json:"kubernetes,omitempty"`
	Kafka      *KafkaConfig `yaml:"kafka,omitempty" json:"kafka,omitempty"`
	Kong       *KongConfig  `yaml:"kong,omitempty" json:"kong,omitempty"`
}

// TargetType represents infrastructure component type
type TargetType string

const (
	TargetKubernetes TargetType = "kubernetes"
	TargetKafka      TargetType = "kafka"
	TargetKong       TargetType = "kong"
)

// K8sConfig contains Kubernetes-specific configuration
type K8sConfig struct {
	Namespace  string   `yaml:"namespace" json:"namespace"`
	Resources  []string `yaml:"resources" json:"resources"`
	Verbs      []string `yaml:"verbs" json:"verbs"`
}

// KafkaConfig contains Kafka-specific configuration  
type KafkaConfig struct {
	Topics     []string `yaml:"topics" json:"topics"`
	Operations []string `yaml:"operations" json:"operations"`
	ConsumerGroup string `yaml:"consumerGroup,omitempty" json:"consumerGroup,omitempty"`
}

// KongConfig contains Kong Gateway configuration
type KongConfig struct {
	Routes     []string `yaml:"routes" json:"routes"`
	Services   []string `yaml:"services" json:"services"`
	Plugins    []KongPlugin `yaml:"plugins,omitempty" json:"plugins,omitempty"`
}

// KongPlugin represents a Kong plugin configuration
type KongPlugin struct {
	Name   string                 `yaml:"name" json:"name"`
	Config map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
}

// JITConfig defines Just-In-Time access parameters
type JITConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	Duration   string `yaml:"duration" json:"duration"`
	MaxRenewals int   `yaml:"maxRenewals,omitempty" json:"maxRenewals,omitempty"`
	ApprovalRequired bool `yaml:"approvalRequired,omitempty" json:"approvalRequired,omitempty"`
}

// QuotaSpec defines Kubernetes resource quotas
type QuotaSpec struct {
	CPU       string `yaml:"cpu,omitempty" json:"cpu,omitempty"`
	Memory    string `yaml:"memory,omitempty" json:"memory,omitempty"`
	Pods      int    `yaml:"pods,omitempty" json:"pods,omitempty"`
	Services  int    `yaml:"services,omitempty" json:"services,omitempty"`
	Secrets   int    `yaml:"secrets,omitempty" json:"secrets,omitempty"`
}

// PolicyStatus represents the current state of a policy
type PolicyStatus struct {
	State       PolicyState `json:"state"`
	Message     string      `json:"message,omitempty"`
	LastApplied time.Time   `json:"lastApplied,omitempty"`
	DriftDetected bool      `json:"driftDetected,omitempty"`
}

// PolicyState represents the policy synchronization state
type PolicyState string

const (
	StatePending    PolicyState = "Pending"
	StateApplied    PolicyState = "Applied"
	StateFailed     PolicyState = "Failed"
	StateDrifted    PolicyState = "Drifted"
	StateReconciling PolicyState = "Reconciling"
)
