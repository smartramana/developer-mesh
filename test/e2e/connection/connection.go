package connection

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"

	ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

// ConnectionConfig holds connection configuration
type ConnectionConfig struct {
	BaseURL          string
	APIKey           string
	TenantID         string
	MaxRetries       int
	RetryDelay       time.Duration
	Timeout          time.Duration
	TLSConfig        *tls.Config
	Headers          map[string]string
	CompressionLevel int
}

// DefaultConfig returns default connection configuration
func DefaultConfig() *ConnectionConfig {
	return &ConnectionConfig{
		MaxRetries:       3,
		RetryDelay:       2 * time.Second,
		Timeout:          30 * time.Second,
		CompressionLevel: 6,
	}
}

// ConnectionManager manages WebSocket connections
type ConnectionManager struct {
	config *ConnectionConfig
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(config *ConnectionConfig) *ConnectionManager {
	if config == nil {
		config = DefaultConfig()
	}
	return &ConnectionManager{config: config}
}

// EstablishConnection creates a new WebSocket connection
func (cm *ConnectionManager) EstablishConnection(ctx context.Context) (*websocket.Conn, error) {
	wsURL, err := cm.buildWebSocketURL()
	if err != nil {
		return nil, err
	}

	opts := cm.buildDialOptions()

	var conn *websocket.Conn
	var lastErr error

	for attempt := 0; attempt <= cm.config.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(cm.config.RetryDelay)
		}

		dialCtx, cancel := context.WithTimeout(ctx, cm.config.Timeout)
		conn, _, lastErr = websocket.Dial(dialCtx, wsURL, opts)
		cancel()

		if lastErr == nil {
			return conn, nil
		}
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", cm.config.MaxRetries+1, lastErr)
}

// buildWebSocketURL constructs the WebSocket URL
func (cm *ConnectionManager) buildWebSocketURL() (string, error) {
	baseURL := cm.config.BaseURL

	// Parse the base URL
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}

	// Convert HTTP(S) to WS(S)
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
		// Already WebSocket URL
	default:
		return "", fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}

	// Ensure path ends with /ws
	if !strings.HasSuffix(u.Path, "/ws") {
		u.Path = strings.TrimSuffix(u.Path, "/") + "/ws"
	}

	return u.String(), nil
}

// buildDialOptions constructs WebSocket dial options
func (cm *ConnectionManager) buildDialOptions() *websocket.DialOptions {
	headers := http.Header{}

	// Add authorization
	if cm.config.APIKey != "" {
		headers.Set("Authorization", "Bearer "+cm.config.APIKey)
	}

	// Add tenant ID
	if cm.config.TenantID != "" {
		headers.Set("X-Tenant-ID", cm.config.TenantID)
	}

	// Add custom headers
	for key, value := range cm.config.Headers {
		headers.Set(key, value)
	}

	// Add default headers
	headers.Set("X-Client-Type", "e2e-test")
	headers.Set("X-Client-Version", "1.0.0")

	opts := &websocket.DialOptions{
		HTTPHeader:   headers,
		Subprotocols: []string{"mcp.v1"}, // MCP server expects this subprotocol
	}

	// Configure compression
	if cm.config.CompressionLevel > 0 {
		opts.CompressionMode = websocket.CompressionContextTakeover
	}

	// Configure TLS if provided
	if cm.config.TLSConfig != nil {
		opts.HTTPClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: cm.config.TLSConfig,
			},
		}
	}

	return opts
}

// ConnectionHelper provides utility functions for WebSocket connections
type ConnectionHelper struct {
	conn *websocket.Conn
}

// NewConnectionHelper creates a new connection helper
func NewConnectionHelper(conn *websocket.Conn) *ConnectionHelper {
	return &ConnectionHelper{conn: conn}
}

// SendJSON sends a JSON message
func (ch *ConnectionHelper) SendJSON(ctx context.Context, v interface{}) error {
	return wsjson.Write(ctx, ch.conn, v)
}

// ReadJSON reads a JSON message
func (ch *ConnectionHelper) ReadJSON(ctx context.Context, v interface{}) error {
	return wsjson.Read(ctx, ch.conn, v)
}

// SendBinary sends a binary message
func (ch *ConnectionHelper) SendBinary(ctx context.Context, data []byte) error {
	return ch.conn.Write(ctx, websocket.MessageBinary, data)
}

// ReadBinary reads a binary message
func (ch *ConnectionHelper) ReadBinary(ctx context.Context) ([]byte, error) {
	msgType, data, err := ch.conn.Read(ctx)
	if err != nil {
		return nil, err
	}

	if msgType != websocket.MessageBinary {
		return nil, fmt.Errorf("expected binary message, got %v", msgType)
	}

	return data, nil
}

// Ping sends a ping message
func (ch *ConnectionHelper) Ping(ctx context.Context) error {
	return ch.conn.Ping(ctx)
}

// Close closes the connection
func (ch *ConnectionHelper) Close(reason string) error {
	return ch.conn.Close(websocket.StatusNormalClosure, reason)
}

// ConnectionPool manages a pool of WebSocket connections
type ConnectionPool struct {
	manager     *ConnectionManager
	connections []*websocket.Conn
	maxSize     int
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(manager *ConnectionManager, maxSize int) *ConnectionPool {
	return &ConnectionPool{
		manager:     manager,
		connections: make([]*websocket.Conn, 0, maxSize),
		maxSize:     maxSize,
	}
}

// Get retrieves a connection from the pool
func (cp *ConnectionPool) Get(ctx context.Context) (*websocket.Conn, error) {
	if len(cp.connections) > 0 {
		conn := cp.connections[len(cp.connections)-1]
		cp.connections = cp.connections[:len(cp.connections)-1]

		// Test if connection is still alive
		if err := conn.Ping(ctx); err == nil {
			return conn, nil
		}
		// Connection is dead, create a new one
		_ = conn.Close(websocket.StatusGoingAway, "connection dead")
	}

	// Create new connection
	return cp.manager.EstablishConnection(ctx)
}

// Put returns a connection to the pool
func (cp *ConnectionPool) Put(conn *websocket.Conn) {
	if len(cp.connections) < cp.maxSize {
		cp.connections = append(cp.connections, conn)
	} else {
		// Pool is full, close the connection
		_ = conn.Close(websocket.StatusNormalClosure, "pool full")
	}
}

// Close closes all connections in the pool
func (cp *ConnectionPool) Close() {
	for _, conn := range cp.connections {
		_ = conn.Close(websocket.StatusNormalClosure, "pool closing")
	}
	cp.connections = nil
}

// TestConnection tests a connection with initialization
func TestConnection(ctx context.Context, config *ConnectionConfig) error {
	manager := NewConnectionManager(config)

	conn, err := manager.EstablishConnection(ctx)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "test complete")
	}()

	// Send initialization message
	initMsg := ws.Message{
		ID:     uuid.New().String(),
		Type:   ws.MessageTypeRequest,
		Method: "initialize",
		Params: map[string]interface{}{
			"name":    "connection-test",
			"version": "1.0.0",
		},
	}

	if err := wsjson.Write(ctx, conn, initMsg); err != nil {
		return fmt.Errorf("failed to send init: %w", err)
	}

	// Read response
	var response ws.Message
	if err := wsjson.Read(ctx, conn, &response); err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if response.Error != nil {
		return fmt.Errorf("initialization error: %s", response.Error.Message)
	}

	return nil
}
