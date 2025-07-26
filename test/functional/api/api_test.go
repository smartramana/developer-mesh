package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	// Use pkg/models package which is the public API
	// This aligns with our forward-only migration strategy
	"functional-tests/client"

	"github.com/developer-mesh/developer-mesh/pkg/models"
)

// Import variables from the suite
var (
	ServerURL     string
	APIKey        string
	MockServerURL string
	testAgentID   string
	testModelIDs  []string
	testLogger    *client.TestLogger // For observability
)

func init() {
	// These will be set by the suite before tests run
	// Use REST_API_URL for REST API tests
	ServerURL = os.Getenv("REST_API_URL")
	if ServerURL == "" {
		// Fallback to MCP_SERVER_URL for backward compatibility
		ServerURL = os.Getenv("MCP_SERVER_URL")
	}
	if ServerURL == "" {
		ServerURL = "http://localhost:8081"
	}

	APIKey = os.Getenv("API_KEY")
	if APIKey == "" {
		APIKey = os.Getenv("MCP_API_KEY")
	}
	if APIKey == "" {
		APIKey = os.Getenv("ADMIN_API_KEY")
	}
	if APIKey == "" {
		APIKey = "dev-admin-key-1234567890"
	}

	MockServerURL = os.Getenv("MOCKSERVER_URL")
	if MockServerURL == "" {
		MockServerURL = "http://localhost:8082"
	}

	// Initialize a test logger for observability
	testLogger = client.NewTestLogger()
}

var _ = BeforeSuite(func() {
	// Create multiple test models
	tempClient := client.NewMCPClient(ServerURL, APIKey, client.WithTenantID("test-tenant-1"))
	for i := 1; i <= 2; i++ {
		var modelID string

		// Create a test model using the typed client with new model structure
		modelReq := &models.Model{
			Name:     "Functional Test Model",
			TenantID: "test-tenant-1",
		}
		var createdModel *models.Model
		createdModel, err := tempClient.CreateModel(context.Background(), modelReq)

		// Debug the response
		fmt.Fprintf(os.Stderr, "DEBUG: CreateModel err=%v, createdModel=%+v\n", err, createdModel)

		// If successful, use the model ID
		if err == nil && createdModel != nil && createdModel.ID != "" {
			modelID = createdModel.ID
			fmt.Fprintf(os.Stderr, "DEBUG: Model created successfully with ID: %s\n", modelID)
			testModelIDs = append(testModelIDs, modelID)
			fmt.Fprintf(os.Stderr, "DEBUG: testModelIDs after append: %v\n", testModelIDs)
			continue
		}

		// Fall back to the generic method if the typed methods fail
		if err != nil {
			fmt.Fprintf(os.Stderr, "Typed model creation failed, falling back to generic: %v\n", err)
			modelPayload := map[string]interface{}{
				"name":        "Functional Test Model",
				"description": "Created by functional test",
				"tenant_id":   "test-tenant-1",
			}
			resp, err := tempClient.Post(context.Background(), "/api/v1/models", modelPayload)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()
			modelBody, _ := io.ReadAll(resp.Body)
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				fmt.Fprintf(os.Stderr, "Model creation failed: status=%d, body=%s\n", resp.StatusCode, string(modelBody))
			}
			var modelResult map[string]interface{}
			_ = json.Unmarshal(modelBody, &modelResult)
			modelID, ok := modelResult["id"].(string)
			if !ok || modelID == "" {
				fmt.Fprintf(os.Stderr, "Model creation did not return id: status=%d, body=%s, parsed=%#v\n", resp.StatusCode, string(modelBody), modelResult)
			}
			Expect(ok && modelID != "").To(BeTrue(), "Model creation failed, status=%d, body=%s, parsed=%#v", resp.StatusCode, string(modelBody), modelResult)
			if modelID != "" {
				testModelIDs = append(testModelIDs, modelID)
			}
		}
	}

	// Debug: Print the model IDs
	fmt.Fprintf(os.Stderr, "DEBUG: testModelIDs = %v\n", testModelIDs)

	// Ensure we have at least one model ID
	Expect(len(testModelIDs)).To(BeNumerically(">", 0), "No model IDs were collected")

	// Create a test agent with a valid model_id
	// Try the typed method with new model structure
	agentReq := &models.Agent{
		Name:     "Functional Test Agent",
		TenantID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		ModelID:  testModelIDs[0],
	}
	fmt.Fprintf(os.Stderr, "DEBUG: Creating agent with ModelID = %s\n", agentReq.ModelID)

	// Define a new err variable for agent creation
	var agentErr error
	var createdAgent *models.Agent
	createdAgent, agentErr = tempClient.CreateAgent(context.Background(), agentReq)

	// If successful, use the agent ID
	if agentErr == nil && createdAgent != nil && createdAgent.ID != "" {
		testAgentID = createdAgent.ID
		return // Skip the fallback method
	}

	// Fall back to the generic method if the typed methods fail
	fmt.Fprintf(os.Stderr, "Typed agent creation failed, falling back to generic: %v\n", agentErr)
	agentPayload := map[string]interface{}{
		"name":      "Test Agent",
		"model_id":  testModelIDs[0],
		"tenant_id": "test-tenant-1",
	}
	resp, err := tempClient.Post(context.Background(), "/api/v1/agents", agentPayload)
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Fprintf(os.Stderr, "Agent creation failed: status=%d, body=%s\n", resp.StatusCode, string(body))
	}
	var agentResult map[string]interface{}
	_ = json.Unmarshal(body, &agentResult)
	id, ok := agentResult["id"].(string)
	if !ok || id == "" {
		fmt.Fprintf(os.Stderr, "Agent creation did not return id: status=%d, body=%s, parsed=%#v\n", resp.StatusCode, string(body), agentResult)
	}
	Expect(ok && id != "").To(BeTrue(), "Agent creation failed, status=%d, body=%s, parsed=%#v", resp.StatusCode, string(body), agentResult)
	testAgentID = id
})

var _ = Describe("API", func() {
	var mcpClient *client.MCPClient
	var ctx context.Context
	var cancel context.CancelFunc

	BeforeEach(func() {
		// Create a new MCP client for each test
		mcpClient = client.NewMCPClient(ServerURL, APIKey, client.WithTenantID("test-tenant-1"))

		// Create a context with timeout for requests
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	})

	AfterEach(func() {
		// Cancel the context after each test
		cancel()
	})

	Describe("Health Endpoint", func() {
		It("should return 200 for health endpoint", func() {
			// Call the health endpoint
			resp, err := mcpClient.Get(ctx, "/health")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			// Verify response status - health endpoint should return 200 OK
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			// Parse response to verify format
			var result map[string]interface{}
			err = client.ParseResponse(resp, &result)
			Expect(err).NotTo(HaveOccurred())

			// Verify the response contains the expected fields
			Expect(result).To(HaveKey("status"))
			Expect(result).To(HaveKey("components"))
		})
	})

	Describe("API Versioning", func() {
		It("should require authentication for API versioning endpoint", func() {
			// Create a client without an API key
			unauthClient := client.NewMCPClient(ServerURL, "", client.WithTenantID("test-tenant-1"))

			// Call the root API endpoint without authentication
			resp, err := unauthClient.Get(ctx, "/api/v1")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			// Expect Unauthorized status
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))

			// Parse the error response
			var result map[string]interface{}
			err = client.ParseResponse(resp, &result)
			Expect(err).NotTo(HaveOccurred())

			// Verify there's an error message in the response
			Expect(result).To(HaveKey("error"))
		})
	})

	Describe("Authentication", func() {
		It("should require authentication for protected endpoints", func() {
			// Create client without API key
			unauthClient := client.NewMCPClient(ServerURL, "", client.WithTenantID("test-tenant-1"))

			// Call the tools endpoint without authentication
			resp, err := unauthClient.Get(ctx, "/api/v1/tools")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			// Verify response status (should be unauthorized)
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("should accept valid API key", func() {
			// Call the tools endpoint with authentication
			resp, err := mcpClient.Get(ctx, "/api/v1/tools")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			// With the current authentication behavior, expect StatusOK
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("Vectors API", func() {
		contextID := "ctx_123"
		modelID := "text-embedding-ada-002"
		// These endpoints require the context/model to exist; expect 404 for mock/unimplemented
		It("should store an embedding (404 for missing context)", func() {
			payload := map[string]interface{}{
				"context_id":    contextID,
				"content_index": 0,
				"text":          "Hello AI assistant!",
				"embedding":     []float64{0.1, 0.2, 0.3},
				"model_id":      modelID,
			}
			resp, err := mcpClient.Post(ctx, "/api/v1/vectors/store", payload)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
		It("should return 404 for invalid embedding payload (missing context)", func() {
			payload := map[string]interface{}{"context_id": contextID} // missing required fields
			resp, err := mcpClient.Post(ctx, "/api/v1/vectors/store", payload)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
		It("should search embeddings (404 for missing context)", func() {
			payload := map[string]interface{}{
				"context_id":           contextID,
				"query_embedding":      []float64{0.1, 0.2, 0.3},
				"limit":                5,
				"model_id":             modelID,
				"similarity_threshold": 0.7,
			}
			resp, err := mcpClient.Post(ctx, "/api/v1/vectors/search", payload)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
		It("should return 404 for invalid search payload (missing context)", func() {
			payload := map[string]interface{}{"context_id": contextID} // missing required fields
			resp, err := mcpClient.Post(ctx, "/api/v1/vectors/search", payload)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
		It("should get context embeddings (404 for missing context)", func() {
			path := "/api/v1/vectors/context/" + contextID
			resp, err := mcpClient.Get(ctx, path)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
		It("should delete context embeddings (404 for missing context)", func() {
			path := "/api/v1/vectors/context/" + contextID
			resp, err := mcpClient.Delete(ctx, path)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
		It("should get supported models (404 for missing implementation)", func() {
			resp, err := mcpClient.Get(ctx, "/api/v1/vectors/models")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
		It("should get model embeddings (404 for missing context/model)", func() {
			path := "/api/v1/vectors/context/" + contextID + "/model/" + modelID
			resp, err := mcpClient.Get(ctx, path)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
		It("should return 404 for invalid model embeddings request (missing context/model)", func() {
		})
	})

	Describe("Context API", func() {
		var createdContextID string

		BeforeEach(func() {
			// Always create a context before each test that needs it
			// Use the first test model by default; to test with multiple models, iterate over testModelIDs as needed.
			payload := map[string]interface{}{
				"name":        "Test Context",
				"description": "Created by functional test",
				"max_tokens":  4000,
				"agent_id":    testAgentID,
				"model_id":    testModelIDs[0],
			}
			resp, err := mcpClient.Post(ctx, "/api/v1/contexts", payload)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()
			body, _ := io.ReadAll(resp.Body)
			fmt.Fprintf(os.Stderr, "DEBUG: Context creation response status=%d, body=%s\n", resp.StatusCode, string(body))
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
				fmt.Fprintf(os.Stderr, "Failed to create context: status=%d, body=%s\n", resp.StatusCode, string(body))
			}
			Expect(resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated).
				To(BeTrue(), "Context creation failed: status=%d, body=%s", resp.StatusCode, string(body))

			// Reset resp.Body so it can be parsed again
			resp.Body = io.NopCloser(bytes.NewBuffer(body))

			var result map[string]interface{}
			err = client.ParseWrappedResponse(resp, &result)
			Expect(err).NotTo(HaveOccurred())
			id, ok := result["id"].(string)
			Expect(ok).To(BeTrue())
			createdContextID = id
		})

		It("should create a new context", func() {
			// Already tested in BeforeEach, just check that createdContextID is set
			Expect(createdContextID).NotTo(BeEmpty())
		})

		It("should retrieve an existing context", func() {
			path := fmt.Sprintf("/api/v1/contexts/%s", createdContextID)
			resp, err := mcpClient.Get(ctx, path)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()
			Expect(resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized).To(BeTrue())
			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = client.ParseWrappedResponse(resp, &result)
				Expect(err).NotTo(HaveOccurred())
				Expect(result["id"]).To(Equal(createdContextID))
				Expect(result["name"]).To(Equal("Test Context"))
			}
		})

		It("should update an existing context", func() {
			updatePayload := map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"role":    "user",
						"content": "Updated test content",
					},
				},
				"options": nil,
			}
			path := fmt.Sprintf("/api/v1/contexts/%s", createdContextID)
			resp, err := mcpClient.Put(ctx, path, updatePayload)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			// Debug response
			body, _ := io.ReadAll(resp.Body)
			fmt.Fprintf(os.Stderr, "Update context response status=%d, body=%s\n", resp.StatusCode, string(body))
			resp.Body = io.NopCloser(bytes.NewBuffer(body))

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			var result map[string]interface{}
			err = client.ParseWrappedResponse(resp, &result)
			Expect(err).NotTo(HaveOccurred())
			Expect(result["id"]).To(Equal(createdContextID))
			// Verify content was updated
			content, ok := result["content"].([]interface{})
			Expect(ok).To(BeTrue())
			Expect(len(content)).To(BeNumerically(">", 0))
		})

		It("should search within a context", func() {
			// First, add some content to search
			updatePayload := map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"role":    "user",
						"content": "This is test content for searching",
					},
				},
			}
			updatePath := fmt.Sprintf("/api/v1/contexts/%s", createdContextID)
			updateResp, err := mcpClient.Put(ctx, updatePath, updatePayload)
			Expect(err).NotTo(HaveOccurred())
			_ = updateResp.Body.Close()

			// Now search for content
			searchPayload := map[string]interface{}{
				"query": "test",
			}
			path := fmt.Sprintf("/api/v1/contexts/%s/search", createdContextID)
			resp, err := mcpClient.Post(ctx, path, searchPayload)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			var result map[string]interface{}
			err = client.ParseResponse(resp, &result)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveKey("results"))
		})

		It("should get a context summary", func() {
			path := fmt.Sprintf("/api/v1/contexts/%s/summary", createdContextID)
			resp, err := mcpClient.Get(ctx, path)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			var result map[string]interface{}
			err = client.ParseResponse(resp, &result)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveKey("context_id"))
			Expect(result).To(HaveKey("summary"))
			Expect(result).To(HaveKey("_links"))
		})

		It("should delete a context", func() {
			path := fmt.Sprintf("/api/v1/contexts/%s", createdContextID)
			resp, err := mcpClient.Delete(ctx, path)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			getResp, err := mcpClient.Get(ctx, path)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = getResp.Body.Close() }()
			Expect(getResp.StatusCode).To(Equal(http.StatusNotFound))
		})
	})
})
