package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository/model"
)

// MockModelRepository is a mock implementation of the ModelRepository interface
type MockModelRepository struct {
	mock.Mock
}

// Implement the ModelRepository interface methods
func (m *MockModelRepository) Create(ctx context.Context, model *models.Model) error {
	args := m.Called(ctx, model)
	return args.Error(0)
}

func (m *MockModelRepository) Get(ctx context.Context, id string) (*models.Model, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Model), args.Error(1)
}

func (m *MockModelRepository) List(ctx context.Context, filter model.Filter) ([]*models.Model, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]*models.Model), args.Error(1)
}

func (m *MockModelRepository) Update(ctx context.Context, model *models.Model) error {
	args := m.Called(ctx, model)
	return args.Error(0)
}

func (m *MockModelRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Legacy API-specific methods for backward compatibility
func (m *MockModelRepository) CreateModel(ctx context.Context, model *models.Model) error {
	args := m.Called(ctx, model)
	return args.Error(0)
}

func (m *MockModelRepository) GetModelByID(ctx context.Context, id string, tenantID string) (*models.Model, error) {
	args := m.Called(ctx, id, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Model), args.Error(1)
}

func (m *MockModelRepository) ListModels(ctx context.Context, tenantID string) ([]*models.Model, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]*models.Model), args.Error(1)
}

func (m *MockModelRepository) UpdateModel(ctx context.Context, model *models.Model) error {
	args := m.Called(ctx, model)
	return args.Error(0)
}

func (m *MockModelRepository) DeleteModel(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Helper function to setup the test context with tenant ID
func setupTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	// Setup a minimal HTTP request
	req := httptest.NewRequest("GET", "/", nil)
	c.Request = req
	
	// Set the user context with tenant ID as expected by GetTenantIDFromContext
	c.Set("user", map[string]interface{}{
		"tenant_id": "test-tenant-id",
	})
	
	return c, w
}

func TestModelAPI_CreateModel(t *testing.T) {
	// Setup
	mockRepo := new(MockModelRepository)
	api := &ModelAPI{repo: mockRepo}
	
	// Create test model
	testModel := &models.Model{
		Name: "Test Model",
	}
	
	// Convert to JSON
	modelJSON, _ := json.Marshal(testModel)
	
	// Setup context
	c, w := setupTestContext()
	c.Request = httptest.NewRequest("POST", "/", bytes.NewBuffer(modelJSON))
	c.Request.Header.Set("Content-Type", "application/json")
	
	// Mock expectations
	mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *models.Model) bool {
		return m.Name == "Test Model" && m.TenantID == "test-tenant-id"
	})).Return(nil)
	
	// Execute
	api.createModel(c)
	
	// Assert
	assert.Equal(t, http.StatusCreated, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestModelAPI_ListModels(t *testing.T) {
	// Setup
	mockRepo := new(MockModelRepository)
	api := &ModelAPI{repo: mockRepo}
	
	// Setup mock data
	mockModels := []*models.Model{
		{ID: "model1", Name: "Model 1", TenantID: "test-tenant-id"},
		{ID: "model2", Name: "Model 2", TenantID: "test-tenant-id"},
	}
	
	// Setup context
	c, w := setupTestContext()
	
	// Mock expectations - should use standardized List method with proper Filter
	mockRepo.On("List", mock.Anything, mock.MatchedBy(func(f model.Filter) bool {
		return f["tenant_id"] == "test-tenant-id"
	})).Return(mockModels, nil)
	
	// Execute
	api.listModels(c)
	
	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	
	// Parse response
	var response struct {
		Models []*models.Model `json:"models"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response.Models, 2)
	
	mockRepo.AssertExpectations(t)
}

func TestModelAPI_UpdateModel_Success(t *testing.T) {
	// Setup
	mockRepo := new(MockModelRepository)
	api := &ModelAPI{repo: mockRepo}
	
	// Setup test data
	modelID := "test-model-id"
	tenantID := "test-tenant-id"
	
	existingModel := &models.Model{
		ID:       modelID,
		Name:     "Original Name",
		TenantID: tenantID,
	}
	
	updateModel := &models.Model{
		Name: "Updated Name",
	}
	
	// Convert update to JSON
	updateJSON, _ := json.Marshal(updateModel)
	
	// Setup context
	c, w := setupTestContext()
	c.Params = []gin.Param{{Key: "id", Value: modelID}}
	c.Request = httptest.NewRequest("PUT", "/", bytes.NewBuffer(updateJSON))
	c.Request.Header.Set("Content-Type", "application/json")
	
	// Mock expectations
	// First Get call - standardized Repository.Get with ID
	mockRepo.On("Get", mock.Anything, modelID).Return(existingModel, nil)
	
	// Update call with proper model data
	mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(m *models.Model) bool {
		return m.ID == modelID && m.Name == "Updated Name" && m.TenantID == tenantID
	})).Return(nil)
	
	// Execute
	api.updateModel(c)
	
	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestModelAPI_UpdateModel_NotFound(t *testing.T) {
	// Setup
	mockRepo := new(MockModelRepository)
	api := &ModelAPI{repo: mockRepo}
	
	// Setup test data
	modelID := "missing-model-id"
	
	updateModel := &models.Model{
		Name: "Updated Name",
	}
	
	// Convert update to JSON
	updateJSON, _ := json.Marshal(updateModel)
	
	// Setup context
	c, w := setupTestContext()
	c.Params = []gin.Param{{Key: "id", Value: modelID}}
	c.Request = httptest.NewRequest("PUT", "/", bytes.NewBuffer(updateJSON))
	c.Request.Header.Set("Content-Type", "application/json")
	
	// Mock expectations - model not found
	mockRepo.On("Get", mock.Anything, modelID).Return(nil, fmt.Errorf("not found"))
	
	// Execute
	api.updateModel(c)
	
	// Assert
	assert.Equal(t, http.StatusNotFound, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestModelAPI_UpdateModel_WrongTenant(t *testing.T) {
	// Setup
	mockRepo := new(MockModelRepository)
	api := &ModelAPI{repo: mockRepo}
	
	// Setup test data
	modelID := "test-model-id"
	
	// Model belongs to a different tenant
	existingModel := &models.Model{
		ID:       modelID,
		Name:     "Original Name",
		TenantID: "different-tenant-id", // Different from the request tenant
	}
	
	updateModel := &models.Model{
		Name: "Updated Name",
	}
	
	// Convert update to JSON
	updateJSON, _ := json.Marshal(updateModel)
	
	// Setup context
	c, w := setupTestContext() // This sets tenant to "test-tenant-id"
	c.Params = []gin.Param{{Key: "id", Value: modelID}}
	c.Request = httptest.NewRequest("PUT", "/", bytes.NewBuffer(updateJSON))
	c.Request.Header.Set("Content-Type", "application/json")
	
	// Mock expectations
	mockRepo.On("Get", mock.Anything, modelID).Return(existingModel, nil)
	
	// Execute
	api.updateModel(c)
	
	// Assert
	assert.Equal(t, http.StatusForbidden, w.Code)
	mockRepo.AssertExpectations(t)
}
