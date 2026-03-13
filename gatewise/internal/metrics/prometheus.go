package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Policy metrics
	PoliciesTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gatewise_policies_total",
		Help: "Total number of active policies",
	})

	PolicyReconciliations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gatewise_policy_reconciliations_total",
		Help: "Total number of policy reconciliations",
	}, []string{"policy", "status"})

	PolicyValidations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gatewise_policy_validations_total",
		Help: "Total number of policy validations",
	}, []string{"status"})

	// JIT Access metrics
	JITGrantsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gatewise_jit_grants_active",
		Help: "Number of active JIT grants",
	})

	JITRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gatewise_jit_requests_total",
		Help: "Total number of JIT access requests",
	}, []string{"status"})

	JITGrantDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "gatewise_jit_grant_duration_seconds",
		Help:    "Duration of JIT grants in seconds",
		Buckets: prometheus.ExponentialBuckets(3600, 2, 6), // 1h, 2h, 4h, 8h, 16h, 32h
	})

	// Drift detection metrics
	DriftsDetected = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gatewise_drifts_detected",
		Help: "Number of configuration drifts detected",
	})

	DriftRepairs = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gatewise_drift_repairs_total",
		Help: "Total number of drift repairs",
	}, []string{"status"})

	DriftDetectionDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "gatewise_drift_detection_duration_seconds",
		Help:    "Duration of drift detection cycles",
		Buckets: prometheus.DefBuckets,
	})

	// Connector metrics
	ConnectorStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gatewise_connector_status",
		Help: "Status of connectors (1=available, 0=unavailable)",
	}, []string{"connector", "mode"})

	ConnectorOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gatewise_connector_operations_total",
		Help: "Total number of connector operations",
	}, []string{"connector", "operation", "status"})

	// API metrics
	APIRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gatewise_api_requests_total",
		Help: "Total number of API requests",
	}, []string{"method", "endpoint", "status"})

	APIRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gatewise_api_request_duration_seconds",
		Help:    "Duration of API requests",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "endpoint"})

	// License metrics
	LicenseTier = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gatewise_license_tier",
		Help: "Current license tier (0=community, 1=enterprise)",
	}, []string{"tier"})

	LicenseFeatures = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gatewise_license_features",
		Help: "Licensed features (1=enabled, 0=disabled)",
	}, []string{"feature"})

	// System metrics
	ReconcilerRunning = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gatewise_reconciler_running",
		Help: "Whether the reconciler is running (1=yes, 0=no)",
	})

	DatabaseConnected = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gatewise_database_connected",
		Help: "Whether the database is connected (1=yes, 0=no)",
	})
)

// InitMetrics initializes all metrics with default values
func InitMetrics() {
	PoliciesTotal.Set(0)
	JITGrantsActive.Set(0)
	DriftsDetected.Set(0)
	ReconcilerRunning.Set(0)
	DatabaseConnected.Set(0)
	
	// Set default license to community
	LicenseTier.WithLabelValues("community").Set(1)
	LicenseTier.WithLabelValues("enterprise").Set(0)
}

// UpdateLicenseTier updates the license tier metric
func UpdateLicenseTier(tier string) {
	if tier == "enterprise" {
		LicenseTier.WithLabelValues("community").Set(0)
		LicenseTier.WithLabelValues("enterprise").Set(1)
	} else {
		LicenseTier.WithLabelValues("community").Set(1)
		LicenseTier.WithLabelValues("enterprise").Set(0)
	}
}
