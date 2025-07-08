package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/lib/pq"
)

// TenantConfig represents per-tenant configuration settings
type TenantConfig struct {
	ID              string                 `json:"id" db:"id"`
	TenantID        string                 `json:"tenant_id" db:"tenant_id"`
	RateLimitConfig RateLimitConfig        `json:"rate_limit_config" db:"rate_limit_config"`
	ServiceTokens   map[string]string      `json:"-" db:"-"`              // Decrypted in memory only
	EncryptedTokens json.RawMessage        `json:"-" db:"service_tokens"` // Encrypted in database
	AllowedOrigins  pq.StringArray         `json:"allowed_origins" db:"allowed_origins"`
	Features        map[string]interface{} `json:"features" db:"-"` // Parsed from JSONB
	FeaturesJSON    json.RawMessage        `json:"-" db:"features"` // Raw JSONB from database
	CreatedAt       time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at" db:"updated_at"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	// Default rate limits
	DefaultRequestsPerMinute int `json:"default_requests_per_minute"`
	DefaultRequestsPerHour   int `json:"default_requests_per_hour"`
	DefaultRequestsPerDay    int `json:"default_requests_per_day"`

	// Key type specific overrides
	KeyTypeOverrides map[string]KeyTypeRateLimit `json:"key_type_overrides,omitempty"`

	// Endpoint specific overrides
	EndpointOverrides map[string]EndpointRateLimit `json:"endpoint_overrides,omitempty"`
}

// KeyTypeRateLimit represents rate limits for a specific key type
type KeyTypeRateLimit struct {
	RequestsPerMinute int `json:"requests_per_minute"`
	RequestsPerHour   int `json:"requests_per_hour"`
	RequestsPerDay    int `json:"requests_per_day"`
}

// EndpointRateLimit represents rate limits for a specific endpoint
type EndpointRateLimit struct {
	RequestsPerMinute int `json:"requests_per_minute"`
	BurstSize         int `json:"burst_size"`
}

// Scan implements sql.Scanner for RateLimitConfig
func (r *RateLimitConfig) Scan(value interface{}) error {
	if value == nil {
		*r = RateLimitConfig{}
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, r)
	case string:
		return json.Unmarshal([]byte(v), r)
	default:
		return json.Unmarshal([]byte("{}"), r)
	}
}

// Value implements driver.Valuer for RateLimitConfig
func (r RateLimitConfig) Value() (driver.Value, error) {
	return json.Marshal(r)
}

// DefaultTenantConfig returns a default configuration for a tenant
func DefaultTenantConfig(tenantID string) *TenantConfig {
	return &TenantConfig{
		TenantID: tenantID,
		RateLimitConfig: RateLimitConfig{
			DefaultRequestsPerMinute: 60,
			DefaultRequestsPerHour:   1000,
			DefaultRequestsPerDay:    10000,
			KeyTypeOverrides:         make(map[string]KeyTypeRateLimit),
			EndpointOverrides:        make(map[string]EndpointRateLimit),
		},
		ServiceTokens:  make(map[string]string),
		AllowedOrigins: pq.StringArray{},
		Features:       make(map[string]interface{}),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

// IsFeatureEnabled checks if a specific feature is enabled for the tenant
func (tc *TenantConfig) IsFeatureEnabled(feature string) bool {
	if tc.Features == nil {
		return false
	}

	val, exists := tc.Features[feature]
	if !exists {
		return false
	}

	// Handle different types of feature values
	switch v := val.(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "enabled" || v == "on"
	default:
		return false
	}
}

// GetRateLimitForKeyType returns the rate limit configuration for a specific key type
func (tc *TenantConfig) GetRateLimitForKeyType(keyType string) KeyTypeRateLimit {
	if override, exists := tc.RateLimitConfig.KeyTypeOverrides[keyType]; exists {
		return override
	}

	// Return defaults
	return KeyTypeRateLimit{
		RequestsPerMinute: tc.RateLimitConfig.DefaultRequestsPerMinute,
		RequestsPerHour:   tc.RateLimitConfig.DefaultRequestsPerHour,
		RequestsPerDay:    tc.RateLimitConfig.DefaultRequestsPerDay,
	}
}

// GetRateLimitForEndpoint returns the rate limit configuration for a specific endpoint
func (tc *TenantConfig) GetRateLimitForEndpoint(endpoint string) (EndpointRateLimit, bool) {
	limit, exists := tc.RateLimitConfig.EndpointOverrides[endpoint]
	return limit, exists
}

// HasServiceToken checks if a service token exists for a provider
func (tc *TenantConfig) HasServiceToken(provider string) bool {
	_, exists := tc.ServiceTokens[provider]
	return exists
}

// GetServiceToken returns the decrypted service token for a provider
func (tc *TenantConfig) GetServiceToken(provider string) (string, bool) {
	token, exists := tc.ServiceTokens[provider]
	return token, exists
}
