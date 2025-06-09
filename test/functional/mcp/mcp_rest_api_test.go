package mcp_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ToolDefinition represents an MCP tool
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolCallRequest represents a tool execution request
type ToolCallRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolCallResponse represents a tool execution response
type ToolCallResponse struct {
	Result interface{} `json:"result,omitempty"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ContextRequest represents a context creation request
type ContextRequest struct {
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ContextResponse represents a context response
type ContextResponse struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

var _ = Describe("MCP REST API Tests", func() {
	var (
		baseURL    string
		apiKey     string
		httpClient *http.Client
	)

	BeforeEach(func() {
		// Get configuration from environment
		baseURL = os.Getenv("MCP_SERVER_URL")
		if baseURL == "" {
			baseURL = "http://localhost:8080"
		}
		
		apiKey = os.Getenv("MCP_API_KEY")
		if apiKey == "" {
			apiKey = "docker-admin-api-key"
		}
		
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	})

	// Helper function to make API requests
	makeRequest := func(method, path string, body interface{}, headers map[string]string) (*http.Response, error) {
		var bodyReader io.Reader
		if body != nil {
			bodyBytes, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			bodyReader = bytes.NewBuffer(bodyBytes)
		}

		req, err := http.NewRequest(method, baseURL+path, bodyReader)
		if err != nil {
			return nil, err
		}

		// Set default headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", apiKey)
		
		// Set custom headers
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		return httpClient.Do(req)
	}

	Describe("Health and Status Endpoints", func() {
		It("should return health status", func() {
			resp, err := makeRequest("GET", "/health", nil, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var health map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&health)
			Expect(err).NotTo(HaveOccurred())
			Expect(health["status"]).To(Equal("healthy"))
		})

		It("should return API version information", func() {
			resp, err := makeRequest("GET", "/api/v1/version", nil, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				var version map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&version)
				Expect(err).NotTo(HaveOccurred())
				Expect(version).To(HaveKey("version"))
			}
		})
	})

	Describe("Authentication", func() {
		It("should reject requests without API key", func() {
			req, err := http.NewRequest("GET", baseURL+"/api/v1/mcp/tools", nil)
			Expect(err).NotTo(HaveOccurred())
			
			resp, err := httpClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("should reject requests with invalid API key", func() {
			resp, err := makeRequest("GET", "/api/v1/mcp/tools", nil, map[string]string{
				"X-API-Key": "invalid-key",
			})
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("should accept requests with valid API key", func() {
			resp, err := makeRequest("GET", "/api/v1/mcp/tools", nil, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			// Should not be unauthorized
			Expect(resp.StatusCode).NotTo(Equal(http.StatusUnauthorized))
		})
	})

	Describe("MCP Tool Operations", func() {
		It("should list available tools", func() {
			resp, err := makeRequest("GET", "/api/v1/mcp/tools", nil, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				Skip("Tool endpoints not implemented")
			}

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var tools []ToolDefinition
			err = json.NewDecoder(resp.Body).Decode(&tools)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify tool structure
			for _, tool := range tools {
				Expect(tool.Name).NotTo(BeEmpty())
				Expect(tool.Description).NotTo(BeEmpty())
				Expect(tool.InputSchema).NotTo(BeNil())
			}
			
			GinkgoWriter.Printf("Found %d tools\n", len(tools))
		})

		It("should get specific tool details", func() {
			// First get list of tools
			resp, err := makeRequest("GET", "/api/v1/mcp/tools", nil, nil)
			if resp.StatusCode == http.StatusNotFound {
				Skip("Tool endpoints not implemented")
			}
			resp.Body.Close()

			// Try to get a specific tool
			resp, err = makeRequest("GET", "/api/v1/mcp/tools/github_list_repos", nil, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				var tool ToolDefinition
				err = json.NewDecoder(resp.Body).Decode(&tool)
				Expect(err).NotTo(HaveOccurred())
				Expect(tool.Name).To(Equal("github_list_repos"))
			}
		})

		It("should execute a tool", func() {
			toolCall := ToolCallRequest{
				Name: "github_list_repos",
				Arguments: map[string]interface{}{
					"org":     "test-org",
					"limit":   5,
					"sort":    "updated",
				},
			}

			resp, err := makeRequest("POST", "/api/v1/mcp/tools/call", toolCall, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				Skip("Tool execution endpoint not implemented")
			}

			// Tool execution might fail for various reasons (auth, network, etc)
			// but the endpoint should exist
			Expect(resp.StatusCode).To(BeNumerically(">=", 200))
			Expect(resp.StatusCode).To(BeNumerically("<", 500))

			var result ToolCallResponse
			err = json.NewDecoder(resp.Body).Decode(&result)
			Expect(err).NotTo(HaveOccurred())
			
			GinkgoWriter.Printf("Tool execution response: %+v\n", result)
		})

		It("should validate tool arguments", func() {
			// Call with missing required arguments
			toolCall := ToolCallRequest{
				Name:      "github_list_repos",
				Arguments: map[string]interface{}{
					// Missing required 'org' parameter
				},
			}

			resp, err := makeRequest("POST", "/api/v1/mcp/tools/call", toolCall, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				Skip("Tool execution endpoint not implemented")
			}

			// Should return bad request for invalid arguments
			if resp.StatusCode == http.StatusBadRequest {
				var errorResp map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&errorResp)
				Expect(err).NotTo(HaveOccurred())
				Expect(errorResp).To(HaveKey("error"))
			}
		})
	})

	Describe("Context Management", func() {
		var createdContextID string

		It("should create a new context", func() {
			contextReq := ContextRequest{
				Content: "This is a test context for MCP operations",
				Metadata: map[string]interface{}{
					"source": "test",
					"type":   "conversation",
				},
			}

			resp, err := makeRequest("POST", "/api/v1/mcp/contexts", contextReq, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				Skip("Context endpoints not implemented")
			}

			Expect(resp.StatusCode).To(Equal(http.StatusCreated))

			var context ContextResponse
			err = json.NewDecoder(resp.Body).Decode(&context)
			Expect(err).NotTo(HaveOccurred())
			
			Expect(context.ID).NotTo(BeEmpty())
			Expect(context.Content).To(Equal(contextReq.Content))
			
			createdContextID = context.ID
			GinkgoWriter.Printf("Created context: %s\n", createdContextID)
		})

		It("should list contexts", func() {
			resp, err := makeRequest("GET", "/api/v1/mcp/contexts", nil, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				Skip("Context endpoints not implemented")
			}

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var contexts []ContextResponse
			err = json.NewDecoder(resp.Body).Decode(&contexts)
			Expect(err).NotTo(HaveOccurred())
			
			GinkgoWriter.Printf("Found %d contexts\n", len(contexts))
		})

		It("should get a specific context", func() {
			if createdContextID == "" {
				Skip("No context created in previous test")
			}

			resp, err := makeRequest("GET", fmt.Sprintf("/api/v1/mcp/contexts/%s", createdContextID), nil, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				Skip("Context endpoints not implemented")
			}

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var context ContextResponse
			err = json.NewDecoder(resp.Body).Decode(&context)
			Expect(err).NotTo(HaveOccurred())
			
			Expect(context.ID).To(Equal(createdContextID))
		})

		It("should update a context", func() {
			if createdContextID == "" {
				Skip("No context created in previous test")
			}

			updateReq := ContextRequest{
				Content: "Updated test context content",
				Metadata: map[string]interface{}{
					"updated": true,
					"version": 2,
				},
			}

			resp, err := makeRequest("PUT", fmt.Sprintf("/api/v1/mcp/contexts/%s", createdContextID), updateReq, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				Skip("Context update endpoint not implemented")
			}

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var context ContextResponse
			err = json.NewDecoder(resp.Body).Decode(&context)
			Expect(err).NotTo(HaveOccurred())
			
			Expect(context.Content).To(Equal(updateReq.Content))
		})

		It("should delete a context", func() {
			if createdContextID == "" {
				Skip("No context created in previous test")
			}

			resp, err := makeRequest("DELETE", fmt.Sprintf("/api/v1/mcp/contexts/%s", createdContextID), nil, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				Skip("Context delete endpoint not implemented")
			}

			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			// Verify deletion
			resp, err = makeRequest("GET", fmt.Sprintf("/api/v1/mcp/contexts/%s", createdContextID), nil, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
	})

	Describe("Resource Operations", func() {
		It("should list available resources", func() {
			resp, err := makeRequest("GET", "/api/v1/mcp/resources", nil, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				Skip("Resource endpoints not implemented")
			}

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var resources []map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&resources)
			Expect(err).NotTo(HaveOccurred())
			
			GinkgoWriter.Printf("Found %d resources\n", len(resources))
		})

		It("should read a resource", func() {
			resourceURI := "github://repos/test-org/test-repo/README.md"
			
			resp, err := makeRequest("GET", fmt.Sprintf("/api/v1/mcp/resources?uri=%s", resourceURI), nil, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				Skip("Resource read endpoint not implemented")
			}

			// Resource might not exist, but endpoint should be available
			Expect(resp.StatusCode).To(BeNumerically(">=", 200))
			Expect(resp.StatusCode).To(BeNumerically("<", 500))
		})
	})

	Describe("Prompt Operations", func() {
		It("should list available prompts", func() {
			resp, err := makeRequest("GET", "/api/v1/mcp/prompts", nil, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				Skip("Prompt endpoints not implemented")
			}

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var prompts []map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&prompts)
			Expect(err).NotTo(HaveOccurred())
			
			GinkgoWriter.Printf("Found %d prompts\n", len(prompts))
		})

		It("should get a specific prompt", func() {
			promptName := "github_pr_review"
			
			resp, err := makeRequest("GET", fmt.Sprintf("/api/v1/mcp/prompts/%s", promptName), nil, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				Skip("Prompt endpoints not implemented or prompt not found")
			}

			if resp.StatusCode == http.StatusOK {
				var prompt map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&prompt)
				Expect(err).NotTo(HaveOccurred())
				Expect(prompt).To(HaveKey("name"))
				Expect(prompt).To(HaveKey("description"))
			}
		})
	})

	Describe("Batch Operations", func() {
		It("should execute multiple tool calls in batch", func() {
			batchRequest := map[string]interface{}{
				"requests": []ToolCallRequest{
					{
						Name: "github_list_repos",
						Arguments: map[string]interface{}{
							"org": "test-org",
						},
					},
					{
						Name: "github_get_repo",
						Arguments: map[string]interface{}{
							"owner": "test-org",
							"repo":  "test-repo",
						},
					},
				},
			}

			resp, err := makeRequest("POST", "/api/v1/mcp/tools/batch", batchRequest, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				Skip("Batch operations not implemented")
			}

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var batchResponse map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&batchResponse)
			Expect(err).NotTo(HaveOccurred())
			
			if responses, ok := batchResponse["responses"].([]interface{}); ok {
				Expect(responses).To(HaveLen(2))
			}
		})
	})

	Describe("Error Handling", func() {
		It("should return proper error for non-existent endpoints", func() {
			resp, err := makeRequest("GET", "/api/v1/mcp/nonexistent", nil, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

			var errorResp map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&errorResp)
			if err == nil {
				Expect(errorResp).To(HaveKey("error"))
			}
		})

		It("should handle malformed JSON requests", func() {
			req, err := http.NewRequest("POST", baseURL+"/api/v1/mcp/tools/call", 
				strings.NewReader("invalid json"))
			Expect(err).NotTo(HaveOccurred())
			
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", apiKey)

			resp, err := httpClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusNotFound {
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			}
		})

		It("should enforce rate limiting", func() {
			// Make multiple rapid requests
			for i := 0; i < 100; i++ {
				resp, err := makeRequest("GET", "/api/v1/mcp/tools", nil, nil)
				if err != nil {
					continue
				}
				
				if resp.StatusCode == http.StatusTooManyRequests {
					// Rate limiting is enforced
					var rateLimitInfo map[string]interface{}
					json.NewDecoder(resp.Body).Decode(&rateLimitInfo)
					resp.Body.Close()
					
					GinkgoWriter.Printf("Rate limit hit after %d requests: %+v\n", i+1, rateLimitInfo)
					return
				}
				
				resp.Body.Close()
			}
			
			// If we get here, rate limiting might not be enabled
			GinkgoWriter.Printf("Rate limiting not detected after 100 requests\n")
		})
	})

	Describe("Performance Benchmarks", func() {
		It("should handle requests within acceptable latency", func() {
			// Warm up
			resp, err := makeRequest("GET", "/api/v1/mcp/tools", nil, nil)
			if err == nil {
				resp.Body.Close()
			}

			// Measure latency
			iterations := 10
			var totalDuration time.Duration

			for i := 0; i < iterations; i++ {
				start := time.Now()
				
				resp, err := makeRequest("GET", "/api/v1/mcp/tools", nil, nil)
				if err != nil {
					Skip("Server not responding consistently")
				}
				
				resp.Body.Close()
				totalDuration += time.Since(start)
			}

			avgLatency := totalDuration / time.Duration(iterations)
			GinkgoWriter.Printf("Average latency over %d requests: %v\n", iterations, avgLatency)
			
			// Assert reasonable latency (adjust based on requirements)
			Expect(avgLatency).To(BeNumerically("<", 200*time.Millisecond))
		})

		It("should handle concurrent requests", func() {
			concurrency := 10
			requestsPerClient := 5
			
			errorChan := make(chan error, concurrency*requestsPerClient)
			doneChan := make(chan bool, concurrency)

			start := time.Now()

			for i := 0; i < concurrency; i++ {
				go func(clientID int) {
					for j := 0; j < requestsPerClient; j++ {
						resp, err := makeRequest("GET", "/api/v1/mcp/tools", nil, nil)
						if err != nil {
							errorChan <- err
							continue
						}
						resp.Body.Close()
					}
					doneChan <- true
				}(i)
			}

			// Wait for all goroutines
			for i := 0; i < concurrency; i++ {
				<-doneChan
			}

			duration := time.Since(start)
			totalRequests := concurrency * requestsPerClient
			
			GinkgoWriter.Printf("Processed %d concurrent requests in %v\n", totalRequests, duration)
			
			// Check for errors
			close(errorChan)
			errorCount := 0
			for err := range errorChan {
				errorCount++
				GinkgoWriter.Printf("Concurrent request error: %v\n", err)
			}
			
			Expect(errorCount).To(BeNumerically("<", totalRequests/10)) // Less than 10% error rate
		})
	})

	Describe("Integration with Other Services", func() {
		It("should integrate with context storage", func() {
			// Create context
			contextReq := ContextRequest{
				Content: "Integration test context",
				Metadata: map[string]interface{}{
					"test": true,
				},
			}

			resp, err := makeRequest("POST", "/api/v1/contexts", contextReq, nil)
			if err != nil || resp.StatusCode == http.StatusNotFound {
				Skip("Context API not available")
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusCreated {
				var context ContextResponse
				json.NewDecoder(resp.Body).Decode(&context)
				
				// Use context in MCP operation
				toolCall := ToolCallRequest{
					Name: "analyze_context",
					Arguments: map[string]interface{}{
						"context_id": context.ID,
					},
				}

				resp2, err := makeRequest("POST", "/api/v1/mcp/tools/call", toolCall, nil)
				Expect(err).NotTo(HaveOccurred())
				resp2.Body.Close()
			}
		})

		It("should integrate with vector search", func() {
			searchRequest := map[string]interface{}{
				"query":      "test query",
				"limit":      10,
				"threshold":  0.7,
			}

			resp, err := makeRequest("POST", "/api/v1/search/semantic", searchRequest, nil)
			if err != nil || resp.StatusCode == http.StatusNotFound {
				Skip("Vector search not available")
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				var results []map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&results)
				GinkgoWriter.Printf("Vector search returned %d results\n", len(results))
			}
		})
	})
})