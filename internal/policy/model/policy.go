// Package model provides data structures for policy management.
package model

// Policy represents a multi-tenant authorization policy definition.
type Policy struct {
	Tenant      string   `json:"tenant" yaml:"tenant"`
	Role        string   `json:"role" yaml:"role"`
	Permissions []string `json:"permissions" yaml:"permissions"`
}
