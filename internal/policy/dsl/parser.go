package dsl

import (
	"errors"
	"strings"

	"github.com/tomeko19/gatewise/internal/policy/model"
	"gopkg.in/yaml.v3"
)

var (
	// ErrInvalidPolicy indicates the policy file is invalid.
	ErrInvalidPolicy = errors.New("invalid policy")
)

// ParsePolicyYAML parses a policy YAML into a Policy model and performs minimal validation.
func ParsePolicyYAML(b []byte) (*model.Policy, error) {
	var p model.Policy
	if err := yaml.Unmarshal(b, &p); err != nil {
		return nil, err
	}

	p.Tenant = strings.TrimSpace(p.Tenant)
	p.Role = strings.TrimSpace(p.Role)

	if p.Tenant == "" || p.Role == "" {
		return nil, ErrInvalidPolicy
	}
	if len(p.Permissions) == 0 {
		return nil, ErrInvalidPolicy
	}
	for i := range p.Permissions {
		p.Permissions[i] = strings.TrimSpace(p.Permissions[i])
		if p.Permissions[i] == "" {
			return nil, ErrInvalidPolicy
		}
	}

	return &p, nil
}
