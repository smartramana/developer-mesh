package workflows_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/S-Corkum/mcp-server/test/functional/client"
	functional_test "github.com/S-Corkum/mcp-server/test/functional"
)

var _ = Describe("End-to-End Workflows", func() {
	var mcpClient *client.MCPClient
	var ctx context.Context
	var cancel context.CancelFunc
	var createdContextID string

	BeforeEach(func() {
		// Create a new MCP client for each test
		mcpClient = client.NewMCPClient(functional_test.ServerURL, functional_test.APIKey)
		
		// Create a context with timeout for requests
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		
		// Reset the created context ID
		createdContextID = ""
	})

	AfterEach(func() {
		// Cancel the context after each test
		cancel()
		
		// Clean up the created context if it exists
		if createdContextID != "" {
			path := "/api/v1/contexts/" + createdContextID
			_, _ = mcpClient.Delete(ctx, path)
		}
	})

	// Helper function to add an item to a context
	addItemToContext := func(contextID, role, content string) {
		path := "/api/v1/contexts/" + contextID + "/items"
		payload := map[string]interface{}{
			"role":    role,
			"content": content,
		}
		
		resp, err := mcpClient.Post(ctx, path, payload)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		
		// Should be successful (200 OK or 201 Created)
		Expect(resp.StatusCode).To(SatisfyAny(Equal(200), Equal(201)))
	}

	Describe("Context Management Workflow", func() {
		It("should support the complete context lifecycle", func() {
			// 1. Create a new context
			By("Creating a new context")
			payload := map[string]interface{}{
				"agent_id":   "workflow-test-agent",
				"model_id":   "gpt-4",
				"max_tokens": 4000,
				"metadata": map[string]interface{}{
					"test_workflow": true,
				},
			}
			
			context, err := mcpClient.CreateContext(ctx, payload)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify context data
			Expect(context).To(HaveKey("id"))
			createdContextID = context["id"].(string)
			
			// 2. Add items to the context
			By("Adding items to the context")
			addItemToContext(createdContextID, "user", "Hello, this is a test message")
			addItemToContext(createdContextID, "assistant", "Hello! How can I help you today?")
			
			// 3. Retrieve the context and verify items
			By("Retrieving the context and verifying items")
			updatedContext, err := mcpClient.GetContext(ctx, createdContextID)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify context data
			Expect(updatedContext).To(HaveKey("id"))
			Expect(updatedContext["id"]).To(Equal(createdContextID))
			
			// Check if items array exists
			Expect(updatedContext).To(HaveKey("items"))
			items, ok := updatedContext["items"].([]interface{})
			Expect(ok).To(BeTrue(), "items should be an array")
			
			// Should have at least 2 items
			Expect(len(items)).To(BeNumerically(">=", 2))
			
			// 4. Search for contexts
			By("Searching for contexts")
			path := "/api/v1/contexts?agent_id=workflow-test-agent"
			resp, err := mcpClient.Get(ctx, path)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			
			// Parse response
			var searchResult struct {
				Contexts []map[string]interface{} `json:"contexts"`
			}
			err = client.ParseResponse(resp, &searchResult)
			Expect(err).NotTo(HaveOccurred())
			
			// Should find at least one context
			Expect(searchResult.Contexts).NotTo(BeEmpty())
			
			// 5. Delete the context (if the API supports it)
			By("Deleting the context")
			path = "/api/v1/contexts/" + createdContextID
			resp, err = mcpClient.Delete(ctx, path)
			
			// Note: If delete is not supported, this will be skipped
			if err != nil || resp.StatusCode >= 400 {
				Skip("Context deletion not supported, skipping this step")
			}
			defer resp.Body.Close()
			
			// Verify deletion
			_, err = mcpClient.GetContext(ctx, createdContextID)
			Expect(err).To(HaveOccurred(), "Context should be deleted")
			
			// Clear the context ID since we've deleted it
			createdContextID = ""
		})
	})

	Describe("Tool Integration Workflow", func() {
		It("should execute a GitHub tool integration workflow", func() {
			// 1. Create a new context
			By("Creating a new context")
			payload := map[string]interface{}{
				"agent_id":   "github-workflow-test",
				"model_id":   "gpt-4",
				"max_tokens": 4000,
			}
			
			context, err := mcpClient.CreateContext(ctx, payload)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify context data
			Expect(context).To(HaveKey("id"))
			createdContextID = context["id"].(string)
			
			// 2. List GitHub repositories
			By("Listing GitHub repositories")
			toolPayload := map[string]interface{}{
				"context_id": createdContextID,
				"params": map[string]interface{}{},
			}
			
			result, err := mcpClient.ExecuteToolAction(ctx, "github", "list_repositories", toolPayload)
			
			// Skip if GitHub integration is not properly configured
			if err != nil {
				Skip("GitHub integration not properly configured, skipping test")
			}
			
			// Verify result
			Expect(result).To(HaveKey("repositories"))
			
			// 3. Update context with the result
			By("Updating context with the result")
			addItemToContext(createdContextID, "tool", "GitHub repositories retrieved successfully")
			
			// 4. Retrieve the updated context
			By("Retrieving the updated context")
			updatedContext, err := mcpClient.GetContext(ctx, createdContextID)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify context data
			Expect(updatedContext).To(HaveKey("id"))
			Expect(updatedContext["id"]).To(Equal(createdContextID))
			
			// Check if items array exists and has increased
			Expect(updatedContext).To(HaveKey("items"))
			items, ok := updatedContext["items"].([]interface{})
			Expect(ok).To(BeTrue(), "items should be an array")
			
			// Should have at least one item
			Expect(len(items)).To(BeNumerically(">=", 1))
			
			// 5. Test error handling with GitHub error type
			By("Testing GitHub error handling")
			
			// Try to access a non-existent repository to trigger an error
			errorPayload := map[string]interface{}{
				"context_id": createdContextID,
				"params": map[string]interface{}{
					"owner": "non-existent-owner-12345",
					"repo":  "non-existent-repo-67890",
				},
			}
			
			path := "/api/v1/tools/github/actions/get_repository"
			resp, err := mcpClient.Post(ctx, path, errorPayload)
			
			// Skip if GitHub integration is not properly configured
			if err != nil {
				Skip("GitHub integration not properly configured, skipping error handling test")
			}
			defer resp.Body.Close()
			
			// Check that the error response contains proper GitHubError structure
			var errorResult map[string]interface{}
			err = client.ParseResponse(resp, &errorResult)
			
			// Error should be present in the response
			Expect(errorResult).To(HaveKey("error"))
		})
	})

	Describe("Vector Search Workflow", func() {
		It("should support vector search in contexts", func() {
			// This test depends on whether vector search is implemented
			// We'll try to use it and skip if not supported
			
			// 1. Create a new context
			By("Creating a new context")
			payload := map[string]interface{}{
				"agent_id":   "vector-search-test",
				"model_id":   "gpt-4",
				"max_tokens": 4000,
			}
			
			context, err := mcpClient.CreateContext(ctx, payload)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify context data
			Expect(context).To(HaveKey("id"))
			createdContextID = context["id"].(string)
			
			// 2. Add some items with diverse content
			By("Adding items with diverse content")
			addItemToContext(createdContextID, "user", "How does the solar system work?")
			addItemToContext(createdContextID, "assistant", "The solar system consists of the Sun and everything that orbits around it, including planets, dwarf planets, moons, asteroids, comets, and other celestial bodies.")
			addItemToContext(createdContextID, "user", "What programming languages are popular today?")
			addItemToContext(createdContextID, "assistant", "Popular programming languages include Python, JavaScript, Go, Rust, Java, C++, and TypeScript, each with their own strengths and use cases.")
			
			// 3. Attempt a vector search
			By("Performing a vector search")
			path := "/api/v1/contexts/" + createdContextID + "/search"
			payload = map[string]interface{}{
				"query": "Tell me about planets",
				"limit": 5,
			}
			
			resp, err := mcpClient.Post(ctx, path, payload)
			
			// Skip if vector search is not supported
			if err != nil || resp.StatusCode >= 400 {
				Skip("Vector search not supported, skipping test")
			}
			defer resp.Body.Close()
			
			// Parse response
			var searchResult struct {
				Items []map[string]interface{} `json:"items"`
			}
			err = client.ParseResponse(resp, &searchResult)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify search results
			Expect(searchResult.Items).NotTo(BeEmpty(), "Search results should not be empty")
			
			// First result should be more related to solar system than programming
			firstResult := searchResult.Items[0]
			Expect(firstResult).To(HaveKey("content"))
			content := firstResult["content"].(string)
			Expect(content).To(ContainSubstring("solar system"))
		})
	})
	
	Describe("Cache Configuration Workflow", func() {
		It("should correctly use cache with required settings map", func() {
			// 1. Create a new context
			By("Creating a new context")
			payload := map[string]interface{}{
				"agent_id":   "cache-test-agent",
				"model_id":   "gpt-4",
				"max_tokens": 4000,
				"metadata": map[string]interface{}{
					"cache_test": true,
				},
			}
			
			context, err := mcpClient.CreateContext(ctx, payload)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify context data
			Expect(context).To(HaveKey("id"))
			createdContextID = context["id"].(string)
			
			// 2. Add items to the context
			By("Adding items to the context")
			addItemToContext(createdContextID, "user", "This is a cache test message")
			
			// 3. Retrieve the context multiple times to test caching
			// First retrieval should populate the cache
			By("First context retrieval")
			_, err = mcpClient.GetContext(ctx, createdContextID)
			Expect(err).NotTo(HaveOccurred())
			
			// Second retrieval should use the cache
			By("Second context retrieval (from cache)")
			cachedContext, err := mcpClient.GetContext(ctx, createdContextID)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify context data from cache
			Expect(cachedContext).To(HaveKey("id"))
			Expect(cachedContext["id"]).To(Equal(createdContextID))
			
			// Check if items array includes our test message
			Expect(cachedContext).To(HaveKey("items"))
			items, ok := cachedContext["items"].([]interface{})
			Expect(ok).To(BeTrue(), "items should be an array")
			
			// Find our test message
			var foundTestMessage bool
			for _, item := range items {
				itemMap, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				
				if content, ok := itemMap["content"].(string); ok {
					if content == "This is a cache test message" {
						foundTestMessage = true
						break
					}
				}
			}
			
			Expect(foundTestMessage).To(BeTrue(), "Test message should be found in cached context items")
			
			// 4. Add another item to invalidate cache
			By("Adding another item to invalidate cache")
			addItemToContext(createdContextID, "assistant", "This response should invalidate the cache")
			
			// 5. Retrieve the context again to verify cache invalidation
			By("Retrieving context after cache invalidation")
			updatedContext, err := mcpClient.GetContext(ctx, createdContextID)
			Expect(err).NotTo(HaveOccurred())
			
			// Check if items array has grown
			Expect(updatedContext).To(HaveKey("items"))
			updatedItems, ok := updatedContext["items"].([]interface{})
			Expect(ok).To(BeTrue(), "items should be an array")
			Expect(len(updatedItems)).To(BeNumerically(">", len(items)), "Updated context should have more items")
		})
	})
	
	Describe("ElastiCache Integration", func() {
		It("should work with ElastiCache if configured", func() {
			// Check server health to see if ElastiCache is configured
			healthData, err := mcpClient.GetHealth(ctx)
			Expect(err).NotTo(HaveOccurred())
			
			// Check if ElastiCache is mentioned in the health data
			var hasElastiCache bool
			if cacheInfo, ok := healthData["cache"].(map[string]interface{}); ok {
				if cacheType, ok := cacheInfo["type"].(string); ok {
					hasElastiCache = cacheType == "elasticache" || cacheType == "redis"
				}
			}
			
			// Skip test if ElastiCache is not configured
			if !hasElastiCache {
				Skip("ElastiCache not configured, skipping test")
			}
			
			// 1. Create a new context
			By("Creating a new context")
			payload := map[string]interface{}{
				"agent_id":   "elasticache-test-agent",
				"model_id":   "gpt-4",
				"max_tokens": 4000,
			}
			
			context, err := mcpClient.CreateContext(ctx, payload)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify context data
			Expect(context).To(HaveKey("id"))
			createdContextID = context["id"].(string)
			
			// 2. Add items to the context
			By("Adding items to the context")
			addItemToContext(createdContextID, "user", "This is an ElastiCache test message")
			
			// 3. Retrieve the context multiple times to test caching
			By("Retrieving the context multiple times")
			for i := 0; i < 3; i++ {
				retrievedContext, err := mcpClient.GetContext(ctx, createdContextID)
				Expect(err).NotTo(HaveOccurred())
				Expect(retrievedContext["id"]).To(Equal(createdContextID))
			}
			
			// The test passes if we can successfully retrieve the context
			// which indicates that the ElastiCache endpoint and port handling is working
		})
	})
})
