// Package api provides the REST API for Gatewise Control Plane
package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gatewise/gatewise/internal/connector"
	"github.com/gatewise/gatewise/internal/metrics"
	"github.com/gatewise/gatewise/internal/policy"
	"github.com/gatewise/gatewise/internal/reconciler"
	"github.com/gatewise/gatewise/internal/store"
	"github.com/gatewise/gatewise/pkg/license"
	"github.com/gatewise/gatewise/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server represents the API server
type Server struct {
	router     *gin.Engine
	store      *store.PostgresStore
	factory    *connector.Factory
	reconciler *reconciler.Reconciler
	licenseMgr *license.Manager
	parser     *policy.Parser
	validator  *policy.Validator
}

// NewServer creates a new API server
func NewServer(pgStore *store.PostgresStore, factory *connector.Factory, rec *reconciler.Reconciler, licenseMgr *license.Manager) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(CORSMiddleware())

	s := &Server{
		router:     router,
		store:      pgStore,
		factory:    factory,
		reconciler: rec,
		licenseMgr: licenseMgr,
		parser:     policy.NewParser(false),
		validator:  policy.NewValidator(),
	}

	s.setupRoutes()
	return s
}

// CORSMiddleware handles CORS
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Prometheus metrics endpoint (outside /api prefix for standard convention)
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	
	// Initialize metrics
	metrics.InitMetrics()
	
	api := s.router.Group("/api")
	{
		// Health & Status
		api.GET("/health", s.healthCheck)
		api.GET("/status", s.getStatus)

		// Policies
		policies := api.Group("/policies")
		{
			policies.GET("", s.listPolicies)
			policies.GET("/:name", s.getPolicy)
			policies.POST("", s.createPolicy)
			policies.PUT("/:name", s.updatePolicy)
			policies.DELETE("/:name", s.deletePolicy)
			policies.POST("/:name/reconcile", s.reconcilePolicy)
			policies.POST("/validate", s.validatePolicy)
		}

		// Audit Logs
		api.GET("/audit", s.listAuditLogs)

		// Connectors
		connectors := api.Group("/connectors")
		{
			connectors.GET("", s.listConnectors)
			connectors.GET("/:type/health", s.connectorHealth)
		}

		// License
		api.GET("/license", s.getLicense)
		api.POST("/license", s.setLicense)
	}
}

// Run starts the API server
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}

// GetRouter returns the gin router for testing
func (s *Server) GetRouter() *gin.Engine {
	return s.router
}

// ========== Health & Status ==========

func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// StatusResponse represents the status response
type StatusResponse struct {
	Version       string                    `json:"version"`
	License       license.Tier              `json:"license"`
	Features      []license.Feature         `json:"features"`
	Connectors    []ConnectorStatus         `json:"connectors"`
	PolicyCount   int                       `json:"policyCount"`
	DatabaseOK    bool                      `json:"databaseOk"`
}

// ConnectorStatus represents connector status
type ConnectorStatus struct {
	Type    string `json:"type"`
	Mode    string `json:"mode"`
	Status  string `json:"status"`
	Enabled bool   `json:"enabled"`
}

func (s *Server) getStatus(c *gin.Context) {
	status := StatusResponse{
		Version:    "0.2.0-alpha",
		License:    s.licenseMgr.GetTier(),
		Features:   s.licenseMgr.ListEnabledFeatures(),
		Connectors: []ConnectorStatus{},
		DatabaseOK: s.store != nil,
	}

	// Add connector statuses
	connectorTypes := []connector.ConnectorType{
		connector.TypeKubernetes,
		connector.TypeKafka,
		connector.TypeKong,
	}

	for _, ct := range connectorTypes {
		cs := ConnectorStatus{
			Type:    string(ct),
			Status:  "not_configured",
			Enabled: false,
		}

		if conn, exists := s.factory.GetConnector(ct); exists {
			cs.Mode = string(conn.Mode())
			cs.Enabled = true
			if err := conn.HealthCheck(c.Request.Context()); err == nil {
				cs.Status = "healthy"
			} else {
				cs.Status = "unhealthy"
			}
		}

		status.Connectors = append(status.Connectors, cs)
	}

	// Get policy count
	if s.store != nil {
		policies, err := s.store.ListPolicies(c.Request.Context(), "", "")
		if err == nil {
			status.PolicyCount = len(policies)
		}
	}

	c.JSON(http.StatusOK, status)
}

// ========== Policies ==========

// PolicyListResponse represents the policy list response
type PolicyListResponse struct {
	Policies []*PolicyItem `json:"policies"`
	Total    int           `json:"total"`
}

// PolicyItem represents a policy in the list
type PolicyItem struct {
	Name      string          `json:"name"`
	Kind      string          `json:"kind"`
	Namespace string          `json:"namespace"`
	Team      string          `json:"team"`
	Status    string          `json:"status"`
	Targets   int             `json:"targets"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

func (s *Server) listPolicies(c *gin.Context) {
	if s.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not available"})
		return
	}

	kind := c.Query("kind")
	namespace := c.Query("namespace")

	policies, err := s.store.ListPolicies(c.Request.Context(), kind, namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]*PolicyItem, 0, len(policies))
	for _, p := range policies {
		items = append(items, &PolicyItem{
			Name:      p.Metadata.Name,
			Kind:      string(p.Kind),
			Namespace: p.Metadata.Namespace,
			Team:      p.Spec.Team,
			Status:    "Synced", // TODO: Get from status field
			Targets:   len(p.Spec.Targets),
			CreatedAt: p.Metadata.CreatedAt,
			UpdatedAt: p.Metadata.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, PolicyListResponse{
		Policies: items,
		Total:    len(items),
	})
}

func (s *Server) getPolicy(c *gin.Context) {
	if s.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not available"})
		return
	}

	name := c.Param("name")
	p, err := s.store.GetPolicy(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if p == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "policy not found"})
		return
	}

	c.JSON(http.StatusOK, p)
}

// CreatePolicyRequest represents the request to create a policy
type CreatePolicyRequest struct {
	Policy *models.Policy `json:"policy"`
	YAML   string         `json:"yaml,omitempty"`
}

func (s *Server) createPolicy(c *gin.Context) {
	if s.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not available"})
		return
	}

	var req CreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var p *models.Policy
	var err error

	// Parse from YAML if provided
	if req.YAML != "" {
		p, err = s.parser.ParseBytes([]byte(req.YAML))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid YAML: %s", err)})
			return
		}
	} else if req.Policy != nil {
		p = req.Policy
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "policy or yaml required"})
		return
	}

	// Validate
	result := s.validator.Validate(p)
	if !result.IsValid() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "validation failed",
			"errors": result.Errors,
		})
		return
	}

	// Store
	id, err := s.store.CreatePolicy(c.Request.Context(), p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":      id,
		"name":    p.Metadata.Name,
		"message": "policy created successfully",
	})
}

func (s *Server) updatePolicy(c *gin.Context) {
	if s.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not available"})
		return
	}

	name := c.Param("name")

	// Check if exists
	existing, err := s.store.GetPolicy(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "policy not found"})
		return
	}

	var req CreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var p *models.Policy
	if req.YAML != "" {
		p, err = s.parser.ParseBytes([]byte(req.YAML))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid YAML: %s", err)})
			return
		}
	} else if req.Policy != nil {
		p = req.Policy
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "policy or yaml required"})
		return
	}

	// Validate
	result := s.validator.Validate(p)
	if !result.IsValid() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "validation failed",
			"errors": result.Errors,
		})
		return
	}

	// Delete old and create new (simple approach)
	s.store.DeletePolicy(c.Request.Context(), name)
	p.Metadata.Name = name // Ensure name stays the same
	_, err = s.store.CreatePolicy(c.Request.Context(), p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "policy updated successfully"})
}

func (s *Server) deletePolicy(c *gin.Context) {
	if s.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not available"})
		return
	}

	name := c.Param("name")

	err := s.store.DeletePolicy(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "policy deleted successfully"})
}

func (s *Server) reconcilePolicy(c *gin.Context) {
	if s.store == nil || s.reconciler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service not available"})
		return
	}

	name := c.Param("name")

	p, err := s.store.GetPolicy(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if p == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "policy not found"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()

	err = s.reconciler.ReconcilePolicy(ctx, p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   err.Error(),
			"status":  "failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "policy reconciled successfully",
		"status":  "applied",
	})
}

// ValidatePolicyRequest represents the validation request
type ValidatePolicyRequest struct {
	YAML string `json:"yaml"`
}

// ValidationResponse represents the validation response
type ValidationResponse struct {
	Valid    bool                     `json:"valid"`
	Errors   []policy.ValidationError `json:"errors,omitempty"`
	Warnings []string                 `json:"warnings,omitempty"`
	Policy   *models.Policy           `json:"policy,omitempty"`
}

func (s *Server) validatePolicy(c *gin.Context) {
	var req ValidatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	p, err := s.parser.ParseBytes([]byte(req.YAML))
	if err != nil {
		c.JSON(http.StatusOK, ValidationResponse{
			Valid:  false,
			Errors: []policy.ValidationError{{Field: "yaml", Message: err.Error()}},
		})
		return
	}

	result := s.validator.Validate(p)

	c.JSON(http.StatusOK, ValidationResponse{
		Valid:    result.IsValid(),
		Errors:   result.Errors,
		Warnings: result.Warnings,
		Policy:   p,
	})
}

// ========== Audit Logs ==========

func (s *Server) listAuditLogs(c *gin.Context) {
	// TODO: Implement audit log listing from PostgreSQL
	c.JSON(http.StatusOK, gin.H{
		"logs":  []interface{}{},
		"total": 0,
	})
}

// ========== Connectors ==========

func (s *Server) listConnectors(c *gin.Context) {
	connectors := []ConnectorStatus{}

	connectorTypes := []connector.ConnectorType{
		connector.TypeKubernetes,
		connector.TypeKafka,
		connector.TypeKong,
	}

	for _, ct := range connectorTypes {
		cs := ConnectorStatus{
			Type:    string(ct),
			Status:  "not_configured",
			Enabled: false,
		}

		if conn, exists := s.factory.GetConnector(ct); exists {
			cs.Mode = string(conn.Mode())
			cs.Enabled = true
			if err := conn.HealthCheck(c.Request.Context()); err == nil {
				cs.Status = "healthy"
			} else {
				cs.Status = "unhealthy"
			}
		}

		connectors = append(connectors, cs)
	}

	c.JSON(http.StatusOK, gin.H{"connectors": connectors})
}

func (s *Server) connectorHealth(c *gin.Context) {
	connType := connector.ConnectorType(c.Param("type"))

	conn, exists := s.factory.GetConnector(connType)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "connector not configured"})
		return
	}

	if err := conn.HealthCheck(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

// ========== License ==========

func (s *Server) getLicense(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"tier":     s.licenseMgr.GetTier(),
		"features": s.licenseMgr.ListEnabledFeatures(),
	})
}

// SetLicenseRequest represents the license update request
type SetLicenseRequest struct {
	Key string `json:"key"`
}

func (s *Server) setLicense(c *gin.Context) {
	var req SetLicenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.licenseMgr.SetLicenseKey(req.Key); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "license updated",
		"tier":     s.licenseMgr.GetTier(),
		"features": s.licenseMgr.ListEnabledFeatures(),
	})
}
