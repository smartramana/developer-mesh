# WebSocket Implementation Plan for MCP Server - Optimized for Claude Code

## Executive Summary

This plan implements a production-grade WebSocket server for the MCP platform in Go, optimized for Claude Code with Opus 4 to implement successfully on the first attempt. The plan is structured with discrete, testable tasks that build incrementally from basic functionality to high-performance, secure operations.

## Implementation Approach for Claude Code

### 1. Incremental Development
- **Phase 1**: Basic WebSocket server with text protocol
- **Phase 2**: Binary protocol with performance optimizations
- **Phase 3**: Security enhancements and authentication
- **Phase 4**: Production hardening and scale testing

### 2. Test-Driven Development
- Each component has isolated unit tests
- Integration tests verify component interactions
- Benchmark tests measure performance improvements
- All tests must pass before proceeding

### 3. Clear Integration Path
- Integrates with existing MCP server architecture
- Uses established patterns from the codebase
- Leverages existing auth and monitoring systems
- Maintains backward compatibility

## Target Performance & Security Requirements

### Performance (Day One)
- **Concurrent Connections**: 100,000+ per server
- **Message Latency**: < 150μs (p50), < 600μs (p99) with full security
- **Throughput**: 8M+ messages/second per server (authenticated)
- **Memory**: < 2KB per idle connection
- **CPU**: < 0.001% per idle connection
- **Zero GC pressure**: Lock-free, zero-allocation design

### Security (Non-Negotiable)
- **Memory safety**: 100% for critical paths (Rust)
- **Authentication**: mTLS + JWT/PASETO per request
- **Encryption**: AES-256-GCM or ChaCha20-Poly1305
- **Attack mitigation**: < 1μs detection via eBPF
- **Compliance**: SOC2 Type II, NIST CSF, PCI-DSS ready

## Technology Stack

### Core Dependencies (Go)
- **nhooyr.io/websocket**: Modern WebSocket library with good performance
- **github.com/golang-jwt/jwt/v5**: JWT authentication
- **github.com/klauspost/compress**: Fast compression algorithms
- **github.com/cespare/xxhash/v2**: Fast hashing for message IDs

### Existing MCP Components
- **pkg/auth**: Authentication and authorization
- **pkg/observability**: Metrics and logging
- **pkg/models**: Data models and types
- **pkg/events**: Event bus for real-time updates

### Performance Libraries (Phase 2+)
- **github.com/puzpuzpuz/xsync/v3**: Lock-free maps
- **golang.org/x/sync/errgroup**: Concurrent operations
- **github.com/bytedance/sonic**: Fast JSON parsing

## Implementation Tasks (Discrete & Ordered)

### Task 1: Core Data Structures (Day 1)
**File**: `pkg/models/websocket/types.go`
```go
package websocket

import (
    "time"
    "sync/atomic"
)

// MessageType represents WebSocket message types
type MessageType uint8

const (
    MessageTypeRequest MessageType = iota
    MessageTypeResponse
    MessageTypeNotification
    MessageTypeError
    MessageTypePing
    MessageTypePong
)

// Message represents a WebSocket message (JSON initially)
type Message struct {
    ID      string      `json:"id"`
    Type    MessageType `json:"type"`
    Method  string      `json:"method,omitempty"`
    Params  interface{} `json:"params,omitempty"`
    Result  interface{} `json:"result,omitempty"`
    Error   *Error      `json:"error,omitempty"`
}

// Connection represents a WebSocket connection
type Connection struct {
    ID        string
    AgentID   string
    TenantID  string
    State     atomic.Value // ConnectionState
    CreatedAt time.Time
    LastPing  time.Time
}
```

**Tests**: `pkg/models/websocket/types_test.go`
```go
func TestMessageSerialization(t *testing.T) {
    msg := &Message{
        ID:     "test-123",
        Type:   MessageTypeRequest,
        Method: "tool.execute",
        Params: map[string]string{"tool": "github"},
    }
    
    data, err := json.Marshal(msg)
    assert.NoError(t, err)
    
    var decoded Message
    err = json.Unmarshal(data, &decoded)
    assert.NoError(t, err)
    assert.Equal(t, msg.ID, decoded.ID)
}
```

### Task 2: Basic WebSocket Server (Day 2)
**File**: `apps/mcp-server/internal/api/websocket/server.go`

```go
package websocket

import (
    "context"
    "net/http"
    "sync"
    "time"
    
    "nhooyr.io/websocket"
    "github.com/google/uuid"
    "github.com/rs/zerolog/log"
    
    "github.com/S-Corkum/devops-mcp/pkg/auth"
    "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
    "github.com/S-Corkum/devops-mcp/pkg/observability"
)

type Server struct {
    connections map[string]*Connection
    mu          sync.RWMutex
    
    auth        auth.Service
    metrics     observability.MetricsClient
    logger      observability.Logger
    
    config      Config
}

type Config struct {
    MaxConnections   int           `mapstructure:"max_connections"`
    ReadBufferSize   int           `mapstructure:"read_buffer_size"`
    WriteBufferSize  int           `mapstructure:"write_buffer_size"`
    PingInterval     time.Duration `mapstructure:"ping_interval"`
    PongTimeout      time.Duration `mapstructure:"pong_timeout"`
    MaxMessageSize   int64         `mapstructure:"max_message_size"`
}

func NewServer(auth auth.Service, metrics observability.MetricsClient, logger observability.Logger, config Config) *Server {
    return &Server{
        connections: make(map[string]*Connection),
        auth:        auth,
        metrics:     metrics,
        logger:      logger,
        config:      config,
    }
}

func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
    // Authenticate request
    claims, err := s.auth.ValidateRequest(r)
    if err != nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    
    // Check connection limit
    if s.ConnectionCount() >= s.config.MaxConnections {
        http.Error(w, "Too Many Connections", http.StatusServiceUnavailable)
        return
    }
    
    // Accept WebSocket connection
    conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
        Subprotocols: []string{"mcp.v1"},
    })
    if err != nil {
        s.logger.Error("WebSocket accept failed", map[string]interface{}{
            "error": err.Error(),
        })
        return
    }
    
    // Create connection object
    connection := &Connection{
        ID:        uuid.New().String(),
        AgentID:   claims.AgentID,
        TenantID:  claims.TenantID,
        conn:      conn,
        send:      make(chan []byte, 256),
        hub:       s,
    }
    
    // Register connection
    s.addConnection(connection)
    
    // Start connection handlers
    go connection.writePump()
    go connection.readPump()
    
    s.logger.Info("WebSocket connection established", map[string]interface{}{
        "connection_id": connection.ID,
        "agent_id":      connection.AgentID,
        "tenant_id":     connection.TenantID,
    })
}
```

### Task 3: Connection Management (Day 3)
**File**: `apps/mcp-server/internal/api/websocket/connection.go`

```go
package websocket

import (
    "context"
    "encoding/json"
    "time"
    
    "nhooyr.io/websocket"
    "nhooyr.io/websocket/wsjson"
)

type Connection struct {
    ID       string
    AgentID  string
    TenantID string
    
    conn     *websocket.Conn
    send     chan []byte
    hub      *Server
    
    // Rate limiting
    lastMessage time.Time
    messageRate *RateLimiter
}

func (c *Connection) readPump() {
    defer func() {
        c.hub.removeConnection(c)
        c.conn.Close(websocket.StatusNormalClosure, "")
    }()
    
    ctx := context.Background()
    
    for {
        // Read message
        var msg websocket.Message
        err := wsjson.Read(ctx, c.conn, &msg)
        if err != nil {
            if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
                return
            }
            c.hub.logger.Error("Read error", map[string]interface{}{
                "error":         err.Error(),
                "connection_id": c.ID,
            })
            return
        }
        
        // Rate limiting
        if !c.messageRate.Allow() {
            c.sendError("Rate limit exceeded")
            continue
        }
        
        // Process message
        response, err := c.hub.processMessage(ctx, c, &msg)
        if err != nil {
            c.sendError(err.Error())
            continue
        }
        
        // Send response
        if response != nil {
            c.send <- response
        }
    }
}

func (c *Connection) writePump() {
    ticker := time.NewTicker(c.hub.config.PingInterval)
    defer func() {
        ticker.Stop()
        c.conn.Close(websocket.StatusNormalClosure, "")
    }()
    
    ctx := context.Background()
    
    for {
        select {
        case message, ok := <-c.send:
            if !ok {
                return
            }
            
            err := c.conn.Write(ctx, websocket.MessageText, message)
            if err != nil {
                return
            }
            
        case <-ticker.C:
            if err := c.conn.Ping(ctx); err != nil {
                return
            }
        }
    }
}
```

### Task 4: Message Processing (Day 4)
**File**: `apps/mcp-server/internal/api/websocket/handlers.go`

```go
package websocket

import (
    "context"
    "encoding/json"
    "fmt"
    
    "github.com/S-Corkum/devops-mcp/pkg/models"
)

// MessageHandler processes a specific message type
type MessageHandler func(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error)

// RegisterHandlers sets up all message handlers
func (s *Server) RegisterHandlers() {
    s.handlers = map[string]MessageHandler{
        "initialize":      s.handleInitialize,
        "tool.list":       s.handleToolList,
        "tool.execute":    s.handleToolExecute,
        "context.get":     s.handleContextGet,
        "context.update":  s.handleContextUpdate,
        "event.subscribe": s.handleEventSubscribe,
    }
}

func (s *Server) processMessage(ctx context.Context, conn *Connection, msg *Message) ([]byte, error) {
    // Validate message
    if msg.Type != MessageTypeRequest {
        return nil, fmt.Errorf("invalid message type: %d", msg.Type)
    }
    
    // Get handler
    handler, ok := s.handlers[msg.Method]
    if !ok {
        return s.createErrorResponse(msg.ID, -32601, "Method not found")
    }
    
    // Execute handler
    result, err := handler(ctx, conn, msg.Params)
    if err != nil {
        return s.createErrorResponse(msg.ID, -32603, err.Error())
    }
    
    // Create response
    response := &Message{
        ID:     msg.ID,
        Type:   MessageTypeResponse,
        Result: result,
    }
    
    return json.Marshal(response)
}

// Example handler implementation
func (s *Server) handleToolList(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
    // Get available tools for this agent
    tools, err := s.toolRegistry.GetToolsForAgent(conn.AgentID)
    if err != nil {
        return nil, err
    }
    
    return map[string]interface{}{
        "tools": tools,
    }, nil
}
```

**Tests**: `apps/mcp-server/internal/api/websocket/handlers_test.go`
```go
func TestProcessMessage(t *testing.T) {
    server := NewTestServer()
    conn := &Connection{ID: "test-conn", AgentID: "test-agent"}
    
    msg := &Message{
        ID:     "123",
        Type:   MessageTypeRequest,
        Method: "tool.list",
    }
    
    response, err := server.processMessage(context.Background(), conn, msg)
    assert.NoError(t, err)
    assert.Contains(t, string(response), "tools")
}
```

### Task 5: Binary Protocol (Day 5-6)
**File**: `pkg/models/websocket/binary.go`

```go
package websocket

import (
    "encoding/binary"
    "errors"
    "io"
)

// BinaryHeader represents the binary protocol header (24 bytes)
type BinaryHeader struct {
    Magic      uint32 // 0x4D435057 "MCPW"
    Version    uint8  // Protocol version (1)
    Type       uint8  // Message type
    Flags      uint16 // Compression, encryption flags
    SequenceID uint64 // Message sequence ID
    Method     uint16 // Method enum (not string)
    Reserved   uint16 // Padding for alignment
    DataSize   uint32 // Payload size
}

// Method enums for binary protocol
const (
    MethodInitialize     uint16 = 1
    MethodToolList       uint16 = 2
    MethodToolExecute    uint16 = 3
    MethodContextGet     uint16 = 4
    MethodContextUpdate  uint16 = 5
    MethodEventSubscribe uint16 = 6
)

// ParseBinaryHeader reads and validates a binary header
func ParseBinaryHeader(r io.Reader) (*BinaryHeader, error) {
    header := &BinaryHeader{}
    
    // Read 24 bytes
    buf := make([]byte, 24)
    if _, err := io.ReadFull(r, buf); err != nil {
        return nil, err
    }
    
    // Parse fields
    header.Magic = binary.BigEndian.Uint32(buf[0:4])
    header.Version = buf[4]
    header.Type = buf[5]
    header.Flags = binary.BigEndian.Uint16(buf[6:8])
    header.SequenceID = binary.BigEndian.Uint64(buf[8:16])
    header.Method = binary.BigEndian.Uint16(buf[16:18])
    header.Reserved = binary.BigEndian.Uint16(buf[18:20])
    header.DataSize = binary.BigEndian.Uint32(buf[20:24])
    
    // Validate
    if header.Magic != 0x4D435057 {
        return nil, errors.New("invalid magic number")
    }
    
    if header.Version != 1 {
        return nil, errors.New("unsupported protocol version")
    }
    
    if header.DataSize > 1024*1024 { // 1MB max
        return nil, errors.New("payload too large")
    }
    
    return header, nil
}

// WriteBinaryHeader writes a binary header
func WriteBinaryHeader(w io.Writer, header *BinaryHeader) error {
    buf := make([]byte, 24)
    
    binary.BigEndian.PutUint32(buf[0:4], header.Magic)
    buf[4] = header.Version
    buf[5] = header.Type
    binary.BigEndian.PutUint16(buf[6:8], header.Flags)
    binary.BigEndian.PutUint64(buf[8:16], header.SequenceID)
    binary.BigEndian.PutUint16(buf[16:18], header.Method)
    binary.BigEndian.PutUint16(buf[18:20], header.Reserved)
    binary.BigEndian.PutUint32(buf[20:24], header.DataSize)
    
    _, err := w.Write(buf)
    return err
}
```

**Benchmark**: `pkg/models/websocket/binary_test.go`
```go
func BenchmarkBinaryParsing(b *testing.B) {
    // Create test message
    header := &BinaryHeader{
        Magic:      0x4D435057,
        Version:    1,
        Type:       MessageTypeRequest,
        Method:     MethodToolExecute,
        SequenceID: 12345,
        DataSize:   100,
    }
    
    buf := &bytes.Buffer{}
    WriteBinaryHeader(buf, header)
    data := buf.Bytes()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        r := bytes.NewReader(data)
        _, _ = ParseBinaryHeader(r)
    }
}
```

### Task 6: Authentication & Security (Day 7)
**File**: `apps/mcp-server/internal/api/websocket/auth.go`

```go
package websocket

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "errors"
    "time"
    
    "github.com/golang-jwt/jwt/v5"
    
    "github.com/S-Corkum/devops-mcp/pkg/auth"
)

// AuthenticatedMessage adds authentication to messages
type AuthenticatedMessage struct {
    Message   *Message
    Signature string    // HMAC signature
    Timestamp time.Time // Anti-replay
}

// ValidateConnection performs initial authentication
func (s *Server) ValidateConnection(token string) (*auth.Claims, error) {
    // Parse JWT token
    parsedToken, err := jwt.ParseWithClaims(token, &auth.Claims{}, func(token *jwt.Token) (interface{}, error) {
        return s.auth.GetSigningKey(), nil
    })
    
    if err != nil {
        return nil, err
    }
    
    claims, ok := parsedToken.Claims.(*auth.Claims)
    if !ok || !parsedToken.Valid {
        return nil, errors.New("invalid token")
    }
    
    // Check expiration
    if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
        return nil, errors.New("token expired")
    }
    
    return claims, nil
}

// SignMessage creates HMAC signature for a message
func (c *Connection) SignMessage(msg []byte) string {
    h := hmac.New(sha256.New, c.sessionKey)
    h.Write(msg)
    h.Write([]byte(c.ID))
    h.Write([]byte(fmt.Sprintf("%d", time.Now().Unix())))
    
    return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// VerifyMessage validates HMAC signature
func (c *Connection) VerifyMessage(msg []byte, signature string) error {
    expected := c.SignMessage(msg)
    
    // Constant-time comparison
    if !hmac.Equal([]byte(expected), []byte(signature)) {
        return errors.New("invalid signature")
    }
    
    return nil
}

// RateLimiter implements token bucket algorithm
type RateLimiter struct {
    tokens    float64
    capacity  float64
    rate      float64
    lastCheck time.Time
}

func NewRateLimiter(rate, capacity float64) *RateLimiter {
    return &RateLimiter{
        tokens:    capacity,
        capacity:  capacity,
        rate:      rate,
        lastCheck: time.Now(),
    }
}

func (r *RateLimiter) Allow() bool {
    now := time.Now()
    elapsed := now.Sub(r.lastCheck).Seconds()
    r.lastCheck = now
    
    // Add tokens based on elapsed time
    r.tokens += elapsed * r.rate
    if r.tokens > r.capacity {
        r.tokens = r.capacity
    }
    
    // Check if we have tokens
    if r.tokens >= 1.0 {
        r.tokens -= 1.0
        return true
    }
    
    return false
}
```

### Task 7: Integration with MCP Server (Day 8)
**File**: `apps/mcp-server/internal/api/server.go` (UPDATE)

```go
// UPDATE existing server.go to add WebSocket support
func (s *Server) setupRoutes() {
    // ... existing routes ...
    
    // Add WebSocket endpoint
    if s.config.WebSocket.Enabled {
        wsServer := websocket.NewServer(
            s.auth,
            s.metrics,
            s.logger,
            s.config.WebSocket,
        )
        
        // Register WebSocket handlers that integrate with existing services
        wsServer.SetToolRegistry(s.toolRegistry)
        wsServer.SetContextManager(s.contextManager)
        wsServer.SetEventBus(s.eventBus)
        
        // Mount WebSocket endpoint
        s.router.HandleFunc("/ws", wsServer.HandleWebSocket)
        
        s.logger.Info("WebSocket server enabled", map[string]interface{}{
            "max_connections": s.config.WebSocket.MaxConnections,
            "port":            s.config.Port,
        })
    }
}
```

### Task 8: Configuration (Day 8)
**File**: `configs/config.base.yaml` (UPDATE)
```yaml
# Add WebSocket configuration
websocket:
  enabled: true
  max_connections: 10000
  read_buffer_size: 4096
  write_buffer_size: 4096
  ping_interval: 30s
  pong_timeout: 60s
  max_message_size: 1048576  # 1MB
  compression:
    enabled: true
    threshold: 1024  # Compress messages > 1KB
  rate_limit:
    requests_per_minute: 1000
    burst: 100
```

### Task 9: Performance Optimization (Day 9-10)
**File**: `apps/mcp-server/internal/api/websocket/pool.go`

```go
package websocket

import (
    "sync"
    "sync/atomic"
    "runtime"
)

// ConnectionPool manages pre-allocated connections
type ConnectionPool struct {
    pools    []*sync.Pool
    numPools int
    metrics  *PoolMetrics
}

type PoolMetrics struct {
    Allocated   atomic.Int64
    InUse       atomic.Int64
    Recycled    atomic.Int64
}

func NewConnectionPool() *ConnectionPool {
    numPools := runtime.NumCPU()
    pools := make([]*sync.Pool, numPools)
    
    for i := 0; i < numPools; i++ {
        pools[i] = &sync.Pool{
            New: func() interface{} {
                return &Connection{
                    send:        make(chan []byte, 256),
                    messageRate: NewRateLimiter(1000, 100), // 1000/min, burst 100
                }
            },
        }
    }
    
    return &ConnectionPool{
        pools:    pools,
        numPools: numPools,
        metrics:  &PoolMetrics{},
    }
}

func (p *ConnectionPool) Get() *Connection {
    // Select pool based on current goroutine
    poolIdx := runtime.CPUNum() % p.numPools
    conn := p.pools[poolIdx].Get().(*Connection)
    
    p.metrics.InUse.Add(1)
    return conn
}

func (p *ConnectionPool) Put(conn *Connection) {
    // Reset connection state
    conn.ID = ""
    conn.AgentID = ""
    conn.TenantID = ""
    conn.lastMessage = time.Time{}
    
    // Return to pool
    poolIdx := runtime.CPUNum() % p.numPools
    p.pools[poolIdx].Put(conn)
    
    p.metrics.InUse.Add(-1)
    p.metrics.Recycled.Add(1)
}
```

### Task 10: Message Batching (Day 10)
**File**: `apps/mcp-server/internal/api/websocket/batch.go`
```go
package websocket

import (
    "sync"
    "time"
)

// MessageBatcher optimizes network utilization
type MessageBatcher struct {
    messages  []*Message
    mu        sync.Mutex
    timer     *time.Timer
    maxSize   int
    maxWait   time.Duration
    flushFunc func([]*Message)
}

func NewMessageBatcher(maxSize int, maxWait time.Duration, flush func([]*Message)) *MessageBatcher {
    return &MessageBatcher{
        messages:  make([]*Message, 0, maxSize),
        maxSize:   maxSize,
        maxWait:   maxWait,
        flushFunc: flush,
    }
}

func (b *MessageBatcher) Add(msg *Message) {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    b.messages = append(b.messages, msg)
    
    // Flush if batch is full
    if len(b.messages) >= b.maxSize {
        b.flush()
        return
    }
    
    // Start timer if this is the first message
    if len(b.messages) == 1 {
        b.timer = time.AfterFunc(b.maxWait, func() {
            b.mu.Lock()
            defer b.mu.Unlock()
            b.flush()
        })
    }
}

func (b *MessageBatcher) flush() {
    if len(b.messages) == 0 {
        return
    }
    
    // Stop timer
    if b.timer != nil {
        b.timer.Stop()
        b.timer = nil
    }
    
    // Send batch
    batch := b.messages
    b.messages = make([]*Message, 0, b.maxSize)
    
    go b.flushFunc(batch)
}
```

### Task 11: Monitoring & Testing (Day 11-12)
**File**: `apps/mcp-server/internal/api/websocket/metrics.go`

```go
package websocket

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // Connection metrics
    connectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "websocket_connections_active",
        Help: "Current number of active WebSocket connections",
    })
    
    connectionsTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "websocket_connections_total",
        Help: "Total number of WebSocket connections",
    })
    
    // Message metrics
    messagesReceived = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "websocket_messages_received_total",
        Help: "Total messages received by type",
    }, []string{"type", "method"})
    
    messageLatency = promauto.NewHistogram(prometheus.HistogramOpts{
        Name:    "websocket_message_latency_seconds",
        Help:    "Message processing latency",
        Buckets: prometheus.ExponentialBuckets(0.00001, 2, 20), // 10μs to 10s
    })
    
    // Error metrics
    errors = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "websocket_errors_total",
        Help: "Total errors by type",
    }, []string{"type"})
)

// RecordMetrics updates Prometheus metrics
func (s *Server) RecordMetrics(event string, labels map[string]string) {
    switch event {
    case "connection_opened":
        connectionsActive.Inc()
        connectionsTotal.Inc()
    case "connection_closed":
        connectionsActive.Dec()
    case "message_received":
        messagesReceived.WithLabelValues(labels["type"], labels["method"]).Inc()
    case "error":
        errors.WithLabelValues(labels["type"]).Inc()
    }
}
```

### Task 12: Unit Tests for All Components (Day 12)
**File**: `apps/mcp-server/internal/api/websocket/server_test.go`
```go
package websocket

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "nhooyr.io/websocket"
    
    "github.com/S-Corkum/devops-mcp/pkg/auth"
    "github.com/S-Corkum/devops-mcp/pkg/observability"
)

// Mock dependencies
type MockAuthService struct {
    mock.Mock
}

func (m *MockAuthService) ValidateRequest(r *http.Request) (*auth.Claims, error) {
    args := m.Called(r)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*auth.Claims), args.Error(1)
}

type MockMetricsClient struct {
    mock.Mock
}

func (m *MockMetricsClient) IncrementCounter(name string, value float64) {
    m.Called(name, value)
}

func (m *MockMetricsClient) RecordHistogram(name string, value float64) {
    m.Called(name, value)
}

// Test server creation
func TestNewServer(t *testing.T) {
    auth := &MockAuthService{}
    metrics := &MockMetricsClient{}
    logger := observability.NewNoopLogger()
    
    config := Config{
        MaxConnections:  100,
        ReadBufferSize:  4096,
        WriteBufferSize: 4096,
        PingInterval:    30 * time.Second,
        PongTimeout:     60 * time.Second,
    }
    
    server := NewServer(auth, metrics, logger, config)
    
    assert.NotNil(t, server)
    assert.Equal(t, config.MaxConnections, server.config.MaxConnections)
    assert.NotNil(t, server.connections)
}

// Test WebSocket upgrade
func TestHandleWebSocket_Success(t *testing.T) {
    // Setup
    auth := &MockAuthService{}
    metrics := &MockMetricsClient{}
    logger := observability.NewNoopLogger()
    
    server := NewServer(auth, metrics, logger, Config{
        MaxConnections: 100,
    })
    
    // Mock auth success
    auth.On("ValidateRequest", mock.Anything).Return(&auth.Claims{
        AgentID:  "test-agent",
        TenantID: "test-tenant",
    }, nil)
    
    metrics.On("IncrementCounter", "websocket_connections_total", float64(1)).Return()
    metrics.On("IncrementCounter", "websocket_connections_active", float64(1)).Return()
    
    // Create test server
    ts := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
    defer ts.Close()
    
    // Connect client
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    wsURL := "ws" + ts.URL[4:] // Convert http:// to ws://
    conn, _, err := websocket.Dial(ctx, wsURL, nil)
    
    assert.NoError(t, err)
    assert.NotNil(t, conn)
    
    // Verify connection registered
    assert.Equal(t, 1, server.ConnectionCount())
    
    // Cleanup
    conn.Close(websocket.StatusNormalClosure, "")
    
    // Wait for cleanup
    time.Sleep(100 * time.Millisecond)
    assert.Equal(t, 0, server.ConnectionCount())
    
    auth.AssertExpectations(t)
    metrics.AssertExpectations(t)
}

// Test connection limit
func TestHandleWebSocket_ConnectionLimit(t *testing.T) {
    auth := &MockAuthService{}
    metrics := &MockMetricsClient{}
    logger := observability.NewNoopLogger()
    
    server := NewServer(auth, metrics, logger, Config{
        MaxConnections: 1, // Only allow 1 connection
    })
    
    // Add a connection manually
    server.connections["existing"] = &Connection{}
    
    // Mock auth (should not be called)
    auth.On("ValidateRequest", mock.Anything).Maybe().Return(&auth.Claims{
        AgentID:  "test-agent",
        TenantID: "test-tenant",
    }, nil)
    
    // Try to connect
    req := httptest.NewRequest("GET", "/ws", nil)
    req.Header.Set("Upgrade", "websocket")
    req.Header.Set("Connection", "upgrade")
    
    w := httptest.NewRecorder()
    server.HandleWebSocket(w, req)
    
    // Should return 503
    assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// Test message processing
func TestProcessMessage(t *testing.T) {
    server := &Server{
        handlers: make(map[string]MessageHandler),
        logger:   observability.NewNoopLogger(),
    }
    
    // Register test handler
    server.handlers["test.echo"] = func(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
        return map[string]string{"echo": string(params)}, nil
    }
    
    conn := &Connection{ID: "test-conn"}
    
    msg := &Message{
        ID:     "msg-1",
        Type:   MessageTypeRequest,
        Method: "test.echo",
        Params: json.RawMessage(`{"data":"hello"}`),
    }
    
    response, err := server.processMessage(context.Background(), conn, msg)
    
    assert.NoError(t, err)
    assert.NotNil(t, response)
    
    // Parse response
    var respMsg Message
    err = json.Unmarshal(response, &respMsg)
    assert.NoError(t, err)
    
    assert.Equal(t, "msg-1", respMsg.ID)
    assert.Equal(t, MessageTypeResponse, respMsg.Type)
    assert.NotNil(t, respMsg.Result)
}

// Test rate limiting
func TestRateLimiter(t *testing.T) {
    rl := NewRateLimiter(10, 5) // 10 per second, burst 5
    
    // Should allow burst
    for i := 0; i < 5; i++ {
        assert.True(t, rl.Allow())
    }
    
    // Should be rate limited
    allowed := 0
    for i := 0; i < 10; i++ {
        if rl.Allow() {
            allowed++
        }
    }
    assert.Less(t, allowed, 10)
    
    // Wait and try again
    time.Sleep(1 * time.Second)
    assert.True(t, rl.Allow())
}

// Test binary protocol
func TestBinaryProtocol(t *testing.T) {
    // Test header parsing
    header := &BinaryHeader{
        Magic:      0x4D435057,
        Version:    1,
        Type:       MessageTypeRequest,
        SequenceID: 12345,
        Method:     MethodToolList,
        DataSize:   100,
    }
    
    buf := &bytes.Buffer{}
    err := WriteBinaryHeader(buf, header)
    assert.NoError(t, err)
    
    parsed, err := ParseBinaryHeader(buf)
    assert.NoError(t, err)
    
    assert.Equal(t, header.Magic, parsed.Magic)
    assert.Equal(t, header.Version, parsed.Version)
    assert.Equal(t, header.Type, parsed.Type)
    assert.Equal(t, header.SequenceID, parsed.SequenceID)
    assert.Equal(t, header.Method, parsed.Method)
    assert.Equal(t, header.DataSize, parsed.DataSize)
}

// Test connection pool
func TestConnectionPool(t *testing.T) {
    pool := NewConnectionPool(10)
    
    // Get connection
    conn := pool.Get()
    assert.NotNil(t, conn)
    assert.NotNil(t, conn.send)
    assert.NotNil(t, conn.messageRate)
    
    // Return connection
    pool.Put(conn)
    
    // Get should return same connection
    conn2 := pool.Get()
    assert.Equal(t, "", conn2.ID) // Should be reset
}

// Test message batching
func TestMessageBatcher(t *testing.T) {
    var flushed []*Message
    batcher := NewMessageBatcher(5, 100*time.Millisecond, func(msgs []*Message) {
        flushed = msgs
    })
    
    // Add messages up to batch size
    for i := 0; i < 5; i++ {
        batcher.Add(&Message{ID: fmt.Sprintf("msg-%d", i)})
    }
    
    // Should flush immediately
    time.Sleep(10 * time.Millisecond)
    assert.Len(t, flushed, 5)
    
    // Test timeout flush
    flushed = nil
    batcher.Add(&Message{ID: "msg-6"})
    
    // Should not flush yet
    time.Sleep(50 * time.Millisecond)
    assert.Nil(t, flushed)
    
    // Should flush after timeout
    time.Sleep(60 * time.Millisecond)
    assert.Len(t, flushed, 1)
}
```

### Task 12.5: Integration Tests (Day 12)
**File**: `apps/mcp-server/internal/api/websocket/integration_test.go`
```go
package websocket_test

import (
    "context"
    "testing"
    "time"
    
    "nhooyr.io/websocket"
    "nhooyr.io/websocket/wsjson"
    "github.com/stretchr/testify/suite"
    
    "github.com/S-Corkum/devops-mcp/test/functional/foundation"
)

type WebSocketTestSuite struct {
    suite.Suite
    server *TestServer
    client *websocket.Conn
}

func (s *WebSocketTestSuite) SetupTest() {
    s.server = NewTestServer()
    s.server.Start()
    
    // Connect client
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    conn, _, err := websocket.Dial(ctx, s.server.URL("/ws"), nil)
    s.Require().NoError(err)
    s.client = conn
}

func (s *WebSocketTestSuite) TestMessageRoundTrip() {
    ctx := context.Background()
    
    // Send request
    request := &Message{
        ID:     "test-123",
        Type:   MessageTypeRequest,
        Method: "tool.list",
    }
    
    err := wsjson.Write(ctx, s.client, request)
    s.Require().NoError(err)
    
    // Read response
    var response Message
    err = wsjson.Read(ctx, s.client, &response)
    s.Require().NoError(err)
    
    s.Equal(request.ID, response.ID)
    s.Equal(MessageTypeResponse, response.Type)
    s.NotNil(response.Result)
}

func (s *WebSocketTestSuite) TestConcurrentConnections() {
    ctx := context.Background()
    numConns := 100
    
    // Create multiple connections
    conns := make([]*websocket.Conn, numConns)
    for i := 0; i < numConns; i++ {
        conn, _, err := websocket.Dial(ctx, s.server.URL("/ws"), nil)
        s.Require().NoError(err)
        conns[i] = conn
    }
    
    // Send messages concurrently
    errCh := make(chan error, numConns)
    for i, conn := range conns {
        go func(idx int, c *websocket.Conn) {
            msg := &Message{
                ID:     fmt.Sprintf("msg-%d", idx),
                Type:   MessageTypeRequest,
                Method: "tool.list",
            }
            
            if err := wsjson.Write(ctx, c, msg); err != nil {
                errCh <- err
                return
            }
            
            var resp Message
            if err := wsjson.Read(ctx, c, &resp); err != nil {
                errCh <- err
                return
            }
            
            errCh <- nil
        }(i, conn)
    }
    
    // Wait for all responses
    for i := 0; i < numConns; i++ {
        err := <-errCh
        s.NoError(err)
    }
    
    // Clean up
    for _, conn := range conns {
        conn.Close(websocket.StatusNormalClosure, "")
    }
}

func TestWebSocketSuite(t *testing.T) {
    suite.Run(t, new(WebSocketTestSuite))
}
```

### Task 14: Integration Tests with Existing MCP (Day 14)
**File**: `test/integration/websocket_integration_test.go`
```go
package integration

import (
    "context"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "nhooyr.io/websocket"
    "nhooyr.io/websocket/wsjson"
    
    "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
    "github.com/S-Corkum/devops-mcp/test/integration/testutils"
)

// Test WebSocket integration with existing MCP services
func TestWebSocketMCPIntegration(t *testing.T) {
    // Skip if not integration test
    testutils.SkipIfShort(t)
    
    // Setup test environment with all MCP services
    env := testutils.NewTestEnvironment(t)
    defer env.Cleanup()
    
    // Start MCP server with WebSocket enabled
    server := env.StartMCPServer(testutils.ServerConfig{
        EnableWebSocket: true,
        EnableAuth:      true,
        EnableTools:     true,
    })
    
    // Get auth token
    token := env.CreateTestToken("test-agent", "test-tenant")
    
    // Connect WebSocket client
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    conn, _, err := websocket.Dial(ctx, server.WebSocketURL(), &websocket.DialOptions{
        HTTPHeader: map[string][]string{
            "Authorization": {"Bearer " + token},
        },
    })
    require.NoError(t, err)
    defer conn.Close(websocket.StatusNormalClosure, "")
    
    t.Run("ToolExecution", func(t *testing.T) {
        // Execute tool through WebSocket
        request := &websocket.Message{
            ID:     "test-1",
            Type:   websocket.MessageTypeRequest,
            Method: "tool.execute",
            Params: map[string]interface{}{
                "tool": "github.list_repos",
                "args": map[string]string{"org": "test-org"},
            },
        }
        
        err := wsjson.Write(ctx, conn, request)
        require.NoError(t, err)
        
        var response websocket.Message
        err = wsjson.Read(ctx, conn, &response)
        require.NoError(t, err)
        
        assert.Equal(t, request.ID, response.ID)
        assert.Equal(t, websocket.MessageTypeResponse, response.Type)
        assert.NotNil(t, response.Result)
    })
    
    t.Run("ContextManagement", func(t *testing.T) {
        // Create context through WebSocket
        createReq := &websocket.Message{
            ID:     "test-2",
            Type:   websocket.MessageTypeRequest,
            Method: "context.create",
            Params: map[string]interface{}{
                "content": "test context data",
                "metadata": map[string]string{"type": "test"},
            },
        }
        
        err := wsjson.Write(ctx, conn, createReq)
        require.NoError(t, err)
        
        var createResp websocket.Message
        err = wsjson.Read(ctx, conn, &createResp)
        require.NoError(t, err)
        
        contextID := createResp.Result.(map[string]interface{})["id"].(string)
        assert.NotEmpty(t, contextID)
        
        // Update context
        updateReq := &websocket.Message{
            ID:     "test-3",
            Type:   websocket.MessageTypeRequest,
            Method: "context.update",
            Params: map[string]interface{}{
                "id":      contextID,
                "content": "updated context data",
            },
        }
        
        err = wsjson.Write(ctx, conn, updateReq)
        require.NoError(t, err)
        
        var updateResp websocket.Message
        err = wsjson.Read(ctx, conn, &updateResp)
        require.NoError(t, err)
        
        assert.Equal(t, websocket.MessageTypeResponse, updateResp.Type)
    })
    
    t.Run("EventSubscription", func(t *testing.T) {
        // Subscribe to events
        subReq := &websocket.Message{
            ID:     "test-4",
            Type:   websocket.MessageTypeRequest,
            Method: "event.subscribe",
            Params: map[string]interface{}{
                "events": []string{"tool.executed", "context.updated"},
            },
        }
        
        err := wsjson.Write(ctx, conn, subReq)
        require.NoError(t, err)
        
        var subResp websocket.Message
        err = wsjson.Read(ctx, conn, &subResp)
        require.NoError(t, err)
        
        // Trigger an event through REST API
        env.ExecuteTool("github.create_issue", map[string]string{
            "repo":  "test-repo",
            "title": "Test Issue",
        })
        
        // Should receive event notification
        var event websocket.Message
        err = wsjson.Read(ctx, conn, &event)
        require.NoError(t, err)
        
        assert.Equal(t, websocket.MessageTypeNotification, event.Type)
        assert.Equal(t, "tool.executed", event.Method)
    })
}

// Test WebSocket binary protocol
func TestWebSocketBinaryProtocol(t *testing.T) {
    testutils.SkipIfShort(t)
    
    env := testutils.NewTestEnvironment(t)
    defer env.Cleanup()
    
    server := env.StartMCPServer(testutils.ServerConfig{
        EnableWebSocket: true,
        WebSocketConfig: map[string]interface{}{
            "enable_binary_protocol": true,
        },
    })
    
    // Connect with binary subprotocol
    ctx := context.Background()
    conn, _, err := websocket.Dial(ctx, server.WebSocketURL(), &websocket.DialOptions{
        Subprotocols: []string{"mcp.v1.binary"},
    })
    require.NoError(t, err)
    defer conn.Close(websocket.StatusNormalClosure, "")
    
    // Send binary message
    header := &websocket.BinaryHeader{
        Magic:      0x4D435057,
        Version:    1,
        Type:       websocket.MessageTypeRequest,
        SequenceID: 1,
        Method:     websocket.MethodToolList,
        DataSize:   0,
    }
    
    buf := &bytes.Buffer{}
    err = websocket.WriteBinaryHeader(buf, header)
    require.NoError(t, err)
    
    err = conn.Write(ctx, websocket.MessageBinary, buf.Bytes())
    require.NoError(t, err)
    
    // Read binary response
    _, data, err := conn.Read(ctx)
    require.NoError(t, err)
    
    respHeader, _, err := websocket.ParseBinaryHeader(bytes.NewReader(data))
    require.NoError(t, err)
    
    assert.Equal(t, header.SequenceID, respHeader.SequenceID)
    assert.Equal(t, websocket.MessageTypeResponse, respHeader.Type)
}
```

## Performance & Load Testing

### Task 13: Functional Tests (Day 13)
**File**: `test/functional/websocket/websocket_functional_test.go`
```go
package websocket_test

import (
    "context"
    "fmt"
    "sync"
    "testing"
    "time"
    
    "github.com/stretchr/testify/suite"
    "nhooyr.io/websocket"
    
    "github.com/S-Corkum/devops-mcp/test/functional/foundation"
)

type WebSocketFunctionalSuite struct {
    suite.Suite
    foundation *foundation.TestContext
    serverURL  string
}

func (s *WebSocketFunctionalSuite) SetupSuite() {
    s.foundation = foundation.NewTestContext(s.T())
    s.serverURL = s.foundation.GetWebSocketURL()
}

func (s *WebSocketFunctionalSuite) TestAuthentication() {
    tests := []struct {
        name      string
        token     string
        expectErr bool
    }{
        {
            name:      "ValidToken",
            token:     s.foundation.CreateToken("agent-1", "tenant-1"),
            expectErr: false,
        },
        {
            name:      "InvalidToken",
            token:     "invalid-token",
            expectErr: true,
        },
        {
            name:      "ExpiredToken",
            token:     s.foundation.CreateExpiredToken(),
            expectErr: true,
        },
        {
            name:      "NoToken",
            token:     "",
            expectErr: true,
        },
    }
    
    for _, tt := range tests {
        s.Run(tt.name, func() {
            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            defer cancel()
            
            opts := &websocket.DialOptions{}
            if tt.token != "" {
                opts.HTTPHeader = map[string][]string{
                    "Authorization": {"Bearer " + tt.token},
                }
            }
            
            conn, _, err := websocket.Dial(ctx, s.serverURL, opts)
            if tt.expectErr {
                s.Error(err)
            } else {
                s.NoError(err)
                conn.Close(websocket.StatusNormalClosure, "")
            }
        })
    }
}

func (s *WebSocketFunctionalSuite) TestRateLimiting() {
    // Connect client
    conn := s.foundation.ConnectWebSocket("agent-1", "tenant-1")
    defer conn.Close()
    
    // Send messages rapidly
    burst := 150 // Assuming rate limit is 100/min with burst 100
    errors := 0
    
    for i := 0; i < burst; i++ {
        msg := s.foundation.CreateMessage("test", fmt.Sprintf("msg-%d", i), nil)
        err := conn.Send(msg)
        if err != nil {
            errors++
        }
        
        resp, err := conn.Receive()
        if err == nil && resp.Error != nil && resp.Error.Code == 4002 {
            errors++ // Rate limit error
        }
    }
    
    // Should have some rate limit errors
    s.Greater(errors, 0, "Expected rate limiting to kick in")
}

func (s *WebSocketFunctionalSuite) TestConnectionLimits() {
    // Get max connections from config
    maxConns := s.foundation.GetConfig().WebSocket.MaxConnections
    
    // Create connections up to limit
    conns := make([]*foundation.WebSocketClient, 0, maxConns+10)
    
    // Connect up to limit
    for i := 0; i < maxConns; i++ {
        conn := s.foundation.ConnectWebSocket(
            fmt.Sprintf("agent-%d", i),
            "tenant-1",
        )
        s.NotNil(conn)
        conns = append(conns, conn)
    }
    
    // Try to exceed limit
    for i := 0; i < 10; i++ {
        conn, err := s.foundation.TryConnectWebSocket(
            fmt.Sprintf("agent-extra-%d", i),
            "tenant-1",
        )
        if err == nil {
            conns = append(conns, conn)
        }
    }
    
    // Should not exceed max connections
    s.LessOrEqual(len(conns), maxConns)
    
    // Cleanup
    for _, conn := range conns {
        conn.Close()
    }
}

func (s *WebSocketFunctionalSuite) TestMessageOrdering() {
    conn := s.foundation.ConnectWebSocket("agent-1", "tenant-1")
    defer conn.Close()
    
    // Send multiple messages
    numMessages := 100
    sent := make([]string, numMessages)
    
    for i := 0; i < numMessages; i++ {
        id := fmt.Sprintf("msg-%d", i)
        sent[i] = id
        
        msg := s.foundation.CreateMessage("echo", id, map[string]interface{}{
            "data": fmt.Sprintf("test-data-%d", i),
        })
        
        err := conn.Send(msg)
        s.NoError(err)
    }
    
    // Receive responses
    received := make([]string, 0, numMessages)
    for i := 0; i < numMessages; i++ {
        resp, err := conn.Receive()
        s.NoError(err)
        received = append(received, resp.ID)
    }
    
    // Verify order preserved
    s.Equal(sent, received, "Message order should be preserved")
}

func (s *WebSocketFunctionalSuite) TestConcurrentOperations() {
    numClients := 50
    numMessages := 100
    
    var wg sync.WaitGroup
    errors := make(chan error, numClients*numMessages)
    
    for i := 0; i < numClients; i++ {
        wg.Add(1)
        go func(clientID int) {
            defer wg.Done()
            
            conn := s.foundation.ConnectWebSocket(
                fmt.Sprintf("agent-%d", clientID),
                "tenant-1",
            )
            defer conn.Close()
            
            // Send messages concurrently
            for j := 0; j < numMessages; j++ {
                msg := s.foundation.CreateMessage(
                    "tool.execute",
                    fmt.Sprintf("client-%d-msg-%d", clientID, j),
                    map[string]interface{}{
                        "tool": "echo",
                        "args": map[string]string{"data": "test"},
                    },
                )
                
                if err := conn.Send(msg); err != nil {
                    errors <- err
                    continue
                }
                
                if _, err := conn.Receive(); err != nil {
                    errors <- err
                }
            }
        }(i)
    }
    
    wg.Wait()
    close(errors)
    
    // Check for errors
    errorCount := 0
    for err := range errors {
        if err != nil {
            errorCount++
            s.T().Logf("Error: %v", err)
        }
    }
    
    // Allow small percentage of errors
    maxErrors := (numClients * numMessages) / 100 // 1%
    s.LessOrEqual(errorCount, maxErrors, "Too many errors in concurrent operations")
}

func (s *WebSocketFunctionalSuite) TestGracefulReconnection() {
    conn := s.foundation.ConnectWebSocket("agent-1", "tenant-1")
    
    // Send initial message
    msg1 := s.foundation.CreateMessage("echo", "msg-1", nil)
    err := conn.Send(msg1)
    s.NoError(err)
    
    resp1, err := conn.Receive()
    s.NoError(err)
    s.Equal("msg-1", resp1.ID)
    
    // Simulate connection drop
    conn.Close()
    
    // Reconnect
    conn = s.foundation.ConnectWebSocket("agent-1", "tenant-1")
    defer conn.Close()
    
    // Should be able to continue
    msg2 := s.foundation.CreateMessage("echo", "msg-2", nil)
    err = conn.Send(msg2)
    s.NoError(err)
    
    resp2, err := conn.Receive()
    s.NoError(err)
    s.Equal("msg-2", resp2.ID)
}

func (s *WebSocketFunctionalSuite) TestBinaryProtocol() {
    // Skip if binary protocol not enabled
    if !s.foundation.GetConfig().WebSocket.BinaryProtocol.Enabled {
        s.T().Skip("Binary protocol not enabled")
    }
    
    conn := s.foundation.ConnectWebSocketBinary("agent-1", "tenant-1")
    defer conn.Close()
    
    // Send binary message
    header := &websocket.BinaryHeader{
        Magic:      0x4D435057,
        Version:    1,
        Type:       websocket.MessageTypeRequest,
        SequenceID: 1,
        Method:     websocket.MethodToolList,
    }
    
    err := conn.SendBinary(header, nil)
    s.NoError(err)
    
    // Receive binary response
    respHeader, data, err := conn.ReceiveBinary()
    s.NoError(err)
    
    s.Equal(header.SequenceID, respHeader.SequenceID)
    s.Equal(websocket.MessageTypeResponse, respHeader.Type)
    s.NotNil(data)
}

func TestWebSocketFunctionalSuite(t *testing.T) {
    suite.Run(t, new(WebSocketFunctionalSuite))
}
```

### Task 14: Load Testing (Day 14)
**File**: `test/load/websocket_load_test.go`
```go
package load

import (
    "context"
    "sync"
    "sync/atomic"
    "testing"
    "time"
)

func BenchmarkWebSocketThroughput(b *testing.B) {
    server := StartTestServer()
    defer server.Stop()
    
    numClients := 1000
    messagesPerClient := b.N / numClients
    
    var (
        totalMessages atomic.Int64
        totalErrors   atomic.Int64
    )
    
    // Create clients
    clients := make([]*TestClient, numClients)
    for i := 0; i < numClients; i++ {
        client, err := NewTestClient(server.URL())
        if err != nil {
            b.Fatal(err)
        }
        clients[i] = client
    }
    
    // Run load test
    b.ResetTimer()
    start := time.Now()
    
    var wg sync.WaitGroup
    for _, client := range clients {
        wg.Add(1)
        go func(c *TestClient) {
            defer wg.Done()
            
            for i := 0; i < messagesPerClient; i++ {
                err := c.SendMessage(&Message{
                    ID:     fmt.Sprintf("msg-%d", i),
                    Type:   MessageTypeRequest,
                    Method: "tool.execute",
                    Params: map[string]string{"tool": "test"},
                })
                
                if err != nil {
                    totalErrors.Add(1)
                } else {
                    totalMessages.Add(1)
                }
            }
        }(client)
    }
    
    wg.Wait()
    elapsed := time.Since(start)
    
    // Report metrics
    msgsPerSec := float64(totalMessages.Load()) / elapsed.Seconds()
    b.ReportMetric(msgsPerSec, "msgs/sec")
    b.ReportMetric(float64(totalErrors.Load()), "errors")
    b.ReportMetric(float64(numClients), "clients")
}
```

## Implementation Checklist for Claude Code

### Pre-Implementation Setup
- [ ] Create feature branch: `git checkout -b feature/websocket-server`
- [ ] Add dependencies to go.mod:
  ```
  nhooyr.io/websocket v1.8.10
  github.com/golang-jwt/jwt/v5 v5.2.0
  github.com/klauspost/compress v1.17.4
  github.com/puzpuzpuz/xsync/v3 v3.0.2
  ```
- [ ] Create directory structure:
  ```
  mkdir -p apps/mcp-server/internal/api/websocket
  mkdir -p pkg/models/websocket
  mkdir -p test/functional/websocket
  ```

### Implementation Order
1. [ ] **Task 1**: Core data structures (types.go)
2. [ ] **Task 2**: Basic WebSocket server (server.go)
3. [ ] **Task 3**: Connection management (connection.go)
4. [ ] **Task 4**: Message processing (handlers.go)
5. [ ] **Task 5**: Binary protocol (binary.go)
6. [ ] **Task 6**: Authentication & security (auth.go)
7. [ ] **Task 7**: Integration with MCP server
8. [ ] **Task 8**: Configuration updates
9. [ ] **Task 9**: Performance optimization (pool.go)
10. [ ] **Task 10**: Message batching (batch.go)
11. [ ] **Task 11**: Monitoring & metrics
12. [ ] **Task 12**: Unit tests for all components
13. [ ] **Task 13**: Functional tests
14. [ ] **Task 14**: Integration tests with MCP
15. [ ] **Task 15**: Load testing
16. [ ] **Task 16**: Documentation updates

### Task 16: Documentation Updates (Day 15)
**File**: `docs/api-reference/websocket-api-reference.md` (NEW)
```markdown
# WebSocket API Reference

## Overview

The MCP WebSocket API provides real-time, bidirectional communication for AI agents and IDEs. This API supports both JSON and binary protocols for optimal performance.

## Connection

### Endpoint
```
wss://{host}/ws
```

### Authentication
- Bearer token in Authorization header
- Token validated on connection
- Per-message HMAC signatures (optional)

### Subprotocols
- `mcp.v1` - JSON protocol (default)
- `mcp.v1.binary` - Binary protocol

## Message Format

### JSON Protocol
```json
{
  "id": "unique-message-id",
  "type": 0,  // 0=request, 1=response, 2=notification, 3=error
  "method": "tool.execute",
  "params": {},  // Method-specific parameters
  "result": {},  // Response only
  "error": {}    // Error only
}
```

### Binary Protocol
```
+--------+--------+--------+--------+
| Magic (4 bytes) | 0x4D435057      |
+--------+--------+--------+--------+
| Ver|Typ| Flags  | Method ID       |
+--------+--------+--------+--------+
| Sequence ID (8 bytes)             |
+--------+--------+--------+--------+
| Reserved | Data Size (4 bytes)    |
+--------+--------+--------+--------+
```

## Methods

### Tool Operations
- `tool.list` - List available tools
- `tool.execute` - Execute a tool
- `tool.cancel` - Cancel tool execution

### Context Management
- `context.create` - Create new context
- `context.get` - Retrieve context
- `context.update` - Update context
- `context.delete` - Delete context
- `context.list` - List contexts
- `context.search` - Search within context

### Event Subscriptions
- `event.subscribe` - Subscribe to events
- `event.unsubscribe` - Unsubscribe from events

## Error Codes
- 4000 - Invalid message format
- 4001 - Authentication failed
- 4002 - Rate limit exceeded
- 4003 - Internal server error
- 4004 - Method not found
- 4005 - Invalid parameters

## Rate Limiting
- Default: 1000 requests/minute
- Burst: 100 requests
- Per-connection limits
- Global server limits

## Examples

### Connect and Authenticate
```javascript
const ws = new WebSocket('wss://api.example.com/ws', {
  headers: {
    'Authorization': 'Bearer ' + token
  }
});
```

### Execute Tool
```javascript
ws.send(JSON.stringify({
  id: "123",
  type: 0,
  method: "tool.execute",
  params: {
    tool: "github.list_repos",
    args: { org: "example" }
  }
}));
```

### Subscribe to Events
```javascript
ws.send(JSON.stringify({
  id: "456",
  type: 0,
  method: "event.subscribe",
  params: {
    events: ["tool.executed", "context.updated"]
  }
}));
```
```

**File**: `docs/architecture/websocket-architecture.md` (NEW)
```markdown
# WebSocket Architecture

## Design Overview

The MCP WebSocket server implements a high-performance, secure real-time communication layer optimized for AI agents and developer tools.

### Key Design Principles
1. **Performance First**: Binary protocol, zero-allocation design
2. **Security by Default**: JWT auth, rate limiting, HMAC signatures
3. **Scalability**: 100K+ concurrent connections per server
4. **Reliability**: Graceful reconnection, message ordering

## Component Architecture

```
┌─────────────────┐     ┌──────────────────┐
│   WebSocket     │────▶│  Authentication  │
│    Handler      │     └──────────────────┘
└────────┬────────┘              
         │                       
         ▼                       
┌─────────────────┐     ┌──────────────────┐
│  Connection     │────▶│   Rate Limiter   │
│    Manager      │     └──────────────────┘
└────────┬────────┘              
         │                       
         ▼                       
┌─────────────────┐     ┌──────────────────┐
│   Message       │────▶│  Message Router  │
│   Processor     │     └──────────────────┘
└────────┬────────┘              
         │                       
         ▼                       
┌─────────────────┐     ┌──────────────────┐
│    Handler      │────▶│   MCP Services   │
│   Registry      │     │  (Tools, Context)│
└─────────────────┘     └──────────────────┘
```

## Performance Optimizations

### Connection Pooling
- Pre-allocated connection objects
- CPU-sharded pools for cache locality
- Zero-allocation recycling

### Message Processing
- Lock-free message queues
- Batch processing for network efficiency
- Binary protocol for 10x performance

### Memory Management
- Object pools for all allocations
- Ring buffers for message queues
- Minimal GC pressure design

## Security Architecture

### Authentication Flow
1. JWT validation on connection
2. Claims extraction and caching
3. Per-message authorization checks
4. Optional HMAC signatures

### Rate Limiting
- Token bucket algorithm
- Per-connection limits
- Global server limits
- Automatic backpressure

### Attack Mitigation
- Connection limits per IP
- Message size limits
- Timeout protection
- Input validation

## Deployment Architecture

### Kubernetes Deployment
```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: mcp-websocket
spec:
  replicas: 3
  serviceName: mcp-websocket
  template:
    spec:
      containers:
      - name: mcp-server
        env:
        - name: WEBSOCKET_ENABLED
          value: "true"
        - name: MAX_CONNECTIONS
          value: "100000"
        resources:
          requests:
            memory: "4Gi"
            cpu: "2"
          limits:
            memory: "8Gi"
            cpu: "4"
```

### Load Balancing
- Sticky sessions for WebSocket
- Health check endpoints
- Graceful connection draining

## Monitoring & Observability

### Metrics
- Connection count (active/total)
- Message throughput
- Latency percentiles
- Error rates

### Logging
- Structured JSON logs
- Correlation IDs
- Performance tracing

### Dashboards
- Real-time connection monitoring
- Performance metrics
- Error analysis
```

**File**: `docs/operations/websocket-operations.md` (NEW)
```markdown
# WebSocket Operations Guide

## Deployment

### Prerequisites
- Kubernetes 1.25+
- Ingress with WebSocket support
- Redis for session management

### Configuration
```yaml
websocket:
  enabled: true
  max_connections: 100000
  read_buffer_size: 4096
  write_buffer_size: 4096
  ping_interval: 30s
  pong_timeout: 60s
  binary_protocol:
    enabled: true
  compression:
    enabled: true
    threshold: 1024
  rate_limit:
    requests_per_minute: 1000
    burst: 100
```

### Environment Variables
```bash
WEBSOCKET_ENABLED=true
WEBSOCKET_MAX_CONNECTIONS=100000
WEBSOCKET_BINARY_PROTOCOL=true
WEBSOCKET_COMPRESSION=true
```

## Monitoring

### Key Metrics
- `websocket_connections_active` - Current connections
- `websocket_messages_received_total` - Message count
- `websocket_message_latency_seconds` - Processing time
- `websocket_errors_total` - Error count

### Prometheus Queries
```promql
# Connection growth rate
rate(websocket_connections_total[5m])

# Message throughput
rate(websocket_messages_received_total[1m])

# Error rate
rate(websocket_errors_total[5m])

# Latency percentiles
histogram_quantile(0.99, websocket_message_latency_seconds)
```

### Grafana Dashboard
Import dashboard ID: 15423

## Troubleshooting

### High Memory Usage
1. Check connection count: `curl /metrics | grep websocket_connections_active`
2. Review buffer sizes in config
3. Enable connection limits per IP

### Connection Drops
1. Check ping/pong intervals
2. Review proxy timeout settings
3. Verify load balancer sticky sessions

### Performance Issues
1. Enable binary protocol
2. Adjust message batching
3. Review rate limits
4. Check CPU sharding

## Scaling

### Horizontal Scaling
```bash
kubectl scale statefulset mcp-websocket --replicas=5
```

### Vertical Scaling
```yaml
resources:
  requests:
    memory: "8Gi"
    cpu: "4"
  limits:
    memory: "16Gi"
    cpu: "8"
```

### Connection Distribution
- Use consistent hashing in load balancer
- Enable session affinity
- Monitor connection imbalance

## Security

### TLS Configuration
```yaml
tls:
  cert_file: /etc/tls/cert.pem
  key_file: /etc/tls/key.pem
  min_version: "1.3"
  cipher_suites:
    - TLS_AES_128_GCM_SHA256
    - TLS_AES_256_GCM_SHA384
    - TLS_CHACHA20_POLY1305_SHA256
```

### Rate Limiting
```yaml
rate_limit:
  enabled: true
  requests_per_minute: 1000
  burst: 100
  by_ip: true
  by_user: true
```

### Access Control
- JWT validation required
- IP allowlisting available
- Per-method authorization

## Maintenance

### Rolling Updates
```bash
kubectl rollout restart statefulset mcp-websocket
```

### Connection Draining
```bash
# Mark pod for deletion
kubectl label pod mcp-websocket-0 drain=true

# Wait for connections to close
kubectl exec mcp-websocket-0 -- /health/drain

# Delete pod
kubectl delete pod mcp-websocket-0
```

### Backup & Recovery
- Stateless design - no backup needed
- Session state in Redis
- Automatic reconnection handling
```

**File**: `configs/config.base.yaml` (UPDATE - add detailed WebSocket config)
```yaml
# WebSocket Configuration
websocket:
  # Enable WebSocket server
  enabled: true
  
  # Connection limits
  max_connections: 100000
  max_connections_per_ip: 100
  
  # Buffer sizes
  read_buffer_size: 4096
  write_buffer_size: 4096
  max_message_size: 1048576  # 1MB
  
  # Timeouts
  ping_interval: 30s
  pong_timeout: 60s
  write_timeout: 10s
  
  # Binary protocol
  binary_protocol:
    enabled: true
    fallback_to_json: true
  
  # Compression
  compression:
    enabled: true
    threshold: 1024  # Compress messages > 1KB
    level: 6  # 1-9, higher = better compression
  
  # Rate limiting
  rate_limit:
    enabled: true
    requests_per_minute: 1000
    burst: 100
    by_ip: true
    by_user: true
  
  # Performance
  performance:
    connection_pool_size: 1000
    message_batch_size: 100
    message_batch_timeout: 10ms
    cpu_sharding: true
    numa_aware: true
  
  # Security
  security:
    require_auth: true
    hmac_signatures: false
    allowed_origins:
      - "https://*.example.com"
    max_frame_size: 1048576
```

**File**: `README.md` (UPDATE - add WebSocket section)
```markdown
## WebSocket Support

The MCP platform now includes a high-performance WebSocket server for real-time communication:

### Features
- **Extreme Performance**: 100K+ concurrent connections, sub-millisecond latency
- **Binary Protocol**: 10x faster than JSON for high-throughput scenarios
- **Security**: JWT auth, rate limiting, optional HMAC signatures
- **Reliability**: Automatic reconnection, message ordering, graceful shutdown

### Quick Start
```javascript
// Connect to WebSocket
const ws = new WebSocket('wss://api.example.com/ws', {
  headers: { 'Authorization': 'Bearer ' + token }
});

// Send message
ws.send(JSON.stringify({
  id: "123",
  type: 0,
  method: "tool.execute",
  params: { tool: "github.list_repos" }
}));
```

### Configuration
Enable WebSocket in your config:
```yaml
websocket:
  enabled: true
  max_connections: 100000
  binary_protocol:
    enabled: true
```

See [WebSocket Documentation](docs/api-reference/websocket-api-reference.md) for details.
```

**File**: `docs/migration/websocket-migration-guide.md` (NEW)
```markdown
# WebSocket Migration Guide

## Overview

This guide helps you migrate from REST API polling to WebSocket real-time communication.

## Benefits of Migration

### Performance
- 10-100x reduction in latency
- 90% reduction in bandwidth usage
- Eliminate polling overhead

### Features
- Real-time event notifications
- Bidirectional communication
- Connection state management

## Migration Steps

### 1. Update Client Libraries
```bash
npm install @mcp/websocket-client@latest
```

### 2. Replace Polling with WebSocket
**Before (REST Polling):**
```javascript
// Poll for updates
setInterval(async () => {
  const response = await fetch('/api/v1/events');
  const events = await response.json();
  processEvents(events);
}, 5000);
```

**After (WebSocket):**
```javascript
// Real-time updates
const ws = new MCPWebSocket(url, { auth: token });

ws.on('event', (event) => {
  processEvent(event);
});

ws.subscribe(['tool.executed', 'context.updated']);
```

### 3. Handle Connection Lifecycle
```javascript
ws.on('open', () => {
  console.log('Connected');
  // Re-subscribe to events
});

ws.on('close', () => {
  console.log('Disconnected');
  // Automatic reconnection handled
});

ws.on('error', (error) => {
  console.error('WebSocket error:', error);
});
```

### 4. Implement Message Handling
```javascript
// Request-response pattern
const result = await ws.request('tool.execute', {
  tool: 'github.create_issue',
  args: { title: 'Test Issue' }
});

// Fire-and-forget
ws.send('context.update', {
  id: 'ctx-123',
  content: 'Updated content'
});
```

## API Mapping

| REST API | WebSocket Method |
|----------|------------------|
| GET /api/v1/tools | tool.list |
| POST /api/v1/tools/:id/execute | tool.execute |
| GET /api/v1/contexts | context.list |
| GET /api/v1/contexts/:id | context.get |
| POST /api/v1/contexts | context.create |
| PUT /api/v1/contexts/:id | context.update |
| DELETE /api/v1/contexts/:id | context.delete |

## Best Practices

### Error Handling
```javascript
ws.on('error', (error) => {
  if (error.code === 4001) {
    // Re-authenticate
    refreshToken();
  } else if (error.code === 4002) {
    // Rate limited - back off
    setTimeout(() => retry(), 5000);
  }
});
```

### State Management
```javascript
class MCPClient {
  constructor() {
    this.ws = null;
    this.subscriptions = new Set();
    this.pendingRequests = new Map();
  }
  
  connect() {
    this.ws = new MCPWebSocket(url);
    
    this.ws.on('open', () => {
      // Re-establish subscriptions
      this.subscriptions.forEach(event => {
        this.ws.subscribe([event]);
      });
    });
  }
}
```

### Performance Optimization
```javascript
// Batch messages
const batcher = new MessageBatcher(ws);

for (let i = 0; i < 1000; i++) {
  batcher.add('tool.execute', { tool: 'process', id: i });
}

batcher.flush(); // Sends as single batch
```

## Testing

### Unit Tests
```javascript
describe('WebSocket Client', () => {
  it('should handle reconnection', async () => {
    const ws = new MCPWebSocket(url);
    
    // Simulate disconnect
    ws.disconnect();
    
    // Should auto-reconnect
    await wait(1000);
    expect(ws.connected).toBe(true);
  });
});
```

### Integration Tests
```javascript
it('should execute tools via WebSocket', async () => {
  const ws = new MCPWebSocket(testServer.url);
  
  const result = await ws.request('tool.execute', {
    tool: 'echo',
    args: { message: 'test' }
  });
  
  expect(result.output).toBe('test');
});
```

## Rollback Plan

If issues arise, you can run both REST and WebSocket in parallel:

```javascript
class HybridClient {
  constructor() {
    this.ws = new MCPWebSocket(url);
    this.rest = new MCPRestClient(url);
  }
  
  async execute(tool, args) {
    try {
      // Try WebSocket first
      return await this.ws.request('tool.execute', { tool, args });
    } catch (error) {
      // Fall back to REST
      return await this.rest.post(`/tools/${tool}/execute`, args);
    }
  }
}
```

## Support

- Documentation: [WebSocket API Reference](../api-reference/websocket-api-reference.md)
- Examples: [WebSocket Examples](../examples/websocket-examples.md)
- Issues: https://github.com/S-Corkum/devops-mcp/issues
```

### Testing at Each Step
- After Task 1: Run `go test ./pkg/models/websocket/...`
- After Task 2-3: Test basic connection with wscat
- After Task 4: Test message round-trip
- After Task 5: Benchmark binary vs JSON
- After Task 6: Verify authentication works
- After Task 7: Test full integration
- After Task 9-10: Run performance benchmarks
- After Task 12: Run unit test suite with coverage
- After Task 13: Run functional test suite
- After Task 14: Run integration tests with full MCP
- After Task 15: Verify 10K+ connections work
- After Task 16: Review all documentation is complete

## Common Implementation Patterns

### Error Handling Pattern
```go
// Consistent error handling across all components
type WebSocketError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Details any    `json:"details,omitempty"`
}

func (e *WebSocketError) Error() string {
    return fmt.Sprintf("WebSocket error %d: %s", e.Code, e.Message)
}

// Standard error codes
const (
    ErrCodeInvalidMessage = 4000
    ErrCodeAuthFailed     = 4001
    ErrCodeRateLimited    = 4002
    ErrCodeServerError    = 4003
)
```

### Context Pattern
```go
// Use context for cancellation and timeouts
func (s *Server) processWithTimeout(ctx context.Context, fn func() error) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    done := make(chan error, 1)
    go func() {
        done <- fn()
    }()
    
    select {
    case err := <-done:
        return err
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

### Testing Helpers
```go
// Reusable test server
type TestServer struct {
    *httptest.Server
    wsServer *Server
}

func NewTestServer() *TestServer {
    // Create test dependencies
    auth := &MockAuthService{}
    metrics := &MockMetrics{}
    logger := &MockLogger{}
    
    config := Config{
        MaxConnections:  100,
        ReadBufferSize:  4096,
        WriteBufferSize: 4096,
    }
    
    wsServer := NewServer(auth, metrics, logger, config)
    
    mux := http.NewServeMux()
    mux.HandleFunc("/ws", wsServer.HandleWebSocket)
    
    return &TestServer{
        Server:   httptest.NewServer(mux),
        wsServer: wsServer,
    }
}
```

## File Structure Summary

```
apps/mcp-server/
├── internal/
│   └── api/
│       ├── websocket/
│       │   ├── server.go          # Main WebSocket server
│       │   ├── connection.go      # Connection management
│       │   ├── handlers.go        # Message handlers
│       │   ├── auth.go           # Authentication
│       │   ├── pool.go           # Connection pooling
│       │   ├── batch.go          # Message batching
│       │   ├── metrics.go        # Prometheus metrics
│       │   └── *_test.go         # Unit tests
│       └── server.go             # UPDATE: Add WebSocket
└── cmd/server/main.go            # UPDATE: Enable WebSocket

pkg/
├── models/
│   └── websocket/
│       ├── types.go              # Core types
│       ├── binary.go             # Binary protocol
│       └── *_test.go             # Unit tests
└── client/
    └── websocket/
        ├── client.go             # WebSocket client
        └── client_test.go        # Client tests

configs/
└── config.base.yaml              # UPDATE: Add WebSocket config

test/
├── functional/
│   └── websocket/
│       └── websocket_test.go     # Functional tests
└── load/
    └── websocket_load_test.go    # Load tests
```

## Expected Performance (Go Implementation)

### Phase 1: Basic WebSocket (JSON)
- **Connections**: 10K concurrent
- **Latency**: < 5ms (p99)
- **Throughput**: 100K messages/second
- **Memory**: < 100KB per connection

### Phase 2: With Binary Protocol
- **Connections**: 50K concurrent
- **Latency**: < 1ms (p99)
- **Throughput**: 500K messages/second
- **Memory**: < 20KB per connection

### Phase 3: Fully Optimized
- **Connections**: 100K+ concurrent
- **Latency**: < 500μs (p99)
- **Throughput**: 1M+ messages/second
- **Memory**: < 10KB per connection

### Security Features (All Phases)
- **JWT Authentication**: On every connection
- **HMAC Signatures**: Optional per-message
- **Rate Limiting**: Per connection and global
- **TLS 1.3**: With modern cipher suites

## Testing Commands for Claude Code

### Unit Tests (After each task)
```bash
# Run specific package tests
go test -v ./pkg/models/websocket/...
go test -v ./apps/mcp-server/internal/api/websocket/...

# Run with race detector
go test -race ./...

# Run with coverage
go test -cover ./...
```

### Manual Testing
```bash
# Test with wscat
npm install -g wscat
wscat -c ws://localhost:8080/ws -H "Authorization: Bearer $JWT_TOKEN"

# Send test message
> {"id":"1","type":0,"method":"tool.list"}
```

### Benchmarks
```bash
# Run benchmarks
go test -bench=. ./pkg/models/websocket/...
go test -bench=. -benchmem ./apps/mcp-server/internal/api/websocket/...

# Profile CPU
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof

# Profile memory
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof
```

### Load Testing
```bash
# Run load test
go test -bench=BenchmarkWebSocketThroughput ./test/load/... -benchtime=30s

# Monitor during test
curl http://localhost:9090/metrics | grep websocket
```

## Summary for Claude Code Implementation

This plan is optimized for Claude Code with Opus 4 to implement successfully:

1. **Discrete Tasks**: Each task is independent and testable
2. **Clear Dependencies**: Tasks build on each other logically
3. **Complete Code**: All examples are runnable Go code
4. **Integration Path**: Clear steps to integrate with existing MCP
5. **Testing Strategy**: Tests provided for each component
6. **Performance Path**: Start simple, optimize incrementally

The implementation follows Go best practices and integrates cleanly with the existing MCP architecture. Each task can be completed and tested independently, ensuring progress is measurable and issues are caught early.

**Expected Timeline**: 15 days for complete implementation with comprehensive testing.

## Test Execution Strategy

### Unit Tests (Continuous)
```bash
# Run after each component
go test -v -race -cover ./apps/mcp-server/internal/api/websocket/...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Functional Tests (Daily)
```bash
# Run functional test suite
go test -v ./test/functional/websocket/... -tags=functional

# Run with specific test
go test -v ./test/functional/websocket/... -run TestAuthentication
```

### Integration Tests (After Integration)
```bash
# Start test environment
docker-compose -f docker-compose.test.yml up -d

# Run integration tests
go test -v ./test/integration/... -tags=integration

# Cleanup
docker-compose -f docker-compose.test.yml down
```

### Load Tests (Final Validation)
```bash
# Run basic load test
go test -bench=. ./test/load/... -benchtime=30s

# Run extended load test
go test -bench=BenchmarkWebSocketThroughput ./test/load/... -benchtime=5m

# Run with profiling
go test -bench=. -cpuprofile=cpu.prof -memprofile=mem.prof ./test/load/...
```