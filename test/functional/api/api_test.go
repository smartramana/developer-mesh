package api_test

import (
	"context"
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
		It("should retrieve an existing context", func() {
			Skip("Context API endpoints are no longer supported")
			
			// Try to get the test context
			context, err := mcpClient.GetContext(ctx, "ctx-test-001")
			
			// We might get a 404 if the context doesn't exist, which is OK for this test
			if err != nil {
				Skip("Test context not found, skipping test")
			}
			
			// Verify context data
			Expect(context).To(HaveKey("id"))
			Expect(context["id"]).To(Equal("ctx-test-001"))
		})

		It("should create a new context", func() {
			Skip("Context API endpoints are no longer supported")
			
			// Create a new context
			payload := map[string]interface{}{
				"agent_id": "test-agent",
				"model_id": "gpt-4",
				"max_tokens": 4000,
			}
			
			context, err := mcpClient.CreateContext(ctx, payload)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify context data
			Expect(context).To(HaveKey("id"))
			Expect(context).To(HaveKey("agent_id"))
			Expect(context["agent_id"]).To(Equal("test-agent"))
			Expect(context).To(HaveKey("model_id"))
			Expect(context["model_id"]).To(Equal("gpt-4"))
		})
	})
})
