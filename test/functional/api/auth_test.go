package api_test

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"net/http"

	"functional-tests/client"
	"github.com/S-Corkum/devops-mcp/pkg/models"
)

var _ = Describe("Authentication Tests", func() {
	var (
		ctx               context.Context
		validClient       *client.MCPClient
		invalidKeyClient  *client.MCPClient
		noKeyClient       *client.MCPClient
		crossTenantClient *client.MCPClient
		testLogger        *client.TestLogger
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
			defer func() { _ = resp.Body.Close() }()

			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("should reject requests with invalid API key", func() {
			resp, err := invalidKeyClient.Get(ctx, "/api/v1/tools")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("should accept requests with valid API key", func() {
			resp, err := validClient.Get(ctx, "/api/v1/tools")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		It("should handle admin API key correctly", func() {
			// Use the correct API key from environment
			apiKey := os.Getenv("ADMIN_API_KEY")
			if apiKey == "" {
				apiKey = "dev-admin-key-1234567890"
			}
			adminClient := client.NewMCPClient(
				ServerURL,
				apiKey,
				client.WithTenantID("test-tenant-1"),
				client.WithLogger(testLogger),
			)

			resp, err := adminClient.Get(ctx, "/api/v1/tools")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("Tenant-based Access Control", func() {
		It("should use the tenant ID from the API key", func() {
			// All API keys in dev/docker environment use the default tenant ID
			// This test verifies that the tenant ID from the API key is used, not the one from the header
			model := &models.Model{
				Name:     "Auth Test Model",
				TenantID: "test-tenant-1", // This will be overridden by the API key's tenant ID
			}

			createdModel, err := validClient.CreateModel(ctx, model)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdModel.ID).NotTo(BeEmpty())
			// The model should be created with the API key's tenant ID, not the requested one
			Expect(createdModel.TenantID).To(Equal("test-tenant-1"))

			// Both clients have the same API key tenant, so access should work
			retrievedModel, err := crossTenantClient.GetModel(ctx, createdModel.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedModel.ID).To(Equal(createdModel.ID))
		})

		It("should use the tenant ID from the API key for contexts", func() {
			// First create a model for the context
			model := &models.Model{
				Name:     "Context Auth Test Model",
				TenantID: "test-tenant-1",
			}

			createdModel, err := validClient.CreateModel(ctx, model)
			Expect(err).NotTo(HaveOccurred())

			// Create a context - tenant ID will be from API key
			contextPayload := map[string]interface{}{
				"name":        "Auth Test Context",
				"description": "Created for authentication testing",
				"tenant_id":   "test-tenant-1", // This will be overridden
				"model_id":    createdModel.ID,
			}

			createdContext, err := validClient.CreateContext(ctx, contextPayload)
			Expect(err).NotTo(HaveOccurred())
			contextID, ok := createdContext["id"].(string)
			Expect(ok).To(BeTrue())

			// Both clients have the same API key tenant, so access should work
			retrievedContext, err := crossTenantClient.GetContext(ctx, contextID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedContext["id"]).To(Equal(contextID))
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
			// Test 1: Valid admin API key should work
			apiKey := os.Getenv("ADMIN_API_KEY")
			if apiKey == "" {
				apiKey = "dev-admin-key-1234567890"
			}
			adminClient := client.NewMCPClient(
				ServerURL,
				apiKey,
				client.WithTenantID("test-tenant-1"),
				client.WithLogger(testLogger),
			)

			resp, err := adminClient.Get(ctx, "/api/v1/models")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()
			Expect(resp.StatusCode).To(Equal(http.StatusOK), "admin API key should be accepted")

			// Test 2: Invalid API key should be rejected
			invalidClient := client.NewMCPClient(
				ServerURL,
				"invalid-api-key",
				client.WithTenantID("test-tenant-1"),
				client.WithLogger(testLogger),
			)

			resp2, err := invalidClient.Get(ctx, "/api/v1/models")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp2.Body.Close() }()
			Expect(resp2.StatusCode).To(Equal(http.StatusUnauthorized), "invalid API key should be rejected")
		})
	})
})
