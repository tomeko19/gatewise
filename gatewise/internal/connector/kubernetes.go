// Package connector provides the Kubernetes connector implementation
package connector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gatewise/gatewise/pkg/models"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sConnector implements the Connector interface for Kubernetes
type K8sConnector struct {
	clientset  *kubernetes.Clientset
	config     *rest.Config
	mode       ConnectorMode
	kubeconfig string
}

// NewK8sConnector creates a new Kubernetes connector
func NewK8sConnector(kubeconfig string, mode ConnectorMode) (*K8sConnector, error) {
	return &K8sConnector{
		kubeconfig: kubeconfig,
		mode:       mode,
	}, nil
}

// Type returns the connector type
func (k *K8sConnector) Type() ConnectorType {
	return TypeKubernetes
}

// Mode returns the connector mode
func (k *K8sConnector) Mode() ConnectorMode {
	return k.mode
}

// Initialize sets up the Kubernetes client
func (k *K8sConnector) Initialize(ctx context.Context) error {
	var config *rest.Config
	var err error

	switch k.mode {
	case ModeEmbedded:
		// In-cluster configuration
		config, err = rest.InClusterConfig()
		if err != nil {
			// Fallback to default kubeconfig for local development
			home, _ := os.UserHomeDir()
			kubeconfig := filepath.Join(home, ".kube", "config")
			config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
			if err != nil {
				return fmt.Errorf("failed to build config: %w", err)
			}
		}
	case ModeExternal:
		// External kubeconfig
		if k.kubeconfig == "" {
			return fmt.Errorf("kubeconfig path required for external mode")
		}
		config, err = clientcmd.BuildConfigFromFlags("", k.kubeconfig)
		if err != nil {
			return fmt.Errorf("failed to build config from %s: %w", k.kubeconfig, err)
		}
	default:
		return fmt.Errorf("unknown mode: %s", k.mode)
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	k.config = config
	k.clientset = clientset
	return nil
}

// HealthCheck verifies connectivity to the cluster
func (k *K8sConnector) HealthCheck(ctx context.Context) error {
	if k.clientset == nil {
		return fmt.Errorf("connector not initialized")
	}

	_, err := k.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("failed to connect to cluster: %w", err)
	}
	return nil
}

// Reconcile applies the Kubernetes resources defined in the policy
func (k *K8sConnector) Reconcile(ctx context.Context, policy *models.Policy, target *models.Target) (*ReconcileResult, error) {
	if k.clientset == nil {
		return nil, fmt.Errorf("connector not initialized")
	}

	if target.K8s == nil {
		return nil, fmt.Errorf("kubernetes configuration missing in target")
	}

	result := &ReconcileResult{Success: true}

	// Step 1: Create/Update Namespace
	ns, err := k.reconcileNamespace(ctx, policy, target)
	if err != nil {
		result.Errors = append(result.Errors, err)
		result.Success = false
	} else if ns != "" {
		result.ResourcesCreated = append(result.ResourcesCreated, fmt.Sprintf("Namespace/%s", ns))
	}

	// Step 2: Create/Update ResourceQuota if specified
	if policy.Spec.Quotas != nil {
		quota, err := k.reconcileResourceQuota(ctx, policy, target)
		if err != nil {
			result.Errors = append(result.Errors, err)
			result.Success = false
		} else if quota != "" {
			result.ResourcesCreated = append(result.ResourcesCreated, fmt.Sprintf("ResourceQuota/%s", quota))
		}
	}

	// Step 3: Create/Update RBAC (Role + RoleBinding)
	role, binding, err := k.reconcileRBAC(ctx, policy, target)
	if err != nil {
		result.Errors = append(result.Errors, err)
		result.Success = false
	} else {
		if role != "" {
			result.ResourcesCreated = append(result.ResourcesCreated, fmt.Sprintf("Role/%s", role))
		}
		if binding != "" {
			result.ResourcesCreated = append(result.ResourcesCreated, fmt.Sprintf("RoleBinding/%s", binding))
		}
	}

	if result.Success {
		result.Message = fmt.Sprintf("Successfully reconciled %d resources", len(result.ResourcesCreated))
	} else {
		result.Message = fmt.Sprintf("Reconciliation completed with %d errors", len(result.Errors))
	}

	return result, nil
}

// reconcileNamespace creates or updates the namespace
func (k *K8sConnector) reconcileNamespace(ctx context.Context, policy *models.Policy, target *models.Target) (string, error) {
	nsName := target.K8s.Namespace

	// Build namespace object
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
			Labels: map[string]string{
				"gatewise.io/managed":    "true",
				"gatewise.io/policy":     policy.Metadata.Name,
				"gatewise.io/team":       policy.Spec.Team,
			},
			Annotations: map[string]string{
				"gatewise.io/description": fmt.Sprintf("Managed by Gatewise policy: %s", policy.Metadata.Name),
			},
		},
	}

	// Add policy labels to namespace
	for k, v := range policy.Metadata.Labels {
		ns.Labels[k] = v
	}

	// Try to create, update if exists
	_, err := k.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			// Update existing namespace
			existing, getErr := k.clientset.CoreV1().Namespaces().Get(ctx, nsName, metav1.GetOptions{})
			if getErr != nil {
				return "", fmt.Errorf("failed to get existing namespace: %w", getErr)
			}

			// Merge labels
			if existing.Labels == nil {
				existing.Labels = make(map[string]string)
			}
			for k, v := range ns.Labels {
				existing.Labels[k] = v
			}
			if existing.Annotations == nil {
				existing.Annotations = make(map[string]string)
			}
			for k, v := range ns.Annotations {
				existing.Annotations[k] = v
			}

			_, updateErr := k.clientset.CoreV1().Namespaces().Update(ctx, existing, metav1.UpdateOptions{})
			if updateErr != nil {
				return "", fmt.Errorf("failed to update namespace: %w", updateErr)
			}
			return nsName, nil
		}
		return "", fmt.Errorf("failed to create namespace: %w", err)
	}

	return nsName, nil
}

// reconcileResourceQuota creates or updates resource quotas
func (k *K8sConnector) reconcileResourceQuota(ctx context.Context, policy *models.Policy, target *models.Target) (string, error) {
	nsName := target.K8s.Namespace
	quotaName := fmt.Sprintf("%s-quota", policy.Metadata.Name)
	quotas := policy.Spec.Quotas

	// Build hard limits
	hard := corev1.ResourceList{}

	if quotas.CPU != "" {
		hard[corev1.ResourceLimitsCPU] = resource.MustParse(quotas.CPU)
		hard[corev1.ResourceRequestsCPU] = resource.MustParse(quotas.CPU)
	}
	if quotas.Memory != "" {
		hard[corev1.ResourceLimitsMemory] = resource.MustParse(quotas.Memory)
		hard[corev1.ResourceRequestsMemory] = resource.MustParse(quotas.Memory)
	}
	if quotas.Pods > 0 {
		hard[corev1.ResourcePods] = resource.MustParse(fmt.Sprintf("%d", quotas.Pods))
	}
	if quotas.Services > 0 {
		hard[corev1.ResourceServices] = resource.MustParse(fmt.Sprintf("%d", quotas.Services))
	}
	if quotas.Secrets > 0 {
		hard[corev1.ResourceSecrets] = resource.MustParse(fmt.Sprintf("%d", quotas.Secrets))
	}

	// Build ResourceQuota object
	rq := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      quotaName,
			Namespace: nsName,
			Labels: map[string]string{
				"gatewise.io/managed": "true",
				"gatewise.io/policy":  policy.Metadata.Name,
			},
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: hard,
		},
	}

	// Try to create, update if exists
	_, err := k.clientset.CoreV1().ResourceQuotas(nsName).Create(ctx, rq, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			_, updateErr := k.clientset.CoreV1().ResourceQuotas(nsName).Update(ctx, rq, metav1.UpdateOptions{})
			if updateErr != nil {
				return "", fmt.Errorf("failed to update resource quota: %w", updateErr)
			}
			return quotaName, nil
		}
		return "", fmt.Errorf("failed to create resource quota: %w", err)
	}

	return quotaName, nil
}

// reconcileRBAC creates or updates Role and RoleBinding
func (k *K8sConnector) reconcileRBAC(ctx context.Context, policy *models.Policy, target *models.Target) (string, string, error) {
	nsName := target.K8s.Namespace
	roleName := fmt.Sprintf("%s-role", policy.Metadata.Name)
	bindingName := fmt.Sprintf("%s-binding", policy.Metadata.Name)

	// Build Role
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: nsName,
			Labels: map[string]string{
				"gatewise.io/managed": "true",
				"gatewise.io/policy":  policy.Metadata.Name,
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: target.K8s.Resources,
				Verbs:     target.K8s.Verbs,
			},
		},
	}

	// Add apps API group if deployments are included
	for _, res := range target.K8s.Resources {
		if res == "deployments" || res == "replicasets" || res == "statefulsets" || res == "daemonsets" {
			role.Rules = append(role.Rules, rbacv1.PolicyRule{
				APIGroups: []string{"apps"},
				Resources: []string{res},
				Verbs:     target.K8s.Verbs,
			})
		}
	}

	// Create/Update Role
	_, err := k.clientset.RbacV1().Roles(nsName).Create(ctx, role, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			_, updateErr := k.clientset.RbacV1().Roles(nsName).Update(ctx, role, metav1.UpdateOptions{})
			if updateErr != nil {
				return "", "", fmt.Errorf("failed to update role: %w", updateErr)
			}
		} else {
			return "", "", fmt.Errorf("failed to create role: %w", err)
		}
	}

	// Build RoleBinding
	subjects := make([]rbacv1.Subject, 0, len(policy.Spec.Members))
	for _, member := range policy.Spec.Members {
		// Determine subject kind based on format
		kind := "User"
		name := member
		if strings.HasPrefix(member, "group:") {
			kind = "Group"
			name = strings.TrimPrefix(member, "group:")
		} else if strings.HasPrefix(member, "sa:") {
			kind = "ServiceAccount"
			name = strings.TrimPrefix(member, "sa:")
		}

		subjects = append(subjects, rbacv1.Subject{
			Kind: kind,
			Name: name,
		})
	}

	binding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: nsName,
			Labels: map[string]string{
				"gatewise.io/managed": "true",
				"gatewise.io/policy":  policy.Metadata.Name,
			},
		},
		Subjects: subjects,
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleName,
		},
	}

	// Create/Update RoleBinding
	_, err = k.clientset.RbacV1().RoleBindings(nsName).Create(ctx, binding, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			_, updateErr := k.clientset.RbacV1().RoleBindings(nsName).Update(ctx, binding, metav1.UpdateOptions{})
			if updateErr != nil {
				return roleName, "", fmt.Errorf("failed to update role binding: %w", updateErr)
			}
		} else {
			return roleName, "", fmt.Errorf("failed to create role binding: %w", err)
		}
	}

	return roleName, bindingName, nil
}

// Delete removes all resources created by a policy
func (k *K8sConnector) Delete(ctx context.Context, policy *models.Policy, target *models.Target) error {
	if k.clientset == nil {
		return fmt.Errorf("connector not initialized")
	}

	nsName := target.K8s.Namespace
	roleName := fmt.Sprintf("%s-role", policy.Metadata.Name)
	bindingName := fmt.Sprintf("%s-binding", policy.Metadata.Name)
	quotaName := fmt.Sprintf("%s-quota", policy.Metadata.Name)

	var errs []error

	// Delete RoleBinding
	err := k.clientset.RbacV1().RoleBindings(nsName).Delete(ctx, bindingName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("failed to delete role binding: %w", err))
	}

	// Delete Role
	err = k.clientset.RbacV1().Roles(nsName).Delete(ctx, roleName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("failed to delete role: %w", err))
	}

	// Delete ResourceQuota
	err = k.clientset.CoreV1().ResourceQuotas(nsName).Delete(ctx, quotaName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("failed to delete resource quota: %w", err))
	}

	// Note: We don't delete the namespace by default to prevent data loss
	// This can be controlled by a policy annotation

	if len(errs) > 0 {
		return fmt.Errorf("deletion completed with errors: %v", errs)
	}
	return nil
}

// Close cleans up the connector
func (k *K8sConnector) Close() error {
	// Nothing to clean up for K8s client
	return nil
}

// Discover implements CapabilityDiscovery
func (k *K8sConnector) Discover(ctx context.Context) (bool, error) {
	if k.clientset == nil {
		if err := k.Initialize(ctx); err != nil {
			return false, nil // Not available
		}
	}

	if err := k.HealthCheck(ctx); err != nil {
		return false, nil
	}
	return true, nil
}

// GetCapabilities returns available K8s features
func (k *K8sConnector) GetCapabilities(ctx context.Context) ([]string, error) {
	capabilities := []string{
		"namespace",
		"resourcequota",
		"role",
		"rolebinding",
	}

	// Check for CRDs
	// TODO: Add CRD discovery for advanced capabilities

	return capabilities, nil
}
