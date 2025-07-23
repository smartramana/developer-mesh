package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAPIContextManager is a mock implementation of ContextManagerInterface for API tests
type MockAPIContextManager struct {
	mock.Mock
}

func (m *MockAPIContextManager) CreateContext(ctx context.Context, context *models.Context) (*models.Context, error) {
	args := m.Called(ctx, context)
	return args.Get(0).(*models.Context), args.Error(1)
}

func (m *MockAPIContextManager) GetContext(ctx context.Context, contextID string) (*models.Context, error) {
	args := m.Called(ctx, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Context), args.Error(1)
}

func (m *MockAPIContextManager) UpdateContext(ctx context.Context, contextID string, context *models.Context, options *models.ContextUpdateOptions) (*models.Context, error) {
	args := m.Called(ctx, contextID, context, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Context), args.Error(1)
}

func (m *MockAPIContextManager) DeleteContext(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

func (m *MockAPIContextManager) ListContexts(ctx context.Context, agentID, sessionID string, options map[string]interface{}) ([]*models.Context, error) {
	args := m.Called(ctx, agentID, sessionID, options)
	return args.Get(0).([]*models.Context), args.Error(1)
}

func (m *MockAPIContextManager) SearchInContext(ctx context.Context, contextID, query string) ([]models.ContextItem, error) {
	args := m.Called(ctx, contextID, query)
	return args.Get(0).([]models.ContextItem), args.Error(1)
}

func (m *MockAPIContextManager) SummarizeContext(ctx context.Context, contextID string) (string, error) {
	args := m.Called(ctx, contextID)
	return args.String(0), args.Error(1)
}

func TestUpdateContext(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Define test cases
	testCases := []struct {
		name               string
		contextID          string
		requestBody        string
		setupMocks         func(mockContextManager *MockAPIContextManager)
		expectedStatusCode int
		expectedResponse   string
	}{
		{
			name:      "successful update with new content",
			contextID: "test-id-1",
			requestBody: `{"content": [
				{"role": "user", "content": "Hello, how are you?", "tokens": 5},
				{"role": "assistant", "content": "I'm doing well, thank you!", "tokens": 6}
			]}`,
			setupMocks: func(mockContextManager *MockAPIContextManager) {
				existingContext := &models.Context{
					ID:        "test-id-1",
					AgentID:   "test-agent",
					ModelID:   "gpt-4",
					Content:   []models.ContextItem{},
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				newContent := []models.ContextItem{
					{
						Role:    "user",
						Content: "Hello, how are you?",
						Tokens:  5,
					},
					{
						Role:    "assistant",
						Content: "I'm doing well, thank you!",
						Tokens:  6,
					},
				}

				updatedContext := &models.Context{
					ID:        "test-id-1",
					AgentID:   "test-agent",
					ModelID:   "gpt-4",
					Content:   newContent,
					CreatedAt: existingContext.CreatedAt,
					UpdatedAt: existingContext.UpdatedAt,
				}

				mockContextManager.On("GetContext", mock.Anything, "test-id-1").Return(existingContext, nil)
				mockContextManager.On("UpdateContext", mock.Anything, "test-id-1", mock.Anything, mock.Anything).Return(updatedContext, nil)
			},
			expectedStatusCode: http.StatusOK,
			expectedResponse:   "",
		},
		{
			name:      "context not found",
			contextID: "nonexistent-id",
			requestBody: `{"content": [
				{"role": "user", "content": "Hello", "tokens": 1}
			]}`,
			setupMocks: func(mockContextManager *MockAPIContextManager) {
				mockContextManager.On("GetContext", mock.Anything, "nonexistent-id").Return(nil, assert.AnError)
			},
			expectedStatusCode: http.StatusNotFound,
			expectedResponse:   "context not found",
		},
		{
			name:        "invalid request body",
			contextID:   "test-id-1",
			requestBody: `{"invalid json`,
			setupMocks: func(mockContextManager *MockAPIContextManager) {
				// No mocks needed as request parsing will fail
			},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "invalid request body",
		},
		{
			name:      "update failure",
			contextID: "error-id",
			requestBody: `{"content": [
				{"role": "user", "content": "Content that causes error", "tokens": 5}
			]}`,
			setupMocks: func(mockContextManager *MockAPIContextManager) {
				existingContext := &models.Context{
					ID:        "error-id",
					AgentID:   "test-agent",
					ModelID:   "gpt-4",
					Content:   []models.ContextItem{},
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				mockContextManager.On("GetContext", mock.Anything, "error-id").Return(existingContext, nil)
				mockContextManager.On("UpdateContext", mock.Anything, "error-id", mock.Anything, mock.Anything).Return(nil, assert.AnError)
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedResponse:   "failed to update context",
		},
	}

	// Execute test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new mock context manager for each test
			mockContextManager := new(MockAPIContextManager)

			// Setup mocks for this test
			tc.setupMocks(mockContextManager)

			// Create API and router
			api := NewMCPAPI(mockContextManager)
			router := gin.Default()

			// Register routes
			group := router.Group("/api/v1/mcp")
			{
				group.PUT("/context/:id", api.updateContext)
			}

			// Create request
			req, _ := http.NewRequest("PUT", "/api/v1/mcp/context/"+tc.contextID, strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", "application/json")

			// Perform request
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			// Assert status code
			assert.Equal(t, tc.expectedStatusCode, resp.Code)

			// Assert response content if expected
			if tc.expectedResponse != "" {
				assert.Contains(t, resp.Body.String(), tc.expectedResponse)
			}

			// Verify that all expected mock calls were made
			mockContextManager.AssertExpectations(t)
		})
	}
}
