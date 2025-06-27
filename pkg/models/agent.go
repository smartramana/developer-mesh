package models

import (
	"time"

	"github.com/google/uuid"
)

// Agent represents an AI agent in the system
type Agent struct {
	ID           string                 `json:"id" db:"id"`
	TenantID     uuid.UUID              `json:"tenant_id" db:"tenant_id"`
	Name         string                 `json:"name" db:"name"`
	ModelID      string                 `json:"model_id" db:"model_id"`
	Type         string                 `json:"type" db:"type"`
	Status       string                 `json:"status" db:"status"` // available, busy, offline
	Capabilities []string               `json:"capabilities" db:"capabilities"`
	Metadata     map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt    time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at" db:"updated_at"`
	LastSeenAt   *time.Time             `json:"last_seen_at" db:"last_seen_at"`
}
