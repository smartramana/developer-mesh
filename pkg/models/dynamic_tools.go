package models

import (
	"encoding/json"
	"time"
)

// DynamicTool represents a dynamically configured tool
type DynamicTool struct {
	ID                string                 `json:"id" db:"id"`
	TenantID          string                 `json:"tenant_id" db:"tenant_id"`
	ToolName          string                 `json:"tool_name" db:"tool_name"`
	DisplayName       string                 `json:"display_name" db:"display_name"`
	Config            map[string]interface{} `json:"config" db:"config"`
	AuthType          string                 `json:"auth_type" db:"auth_type"`
	Status            string                 `json:"status" db:"status"`
	HealthStatus      json.RawMessage        `json:"health_status" db:"health_status"`
	LastHealthCheck   *time.Time             `json:"last_health_check,omitempty" db:"last_health_check"`
	CreatedAt         time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at" db:"updated_at"`
	Provider          string                 `json:"provider,omitempty" db:"provider"`
	RetryPolicy       *ToolRetryPolicy       `json:"retry_policy,omitempty" db:"retry_policy"`
	PassthroughConfig *PassthroughConfig     `json:"passthrough_config,omitempty" db:"passthrough_config"`
}

// DiscoverySession represents an active discovery session
type DiscoverySession struct {
	ID                string                 `json:"id" db:"id"`
	SessionID         string                 `json:"session_id" db:"session_id"`
	TenantID          string                 `json:"tenant_id" db:"tenant_id"`
	BaseURL           string                 `json:"base_url" db:"base_url"`
	Status            string                 `json:"status" db:"status"`
	DiscoveryResult   *DiscoveryResult       `json:"discovery_result,omitempty" db:"discovery_result"`
	DiscoveryMetadata map[string]interface{} `json:"discovery_metadata" db:"discovery_metadata"`
	CreatedAt         time.Time              `json:"created_at" db:"created_at"`
	ExpiresAt         time.Time              `json:"expires_at" db:"expires_at"`
}

// ToolHealthStatus represents the health status of a tool
type ToolHealthStatus struct {
	IsHealthy    bool                   `json:"is_healthy"`
	LastChecked  time.Time              `json:"last_checked"`
	ResponseTime int                    `json:"response_time_ms"`
	Error        string                 `json:"error,omitempty"`
	Version      string                 `json:"version,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

// ToolAction represents an available action on a tool
type ToolAction struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Method      string                 `json:"method"`
	Path        string                 `json:"path"`
	Parameters  []ActionParameter      `json:"parameters,omitempty"`
	Responses   map[string]interface{} `json:"responses,omitempty"`
}

// ActionParameter represents a parameter for a tool action
type ActionParameter struct {
	Name        string      `json:"name"`
	In          string      `json:"in"` // query, path, header, body
	Required    bool        `json:"required"`
	Description string      `json:"description,omitempty"`
	Type        string      `json:"type"`
	Default     interface{} `json:"default,omitempty"`
}

// ToolExecutionRequest represents a request to execute a tool action
type ToolExecutionRequest struct {
	Action     string                 `json:"action"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Headers    map[string]string      `json:"headers,omitempty"`
	Timeout    int                    `json:"timeout,omitempty"` // in seconds
}

// ToolExecutionResponse represents the response from executing a tool action
type ToolExecutionResponse struct {
	Success    bool                `json:"success"`
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers,omitempty"`
	Body       interface{}         `json:"body,omitempty"`
	Error      string              `json:"error,omitempty"`
	Duration   int64               `json:"duration_ms"`
	ExecutedAt time.Time           `json:"executed_at"`
}

// DiscoveryHint provides user-supplied hints for API discovery
type DiscoveryHint struct {
	OpenAPIURL       string            `json:"openapi_url,omitempty"`
	AuthHeaders      map[string]string `json:"auth_headers,omitempty"`
	CustomPaths      []string          `json:"custom_paths,omitempty"`
	APIFormat        string            `json:"api_format,omitempty"`
	ExampleEndpoint  string            `json:"example_endpoint,omitempty"`
	DocumentationURL string            `json:"documentation_url,omitempty"`
}

// ToolRetryPolicy defines retry configuration for a tool
type ToolRetryPolicy struct {
	MaxRetries       int      `json:"max_retries"`
	InitialDelay     int      `json:"initial_delay_ms"`
	MaxDelay         int      `json:"max_delay_ms"`
	Multiplier       float64  `json:"multiplier"`
	RetryableErrors  []string `json:"retryable_errors"`
	RetryOnTimeout   bool     `json:"retry_on_timeout"`
	RetryOnRateLimit bool     `json:"retry_on_rate_limit"`
}

// PassthroughConfig defines how user token passthrough should be handled
type PassthroughConfig struct {
	Mode              string `json:"mode"`                // optional, required, disabled
	FallbackToService bool   `json:"fallback_to_service"` // Allow fallback to service account
}

// DiscoveryResult contains the results of API discovery
type DiscoveryResult struct {
	Status           string                 `json:"status"`
	OpenAPISpec      interface{}            `json:"openapi_spec,omitempty"`
	SpecURL          string                 `json:"spec_url,omitempty"`
	DiscoveredURLs   []string               `json:"discovered_urls"`
	RequiresManual   bool                   `json:"requires_manual"`
	SuggestedActions []string               `json:"suggested_actions,omitempty"`
	Capabilities     []string               `json:"capabilities,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}
