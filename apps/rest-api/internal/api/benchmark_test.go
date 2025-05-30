package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/config"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"rest-api/internal/core"
)

// mockBenchmarkEngine implements the necessary methods for benchmarking
type mockBenchmarkEngine struct {
	contextManager core.ContextManagerInterface
}

func (m *mockBenchmarkEngine) Health() map[string]string {
	return map[string]string{"status": "healthy"}
}

func (m *mockBenchmarkEngine) GetAdapter(name string) (any, error) {
	return nil, nil
}

func (m *mockBenchmarkEngine) GetContextManager() core.ContextManagerInterface {
	return m.contextManager
}

func (m *mockBenchmarkEngine) RegisterAdapter(name string, adapter any) {}

func (m *mockBenchmarkEngine) SetContextManager(manager core.ContextManagerInterface) {
	m.contextManager = manager
}

func (m *mockBenchmarkEngine) Shutdown(ctx context.Context) error {
	return nil
}

// mockBenchmarkContextManager implements ContextManagerInterface for benchmarking
type mockBenchmarkContextManager struct{}

func (m *mockBenchmarkContextManager) CreateContext(ctx context.Context, context *models.Context) (*models.Context, error) {
	return context, nil
}

func (m *mockBenchmarkContextManager) GetContext(ctx context.Context, contextID string) (*models.Context, error) {
	return &models.Context{
		ID:        contextID,
		Name:      "test",
		AgentID:   "agent1",
		ModelID:   "model1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (m *mockBenchmarkContextManager) UpdateContext(ctx context.Context, contextID string, context *models.Context, options *models.ContextUpdateOptions) (*models.Context, error) {
	return context, nil
}

func (m *mockBenchmarkContextManager) DeleteContext(ctx context.Context, contextID string) error {
	return nil
}

func (m *mockBenchmarkContextManager) ListContexts(ctx context.Context, agentID, sessionID string, options map[string]any) ([]*models.Context, error) {
	return []*models.Context{
		{
			ID:        "ctx1",
			Name:      "test1",
			AgentID:   agentID,
			SessionID: sessionID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "ctx2",
			Name:      "test2",
			AgentID:   agentID,
			SessionID: sessionID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}, nil
}

func (m *mockBenchmarkContextManager) SearchInContext(ctx context.Context, contextID, query string) ([]models.ContextItem, error) {
	return []models.ContextItem{
		{
			ID:        "item1",
			Role:      "user",
			Content:   "test content",
			Timestamp: time.Now(),
		},
	}, nil
}

func (m *mockBenchmarkContextManager) SummarizeContext(ctx context.Context, contextID string) (string, error) {
	return "context summary", nil
}

// setupBenchmarkServer creates a server instance for benchmarking
func setupBenchmarkServer(_ *testing.B) *Server {
	// Create a mock database connection (not used in mock context manager)
	db := &sqlx.DB{}

	// Create logger and metrics
	logger := observability.NewLogger("benchmark")
	metrics := observability.NewMetricsClient()

	// Create mock engine with mock context manager
	contextManager := &mockBenchmarkContextManager{}
	engine := core.NewEngine(logger)
	engine.SetContextManager(contextManager)

	// Create server config
	cfg := DefaultConfig()
	cfg.EnableSwagger = false
	cfg.EnableCORS = false
	cfg.RateLimit.Enabled = false

	// Create config for vector DB (disabled)
	appConfig := &config.Config{
		Database: config.DatabaseConfig{
			Vector: map[string]any{
				"enabled": false,
			},
		},
	}

	// Create server
	server := NewServer(engine, cfg, db, metrics, appConfig)

	// Initialize routes
	server.Initialize(context.Background())

	return server
}

// BenchmarkHealthEndpoint benchmarks the health check endpoint
func BenchmarkHealthEndpoint(b *testing.B) {
	server := setupBenchmarkServer(b)
	gin.SetMode(gin.ReleaseMode)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/health", nil)
			server.router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
			}
		}
	})
}

// BenchmarkContextGet benchmarks context retrieval
func BenchmarkContextGet(b *testing.B) {
	server := setupBenchmarkServer(b)
	gin.SetMode(gin.ReleaseMode)

	// Set up authentication
	InitAPIKeys(map[string]string{
		"test-key": "admin",
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/v1/contexts/test-context", nil)
			req.Header.Set("X-API-Key", "test-key")
			server.router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
			}
		}
	})
}

// BenchmarkContextList benchmarks context listing
func BenchmarkContextList(b *testing.B) {
	server := setupBenchmarkServer(b)
	gin.SetMode(gin.ReleaseMode)

	// Set up authentication
	InitAPIKeys(map[string]string{
		"test-key": "admin",
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/v1/contexts?agent_id=agent1", nil)
			req.Header.Set("X-API-Key", "test-key")
			server.router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
			}
		}
	})
}

// BenchmarkMetricsEndpoint benchmarks the metrics endpoint
func BenchmarkMetricsEndpoint(b *testing.B) {
	server := setupBenchmarkServer(b)
	gin.SetMode(gin.ReleaseMode)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/metrics", nil)
			server.router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
			}
		}
	})
}

// BenchmarkAPIRoot benchmarks the API root endpoint
func BenchmarkAPIRoot(b *testing.B) {
	server := setupBenchmarkServer(b)
	gin.SetMode(gin.ReleaseMode)

	// Set up authentication
	InitAPIKeys(map[string]string{
		"test-key": "admin",
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/v1/", nil)
			req.Header.Set("X-API-Key", "test-key")
			server.router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
			}
		}
	})
}

// BenchmarkConcurrentRequests benchmarks handling concurrent requests
func BenchmarkConcurrentRequests(b *testing.B) {
	server := setupBenchmarkServer(b)
	gin.SetMode(gin.ReleaseMode)

	// Set up authentication
	InitAPIKeys(map[string]string{
		"test-key": "admin",
	})

	endpoints := []string{
		"/health",
		"/metrics",
		"/api/v1/",
		"/api/v1/contexts/test-context",
		"/api/v1/contexts?agent_id=agent1",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			endpoint := endpoints[i%len(endpoints)]
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", endpoint, nil)

			// Add auth header for protected endpoints
			if endpoint != "/health" && endpoint != "/metrics" {
				req.Header.Set("X-API-Key", "test-key")
			}

			server.router.ServeHTTP(w, req)

			if w.Code != http.StatusOK && w.Code != http.StatusUnauthorized {
				b.Errorf("Expected status 200 or 401 for %s, got %d", endpoint, w.Code)
			}

			i++
		}
	})
}

// BenchmarkMemoryAllocation benchmarks memory allocation for different operations
func BenchmarkMemoryAllocation(b *testing.B) {
	server := setupBenchmarkServer(b)
	gin.SetMode(gin.ReleaseMode)

	// Set up authentication
	InitAPIKeys(map[string]string{
		"test-key": "admin",
	})

	b.Run("HealthCheck", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/health", nil)
			server.router.ServeHTTP(w, req)
		}
	})

	b.Run("ContextGet", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/v1/contexts/test-context", nil)
			req.Header.Set("X-API-Key", "test-key")
			server.router.ServeHTTP(w, req)
		}
	})

	b.Run("ContextList", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/v1/contexts?agent_id=agent1", nil)
			req.Header.Set("X-API-Key", "test-key")
			server.router.ServeHTTP(w, req)
		}
	})
}
