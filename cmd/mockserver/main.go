package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

func main() {
	log.Println("Starting mock server on port 8081")

	// GitHub API mock
	http.HandleFunc("/mock-github/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Mock GitHub request: %s %s", r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": true,
			"message": "Mock GitHub response",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		json.NewEncoder(w).Encode(response)
	})

	// Harness API mock
	http.HandleFunc("/mock-harness/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Mock Harness request: %s %s", r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": true,
			"message": "Mock Harness response",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		json.NewEncoder(w).Encode(response)
	})

	// SonarQube API mock
	http.HandleFunc("/mock-sonarqube/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Mock SonarQube request: %s %s", r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": true,
			"message": "Mock SonarQube response",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		json.NewEncoder(w).Encode(response)
	})

	// Artifactory API mock
	http.HandleFunc("/mock-artifactory/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Mock Artifactory request: %s %s", r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": true,
			"message": "Mock Artifactory response",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		json.NewEncoder(w).Encode(response)
	})

	// Xray API mock
	http.HandleFunc("/mock-xray/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Mock Xray request: %s %s", r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": true,
			"message": "Mock Xray response",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		json.NewEncoder(w).Encode(response)
	})

	// Mock webhook endpoints
	http.HandleFunc("/api/v1/webhook/github", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Mock GitHub webhook received: %s", r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	http.HandleFunc("/api/v1/webhook/harness", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Mock Harness webhook received: %s", r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	http.HandleFunc("/api/v1/webhook/sonarqube", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Mock SonarQube webhook received: %s", r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	http.HandleFunc("/api/v1/webhook/artifactory", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Mock Artifactory webhook received: %s", r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	http.HandleFunc("/api/v1/webhook/xray", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Mock Xray webhook received: %s", r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Add a health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	// Start the server
	log.Fatal(http.ListenAndServe(":8081", nil))
}
