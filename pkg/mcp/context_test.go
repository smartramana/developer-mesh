package mcp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestContext tests the Context structure and behavior
func TestContext(t *testing.T) {
	// Create a test context
	now := time.Now()
	ctx := Context{
		ID:           "test-context-123",
		AgentID:      "test-agent",
		ModelID:      "test-model",
		SessionID:    "test-session",
		Content:      []ContextItem{},
		Metadata:     map[string]interface{}{"test": "value"},
		CreatedAt:    now,
		UpdatedAt:    now,
		ExpiresAt:    now.Add(24 * time.Hour),
		MaxTokens:    1000,
		CurrentTokens: 0,
	}

	// Test basic field access
	assert.Equal(t, "test-context-123", ctx.ID)
	assert.Equal(t, "test-agent", ctx.AgentID)
	assert.Equal(t, "test-model", ctx.ModelID)
	assert.Equal(t, "test-session", ctx.SessionID)
	assert.Equal(t, 0, len(ctx.Content))
	assert.Equal(t, "value", ctx.Metadata["test"])
	assert.Equal(t, now, ctx.CreatedAt)
	assert.Equal(t, now, ctx.UpdatedAt)
	assert.Equal(t, now.Add(24*time.Hour), ctx.ExpiresAt)
	assert.Equal(t, 1000, ctx.MaxTokens)
	assert.Equal(t, 0, ctx.CurrentTokens)

	// Add content items
	ctx.Content = append(ctx.Content, ContextItem{
		Role:      "system",
		Content:   "You are a helpful assistant.",
		Timestamp: now,
		Tokens:    8,
	})
	ctx.Content = append(ctx.Content, ContextItem{
		Role:      "user",
		Content:   "Hello, how are you?",
		Timestamp: now.Add(time.Minute),
		Tokens:    5,
	})
	ctx.CurrentTokens = 13

	// Test content manipulation
	assert.Equal(t, 2, len(ctx.Content))
	assert.Equal(t, "system", ctx.Content[0].Role)
	assert.Equal(t, "You are a helpful assistant.", ctx.Content[0].Content)
	assert.Equal(t, 8, ctx.Content[0].Tokens)
	assert.Equal(t, "user", ctx.Content[1].Role)
	assert.Equal(t, 13, ctx.CurrentTokens)
}

// TestContextItem tests the ContextItem structure and behavior
func TestContextItem(t *testing.T) {
	// Create a test context item
	now := time.Now()
	item := ContextItem{
		Role:      "user",
		Content:   "Test message",
		Timestamp: now,
		Tokens:    2,
		Metadata:  map[string]interface{}{"source": "test"},
	}

	// Test basic field access
	assert.Equal(t, "user", item.Role)
	assert.Equal(t, "Test message", item.Content)
	assert.Equal(t, now, item.Timestamp)
	assert.Equal(t, 2, item.Tokens)
	assert.Equal(t, "test", item.Metadata["source"])
}

// TestContextUpdateOptions tests the ContextUpdateOptions structure
func TestContextUpdateOptions(t *testing.T) {
	// Create test options
	options := ContextUpdateOptions{
		Truncate:         true,
		TruncateStrategy: "oldest_first",
		RelevanceParameters: map[string]interface{}{
			"min_similarity": 0.7,
		},
	}

	// Test basic field access
	assert.True(t, options.Truncate)
	assert.Equal(t, "oldest_first", options.TruncateStrategy)
	assert.Equal(t, 0.7, options.RelevanceParameters["min_similarity"])
}

// TestModelRequest tests the ModelRequest structure
func TestModelRequest(t *testing.T) {
	// Create a test model request
	request := ModelRequest{
		ModelID:    "test-model",
		ContextID:  "test-context",
		Parameters: map[string]interface{}{
			"temperature": 0.7,
			"max_tokens":  100,
		},
	}

	// Test basic field access
	assert.Equal(t, "test-model", request.ModelID)
	assert.Equal(t, "test-context", request.ContextID)
	assert.Equal(t, 0.7, request.Parameters["temperature"])
	
	// Handle max_tokens parameter which could be either int or float64
	maxTokens, ok := request.Parameters["max_tokens"]
	assert.True(t, ok)
	
	// Type assertion with type switch to handle both possible types
	var maxTokensValue int
	switch v := maxTokens.(type) {
	case int:
		maxTokensValue = v
	case float64:
		maxTokensValue = int(v)
	default:
		t.Fatalf("max_tokens has unexpected type: %T", maxTokens)
	}
	
	assert.Equal(t, 100, maxTokensValue)
}

// TestModelResponse tests the ModelResponse structure
func TestModelResponse(t *testing.T) {
	// Create a test model response
	response := ModelResponse{
		RequestID: "test-request",
		ModelID:   "test-model",
		Content:   "This is a test response",
		Tokens:    5,
		Metadata:  map[string]interface{}{
			"finish_reason": "stop",
		},
	}

	// Test basic field access
	assert.Equal(t, "test-request", response.RequestID)
	assert.Equal(t, "test-model", response.ModelID)
	assert.Equal(t, "This is a test response", response.Content)
	assert.Equal(t, 5, response.Tokens)
	assert.Equal(t, "stop", response.Metadata["finish_reason"])
}

// TestEventFilterAgentAndSessionID tests the agent and session ID filtering
func TestEventFilterAgentAndSessionID(t *testing.T) {
	baseTime := time.Now()
	
	tests := []struct {
		name   string
		filter EventFilter
		event  Event
		want   bool
	}{
		{
			name: "match agent ID",
			filter: EventFilter{
				AgentIDs: []string{"agent-123"},
			},
			event: Event{
				Source:    "github",
				Type:      "pull_request",
				Timestamp: baseTime,
				AgentID:   "agent-123",
				Data:      nil,
			},
			want: true,
		},
		{
			name: "match multiple agent IDs",
			filter: EventFilter{
				AgentIDs: []string{"agent-456", "agent-123"},
			},
			event: Event{
				Source:    "github",
				Type:      "pull_request",
				Timestamp: baseTime,
				AgentID:   "agent-123",
				Data:      nil,
			},
			want: true,
		},
		{
			name: "agent ID doesn't match",
			filter: EventFilter{
				AgentIDs: []string{"agent-456"},
			},
			event: Event{
				Source:    "github",
				Type:      "pull_request",
				Timestamp: baseTime,
				AgentID:   "agent-123",
				Data:      nil,
			},
			want: false,
		},
		{
			name: "match session ID",
			filter: EventFilter{
				SessionIDs: []string{"session-123"},
			},
			event: Event{
				Source:    "github",
				Type:      "pull_request",
				Timestamp: baseTime,
				AgentID:   "agent-123",
				SessionID: "session-123",
				Data:      nil,
			},
			want: true,
		},
		{
			name: "match multiple session IDs",
			filter: EventFilter{
				SessionIDs: []string{"session-456", "session-123"},
			},
			event: Event{
				Source:    "github",
				Type:      "pull_request",
				Timestamp: baseTime,
				AgentID:   "agent-123",
				SessionID: "session-123",
				Data:      nil,
			},
			want: true,
		},
		{
			name: "session ID doesn't match",
			filter: EventFilter{
				SessionIDs: []string{"session-456"},
			},
			event: Event{
				Source:    "github",
				Type:      "pull_request",
				Timestamp: baseTime,
				AgentID:   "agent-123",
				SessionID: "session-123",
				Data:      nil,
			},
			want: false,
		},
		{
			name: "match all criteria",
			filter: EventFilter{
				Sources:    []string{"github"},
				Types:      []string{"pull_request"},
				AgentIDs:   []string{"agent-123"},
				SessionIDs: []string{"session-123"},
				After:      baseTime.Add(-1 * time.Hour),
				Before:     baseTime.Add(1 * time.Hour),
			},
			event: Event{
				Source:    "github",
				Type:      "pull_request",
				Timestamp: baseTime,
				AgentID:   "agent-123",
				SessionID: "session-123",
				Data:      nil,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.MatchEvent(tt.event)
			assert.Equal(t, tt.want, got)
		})
	}
}
