// Package policy provides validation for Gatewise policies
package policy

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gatewise/gatewise/pkg/models"
)

// ValidationError represents a policy validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationResult contains all validation errors
type ValidationResult struct {
	Errors   []ValidationError
	Warnings []string
}

// IsValid returns true if there are no validation errors
func (r *ValidationResult) IsValid() bool {
	return len(r.Errors) == 0
}

// Validator handles policy validation
type Validator struct {
	forbiddenNames    []string
	maxNameLength     int
	validAPIVersions  []string
}

// NewValidator creates a new policy validator with default rules
func NewValidator() *Validator {
	return &Validator{
		forbiddenNames: []string{
			"kube-system",
			"kube-public",
			"kube-node-lease",
			"default",
			"admin",
			"root",
		},
		maxNameLength: 63, // Kubernetes naming convention
		validAPIVersions: []string{
			"gatewise.io/v1alpha1",
			"gatewise.io/v1beta1",
			"gatewise.io/v1",
		},
	}
}

// Validate performs comprehensive validation on a policy
func (v *Validator) Validate(policy *models.Policy) *ValidationResult {
	result := &ValidationResult{}

	// Validate API Version
	v.validateAPIVersion(policy.APIVersion, result)

	// Validate Kind
	v.validateKind(policy.Kind, result)

	// Validate Metadata
	v.validateMetadata(&policy.Metadata, result)

	// Validate Spec based on Kind
	v.validateSpec(policy.Kind, &policy.Spec, result)

	return result
}

func (v *Validator) validateAPIVersion(version string, result *ValidationResult) {
	if version == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "apiVersion",
			Message: "apiVersion is required",
		})
		return
	}

	valid := false
	for _, validVersion := range v.validAPIVersions {
		if version == validVersion {
			valid = true
			break
		}
	}

	if !valid {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "apiVersion",
			Message: fmt.Sprintf("invalid apiVersion '%s', must be one of: %v", version, v.validAPIVersions),
		})
	}
}

func (v *Validator) validateKind(kind models.PolicyKind, result *ValidationResult) {
	if kind == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "kind",
			Message: "kind is required",
		})
		return
	}

	validKinds := []models.PolicyKind{
		models.KindAccessPolicy,
		models.KindTeamBinding,
		models.KindResourceQuota,
	}

	valid := false
	for _, validKind := range validKinds {
		if kind == validKind {
			valid = true
			break
		}
	}

	if !valid {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "kind",
			Message: fmt.Sprintf("invalid kind '%s', must be one of: %v", kind, validKinds),
		})
	}
}

func (v *Validator) validateMetadata(meta *models.PolicyMetadata, result *ValidationResult) {
	// Validate name
	if meta.Name == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "metadata.name",
			Message: "name is required",
		})
	} else {
		// Check name format (DNS-1123 subdomain)
		if !v.isValidDNSName(meta.Name) {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "metadata.name",
				Message: "name must be a valid DNS-1123 subdomain (lowercase, alphanumeric, hyphens)",
			})
		}

		// Check forbidden names
		for _, forbidden := range v.forbiddenNames {
			if strings.EqualFold(meta.Name, forbidden) {
				result.Errors = append(result.Errors, ValidationError{
					Field:   "metadata.name",
					Message: fmt.Sprintf("name '%s' is forbidden", meta.Name),
				})
			}
		}

		// Check name length
		if len(meta.Name) > v.maxNameLength {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "metadata.name",
				Message: fmt.Sprintf("name exceeds maximum length of %d characters", v.maxNameLength),
			})
		}
	}

	// Validate labels
	for key, value := range meta.Labels {
		if !v.isValidLabelKey(key) {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("metadata.labels[%s]", key),
				Message: "invalid label key format",
			})
		}
		if !v.isValidLabelValue(value) {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("metadata.labels[%s]", key),
				Message: "invalid label value format",
			})
		}
	}
}

func (v *Validator) validateSpec(kind models.PolicyKind, spec *models.PolicySpec, result *ValidationResult) {
	switch kind {
	case models.KindAccessPolicy:
		v.validateAccessPolicySpec(spec, result)
	case models.KindTeamBinding:
		v.validateTeamBindingSpec(spec, result)
	case models.KindResourceQuota:
		v.validateResourceQuotaSpec(spec, result)
	}
}

func (v *Validator) validateAccessPolicySpec(spec *models.PolicySpec, result *ValidationResult) {
	if len(spec.Targets) == 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "spec.targets",
			Message: "at least one target is required for AccessPolicy",
		})
		return
	}

	for i, target := range spec.Targets {
		// Validate target type
		if target.Type == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("spec.targets[%d].type", i),
				Message: "target type is required",
			})
		}

		// Validate target name
		if target.Name == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("spec.targets[%d].name", i),
				Message: "target name is required",
			})
		}

		// Validate type-specific configuration
		v.validateTargetConfig(&target, i, result)
	}

	// Validate JIT configuration if present
	if spec.JIT != nil && spec.JIT.Enabled {
		v.validateJITConfig(spec.JIT, result)
	}
}

func (v *Validator) validateTargetConfig(target *models.Target, index int, result *ValidationResult) {
	switch target.Type {
	case models.TargetKubernetes:
		if target.K8s == nil {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("spec.targets[%d].kubernetes", index),
				Message: "kubernetes configuration is required for kubernetes target",
			})
		} else {
			if target.K8s.Namespace == "" {
				result.Errors = append(result.Errors, ValidationError{
					Field:   fmt.Sprintf("spec.targets[%d].kubernetes.namespace", index),
					Message: "namespace is required",
				})
			}
		}
	case models.TargetKafka:
		if target.Kafka == nil {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("spec.targets[%d].kafka", index),
				Message: "kafka configuration is required for kafka target",
			})
		} else {
			if len(target.Kafka.Topics) == 0 {
				result.Errors = append(result.Errors, ValidationError{
					Field:   fmt.Sprintf("spec.targets[%d].kafka.topics", index),
					Message: "at least one topic is required",
				})
			}
		}
	case models.TargetKong:
		if target.Kong == nil {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("spec.targets[%d].kong", index),
				Message: "kong configuration is required for kong target",
			})
		}
	}
}

func (v *Validator) validateTeamBindingSpec(spec *models.PolicySpec, result *ValidationResult) {
	if spec.Team == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "spec.team",
			Message: "team name is required for TeamBinding",
		})
	}

	if len(spec.Members) == 0 {
		result.Warnings = append(result.Warnings, "TeamBinding has no members defined")
	}
}

func (v *Validator) validateResourceQuotaSpec(spec *models.PolicySpec, result *ValidationResult) {
	if spec.Quotas == nil {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "spec.quotas",
			Message: "quotas are required for ResourceQuota",
		})
	}
}

func (v *Validator) validateJITConfig(jit *models.JITConfig, result *ValidationResult) {
	if jit.Duration == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "spec.jit.duration",
			Message: "duration is required when JIT is enabled",
		})
		return
	}

	// Validate duration format (e.g., "2h", "30m", "1h30m")
	_, err := time.ParseDuration(jit.Duration)
	if err != nil {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "spec.jit.duration",
			Message: fmt.Sprintf("invalid duration format: %s", jit.Duration),
		})
	}
}

// DNS-1123 subdomain validation
func (v *Validator) isValidDNSName(name string) bool {
	pattern := `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	matched, _ := regexp.MatchString(pattern, name)
	return matched && len(name) <= v.maxNameLength
}

// Label key validation
func (v *Validator) isValidLabelKey(key string) bool {
	// Simplified validation - allows prefix/name format
	pattern := `^([a-zA-Z0-9][-a-zA-Z0-9_.]*)?[a-zA-Z0-9]$|^[a-zA-Z0-9]$`
	matched, _ := regexp.MatchString(pattern, key)
	return matched && len(key) <= 253
}

// Label value validation
func (v *Validator) isValidLabelValue(value string) bool {
	if value == "" {
		return true // Empty values are allowed
	}
	pattern := `^[a-zA-Z0-9]([-a-zA-Z0-9_.]*[a-zA-Z0-9])?$`
	matched, _ := regexp.MatchString(pattern, value)
	return matched && len(value) <= 63
}
