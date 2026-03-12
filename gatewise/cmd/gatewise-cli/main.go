// Package main provides the Gatewise CLI tool
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/gatewise/gatewise/internal/policy"
	"github.com/gatewise/gatewise/pkg/models"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	version   = "0.1.0-alpha"
	outputFmt string
	strict    bool
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

	// Add commands
	rootCmd.AddCommand(parseCmd())
	rootCmd.AddCommand(validateCmd())
	rootCmd.AddCommand(applyCmd())
	rootCmd.AddCommand(getCmd())

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

// applyCmd creates the 'apply' command (placeholder for future)
func applyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply [file]",
		Short: "Apply a policy to the control plane",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parser := policy.NewParser(strict)
			p, err := parser.ParseFile(args[0])
			if err != nil {
				return fmt.Errorf("failed to parse: %w", err)
			}

			// Validate first
			validator := policy.NewValidator()
			result := validator.Validate(p)
			if !result.IsValid() {
				fmt.Printf("✗ Policy validation failed\n")
				for _, e := range result.Errors {
					fmt.Printf("  • %s: %s\n", e.Field, e.Message)
				}
				return fmt.Errorf("cannot apply invalid policy")
			}

			// TODO: Connect to server and apply policy
			fmt.Printf("✓ Policy '%s' parsed and validated\n", p.Metadata.Name)
			fmt.Println("  → Ready to apply (server connection not yet implemented)")
			return nil
		},
	}
	return cmd
}

// getCmd creates the 'get' command (placeholder for future)
func getCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [resource]",
		Short: "Get resources from the control plane",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Connect to server and get resources
			fmt.Printf("→ Get %s (server connection not yet implemented)\n", strings.Join(args, ", "))
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
