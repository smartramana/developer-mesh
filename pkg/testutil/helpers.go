// Package testutil provides common test utilities and helpers
package testutil

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestConfig holds test configuration
type TestConfig struct {
	DatabaseDSN   string
	RedisAddr     string
	APIBaseURL    string
	TestTenantID  string
	TestAuthToken string
}

// LoadTestConfig loads test configuration from environment
func LoadTestConfig() *TestConfig {
	return &TestConfig{
		DatabaseDSN:   getEnvOrDefault("TEST_DATABASE_DSN", "postgresql://test:test@localhost:5432/test?sslmode=disable"),
		RedisAddr:     getEnvOrDefault("TEST_REDIS_ADDR", "localhost:6379"),
		APIBaseURL:    getEnvOrDefault("TEST_API_BASE_URL", "http://localhost:8081"),
		TestTenantID:  getEnvOrDefault("TEST_TENANT_ID", "test-tenant-"+uuid.New().String()),
		TestAuthToken: getEnvOrDefault("TEST_AUTH_TOKEN", "test-token"),
	}
}

// TestDatabase provides test database utilities
type TestDatabase struct {
	DB *sql.DB
}

// NewTestDatabase creates a new test database connection
func NewTestDatabase(t *testing.T, dsn string) *TestDatabase {
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)

	// Verify connection
	err = db.Ping()
	require.NoError(t, err)

	return &TestDatabase{DB: db}
}

// Cleanup cleans up test data
func (tdb *TestDatabase) Cleanup(t *testing.T, tenantID string) {
	queries := []string{
		fmt.Sprintf("DELETE FROM mcp.contexts WHERE agent_id LIKE '%s-%%'", tenantID),
		fmt.Sprintf("DELETE FROM mcp.agents WHERE tenant_id = '%s'", tenantID),
		fmt.Sprintf("DELETE FROM mcp.models WHERE tenant_id = '%s'", tenantID),
	}

	for _, query := range queries {
		_, err := tdb.DB.Exec(query)
		if err != nil {
			t.Logf("Cleanup query failed: %v", err)
		}
	}
}

// Close closes the database connection
func (tdb *TestDatabase) Close() {
	if tdb.DB != nil {
		if err := tdb.DB.Close(); err != nil {
			// Test helper - log but don't fail
			_ = err
		}
	}
}

// HTTPClient provides a test HTTP client
type HTTPClient struct {
	BaseURL   string
	AuthToken string
	TenantID  string
	Client    *http.Client
}

// NewHTTPClient creates a new test HTTP client
func NewHTTPClient(config *TestConfig) *HTTPClient {
	return &HTTPClient{
		BaseURL:   config.APIBaseURL,
		AuthToken: config.TestAuthToken,
		TenantID:  config.TestTenantID,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Request makes an HTTP request with standard headers
func (c *HTTPClient) Request(method, path string, body any) (*http.Response, error) {
	var bodyReader *bytes.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	var req *http.Request
	var err error
	if bodyReader != nil {
		req, err = http.NewRequest(method, c.BaseURL+path, bodyReader)
	} else {
		req, err = http.NewRequest(method, c.BaseURL+path, nil)
	}
	if err != nil {
		return nil, err
	}

	// Set standard headers
	req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	req.Header.Set("X-Tenant-ID", c.TenantID)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.Client.Do(req)
}

// TestServer provides a test Gin server
type TestServer struct {
	Router *gin.Engine
}

// NewTestServer creates a new test server
func NewTestServer() *TestServer {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(gin.Recovery())

	return &TestServer{
		Router: router,
	}
}

// AddAuthMiddleware adds test authentication middleware
func (ts *TestServer) AddAuthMiddleware() {
	ts.Router.Use(func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token != "" {
			c.Set("user", map[string]any{
				"id":        "test-user",
				"tenant_id": c.GetHeader("X-Tenant-ID"),
			})
		}
		c.Next()
	})
}

// ServeHTTP implements http.Handler
func (ts *TestServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ts.Router.ServeHTTP(w, r)
}

// TestFixtures provides test data fixtures
type TestFixtures struct {
	Contexts []Context
	Agents   []Agent
	Models   []Model
}

// Context represents a test context
type Context struct {
	ID        string         `json:"id"`
	AgentID   string         `json:"agent_id"`
	ModelID   string         `json:"model_id"`
	MaxTokens int            `json:"max_tokens"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"created_at"`
}

// Agent represents a test agent
type Agent struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Config      map[string]any `json:"config"`
}

// Model represents a test model
type Model struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	ModelType string `json:"model_type"`
}

// LoadFixtures loads test fixtures
func LoadFixtures(tenantID string) *TestFixtures {
	return &TestFixtures{
		Contexts: []Context{
			{
				ID:        uuid.New().String(),
				AgentID:   "fixture-agent-1",
				ModelID:   "fixture-model-1",
				MaxTokens: 4000,
				Metadata:  map[string]any{"fixture": true},
				CreatedAt: time.Now(),
			},
		},
		Agents: []Agent{
			{
				ID:          uuid.New().String(),
				TenantID:    tenantID,
				Name:        "fixture-agent-1",
				Description: "Test fixture agent",
				Config:      map[string]any{"model": "gpt-4"},
			},
		},
		Models: []Model{
			{
				ID:        uuid.New().String(),
				TenantID:  tenantID,
				Name:      "fixture-model-1",
				Provider:  "openai",
				ModelType: "chat",
			},
		},
	}
}

// AssertJSONResponse asserts JSON response matches expected structure
func AssertJSONResponse(t *testing.T, w *httptest.ResponseRecorder, expected any) {
	var actual any
	err := json.Unmarshal(w.Body.Bytes(), &actual)
	require.NoError(t, err)

	expectedJSON, err := json.Marshal(expected)
	require.NoError(t, err)

	actualJSON, err := json.Marshal(actual)
	require.NoError(t, err)

	require.JSONEq(t, string(expectedJSON), string(actualJSON))
}

// WaitForCondition waits for a condition to be true
func WaitForCondition(t *testing.T, timeout time.Duration, check func() bool, message string) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("Timeout waiting for condition: %s", message)
}

// MockClock provides a mock clock for testing
type MockClock struct {
	CurrentTime time.Time
}

// Now returns the mock current time
func (mc *MockClock) Now() time.Time {
	return mc.CurrentTime
}

// Advance advances the mock clock
func (mc *MockClock) Advance(d time.Duration) {
	mc.CurrentTime = mc.CurrentTime.Add(d)
}

// ContextWithTimeout creates a context with timeout and cleanup
func ContextWithTimeout(t *testing.T, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(func() {
		cancel()
	})
	return ctx, cancel
}

// RandomString generates a random string of given length
func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// GenerateLargePayload generates a large payload for testing
func GenerateLargePayload(sizeBytes int) []byte {
	payload := make([]byte, sizeBytes)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	return payload
}

// Helper functions

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// TestMetrics provides test metrics collection
type TestMetrics struct {
	Counters map[string]int64
	Gauges   map[string]float64
	mu       sync.RWMutex
}

// NewTestMetrics creates a new test metrics collector
func NewTestMetrics() *TestMetrics {
	return &TestMetrics{
		Counters: make(map[string]int64),
		Gauges:   make(map[string]float64),
	}
}

// IncrCounter increments a counter
func (tm *TestMetrics) IncrCounter(name string, value int64) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.Counters[name] += value
}

// SetGauge sets a gauge value
func (tm *TestMetrics) SetGauge(name string, value float64) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.Gauges[name] = value
}

// GetCounter gets a counter value
func (tm *TestMetrics) GetCounter(name string) int64 {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.Counters[name]
}

// GetGauge gets a gauge value
func (tm *TestMetrics) GetGauge(name string) float64 {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.Gauges[name]
}
