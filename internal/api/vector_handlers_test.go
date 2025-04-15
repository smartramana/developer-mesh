package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	
	"github.com/S-Corkum/mcp-server/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock EmbeddingRepository for testing
type MockEmbeddingRepository struct {
	mock.Mock
}

func (m *MockEmbeddingRepository) StoreEmbedding(ctx interface{}, embedding *repository.Embedding) error {
	args := m.Called(ctx, embedding)
	embedding.ID = "embedding-test-id" // Set a test ID
	return args.Error(0)
}

func (m *MockEmbeddingRepository) SearchEmbeddings(ctx interface{}, queryVector []float32, contextID string, limit int) ([]repository.Embedding, error) {
	args := m.Called(ctx, queryVector, contextID, limit)
	return args.Get(0).([]repository.Embedding), args.Error(1)
}

func (m *MockEmbeddingRepository) GetContextEmbeddings(ctx interface{}, contextID string) ([]repository.Embedding, error) {
	args := m.Called(ctx, contextID)
	return args.Get(0).([]repository.Embedding), args.Error(1)
}

func (m *MockEmbeddingRepository) DeleteContextEmbeddings(ctx interface{}, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

// Set up a test server with the mock repository
func setupVectorTestServer(mockRepo *MockEmbeddingRepository) *Server {
	gin.SetMode(gin.TestMode)
	server := &Server{
		router:        gin.New(),
		embeddingRepo: mockRepo,
	}
	
	// Setup routes
	vectorRoutes := server.router.Group("/api/v1/vectors")
	{
		vectorRoutes.POST("/store", server.storeEmbedding)
		vectorRoutes.POST("/search", server.searchEmbeddings)
		vectorRoutes.GET("/context/:context_id", server.getContextEmbeddings)
		vectorRoutes.DELETE("/context/:context_id", server.deleteContextEmbeddings)
	}
	
	return server
}

func TestStoreEmbeddingHandler(t *testing.T) {
	// Create mock repository
	mockRepo := new(MockEmbeddingRepository)
	server := setupVectorTestServer(mockRepo)
	
	// Set expectations
	mockRepo.On("StoreEmbedding", mock.Anything, mock.AnythingOfType("*repository.Embedding")).Return(nil)
	
	// Create test request
	reqBody := StoreEmbeddingRequest{
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
	
	// Test data
	contextID := "context-123"
	queryVector := []float32{0.1, 0.2, 0.3}
	
	// Mock results
	mockResults := []repository.Embedding{
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
	mockRepo.On("SearchEmbeddings", mock.Anything, queryVector, contextID, 10).Return(mockResults, nil)
	
	// Create test request
	reqBody := SearchEmbeddingsRequest{
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
		Embeddings []repository.Embedding `json:"embeddings"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp.Embeddings, 2)
	assert.Equal(t, "emb-1", resp.Embeddings[0].ID)
}

func TestGetContextEmbeddingsHandler(t *testing.T) {
	// Create mock repository
	mockRepo := new(MockEmbeddingRepository)
	server := setupVectorTestServer(mockRepo)
	
	// Test data
	contextID := "context-123"
	
	// Mock results
	mockResults := []repository.Embedding{
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
		Embeddings []repository.Embedding `json:"embeddings"`
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
