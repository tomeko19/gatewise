// Package main provides the Gatewise Operator
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gatewise/gatewise/internal/connector"
	"github.com/gatewise/gatewise/internal/operator"
	"github.com/gatewise/gatewise/internal/reconciler"
	"github.com/gatewise/gatewise/internal/store"
	"github.com/gatewise/gatewise/pkg/license"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var (
	kubeconfig  string
	namespace   string
	dbURL       string
	licenseKey  string
)

func main() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (leave empty for in-cluster config)")
	flag.StringVar(&namespace, "namespace", "default", "Namespace to watch for GatewisePolicies")
	flag.StringVar(&dbURL, "db-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection string")
	flag.StringVar(&licenseKey, "license", os.Getenv("GATEWISE_LICENSE"), "Enterprise license key")
	flag.Parse()

	fmt.Println("════════════════════════════════════════════════════════")
	fmt.Println("  🚀 Gatewise Operator v0.3.0-alpha")
	fmt.Println("  GitOps-native Kubernetes Operator for Gatewise Policies")
	fmt.Println("════════════════════════════════════════════════════════")
	fmt.Println()

	// Initialize license manager
	licenseMgr := license.NewManager()
	tier := "Community"
	if licenseMgr.GetTier() == license.TierEnterprise {
		tier = "Enterprise"
	}
	if licenseKey != "" {
		fmt.Printf("📜 License Key: %s...\n", licenseKey[:min(10, len(licenseKey))])
	}
	fmt.Printf("📜 License: %s\n", tier)
	fmt.Printf("📦 Watching namespace: %s\n", namespace)
	fmt.Println()

	// Initialize PostgreSQL store
	if dbURL == "" {
		fmt.Println("❌ DATABASE_URL not set")
		os.Exit(1)
	}

	pgStore, err := store.NewPostgresStore(dbURL)
	if err != nil {
		fmt.Printf("❌ Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pgStore.Close()
	fmt.Println("✅ Connected to PostgreSQL")

	// Initialize connector factory
	factory := connector.NewFactory(licenseMgr)
	fmt.Println("✅ Connector factory initialized")

	// Initialize reconciler
	rec := reconciler.NewReconciler(reconciler.DefaultConfig(), factory, pgStore, licenseMgr)
	fmt.Println("✅ Reconciler initialized")
	fmt.Println()

	// Create operator
	op, err := operator.NewOperator(kubeconfig, namespace, rec, pgStore, licenseMgr)
	if err != nil {
		fmt.Printf("❌ Failed to create operator: %v\n", err)
		os.Exit(1)
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		fmt.Printf("\n⚠️  Received signal %v, shutting down...\n", sig)
		cancel()
	}()

	// Run operator
	if err := op.Run(ctx); err != nil && err != context.Canceled {
		fmt.Printf("❌ Operator error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("👋 Gatewise Operator stopped")
}
