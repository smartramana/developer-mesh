package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// GitHubHandler handles mock GitHub API requests
func GitHubHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Mock GitHub request: %s %s", r.Method, r.URL.Path)
	w.Header().Set("Content-Type", "application/json")

	// Special handling for rate limit endpoint that's used for health checks
	if r.URL.Path == "/mock-github/rate_limit" {
		response := map[string]interface{}{
			"resources": map[string]interface{}{
				"core": map[string]interface{}{
					"limit":     5000,
					"used":      0,
					"remaining": 5000,
					"reset":     time.Now().Add(1 * time.Hour).Unix(),
				},
			},
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Special handling for health endpoint
	if r.URL.Path == "/mock-github/health" {
		response := map[string]interface{}{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Default response for other endpoints
	response := map[string]interface{}{
		"success":   true,
		"message":   "Mock GitHub response",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	json.NewEncoder(w).Encode(response)
}
