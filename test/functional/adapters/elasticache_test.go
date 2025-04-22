package adapters_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/S-Corkum/mcp-server/test/functional/client"
)

var _ = Describe("ElastiCache Integration", func() {
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

	// Helper function to check if ElastiCache is configured
	isElastiCacheConfigured := func() bool {
		// Check server health to see if ElastiCache is configured
		healthData, err := mcpClient.GetHealth(ctx)
		if err != nil {
			return false
		}
		
		// Check if ElastiCache is mentioned in the health data
		if cacheInfo, ok := healthData["cache"].(map[string]interface{}); ok {
			if cacheType, ok := cacheInfo["type"].(string); ok {
				return cacheType == "elasticache" || cacheType == "redis" || cacheType == "redis_cluster"
			}
		}
		
		return false
	}

	Describe("ElastiCache Connectivity", func() {
		It("should connect to ElastiCache if configured", func() {
			// Skip test if ElastiCache is not configured
			if !isElastiCacheConfigured() {
				Skip("ElastiCache not configured, skipping test")
			}
			
			// Create a context to test ElastiCache operations
			payload := map[string]interface{}{
				"agent_id":   "elasticache-connectivity-test",
				"model_id":   "gpt-4",
				"max_tokens": 4000,
				"metadata": map[string]interface{}{
					"elasticache_test": true,
				},
			}
			
			context, err := mcpClient.CreateContext(ctx, payload)
			Expect(err).NotTo(HaveOccurred())
			contextID := context["id"].(string)
			
			// Retrieve the context multiple times to test caching
			for i := 0; i < 3; i++ {
				retrievedContext, err := mcpClient.GetContext(ctx, contextID)
				Expect(err).NotTo(HaveOccurred())
				Expect(retrievedContext["id"]).To(Equal(contextID))
			}
			
			// Clean up the context
			path := "/api/v1/contexts/" + contextID
			_, _ = mcpClient.Delete(ctx, path)
		})

		It("should handle cache operations with ElastiCache configured", func() {
			// Skip test if ElastiCache is not configured
			if !isElastiCacheConfigured() {
				Skip("ElastiCache not configured, skipping test")
			}
			
			// Create a context for cache testing
			payload := map[string]interface{}{
				"agent_id":   "elasticache-operations-test",
				"model_id":   "gpt-4",
				"max_tokens": 4000,
			}
			
			context, err := mcpClient.CreateContext(ctx, payload)
			Expect(err).NotTo(HaveOccurred())
			contextID := context["id"].(string)
			
			// Add an item to the context
			path := "/api/v1/contexts/" + contextID + "/items"
			itemPayload := map[string]interface{}{
				"role":    "user",
				"content": "This is an ElastiCache test message",
			}
			
			resp, err := mcpClient.Post(ctx, path, itemPayload)
			Expect(err).NotTo(HaveOccurred())
			resp.Body.Close()
			
			// Should be successful (200 OK or 201 Created)
			Expect(resp.StatusCode).To(SatisfyAny(Equal(200), Equal(201)))
			
			// Get the context to populate cache
			context1, err := mcpClient.GetContext(ctx, contextID)
			Expect(err).NotTo(HaveOccurred())
			
			// Get context again from cache
			context2, err := mcpClient.GetContext(ctx, contextID)
			Expect(err).NotTo(HaveOccurred())
			
			// Both contexts should be identical
			context1Bytes, err := json.Marshal(context1)
			Expect(err).NotTo(HaveOccurred())
			
			context2Bytes, err := json.Marshal(context2)
			Expect(err).NotTo(HaveOccurred())
			
			Expect(string(context1Bytes)).To(Equal(string(context2Bytes)))
			
			// Add another item to invalidate cache
			itemPayload2 := map[string]interface{}{
				"role":    "assistant",
				"content": "This should invalidate the ElastiCache entry",
			}
			
			resp, err = mcpClient.Post(ctx, path, itemPayload2)
			Expect(err).NotTo(HaveOccurred())
			resp.Body.Close()
			
			// Get updated context
			context3, err := mcpClient.GetContext(ctx, contextID)
			Expect(err).NotTo(HaveOccurred())
			
			// Updated context should have more items
			items1, ok := context1["items"].([]interface{})
			Expect(ok).To(BeTrue(), "items should be an array")
			
			items3, ok := context3["items"].([]interface{})
			Expect(ok).To(BeTrue(), "items should be an array")
			
			Expect(len(items3)).To(BeNumerically(">", len(items1)))
			
			// Clean up the context
			deletePath := "/api/v1/contexts/" + contextID
			_, _ = mcpClient.Delete(ctx, deletePath)
		})
	})

	Describe("ElastiCache Configuration", func() {
		It("should handle ElastiCache endpoint and port configuration correctly", func() {
			// Skip test if ElastiCache is not configured
			if !isElastiCacheConfigured() {
				Skip("ElastiCache not configured, skipping test")
			}
			
			// Get server configuration endpoint
			path := "/api/v1/config"
			resp, err := mcpClient.Get(ctx, path)
			
			// Skip if config endpoint is not available
			if err != nil || resp.StatusCode != http.StatusOK {
				Skip("Config endpoint not available, skipping test")
			}
			defer resp.Body.Close()
			
			// Parse configuration
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			
			var config map[string]interface{}
			err = json.Unmarshal(body, &config)
			Expect(err).NotTo(HaveOccurred())
			
			// Find cache configuration
			var cacheConfig map[string]interface{}
			
			if cfg, ok := config["cache"].(map[string]interface{}); ok {
				cacheConfig = cfg
			}
			
			// Skip if cache configuration is not available
			if cacheConfig == nil {
				Skip("Cache configuration not available, skipping test")
			}
			
			// Verify elasticache configuration exists if it's being used
			var elasticacheConfig map[string]interface{}
			var elasticacheFound bool
			
			if elCache, ok := cacheConfig["elasticache"].(map[string]interface{}); ok {
				elasticacheConfig = elCache
				elasticacheFound = true
			}
			
			if useAWS, ok := cacheConfig["use_aws"].(bool); ok && useAWS {
				Expect(elasticacheFound).To(BeTrue(), "ElastiCache configuration should exist when use_aws is true")
			}
			
			if elasticacheFound {
				// Verify ElastiCache configuration has minimum required fields
				if primaryEndpoint, ok := elasticacheConfig["primary_endpoint"].(string); ok {
					Expect(primaryEndpoint).NotTo(BeEmpty(), "ElastiCache primary endpoint should not be empty")
				}
				
				// Verify port is set
				var port float64
				if p, ok := elasticacheConfig["port"].(float64); ok {
					port = p
				}
				
				Expect(port).To(BeNumerically(">", 0), "ElastiCache port should be set")
				
				// Check if cluster mode is correctly set
				if clusterMode, ok := elasticacheConfig["cluster_mode"].(bool); ok && clusterMode {
					// For cluster mode, verify we have configuration for multiple nodes
					hasNodes := false
					if nodes, ok := elasticacheConfig["cache_nodes"].([]interface{}); ok && len(nodes) > 0 {
						hasNodes = true
					}
					
					hasReaderEndpoint := false
					if readerEndpoint, ok := elasticacheConfig["reader_endpoint"].(string); ok && readerEndpoint != "" {
						hasReaderEndpoint = true
					}
					
					Expect(hasNodes || hasReaderEndpoint).To(BeTrue(), 
						"Cluster mode should have either cache_nodes or reader_endpoint configured")
				}
			}
			
			// Test basic cache operation to verify configuration works
			// Create a test context
			payload := map[string]interface{}{
				"agent_id":   "elasticache-config-test",
				"model_id":   "gpt-4",
				"max_tokens": 4000,
			}
			
			context, err := mcpClient.CreateContext(ctx, payload)
			Expect(err).NotTo(HaveOccurred())
			contextID := context["id"].(string)
			
			// Retrieve the context to test cache operation
			_, err = mcpClient.GetContext(ctx, contextID)
			Expect(err).NotTo(HaveOccurred())
			
			// Clean up the context
			deletePath := "/api/v1/contexts/" + contextID
			_, _ = mcpClient.Delete(ctx, deletePath)
		})
	})
})
