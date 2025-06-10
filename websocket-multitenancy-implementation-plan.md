# WebSocket and Multi-Tenancy Implementation Plan

## Executive Summary

This plan addresses three critical issues identified during functional testing:
1. WebSocket tests failing due to incorrect service targeting
2. Tenant isolation not being properly enforced
3. Test architecture mixing protocols and endpoints

The solution leverages the coder/websocket library's advanced features and follows DevOps MCP best practices from CLAUDE.md.

## Current State Analysis

### Issues Identified

1. **WebSocket Service Routing**
   - Tests attempting to connect to WebSocket endpoints on REST API (port 8081)
   - WebSocket functionality correctly implemented in MCP server (port 8080)
   - No service discovery or routing configuration in tests

2. **Tenant Isolation Failures**
   - API keys hardcoded to "dev-tenant" regardless of configuration
   - No tenant context propagation through the request lifecycle
   - Missing tenant validation at resource access points

3. **Test Architecture Problems**
   - Mixed protocol tests in single test files
   - No clear separation between WebSocket and REST tests
   - Missing integration test patterns for cross-service scenarios

### Current Strengths ✅
- Using coder/websocket library with nhooyr's minimal design
- JWT/API key authentication implemented
- HMAC signatures for message integrity
- Rate limiting infrastructure in place
- Binary protocol with MessagePack for performance

## Implementation Plan

### Phase 1: Immediate Fixes (1-2 days)

#### 1.1 Fix Test Service Routing (Following CLAUDE.md patterns)

**File**: `test/functional/shared/config.go`
```go
package shared

import (
	"os"
	"fmt"
	"time"
	
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// ServiceConfig holds test service endpoints
type ServiceConfig struct {
	WebSocketURL  string
	RestAPIURL    string
	MockServerURL string
	logger        observability.Logger
}

// GetTestConfig returns test configuration following CLAUDE.md patterns
func GetTestConfig() *ServiceConfig {
	logger := observability.NewLogger("test-config")
	
	config := &ServiceConfig{
		WebSocketURL:  getEnvOrDefault("MCP_WEBSOCKET_URL", "ws://localhost:8080/ws"),
		RestAPIURL:    getEnvOrDefault("REST_API_URL", "http://localhost:8081"),
		MockServerURL: getEnvOrDefault("MOCKSERVER_URL", "http://localhost:8082"),
		logger:        logger,
	}
	
	// Log configuration for debugging (following CLAUDE.md)
	logger.Info("Test configuration loaded", map[string]interface{}{
		"websocket_url": config.WebSocketURL,
		"rest_api_url":  config.RestAPIURL,
		"mock_server":   config.MockServerURL,
	})
	
	return config
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
```

**File**: `test/functional/websocket/websocket_suite_test.go`
```go
package websocket_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
	
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	
	"functional-tests/shared"
	"github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

var (
	config *shared.ServiceConfig
	wsURL  string
)

func TestWebSocket(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "WebSocket Suite")
}

var _ = BeforeSuite(func() {
	config = shared.GetTestConfig()
	wsURL = config.WebSocketURL
	
	// Verify WebSocket endpoint health (following CLAUDE.md patterns)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	healthURL := strings.Replace(wsURL, "ws://", "http://", 1)
	healthURL = strings.Replace(healthURL, "/ws", "/health", 1)
	
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	Expect(err).NotTo(HaveOccurred())
	
	resp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(200))
})
```

#### 1.2 Fix Tenant Context Propagation (Using Repository Pattern)

**File**: `apps/rest-api/internal/api/middleware.go`
```go
// extractTenantContext follows CLAUDE.md adapter pattern
func (s *Server) extractTenantContext(c *gin.Context) {
	// Get auth context from middleware
	authVal, exists := c.Get("auth")
	if !exists {
		s.logger.Warn("No auth context found", map[string]interface{}{
			"path": c.Request.URL.Path,
		})
		return
	}
	
	// Type assertion with safety check
	authCtx, ok := authVal.(*auth.Context)
	if !ok {
		s.logger.Error("Invalid auth context type", map[string]interface{}{
			"type": fmt.Sprintf("%T", authVal),
		})
		return
	}
	
	// Set tenant context for downstream handlers
	c.Set("tenant_id", authCtx.TenantID)
	c.Set("auth_context", authCtx)
	
	// Add tenant header for tracing
	c.Header("X-Tenant-ID", authCtx.TenantID)
	
	s.logger.Debug("Tenant context extracted", map[string]interface{}{
		"tenant_id": authCtx.TenantID,
		"user_id":   authCtx.UserID,
		"path":      c.Request.URL.Path,
	})
}
```

**File**: `apps/rest-api/internal/api/model_api.go`
```go
// createModel follows repository pattern from CLAUDE.md
func (api *ModelAPI) createModel(c *gin.Context) {
	// Extract tenant context
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		responses.Error(c, http.StatusBadRequest, "tenant context required")
		return
	}
	
	// Parse request
	var req models.Model
	if err := c.ShouldBindJSON(&req); err != nil {
		api.logger.Warn("Invalid request payload", map[string]interface{}{
			"error": err.Error(),
			"tenant_id": tenantID,
		})
		responses.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	
	// Enforce tenant isolation
	req.TenantID = tenantID
	
	// Use repository adapter pattern
	ctx := context.WithValue(c.Request.Context(), "tenant_id", tenantID)
	created, err := api.repository.Create(ctx, &req)
	if err != nil {
		api.logger.Error("Failed to create model", map[string]interface{}{
			"error": err.Error(),
			"tenant_id": tenantID,
		})
		responses.Error(c, http.StatusInternalServerError, "failed to create model")
		return
	}
	
	responses.JSON(c, http.StatusCreated, created)
}
```

#### 1.3 Reorganize Test Structure (Following Go Workspace Pattern)

```bash
# Following CLAUDE.md test organization
test/
├── functional/
│   ├── websocket/
│   │   ├── websocket_suite_test.go
│   │   ├── connection_test.go      # coder/websocket connection tests
│   │   ├── auth_test.go            # JWT/API key auth tests
│   │   ├── binary_protocol_test.go # MessagePack protocol tests
│   │   ├── rate_limit_test.go      # Rate limiting tests
│   │   └── tenant_isolation_test.go # Multi-tenant tests
│   ├── rest/
│   │   ├── rest_suite_test.go
│   │   ├── model_test.go           # Model CRUD with adapters
│   │   ├── agent_test.go           # Agent operations
│   │   ├── auth_test.go            # REST auth tests
│   │   └── tenant_test.go          # Tenant isolation
│   ├── integration/
│   │   ├── suite_test.go
│   │   ├── event_flow_test.go      # SQS event integration
│   │   └── cross_service_test.go   # WebSocket + REST
│   └── shared/
│       ├── config.go               # Test configuration
│       ├── clients.go              # HTTP/WS client wrappers
│       ├── fixtures.go             # Test data factories
│       └── assertions.go           # Custom Gomega matchers
```

### Phase 2: Leverage coder/websocket Features (3-5 days)

#### 2.1 Enhanced WebSocket Implementation

**File**: `apps/mcp-server/internal/api/websocket/connection.go`
```go
package websocket

import (
	"context"
	"sync"
	"time"
	
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/vmihailenco/msgpack/v5"
	
	"github.com/S-Corkum/devops-mcp/pkg/models/websocket"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// Connection represents a WebSocket connection with coder/websocket
type Connection struct {
	ID         string
	TenantID   string
	UserID     string
	conn       *websocket.Conn
	hub        *Hub
	send       chan []byte
	
	// coder/websocket specific features
	ctx        context.Context
	cancel     context.CancelFunc
	
	// Rate limiting
	limiter    *TenantRateLimiter
	
	// Metrics
	metrics    *ConnectionMetrics
	
	// Binary protocol with MessagePack
	encoder    *msgpack.Encoder
	decoder    *msgpack.Decoder
	
	logger     observability.Logger
	mu         sync.RWMutex
}

// readPump leverages coder/websocket's context support
func (c *Connection) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.cancel()
	}()
	
	// Configure coder/websocket read limits
	c.conn.SetReadLimit(c.hub.config.MaxMessageSize)
	
	for {
		// Use context for graceful shutdown
		messageType, data, err := c.conn.Read(c.ctx)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				c.logger.Info("Connection closed normally", map[string]interface{}{
					"connection_id": c.ID,
					"tenant_id": c.TenantID,
				})
			} else {
				c.logger.Error("Read error", map[string]interface{}{
					"error": err.Error(),
					"connection_id": c.ID,
				})
			}
			return
		}
		
		// Handle binary messages with MessagePack
		if messageType == websocket.MessageBinary {
			if err := c.handleBinaryMessage(data); err != nil {
				c.logger.Error("Failed to handle binary message", map[string]interface{}{
					"error": err.Error(),
					"size": len(data),
				})
			}
		} else {
			// Handle text messages (JSON)
			if err := c.handleTextMessage(data); err != nil {
				c.logger.Error("Failed to handle text message", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}
	}
}

// writePump uses coder/websocket's efficient write methods
func (c *Connection) writePump() {
	ticker := time.NewTicker(c.hub.config.PingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()
	
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// Channel closed
				c.conn.Close(websocket.StatusNormalClosure, "")
				return
			}
			
			// Write with context timeout
			ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
			err := c.conn.Write(ctx, websocket.MessageBinary, message)
			cancel()
			
			if err != nil {
				c.logger.Error("Write error", map[string]interface{}{
					"error": err.Error(),
					"connection_id": c.ID,
				})
				return
			}
			
			// Update metrics
			c.metrics.MessagesSent++
			c.metrics.BytesSent += int64(len(message))
			
		case <-ticker.C:
			// Send ping using coder/websocket's Ping method
			ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
			err := c.conn.Ping(ctx)
			cancel()
			
			if err != nil {
				c.logger.Debug("Ping failed", map[string]interface{}{
					"error": err.Error(),
					"connection_id": c.ID,
				})
				return
			}
			
		case <-c.ctx.Done():
			return
		}
	}
}

// SendJSON uses coder/websocket's wsjson helper
func (c *Connection) SendJSON(v interface{}) error {
	ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
	defer cancel()
	
	return wsjson.Write(ctx, c.conn, v)
}

// SendBinary uses MessagePack for efficient binary encoding
func (c *Connection) SendBinary(msg *websocket.Message) error {
	data, err := msgpack.Marshal(msg)
	if err != nil {
		return err
	}
	
	select {
	case c.send <- data:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
		return ErrConnectionBufferFull
	}
}
```

#### 2.2 Advanced Origin Validation with coder/websocket

**File**: `apps/mcp-server/internal/api/websocket/server.go`
```go
// HandleWebSocket uses coder/websocket's Accept options
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract auth context
	authCtx, err := s.authenticateWebSocket(r)
	if err != nil {
		s.logger.Warn("WebSocket auth failed", map[string]interface{}{
			"error": err.Error(),
			"ip": r.RemoteAddr,
		})
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	
	// Configure accept options
	acceptOptions := &websocket.AcceptOptions{
		OriginPatterns: s.config.Security.AllowedOrigins,
		CompressionMode: websocket.CompressionContextTakeover,
		CompressionThreshold: 512, // Compress messages > 512 bytes
	}
	
	// Accept connection with coder/websocket
	conn, err := websocket.Accept(w, r, acceptOptions)
	if err != nil {
		s.logger.Error("Failed to accept WebSocket", map[string]interface{}{
			"error": err.Error(),
			"tenant_id": authCtx.TenantID,
		})
		return
	}
	
	// Create connection context
	ctx, cancel := context.WithCancel(r.Context())
	
	// Create connection object
	connection := &Connection{
		ID:       generateConnectionID(),
		TenantID: authCtx.TenantID,
		UserID:   authCtx.UserID,
		conn:     conn,
		hub:      s.hub,
		send:     make(chan []byte, 256),
		ctx:      ctx,
		cancel:   cancel,
		limiter:  s.rateLimiter.GetTenantLimiter(authCtx.TenantID),
		logger:   s.logger,
	}
	
	// Check tenant connection limits
	if err := s.tenantManager.AddConnection(connection); err != nil {
		s.logger.Warn("Tenant connection limit reached", map[string]interface{}{
			"tenant_id": authCtx.TenantID,
			"error": err.Error(),
		})
		conn.Close(websocket.StatusTryAgainLater, "Connection limit reached")
		return
	}
	
	// Register connection
	s.hub.register <- connection
	
	// Start connection pumps
	go connection.writePump()
	go connection.readPump()
	
	s.logger.Info("WebSocket connection established", map[string]interface{}{
		"connection_id": connection.ID,
		"tenant_id": authCtx.TenantID,
		"user_id": authCtx.UserID,
	})
}
```

### Phase 3: Multi-Tenant Architecture (1-2 weeks)

#### 3.1 Tenant-Aware Connection Pool

**File**: `apps/mcp-server/internal/api/websocket/tenant_manager.go`
```go
package websocket

import (
	"context"
	"fmt"
	"sync"
	"time"
	
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/cache"
)

// TenantConnectionManager manages connections per tenant
type TenantConnectionManager struct {
	tenants map[string]*TenantContext
	mu      sync.RWMutex
	
	// Configuration
	defaultLimits *TenantLimits
	
	// Dependencies
	cache   cache.Cache
	logger  observability.Logger
	metrics *TenantMetricsCollector
}

// TenantContext holds tenant-specific connection state
type TenantContext struct {
	TenantID    string
	Connections sync.Map // map[string]*Connection
	Limits      *TenantLimits
	Metrics     *TenantMetrics
	
	// Rate limiting
	rateLimiter *TenantRateLimiter
	
	// Circuit breaker for tenant
	circuitBreaker *CircuitBreaker
}

// TenantLimits defines resource limits per tenant
type TenantLimits struct {
	MaxConnections    int     `json:"max_connections"`
	MaxMessageRate    float64 `json:"max_message_rate"`
	MaxBandwidth      int64   `json:"max_bandwidth"`
	MaxMessageSize    int64   `json:"max_message_size"`
	MaxConcurrency    int     `json:"max_concurrency"`
	BurstMultiplier   float64 `json:"burst_multiplier"`
}

// NewTenantConnectionManager creates a new tenant manager
func NewTenantConnectionManager(
	cache cache.Cache,
	logger observability.Logger,
	metrics *TenantMetricsCollector,
) *TenantConnectionManager {
	return &TenantConnectionManager{
		tenants: make(map[string]*TenantContext),
		defaultLimits: &TenantLimits{
			MaxConnections:  100,
			MaxMessageRate:  1000, // msgs/sec
			MaxBandwidth:    10 * 1024 * 1024, // 10MB/s
			MaxMessageSize:  1024 * 1024, // 1MB
			MaxConcurrency:  10,
			BurstMultiplier: 1.5,
		},
		cache:   cache,
		logger:  logger,
		metrics: metrics,
	}
}

// AddConnection adds a connection for a tenant
func (tcm *TenantConnectionManager) AddConnection(conn *Connection) error {
	tenant := tcm.getOrCreateTenant(conn.TenantID)
	
	// Check connection limit
	count := tcm.getConnectionCount(conn.TenantID)
	if count >= tenant.Limits.MaxConnections {
		tcm.metrics.RecordLimitExceeded(conn.TenantID, "connections")
		return fmt.Errorf("tenant %s has reached connection limit (%d)", 
			conn.TenantID, tenant.Limits.MaxConnections)
	}
	
	// Add connection
	tenant.Connections.Store(conn.ID, conn)
	tenant.Metrics.ActiveConnections++
	
	// Update metrics
	tcm.metrics.UpdateConnectionCount(conn.TenantID, count+1)
	
	tcm.logger.Debug("Connection added", map[string]interface{}{
		"tenant_id": conn.TenantID,
		"connection_id": conn.ID,
		"total_connections": count + 1,
	})
	
	return nil
}

// ValidateResourceAccess checks tenant boundaries
func (tcm *TenantConnectionManager) ValidateResourceAccess(
	ctx context.Context,
	tenantID string,
	resourceTenantID string,
	operation string,
) error {
	if tenantID != resourceTenantID {
		// Log security violation
		tcm.logger.Error("Tenant isolation violation", map[string]interface{}{
			"requesting_tenant": tenantID,
			"resource_tenant": resourceTenantID,
			"operation": operation,
			"trace_id": ctx.Value("trace_id"),
		})
		
		// Record metric
		tcm.metrics.RecordSecurityViolation(tenantID, "cross_tenant_access")
		
		// Check if this tenant has too many violations
		violations := tcm.getViolationCount(tenantID)
		if violations > 10 {
			// Circuit break the tenant
			tenant := tcm.getTenant(tenantID)
			if tenant != nil && tenant.circuitBreaker != nil {
				tenant.circuitBreaker.Open()
			}
		}
		
		return fmt.Errorf("tenant %s cannot access resources of tenant %s", 
			tenantID, resourceTenantID)
	}
	
	return nil
}

// getOrCreateTenant gets or creates a tenant context
func (tcm *TenantConnectionManager) getOrCreateTenant(tenantID string) *TenantContext {
	tcm.mu.RLock()
	tenant, exists := tcm.tenants[tenantID]
	tcm.mu.RUnlock()
	
	if exists {
		return tenant
	}
	
	tcm.mu.Lock()
	defer tcm.mu.Unlock()
	
	// Double-check after acquiring write lock
	if tenant, exists = tcm.tenants[tenantID]; exists {
		return tenant
	}
	
	// Load tenant limits from cache or use defaults
	limits := tcm.loadTenantLimits(tenantID)
	
	tenant = &TenantContext{
		TenantID: tenantID,
		Limits:   limits,
		Metrics:  &TenantMetrics{},
		rateLimiter: NewTenantRateLimiter(limits),
		circuitBreaker: NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 2,
			Timeout:         30 * time.Second,
		}),
	}
	
	tcm.tenants[tenantID] = tenant
	return tenant
}

// loadTenantLimits loads limits from cache or config
func (tcm *TenantConnectionManager) loadTenantLimits(tenantID string) *TenantLimits {
	ctx := context.Background()
	key := fmt.Sprintf("tenant:limits:%s", tenantID)
	
	var limits TenantLimits
	if err := tcm.cache.Get(ctx, key, &limits); err == nil {
		return &limits
	}
	
	// Return copy of defaults
	defaultCopy := *tcm.defaultLimits
	return &defaultCopy
}
```

#### 3.2 Event-Driven Architecture with SQS

**File**: `pkg/events/websocket_events.go`
```go
package events

import (
	"context"
	"encoding/json"
	"time"
	
	"github.com/S-Corkum/devops-mcp/pkg/queue"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// WebSocketEventPublisher publishes WebSocket events to SQS
type WebSocketEventPublisher struct {
	sqsClient *queue.SQSClient
	logger    observability.Logger
}

// PublishConnectionEvent publishes connection events
func (p *WebSocketEventPublisher) PublishConnectionEvent(
	ctx context.Context,
	eventType string,
	tenantID string,
	connectionID string,
	metadata map[string]interface{},
) error {
	event := queue.SQSEvent{
		DeliveryID: generateEventID(),
		EventType:  eventType,
		RepoName:   "websocket",
		SenderName: "mcp-server",
		Payload: json.RawMessage(mustMarshal(map[string]interface{}{
			"connection_id": connectionID,
			"tenant_id":     tenantID,
			"timestamp":     time.Now().UTC(),
			"metadata":      metadata,
		})),
		AuthContext: &queue.EventAuthContext{
			TenantID:      tenantID,
			PrincipalType: "system",
		},
	}
	
	if err := p.sqsClient.EnqueueEvent(ctx, event); err != nil {
		p.logger.Error("Failed to publish WebSocket event", map[string]interface{}{
			"error": err.Error(),
			"event_type": eventType,
			"tenant_id": tenantID,
		})
		return err
	}
	
	return nil
}
```

### Phase 4: Database Row-Level Security (Following Repository Pattern)

**Migration**: `migrations/000006_tenant_rls.up.sql`
```sql
-- Enable RLS on all tenant-scoped tables
ALTER TABLE models ENABLE ROW LEVEL SECURITY;
ALTER TABLE agents ENABLE ROW LEVEL SECURITY;
ALTER TABLE contexts ENABLE ROW LEVEL SECURITY;
ALTER TABLE context_references ENABLE ROW LEVEL SECURITY;

-- Create security definer function for setting tenant
CREATE OR REPLACE FUNCTION set_tenant_context(tenant_id text)
RETURNS void AS $$
BEGIN
    PERFORM set_config('app.current_tenant', tenant_id, true);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- RLS policies for models table
CREATE POLICY tenant_isolation_select ON models
    FOR SELECT
    USING (tenant_id = current_setting('app.current_tenant', true)::text);

CREATE POLICY tenant_isolation_insert ON models
    FOR INSERT
    WITH CHECK (tenant_id = current_setting('app.current_tenant', true)::text);

CREATE POLICY tenant_isolation_update ON models
    FOR UPDATE
    USING (tenant_id = current_setting('app.current_tenant', true)::text)
    WITH CHECK (tenant_id = current_setting('app.current_tenant', true)::text);

CREATE POLICY tenant_isolation_delete ON models
    FOR DELETE
    USING (tenant_id = current_setting('app.current_tenant', true)::text);

-- Repeat for other tables...

-- Create indexes for tenant_id for performance
CREATE INDEX IF NOT EXISTS idx_models_tenant_id ON models(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agents_tenant_id ON agents(tenant_id);
CREATE INDEX IF NOT EXISTS idx_contexts_tenant_id ON contexts(tenant_id);
```

**File**: `pkg/database/tenant_context.go`
```go
package database

import (
	"context"
	"database/sql"
	"fmt"
)

// WithTenantContext sets the tenant context for RLS
func WithTenantContext(ctx context.Context, db *sql.DB, tenantID string) (*sql.Conn, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	
	// Set tenant context
	_, err = conn.ExecContext(ctx, "SELECT set_tenant_context($1)", tenantID)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to set tenant context: %w", err)
	}
	
	return conn, nil
}

// TenantAwareRepository wraps repository with tenant context
type TenantAwareRepository struct {
	db       *sql.DB
	tenantID string
}

func (r *TenantAwareRepository) WithTenant(ctx context.Context, fn func(*sql.Conn) error) error {
	conn, err := WithTenantContext(ctx, r.db, r.tenantID)
	if err != nil {
		return err
	}
	defer conn.Close()
	
	return fn(conn)
}
```

### Phase 5: Monitoring and Testing (3-5 days)

#### 5.1 WebSocket-Specific Metrics

**File**: `apps/mcp-server/internal/api/websocket/metrics.go`
```go
package websocket

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// WebSocketMetrics tracks WebSocket-specific metrics
type WebSocketMetrics struct {
	// Connection metrics
	connectionsTotal      *prometheus.CounterVec
	connectionsActive     *prometheus.GaugeVec
	connectionDuration    *prometheus.HistogramVec
	
	// Message metrics
	messagesReceived      *prometheus.CounterVec
	messagesSent          *prometheus.CounterVec
	messageSize           *prometheus.HistogramVec
	messageProcessingTime *prometheus.HistogramVec
	
	// Protocol metrics
	binaryMessages        *prometheus.CounterVec
	textMessages          *prometheus.CounterVec
	compressionRatio      *prometheus.HistogramVec
	
	// Error metrics
	errors                *prometheus.CounterVec
	rateLimitHits         *prometheus.CounterVec
	
	// Tenant metrics
	tenantQuotaUsage      *prometheus.GaugeVec
	tenantViolations      *prometheus.CounterVec
}

func NewWebSocketMetrics() *WebSocketMetrics {
	return &WebSocketMetrics{
		connectionsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "websocket_connections_total",
				Help: "Total number of WebSocket connections",
			},
			[]string{"tenant_id", "auth_type", "status"},
		),
		connectionsActive: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "websocket_connections_active",
				Help: "Number of active WebSocket connections",
			},
			[]string{"tenant_id", "protocol_version"},
		),
		messageProcessingTime: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "websocket_message_processing_seconds",
				Help: "Time taken to process WebSocket messages",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
			},
			[]string{"tenant_id", "method", "status"},
		),
		// ... more metrics
	}
}
```

#### 5.2 WebSocket Load Testing with coder/websocket

**File**: `test/load/websocket_load_test.go`
```go
package load_test

import (
	"context"
	"sync"
	"testing"
	"time"
	
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/require"
	
	"github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

func TestWebSocketLoad(t *testing.T) {
	const (
		numClients     = 1000
		numMessages    = 100
		messageSize    = 1024
		testDuration   = 5 * time.Minute
	)
	
	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()
	
	// Metrics collection
	metrics := &LoadTestMetrics{
		ConnectionTimes: make([]time.Duration, 0, numClients),
		MessageTimes:    make([]time.Duration, 0, numClients*numMessages),
	}
	
	// Create client pool
	var wg sync.WaitGroup
	clientErrors := make(chan error, numClients)
	
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			
			if err := runClient(ctx, clientID, numMessages, metrics); err != nil {
				clientErrors <- err
			}
		}(i)
		
		// Stagger client connections
		time.Sleep(10 * time.Millisecond)
	}
	
	// Wait for all clients
	wg.Wait()
	close(clientErrors)
	
	// Check for errors
	var errorCount int
	for err := range clientErrors {
		t.Logf("Client error: %v", err)
		errorCount++
	}
	
	// Calculate metrics
	avgConnTime := calculateAverage(metrics.ConnectionTimes)
	avgMsgTime := calculateAverage(metrics.MessageTimes)
	p99MsgTime := calculatePercentile(metrics.MessageTimes, 99)
	
	t.Logf("Load test results:")
	t.Logf("  Clients: %d", numClients)
	t.Logf("  Messages per client: %d", numMessages)
	t.Logf("  Errors: %d (%.2f%%)", errorCount, float64(errorCount)/float64(numClients)*100)
	t.Logf("  Avg connection time: %v", avgConnTime)
	t.Logf("  Avg message RTT: %v", avgMsgTime)
	t.Logf("  P99 message RTT: %v", p99MsgTime)
	
	// Assert performance requirements
	require.Less(t, avgConnTime, 100*time.Millisecond)
	require.Less(t, p99MsgTime, 100*time.Millisecond)
	require.Less(t, float64(errorCount)/float64(numClients), 0.01) // < 1% error rate
}

func runClient(ctx context.Context, clientID int, numMessages int, metrics *LoadTestMetrics) error {
	// Connect with coder/websocket
	connStart := time.Now()
	conn, _, err := websocket.Dial(ctx, testWSURL, &websocket.DialOptions{
		HTTPHeader: map[string][]string{
			"Authorization": {getTestToken(clientID)},
		},
	})
	if err != nil {
		return err
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	
	metrics.mu.Lock()
	metrics.ConnectionTimes = append(metrics.ConnectionTimes, time.Since(connStart))
	metrics.mu.Unlock()
	
	// Send messages
	for i := 0; i < numMessages; i++ {
		msg := &websocket.Message{
			ID:     generateMessageID(),
			Method: "test.echo",
			Params: map[string]interface{}{
				"client_id": clientID,
				"message_id": i,
				"timestamp": time.Now().UnixNano(),
				"payload": generatePayload(messageSize),
			},
		}
		
		msgStart := time.Now()
		
		// Send with wsjson
		if err := wsjson.Write(ctx, conn, msg); err != nil {
			return err
		}
		
		// Read response
		var response websocket.Message
		if err := wsjson.Read(ctx, conn, &response); err != nil {
			return err
		}
		
		metrics.mu.Lock()
		metrics.MessageTimes = append(metrics.MessageTimes, time.Since(msgStart))
		metrics.mu.Unlock()
		
		// Pace messages
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	
	return nil
}
```

## Success Metrics

### Functional
- ✅ 100% of WebSocket tests passing against correct service
- ✅ 100% of tenant isolation tests passing
- ✅ Zero cross-tenant data leaks
- ✅ All coder/websocket features utilized

### Performance (Leveraging coder/websocket)
- ✅ WebSocket latency < 50ms (p99) with compression
- ✅ Support 10K+ concurrent connections per server
- ✅ Message throughput > 100K msg/sec with binary protocol
- ✅ Memory usage < 100MB for 10K idle connections

### Security
- ✅ Zero critical vulnerabilities in OWASP scan
- ✅ 100% tenant isolation at all layers
- ✅ Full audit trail with correlation IDs
- ✅ Automatic security monitoring and alerting

### Code Quality (Following CLAUDE.md)
- ✅ All packages follow Go workspace structure
- ✅ Repository pattern used consistently
- ✅ Proper error handling with context
- ✅ Comprehensive test coverage (>85%)

## Implementation Timeline

### Week 1: Foundation
- Day 1-2: Fix test routing and tenant propagation
- Day 3-4: Reorganize test structure
- Day 5: Implement coder/websocket enhancements

### Week 2: Security & Architecture
- Day 1-2: Tenant isolation at WebSocket layer
- Day 3-4: Database RLS implementation
- Day 5: Event-driven architecture setup

### Week 3: Production Readiness
- Day 1-2: Monitoring and metrics
- Day 3-4: Load testing and optimization
- Day 5: Documentation and deployment prep

### Week 4: Polish & Deploy
- Day 1-2: Security audit and fixes
- Day 3-4: Performance tuning
- Day 5: Rollout to staging/production

## Key Improvements for Claude Code with Opus 4

1. **Concise Code Examples**: Each example is focused and demonstrates best practices
2. **Clear File Paths**: All paths follow the Go workspace structure
3. **Minimal Comments**: Code is self-documenting with clear naming
4. **Test-First Approach**: Tests defined before implementation
5. **Performance Focus**: Leveraging coder/websocket's efficiency features
6. **Security by Default**: Tenant isolation at every layer
7. **Observable System**: Comprehensive metrics and logging

This plan fully leverages coder/websocket's features including:
- Context-based cancellation
- Efficient binary protocol with MessagePack
- Built-in compression
- Origin pattern matching
- Graceful shutdown
- Memory-efficient connection handling

The implementation follows all CLAUDE.md best practices and industry standards for multi-tenant SaaS platforms.