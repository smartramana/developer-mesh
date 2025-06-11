package rest_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	
	"functional-tests/shared"
	"github.com/S-Corkum/devops-mcp/pkg/models"
)

var _ = Describe("Model API", func() {
	var (
		client *http.Client
		tenant1Key string
		tenant2Key string
	)
	
	BeforeEach(func() {
		client = &http.Client{}
		tenant1Key = shared.GetTestAPIKey("test-tenant-1")
		tenant2Key = shared.GetTestAPIKey("test-tenant-2")
	})
	
	Context("Tenant Isolation", func() {
		var modelID string
		
		It("should create a model for tenant 1", func() {
			model := models.Model{
				Name: "Test Model Tenant 1",
				Provider: "openai",
				Configuration: map[string]interface{}{
					"apiKey": "test-key",
				},
			}
			
			body, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())
			
			req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/models", restURL), bytes.NewReader(body))
			Expect(err).NotTo(HaveOccurred())
			
			// Set headers for tenant 1
			for k, v := range shared.GetAuthHeaders(tenant1Key) {
				req.Header.Set(k, v)
			}
			
			resp, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			
			var createdModel models.Model
			err = json.NewDecoder(resp.Body).Decode(&createdModel)
			Expect(err).NotTo(HaveOccurred())
			
			Expect(createdModel.Name).To(Equal("Test Model Tenant 1"))
			Expect(createdModel.TenantID).To(Equal("test-tenant-1"))
			modelID = createdModel.ID
		})
		
		It("should not allow tenant 2 to access tenant 1's model", func() {
			req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/models/%s", restURL, modelID), nil)
			Expect(err).NotTo(HaveOccurred())
			
			// Set headers for tenant 2
			for k, v := range shared.GetAuthHeaders(tenant2Key) {
				req.Header.Set(k, v)
			}
			
			resp, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			
			// Should get forbidden or not found
			Expect(resp.StatusCode).To(Or(Equal(http.StatusForbidden), Equal(http.StatusNotFound)))
		})
		
		It("should list only models for the authenticated tenant", func() {
			// Create a model for tenant 2
			model := models.Model{
				Name: "Test Model Tenant 2",
				Provider: "anthropic",
			}
			
			body, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())
			
			req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/models", restURL), bytes.NewReader(body))
			Expect(err).NotTo(HaveOccurred())
			
			// Set headers for tenant 2
			for k, v := range shared.GetAuthHeaders(tenant2Key) {
				req.Header.Set(k, v)
			}
			
			resp, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred())
			resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			
			// Now list models for tenant 2
			req, err = http.NewRequest("GET", fmt.Sprintf("%s/api/v1/models", restURL), nil)
			Expect(err).NotTo(HaveOccurred())
			
			for k, v := range shared.GetAuthHeaders(tenant2Key) {
				req.Header.Set(k, v)
			}
			
			resp, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			
			bodyBytes, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			
			var listResp struct {
				Models []models.Model `json:"models"`
				Total  int `json:"total"`
			}
			err = json.Unmarshal(bodyBytes, &listResp)
			Expect(err).NotTo(HaveOccurred())
			
			// Should only see tenant 2's models
			for _, m := range listResp.Models {
				Expect(m.TenantID).To(Equal("test-tenant-2"))
			}
		})
		
		It("should not allow updating another tenant's model", func() {
			updateReq := models.Model{
				Name: "Hacked Model Name",
			}
			
			body, err := json.Marshal(updateReq)
			Expect(err).NotTo(HaveOccurred())
			
			req, err := http.NewRequest("PUT", fmt.Sprintf("%s/api/v1/models/%s", restURL, modelID), bytes.NewReader(body))
			Expect(err).NotTo(HaveOccurred())
			
			// Set headers for tenant 2 trying to update tenant 1's model
			for k, v := range shared.GetAuthHeaders(tenant2Key) {
				req.Header.Set(k, v)
			}
			
			resp, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			
			// Should get forbidden or not found
			Expect(resp.StatusCode).To(Or(Equal(http.StatusForbidden), Equal(http.StatusNotFound)))
		})
	})
})