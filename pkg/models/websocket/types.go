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
    MessageTypeBatch
    MessageTypeProgress
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

// Error represents a WebSocket error
type Error struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
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

// ConnectionState represents the state of a WebSocket connection
type ConnectionState int

const (
    ConnectionStateConnecting ConnectionState = iota
    ConnectionStateConnected
    ConnectionStateClosing
    ConnectionStateClosed
)

// Standard error codes
const (
    ErrCodeInvalidMessage      = 4000
    ErrCodeAuthFailed          = 4001
    ErrCodeRateLimited         = 4002
    ErrCodeServerError         = 4003
    ErrCodeMethodNotFound      = 4004
    ErrCodeInvalidParams       = 4005
    ErrCodeOperationCancelled  = 4006
    ErrCodeContextTooLarge     = 4007
    ErrCodeConflict            = 4008
)

// NewError creates a new WebSocket error
func NewError(code int, message string, data interface{}) *Error {
    return &Error{
        Code:    code,
        Message: message,
        Data:    data,
    }
}

// Error implements the error interface
func (e *Error) Error() string {
    return e.Message
}

// IsRequestMessage checks if the message is a request
func (m *Message) IsRequest() bool {
    return m.Type == MessageTypeRequest
}

// IsResponse checks if the message is a response
func (m *Message) IsResponse() bool {
    return m.Type == MessageTypeResponse
}

// IsNotification checks if the message is a notification
func (m *Message) IsNotification() bool {
    return m.Type == MessageTypeNotification
}

// IsError checks if the message is an error
func (m *Message) IsError() bool {
    return m.Type == MessageTypeError
}

// GetState returns the current connection state
func (c *Connection) GetState() ConnectionState {
    if state := c.State.Load(); state != nil {
        return state.(ConnectionState)
    }
    return ConnectionStateClosed
}

// SetState sets the connection state
func (c *Connection) SetState(state ConnectionState) {
    c.State.Store(state)
}

// IsActive checks if the connection is active
func (c *Connection) IsActive() bool {
    state := c.GetState()
    return state == ConnectionStateConnected || state == ConnectionStateConnecting
}