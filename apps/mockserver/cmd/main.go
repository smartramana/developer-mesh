package main

import (
	"log"
	"net/http"

	"github.com/S-Corkum/devops-mcp/apps/mockserver/internal/handlers"
)

func main() {
	log.Println("Starting mock server on port 8081")

	// Setup all the handlers
	handlers.SetupHandlers()

	// Add a health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	// Start the server
	log.Fatal(http.ListenAndServe(":8081", nil))
}
