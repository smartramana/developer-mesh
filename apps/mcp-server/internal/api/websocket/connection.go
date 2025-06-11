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
        if err := c.conn.Close(websocket.StatusNormalClosure, ""); err != nil {
            // Log error but don't fail - connection is already being closed
            if c.hub.logger != nil {
                c.hub.logger.Debug("Error closing WebSocket connection", map[string]interface{}{
                    "error": err.Error(),
                    "connection_id": c.ID,
                })
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
        if err := c.conn.Close(websocket.StatusNormalClosure, ""); err != nil {
            // Log error but don't fail - connection is already being closed
            if c.hub.logger != nil {
                c.hub.logger.Debug("Error closing WebSocket connection in writePump", map[string]interface{}{
                    "error": err.Error(),
                    "connection_id": c.ID,
                })
            }
        }
    }()
    
    ctx := context.Background()
    
    for {
        select {
        case message, ok := <-c.send:
            if !ok {
                // The hub closed the channel
                if err := c.conn.Close(websocket.StatusNormalClosure, ""); err != nil {
                    if c.hub.logger != nil {
                        c.hub.logger.Debug("Error closing connection when hub closed channel", map[string]interface{}{
                            "error": err.Error(),
                            "connection_id": c.ID,
                        })
                    }
                }
                return
            }
            
            // Set timeout for write operation
            writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
            defer cancel()
            
            if err := c.conn.Write(writeCtx, websocket.MessageText, message); err != nil {
                c.hub.logger.Error("Write error", map[string]interface{}{
                    "error":         err.Error(),
                    "connection_id": c.ID,
                })
                return
            }
            
            // Record sent message
            c.hub.metricsCollector.RecordMessage("sent", "response", c.TenantID, 0)
            
        case <-ticker.C:
            // Send ping to detect disconnected clients
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