package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mockserver/internal/handlers"
)

func main() {
	// Command line flags
	var (
		port = flag.String("port", "8082", "Port to run the mock server on")
		host = flag.String("host", "", "Host to bind the server to")
	)
	flag.Parse()

	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	addr := fmt.Sprintf("%s:%s", *host, *port)
	log.Printf("Starting mock server on %s", addr)

	// Create a new ServeMux for better control
	mux := http.NewServeMux()

	// Setup all the handlers
	handlers.SetupHandlers(mux)

	// Add a health check endpoint
	mux.HandleFunc("/health", healthHandler)

	// Create server with timeouts for production use
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Run server in a goroutine so it doesn't block
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

// healthHandler returns the health status of the mock server
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// Use fmt.Fprintf for better error handling
	if _, err := fmt.Fprintf(w, `{"status":"healthy","timestamp":"%s"}`, time.Now().Format(time.RFC3339)); err != nil {
		log.Printf("Failed to write health response: %v", err)
	}
}
