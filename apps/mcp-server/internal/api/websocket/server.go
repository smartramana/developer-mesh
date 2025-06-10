package websocket

import (
    "net/http"
    "strings"
    "sync"
    "time"
    
    "github.com/golang-jwt/jwt/v4"
    "github.com/coder/websocket"
    "github.com/google/uuid"
    
    "github.com/S-Corkum/devops-mcp/pkg/auth"
    ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
    "github.com/S-Corkum/devops-mcp/pkg/observability"
)

type Server struct {
    connections map[string]*Connection
    mu          sync.RWMutex
    handlers    map[string]MessageHandler
    
    auth        auth.Service
    metrics     observability.MetricsClient
    logger      observability.Logger
    
    config      Config
    
    // Dependencies (will be set in Task 7)
    toolRegistry   ToolRegistry
    contextManager ContextManager
    eventBus       EventBus
    
    // Security components
    sessionManager  *SessionManager
    ipRateLimiter   *IPRateLimiter
    antiReplayCache *AntiReplayCache
    
    // Performance components
    connectionPool *ConnectionPoolManager
    batchManager   *BatchManager
    
    // Metrics
    metricsCollector *MetricsCollector
    
    // Server start time
    startTime time.Time
}

type Config struct {
    MaxConnections   int           `mapstructure:"max_connections"`
    ReadBufferSize   int           `mapstructure:"read_buffer_size"`
    WriteBufferSize  int           `mapstructure:"write_buffer_size"`
    PingInterval     time.Duration `mapstructure:"ping_interval"`
    PongTimeout      time.Duration `mapstructure:"pong_timeout"`
    MaxMessageSize   int64         `mapstructure:"max_message_size"`
    
    // Security settings
    Security        SecurityConfig       `mapstructure:"security"`
    RateLimit       RateLimiterConfig    `mapstructure:"rate_limit"`
}

// Connection wraps the WebSocket connection and adds our metadata
type Connection struct {
    *ws.Connection
    conn     *websocket.Conn
    send     chan []byte
    hub      *Server
}

func NewServer(auth auth.Service, metrics observability.MetricsClient, logger observability.Logger, config Config) *Server {
    s := &Server{
        connections: make(map[string]*Connection),
        handlers:    make(map[string]MessageHandler),
        auth:        auth,
        metrics:     metrics,
        logger:      logger,
        config:      config,
        startTime:   time.Now(),
    }
    
    // Initialize security components
    s.sessionManager = NewSessionManager()
    
    // Initialize IP rate limiter if enabled
    if config.RateLimit.PerIP {
        s.ipRateLimiter = NewIPRateLimiter(&config.RateLimit)
    }
    
    // Initialize anti-replay cache
    s.antiReplayCache = NewAntiReplayCache(5 * time.Minute)
    
    // Initialize connection pool for performance
    s.connectionPool = NewConnectionPoolManager(config.MaxConnections)
    
    // Initialize batch manager for message batching
    batchConfig := DefaultBatchConfig()
    s.batchManager = NewBatchManager(batchConfig, logger, metrics)
    
    // Initialize metrics collector
    s.metricsCollector = NewMetricsCollector(metrics)
    
    // Register handlers
    s.RegisterHandlers()
    
    return s
}

func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
    // Check IP rate limit
    if s.ipRateLimiter != nil {
        clientIP := s.getClientIP(r)
        if !s.ipRateLimiter.Allow(clientIP) {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }
    }
    
    // Authenticate request
    claims, err := s.authenticateRequest(r)
    if err != nil {
        s.metricsCollector.RecordConnectionFailure("auth_failed")
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    
    // Check connection limit
    if s.ConnectionCount() >= s.config.MaxConnections {
        s.metricsCollector.RecordConnectionFailure("max_connections")
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
    
    // Set connection limits
    conn.SetReadLimit(s.config.MaxMessageSize)
    
    // Get connection from pool
    connection := s.connectionPool.Get()
    
    // Initialize connection
    connection.Connection = &ws.Connection{
        ID:        uuid.New().String(),
        AgentID:   claims.UserID, // Using UserID as AgentID for now
        TenantID:  claims.TenantID,
        CreatedAt: time.Now(),
    }
    connection.conn = conn
    connection.hub = s
    
    // Set initial state
    connection.SetState(ws.ConnectionStateConnecting)
    
    // Register connection
    s.addConnection(connection)
    
    // Generate session key for HMAC signatures
    if s.config.Security.HMACSignatures {
        _, err := s.sessionManager.GenerateSessionKey(connection.ID)
        if err != nil {
            s.logger.Warn("Failed to generate session key", map[string]interface{}{
                "connection_id": connection.ID,
                "error": err.Error(),
            })
        }
    }
    
    // Set connected state
    connection.SetState(ws.ConnectionStateConnected)
    
    // Start connection handlers
    go connection.writePump()
    go connection.readPump()
    
    s.logger.Info("WebSocket connection established", map[string]interface{}{
        "connection_id": connection.ID,
        "agent_id":      connection.AgentID,
        "tenant_id":     connection.TenantID,
    })
    
    // Record connection metrics
    s.metricsCollector.RecordConnection(connection.TenantID)
}

// ConnectionCount returns the current number of active connections
func (s *Server) ConnectionCount() int {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return len(s.connections)
}

// addConnection registers a new connection
func (s *Server) addConnection(conn *Connection) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.connections[conn.ID] = conn
    
    // Increment metrics
    s.metrics.IncrementCounter("websocket_connections_total", 1)
    s.metrics.RecordGauge("websocket_connections_active", float64(len(s.connections)), nil)
}

// removeConnection unregisters a connection
func (s *Server) removeConnection(conn *Connection) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    if _, ok := s.connections[conn.ID]; ok {
        delete(s.connections, conn.ID)
        
        // Clean up session key
        if s.sessionManager != nil {
            s.sessionManager.RemoveSessionKey(conn.ID)
        }
        
        // Update metrics
        s.metrics.RecordGauge("websocket_connections_active", float64(len(s.connections)), nil)
        
        s.logger.Info("WebSocket connection closed", map[string]interface{}{
            "connection_id": conn.ID,
            "agent_id":      conn.AgentID,
        })
        
        // Record disconnection metrics
        duration := time.Since(conn.CreatedAt)
        s.metricsCollector.RecordDisconnection(conn.TenantID, duration)
        
        // Return connection to pool after cleanup
        s.connectionPool.Put(conn)
    }
}

// GetConnection retrieves a connection by ID
func (s *Server) GetConnection(id string) (*Connection, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    conn, ok := s.connections[id]
    return conn, ok
}

// Broadcast sends a message to all connections for a specific tenant
func (s *Server) Broadcast(tenantID string, message []byte) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    for _, conn := range s.connections {
        if conn.TenantID == tenantID {
            select {
            case conn.send <- message:
            default:
                // Channel full, skip this connection
                s.logger.Warn("Skipping broadcast to connection - channel full", map[string]interface{}{
                    "connection_id": conn.ID,
                })
            }
        }
    }
}

// SendToAgent sends a message to all connections for a specific agent
func (s *Server) SendToAgent(agentID string, message []byte) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    for _, conn := range s.connections {
        if conn.AgentID == agentID {
            select {
            case conn.send <- message:
            default:
                // Channel full, skip this connection
                s.logger.Warn("Skipping message to connection - channel full", map[string]interface{}{
                    "connection_id": conn.ID,
                })
            }
        }
    }
}

// SetToolRegistry sets the tool registry for the server
func (s *Server) SetToolRegistry(registry ToolRegistry) {
    s.toolRegistry = registry
}

// SetContextManager sets the context manager for the server
func (s *Server) SetContextManager(manager ContextManager) {
    s.contextManager = manager
}

// SetEventBus sets the event bus for the server
func (s *Server) SetEventBus(bus EventBus) {
    s.eventBus = bus
}

// Close gracefully shuts down the server
func (s *Server) Close() error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // Close all connections
    for _, conn := range s.connections {
        conn.SetState(ws.ConnectionStateClosing)
        conn.conn.Close(websocket.StatusNormalClosure, "Server shutting down")
    }
    
    // Clear connections map
    s.connections = make(map[string]*Connection)
    
    return nil
}

// authenticateRequest validates the request and returns auth claims
func (s *Server) authenticateRequest(r *http.Request) (*auth.Claims, error) {
    // Check Authorization header
    authHeader := r.Header.Get("Authorization")
    if authHeader == "" {
        return nil, auth.ErrNoAPIKey
    }
    
    // Handle Bearer token
    if strings.HasPrefix(authHeader, "Bearer ") {
        token := strings.TrimPrefix(authHeader, "Bearer ")
        
        // Try to validate as JWT first
        if strings.Count(token, ".") == 2 { // Looks like a JWT
            user, err := s.auth.ValidateJWT(r.Context(), token)
            if err != nil {
                return nil, err
            }
            
            // Convert User to Claims
            return &auth.Claims{
                RegisteredClaims: jwt.RegisteredClaims{
                    Subject: user.ID,
                },
                TenantID: user.TenantID,
                UserID:   user.ID,
                Scopes:   user.Scopes,
            }, nil
        }
        
        // Otherwise try as API key
        user, err := s.auth.ValidateAPIKey(r.Context(), token)
        if err != nil {
            return nil, err
        }
        
        // Convert User to Claims
        return &auth.Claims{
            RegisteredClaims: jwt.RegisteredClaims{
                Subject: user.ID,
            },
            TenantID: user.TenantID,
            UserID:   user.ID,
            Scopes:   user.Scopes,
        }, nil
    }
    
    return nil, auth.ErrInvalidToken
}

// getClientIP extracts the client IP address from the request
func (s *Server) getClientIP(r *http.Request) string {
    // Check X-Forwarded-For header first (for proxies)
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        // Take the first IP in the chain
        if idx := strings.Index(xff, ","); idx != -1 {
            return strings.TrimSpace(xff[:idx])
        }
        return xff
    }
    
    // Check X-Real-IP header
    if xri := r.Header.Get("X-Real-IP"); xri != "" {
        return xri
    }
    
    // Fall back to RemoteAddr
    if idx := strings.LastIndex(r.RemoteAddr, ":"); idx != -1 {
        return r.RemoteAddr[:idx]
    }
    
    return r.RemoteAddr
}