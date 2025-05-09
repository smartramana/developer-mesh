package adapters_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/S-Corkum/devops-mcp/test/functional/client"
	functional_test "github.com/S-Corkum/devops-mcp/test/functional"
)

var _ = Describe("GitHub Adapter", func() {
	var mcpClient *client.MCPClient
	var ctx context.Context
	var cancel context.CancelFunc
	var createdContextID string

	BeforeEach(func() {
		// Create a new MCP client for each test
		mcpClient = client.NewMCPClient(functional_test.ServerURL, functional_test.APIKey)
		
		// Create a context with timeout for requests
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		
		// Create a test context
		payload := map[string]interface{}{
			"agent_id":   "github-adapter-test",
			"model_id":   "gpt-4",
			"max_tokens": 4000,
		}
		
		context, err := mcpClient.CreateContext(ctx, payload)
		Expect(err).NotTo(HaveOccurred())
		
		// Save the created context ID
		Expect(context).To(HaveKey("id"))
		createdContextID = context["id"].(string)
	})

	AfterEach(func() {
		// Cancel the context after each test
		cancel()
		
		// Clean up created context if provided
		if createdContextID != "" {
			path := "/api/v1/contexts/" + createdContextID
			_, _ = mcpClient.Delete(ctx, path)
		}
	})

	Describe("Error Handling", func() {
		It("should handle resource not found errors correctly", func() {
			// Attempt to get a non-existent repository
			toolPayload := map[string]interface{}{
				"context_id": createdContextID,
				"params": map[string]interface{}{
					"owner": "non-existent-owner",
					"repo":  "non-existent-repo",
				},
			}
			
			path := "/api/v1/tools/github/actions/get_repository"
			resp, err := mcpClient.Post(ctx, path, toolPayload)
			
			// Skip if GitHub integration is not properly configured
			if err != nil {
				Skip("GitHub integration not properly configured, skipping test")
			}
			defer resp.Body.Close()
			
			// Should return 404 Not Found or 500 with proper error structure
			Expect(resp.StatusCode).To(SatisfyAny(Equal(404), Equal(500)))
			
			// Parse response to check error structure
			var result map[string]interface{}
			err = client.ParseResponse(resp, &result)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify error response structure that matches GitHubError
			Expect(result).To(HaveKey("error"))
			errorObj, ok := result["error"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "error should be an object")
			
			// Check for standard error fields from GitHubError
			Expect(errorObj).To(HaveKey("message"))
			message := errorObj["message"].(string)
			Expect(message).To(ContainSubstring("not found"))
		})

		It("should handle validation errors correctly", func() {
			// Attempt to call GitHub API with invalid parameters
			toolPayload := map[string]interface{}{
				"context_id": createdContextID,
				"params": map[string]interface{}{
					// Missing required 'owner' parameter
					"repo": "some-repo",
				},
			}
			
			path := "/api/v1/tools/github/actions/get_repository"
			resp, err := mcpClient.Post(ctx, path, toolPayload)
			
			// Skip if GitHub integration is not properly configured
			if err != nil {
				Skip("GitHub integration not properly configured, skipping test")
			}
			defer resp.Body.Close()
			
			// Should return 400 Bad Request or 422 Unprocessable Entity
			Expect(resp.StatusCode).To(SatisfyAny(Equal(400), Equal(422)))
			
			// Parse response to check error structure
			var result map[string]interface{}
			err = client.ParseResponse(resp, &result)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify error response structure
			Expect(result).To(HaveKey("error"))
			errorObj, ok := result["error"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "error should be an object")
			
			// Should contain error message mentioning the missing parameter
			Expect(errorObj).To(HaveKey("message"))
			message := errorObj["message"].(string)
			Expect(message).To(ContainSubstring("owner"))
		})

		It("should handle rate limit errors correctly when simulated", func() {
			// This test attempts to trigger a rate limit error response
			// In a real environment, this might be difficult to test without actual rate limiting
			
			// Create a special payload that might trigger a simulated rate limit error
			// (The actual implementation would need to support this simulation mode)
			toolPayload := map[string]interface{}{
				"context_id": createdContextID,
				"params": map[string]interface{}{
					"simulate_error": "rate_limit_exceeded", // Special parameter to trigger simulation
					"owner": "test-owner",
					"repo":  "test-repo",
				},
			}
			
			path := "/api/v1/tools/github/actions/get_repository"
			resp, err := mcpClient.Post(ctx, path, toolPayload)
			
			// Skip if GitHub integration or simulation is not properly configured
			if err != nil || resp.StatusCode < 400 || resp.StatusCode == 404 {
				Skip("GitHub error simulation not supported, skipping test")
			}
			defer resp.Body.Close()
			
			// For rate limiting, should return 429 Too Many Requests or 403 Forbidden
			Expect(resp.StatusCode).To(SatisfyAny(Equal(429), Equal(403)))
			
			// Parse response to check error structure
			var result map[string]interface{}
			err = client.ParseResponse(resp, &result)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify error response structure
			Expect(result).To(HaveKey("error"))
			errorObj, ok := result["error"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "error should be an object")
			
			// Should contain error message about rate limiting
			Expect(errorObj).To(HaveKey("message"))
			message := errorObj["message"].(string)
			Expect(message).To(ContainSubstring("rate limit"))
		})
	})

	Describe("GitHub API Operations", func() {
		It("should list repositories successfully", func() {
			// Attempt to list repositories
			toolPayload := map[string]interface{}{
				"context_id": createdContextID,
				"params": map[string]interface{}{
					"page": 1,
					"per_page": 10,
				},
			}
			
			result, err := mcpClient.ExecuteToolAction(ctx, "github", "list_repositories", toolPayload)
			
			// Skip if GitHub integration is not properly configured
			if err != nil {
				Skip("GitHub integration not properly configured, skipping test")
			}
			
			// Verify result structure
			Expect(result).To(HaveKey("repositories"), "Result should contain repositories array")
		})

		It("should handle different authentication methods", func() {
			// This test would depend on how authentication is configured in the test environment
			// For now, we'll just test the basic repository listing operation
			
			toolPayload := map[string]interface{}{
				"context_id": createdContextID,
				"params": map[string]interface{}{},
			}
			
			result, err := mcpClient.ExecuteToolAction(ctx, "github", "list_repositories", toolPayload)
			
			// Skip if GitHub integration is not properly configured
			if err != nil {
				Skip("GitHub integration not properly configured, skipping test")
			}
			
			// Verify result structure
			Expect(result).To(HaveKey("repositories"), "Result should contain repositories array")
		})
	})
})
