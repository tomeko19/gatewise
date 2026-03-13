package operator

import (
	"context"
	"fmt"
	"time"

	"github.com/gatewise/gatewise/internal/reconciler"
	"github.com/gatewise/gatewise/internal/store"
	"github.com/gatewise/gatewise/pkg/license"
	"github.com/gatewise/gatewise/pkg/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Operator watches GatewisePolicy CRDs and reconciles them
type Operator struct {
	client      dynamic.Interface
	reconciler  *reconciler.Reconciler
	store       *store.PostgresStore
	licenseMgr  *license.Manager
	stopCh      chan struct{}
	namespace   string
	gvr         schema.GroupVersionResource
}

// NewOperator creates a new Gatewise Operator
func NewOperator(kubeconfig string, namespace string, rec *reconciler.Reconciler, store *store.PostgresStore, licenseMgr *license.Manager) (*Operator, error) {
	var config *rest.Config
	var err error

	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes config: %w", err)
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// GatewisePolicy GVR
	gvr := schema.GroupVersionResource{
		Group:    "gatewise.io",
		Version:  "v1",
		Resource: "gatewisepolicies",
	}

	return &Operator{
		client:     client,
		reconciler: rec,
		store:      store,
		licenseMgr: licenseMgr,
		stopCh:     make(chan struct{}),
		namespace:  namespace,
		gvr:        gvr,
	}, nil
}

// Run starts the operator
func (o *Operator) Run(ctx context.Context) error {
	fmt.Println("🚀 Starting Gatewise Operator...")
	fmt.Printf("   Watching GatewisePolicies in namespace: %s\n", o.namespace)

	// Initial sync - list all existing policies
	if err := o.syncExistingPolicies(ctx); err != nil {
		return fmt.Errorf("failed to sync existing policies: %w", err)
	}

	// Start watching for changes
	watcher, err := o.client.Resource(o.gvr).Namespace(o.namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to start watch: %w", err)
	}
	defer watcher.Stop()

	fmt.Println("✅ Operator started, watching for CRD changes...")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-o.stopCh:
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				// Channel closed, restart watch
				fmt.Println("⚠️  Watch channel closed, restarting...")
				time.Sleep(5 * time.Second)
				return o.Run(ctx)
			}

			if err := o.handleEvent(ctx, event); err != nil {
				fmt.Printf("❌ Error handling event: %v\n", err)
			}
		}
	}
}

// syncExistingPolicies syncs all existing CRDs on startup
func (o *Operator) syncExistingPolicies(ctx context.Context) error {
	list, err := o.client.Resource(o.gvr).Namespace(o.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	fmt.Printf("📋 Found %d existing GatewisePolicy resources\n", len(list.Items))

	for _, item := range list.Items {
		if err := o.reconcilePolicy(ctx, &item); err != nil {
			fmt.Printf("⚠️  Failed to sync policy %s: %v\n", item.GetName(), err)
		}
	}

	return nil
}

// handleEvent processes watch events
func (o *Operator) handleEvent(ctx context.Context, event watch.Event) error {
	obj, ok := event.Object.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("unexpected object type: %T", event.Object)
	}

	name := obj.GetName()

	switch event.Type {
	case watch.Added:
		fmt.Printf("➕ Policy ADDED: %s\n", name)
		return o.reconcilePolicy(ctx, obj)

	case watch.Modified:
		fmt.Printf("✏️  Policy MODIFIED: %s\n", name)
		return o.reconcilePolicy(ctx, obj)

	case watch.Deleted:
		fmt.Printf("🗑️  Policy DELETED: %s\n", name)
		return o.deletePolicy(ctx, name)

	default:
		return nil
	}
}

// reconcilePolicy reconciles a single GatewisePolicy CRD
func (o *Operator) reconcilePolicy(ctx context.Context, obj *unstructured.Unstructured) error {
	name := obj.GetName()

	// Update status to Reconciling
	if err := o.updateStatus(ctx, obj, "Reconciling", "Reconciling policy resources"); err != nil {
		fmt.Printf("⚠️  Failed to update status for %s: %v\n", name, err)
	}

	// Convert CRD spec to models.Policy
	policy, err := o.convertCRDToPolicy(obj)
	if err != nil {
		o.updateStatus(ctx, obj, "Failed", fmt.Sprintf("Invalid policy spec: %v", err))
		return err
	}

	// Store policy in database using CreatePolicy
	policyID, err := o.store.CreatePolicy(ctx, policy)
	if err != nil {
		// Policy might already exist, try to get and update
		fmt.Printf("⚠️  Policy %s may already exist, continuing reconciliation\n", name)
	} else {
		fmt.Printf("✅ Policy %s stored in database (ID: %d)\n", name, policyID)
	}

	// Reconcile using the reconciler
	if err := o.reconciler.ReconcilePolicy(ctx, policy); err != nil {
		o.updateStatus(ctx, obj, "Failed", fmt.Sprintf("Reconciliation failed: %v", err))
		return err
	}

	// Update status to Active
	if err := o.updateStatus(ctx, obj, "Active", "Policy successfully reconciled"); err != nil {
		return err
	}

	fmt.Printf("✅ Policy %s reconciled successfully\n", name)
	return nil
}

// deletePolicy handles policy deletion
func (o *Operator) deletePolicy(ctx context.Context, name string) error {
	// Delete from store
	if err := o.store.DeletePolicy(ctx, name); err != nil {
		return fmt.Errorf("failed to delete policy from store: %w", err)
	}

	fmt.Printf("✅ Policy %s deleted successfully\n", name)
	return nil
}

// convertCRDToPolicy converts a GatewisePolicy CRD to models.Policy
func (o *Operator) convertCRDToPolicy(obj *unstructured.Unstructured) (*models.Policy, error) {
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil || !found {
		return nil, fmt.Errorf("spec not found in CRD")
	}

	policy := &models.Policy{
		APIVersion: "gatewise.io/v1",
		Kind:       models.KindAccessPolicy,
		Metadata: models.PolicyMetadata{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
			Labels:    obj.GetLabels(),
		},
		Spec: models.PolicySpec{},
	}

	// Extract team
	if team, found, _ := unstructured.NestedString(spec, "team"); found {
		policy.Spec.Team = team
	}

	// Extract targets from Kubernetes/Kafka/Kong sections
	var targets []models.Target

	// Kubernetes target
	if k8s, found, _ := unstructured.NestedMap(spec, "kubernetes"); found {
		target := models.Target{
			Type: models.TargetKubernetes,
			Name: "kubernetes-access",
			K8s:  o.extractK8sConfig(k8s),
		}
		targets = append(targets, target)
	}

	// Kafka target
	if kafka, found, _ := unstructured.NestedMap(spec, "kafka"); found {
		target := models.Target{
			Type:  models.TargetKafka,
			Name:  "kafka-access",
			Kafka: o.extractKafkaConfigSimple(kafka),
		}
		targets = append(targets, target)
	}

	// Kong target
	if kong, found, _ := unstructured.NestedMap(spec, "kong"); found {
		target := models.Target{
			Type: models.TargetKong,
			Name: "kong-access",
			Kong: o.extractKongConfigSimple(kong),
		}
		targets = append(targets, target)
	}

	policy.Spec.Targets = targets
	return policy, nil
}

// Helper functions to extract config
func (o *Operator) extractK8sConfig(k8s map[string]interface{}) *models.K8sConfig {
	config := &models.K8sConfig{
		Resources: []string{"*"},
		Verbs:     []string{"get", "list", "create", "update", "delete"},
	}

	// Extract namespace from first namespace entry
	if namespaces, found, _ := unstructured.NestedSlice(k8s, "namespaces"); found && len(namespaces) > 0 {
		if nsMap, ok := namespaces[0].(map[string]interface{}); ok {
			if name, found, _ := unstructured.NestedString(nsMap, "name"); found {
				config.Namespace = name
			}
		}
	}

	return config
}

func (o *Operator) extractKafkaConfigSimple(kafka map[string]interface{}) *models.KafkaConfig {
	config := &models.KafkaConfig{
		Operations: []string{"Read", "Write"},
	}

	// Extract topic names
	if topics, found, _ := unstructured.NestedSlice(kafka, "topics"); found {
		for _, t := range topics {
			if topicMap, ok := t.(map[string]interface{}); ok {
				if name, found, _ := unstructured.NestedString(topicMap, "name"); found {
					config.Topics = append(config.Topics, name)
				}
			}
		}
	}

	return config
}

func (o *Operator) extractKongConfigSimple(kong map[string]interface{}) *models.KongConfig {
	config := &models.KongConfig{}

	// Extract route names
	if routes, found, _ := unstructured.NestedSlice(kong, "routes"); found {
		for _, r := range routes {
			if routeMap, ok := r.(map[string]interface{}); ok {
				if name, found, _ := unstructured.NestedString(routeMap, "name"); found {
					config.Routes = append(config.Routes, name)
				}
			}
		}
	}

	// Extract service names
	if services, found, _ := unstructured.NestedSlice(kong, "services"); found {
		for _, s := range services {
			if svcMap, ok := s.(map[string]interface{}); ok {
				if name, found, _ := unstructured.NestedString(svcMap, "name"); found {
					config.Services = append(config.Services, name)
				}
			}
		}
	}

	return config
}

// updateStatus updates the status of a GatewisePolicy CRD
func (o *Operator) updateStatus(ctx context.Context, obj *unstructured.Unstructured, phase, message string) error {
	// Update status fields
	status := map[string]interface{}{
		"phase":              phase,
		"lastReconcileTime":  time.Now().Format(time.RFC3339),
		"observedGeneration": obj.GetGeneration(),
		"conditions": []map[string]interface{}{
			{
				"type":               "Ready",
				"status":             "True",
				"lastTransitionTime": time.Now().Format(time.RFC3339),
				"reason":             phase,
				"message":            message,
			},
		},
	}

	if err := unstructured.SetNestedMap(obj.Object, status, "status"); err != nil {
		return err
	}

	// Update the status subresource
	_, err := o.client.Resource(o.gvr).Namespace(o.namespace).UpdateStatus(ctx, obj, metav1.UpdateOptions{})
	return err
}

// Stop stops the operator
func (o *Operator) Stop() {
	close(o.stopCh)
}
