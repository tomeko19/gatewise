// Package connector provides the Kong connector implementation
package connector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gatewise/gatewise/pkg/models"
)

// KongConnector implements the Connector interface for Kong Gateway
type KongConnector struct {
	adminURL   string
	apiKey     string
	httpClient *http.Client
	mode       ConnectorMode
	connected  bool
}

// NewKongConnector creates a new Kong connector
func NewKongConnector(adminURL string, apiKey string, mode ConnectorMode) (*KongConnector, error) {
	return &KongConnector{
		adminURL: strings.TrimSuffix(adminURL, "/"),
		apiKey:   apiKey,
		mode:     mode,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Type returns the connector type
func (k *KongConnector) Type() ConnectorType {
	return TypeKong
}

// Mode returns the connector mode
func (k *KongConnector) Mode() ConnectorMode {
	return k.mode
}

// Initialize sets up the Kong connection
func (k *KongConnector) Initialize(ctx context.Context) error {
	// Verify connectivity
	if err := k.HealthCheck(ctx); err != nil {
		return err
	}
	k.connected = true
	return nil
}

// HealthCheck verifies connectivity to Kong Admin API
func (k *KongConnector) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", k.adminURL+"/status", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if k.apiKey != "" {
		req.Header.Set("Kong-Admin-Token", k.apiKey)
	}

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Kong: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Kong health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// Reconcile applies the Kong resources defined in the policy
func (k *KongConnector) Reconcile(ctx context.Context, policy *models.Policy, target *models.Target) (*ReconcileResult, error) {
	if target.Kong == nil {
		return nil, fmt.Errorf("kong configuration missing in target")
	}

	result := &ReconcileResult{Success: true}

	// Step 1: Create/Update Services
	for _, serviceName := range target.Kong.Services {
		err := k.reconcileService(ctx, serviceName, policy)
		if err != nil {
			result.Errors = append(result.Errors, err)
			result.Success = false
		} else {
			result.ResourcesCreated = append(result.ResourcesCreated, fmt.Sprintf("Service/%s", serviceName))
		}
	}

	// Step 2: Create/Update Routes
	for i, routePath := range target.Kong.Routes {
		serviceName := ""
		if len(target.Kong.Services) > 0 {
			serviceName = target.Kong.Services[i%len(target.Kong.Services)]
		}
		err := k.reconcileRoute(ctx, routePath, serviceName, policy)
		if err != nil {
			result.Errors = append(result.Errors, err)
			result.Success = false
		} else {
			result.ResourcesCreated = append(result.ResourcesCreated, fmt.Sprintf("Route/%s", routePath))
		}
	}

	// Step 3: Apply Plugins
	for _, plugin := range target.Kong.Plugins {
		for _, serviceName := range target.Kong.Services {
			err := k.reconcilePlugin(ctx, serviceName, &plugin, policy)
			if err != nil {
				result.Errors = append(result.Errors, err)
				result.Success = false
			} else {
				result.ResourcesCreated = append(result.ResourcesCreated, 
					fmt.Sprintf("Plugin/%s@%s", plugin.Name, serviceName))
			}
		}
	}

	if result.Success {
		result.Message = fmt.Sprintf("Successfully reconciled %d Kong resources", len(result.ResourcesCreated))
	} else {
		result.Message = fmt.Sprintf("Reconciliation completed with %d errors", len(result.Errors))
	}

	return result, nil
}

// KongService represents a Kong service
type KongService struct {
	ID       string            `json:"id,omitempty"`
	Name     string            `json:"name"`
	URL      string            `json:"url,omitempty"`
	Host     string            `json:"host,omitempty"`
	Port     int               `json:"port,omitempty"`
	Protocol string            `json:"protocol,omitempty"`
	Path     string            `json:"path,omitempty"`
	Tags     []string          `json:"tags,omitempty"`
}

// KongRoute represents a Kong route
type KongRoute struct {
	ID        string   `json:"id,omitempty"`
	Name      string   `json:"name"`
	Paths     []string `json:"paths,omitempty"`
	Methods   []string `json:"methods,omitempty"`
	Service   *KongServiceRef `json:"service,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

// KongServiceRef is a reference to a Kong service
type KongServiceRef struct {
	ID string `json:"id,omitempty"`
}

// KongPluginConfig represents a Kong plugin configuration
type KongPluginConfig struct {
	Name    string                 `json:"name"`
	Config  map[string]interface{} `json:"config,omitempty"`
	Service *KongServiceRef        `json:"service,omitempty"`
	Enabled bool                   `json:"enabled"`
	Tags    []string               `json:"tags,omitempty"`
}

// reconcileService creates or updates a Kong service
func (k *KongConnector) reconcileService(ctx context.Context, serviceName string, policy *models.Policy) error {
	service := KongService{
		Name:     serviceName,
		Host:     fmt.Sprintf("%s.svc.cluster.local", serviceName),
		Port:     80,
		Protocol: "http",
		Tags: []string{
			"gatewise-managed",
			fmt.Sprintf("policy:%s", policy.Metadata.Name),
			fmt.Sprintf("team:%s", policy.Spec.Team),
		},
	}

	// Try to get existing service
	existing, err := k.getService(ctx, serviceName)
	if err == nil && existing != nil {
		// Update existing service
		return k.updateService(ctx, serviceName, &service)
	}

	// Create new service
	return k.createService(ctx, &service)
}

// reconcileRoute creates or updates a Kong route
func (k *KongConnector) reconcileRoute(ctx context.Context, routePath, serviceName string, policy *models.Policy) error {
	// Generate route name from path
	routeName := fmt.Sprintf("%s-%s", policy.Metadata.Name, strings.ReplaceAll(
		strings.Trim(routePath, "/"), "/", "-"))
	routeName = strings.ReplaceAll(routeName, "*", "wildcard")

	route := KongRoute{
		Name:    routeName,
		Paths:   []string{routePath},
		Methods: []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		Tags: []string{
			"gatewise-managed",
			fmt.Sprintf("policy:%s", policy.Metadata.Name),
		},
	}

	// Get service ID if specified
	if serviceName != "" {
		service, err := k.getService(ctx, serviceName)
		if err == nil && service != nil {
			route.Service = &KongServiceRef{ID: service.ID}
		}
	}

	// Try to create or update route
	return k.createOrUpdateRoute(ctx, &route)
}

// reconcilePlugin creates or updates a Kong plugin
func (k *KongConnector) reconcilePlugin(ctx context.Context, serviceName string, plugin *models.KongPlugin, policy *models.Policy) error {
	// Get service ID
	service, err := k.getService(ctx, serviceName)
	if err != nil {
		return fmt.Errorf("failed to get service for plugin: %w", err)
	}

	pluginConfig := KongPluginConfig{
		Name:    plugin.Name,
		Config:  plugin.Config,
		Enabled: true,
		Tags: []string{
			"gatewise-managed",
			fmt.Sprintf("policy:%s", policy.Metadata.Name),
		},
	}

	if service != nil {
		pluginConfig.Service = &KongServiceRef{ID: service.ID}
	}

	return k.createOrUpdatePlugin(ctx, &pluginConfig)
}

// HTTP helper methods

func (k *KongConnector) getService(ctx context.Context, name string) (*KongService, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/services/%s", k.adminURL, name), nil)
	if err != nil {
		return nil, err
	}
	k.setHeaders(req)

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get service: %s", string(body))
	}

	var service KongService
	if err := json.NewDecoder(resp.Body).Decode(&service); err != nil {
		return nil, err
	}

	return &service, nil
}

func (k *KongConnector) createService(ctx context.Context, service *KongService) error {
	body, err := json.Marshal(service)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", k.adminURL+"/services", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	k.setHeaders(req)

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create service: %s", string(respBody))
	}

	return nil
}

func (k *KongConnector) updateService(ctx context.Context, name string, service *KongService) error {
	body, err := json.Marshal(service)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", fmt.Sprintf("%s/services/%s", k.adminURL, name), bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	k.setHeaders(req)

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update service: %s", string(respBody))
	}

	return nil
}

func (k *KongConnector) createOrUpdateRoute(ctx context.Context, route *KongRoute) error {
	body, err := json.Marshal(route)
	if err != nil {
		return err
	}

	// Use PUT with name to create or update
	req, err := http.NewRequestWithContext(ctx, "PUT", fmt.Sprintf("%s/routes/%s", k.adminURL, route.Name), bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	k.setHeaders(req)

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create/update route: %s", string(respBody))
	}

	return nil
}

func (k *KongConnector) createOrUpdatePlugin(ctx context.Context, plugin *KongPluginConfig) error {
	body, err := json.Marshal(plugin)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", k.adminURL+"/plugins", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	k.setHeaders(req)

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 409 Conflict means plugin already exists
	if resp.StatusCode == http.StatusConflict {
		return nil // Already exists, that's fine
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create plugin: %s", string(respBody))
	}

	return nil
}

func (k *KongConnector) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if k.apiKey != "" {
		req.Header.Set("Kong-Admin-Token", k.apiKey)
	}
}

// Delete removes Kong resources created by a policy
func (k *KongConnector) Delete(ctx context.Context, policy *models.Policy, target *models.Target) error {
	var errs []error

	// Delete routes first (they depend on services)
	for _, routePath := range target.Kong.Routes {
		routeName := fmt.Sprintf("%s-%s", policy.Metadata.Name, strings.ReplaceAll(
			strings.Trim(routePath, "/"), "/", "-"))
		routeName = strings.ReplaceAll(routeName, "*", "wildcard")

		req, err := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("%s/routes/%s", k.adminURL, routeName), nil)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		k.setHeaders(req)

		resp, err := k.httpClient.Do(req)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		resp.Body.Close()
	}

	// Note: We don't delete services by default as they might be shared

	if len(errs) > 0 {
		return fmt.Errorf("deletion completed with errors: %v", errs)
	}
	return nil
}

// Close closes the Kong connector
func (k *KongConnector) Close() error {
	return nil
}

// Discover implements CapabilityDiscovery
func (k *KongConnector) Discover(ctx context.Context) (bool, error) {
	if err := k.HealthCheck(ctx); err != nil {
		return false, nil
	}
	return true, nil
}

// GetCapabilities returns available Kong features
func (k *KongConnector) GetCapabilities(ctx context.Context) ([]string, error) {
	return []string{
		"services",
		"routes",
		"plugins",
		"consumers",
	}, nil
}
