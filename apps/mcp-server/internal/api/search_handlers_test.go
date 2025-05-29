package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/S-Corkum/devops-mcp/pkg/embedding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockSearchService is a mock implementation of the SearchService interface
type MockSearchService struct {
	mock.Mock
}

func (m *MockSearchService) Search(ctx context.Context, text string, options *embedding.SearchOptions) (*embedding.SearchResults, error) {
	args := m.Called(ctx, text, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*embedding.SearchResults), args.Error(1)
}

func (m *MockSearchService) SearchByVector(ctx context.Context, vector []float32, options *embedding.SearchOptions) (*embedding.SearchResults, error) {
	args := m.Called(ctx, vector, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*embedding.SearchResults), args.Error(1)
}

func (m *MockSearchService) SearchByContentID(ctx context.Context, contentID string, options *embedding.SearchOptions) (*embedding.SearchResults, error) {
	args := m.Called(ctx, contentID, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*embedding.SearchResults), args.Error(1)
}

func TestHandleSearch(t *testing.T) {
	// Create mock search service
	mockService := new(MockSearchService)
	
	// Create handler
	handler := NewSearchHandler(mockService)
	
	// Create test server
	router := http.NewServeMux()
	handler.RegisterRoutes(router)
	server := httptest.NewServer(router)
	defer server.Close()
	
	// Test case 1: POST request
	t.Run("POST request", func(t *testing.T) {
		// Setup mock response
		mockResults := &embedding.SearchResults{
			Results: []*embedding.SearchResult{
				{
					Content: &embedding.EmbeddingVector{
						ContentID:   "123",
						ContentType: "code",
						Metadata:    map[string]interface{}{"path": "main.go"},
					},
					Score: 0.95,
				},
			},
			Total:   1,
			HasMore: false,
		}
		
		// Configure mock to return our predefined results
		mockService.On("Search", mock.Anything, "test query", mock.Anything).Return(mockResults, nil)
		
		// Create request body
		reqBody := SearchRequest{
			Query:         "test query",
			ContentTypes:  []string{"code"},
			MinSimilarity: 0.8,
			Limit:         10,
		}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)
		
		// Make request
		resp, err := http.Post(server.URL+"/api/v1/search", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()
		
		// Check response
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		// Parse response
		var searchResp SearchResponse
		err = json.NewDecoder(resp.Body).Decode(&searchResp)
		require.NoError(t, err)
		
		// Verify response contents
		assert.Len(t, searchResp.Results, 1)
		assert.Equal(t, "123", searchResp.Results[0].Content.ContentID)
		assert.Equal(t, "code", searchResp.Results[0].Content.ContentType)
		assert.InDelta(t, 0.95, searchResp.Results[0].Score, 0.001)
		assert.Equal(t, "test query", searchResp.Query.Input)
		
		// Verify the mock was called correctly
		mockService.AssertExpectations(t)
	})
	
	// Test case 2: GET request
	t.Run("GET request", func(t *testing.T) {
		// Setup mock response
		mockResults := &embedding.SearchResults{
			Results: []*embedding.SearchResult{
				{
					Content: &embedding.EmbeddingVector{
						ContentID:   "456",
						ContentType: "issue",
						Metadata:    map[string]interface{}{"title": "Bug report"},
					},
					Score: 0.85,
				},
			},
			Total:   1,
			HasMore: false,
		}
		
		// Reset and configure mock
		mockService.ExpectedCalls = nil
		mockService.On("Search", mock.Anything, "bug report", mock.Anything).Return(mockResults, nil)
		
		// Make GET request
		resp, err := http.Get(server.URL + "/api/v1/search?query=bug%20report&content_types=issue&limit=5")
		require.NoError(t, err)
		defer resp.Body.Close()
		
		// Check response
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		// Parse response
		var searchResp SearchResponse
		err = json.NewDecoder(resp.Body).Decode(&searchResp)
		require.NoError(t, err)
		
		// Verify response contents
		assert.Len(t, searchResp.Results, 1)
		assert.Equal(t, "456", searchResp.Results[0].Content.ContentID)
		assert.Equal(t, "issue", searchResp.Results[0].Content.ContentType)
		assert.InDelta(t, 0.85, searchResp.Results[0].Score, 0.001)
		
		// Verify the mock was called with correct parameters
		mockService.AssertExpectations(t)
	})
}

func TestHandleSearchByVector(t *testing.T) {
	// Create mock search service
	mockService := new(MockSearchService)
	
	// Create handler
	handler := NewSearchHandler(mockService)
	
	// Create test server
	router := http.NewServeMux()
	handler.RegisterRoutes(router)
	server := httptest.NewServer(router)
	defer server.Close()
	
	// Setup mock response
	mockResults := &embedding.SearchResults{
		Results: []*embedding.SearchResult{
			{
				Content: &embedding.EmbeddingVector{
					ContentID:   "789",
					ContentType: "code",
					Metadata:    map[string]interface{}{"path": "utils.go"},
				},
				Score: 0.92,
			},
		},
		Total:   1,
		HasMore: false,
	}
	
	// Configure mock
	testVector := []float32{0.1, 0.2, 0.3}
	mockService.On("SearchByVector", mock.Anything, testVector, mock.Anything).Return(mockResults, nil)
	
	// Create request body
	reqBody := SearchByVectorRequest{
		Vector:        testVector,
		ContentTypes:  []string{"code"},
		MinSimilarity: 0.8,
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)
	
	// Make request
	resp, err := http.Post(server.URL+"/api/v1/search/vector", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	
	// Check response
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	// Parse response
	var searchResp SearchResponse
	err = json.NewDecoder(resp.Body).Decode(&searchResp)
	require.NoError(t, err)
	
	// Verify response contents
	assert.Len(t, searchResp.Results, 1)
	assert.Equal(t, "789", searchResp.Results[0].Content.ContentID)
	assert.Equal(t, "code", searchResp.Results[0].Content.ContentType)
	assert.InDelta(t, 0.92, searchResp.Results[0].Score, 0.001)
	
	// Verify the mock was called correctly
	mockService.AssertExpectations(t)
}

func TestHandleSearchSimilar(t *testing.T) {
	// Create mock search service
	mockService := new(MockSearchService)
	
	// Create handler
	handler := NewSearchHandler(mockService)
	
	// Create test server
	router := http.NewServeMux()
	handler.RegisterRoutes(router)
	server := httptest.NewServer(router)
	defer server.Close()
	
	// Setup mock response
	mockResults := &embedding.SearchResults{
		Results: []*embedding.SearchResult{
			{
				Content: &embedding.EmbeddingVector{
					ContentID:   "abc",
					ContentType: "code",
					Metadata:    map[string]interface{}{"path": "similar_file.go"},
				},
				Score: 0.88,
			},
		},
		Total:   1,
		HasMore: false,
	}
	
	// Test both GET and POST methods
	testCases := []struct {
		name    string
		method  string
		url     string
		body    interface{}
		contentID string
	}{
		{
			name:    "GET request",
			method:  "GET",
			url:     server.URL + "/api/v1/search/similar?content_id=code:original&limit=5",
			contentID: "code:original",
		},
		{
			name:    "POST request",
			method:  "POST",
			url:     server.URL + "/api/v1/search/similar",
			body:    map[string]interface{}{"content_id": "code:original", "options": map[string]interface{}{"limit": 5}},
			contentID: "code:original",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset and configure mock
			mockService.ExpectedCalls = nil
			mockService.On("SearchByContentID", mock.Anything, tc.contentID, mock.Anything).Return(mockResults, nil)
			
			var resp *http.Response
			var err error
			
			// Make the request
			if tc.method == "GET" {
				resp, err = http.Get(tc.url)
			} else {
				body, err := json.Marshal(tc.body)
				require.NoError(t, err)
				resp, err = http.Post(tc.url, "application/json", bytes.NewReader(body))
			}
			
			require.NoError(t, err)
			defer resp.Body.Close()
			
			// Check response
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			
			// Parse response
			var searchResp SearchResponse
			err = json.NewDecoder(resp.Body).Decode(&searchResp)
			require.NoError(t, err)
			
			// Verify response contents
			assert.Len(t, searchResp.Results, 1)
			assert.Equal(t, "abc", searchResp.Results[0].Content.ContentID)
			assert.Equal(t, "code", searchResp.Results[0].Content.ContentType)
			assert.InDelta(t, 0.88, searchResp.Results[0].Score, 0.001)
			
			// Verify the mock was called correctly
			mockService.AssertExpectations(t)
		})
	}
}
