// Package main provides the Gatewise CLI tool
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/gatewise/gatewise/internal/policy"
	"github.com/gatewise/gatewise/pkg/models"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	version     = "0.3.0-alpha"
	outputFmt   string
	strict      bool
	serverURL   string
	licenseKey  string
	kubeconfig  string
	dbURL       string
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "gatewise",
		Short:   "Gatewise - Unified Access Layer for Modern Infrastructure",
		Long:    `Gatewise manages access and security for Kubernetes, Kafka, and Kong from a single control plane.`,
		Version: version,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&outputFmt, "output", "o", "table", "Output format (yaml, json, table)")
	rootCmd.PersistentFlags().BoolVar(&strict, "strict", false, "Enable strict parsing mode")
	rootCmd.PersistentFlags().StringVarP(&serverURL, "server", "s", "", "Gatewise server URL (default: http://localhost:8001)")
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	rootCmd.PersistentFlags().StringVar(&dbURL, "db-url", "", "PostgreSQL connection string")
	rootCmd.PersistentFlags().StringVar(&licenseKey, "license", "", "Enterprise license key")

	// Add all commands
	rootCmd.AddCommand(
		// Local commands
		parseCmd(),
		validateCmd(),
		
		// Server commands
		statusCmd(),
		policyCmd(),
		connectorCmd(),
		jitCmd(),
		auditCmd(),
		licenseCmd(),
		driftCmd(),
		serverCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// ============ Local Commands ============

func parseCmd() *cobra.Command {
	var parseAll bool
	cmd := &cobra.Command{
		Use:   "parse [file]",
		Short: "Parse and display a policy file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parser := policy.NewParser(strict)
			
			if parseAll {
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

func validateCmd() *cobra.Command {
	var remote bool
	cmd := &cobra.Command{
		Use:   "validate [file]",
		Short: "Validate a policy file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read file
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}

			if remote && serverURL != "" {
				// Validate via server
				return validateRemote(string(data))
			}

			// Local validation
			parser := policy.NewParser(strict)
			p, err := parser.ParseBytes(data)
			if err != nil {
				return fmt.Errorf("failed to parse: %w", err)
			}

			validator := policy.NewValidator()
			result := validator.Validate(p)

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
			return fmt.Errorf("validation failed with %d error(s)", len(result.Errors))
		},
	}
	cmd.Flags().BoolVar(&remote, "remote", false, "Validate using server API")
	return cmd
}

func validateRemote(yamlContent string) error {
	url := getServerURL() + "/api/policies/validate"
	body, _ := json.Marshal(map[string]string{"yaml": yamlContent})
	
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["valid"] == true {
		fmt.Println("✓ Policy is valid")
		return nil
	}

	fmt.Println("✗ Validation failed:")
	if errors, ok := result["errors"].([]interface{}); ok {
		for _, e := range errors {
			if errMap, ok := e.(map[string]interface{}); ok {
				fmt.Printf("  • %s: %s\n", errMap["field"], errMap["message"])
			}
		}
	}
	return fmt.Errorf("validation failed")
}

// ============ Status Command ============

func statusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show Gatewise status and capabilities",
		RunE: func(cmd *cobra.Command, args []string) error {
			url := getServerURL() + "/api/status"
			resp, err := http.Get(url)
			if err != nil {
				// Fallback to local status
				return showLocalStatus()
			}
			defer resp.Body.Close()

			var status map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&status)

			fmt.Println("╔════════════════════════════════════════════════════════════╗")
			fmt.Println("║               GATEWISE STATUS                              ║")
			fmt.Println("╠════════════════════════════════════════════════════════════╣")
			fmt.Printf("║  Version:     %-44s ║\n", status["version"])
			fmt.Printf("║  License:     %-44s ║\n", status["license"])
			fmt.Printf("║  Policies:    %-44v ║\n", status["policyCount"])
			fmt.Printf("║  Database:    %-44v ║\n", boolToStatus(status["databaseOk"]))
			
			fmt.Println("╠════════════════════════════════════════════════════════════╣")
			fmt.Println("║  CONNECTORS                                                ║")
			fmt.Println("╠════════════════════════════════════════════════════════════╣")
			
			if connectors, ok := status["connectors"].([]interface{}); ok {
				for _, c := range connectors {
					if conn, ok := c.(map[string]interface{}); ok {
						status := conn["status"].(string)
						icon := "✗"
						if status == "healthy" || status == "available" {
							icon = "✓"
						} else if status == "locked" {
							icon = "⚿"
						}
						fmt.Printf("║  %s %-12s %-42s ║\n", icon, conn["type"], status)
					}
				}
			}

			fmt.Println("╠════════════════════════════════════════════════════════════╣")
			fmt.Println("║  ENABLED FEATURES                                          ║")
			fmt.Println("╠════════════════════════════════════════════════════════════╣")
			
			if features, ok := status["features"].([]interface{}); ok {
				for _, f := range features {
					fmt.Printf("║  ✓ %-55s ║\n", f)
				}
			}

			fmt.Println("╚════════════════════════════════════════════════════════════╝")
			return nil
		},
	}
	return cmd
}

func showLocalStatus() error {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║               GATEWISE CLI (Offline Mode)                  ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Version:     %-44s ║\n", version)
	fmt.Println("║  Server:      Not connected                                ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	return nil
}

// ============ Policy Commands ============

func policyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Manage policies",
	}

	cmd.AddCommand(policyListCmd())
	cmd.AddCommand(policyGetCmd())
	cmd.AddCommand(policyCreateCmd())
	cmd.AddCommand(policyDeleteCmd())
	cmd.AddCommand(policyApplyCmd())
	cmd.AddCommand(policyReconcileCmd())

	return cmd
}

func policyListCmd() *cobra.Command {
	var kind, namespace string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all policies",
		RunE: func(cmd *cobra.Command, args []string) error {
			url := getServerURL() + "/api/policies"
			if kind != "" {
				url += "?kind=" + kind
			}
			
			resp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			policies, ok := result["policies"].([]interface{})
			if !ok || len(policies) == 0 {
				fmt.Println("No policies found")
				return nil
			}

			if outputFmt == "json" {
				output, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(output))
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tKIND\tNAMESPACE\tTEAM\tTARGETS\tSTATUS")
			fmt.Fprintln(w, "----\t----\t---------\t----\t-------\t------")
			
			for _, p := range policies {
				pol := p.(map[string]interface{})
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%v\t%s\n",
					pol["name"], pol["kind"], pol["namespace"], pol["team"], pol["targets"], pol["status"])
			}
			w.Flush()
			return nil
		},
	}
	cmd.Flags().StringVarP(&kind, "kind", "k", "", "Filter by kind")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Filter by namespace")
	return cmd
}

func policyGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [name]",
		Short: "Get a specific policy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := getServerURL() + "/api/policies/" + args[0]
			resp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == 404 {
				return fmt.Errorf("policy '%s' not found", args[0])
			}

			var policy map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&policy)

			if outputFmt == "json" {
				output, _ := json.MarshalIndent(policy, "", "  ")
				fmt.Println(string(output))
			} else if outputFmt == "yaml" {
				output, _ := yaml.Marshal(policy)
				fmt.Println(string(output))
			} else {
				fmt.Printf("Name:       %s\n", policy["name"])
				fmt.Printf("Kind:       %s\n", policy["kind"])
				fmt.Printf("Namespace:  %s\n", policy["namespace"])
				fmt.Printf("Team:       %s\n", policy["team"])
				fmt.Printf("Targets:    %v\n", policy["targets"])
				fmt.Printf("Status:     %s\n", policy["status"])
			}
			return nil
		},
	}
}

func policyCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create [file]",
		Short: "Create a policy from file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}

			url := getServerURL() + "/api/policies"
			body, _ := json.Marshal(map[string]string{"yaml": string(data)})
			
			resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			if resp.StatusCode >= 400 {
				return fmt.Errorf("failed to create policy: %v", result["error"])
			}

			fmt.Printf("✓ Policy '%s' created successfully\n", result["name"])
			return nil
		},
	}
}

func policyDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete [name]",
		Short: "Delete a policy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				fmt.Printf("Are you sure you want to delete policy '%s'? [y/N]: ", args[0])
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(response) != "y" {
					fmt.Println("Aborted")
					return nil
				}
			}

			url := getServerURL() + "/api/policies/" + args[0]
			req, _ := http.NewRequest("DELETE", url, nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			fmt.Printf("✓ Policy '%s' deleted\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}

func policyApplyCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "apply [file]",
		Short: "Apply a policy (create or update)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}

			if dryRun {
				// Just validate
				return validateRemote(string(data))
			}

			// Try to create, if exists then it will update
			url := getServerURL() + "/api/policies"
			body, _ := json.Marshal(map[string]string{"yaml": string(data)})
			
			resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			if resp.StatusCode >= 400 {
				return fmt.Errorf("failed to apply policy: %v", result["error"])
			}

			fmt.Printf("✓ Policy applied successfully\n")
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate without applying")
	return cmd
}

func policyReconcileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reconcile [name]",
		Short: "Force reconcile a policy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := getServerURL() + "/api/policies/" + args[0] + "/reconcile"
			resp, err := http.Post(url, "application/json", nil)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			if resp.StatusCode >= 400 {
				return fmt.Errorf("failed to reconcile: %v", result["error"])
			}

			fmt.Printf("✓ Policy '%s' reconciled - Status: %s\n", args[0], result["status"])
			return nil
		},
	}
}

// ============ Connector Commands ============

func connectorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connector",
		Short: "Manage infrastructure connectors",
	}

	cmd.AddCommand(connectorListCmd())
	cmd.AddCommand(connectorHealthCmd())

	return cmd
}

func connectorListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all connectors",
		RunE: func(cmd *cobra.Command, args []string) error {
			url := getServerURL() + "/api/connectors"
			resp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			connectors := result["connectors"].([]interface{})

			if outputFmt == "json" {
				output, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(output))
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "TYPE\tSTATUS\tMODE\tENABLED\tLICENSE")
			fmt.Fprintln(w, "----\t------\t----\t-------\t-------")
			
			for _, c := range connectors {
				conn := c.(map[string]interface{})
				license := "Community"
				if conn["type"] != "kubernetes" {
					license = "Enterprise"
				}
				enabled := "No"
				if conn["enabled"] == true {
					enabled = "Yes"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					conn["type"], conn["status"], conn["mode"], enabled, license)
			}
			w.Flush()
			return nil
		},
	}
}

func connectorHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health [type]",
		Short: "Check connector health",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := getServerURL() + "/api/connectors/" + args[0] + "/health"
			resp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			if resp.StatusCode >= 400 {
				fmt.Printf("✗ %s connector: %s\n", args[0], result["error"])
				return fmt.Errorf("health check failed")
			}

			fmt.Printf("✓ %s connector: %s\n", args[0], result["status"])
			return nil
		},
	}
}

// ============ JIT Commands ============

func jitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jit",
		Short: "Manage Just-In-Time access (Enterprise)",
	}

	cmd.AddCommand(jitRequestCmd())
	cmd.AddCommand(jitListCmd())
	cmd.AddCommand(jitRevokeCmd())
	cmd.AddCommand(jitRenewCmd())

	return cmd
}

func jitRequestCmd() *cobra.Command {
	var policy, reason, duration, grantee string
	cmd := &cobra.Command{
		Use:   "request",
		Short: "Request JIT access",
		RunE: func(cmd *cobra.Command, args []string) error {
			if policy == "" || grantee == "" {
				return fmt.Errorf("--policy and --grantee are required")
			}

			url := getServerURL() + "/api/jit/request"
			body, _ := json.Marshal(map[string]string{
				"policyName": policy,
				"grantee":    grantee,
				"reason":     reason,
				"duration":   duration,
			})
			
			resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			if resp.StatusCode >= 400 {
				return fmt.Errorf("failed to request access: %v", result["error"])
			}

			fmt.Printf("✓ JIT access granted\n")
			fmt.Printf("  ID:        %s\n", result["id"])
			fmt.Printf("  Policy:    %s\n", result["policyName"])
			fmt.Printf("  Grantee:   %s\n", result["grantee"])
			fmt.Printf("  Expires:   %s\n", result["expiresAt"])
			return nil
		},
	}
	cmd.Flags().StringVarP(&policy, "policy", "p", "", "Policy name")
	cmd.Flags().StringVarP(&grantee, "grantee", "g", "", "Grantee (user/email)")
	cmd.Flags().StringVarP(&reason, "reason", "r", "", "Reason for access")
	cmd.Flags().StringVarP(&duration, "duration", "d", "2h", "Access duration (e.g., 2h, 30m)")
	return cmd
}

func jitListCmd() *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List JIT grants",
		RunE: func(cmd *cobra.Command, args []string) error {
			url := getServerURL() + "/api/jit"
			if status != "" {
				url += "?status=" + status
			}

			resp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			grants, ok := result["grants"].([]interface{})
			if !ok || len(grants) == 0 {
				fmt.Println("No JIT grants found")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tPOLICY\tGRANTEE\tSTATUS\tEXPIRES")
			fmt.Fprintln(w, "--\t------\t-------\t------\t-------")
			
			for _, g := range grants {
				grant := g.(map[string]interface{})
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					grant["id"], grant["policyName"], grant["grantee"], grant["status"], grant["expiresAt"])
			}
			w.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (active, expired, revoked)")
	return cmd
}

func jitRevokeCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "revoke [id]",
		Short: "Revoke a JIT grant",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := getServerURL() + "/api/jit/" + args[0] + "/revoke"
			body, _ := json.Marshal(map[string]string{"reason": reason})
			
			resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			fmt.Printf("✓ JIT grant '%s' revoked\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVarP(&reason, "reason", "r", "", "Reason for revocation")
	return cmd
}

func jitRenewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "renew [id]",
		Short: "Renew a JIT grant",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := getServerURL() + "/api/jit/" + args[0] + "/renew"
			resp, err := http.Post(url, "application/json", nil)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			if resp.StatusCode >= 400 {
				return fmt.Errorf("failed to renew: %v", result["error"])
			}

			fmt.Printf("✓ JIT grant renewed - New expiry: %s\n", result["expiresAt"])
			return nil
		},
	}
}

// ============ Audit Commands ============

func auditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "View audit logs",
	}

	cmd.AddCommand(auditListCmd())

	return cmd
}

func auditListCmd() *cobra.Command {
	var limit int
	var policy, action string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List audit logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			url := getServerURL() + "/api/audit"
			resp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			logs, ok := result["logs"].([]interface{})
			if !ok || len(logs) == 0 {
				fmt.Println("No audit logs found")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "TIMESTAMP\tACTION\tPOLICY\tACTOR\tDETAILS")
			fmt.Fprintln(w, "---------\t------\t------\t-----\t-------")
			
			for _, l := range logs {
				log := l.(map[string]interface{})
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					log["timestamp"], log["action"], log["policy"], log["actor"], log["details"])
			}
			w.Flush()
			return nil
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "l", 50, "Number of logs to show")
	cmd.Flags().StringVarP(&policy, "policy", "p", "", "Filter by policy")
	cmd.Flags().StringVarP(&action, "action", "a", "", "Filter by action")
	return cmd
}

// ============ License Commands ============

func licenseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "license",
		Short: "Manage license",
	}

	cmd.AddCommand(licenseShowCmd())
	cmd.AddCommand(licenseSetCmd())

	return cmd
}

func licenseShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current license",
		RunE: func(cmd *cobra.Command, args []string) error {
			url := getServerURL() + "/api/license"
			resp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			fmt.Printf("License Tier: %s\n\n", strings.ToUpper(result["tier"].(string)))
			fmt.Println("Enabled Features:")
			
			if features, ok := result["features"].([]interface{}); ok {
				for _, f := range features {
					fmt.Printf("  ✓ %s\n", f)
				}
			}
			return nil
		},
	}
}

func licenseSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set [key]",
		Short: "Set license key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := getServerURL() + "/api/license"
			body, _ := json.Marshal(map[string]string{"key": args[0]})
			
			resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			fmt.Printf("✓ License updated to: %s\n", strings.ToUpper(result["tier"].(string)))
			return nil
		},
	}
}

// ============ Drift Commands ============

func driftCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Manage drift detection (Enterprise)",
	}

	cmd.AddCommand(driftListCmd())
	cmd.AddCommand(driftRepairCmd())

	return cmd
}

func driftListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List detected drifts",
		RunE: func(cmd *cobra.Command, args []string) error {
			url := getServerURL() + "/api/drift"
			resp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			drifts, ok := result["drifts"].([]interface{})
			if !ok || len(drifts) == 0 {
				fmt.Println("✓ No drifts detected - All resources in sync")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tPOLICY\tRESOURCE\tTYPE\tSTATUS\tDETECTED")
			fmt.Fprintln(w, "--\t------\t--------\t----\t------\t--------")
			
			for _, d := range drifts {
				drift := d.(map[string]interface{})
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					drift["id"], drift["policyName"], drift["resourceName"], 
					drift["driftType"], drift["status"], drift["detectedAt"])
			}
			w.Flush()
			return nil
		},
	}
}

func driftRepairCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "repair [id]",
		Short: "Repair a detected drift",
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				url := getServerURL() + "/api/drift/repair-all"
				resp, err := http.Post(url, "application/json", nil)
				if err != nil {
					return fmt.Errorf("failed to connect to server: %w", err)
				}
				defer resp.Body.Close()
				fmt.Println("✓ All drifts repaired")
				return nil
			}

			if len(args) == 0 {
				return fmt.Errorf("drift ID required (or use --all)")
			}

			url := getServerURL() + "/api/drift/" + args[0] + "/repair"
			resp, err := http.Post(url, "application/json", nil)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			fmt.Printf("✓ Drift '%s' repaired\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Repair all drifts")
	return cmd
}

// ============ Server Command ============

func serverCmd() *cobra.Command {
	var port string
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the Gatewise server",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Starting Gatewise server...")
			fmt.Printf("Use 'gatewise-server --port %s' to start the full server\n", port)
			return nil
		},
	}
	cmd.Flags().StringVar(&port, "port", "8001", "Server port")
	return cmd
}

// ============ Helper Functions ============

func getServerURL() string {
	if serverURL != "" {
		return strings.TrimSuffix(serverURL, "/")
	}
	if env := os.Getenv("GATEWISE_SERVER_URL"); env != "" {
		return strings.TrimSuffix(env, "/")
	}
	return "http://localhost:8001"
}

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
		fmt.Printf("KIND\\t\\tNAME\\t\\t\\tNAMESPACE\\n")
		fmt.Printf("%s\\t%s\\t\\t%s\\n", p.Kind, p.Metadata.Name, p.Metadata.Namespace)
	default:
		data, err := yaml.Marshal(p)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	}
	return nil
}

func boolToStatus(v interface{}) string {
	if v == true {
		return "✓ Connected"
	}
	return "✗ Disconnected"
}
