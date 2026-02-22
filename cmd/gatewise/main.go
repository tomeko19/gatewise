// Command gatewise starts the Gatewise API server.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tomeko19/gatewise/internal/server"
)

func main() {
	addr := getenv("GATEWISE_ADDR", ":8080")

	srv := server.New(addr)

	// graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("[gatewise] server stopped: %v", err)
		}
	}()

	<-stop
	log.Println("[gatewise] shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = srv.Shutdown(ctx)
	log.Println("[gatewise] bye")
}

func getenv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}
