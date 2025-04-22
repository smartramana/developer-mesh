package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	
	"github.com/S-Corkum/mcp-server/internal/api"
	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/S-Corkum/mcp-server/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Defining the EmbeddingRepositoryInterface for tests
type EmbeddingRepositoryInterface interface {
	StoreEmbedding(ctx context.Context, embedding *repository.Embedding) error
	SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, similarityThreshold float64) ([]*repository.Embedding, error)
	SearchEmbeddings_Legacy(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*repository.Embedding, error)
	GetContextEmbeddings(ctx context.Context, contextID string) ([]*repository.Embedding, error)
	DeleteContextEmbeddings(ctx context.Context, contextID string) error
	GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*repository.Embedding, error)
	GetSupportedModels(ctx context.Context) ([]string, error)
	DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error
}

// Mock EmbeddingRepository for testing
type MockEmbeddingRepository struct {
	mock.Mock
}

func (m *MockEmbeddingRepository) StoreEmbedding(ctx context.Context, embedding *repository.Embedding) error {
	args := m.Called(ctx, embedding)
	embedding.ID = "embedding-test-id" // Set a test ID
	return args.Error(0)
}

func (m *MockEmbeddingRepository) SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, similarityThreshold float64) ([]*repository.Embedding, error) {
	args := m.Called(ctx, queryVector, contextID, modelID, limit, similarityThreshold)
	return args.Get(0).([]*repository.Embedding), args.Error(1)
}

func (m *MockEmbeddingRepository) SearchEmbeddings_Legacy(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*repository.Embedding, error) {
	args := m.Called(ctx, queryVector, contextID, limit)
	return args.Get(0).([]*repository.Embedding), args.Error(1)
}

func (m *MockEmbeddingRepository) GetContextEmbeddings(ctx context.Context, contextID string) ([]*repository.Embedding, error) {
	args := m.Called(ctx, contextID)
	return args.Get(0).([]*repository.Embedding), args.Error(1)
}

func (m *MockEmbeddingRepository) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

func (m *MockEmbeddingRepository) GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*repository.Embedding, error) {
	args := m.Called(ctx, contextID, modelID)
	return args.Get(0).([]*repository.Embedding), args.Error(1)
}

func (m *MockEmbeddingRepository) GetSupportedModels(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockEmbeddingRepository) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	args := m.Called(ctx, contextID, modelID)
	return args.Error(0)
}

// TestServer is a simplified version of Server for testing
type TestServer struct {
	router  *gin.Engine
	logger  *observability.Logger
	api     *api.VectorAPI
}

// Set up a test server with the mock repository
func setupVectorTestServer(mockRepo *MockEmbeddingRepository) *TestServer {
	gin.SetMode(gin.TestMode)
	
	// Create a logger for testing
	logger := observability.NewLogger("vector-test")
	
	// Create a server with the mock repository
	router := gin.New()
	
	// Create VectorAPI handler
	vectorAPI := api.NewVectorAPI(mockRepo, logger)
	
	// Setup routes
	vectorAPI.RegisterRoutes(router.Group("/api/v1"))
	
	// Return the test server
	return &TestServer{
		router:  router,
		logger:  logger,
		api:     vectorAPI,
	}
}

func TestStoreEmbeddingHandler(t *testing.T) {
	// Create mock repository
	mockRepo := new(MockEmbeddingRepository)
	server := setupVectorTestServer(mockRepo)
	
	// Set expectations
	mockRepo.On("StoreEmbedding", mock.Anything, mock.AnythingOfType("*repository.Embedding")).Return(nil)
	
	// Create test request
	reqBody := struct {
		ContextID    string    `json:"context_id"`
		ContentIndex int       `json:"content_index"`
		Text         string    `json:"text"`
		Embedding    []float32 `json:"embedding"`
		ModelID      string    `json:"model_id"`
	}{
		ContextID:    "context-123",
		ContentIndex: 1,
		Text:         "Test text",
		Embedding:    []float32{0.1, 0.2, 0.3},
		ModelID:      "test-model",
	}
	
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/v1/vectors/store", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	
	// Perform the request
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	
	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)
	
	// Verify that our expectations were met
	mockRepo.AssertExpectations(t)
	
	// Verify response body
	var resp repository.Embedding
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "embedding-test-id", resp.ID)
}

func TestSearchEmbeddingsHandler(t *testing.T) {
	// Create mock repository
	mockRepo := new(MockEmbeddingRepository)
	server := setupVectorTestServer(mockRepo)
	
	// Test case 1: Legacy search (no model ID)
	t.Run("LegacySearch", func(t *testing.T) {
		// Test data
		contextID := "context-123"
		queryVector := []float32{0.1, 0.2, 0.3}
		
		// Mock results
		mockResults := []*repository.Embedding{
			{
				ID:          "emb-1",
				ContextID:   contextID,
				ContentIndex: 1,
				Text:        "Test text 1",
			},
			{
				ID:          "emb-2",
				ContextID:   contextID,
				ContentIndex: 2,
				Text:        "Test text 2",
			},
		}
		
		// Set expectations
		mockRepo.On("SearchEmbeddings_Legacy", mock.Anything, queryVector, contextID, 10).Return(mockResults, nil)
		
		// Create test request
		reqBody := struct {
			ContextID      string    `json:"context_id"`
			QueryEmbedding []float32 `json:"query_embedding"`
			Limit          int       `json:"limit"`
		}{
			ContextID:      contextID,
			QueryEmbedding: queryVector,
			Limit:          10,
		}
		
		jsonBody, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", "/api/v1/vectors/search", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		
		// Perform the request
		w := httptest.NewRecorder()
		server.router.ServeHTTP(w, req)
		
		// Assert response
		assert.Equal(t, http.StatusOK, w.Code)
		
		// Verify that our expectations were met
		mockRepo.AssertExpectations(t)
		
		// Verify response body
		var resp struct {
			Embeddings []*repository.Embedding `json:"embeddings"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Len(t, resp.Embeddings, 2)
		assert.Equal(t, "emb-1", resp.Embeddings[0].ID)
	})
	
	// Test case 2: Model-specific search
	t.Run("ModelSearch", func(t *testing.T) {
		// Test data
		contextID := "context-123"
		modelID := "test-model"
		queryVector := []float32{0.1, 0.2, 0.3}
		similarityThreshold := 0.7
		
		// Mock results
		mockResults := []*repository.Embedding{
			{
				ID:          "emb-1",
				ContextID:   contextID,
				ModelID:     modelID,
				ContentIndex: 1,
				Text:        "Test text 1",
				Similarity:  0.95,
			},
			{
				ID:          "emb-2",
				ContextID:   contextID,
				ModelID:     modelID,
				ContentIndex: 2,
				Text:        "Test text 2",
				Similarity:  0.85,
			},
		}
		
		// Set expectations
		mockRepo.On("SearchEmbeddings", mock.Anything, queryVector, contextID, modelID, 5, similarityThreshold).Return(mockResults, nil)
		
		// Create test request
		reqBody := struct {
			ContextID           string    `json:"context_id"`
			ModelID             string    `json:"model_id"`
			QueryEmbedding      []float32 `json:"query_embedding"`
			Limit               int       `json:"limit"`
			SimilarityThreshold float64   `json:"similarity_threshold"`
		}{
			ContextID:           contextID,
			ModelID:             modelID,
			QueryEmbedding:      queryVector,
			Limit:               5,
			SimilarityThreshold: similarityThreshold,
		}
		
		jsonBody, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", "/api/v1/vectors/search", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		
		// Perform the request
		w := httptest.NewRecorder()
		server.router.ServeHTTP(w, req)
		
		// Assert response
		assert.Equal(t, http.StatusOK, w.Code)
		
		// Verify that our expectations were met
		mockRepo.AssertExpectations(t)
		
		// Verify response body
		var resp struct {
			Embeddings []*repository.Embedding `json:"embeddings"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Len(t, resp.Embeddings, 2)
		assert.Equal(t, "emb-1", resp.Embeddings[0].ID)
		assert.Equal(t, modelID, resp.Embeddings[0].ModelID)
		assert.Equal(t, 0.95, resp.Embeddings[0].Similarity)
	})
}

func TestGetContextEmbeddingsHandler(t *testing.T) {
	// Create mock repository
	mockRepo := new(MockEmbeddingRepository)
	server := setupVectorTestServer(mockRepo)
	
	// Test data
	contextID := "context-123"
	
	// Mock results
	mockResults := []*repository.Embedding{
		{
			ID:          "emb-1",
			ContextID:   contextID,
			ContentIndex: 1,
			Text:        "Test text 1",
		},
		{
			ID:          "emb-2",
			ContextID:   contextID,
			ContentIndex: 2,
			Text:        "Test text 2",
		},
	}
	
	// Set expectations
	mockRepo.On("GetContextEmbeddings", mock.Anything, contextID).Return(mockResults, nil)
	
	// Create test request
	req, _ := http.NewRequest("GET", "/api/v1/vectors/context/"+contextID, nil)
	
	// Perform the request
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	
	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)
	
	// Verify that our expectations were met
	mockRepo.AssertExpectations(t)
	
	// Verify response body
	var resp struct {
		Embeddings []*repository.Embedding `json:"embeddings"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp.Embeddings, 2)
}

func TestDeleteContextEmbeddingsHandler(t *testing.T) {
	// Create mock repository
	mockRepo := new(MockEmbeddingRepository)
	server := setupVectorTestServer(mockRepo)
	
	// Test data
	contextID := "context-123"
	
	// Set expectations
	mockRepo.On("DeleteContextEmbeddings", mock.Anything, contextID).Return(nil)
	
	// Create test request
	req, _ := http.NewRequest("DELETE", "/api/v1/vectors/context/"+contextID, nil)
	
	// Perform the request
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	
	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)
	
	// Verify that our expectations were met
	mockRepo.AssertExpectations(t)
	
	// Verify response body
	var resp struct {
		Status string `json:"status"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "deleted", resp.Status)
}

func TestGetSupportedModelsHandler(t *testing.T) {
	// Create mock repository
	mockRepo := new(MockEmbeddingRepository)
	server := setupVectorTestServer(mockRepo)
	
	// Test data
	models := []string{
		"test.openai.ada-002",
		"test.anthropic.claude",
		"test.mcp.small",
	}
	
	// Set expectations
	mockRepo.On("GetSupportedModels", mock.Anything).Return(models, nil)
	
	// Create test request
	req, _ := http.NewRequest("GET", "/api/v1/vectors/models", nil)
	
	// Perform the request
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	
	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)
	
	// Verify that our expectations were met
	mockRepo.AssertExpectations(t)
	
	// Verify response body
	var resp struct {
		Models []string `json:"models"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, models, resp.Models)
	assert.Len(t, resp.Models, 3)
}

func TestGetModelEmbeddingsHandler(t *testing.T) {
	// Create mock repository
	mockRepo := new(MockEmbeddingRepository)
	server := setupVectorTestServer(mockRepo)
	
	// Test data
	contextID := "context-123"
	modelID := "test.openai.ada-002"
	
	// Mock results
	mockResults := []*repository.Embedding{
		{
			ID:          "emb-1",
			ContextID:   contextID,
			ModelID:     modelID,
			ContentIndex: 1,
			Text:        "Test text 1",
		},
		{
			ID:          "emb-2",
			ContextID:   contextID,
			ModelID:     modelID,
			ContentIndex: 2,
			Text:        "Test text 2",
		},
	}
	
	// Set expectations
	mockRepo.On("GetEmbeddingsByModel", mock.Anything, contextID, modelID).Return(mockResults, nil)
	
	// Create test request
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/vectors/context/%s/model/%s", contextID, modelID), nil)
	
	// Perform the request
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	
	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)
	
	// Verify that our expectations were met
	mockRepo.AssertExpectations(t)
	
	// Verify response body
	var resp struct {
		Embeddings []*repository.Embedding `json:"embeddings"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp.Embeddings, 2)
	assert.Equal(t, modelID, resp.Embeddings[0].ModelID)
}

func TestDeleteModelEmbeddingsHandler(t *testing.T) {
	// Create mock repository
	mockRepo := new(MockEmbeddingRepository)
	server := setupVectorTestServer(mockRepo)
	
	// Test data
	contextID := "context-123"
	modelID := "test.openai.ada-002"
	
	// Set expectations
	mockRepo.On("DeleteModelEmbeddings", mock.Anything, contextID, modelID).Return(nil)
	
	// Create test request
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/vectors/context/%s/model/%s", contextID, modelID), nil)
	
	// Perform the request
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	
	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)
	
	// Verify that our expectations were met
	mockRepo.AssertExpectations(t)
	
	// Verify response body
	var resp struct {
		Status string `json:"status"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "deleted", resp.Status)
}
