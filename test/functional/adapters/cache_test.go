package adapters_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/S-Corkum/mcp-server/test/functional/client"
	functional_test "github.com/S-Corkum/mcp-server/test/functional"
)

var _ = Describe("Cache Integration", func() {
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
			"agent_id":   "cache-test",
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

	Describe("Cache Operations", func() {
		It("should cache and retrieve context data correctly", func() {
			// First request to create/cache the context
			context1, err := mcpClient.GetContext(ctx, createdContextID)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify first response data
			Expect(context1).To(HaveKey("id"))
			Expect(context1["id"]).To(Equal(createdContextID))
			
			// Add an item to the context
			path := "/api/v1/contexts/" + createdContextID + "/items"
			payload := map[string]interface{}{
				"role":    "user",
				"content": "This is a cache test message",
			}
			
			resp, err := mcpClient.Post(ctx, path, payload)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			
			// Should be successful (200 OK or 201 Created)
			Expect(resp.StatusCode).To(SatisfyAny(Equal(200), Equal(201)))
			
			// Second request should get updated data
			context2, err := mcpClient.GetContext(ctx, createdContextID)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify second response data
			Expect(context2).To(HaveKey("id"))
			Expect(context2["id"]).To(Equal(createdContextID))
			
			// Check if items array exists and has increased
			Expect(context2).To(HaveKey("items"))
			items, ok := context2["items"].([]interface{})
			Expect(ok).To(BeTrue(), "items should be an array")
			
			// Should have at least one item
			Expect(len(items)).To(BeNumerically(">=", 1))
			
			// Verify the added item is present
			found := false
			for _, item := range items {
				itemMap, ok := item.(map[string]interface{})
				Expect(ok).To(BeTrue(), "item should be an object")
				
				if content, hasContent := itemMap["content"].(string); hasContent {
					if content == "This is a cache test message" {
						found = true
						break
					}
				}
			}
			
			Expect(found).To(BeTrue(), "Added item should be found in the context")
		})

		It("should handle cache invalidation correctly", func() {
			// Get the initial context
			context1, err := mcpClient.GetContext(ctx, createdContextID)
			Expect(err).NotTo(HaveOccurred())
			
			// Get initial item count
			var initialItemCount int
			items1, ok := context1["items"].([]interface{})
			if ok {
				initialItemCount = len(items1)
			} else {
				initialItemCount = 0
			}
			
			// Add item to context
			path := "/api/v1/contexts/" + createdContextID + "/items"
			payload := map[string]interface{}{
				"role":    "user",
				"content": "This is a cache invalidation test message",
			}
			
			resp, err := mcpClient.Post(ctx, path, payload)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			
			// Should be successful
			Expect(resp.StatusCode).To(SatisfyAny(Equal(200), Equal(201)))
			
			// Get the updated context
			context2, err := mcpClient.GetContext(ctx, createdContextID)
			Expect(err).NotTo(HaveOccurred())
			
			// Get updated item count
			items2, ok := context2["items"].([]interface{})
			Expect(ok).To(BeTrue(), "items should be an array")
			updatedItemCount := len(items2)
			
			// Should have one more item than before
			Expect(updatedItemCount).To(Equal(initialItemCount + 1))
			
			// Verify the latest item is the one we added
			latestItem := items2[updatedItemCount-1]
			latestItemMap, ok := latestItem.(map[string]interface{})
			Expect(ok).To(BeTrue(), "latest item should be an object")
			
			Expect(latestItemMap).To(HaveKey("content"))
			content, ok := latestItemMap["content"].(string)
			Expect(ok).To(BeTrue(), "content should be a string")
			Expect(content).To(Equal("This is a cache invalidation test message"))
		})
	})

	Describe("Cache Configuration", func() {
		It("should maintain cache consistency under load", func() {
			// This test simulates multiple concurrent operations
			// to verify that the cache remains consistent
			
			// Number of operations to perform
			const numOperations = 5
			
			// Track original item count
			context1, err := mcpClient.GetContext(ctx, createdContextID)
			Expect(err).NotTo(HaveOccurred())
			
			var initialItemCount int
			items1, ok := context1["items"].([]interface{})
			if ok {
				initialItemCount = len(items1)
			} else {
				initialItemCount = 0
			}
			
			// Perform multiple add operations
			for i := 0; i < numOperations; i++ {
				path := "/api/v1/contexts/" + createdContextID + "/items"
				payload := map[string]interface{}{
					"role":    "user",
					"content": "Concurrent operation test message " + fmt.Sprint(i+1),
				}
				
				resp, err := mcpClient.Post(ctx, path, payload)
				Expect(err).NotTo(HaveOccurred())
				resp.Body.Close()
				
				// Should be successful
				Expect(resp.StatusCode).To(SatisfyAny(Equal(200), Equal(201)))
			}
			
			// Get the final context state
			contextFinal, err := mcpClient.GetContext(ctx, createdContextID)
			Expect(err).NotTo(HaveOccurred())
			
			// Get final item count
			itemsFinal, ok := contextFinal["items"].([]interface{})
			Expect(ok).To(BeTrue(), "items should be an array")
			finalItemCount := len(itemsFinal)
			
			// Should have all the added items
			Expect(finalItemCount).To(Equal(initialItemCount + numOperations))
		})
	})
})
