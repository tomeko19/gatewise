// Package main provides the Gatewise CLI tool
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gatewise/gatewise/internal/connector"
	"github.com/gatewise/gatewise/internal/policy"
	"github.com/gatewise/gatewise/internal/reconciler"
	"github.com/gatewise/gatewise/internal/store"
	"github.com/gatewise/gatewise/pkg/license"
	"github.com/gatewise/gatewise/pkg/models"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	version     = "0.2.0-alpha"
	outputFmt   string
	strict      bool
	kubeconfig  string
	dbURL       string
	licenseKey  string
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "gatewise",
		Short:   "Gatewise - Unified Access Layer for Modern Infrastructure",
		Long:    `Gatewise manages access and security for Kubernetes, Kafka, and Kong from a single control plane.`,
		Version: version,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&outputFmt, "output", "o", "yaml", "Output format (yaml, json, table)")
	rootCmd.PersistentFlags().BoolVar(&strict, "strict", false, "Enable strict parsing mode")
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	rootCmd.PersistentFlags().StringVar(&dbURL, "db-url", "", "PostgreSQL connection string")
	rootCmd.PersistentFlags().StringVar(&licenseKey, "license", "", "Enterprise license key")

	// Add commands
	rootCmd.AddCommand(parseCmd())
	rootCmd.AddCommand(validateCmd())
	rootCmd.AddCommand(applyCmd())
	rootCmd.AddCommand(getCmd())
	rootCmd.AddCommand(reconcileCmd())
	rootCmd.AddCommand(statusCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// parseCmd creates the 'parse' command
func parseCmd() *cobra.Command {
	var parseAll bool
	cmd := &cobra.Command{
		Use:   "parse [file]",
		Short: "Parse and display a policy file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parser := policy.NewParser(strict)
			
			if parseAll {
				// Parse all documents in the file
				file, err := os.Open(args[0])
				if err != nil {
					return fmt.Errorf("failed to open file: %w", err)
				}
				defer file.Close()
				
				policies, err := parser.ParseMultiple(file)
				if err != nil {
					return fmt.Errorf("failed to parse: %w", err)
				}
				
				fmt.Printf("Found %d policy document(s)\n---\n", len(policies))
				for i, p := range policies {
					fmt.Printf("# Document %d\n", i+1)
					if err := outputPolicy(p); err != nil {
						return err
					}
					if i < len(policies)-1 {
						fmt.Println("---")
					}
				}
				return nil
			}
			
			// Parse only first document
			p, err := parser.ParseFile(args[0])
			if err != nil {
				return fmt.Errorf("failed to parse: %w", err)
			}

			return outputPolicy(p)
		},
	}
	cmd.Flags().BoolVarP(&parseAll, "all", "a", false, "Parse all documents in multi-document YAML")
	return cmd
}

// validateCmd creates the 'validate' command
func validateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [file]",
		Short: "Validate a policy file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parser := policy.NewParser(strict)
			p, err := parser.ParseFile(args[0])
			if err != nil {
				return fmt.Errorf("failed to parse: %w", err)
			}

			validator := policy.NewValidator()
			result := validator.Validate(p)

			// Output results
			if result.IsValid() {
				fmt.Printf("✓ Policy '%s' is valid\n", p.Metadata.Name)
				if len(result.Warnings) > 0 {
					fmt.Println("\nWarnings:")
					for _, w := range result.Warnings {
						fmt.Printf("  ⚠ %s\n", w)
					}
				}
				return nil
			}

			fmt.Printf("✗ Policy validation failed\n\nErrors:\n")
			for _, e := range result.Errors {
				fmt.Printf("  • %s: %s\n", e.Field, e.Message)
			}
			if len(result.Warnings) > 0 {
				fmt.Println("\nWarnings:")
				for _, w := range result.Warnings {
					fmt.Printf("  ⚠ %s\n", w)
				}
			}

			return fmt.Errorf("validation failed with %d error(s)", len(result.Errors))
		},
	}
	return cmd
}

// applyCmd creates the 'apply' command
func applyCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "apply [file]",
		Short: "Apply a policy to the infrastructure",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse policy
			parser := policy.NewParser(strict)
			p, err := parser.ParseFile(args[0])
			if err != nil {
				return fmt.Errorf("failed to parse: %w", err)
			}

			// Validate
			validator := policy.NewValidator()
			result := validator.Validate(p)
			if !result.IsValid() {
				fmt.Printf("✗ Policy validation failed\n")
				for _, e := range result.Errors {
					fmt.Printf("  • %s: %s\n", e.Field, e.Message)
				}
				return fmt.Errorf("cannot apply invalid policy")
			}

			if dryRun {
				fmt.Printf("✓ Policy '%s' is valid (dry-run)\n", p.Metadata.Name)
				fmt.Println("\nResources that would be created:")
				for _, target := range p.Spec.Targets {
					fmt.Printf("  • [%s] %s\n", target.Type, target.Name)
					if target.K8s != nil {
						fmt.Printf("    - Namespace: %s\n", target.K8s.Namespace)
						fmt.Printf("    - Role: %s-role\n", p.Metadata.Name)
						fmt.Printf("    - RoleBinding: %s-binding\n", p.Metadata.Name)
						if p.Spec.Quotas != nil {
							fmt.Printf("    - ResourceQuota: %s-quota\n", p.Metadata.Name)
						}
					}
				}
				return nil
			}

			// Setup license manager
			licenseMgr := license.NewManager()
			if licenseKey != "" {
				licenseMgr.SetLicenseKey(licenseKey)
			}

			// Setup connector factory
			factory := connector.NewFactory(licenseMgr)

			// Register K8s connector
			mode := connector.ModeEmbedded
			if kubeconfig != "" {
				mode = connector.ModeExternal
			}
			factory.RegisterConfig(&connector.ConnectorConfig{
				Type:    connector.TypeKubernetes,
				Mode:    mode,
				Enabled: true,
				Credentials: map[string]string{
					"kubeconfig": kubeconfig,
				},
			})

			// Setup store if DB URL provided
			var pgStore *store.PostgresStore
			if dbURL != "" {
				var err error
				pgStore, err = store.NewPostgresStore(dbURL)
				if err != nil {
					fmt.Printf("⚠ Warning: Could not connect to database: %v\n", err)
				} else {
					defer pgStore.Close()
					// Store policy
					_, err = pgStore.CreatePolicy(context.Background(), p)
					if err != nil {
						fmt.Printf("⚠ Warning: Could not store policy: %v\n", err)
					}
				}
			}

			// Create reconciler
			rec := reconciler.NewReconciler(nil, factory, pgStore, licenseMgr)

			// Apply the policy
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			fmt.Printf("Applying policy '%s'...\n", p.Metadata.Name)
			err = rec.ReconcilePolicy(ctx, p)
			if err != nil {
				return fmt.Errorf("failed to apply policy: %w", err)
			}

			fmt.Printf("✓ Policy '%s' applied successfully\n", p.Metadata.Name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate without applying")
	return cmd
}

// reconcileCmd creates the 'reconcile' command
func reconcileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Reconcile all policies from database",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbURL == "" {
				return fmt.Errorf("--db-url is required for reconciliation")
			}

			ctx := context.Background()

			// Setup store
			pgStore, err := store.NewPostgresStore(dbURL)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer pgStore.Close()

			// Get all policies
			policies, err := pgStore.ListPolicies(ctx, "", "")
			if err != nil {
				return fmt.Errorf("failed to list policies: %w", err)
			}

			fmt.Printf("Found %d policies to reconcile\n", len(policies))

			// Setup reconciler
			licenseMgr := license.NewManager()
			if licenseKey != "" {
				licenseMgr.SetLicenseKey(licenseKey)
			}
			factory := connector.NewFactory(licenseMgr)
			mode := connector.ModeEmbedded
			if kubeconfig != "" {
				mode = connector.ModeExternal
			}
			factory.RegisterConfig(&connector.ConnectorConfig{
				Type:    connector.TypeKubernetes,
				Mode:    mode,
				Enabled: true,
				Credentials: map[string]string{
					"kubeconfig": kubeconfig,
				},
			})

			rec := reconciler.NewReconciler(nil, factory, pgStore, licenseMgr)

			// Reconcile each policy
			var succeeded, failed int
			for _, p := range policies {
				fmt.Printf("  → Reconciling '%s'...\n", p.Metadata.Name)
				err := rec.ReconcilePolicy(ctx, p)
				if err != nil {
					fmt.Printf("    ✗ Failed: %v\n", err)
					failed++
				} else {
					fmt.Printf("    ✓ Applied\n")
					succeeded++
				}
			}

			fmt.Printf("\nReconciliation complete: %d succeeded, %d failed\n", succeeded, failed)
			return nil
		},
	}
	return cmd
}

// getCmd creates the 'get' command
func getCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [resource] [name]",
		Short: "Get resources from the control plane",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbURL == "" {
				return fmt.Errorf("--db-url is required")
			}

			ctx := context.Background()
			pgStore, err := store.NewPostgresStore(dbURL)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer pgStore.Close()

			resource := strings.ToLower(args[0])

			switch resource {
			case "policy", "policies":
				if len(args) > 1 {
					// Get specific policy
					p, err := pgStore.GetPolicy(ctx, args[1])
					if err != nil {
						return err
					}
					if p == nil {
						return fmt.Errorf("policy '%s' not found", args[1])
					}
					return outputPolicy(p)
				}

				// List all policies
				policies, err := pgStore.ListPolicies(ctx, "", "")
				if err != nil {
					return err
				}

				if outputFmt == "table" {
					fmt.Printf("%-25s %-15s %-20s %-10s\n", "NAME", "KIND", "NAMESPACE", "STATUS")
					fmt.Println(strings.Repeat("-", 75))
					for _, p := range policies {
						fmt.Printf("%-25s %-15s %-20s %-10s\n", 
							p.Metadata.Name, p.Kind, p.Metadata.Namespace, "Synced")
					}
				} else {
					for _, p := range policies {
						outputPolicy(p)
						fmt.Println("---")
					}
				}
			default:
				return fmt.Errorf("unknown resource type: %s", resource)
			}

			return nil
		},
	}
	return cmd
}

// statusCmd creates the 'status' command
func statusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show Gatewise status and capabilities",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("╔════════════════════════════════════════════════════════════╗")
			fmt.Println("║               GATEWISE STATUS                              ║")
			fmt.Println("╠════════════════════════════════════════════════════════════╣")
			fmt.Printf("║  Version:     %-44s ║\n", version)

			// License info
			licenseMgr := license.NewManager()
			if licenseKey != "" {
				licenseMgr.SetLicenseKey(licenseKey)
			}
			fmt.Printf("║  License:     %-44s ║\n", licenseMgr.GetTier())

			// Features
			fmt.Println("╠════════════════════════════════════════════════════════════╣")
			fmt.Println("║  ENABLED FEATURES                                          ║")
			fmt.Println("╠════════════════════════════════════════════════════════════╣")
			features := licenseMgr.ListEnabledFeatures()
			for _, f := range features {
				fmt.Printf("║  ✓ %-55s ║\n", f)
			}

			// Connectors
			fmt.Println("╠════════════════════════════════════════════════════════════╣")
			fmt.Println("║  CONNECTORS                                                ║")
			fmt.Println("╠════════════════════════════════════════════════════════════╣")
			fmt.Printf("║  %-15s %-15s %-25s ║\n", "TYPE", "STATUS", "MODE")
			fmt.Printf("║  %-15s %-15s %-25s ║\n", "kubernetes", "✓ Available", "embedded/external")
			if licenseMgr.IsFeatureEnabled(license.FeatureKafkaConnector) {
				fmt.Printf("║  %-15s %-15s %-25s ║\n", "kafka", "○ Pending", "enterprise")
			} else {
				fmt.Printf("║  %-15s %-15s %-25s ║\n", "kafka", "✗ Locked", "enterprise")
			}
			if licenseMgr.IsFeatureEnabled(license.FeatureKongConnector) {
				fmt.Printf("║  %-15s %-15s %-25s ║\n", "kong", "○ Pending", "enterprise")
			} else {
				fmt.Printf("║  %-15s %-15s %-25s ║\n", "kong", "✗ Locked", "enterprise")
			}

			fmt.Println("╚════════════════════════════════════════════════════════════╝")
			return nil
		},
	}
	return cmd
}

// outputPolicy outputs the policy in the specified format
func outputPolicy(p *models.Policy) error {
	switch outputFmt {
	case "json":
		data, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	case "yaml":
		data, err := yaml.Marshal(p)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	case "table":
		fmt.Printf("KIND\t\tNAME\t\t\tNAMESPACE\n")
		fmt.Printf("%s\t%s\t\t%s\n", p.Kind, p.Metadata.Name, p.Metadata.Namespace)
	default:
		return fmt.Errorf("unknown output format: %s", outputFmt)
	}
	return nil
}
