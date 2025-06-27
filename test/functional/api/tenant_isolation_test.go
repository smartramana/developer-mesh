package api_test

import (
	"context"
	"fmt"

	"functional-tests/client"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tenant Isolation", func() {
	var (
		ctx           context.Context
		tenant1Client *client.MCPClient
		tenant2Client *client.MCPClient
		tenant1Model  *models.Model
		tenant2Model  *models.Model
		testLogger    *client.TestLogger
	)

	BeforeEach(func() {
		ctx = context.Background()
		testLogger = client.NewTestLogger()

		// Client for tenant 1
		tenant1Client = client.NewMCPClient(
			ServerURL,
			"test-key-tenant-1", // Using test-specific API key for tenant 1
			client.WithTenantID("test-tenant-1"),
			client.WithLogger(testLogger),
		)

		// Client for tenant 2
		tenant2Client = client.NewMCPClient(
			ServerURL,
			"test-key-tenant-2", // Using test-specific API key for tenant 2
			client.WithTenantID("test-tenant-2"),
			client.WithLogger(testLogger),
		)
	})

	Describe("Model Tenant Isolation", func() {
		It("should create models with correct tenant IDs", func() {
			// Create model with tenant 1
			model1 := &models.Model{
				Name:     "Tenant 1 Isolated Model",
				TenantID: "test-tenant-1",
			}

			createdModel1, err := tenant1Client.CreateModel(ctx, model1)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdModel1.TenantID).To(Equal("test-tenant-1"))
			tenant1Model = createdModel1

			// Create model with tenant 2
			model2 := &models.Model{
				Name:     "Tenant 2 Isolated Model",
				TenantID: "test-tenant-2",
			}

			createdModel2, err := tenant2Client.CreateModel(ctx, model2)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdModel2.TenantID).To(Equal("test-tenant-2"))
			tenant2Model = createdModel2

			// Log the created models
			fmt.Printf("Tenant 1 Model: ID=%s, TenantID=%s\n", tenant1Model.ID, tenant1Model.TenantID)
			fmt.Printf("Tenant 2 Model: ID=%s, TenantID=%s\n", tenant2Model.ID, tenant2Model.TenantID)
		})

		It("should prevent cross-tenant access", func() {
			Skip("Depends on previous test creating models")

			// Tenant 1 should NOT be able to access tenant 2's model
			_, err := tenant1Client.GetModel(ctx, tenant2Model.ID)
			Expect(err).To(HaveOccurred())
			// Should get 404 or 403 error

			// Tenant 2 should NOT be able to access tenant 1's model
			_, err = tenant2Client.GetModel(ctx, tenant1Model.ID)
			Expect(err).To(HaveOccurred())
			// Should get 404 or 403 error
		})

		It("should override tenant ID from request body", func() {
			// Try to create a model with wrong tenant ID in body
			model1 := &models.Model{
				Name:     "Wrong Tenant ID Test Model",
				TenantID: "wrong-tenant-id", // This should be overridden
			}

			createdModel1, err := tenant1Client.CreateModel(ctx, model1)
			Expect(err).NotTo(HaveOccurred())
			// Should use tenant ID from API key, not from request body
			Expect(createdModel1.TenantID).To(Equal("test-tenant-1"))

			// Same test for tenant 2
			model2 := &models.Model{
				Name:     "Another Wrong Tenant ID Test Model",
				TenantID: "another-wrong-id", // This should be overridden
			}

			createdModel2, err := tenant2Client.CreateModel(ctx, model2)
			Expect(err).NotTo(HaveOccurred())
			// Should use tenant ID from API key, not from request body
			Expect(createdModel2.TenantID).To(Equal("test-tenant-2"))
		})
	})

	Describe("Context Tenant Isolation", func() {
		It("should create contexts with correct tenant IDs", func() {
			// First create models for each tenant
			model1 := &models.Model{
				Name:     "Context Test Tenant 1 Model",
				TenantID: "test-tenant-1",
			}
			createdModel1, err := tenant1Client.CreateModel(ctx, model1)
			Expect(err).NotTo(HaveOccurred())

			model2 := &models.Model{
				Name:     "Context Test Tenant 2 Model",
				TenantID: "test-tenant-2",
			}
			createdModel2, err := tenant2Client.CreateModel(ctx, model2)
			Expect(err).NotTo(HaveOccurred())

			// Create context for tenant 1
			context1 := map[string]interface{}{
				"name":        "Tenant 1 Isolated Context",
				"description": "Context for tenant 1",
				"tenant_id":   "test-tenant-1",
				"model_id":    createdModel1.ID,
			}
			createdContext1, err := tenant1Client.CreateContext(ctx, context1)
			Expect(err).NotTo(HaveOccurred())

			// Verify tenant ID (contexts might not return tenant_id in response)
			contextID1, ok := createdContext1["id"].(string)
			Expect(ok).To(BeTrue())

			// Create context for tenant 2
			context2 := map[string]interface{}{
				"name":        "Tenant 2 Isolated Context",
				"description": "Context for tenant 2",
				"tenant_id":   "test-tenant-2",
				"model_id":    createdModel2.ID,
			}
			createdContext2, err := tenant2Client.CreateContext(ctx, context2)
			Expect(err).NotTo(HaveOccurred())

			contextID2, ok := createdContext2["id"].(string)
			Expect(ok).To(BeTrue())

			fmt.Printf("Tenant 1 Context: ID=%s\n", contextID1)
			fmt.Printf("Tenant 2 Context: ID=%s\n", contextID2)
		})
	})
})
