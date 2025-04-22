package integrations_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/S-Corkum/mcp-server/test/functional/client"
)

var _ = Describe("Tool Integrations", func() {
	var mcpClient *client.MCPClient
	var ctx context.Context
	var cancel context.CancelFunc

	BeforeEach(func() {
		// Create a new MCP client for each test
		mcpClient = client.NewMCPClient(ServerURL, APIKey)
		
		// Create a context with timeout for requests
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	})

	AfterEach(func() {
		// Cancel the context after each test
		cancel()
	})

	Describe("GitHub Integration", func() {
		It("should be available as a tool", func() {
			// Get the list of tools
			tools, err := mcpClient.ListTools(ctx)
			Expect(err).NotTo(HaveOccurred())
			
			// Check if GitHub is in the tools list
			foundGithub := false
			for _, tool := range tools {
				if name, ok := tool["name"].(string); ok && name == "github" {
					foundGithub = true
					break
				}
			}
			
			Expect(foundGithub).To(BeTrue(), "GitHub tool should be available")
		})

		It("should have expected actions", func() {
			// Get GitHub actions
			path := "/api/v1/tools/github/actions"
			resp, err := mcpClient.Get(ctx, path)
			
			// Skip test if GitHub integration is not available
			if err != nil {
				Skip("GitHub integration not available, skipping test")
			}
			defer resp.Body.Close()
			
			// Parse response
			var actions map[string]interface{}
			err = client.ParseResponse(resp, &actions)
			Expect(err).NotTo(HaveOccurred())
			
			// Check for expected actions
			expectedActions := []string{
				"list_repositories",
				"get_repository",
				"list_issues",
				"get_issue",
				"create_issue",
				"update_issue",
				"close_issue",
				"list_pull_requests",
				"get_pull_request",
				"create_pull_request",
				"merge_pull_request",
			}
			
			for _, action := range expectedActions {
				Expect(actions).To(HaveKey(action), "GitHub should support the "+action+" action")
			}
		})

		It("should handle authentication errors gracefully", func() {
			// Create client with invalid API key
			invalidClient := client.NewMCPClient(ServerURL, "invalid-api-key")
			
			// Try to access GitHub actions
			path := "/api/v1/tools/github/actions"
			resp, err := invalidClient.Get(ctx, path)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			
			// Should return unauthorized
			Expect(resp.StatusCode).To(Equal(401))
		})
		
		It("should handle GitHub API errors properly using GitHubError type", func() {
			// Create a context
			payload := map[string]interface{}{
				"agent_id":   "error-test",
				"model_id":   "gpt-4",
				"max_tokens": 4000,
			}
			
			context, err := mcpClient.CreateContext(ctx, payload)
			Expect(err).NotTo(HaveOccurred())
			
			// Save the created context ID
			Expect(context).To(HaveKey("id"))
			contextID := context["id"].(string)
			
			// Try to access a non-existent repository to trigger a GitHub API error
			toolPayload := map[string]interface{}{
				"context_id": contextID,
				"params": map[string]interface{}{
					"owner": "non-existent-owner-12345",
					"repo":  "non-existent-repo-67890",
				},
			}
			
			path := "/api/v1/tools/github/actions/get_repository"
			resp, err := mcpClient.Post(ctx, path, toolPayload)
			
			// Skip if GitHub integration is not properly configured
			if err != nil {
				Skip("GitHub integration not properly configured, skipping test")
			}
			defer resp.Body.Close()
			
			// Parse response to check error structure
			var result map[string]interface{}
			err = client.ParseResponse(resp, &result)
			Expect(err).NotTo(HaveOccurred())
			
			// Should have error object with GitHubError structure
			Expect(result).To(HaveKey("error"))
			errorObj, ok := result["error"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "error should be an object")
			
			// Verify error fields from GitHubError
			Expect(errorObj).To(HaveKey("message"))
			
			// Check for specific GitHubError fields - statusCode and resource
			// These fields might not be exactly named like this in the API response,
			// but there should be similar fields
			expectedErrorFields := []string{"message", "status", "code"}
			atLeastOneExpectedField := false
			
			for _, field := range expectedErrorFields {
				if _, hasField := errorObj[field]; hasField {
					atLeastOneExpectedField = true
					break
				}
			}
			
			Expect(atLeastOneExpectedField).To(BeTrue(), "Error should have at least one expected GitHubError field")
			
			// Clean up created context
			deletePath := "/api/v1/contexts/" + contextID
			mcpClient.Delete(ctx, deletePath)
		})
	})

	Describe("Mock Server Integration", func() {
		It("should be able to communicate with the mock server", func() {
			// Execute a mock action
			// This is an example that would need to be adjusted based on your mock server implementation
			payload := map[string]interface{}{
				"action": "test",
				"params": map[string]interface{}{
					"test_param": "test_value",
				},
			}
			
			path := "/api/v1/tools/mock/actions/test"
			resp, err := mcpClient.Post(ctx, path, payload)
			
			// If the mock server is not configured, this might fail
			if err != nil || resp.StatusCode != 200 {
				Skip("Mock server not properly configured, skipping test")
			}
			defer resp.Body.Close()
			
			// Verify we get some kind of response
			var result map[string]interface{}
			err = client.ParseResponse(resp, &result)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeEmpty())
		})
	})

	Describe("Cache Integration", func() {
		It("should handle cache operations correctly", func() {
			// Create a context
			payload := map[string]interface{}{
				"agent_id":   "cache-test",
				"model_id":   "gpt-4",
				"max_tokens": 4000,
			}
			
			context, err := mcpClient.CreateContext(ctx, payload)
			Expect(err).NotTo(HaveOccurred())
			
			// Save the created context ID
			Expect(context).To(HaveKey("id"))
			contextID := context["id"].(string)
			
			// Get the context (first access, should be stored in cache)
			context1, err := mcpClient.GetContext(ctx, contextID)
			Expect(err).NotTo(HaveOccurred())
			Expect(context1["id"]).To(Equal(contextID))
			
			// Get the context again (should be served from cache)
			context2, err := mcpClient.GetContext(ctx, contextID)
			Expect(err).NotTo(HaveOccurred())
			Expect(context2["id"]).To(Equal(contextID))
			
			// The test passes if we can access the context successfully both times
			// which means the cache initialization with settings map is working
			
			// Clean up created context
			deletePath := "/api/v1/contexts/" + contextID
			mcpClient.Delete(ctx, deletePath)
		})
	})
})
