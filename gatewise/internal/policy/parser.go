// Package policy handles parsing and validation of Gatewise policy files
package policy

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gatewise/gatewise/pkg/models"
	"gopkg.in/yaml.v3"
)

// Parser handles YAML policy parsing
type Parser struct {
	strict bool
}

// NewParser creates a new policy parser
func NewParser(strict bool) *Parser {
	return &Parser{strict: strict}
}

// ParseFile reads and parses a policy from a file path
func (p *Parser) ParseFile(path string) (*models.Policy, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open policy file: %w", err)
	}
	defer file.Close()

	return p.Parse(file)
}

// Parse reads and parses a policy from an io.Reader
func (p *Parser) Parse(r io.Reader) (*models.Policy, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy data: %w", err)
	}

	return p.ParseBytes(data)
}

// ParseBytes parses a policy from raw bytes
func (p *Parser) ParseBytes(data []byte) (*models.Policy, error) {
	var policy models.Policy

	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	if p.strict {
		decoder.KnownFields(true)
	}

	if err := decoder.Decode(&policy); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &policy, nil
}

// ParseMultiple parses multiple policies from a single YAML file (multi-document)
func (p *Parser) ParseMultiple(r io.Reader) ([]*models.Policy, error) {
	var policies []*models.Policy

	decoder := yaml.NewDecoder(r)
	if p.strict {
		decoder.KnownFields(true)
	}

	for {
		var policy models.Policy
		err := decoder.Decode(&policy)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse YAML document: %w", err)
		}
		policies = append(policies, &policy)
	}

	return policies, nil
}

// ParseDirectory parses all YAML files in a directory
func (p *Parser) ParseDirectory(dir string) ([]*models.Policy, error) {
	var policies []*models.Policy

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-YAML files
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Parse the file
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", path, err)
		}
		defer file.Close()

		// Try multi-document parsing
		parsed, err := p.ParseMultiple(file)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}

		policies = append(policies, parsed...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return policies, nil
}
