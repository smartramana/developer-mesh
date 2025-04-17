package mcp

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEventFilter_MatchEvent(t *testing.T) {
	baseTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	
	tests := []struct {
		name   string
		filter EventFilter
		event  Event
		want   bool
	}{
		{
			name: "match all criteria",
			filter: EventFilter{
				Sources: []string{"github"},
				Types:   []string{"pull_request"},
				After:   baseTime.Add(-1 * time.Hour),
				Before:  baseTime.Add(1 * time.Hour),
			},
			event: Event{
				Source:    "github",
				Type:      "pull_request",
				Timestamp: baseTime,
				Data:      nil,
			},
			want: true,
		},
		{
			name: "match multiple sources",
			filter: EventFilter{
				Sources: []string{"harness", "github"},
				Types:   []string{"pull_request"},
			},
			event: Event{
				Source:    "github",
				Type:      "pull_request",
				Timestamp: baseTime,
				Data:      nil,
			},
			want: true,
		},
		{
			name: "match multiple types",
			filter: EventFilter{
				Sources: []string{"github"},
				Types:   []string{"push", "pull_request"},
			},
			event: Event{
				Source:    "github",
				Type:      "pull_request",
				Timestamp: baseTime,
				Data:      nil,
			},
			want: true,
		},
		{
			name: "empty filter matches all",
			filter: EventFilter{
				Sources: []string{},
				Types:   []string{},
			},
			event: Event{
				Source:    "github",
				Type:      "pull_request",
				Timestamp: baseTime,
				Data:      nil,
			},
			want: true,
		},
		{
			name: "source doesn't match",
			filter: EventFilter{
				Sources: []string{"harness"},
				Types:   []string{"pull_request"},
			},
			event: Event{
				Source:    "github",
				Type:      "pull_request",
				Timestamp: baseTime,
				Data:      nil,
			},
			want: false,
		},
		{
			name: "type doesn't match",
			filter: EventFilter{
				Sources: []string{"github"},
				Types:   []string{"push"},
			},
			event: Event{
				Source:    "github",
				Type:      "pull_request",
				Timestamp: baseTime,
				Data:      nil,
			},
			want: false,
		},
		{
			name: "before timestamp doesn't match",
			filter: EventFilter{
				Before: baseTime.Add(-1 * time.Hour),
			},
			event: Event{
				Source:    "github",
				Type:      "pull_request",
				Timestamp: baseTime,
				Data:      nil,
			},
			want: false,
		},
		{
			name: "after timestamp doesn't match",
			filter: EventFilter{
				After: baseTime.Add(1 * time.Hour),
			},
			event: Event{
				Source:    "github",
				Type:      "pull_request",
				Timestamp: baseTime,
				Data:      nil,
			},
			want: false,
		},
		{
			name: "timestamp within range",
			filter: EventFilter{
				After:  baseTime.Add(-2 * time.Hour),
				Before: baseTime.Add(2 * time.Hour),
			},
			event: Event{
				Source:    "github",
				Type:      "pull_request",
				Timestamp: baseTime,
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

func TestEvent_Basics(t *testing.T) {
	// Test event creation and field access
	event := Event{
		Source:    "github",
		Type:      "pull_request",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"key": "value"},
	}
	
	assert.Equal(t, "github", event.Source)
	assert.Equal(t, "pull_request", event.Type)
	assert.NotZero(t, event.Timestamp)
	
	data, ok := event.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "value", data["key"])
}

// TestEvent_JSON tests JSON serialization and deserialization of Event
func TestEvent_JSON(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond) // Truncate to avoid sub-millisecond time precision issues
	
	// Create a test event
	event := Event{
		Source:    "github",
		Type:      "pull_request",
		Timestamp: now,
		AgentID:   "agent-123",
		SessionID: "session-456",
		Data: map[string]interface{}{
			"repo":   "test-repo",
			"number": 42,
			"title":  "Test Pull Request",
			"nested": map[string]interface{}{
				"key": "value",
			},
		},
	}
	
	// Marshal to JSON
	jsonData, err := json.Marshal(event)
	assert.NoError(t, err)
	
	// Unmarshal back to an Event
	var decodedEvent Event
	err = json.Unmarshal(jsonData, &decodedEvent)
	assert.NoError(t, err)
	
	// Verify fields match
	assert.Equal(t, event.Source, decodedEvent.Source)
	assert.Equal(t, event.Type, decodedEvent.Type)
	assert.Equal(t, event.Timestamp.UnixNano(), decodedEvent.Timestamp.UnixNano())
	assert.Equal(t, event.AgentID, decodedEvent.AgentID)
	assert.Equal(t, event.SessionID, decodedEvent.SessionID)
	
	// Verify data field
	decodedData, ok := decodedEvent.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "test-repo", decodedData["repo"])
	assert.Equal(t, float64(42), decodedData["number"]) // JSON numbers are parsed as float64
	assert.Equal(t, "Test Pull Request", decodedData["title"])
	
	// Verify nested data
	nestedData, ok := decodedData["nested"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "value", nestedData["key"])
}

// TestEventFilter_Complex tests complex filtering scenarios
func TestEventFilter_Complex(t *testing.T) {
	baseTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	
	// Test with various combinations of filter criteria
	filter := EventFilter{
		Sources:    []string{"github", "anthropic"},
		Types:      []string{"pull_request", "model_request"},
		AgentIDs:   []string{"agent-123", "agent-456"},
		SessionIDs: []string{"session-789"},
		After:      baseTime.Add(-24 * time.Hour),
		Before:     baseTime.Add(24 * time.Hour),
	}
	
	// Test event that matches source but nothing else
	event1 := Event{
		Source:    "github",
		Type:      "issue",
		Timestamp: baseTime.Add(-48 * time.Hour),
		AgentID:   "agent-789",
		SessionID: "session-111",
	}
	assert.False(t, filter.MatchEvent(event1), "Event with only matching source should not match")
	
	// Test event that matches source and type but nothing else
	event2 := Event{
		Source:    "github",
		Type:      "pull_request",
		Timestamp: baseTime.Add(-48 * time.Hour),
		AgentID:   "agent-789",
		SessionID: "session-111",
	}
	assert.False(t, filter.MatchEvent(event2), "Event with matching source and type but outside time range should not match")
	
	// Test event that matches source, type, and time range but not IDs
	event3 := Event{
		Source:    "github",
		Type:      "pull_request",
		Timestamp: baseTime,
		AgentID:   "agent-789",
		SessionID: "session-111",
	}
	assert.False(t, filter.MatchEvent(event3), "Event with wrong agent and session IDs should not match")
	
	// Test event that matches source, type, time range, and agent ID but not session ID
	event4 := Event{
		Source:    "github",
		Type:      "pull_request",
		Timestamp: baseTime,
		AgentID:   "agent-123",
		SessionID: "session-111",
	}
	assert.False(t, filter.MatchEvent(event4), "Event with wrong session ID should not match")
	
	// Test event that matches all criteria
	event5 := Event{
		Source:    "github",
		Type:      "pull_request",
		Timestamp: baseTime,
		AgentID:   "agent-123",
		SessionID: "session-789",
	}
	assert.True(t, filter.MatchEvent(event5), "Event matching all criteria should match")
	
	// Test event with different matching criteria
	event6 := Event{
		Source:    "anthropic",
		Type:      "model_request",
		Timestamp: baseTime.Add(12 * time.Hour),
		AgentID:   "agent-456",
		SessionID: "session-789",
	}
	assert.True(t, filter.MatchEvent(event6), "Event with different matching criteria should match")
}

// TestEvent_WithComplexData tests events with complex nested data structures
func TestEvent_WithComplexData(t *testing.T) {
	now := time.Now()
	
	// Create an event with complex nested data
	event := Event{
		Source:    "anthropic",
		Type:      "model_response",
		Timestamp: now,
		AgentID:   "agent-123",
		SessionID: "session-456",
		Data: map[string]interface{}{
			"model_id": "claude-3-haiku",
			"prompt": "Tell me a story",
			"response": map[string]interface{}{
				"content": "Once upon a time...",
				"finish_reason": "stop",
				"usage": map[string]interface{}{
					"prompt_tokens": 4,
					"completion_tokens": 100,
					"total_tokens": 104,
				},
			},
			"metadata": map[string]interface{}{
				"user_id": "user-789",
				"request_id": "req-123",
				"timestamp": now.Unix(),
				"tags": []interface{}{
					"story",
					"creative",
					"short",
				},
			},
		},
	}
	
	// Test field access
	assert.Equal(t, "anthropic", event.Source)
	assert.Equal(t, "model_response", event.Type)
	assert.Equal(t, now, event.Timestamp)
	assert.Equal(t, "agent-123", event.AgentID)
	assert.Equal(t, "session-456", event.SessionID)
	
	// Test data field
	data, ok := event.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "claude-3-haiku", data["model_id"])
	assert.Equal(t, "Tell me a story", data["prompt"])
	
	// Test nested data
	response, ok := data["response"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "Once upon a time...", response["content"])
	assert.Equal(t, "stop", response["finish_reason"])
	
	usage, ok := response["usage"].(map[string]interface{})
	assert.True(t, ok)
	
	// Create helper function to handle numeric values which could be int or float64
	assertNumericEquals := func(expected int, actual interface{}, name string) {
		switch v := actual.(type) {
		case int:
			assert.Equal(t, expected, v, "Value of %s should match", name)
		case float64:
			assert.Equal(t, float64(expected), v, "Value of %s should match", name)
		default:
			t.Fatalf("%s has unexpected type: %T", name, actual)
		}
	}
	
	// Test numeric values that could be either int or float64 depending on JSON parsing
	assertNumericEquals(4, usage["prompt_tokens"], "prompt_tokens")
	assertNumericEquals(100, usage["completion_tokens"], "completion_tokens")
	assertNumericEquals(104, usage["total_tokens"], "total_tokens")
	
	metadata, ok := data["metadata"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "user-789", metadata["user_id"])
	assert.Equal(t, "req-123", metadata["request_id"])
	assert.Equal(t, now.Unix(), metadata["timestamp"])
	
	tags, ok := metadata["tags"].([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 3, len(tags))
	assert.Equal(t, "story", tags[0])
	assert.Equal(t, "creative", tags[1])
	assert.Equal(t, "short", tags[2])
	
	// Marshal to JSON and verify it serializes correctly
	jsonData, err := json.Marshal(event)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)
	
	// Unmarshal back to an Event
	var decodedEvent Event
	err = json.Unmarshal(jsonData, &decodedEvent)
	assert.NoError(t, err)
	
	// Verify the structure is preserved
	decodedData, ok := decodedEvent.Data.(map[string]interface{})
	assert.True(t, ok)
	decodedResponse, ok := decodedData["response"].(map[string]interface{})
	assert.True(t, ok)
	decodedUsage, ok := decodedResponse["usage"].(map[string]interface{})
	assert.True(t, ok)
	
	// Use the same helper function for the decoded event
	assertNumericEquals(104, decodedUsage["total_tokens"], "decoded total_tokens")
}

// TestEventFilter_EmptyFilter tests behavior with empty filter criteria
func TestEventFilter_EmptyFilter(t *testing.T) {
	now := time.Now()
	
	// Empty filter should match any event
	emptyFilter := EventFilter{}
	
	// Test with various events
	events := []Event{
		{
			Source:    "github",
			Type:      "pull_request",
			Timestamp: now,
			AgentID:   "agent-123",
			SessionID: "session-456",
		},
		{
			Source:    "anthropic",
			Type:      "model_request",
			Timestamp: now.Add(-1 * time.Hour),
			AgentID:   "agent-789",
			SessionID: "session-101",
		},
		{
			Source:    "custom",
			Type:      "user_interaction",
			Timestamp: now.Add(1 * time.Hour),
			AgentID:   "agent-202",
			SessionID: "session-303",
		},
	}
	
	for i, event := range events {
		assert.True(t, emptyFilter.MatchEvent(event), "Empty filter should match event %d", i)
	}
}
