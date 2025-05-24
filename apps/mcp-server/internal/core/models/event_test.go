package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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

// TestEventJSON tests the JSON serialization and deserialization of Event
func TestEventJSON(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	event := Event{
		Source:    "test",
		Type:      "unit_test",
		Timestamp: now,
		AgentID:   "test-agent",
		SessionID: "test-session",
		Data: map[string]interface{}{
			"key": "value",
			"number": 42,
		},
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(event)
	assert.NoError(t, err)

	// Deserialize from JSON
	var parsedEvent Event
	err = json.Unmarshal(jsonData, &parsedEvent)
	assert.NoError(t, err)

	// Verify fields match
	assert.Equal(t, event.Source, parsedEvent.Source)
	assert.Equal(t, event.Type, parsedEvent.Type)
	assert.Equal(t, event.Timestamp.UTC(), parsedEvent.Timestamp.UTC())
	assert.Equal(t, event.AgentID, parsedEvent.AgentID)
	assert.Equal(t, event.SessionID, parsedEvent.SessionID)
	
	// Check Data field
	parsedData := parsedEvent.Data.(map[string]interface{})
	assert.Equal(t, "value", parsedData["key"])
	assert.Equal(t, float64(42), parsedData["number"]) // JSON numbers are float64
}
