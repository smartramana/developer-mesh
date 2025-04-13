package mcp

import (
	"time"
)

// Event represents an MCP event
type Event struct {
	// Source is the source of the event (e.g., github, harness, sonarqube)
	Source string `json:"source"`

	// Type is the type of the event (e.g., pull_request, deployment)
	Type string `json:"type"`

	// Timestamp is when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// Data contains the event data
	Data interface{} `json:"data"`
}

// EventHandler is a function that processes an event
type EventHandler func(Event) error

// EventFilter defines criteria for filtering events
type EventFilter struct {
	Sources []string  `json:"sources"`
	Types   []string  `json:"types"`
	After   time.Time `json:"after"`
	Before  time.Time `json:"before"`
}

// MatchEvent checks if an event matches the filter criteria
func (f *EventFilter) MatchEvent(event Event) bool {
	// Check sources
	if len(f.Sources) > 0 {
		matched := false
		for _, source := range f.Sources {
			if event.Source == source {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check types
	if len(f.Types) > 0 {
		matched := false
		for _, eventType := range f.Types {
			if event.Type == eventType {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check timestamp
	if !f.After.IsZero() && event.Timestamp.Before(f.After) {
		return false
	}
	if !f.Before.IsZero() && event.Timestamp.After(f.Before) {
		return false
	}

	return true
}
