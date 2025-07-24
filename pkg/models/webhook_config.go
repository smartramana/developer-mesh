package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// WebhookConfig represents a webhook configuration for an organization
type WebhookConfig struct {
	ID               uuid.UUID      `json:"id" db:"id"`
	OrganizationName string         `json:"organization_name" db:"organization_name"`
	WebhookSecret    string         `json:"-" db:"webhook_secret"` // Never expose in JSON
	Enabled          bool           `json:"enabled" db:"enabled"`
	AllowedEvents    pq.StringArray `json:"allowed_events" db:"allowed_events"`
	Metadata         JSONMap        `json:"metadata" db:"metadata"`
	CreatedAt        time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at" db:"updated_at"`
}

// WebhookConfigCreate represents the data needed to create a new webhook configuration
type WebhookConfigCreate struct {
	OrganizationName string         `json:"organization_name" validate:"required,min=1,max=255"`
	WebhookSecret    string         `json:"webhook_secret" validate:"required,min=32"` // Require strong secrets
	AllowedEvents    []string       `json:"allowed_events,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

// WebhookConfigUpdate represents the data needed to update a webhook configuration
type WebhookConfigUpdate struct {
	Enabled       *bool          `json:"enabled,omitempty"`
	WebhookSecret *string        `json:"webhook_secret,omitempty" validate:"omitempty,min=32"`
	AllowedEvents []string       `json:"allowed_events,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// WebhookConfigList represents a list of webhook configurations
type WebhookConfigList struct {
	Items      []*WebhookConfig `json:"items"`
	TotalCount int              `json:"total_count"`
}

// Validate checks if the webhook configuration is valid
func (w *WebhookConfig) Validate() error {
	if w.OrganizationName == "" {
		return fmt.Errorf("organization_name is required")
	}
	if w.WebhookSecret == "" {
		return fmt.Errorf("webhook_secret is required")
	}
	if len(w.AllowedEvents) == 0 {
		return fmt.Errorf("at least one allowed event is required")
	}
	return nil
}

// IsEventAllowed checks if a given event type is allowed for this organization
func (w *WebhookConfig) IsEventAllowed(eventType string) bool {
	for _, allowed := range w.AllowedEvents {
		if allowed == eventType {
			return true
		}
	}
	return false
}
