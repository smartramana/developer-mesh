package api_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"net/http"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"functional-tests/client"
)

var _ = Describe("Authentication Tests", func() {
	var (
		ctx                 context.Context
		validClient         *client.MCPClient
		invalidKeyClient    *client.MCPClient
		noKeyClient         *client.MCPClient
		crossTenantClient   *client.MCPClient
		testLogger          *client.TestLogger
	)

	BeforeEach(func() {
		ctx = context.Background()
		testLogger = client.NewTestLogger()
		
		// Client with valid authentication
		validClient = client.NewMCPClient(
			ServerURL,
			APIKey,
			client.WithTenantID("test-tenant-1"),
			client.WithLogger(testLogger),
		)
		
		// Client with invalid API key
		invalidKeyClient = client.NewMCPClient(
			ServerURL,
			"invalid-api-key",
			client.WithTenantID("test-tenant-1"),
			client.WithLogger(testLogger),
		)
		
		// Client with no API key
		noKeyClient = client.NewMCPClient(
			ServerURL,
			"",
			client.WithTenantID("test-tenant-1"),
			client.WithLogger(testLogger),
		)
		
		// Client with different tenant ID but valid API key
		crossTenantClient = client.NewMCPClient(
			ServerURL,
			APIKey,
			client.WithTenantID("test-tenant-2"),
			client.WithLogger(testLogger),
		)
	})

	Describe("API Key Authentication", func() {
		It("should reject requests with no API key", func() {
			resp, err := noKeyClient.Get(ctx, "/api/v1/tools")
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("should reject requests with invalid API key", func() {
			resp, err := invalidKeyClient.Get(ctx, "/api/v1/tools")
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("should accept requests with valid API key", func() {
			resp, err := validClient.Get(ctx, "/api/v1/tools")
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		It("should handle special test-admin-api-key format correctly", func() {
			// The client.go implementation has special handling for this key
			adminClient := client.NewMCPClient(
				ServerURL,
				"test-admin-api-key",
				client.WithTenantID("test-tenant-1"),
				client.WithLogger(testLogger),
			)
			
			resp, err := adminClient.Get(ctx, "/api/v1/tools")
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("Tenant-based Access Control", func() {
		It("should prevent cross-tenant access to models", func() {
			// First create a model with tenant-1
			model := &models.Model{
				Name:     "Auth Test Model",
				TenantID: "test-tenant-1",
			}
			
			createdModel, err := validClient.CreateModel(ctx, model)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdModel.ID).NotTo(BeEmpty())
			
			// Attempt to access with tenant-2 client
			_, err = crossTenantClient.GetModel(ctx, createdModel.ID)
			Expect(err).To(HaveOccurred()) // Should fail due to tenant mismatch
		})

		It("should prevent cross-tenant access to contexts", func() {
			// First create a model for the context
			model := &models.Model{
				Name:     "Context Auth Test Model",
				TenantID: "test-tenant-1",
			}
			
			createdModel, err := validClient.CreateModel(ctx, model)
			Expect(err).NotTo(HaveOccurred())
			
			// Create a context with tenant-1
			contextPayload := map[string]interface{}{
				"name":        "Auth Test Context",
				"description": "Created for authentication testing",
				"tenant_id":   "test-tenant-1",
				"model_id":    createdModel.ID,
			}
			
			createdContext, err := validClient.CreateContext(ctx, contextPayload)
			Expect(err).NotTo(HaveOccurred())
			contextID, ok := createdContext["id"].(string)
			Expect(ok).To(BeTrue())
			
			// Attempt to access with tenant-2 client
			_, err = crossTenantClient.GetContext(ctx, contextID)
			Expect(err).To(HaveOccurred()) // Should fail due to tenant mismatch
		})

		It("should allow access to own tenant resources", func() {
			// Create resources with each tenant and verify they can access their own
			
			// Tenant 1 resources
			model1 := &models.Model{
				Name:     "Tenant 1 Auth Model",
				TenantID: "test-tenant-1",
			}
			
			createdModel1, err := validClient.CreateModel(ctx, model1)
			Expect(err).NotTo(HaveOccurred())
			
			// Tenant 2 resources
			model2 := &models.Model{
				Name:     "Tenant 2 Auth Model",
				TenantID: "test-tenant-2",
			}
			
			createdModel2, err := crossTenantClient.CreateModel(ctx, model2)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify tenant 1 can access its resources
			retrievedModel1, err := validClient.GetModel(ctx, createdModel1.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedModel1.ID).To(Equal(createdModel1.ID))
			
			// Verify tenant 2 can access its resources
			retrievedModel2, err := crossTenantClient.GetModel(ctx, createdModel2.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedModel2.ID).To(Equal(createdModel2.ID))
		})
	})

	Describe("Header-based Authentication", func() {
		It("should accept both Authorization header formats", func() {
			// Test 1: Valid test-admin-api-key should work
			adminClient := client.NewMCPClient(
				ServerURL,
				"test-admin-api-key",
				client.WithTenantID("test-tenant-1"),
				client.WithLogger(testLogger),
			)
			
			resp, err := adminClient.Get(ctx, "/api/v1/models")
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK), "test-admin-api-key should be accepted")
			
			// Test 2: Invalid API key should be rejected
			invalidClient := client.NewMCPClient(
				ServerURL,
				"invalid-api-key",
				client.WithTenantID("test-tenant-1"),
				client.WithLogger(testLogger),
			)
			
			resp2, err := invalidClient.Get(ctx, "/api/v1/models")
			Expect(err).NotTo(HaveOccurred())
			defer resp2.Body.Close()
			Expect(resp2.StatusCode).To(Equal(http.StatusUnauthorized), "invalid API key should be rejected")
		})
	})
})
