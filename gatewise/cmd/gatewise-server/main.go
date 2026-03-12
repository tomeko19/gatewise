// Package main provides the Gatewise Server
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gatewise/gatewise/internal/api"
	"github.com/gatewise/gatewise/internal/connector"
	"github.com/gatewise/gatewise/internal/reconciler"
	"github.com/gatewise/gatewise/internal/store"
	"github.com/gatewise/gatewise/pkg/license"
)

var (
	port       = flag.String("port", "8001", "API server port")
	dbURL      = flag.String("db-url", "", "PostgreSQL connection string")
	licenseKey = flag.String("license", "", "Enterprise license key")
	kubeconfig = flag.String("kubeconfig", "", "Path to kubeconfig file")
)

func main() {
	flag.Parse()

	// Check environment variables
	if *dbURL == "" {
		*dbURL = os.Getenv("DATABASE_URL")
	}
	if *licenseKey == "" {
		*licenseKey = os.Getenv("GATEWISE_LICENSE_KEY")
	}
	if *kubeconfig == "" {
		*kubeconfig = os.Getenv("KUBECONFIG")
	}

	log.Println("╔══════════════════════════════════════════════════╗")
	log.Println("║        GATEWISE CONTROL PLANE                    ║")
	log.Println("╚══════════════════════════════════════════════════╝")

	// Setup license manager
	licenseMgr := license.NewManager()
	if *licenseKey != "" {
		licenseMgr.SetLicenseKey(*licenseKey)
		log.Printf("→ License: %s", licenseMgr.GetTier())
	} else {
		log.Println("→ License: Community (free)")
	}

	// Setup PostgreSQL
	var pgStore *store.PostgresStore
	var err error
	if *dbURL != "" {
		pgStore, err = store.NewPostgresStore(*dbURL)
		if err != nil {
			log.Printf("⚠ Warning: Could not connect to PostgreSQL: %v", err)
		} else {
			log.Println("✓ Connected to PostgreSQL")
			defer pgStore.Close()
		}
	} else {
		log.Println("⚠ Warning: No database URL configured (--db-url)")
	}

	// Setup connector factory
	factory := connector.NewFactory(licenseMgr)

	// Register Kubernetes connector
	mode := connector.ModeEmbedded
	if *kubeconfig != "" {
		mode = connector.ModeExternal
	}
	factory.RegisterConfig(&connector.ConnectorConfig{
		Type:    connector.TypeKubernetes,
		Mode:    mode,
		Enabled: true,
		Credentials: map[string]string{
			"kubeconfig": *kubeconfig,
		},
	})
	log.Printf("✓ Kubernetes connector registered (mode: %s)", mode)

	// Setup reconciler
	rec := reconciler.NewReconciler(nil, factory, pgStore, licenseMgr)

	// Create API server
	server := api.NewServer(pgStore, factory, rec, licenseMgr)

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("\n→ Shutting down...")
		cancel()
	}()

	_ = ctx // Used for future background tasks

	// Start server
	addr := fmt.Sprintf("0.0.0.0:%s", *port)
	log.Printf("✓ API server listening on %s", addr)
	log.Println("")
	log.Println("Endpoints:")
	log.Println("  GET  /api/health          - Health check")
	log.Println("  GET  /api/status          - Server status")
	log.Println("  GET  /api/policies        - List policies")
	log.Println("  POST /api/policies        - Create policy")
	log.Println("  GET  /api/connectors      - List connectors")
	log.Println("  GET  /api/license         - Get license info")
	log.Println("")

	if err := server.Run(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
