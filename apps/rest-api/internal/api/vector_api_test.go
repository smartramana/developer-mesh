package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"rest-api/internal/repository"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	api "rest-api/internal/api"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)



func setupVectorAPI() (*gin.Engine, *MockEmbeddingRepository) {
	gin.SetMode(gin.TestMode)
	repo := new(MockEmbeddingRepository)
	logger := observability.NewLogger("test")
	apiHandler := api.NewVectorAPI(repo, logger)
	r := gin.New()
	apiHandler.RegisterRoutes(r.Group("/api/v1"))
	return r, repo
}

func TestStoreEmbedding_Success(t *testing.T) {
	r, repo := setupVectorAPI()
	// Use mock.Anything to avoid type compatibility issues with aliases
	repo.On("StoreEmbedding", mock.Anything, mock.Anything).Return(nil)
	body := map[string]interface{}{
		"context_id": "ctx1",
		"content_index": 1,
		"text": "sample",
		"embedding": []float32{0.1, 0.2},
		"model_id": "m1",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/v1/vectors/store", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	// Use a flexible assertion to match our mock implementation
	repo.AssertCalled(t, "StoreEmbedding", mock.Anything, mock.Anything)
}

func TestStoreEmbedding_BadRequest(t *testing.T) {
	r, _ := setupVectorAPI()
	// Missing required fields
	body := map[string]interface{}{"context_id": "ctx1"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/v1/vectors/store", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearchEmbeddings_Success(t *testing.T) {
	r, repo := setupVectorAPI()
	embs := []*repository.Embedding{{ContextID: "ctx1", ContentIndex: 1, Text: "t", Embedding: []float32{0.1}, ModelID: "m1"}}
	repo.On("SearchEmbeddings", mock.Anything, mock.Anything, "ctx1", "m1", 1, float64(0.5)).Return(embs, nil)
	body := map[string]interface{}{
		"context_id": "ctx1",
		"query_embedding": []float32{0.1},
		"limit": 1,
		"model_id": "m1",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/v1/vectors/search", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Contains(t, resp, "embeddings")
}

func TestGetContextEmbeddings_Success(t *testing.T) {
	r, repo := setupVectorAPI()
	embs := []*repository.Embedding{{ContextID: "ctx1"}}
	repo.On("GetContextEmbeddings", mock.Anything, "ctx1").Return(embs, nil)
	req, _ := http.NewRequest("GET", "/api/v1/vectors/context/ctx1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteContextEmbeddings_Success(t *testing.T) {
	r, repo := setupVectorAPI()
	repo.On("DeleteContextEmbeddings", mock.Anything, "ctx1").Return(nil)
	req, _ := http.NewRequest("DELETE", "/api/v1/vectors/context/ctx1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetSupportedModels_Success(t *testing.T) {
	r, repo := setupVectorAPI()
	repo.On("GetSupportedModels", mock.Anything).Return([]string{"m1", "m2"}, nil)
	req, _ := http.NewRequest("GET", "/api/v1/vectors/models", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetModelEmbeddings_Success(t *testing.T) {
	r, repo := setupVectorAPI()
	embs := []*repository.Embedding{{ContextID: "ctx1", ModelID: "m1"}}
	repo.On("GetEmbeddingsByModel", mock.Anything, "ctx1", "m1").Return(embs, nil)
	req, _ := http.NewRequest("GET", "/api/v1/vectors/context/ctx1/model/m1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteModelEmbeddings_Success(t *testing.T) {
	r, repo := setupVectorAPI()
	repo.On("DeleteModelEmbeddings", mock.Anything, "ctx1", "m1").Return(nil)
	req, _ := http.NewRequest("DELETE", "/api/v1/vectors/context/ctx1/model/m1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// Add more error and edge case tests as needed.
