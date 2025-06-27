package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// WorkspaceStatus represents the status of a workspace
type WorkspaceStatus string

const (
	WorkspaceStatusActive   WorkspaceStatus = "active"
	WorkspaceStatusInactive WorkspaceStatus = "inactive"
	WorkspaceStatusArchived WorkspaceStatus = "archived"
	WorkspaceStatusDeleted  WorkspaceStatus = "deleted"
)

// IsValid checks if the workspace status is valid
func (s WorkspaceStatus) IsValid() bool {
	switch s {
	case WorkspaceStatusActive, WorkspaceStatusInactive, WorkspaceStatusArchived, WorkspaceStatusDeleted:
		return true
	default:
		return false
	}
}

// WorkspaceSettings represents the settings for a workspace
type WorkspaceSettings struct {
	Notifications   NotificationSettings   `json:"notifications"`
	Collaboration   CollaborationSettings  `json:"collaboration"`
	Security        SecuritySettings       `json:"security"`
	AutomationRules []AutomationRule       `json:"automation_rules,omitempty"`
	Preferences     map[string]interface{} `json:"preferences,omitempty"`
}

// Value implements the driver.Valuer interface for database storage
func (ws WorkspaceSettings) Value() (driver.Value, error) {
	return json.Marshal(ws)
}

// Scan implements the sql.Scanner interface for database retrieval
func (ws *WorkspaceSettings) Scan(value interface{}) error {
	if value == nil {
		*ws = WorkspaceSettings{}
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, ws)
	case string:
		return json.Unmarshal([]byte(v), ws)
	default:
		return fmt.Errorf("cannot scan type %T into WorkspaceSettings", value)
	}
}

// NotificationSettings defines notification preferences
type NotificationSettings struct {
	Enabled         bool                   `json:"enabled"`
	EmailEnabled    bool                   `json:"email_enabled"`
	WebhookEnabled  bool                   `json:"webhook_enabled"`
	WebhookURL      string                 `json:"webhook_url,omitempty"`
	EventTypes      []string               `json:"event_types,omitempty"`
	DigestFrequency string                 `json:"digest_frequency,omitempty"` // immediate, hourly, daily, weekly
	Preferences     map[string]interface{} `json:"preferences,omitempty"`
}

// CollaborationSettings defines collaboration preferences
type CollaborationSettings struct {
	AllowGuestAccess   bool     `json:"allow_guest_access"`
	RequireApproval    bool     `json:"require_approval"`
	DefaultMemberRole  string   `json:"default_member_role"`
	AllowedDomains     []string `json:"allowed_domains,omitempty"`
	MaxMembers         int      `json:"max_members,omitempty"`
	EnablePresence     bool     `json:"enable_presence"`
	EnableTypingStatus bool     `json:"enable_typing_status"`
	ConflictResolution string   `json:"conflict_resolution"` // manual, auto_merge, last_write_wins
}

// SecuritySettings defines security preferences
type SecuritySettings struct {
	RequireMFA     bool     `json:"require_mfa"`
	AllowAPIAccess bool     `json:"allow_api_access"`
	IPWhitelist    []string `json:"ip_whitelist,omitempty"`
	SessionTimeout int      `json:"session_timeout"` // minutes
	DataEncryption bool     `json:"data_encryption"`
	AuditLogging   bool     `json:"audit_logging"`
	ComplianceMode string   `json:"compliance_mode,omitempty"` // HIPAA, SOC2, etc.
	RetentionDays  int      `json:"retention_days,omitempty"`
}

// AutomationRule defines an automation rule for the workspace
type AutomationRule struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Enabled    bool                   `json:"enabled"`
	Trigger    string                 `json:"trigger"` // event type
	Conditions []RuleCondition        `json:"conditions,omitempty"`
	Actions    []RuleAction           `json:"actions"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// RuleCondition defines a condition for an automation rule
type RuleCondition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"` // eq, ne, gt, lt, contains, etc.
	Value    interface{} `json:"value"`
}

// RuleAction defines an action for an automation rule
type RuleAction struct {
	Type       string                 `json:"type"` // webhook, notification, task, etc.
	Parameters map[string]interface{} `json:"parameters"`
}

// WorkspaceLimits defines resource limits for a workspace
type WorkspaceLimits struct {
	MaxMembers       int   `json:"max_members"`
	MaxDocuments     int   `json:"max_documents"`
	MaxStorageBytes  int64 `json:"max_storage_bytes"`
	MaxOperationsDay int   `json:"max_operations_day"`
	MaxAgents        int   `json:"max_agents"`
	MaxConcurrent    int   `json:"max_concurrent"`
}

// Value implements the driver.Valuer interface for database storage
func (wl WorkspaceLimits) Value() (driver.Value, error) {
	return json.Marshal(wl)
}

// Scan implements the sql.Scanner interface for database retrieval
func (wl *WorkspaceLimits) Scan(value interface{}) error {
	if value == nil {
		*wl = WorkspaceLimits{}
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, wl)
	case string:
		return json.Unmarshal([]byte(v), wl)
	default:
		return fmt.Errorf("cannot scan type %T into WorkspaceLimits", value)
	}
}

// IsWithinLimits checks if the given values are within the workspace limits
func (wl WorkspaceLimits) IsWithinLimits(members, documents int, storageBytes int64) bool {
	if wl.MaxMembers > 0 && members > wl.MaxMembers {
		return false
	}
	if wl.MaxDocuments > 0 && documents > wl.MaxDocuments {
		return false
	}
	if wl.MaxStorageBytes > 0 && storageBytes > wl.MaxStorageBytes {
		return false
	}
	return true
}

// GetDefaultWorkspaceSettings returns default workspace settings
func GetDefaultWorkspaceSettings() WorkspaceSettings {
	return WorkspaceSettings{
		Notifications: NotificationSettings{
			Enabled:         true,
			EmailEnabled:    false,
			WebhookEnabled:  false,
			DigestFrequency: "immediate",
			EventTypes:      []string{"member_joined", "document_created", "document_updated"},
		},
		Collaboration: CollaborationSettings{
			AllowGuestAccess:   false,
			RequireApproval:    true,
			DefaultMemberRole:  "member",
			EnablePresence:     true,
			EnableTypingStatus: true,
			ConflictResolution: "manual",
		},
		Security: SecuritySettings{
			RequireMFA:     false,
			AllowAPIAccess: true,
			SessionTimeout: 60,
			DataEncryption: true,
			AuditLogging:   true,
			RetentionDays:  90,
		},
		AutomationRules: []AutomationRule{},
		Preferences:     make(map[string]interface{}),
	}
}

// GetDefaultWorkspaceLimits returns default workspace limits
func GetDefaultWorkspaceLimits() WorkspaceLimits {
	return WorkspaceLimits{
		MaxMembers:       100,
		MaxDocuments:     1000,
		MaxStorageBytes:  10 * 1024 * 1024 * 1024, // 10 GB
		MaxOperationsDay: 10000,
		MaxAgents:        50,
		MaxConcurrent:    20,
	}
}
