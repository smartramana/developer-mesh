package websocket

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
	assert.Equal(t, msg.Type, decoded.Type)
	assert.Equal(t, msg.Method, decoded.Method)
}

func TestMessageTypeHelpers(t *testing.T) {
	tests := []struct {
		name   string
		msg    Message
		check  func(*Message) bool
		expect bool
	}{
		{
			name:   "IsRequest",
			msg:    Message{Type: MessageTypeRequest},
			check:  (*Message).IsRequest,
			expect: true,
		},
		{
			name:   "IsResponse",
			msg:    Message{Type: MessageTypeResponse},
			check:  (*Message).IsResponse,
			expect: true,
		},
		{
			name:   "IsNotification",
			msg:    Message{Type: MessageTypeNotification},
			check:  (*Message).IsNotification,
			expect: true,
		},
		{
			name:   "IsError",
			msg:    Message{Type: MessageTypeError},
			check:  (*Message).IsError,
			expect: true,
		},
		{
			name:   "IsRequest with wrong type",
			msg:    Message{Type: MessageTypeResponse},
			check:  (*Message).IsRequest,
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, tt.check(&tt.msg))
		})
	}
}

func TestError(t *testing.T) {
	err := NewError(ErrCodeInvalidMessage, "Invalid message format", map[string]string{
		"field": "id",
		"error": "missing",
	})

	assert.Equal(t, ErrCodeInvalidMessage, err.Code)
	assert.Equal(t, "Invalid message format", err.Message)
	assert.NotNil(t, err.Data)

	// Test JSON serialization
	data, jsonErr := json.Marshal(err)
	assert.NoError(t, jsonErr)

	var decoded Error
	jsonErr = json.Unmarshal(data, &decoded)
	assert.NoError(t, jsonErr)
	assert.Equal(t, err.Code, decoded.Code)
	assert.Equal(t, err.Message, decoded.Message)
}

func TestConnectionState(t *testing.T) {
	conn := &Connection{
		ID:        "test-conn",
		AgentID:   "agent-1",
		TenantID:  "tenant-1",
		CreatedAt: time.Now(),
	}

	// Test initial state
	conn.SetState(ConnectionStateConnecting)
	assert.Equal(t, ConnectionStateConnecting, conn.GetState())

	// Test state transitions
	conn.SetState(ConnectionStateConnected)
	assert.Equal(t, ConnectionStateConnected, conn.GetState())
	assert.True(t, conn.IsActive())

	conn.SetState(ConnectionStateClosing)
	assert.Equal(t, ConnectionStateClosing, conn.GetState())
	assert.False(t, conn.IsActive())

	conn.SetState(ConnectionStateClosed)
	assert.Equal(t, ConnectionStateClosed, conn.GetState())
	assert.False(t, conn.IsActive())
}

func TestConnectionIsActive(t *testing.T) {
	conn := &Connection{}

	tests := []struct {
		state    ConnectionState
		expected bool
	}{
		{ConnectionStateConnecting, true},
		{ConnectionStateConnected, true},
		{ConnectionStateClosing, false},
		{ConnectionStateClosed, false},
	}

	for _, tt := range tests {
		conn.SetState(tt.state)
		assert.Equal(t, tt.expected, conn.IsActive(), "State: %v", tt.state)
	}
}

func TestMessageWithError(t *testing.T) {
	msg := &Message{
		ID:   "test-456",
		Type: MessageTypeError,
		Error: &Error{
			Code:    ErrCodeMethodNotFound,
			Message: "Method not found",
			Data:    "unknown.method",
		},
	}

	data, err := json.Marshal(msg)
	assert.NoError(t, err)

	var decoded Message
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, msg.ID, decoded.ID)
	assert.Equal(t, MessageTypeError, decoded.Type)
	assert.NotNil(t, decoded.Error)
	assert.Equal(t, ErrCodeMethodNotFound, decoded.Error.Code)
}

func TestComplexMessageParams(t *testing.T) {
	type ToolParams struct {
		Tool string            `json:"tool"`
		Args map[string]string `json:"args"`
	}

	params := ToolParams{
		Tool: "github.create_issue",
		Args: map[string]string{
			"repo":  "test-repo",
			"title": "Test Issue",
			"body":  "This is a test",
		},
	}

	msg := &Message{
		ID:     "complex-123",
		Type:   MessageTypeRequest,
		Method: "tool.execute",
		Params: params,
	}

	// Test serialization
	data, err := json.Marshal(msg)
	assert.NoError(t, err)

	// Test deserialization
	var decoded Message
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, msg.ID, decoded.ID)

	// Convert params back to struct
	paramsJSON, err := json.Marshal(decoded.Params)
	assert.NoError(t, err)

	var decodedParams ToolParams
	err = json.Unmarshal(paramsJSON, &decodedParams)
	assert.NoError(t, err)
	assert.Equal(t, params.Tool, decodedParams.Tool)
	assert.Equal(t, params.Args["repo"], decodedParams.Args["repo"])
}
