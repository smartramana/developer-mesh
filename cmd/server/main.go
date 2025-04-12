package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/username/mcp-server/internal/api"
	"github.com/username/mcp-server/internal/cache"
	"github.com/username/mcp-server/internal/config"
	"github.com/username/mcp-server/internal/core"
	"github.com/username/mcp-server/internal/database"
	"github.com/username/mcp-server/internal/metrics"
)

func main() {
	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize metrics
	metricsClient := metrics.NewClient(cfg.Metrics)
	defer metricsClient.Close()

	// Initialize database
	db, err := database.NewDatabase(ctx, cfg.Database)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize cache
	cacheClient, err := cache.NewCache(ctx, cfg.Cache)
	if err != nil {
		log.Fatalf("Failed to initialize cache: %v", err)
	}
	defer cacheClient.Close()

	// Initialize core engine
	engine, err := core.NewEngine(ctx, cfg.Engine, db, cacheClient, metricsClient)
	if err != nil {
		log.Fatalf("Failed to initialize core engine: %v", err)
	}
	defer engine.Shutdown(ctx)

	// Initialize API server
	server := api.NewServer(engine, cfg.API)

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on %s", cfg.API.ListenAddress)
		if err := server.Start(); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Received shutdown signal")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCancel()

	// Shutdown API server first
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("API server shutdown error: %v", err)
	}

	log.Println("Server stopped gracefully")
}
