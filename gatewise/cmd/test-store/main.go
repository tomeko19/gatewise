// Package main provides test for PostgreSQL store
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gatewise/gatewise/internal/store"
	"github.com/gatewise/gatewise/pkg/models"
)

func main() {
	// Connect to PostgreSQL
	connStr := "host=localhost port=5432 user=gatewise password=gatewise123 dbname=gatewise sslmode=disable"
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pgStore, err := store.NewPostgresStore(connStr)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pgStore.Close()

	fmt.Println("✓ Connected to PostgreSQL")

	// Create a test policy
	policy := &models.Policy{
		APIVersion: "gatewise.io/v1alpha1",
		Kind:       models.KindAccessPolicy,
		Metadata: models.PolicyMetadata{
			Name:      "test-policy-" + fmt.Sprintf("%d", time.Now().Unix()),
			Namespace: "test-namespace",
			Labels: map[string]string{
				"team":        "platform",
				"environment": "test",
			},
		},
		Spec: models.PolicySpec{
			Team: "platform-team",
			Members: []string{
				"dev1@company.com",
				"dev2@company.com",
			},
			Targets: []models.Target{
				{
					Type: models.TargetKubernetes,
					Name: "k8s-test",
					Actions: []string{"read", "write"},
					K8s: &models.K8sConfig{
						Namespace: "test-ns",
						Resources: []string{"pods", "services"},
						Verbs:     []string{"get", "list", "create"},
					},
				},
			},
		},
	}

	// Create policy
	id, err := pgStore.CreatePolicy(ctx, policy)
	if err != nil {
		log.Fatalf("Failed to create policy: %v", err)
	}
	fmt.Printf("✓ Created policy with ID: %d\n", id)

	// Retrieve policy
	retrieved, err := pgStore.GetPolicy(ctx, policy.Metadata.Name)
	if err != nil {
		log.Fatalf("Failed to get policy: %v", err)
	}
	fmt.Printf("✓ Retrieved policy: %s (Kind: %s)\n", retrieved.Metadata.Name, retrieved.Kind)

	// List policies
	policies, err := pgStore.ListPolicies(ctx, "", "")
	if err != nil {
		log.Fatalf("Failed to list policies: %v", err)
	}
	fmt.Printf("✓ Listed %d policies\n", len(policies))

	// Update status
	err = pgStore.UpdatePolicyStatus(ctx, policy.Metadata.Name, models.StateApplied, "Successfully reconciled")
	if err != nil {
		log.Fatalf("Failed to update status: %v", err)
	}
	fmt.Println("✓ Updated policy status to 'Applied'")

	// Create audit log
	auditEntry := &store.AuditLogEntry{
		PolicyID:   &id,
		PolicyName: policy.Metadata.Name,
		Action:     "CREATE",
		TargetType: "kubernetes",
		TargetName: "k8s-test",
		Actor:      "system",
		Details: map[string]interface{}{
			"namespace": "test-ns",
			"resources": []string{"pods", "services"},
		},
		Success: true,
	}
	err = pgStore.CreateAuditLog(ctx, auditEntry)
	if err != nil {
		log.Fatalf("Failed to create audit log: %v", err)
	}
	fmt.Println("✓ Created audit log entry")

	// Print final state
	fmt.Println("\n=== Final Policy State ===")
	finalPolicy, _ := pgStore.GetPolicy(ctx, policy.Metadata.Name)
	data, _ := json.MarshalIndent(finalPolicy, "", "  ")
	fmt.Println(string(data))

	// Cleanup
	err = pgStore.DeletePolicy(ctx, policy.Metadata.Name)
	if err != nil {
		log.Fatalf("Failed to delete policy: %v", err)
	}
	fmt.Println("\n✓ Cleaned up test policy")

	fmt.Println("\n🎉 All PostgreSQL tests passed!")
}
