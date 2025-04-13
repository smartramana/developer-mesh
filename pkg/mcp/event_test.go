package mcp

import (
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
