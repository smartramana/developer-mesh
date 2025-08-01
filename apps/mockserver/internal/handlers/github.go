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
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	// Special handling for health endpoint
	if r.URL.Path == "/mock-github/health" {
		response := map[string]interface{}{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	// Handle OpenAPI spec request
	if r.URL.Path == "/mock-github/openapi.json" || r.URL.Path == "/mock-github/api/v3/openapi.json" {
		spec := map[string]interface{}{
			"openapi": "3.0.0",
			"info": map[string]interface{}{
				"title":       "Mock GitHub API",
				"version":     "1.0.0",
				"description": "Mock GitHub API for testing",
			},
			"servers": []map[string]interface{}{
				{
					"url": "http://localhost:8081/mock-github",
				},
			},
			"paths": map[string]interface{}{
				"/user": map[string]interface{}{
					"get": map[string]interface{}{
						"summary":     "Get authenticated user",
						"operationId": "getUser",
						"responses": map[string]interface{}{
							"200": map[string]interface{}{
								"description": "Success",
								"content": map[string]interface{}{
									"application/json": map[string]interface{}{
										"schema": map[string]interface{}{
											"type": "object",
											"properties": map[string]interface{}{
												"login": map[string]string{"type": "string"},
												"id":    map[string]string{"type": "integer"},
												"name":  map[string]string{"type": "string"},
											},
										},
									},
								},
							},
						},
					},
				},
				"/repos/{owner}/{repo}": map[string]interface{}{
					"get": map[string]interface{}{
						"summary":     "Get repository",
						"operationId": "getRepo",
						"parameters": []map[string]interface{}{
							{
								"name":     "owner",
								"in":       "path",
								"required": true,
								"schema":   map[string]string{"type": "string"},
							},
							{
								"name":     "repo",
								"in":       "path",
								"required": true,
								"schema":   map[string]string{"type": "string"},
							},
						},
						"responses": map[string]interface{}{
							"200": map[string]interface{}{
								"description": "Success",
							},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(spec)
		return
	}

	// Default response for other endpoints
	response := map[string]interface{}{
		"success":   true,
		"message":   "Mock GitHub response",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	_ = json.NewEncoder(w).Encode(response)
}
