package api_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/S-Corkum/mcp-server/test/functional/client"
)

// Import variables from the suite
var (
	ServerURL string
	APIKey string
	MockServerURL string
)

func init() {
	// These will be set by the suite before tests run
	ServerURL = "http://localhost:8080"
	APIKey = "test-admin-api-key"
	MockServerURL = "http://localhost:8081"
}

var _ = Describe("API", func() {
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

	Describe("Health Endpoint", func() {
		It("should return 200 OK with healthy status", func() {
			// Call the health endpoint
			resp, err := mcpClient.Get(ctx, "/health")
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			// Verify response status
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			// Parse response
			var healthResponse struct {
				Status     string            `json:"status"`
				Components map[string]string `json:"components"`
			}
			
			err = client.ParseResponse(resp, &healthResponse)
			Expect(err).NotTo(HaveOccurred())

			// Verify health status
			Expect(healthResponse.Status).To(Equal("healthy"))
			Expect(healthResponse.Components).NotTo(BeEmpty())
		})
	})

	Describe("API Versioning", func() {
		It("should support API versioning", func() {
			// Call the root API endpoint
			resp, err := mcpClient.Get(ctx, "/api/v1")
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			// Verify response status
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			// Parse response
			var rootResponse struct {
				APIVersion string `json:"api_version"`
			}
			
			err = client.ParseResponse(resp, &rootResponse)
			Expect(err).NotTo(HaveOccurred())

			// Verify API version
			Expect(rootResponse.APIVersion).To(Equal("1.0"))
		})
	})

	Describe("Authentication", func() {
		It("should require authentication for protected endpoints", func() {
			// Create client without API key
			unauthClient := client.NewMCPClient(ServerURL, "")
			
			// Call the tools endpoint without authentication
			resp, err := unauthClient.Get(ctx, "/api/v1/tools")
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			// Verify response status (should be unauthorized)
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("should accept valid API key", func() {
			// Call the tools endpoint with authentication
			resp, err := mcpClient.Get(ctx, "/api/v1/tools")
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			// Verify response status (should be OK)
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("Tools API", func() {
		It("should list available tools", func() {
			// Get the list of tools
			tools, err := mcpClient.ListTools(ctx)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify that tools are returned
			Expect(tools).NotTo(BeEmpty())
			
			// Verify that each tool has the required fields
			for _, tool := range tools {
				Expect(tool).To(HaveKey("name"))
				Expect(tool).To(HaveKey("description"))
			}
		})

		It("should get tool actions", func() {
			// First get the list of tools
			tools, err := mcpClient.ListTools(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(tools).NotTo(BeEmpty())
			
			// Get the first tool
			toolName := tools[0]["name"].(string)
			
			// Get the actions for this tool
			path := "/api/v1/tools/" + toolName + "/actions"
			resp, err := mcpClient.Get(ctx, path)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			
			// Verify response status
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			
			// Parse response
			var actions map[string]interface{}
			err = client.ParseResponse(resp, &actions)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify that actions are returned
			Expect(actions).NotTo(BeEmpty())
		})
	})

	Describe("Context API", func() {
		var createdContextID string

		It("should create a new context", func() {
			// Create a new context
			payload := map[string]interface{}{
				"agent_id": "test-agent",
				"model_id": "gpt-4",
				"max_tokens": 4000,
				"content": []map[string]interface{}{
					{
						"role": "system",
						"content": "You are a DevOps assistant.",
						"tokens": 6,
					},
				},
			}
			
			context, err := mcpClient.CreateContext(ctx, payload)
			Expect(err).NotTo(HaveOccurred())
			
			// Store the created context ID for later tests
			createdContextID = context["id"].(string)
			
			// Verify context data
			Expect(context).To(HaveKey("id"))
			Expect(context).To(HaveKey("agent_id"))
			Expect(context["agent_id"]).To(Equal("test-agent"))
			Expect(context).To(HaveKey("model_id"))
			Expect(context["model_id"]).To(Equal("gpt-4"))
			Expect(context).To(HaveKey("current_tokens"))
			Expect(context["current_tokens"]).To(BeNumerically("==", 6))
		})

		It("should retrieve an existing context", func() {
			// Skip if no context was created in the previous test
			if createdContextID == "" {
				Skip("No context was created in the previous test")
			}
			
			// Get the context
			context, err := mcpClient.GetContext(ctx, createdContextID)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify context data
			Expect(context).To(HaveKey("id"))
			Expect(context["id"]).To(Equal(createdContextID))
			Expect(context).To(HaveKey("agent_id"))
			Expect(context["agent_id"]).To(Equal("test-agent"))
		})

		It("should update an existing context", func() {
			// Skip if no context was created in the previous test
			if createdContextID == "" {
				Skip("No context was created in the previous test")
			}
			
			// Update the context
			updatePayload := map[string]interface{}{
				"context": map[string]interface{}{
					"content": []map[string]interface{}{
						{
							"role": "user",
							"content": "Show me the open pull requests.",
							"tokens": 6,
						},
					},
				},
				"options": map[string]interface{}{
					"truncate": true,
					"truncate_strategy": "oldest_first",
				},
			}
			
			path := fmt.Sprintf("/api/v1/contexts/%s", createdContextID)
			resp, err := mcpClient.Put(ctx, path, updatePayload)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			
			// Verify response status
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			
			// Parse response
			var updatedContext map[string]interface{}
			err = client.ParseResponse(resp, &updatedContext)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify updated context
			Expect(updatedContext).To(HaveKey("current_tokens"))
			Expect(updatedContext["current_tokens"]).To(BeNumerically("==", 12)) // 6 + 6 tokens
		})

		It("should search within a context", func() {
			// Skip if no context was created in the previous test
			if createdContextID == "" {
				Skip("No context was created in the previous test")
			}
			
			// Search in the context
			searchPayload := map[string]interface{}{
				"query": "pull request",
			}
			
			path := fmt.Sprintf("/api/v1/contexts/%s/search", createdContextID)
			resp, err := mcpClient.Post(ctx, path, searchPayload)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			
			// Verify response status
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			
			// Parse response
			var searchResults map[string]interface{}
			err = client.ParseResponse(resp, &searchResults)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify search results
			Expect(searchResults).To(HaveKey("results"))
			results := searchResults["results"].([]interface{})
			Expect(len(results)).To(BeNumerically(">", 0))
		})

		It("should get a context summary", func() {
			// Skip if no context was created in the previous test
			if createdContextID == "" {
				Skip("No context was created in the previous test")
			}
			
			// Get context summary
			path := fmt.Sprintf("/api/v1/contexts/%s/summary", createdContextID)
			resp, err := mcpClient.Get(ctx, path)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			
			// Verify response status
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			
			// Parse response
			var summary map[string]interface{}
			err = client.ParseResponse(resp, &summary)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify summary
			Expect(summary).To(HaveKey("summary"))
			Expect(summary["context_id"]).To(Equal(createdContextID))
		})

		It("should delete a context", func() {
			// Skip if no context was created in the previous test
			if createdContextID == "" {
				Skip("No context was created in the previous test")
			}
			
			// Delete the context
			path := fmt.Sprintf("/api/v1/contexts/%s", createdContextID)
			resp, err := mcpClient.Delete(ctx, path)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			
			// Verify response status
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			
			// Verify the context is deleted
			resp2, err := mcpClient.Get(ctx, path)
			Expect(err).NotTo(HaveOccurred())
			defer resp2.Body.Close()
			
			// Should return 404 Not Found
			Expect(resp2.StatusCode).To(Equal(http.StatusNotFound))
		})
	})
})
