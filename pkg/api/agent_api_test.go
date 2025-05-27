package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
	"github.com/S-Corkum/devops-mcp/pkg/repository/agent"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgentRepository mocks repository.AgentRepository
type MockAgentRepository struct {
	mock.Mock
}

// Core repository methods
func (m *MockAgentRepository) Create(ctx context.Context, agent *models.Agent) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *MockAgentRepository) Get(ctx context.Context, id string) (*models.Agent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}

func (m *MockAgentRepository) List(ctx context.Context, filter agent.Filter) ([]*models.Agent, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Agent), args.Error(1)
}

func (m *MockAgentRepository) Update(ctx context.Context, agent *models.Agent) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *MockAgentRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// API-specific methods
func (m *MockAgentRepository) CreateAgent(ctx context.Context, agent *models.Agent) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *MockAgentRepository) ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Agent), args.Error(1)
}

func (m *MockAgentRepository) GetAgentByID(ctx context.Context, id, tenantID string) (*models.Agent, error) {
	args := m.Called(ctx, id, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}

func (m *MockAgentRepository) UpdateAgent(ctx context.Context, agent *models.Agent) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *MockAgentRepository) DeleteAgent(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Helper to set up Gin and handler
func setupAgentAPI(repo repository.AgentRepository, withTenant bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if withTenant {
		r.Use(func(c *gin.Context) {
			c.Set("user", map[string]interface{}{"tenant_id": "tenant1"})
			c.Next()
		})
	}
	a := NewAgentAPI(repo)
	a.RegisterRoutes(r.Group("/"))
	return r
}

func TestCreateAgent_Success(t *testing.T) {
	repo := new(MockAgentRepository)
	repo.On("CreateAgent", mock.Anything, mock.AnythingOfType("*models.Agent")).Return(nil)

	r := setupAgentAPI(repo, true)
	w := httptest.NewRecorder()
	body := []byte(`{"name":"Test Agent"}`)
	req, _ := http.NewRequest("POST", "/agents", bytes.NewBuffer(body))

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
	repo.AssertCalled(t, "CreateAgent", mock.Anything, mock.AnythingOfType("*models.Agent"))
}

func TestCreateAgent_MissingTenant(t *testing.T) {
	repo := new(MockAgentRepository)
	r := setupAgentAPI(repo, false)
	w := httptest.NewRecorder()
	body := []byte(`{"name":"Test Agent"}`)
	req, _ := http.NewRequest("POST", "/agents", bytes.NewBuffer(body))

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListAgents_Success(t *testing.T) {
	repo := new(MockAgentRepository)
	agents := []*models.Agent{{ID: "a1", TenantID: "tenant1", Name: "Agent1"}}
	repo.On("ListAgents", mock.Anything, "tenant1").Return(agents, nil)

	r := setupAgentAPI(repo, true)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/agents", nil)

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateAgent_NotFound(t *testing.T) {
	repo := new(MockAgentRepository)
	repo.On("GetAgentByID", mock.Anything, "tenant1", "a1").Return((*models.Agent)(nil), errors.New("not found"))

	r := setupAgentAPI(repo, true)
	w := httptest.NewRecorder()
	body := []byte(`{"name":"Updated"}`)
	req, _ := http.NewRequest("PUT", "/agents/a1", bytes.NewBuffer(body))

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// More tests can be added for error cases, unauthorized, etc.
