package models

import (
	"encoding/json"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimitConfig_Scan(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    RateLimitConfig
		wantErr bool
	}{
		{
			name:  "scan nil value",
			value: nil,
			want:  RateLimitConfig{},
		},
		{
			name: "scan byte array",
			value: []byte(`{
				"default_requests_per_minute": 100,
				"default_requests_per_hour": 5000,
				"default_requests_per_day": 50000,
				"key_type_overrides": {
					"admin": {
						"requests_per_minute": 1000,
						"requests_per_hour": 50000,
						"requests_per_day": 500000
					}
				},
				"endpoint_overrides": {
					"/api/v1/expensive": {
						"requests_per_minute": 10,
						"burst_size": 20
					}
				}
			}`),
			want: RateLimitConfig{
				DefaultRequestsPerMinute: 100,
				DefaultRequestsPerHour:   5000,
				DefaultRequestsPerDay:    50000,
				KeyTypeOverrides: map[string]KeyTypeRateLimit{
					"admin": {
						RequestsPerMinute: 1000,
						RequestsPerHour:   50000,
						RequestsPerDay:    500000,
					},
				},
				EndpointOverrides: map[string]EndpointRateLimit{
					"/api/v1/expensive": {
						RequestsPerMinute: 10,
						BurstSize:         20,
					},
				},
			},
		},
		{
			name:  "scan string",
			value: `{"default_requests_per_minute": 60}`,
			want: RateLimitConfig{
				DefaultRequestsPerMinute: 60,
			},
		},
		{
			name:  "scan unsupported type",
			value: 123,
			want:  RateLimitConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r RateLimitConfig
			err := r.Scan(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, r)
			}
		})
	}
}

func TestRateLimitConfig_Value(t *testing.T) {
	r := RateLimitConfig{
		DefaultRequestsPerMinute: 100,
		DefaultRequestsPerHour:   5000,
		DefaultRequestsPerDay:    50000,
		KeyTypeOverrides: map[string]KeyTypeRateLimit{
			"admin": {
				RequestsPerMinute: 1000,
				RequestsPerHour:   50000,
				RequestsPerDay:    500000,
			},
		},
		EndpointOverrides: map[string]EndpointRateLimit{
			"/api/v1/expensive": {
				RequestsPerMinute: 10,
				BurstSize:         20,
			},
		},
	}

	value, err := r.Value()
	require.NoError(t, err)

	// Unmarshal back to verify
	var result map[string]interface{}
	err = json.Unmarshal(value.([]byte), &result)
	require.NoError(t, err)

	assert.Equal(t, float64(100), result["default_requests_per_minute"])
	assert.Equal(t, float64(5000), result["default_requests_per_hour"])
	assert.Equal(t, float64(50000), result["default_requests_per_day"])

	keyTypeOverrides := result["key_type_overrides"].(map[string]interface{})
	adminOverride := keyTypeOverrides["admin"].(map[string]interface{})
	assert.Equal(t, float64(1000), adminOverride["requests_per_minute"])
}

func TestDefaultTenantConfig(t *testing.T) {
	tenantID := "test-tenant-123"
	config := DefaultTenantConfig(tenantID)

	assert.Equal(t, tenantID, config.TenantID)
	assert.Equal(t, 60, config.RateLimitConfig.DefaultRequestsPerMinute)
	assert.Equal(t, 1000, config.RateLimitConfig.DefaultRequestsPerHour)
	assert.Equal(t, 10000, config.RateLimitConfig.DefaultRequestsPerDay)
	assert.NotNil(t, config.RateLimitConfig.KeyTypeOverrides)
	assert.NotNil(t, config.RateLimitConfig.EndpointOverrides)
	assert.NotNil(t, config.ServiceTokens)
	assert.NotNil(t, config.AllowedOrigins)
	assert.NotNil(t, config.Features)
	assert.False(t, config.CreatedAt.IsZero())
	assert.False(t, config.UpdatedAt.IsZero())
}

func TestTenantConfig_IsFeatureEnabled(t *testing.T) {
	tests := []struct {
		name     string
		features map[string]interface{}
		feature  string
		want     bool
	}{
		{
			name:     "nil features map",
			features: nil,
			feature:  "test_feature",
			want:     false,
		},
		{
			name:     "feature not exists",
			features: map[string]interface{}{"other_feature": true},
			feature:  "test_feature",
			want:     false,
		},
		{
			name:     "feature is true boolean",
			features: map[string]interface{}{"test_feature": true},
			feature:  "test_feature",
			want:     true,
		},
		{
			name:     "feature is false boolean",
			features: map[string]interface{}{"test_feature": false},
			feature:  "test_feature",
			want:     false,
		},
		{
			name:     "feature is true string",
			features: map[string]interface{}{"test_feature": "true"},
			feature:  "test_feature",
			want:     true,
		},
		{
			name:     "feature is enabled string",
			features: map[string]interface{}{"test_feature": "enabled"},
			feature:  "test_feature",
			want:     true,
		},
		{
			name:     "feature is on string",
			features: map[string]interface{}{"test_feature": "on"},
			feature:  "test_feature",
			want:     true,
		},
		{
			name:     "feature is false string",
			features: map[string]interface{}{"test_feature": "false"},
			feature:  "test_feature",
			want:     false,
		},
		{
			name:     "feature is other type",
			features: map[string]interface{}{"test_feature": 123},
			feature:  "test_feature",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &TenantConfig{
				Features: tt.features,
			}
			assert.Equal(t, tt.want, tc.IsFeatureEnabled(tt.feature))
		})
	}
}

func TestTenantConfig_GetRateLimitForKeyType(t *testing.T) {
	tc := &TenantConfig{
		RateLimitConfig: RateLimitConfig{
			DefaultRequestsPerMinute: 60,
			DefaultRequestsPerHour:   1000,
			DefaultRequestsPerDay:    10000,
			KeyTypeOverrides: map[string]KeyTypeRateLimit{
				"admin": {
					RequestsPerMinute: 1000,
					RequestsPerHour:   50000,
					RequestsPerDay:    500000,
				},
				"agent": {
					RequestsPerMinute: 500,
					RequestsPerHour:   25000,
					RequestsPerDay:    250000,
				},
			},
		},
	}

	t.Run("get override for admin", func(t *testing.T) {
		limit := tc.GetRateLimitForKeyType("admin")
		assert.Equal(t, 1000, limit.RequestsPerMinute)
		assert.Equal(t, 50000, limit.RequestsPerHour)
		assert.Equal(t, 500000, limit.RequestsPerDay)
	})

	t.Run("get override for agent", func(t *testing.T) {
		limit := tc.GetRateLimitForKeyType("agent")
		assert.Equal(t, 500, limit.RequestsPerMinute)
		assert.Equal(t, 25000, limit.RequestsPerHour)
		assert.Equal(t, 250000, limit.RequestsPerDay)
	})

	t.Run("get default for unknown key type", func(t *testing.T) {
		limit := tc.GetRateLimitForKeyType("user")
		assert.Equal(t, 60, limit.RequestsPerMinute)
		assert.Equal(t, 1000, limit.RequestsPerHour)
		assert.Equal(t, 10000, limit.RequestsPerDay)
	})
}

func TestTenantConfig_GetRateLimitForEndpoint(t *testing.T) {
	tc := &TenantConfig{
		RateLimitConfig: RateLimitConfig{
			EndpointOverrides: map[string]EndpointRateLimit{
				"/api/v1/expensive": {
					RequestsPerMinute: 10,
					BurstSize:         20,
				},
				"/api/v1/cheap": {
					RequestsPerMinute: 1000,
					BurstSize:         2000,
				},
			},
		},
	}

	t.Run("get limit for expensive endpoint", func(t *testing.T) {
		limit, exists := tc.GetRateLimitForEndpoint("/api/v1/expensive")
		assert.True(t, exists)
		assert.Equal(t, 10, limit.RequestsPerMinute)
		assert.Equal(t, 20, limit.BurstSize)
	})

	t.Run("get limit for cheap endpoint", func(t *testing.T) {
		limit, exists := tc.GetRateLimitForEndpoint("/api/v1/cheap")
		assert.True(t, exists)
		assert.Equal(t, 1000, limit.RequestsPerMinute)
		assert.Equal(t, 2000, limit.BurstSize)
	})

	t.Run("no limit for unknown endpoint", func(t *testing.T) {
		_, exists := tc.GetRateLimitForEndpoint("/api/v1/unknown")
		assert.False(t, exists)
	})
}

func TestTenantConfig_ServiceTokens(t *testing.T) {
	tc := &TenantConfig{
		ServiceTokens: map[string]string{
			"github": "ghp_test_token",
			"gitlab": "glpat-test_token",
		},
	}

	t.Run("has service token", func(t *testing.T) {
		assert.True(t, tc.HasServiceToken("github"))
		assert.True(t, tc.HasServiceToken("gitlab"))
		assert.False(t, tc.HasServiceToken("bitbucket"))
	})

	t.Run("get service token", func(t *testing.T) {
		token, exists := tc.GetServiceToken("github")
		assert.True(t, exists)
		assert.Equal(t, "ghp_test_token", token)

		token, exists = tc.GetServiceToken("gitlab")
		assert.True(t, exists)
		assert.Equal(t, "glpat-test_token", token)

		_, exists = tc.GetServiceToken("bitbucket")
		assert.False(t, exists)
	})
}

func TestTenantConfig_CompleteScenario(t *testing.T) {
	// Create a complete tenant config
	tc := &TenantConfig{
		ID:       "config-123",
		TenantID: "tenant-456",
		RateLimitConfig: RateLimitConfig{
			DefaultRequestsPerMinute: 100,
			DefaultRequestsPerHour:   5000,
			DefaultRequestsPerDay:    50000,
			KeyTypeOverrides: map[string]KeyTypeRateLimit{
				"admin": {
					RequestsPerMinute: 1000,
					RequestsPerHour:   50000,
					RequestsPerDay:    500000,
				},
			},
			EndpointOverrides: map[string]EndpointRateLimit{
				"/api/v1/analyze": {
					RequestsPerMinute: 5,
					BurstSize:         10,
				},
			},
		},
		ServiceTokens: map[string]string{
			"github": "ghp_secret",
		},
		AllowedOrigins: pq.StringArray{"https://app.example.com", "https://dev.example.com"},
		Features: map[string]interface{}{
			"advanced_analytics": true,
			"beta_features":      "enabled",
			"max_agents":         10,
		},
	}

	// Test all functionality
	assert.Equal(t, "tenant-456", tc.TenantID)
	assert.True(t, tc.IsFeatureEnabled("advanced_analytics"))
	assert.True(t, tc.IsFeatureEnabled("beta_features"))
	assert.False(t, tc.IsFeatureEnabled("max_agents")) // numeric value returns false

	adminLimit := tc.GetRateLimitForKeyType("admin")
	assert.Equal(t, 1000, adminLimit.RequestsPerMinute)

	userLimit := tc.GetRateLimitForKeyType("user")
	assert.Equal(t, 100, userLimit.RequestsPerMinute) // defaults

	endpointLimit, exists := tc.GetRateLimitForEndpoint("/api/v1/analyze")
	assert.True(t, exists)
	assert.Equal(t, 5, endpointLimit.RequestsPerMinute)

	assert.True(t, tc.HasServiceToken("github"))
	token, _ := tc.GetServiceToken("github")
	assert.Equal(t, "ghp_secret", token)

	assert.Contains(t, tc.AllowedOrigins, "https://app.example.com")
}

