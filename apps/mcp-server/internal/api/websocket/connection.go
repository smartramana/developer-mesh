package websocket

import (
    "context"
    "encoding/json"
    "time"
    
    "github.com/google/uuid"
    "github.com/coder/websocket"
    "github.com/coder/websocket/wsjson"
    
    ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

// ConnectionState tracks additional connection state
type ConnectionState struct {
    BinaryMode           bool
    CompressionThreshold int
    MaxTokens            int
    CurrentTokenUsage    int
    ActiveSessionID      string
    PreviousSessionID    string
    SystemPromptTokens   int
    ConversationTokens   int
    ToolTokens           int
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

// readPump pumps messages from the websocket connection to the hub
func (c *Connection) readPump() {
    defer func() {
        c.SetState(ws.ConnectionStateClosing)
        c.hub.removeConnection(c)
        if c.conn != nil {
            if err := c.conn.Close(websocket.StatusNormalClosure, ""); err != nil {
                // Log error but don't fail - connection is already being closed
                if c.hub.logger != nil {
                    c.hub.logger.Debug("Error closing WebSocket connection", map[string]interface{}{
                        "error": err.Error(),
                        "connection_id": c.ID,
                    })
                }
            }
        }
    }()
    
    ctx := context.Background()
    
    // Create rate limiter (1000 messages per minute with burst of 100)
    rateLimiter := NewRateLimiter(1000.0/60.0, 100)
    
    for {
        // Check if connection is still active
        if !c.IsActive() {
            return
        }
        
        // Read message using pooled object
        msg := GetMessage()
        defer PutMessage(msg)
        
        err := wsjson.Read(ctx, c.conn, msg)
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
        
        // Update last activity
        c.LastPing = time.Now()
        
        // Rate limiting
        if !rateLimiter.Allow() {
            c.hub.metricsCollector.RecordError("rate_limit")
            c.sendError(msg.ID, ws.ErrCodeRateLimited, "Rate limit exceeded", nil)
            continue
        }
        
        // Process message
        start := time.Now()
        response, err := c.hub.processMessage(ctx, c, msg)
        latency := time.Since(start)
        
        // Record message metrics
        c.hub.metricsCollector.RecordMessage("received", msg.Method, c.TenantID, latency)
        
        if err != nil {
            c.sendError(msg.ID, ws.ErrCodeServerError, err.Error(), nil)
            continue
        }
        
        // Send response if any
        if response != nil {
            c.send <- response
        }
    }
}

// writePump pumps messages from the hub to the websocket connection
func (c *Connection) writePump() {
    ticker := time.NewTicker(c.hub.config.PingInterval)
    defer func() {
        ticker.Stop()
        if c.conn != nil {
            if err := c.conn.Close(websocket.StatusNormalClosure, ""); err != nil {
                // Log error but don't fail - connection is already being closed
                if c.hub.logger != nil {
                    c.hub.logger.Debug("Error closing WebSocket connection in writePump", map[string]interface{}{
                        "error": err.Error(),
                        "connection_id": c.ID,
                    })
                }
            }
        }
    }()
    
    ctx := context.Background()
    
    for {
        select {
        case message, ok := <-c.send:
            if !ok {
                // The hub closed the channel
                if c.conn != nil {
                    if err := c.conn.Close(websocket.StatusNormalClosure, ""); err != nil {
                        if c.hub.logger != nil {
                            c.hub.logger.Debug("Error closing connection when hub closed channel", map[string]interface{}{
                                "error": err.Error(),
                                "connection_id": c.ID,
                            })
                        }
                    }
                }
                return
            }
            
            // Set timeout for write operation
            writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
            defer cancel()
            
            if c.conn != nil {
                if err := c.conn.Write(writeCtx, websocket.MessageText, message); err != nil {
                    c.hub.logger.Error("Write error", map[string]interface{}{
                        "error":         err.Error(),
                        "connection_id": c.ID,
                    })
                    return
                }
            } else {
                c.hub.logger.Error("Connection is nil", map[string]interface{}{
                    "connection_id": c.ID,
                })
                return
            }
            
            // Record sent message
            c.hub.metricsCollector.RecordMessage("sent", "response", c.TenantID, 0)
            
        case <-ticker.C:
            // Send ping to detect disconnected clients
            if c.conn != nil {
                pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
                if err := c.conn.Ping(pingCtx); err != nil {
                    cancel()
                    c.hub.logger.Error("Ping error", map[string]interface{}{
                        "error":         err.Error(),
                        "connection_id": c.ID,
                    })
                    return
                }
                cancel()
            }
        }
    }
}

// sendError sends an error message to the client
func (c *Connection) sendError(requestID string, code int, message string, data interface{}) {
    errorMsg := GetMessage()
    defer PutMessage(errorMsg)
    
    errorMsg.ID = requestID
    errorMsg.Type = ws.MessageTypeError
    errorMsg.Error = &ws.Error{
        Code:    code,
        Message: message,
        Data:    data,
    }
    
    response, err := json.Marshal(errorMsg)
    if err != nil {
        c.hub.logger.Error("Failed to marshal error message", map[string]interface{}{
            "error": err.Error(),
        })
        return
    }
    
    select {
    case c.send <- response:
    default:
        // Channel full, log and drop
        c.hub.logger.Warn("Failed to send error message - channel full", map[string]interface{}{
            "connection_id": c.ID,
        })
        c.hub.metricsCollector.RecordMessageDropped("channel_full")
    }
}

// SendMessage sends a message to the client
func (c *Connection) SendMessage(msg *ws.Message) error {
    data, err := json.Marshal(msg)
    if err != nil {
        return err
    }
    
    select {
    case c.send <- data:
        return nil
    default:
        return ErrChannelFull
    }
}

// SendNotification sends a notification to the client
func (c *Connection) SendNotification(method string, params interface{}) error {
    msg := GetMessage()
    defer PutMessage(msg)
    
    msg.ID = uuid.New().String()
    msg.Type = ws.MessageTypeNotification
    msg.Method = method
    msg.Params = params
    
    return c.SendMessage(msg)
}

// Close closes the connection gracefully
func (c *Connection) Close() error {
    c.SetState(ws.ConnectionStateClosing)
    return c.conn.Close(websocket.StatusNormalClosure, "Connection closed by server")
}


// Custom errors
var (
    ErrChannelFull = &ws.Error{
        Code:    ws.ErrCodeServerError,
        Message: "Message channel full",
    }
)

// Extended Connection methods for new features

// SetBinaryMode enables/disables binary protocol for the connection
func (c *Connection) SetBinaryMode(enabled bool) {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.state == nil {
        c.state = &ConnectionState{}
    }
    c.state.BinaryMode = enabled
}

// IsBinaryMode returns whether binary mode is enabled
func (c *Connection) IsBinaryMode() bool {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.state == nil {
        return false
    }
    return c.state.BinaryMode
}

// SetCompressionThreshold sets the message size threshold for compression
func (c *Connection) SetCompressionThreshold(threshold int) {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.state == nil {
        c.state = &ConnectionState{}
    }
    c.state.CompressionThreshold = threshold
}

// GetCompressionThreshold returns the compression threshold
func (c *Connection) GetCompressionThreshold() int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.state == nil {
        return 0
    }
    return c.state.CompressionThreshold
}

// Token management methods

// SetMaxTokens sets the maximum token window for the connection
func (c *Connection) SetMaxTokens(maxTokens int) {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.state == nil {
        c.state = &ConnectionState{}
    }
    c.state.MaxTokens = maxTokens
}

// GetMaxTokens returns the maximum token window
func (c *Connection) GetMaxTokens() int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.state == nil {
        return 200000 // Default for Claude 3 Opus
    }
    if c.state.MaxTokens == 0 {
        return 200000
    }
    return c.state.MaxTokens
}

// GetTokenUsage returns current token usage
func (c *Connection) GetTokenUsage() int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.state == nil {
        return 0
    }
    return c.state.CurrentTokenUsage
}

// UpdateTokenUsage updates the current token usage
func (c *Connection) UpdateTokenUsage(tokens int) {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.state == nil {
        c.state = &ConnectionState{}
    }
    c.state.CurrentTokenUsage = tokens
}

// GetSystemPromptTokens returns system prompt token count
func (c *Connection) GetSystemPromptTokens() int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.state == nil {
        return 0
    }
    return c.state.SystemPromptTokens
}

// GetConversationTokens returns conversation token count
func (c *Connection) GetConversationTokens() int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.state == nil {
        return 0
    }
    return c.state.ConversationTokens
}

// GetToolTokens returns tool token count
func (c *Connection) GetToolTokens() int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.state == nil {
        return 0
    }
    return c.state.ToolTokens
}

// Session management methods

// SetActiveSession sets the active session for the connection
func (c *Connection) SetActiveSession(sessionID string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.state == nil {
        c.state = &ConnectionState{}
    }
    c.state.PreviousSessionID = c.state.ActiveSessionID
    c.state.ActiveSessionID = sessionID
}

// GetActiveSession returns the active session ID
func (c *Connection) GetActiveSession() string {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.state == nil {
        return ""
    }
    return c.state.ActiveSessionID
}

// GetPreviousSession returns the previous session ID
func (c *Connection) GetPreviousSession() string {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.state == nil {
        return ""
    }
    return c.state.PreviousSessionID
}