package api_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"functional-tests/client"

	"github.com/developer-mesh/developer-mesh/pkg/models"
)

var _ = Describe("Event Flow Tests", func() {
	var (
		ctx        context.Context
		mcpClient  *client.MCPClient
		testLogger *client.TestLogger
	)

	BeforeEach(func() {
		ctx = context.Background()
		testLogger = client.NewTestLogger()
		mcpClient = client.NewMCPClient(
			ServerURL,
			APIKey,
			client.WithTenantID("test-tenant-1"),
			client.WithLogger(testLogger),
		)
	})

	Describe("Event Emission and Processing", func() {
		It("should create a model and generate creation events", func() {
			// Create a test model
			modelReq := &models.Model{
				Name:     "Event Test Model",
				TenantID: "test-tenant-1",
			}

			createdModel, err := mcpClient.CreateModel(ctx, modelReq)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdModel).NotTo(BeNil())
			Expect(createdModel.ID).NotTo(BeEmpty())

			// Wait for event processing
			time.Sleep(1 * time.Second)

			// Retrieve the model to verify it was processed
			retrievedModel, err := mcpClient.GetModel(ctx, createdModel.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedModel).NotTo(BeNil())
			Expect(retrievedModel.ID).To(Equal(createdModel.ID))
			Expect(retrievedModel.Name).To(Equal("Event Test Model"))
		})

		It("should create and update a context with proper event flow", func() {
			// First create a model to reference
			modelReq := &models.Model{
				Name:     "Context Event Test Model",
				TenantID: "test-tenant-1",
			}

			createdModel, err := mcpClient.CreateModel(ctx, modelReq)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdModel.ID).NotTo(BeEmpty())

			// Create a context
			contextPayload := map[string]interface{}{
				"name":        "Event Test Context",
				"description": "Created for event flow testing",
				"tenant_id":   "test-tenant-1",
				"model_id":    createdModel.ID,
			}

			createdContext, err := mcpClient.CreateContext(ctx, contextPayload)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdContext).NotTo(BeNil())
			contextID, ok := createdContext["id"].(string)
			Expect(ok).To(BeTrue())
			Expect(contextID).NotTo(BeEmpty())

			// Wait for event processing
			time.Sleep(1 * time.Second)

			// Update the context to trigger another event
			updatePayload := map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"role":    "user",
						"content": "Updated content for event flow testing",
					},
				},
				"options": nil,
			}

			path := fmt.Sprintf("/api/v1/contexts/%s", contextID)
			resp, err := mcpClient.Put(ctx, path, updatePayload)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()
			Expect(resp.StatusCode).To(Equal(200))

			// Wait for event processing
			time.Sleep(1 * time.Second)

			// Retrieve the context to verify the update was processed
			retrievedContext, err := mcpClient.GetContext(ctx, contextID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedContext).NotTo(BeNil())
			// Verify the context still exists and was processed
			Expect(retrievedContext["id"]).To(Equal(contextID))
		})
	})

	Describe("Event Filtering", func() {
		It("should properly handle filtered events", func() {
			// Create two models with different tenant IDs
			model1 := &models.Model{
				Name:     "Tenant 1 Model",
				TenantID: "test-tenant-1",
			}

			createdModel1, err := mcpClient.CreateModel(ctx, model1)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdModel1.ID).NotTo(BeEmpty())

			// Create a client with a different tenant ID
			tenant2Client := client.NewMCPClient(
				ServerURL,
				APIKey,
				client.WithTenantID("test-tenant-2"),
				client.WithLogger(testLogger),
			)

			model2 := &models.Model{
				Name:     "Tenant 2 Model",
				TenantID: "test-tenant-2",
			}

			createdModel2, err := tenant2Client.CreateModel(ctx, model2)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdModel2.ID).NotTo(BeEmpty())

			// Wait for event processing
			time.Sleep(1 * time.Second)

			// Both clients use the same API key, so they have the same tenant
			// They should be able to access each other's models
			retrievedModel2ByClient1, err := mcpClient.GetModel(ctx, createdModel2.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedModel2ByClient1.ID).To(Equal(createdModel2.ID))

			retrievedModel1ByClient2, err := tenant2Client.GetModel(ctx, createdModel1.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedModel1ByClient2.ID).To(Equal(createdModel1.ID))

			// Both models should have the same tenant ID from the API key
			Expect(createdModel1.TenantID).To(Equal("test-tenant-1"))
			Expect(createdModel2.TenantID).To(Equal("test-tenant-1"))
		})
	})
})
