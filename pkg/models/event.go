package models

import (
	"time"
)

// Event represents an MCP event
type Event struct {
	// Source is the source of the event (e.g., openai, anthropic, langchain)
	Source string `json:"source"`

	// Type is the type of the event (e.g., context_update, model_request)
	Type string `json:"type"`

	// Timestamp is when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// Data contains the event data
	Data any `json:"data"`

	// AgentID is the identifier for the AI agent that generated this event
	AgentID string `json:"agent_id"`

	// SessionID is the identifier for the user session
	SessionID string `json:"session_id,omitempty"`
}

// ContextItem represents a single item in a context
type ContextItem struct {
	// ID is the unique identifier for this context item
	ID string `json:"id,omitempty"`

	// Role is the role of this context item (e.g., user, assistant, system)
	Role string `json:"role"`

	// Content is the text content of this context item
	Content string `json:"content"`

	// Timestamp is when this context item was created
	Timestamp time.Time `json:"timestamp"`

	// Tokens is the token count for this context item
	Tokens int `json:"tokens,omitempty"`

	// Metadata contains additional information about this context item
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Context represents an AI model context
type Context struct {
	// ID is the unique identifier for this context
	ID string `json:"id"`

	// Name is the display name of this context
	Name string `json:"name"`

	// Description is a human-readable description of the context
	Description string `json:"description,omitempty"`

	// AgentID is the identifier for the AI agent that owns this context
	AgentID string `json:"agent_id"`

	// ModelID identifies which AI model this context is for
	ModelID string `json:"model_id"`

	// SessionID is the identifier for the user session
	SessionID string `json:"session_id,omitempty"`

	// Content contains the actual context data
	Content []ContextItem `json:"content"`

	// Metadata contains additional information about the context
	Metadata map[string]any `json:"metadata,omitempty"`

	// CreatedAt is when this context was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when this context was last updated
	UpdatedAt time.Time `json:"updated_at"`

	// ExpiresAt is when this context expires (if applicable)
	ExpiresAt time.Time `json:"expires_at,omitempty"`

	// MaxTokens is the maximum number of tokens this context can contain
	MaxTokens int `json:"max_tokens,omitempty"`

	// CurrentTokens is the current token count for this context
	CurrentTokens int `json:"current_tokens,omitempty"`
}

// ContextUpdateOptions provides options for updating a context
type ContextUpdateOptions struct {
	// ReplaceContent indicates whether to replace the entire content or append to it
	ReplaceContent bool `json:"replace_content,omitempty"`

	// Truncate indicates whether to truncate the context if it exceeds the maximum tokens
	Truncate bool `json:"truncate,omitempty"`

	// TruncateStrategy defines the strategy for truncating the context
	TruncateStrategy string `json:"truncate_strategy,omitempty"`
}

// EventHandler is a function that processes an event
type EventHandler func(Event) error

// EventFilter defines criteria for filtering events
type EventFilter struct {
	Sources    []string  `json:"sources"`
	Types      []string  `json:"types"`
	AgentIDs   []string  `json:"agent_ids,omitempty"`
	SessionIDs []string  `json:"session_ids,omitempty"`
	After      time.Time `json:"after"`
	Before     time.Time `json:"before"`
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

	// Check agent IDs
	if len(f.AgentIDs) > 0 {
		matched := false
		for _, agentID := range f.AgentIDs {
			if event.AgentID == agentID {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check session IDs
	if len(f.SessionIDs) > 0 {
		matched := false
		for _, sessionID := range f.SessionIDs {
			if event.SessionID == sessionID {
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
