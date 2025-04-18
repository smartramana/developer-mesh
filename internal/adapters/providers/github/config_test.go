// Package github provides an adapter for interacting with GitHub repositories,
// issues, pull requests, and other GitHub features.
package github

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	
	adapterConfig "github.com/S-Corkum/mcp-server/internal/adapters/config"
)

// TestDefaultConfig tests the default config generation
func TestDefaultConfig(t *testing.T) {
	// Get default config
	config := DefaultConfig()
	
	// Verify default values
	assert.Equal(t, DefaultTimeout, config.Timeout)
	assert.Contains(t, config.EnabledFeatures, FeatureIssues)
	assert.Contains(t, config.EnabledFeatures, FeaturePullRequests)
	assert.Contains(t, config.EnabledFeatures, FeatureRepositories)
	assert.Contains(t, config.EnabledFeatures, FeatureComments)
	
	// Verify that resilience config is initialized
	assert.NotNil(t, config.Resilience)
	assert.NotNil(t, config.Security)
	assert.NotNil(t, config.Observability)
}

// TestConfigClone tests that config cloning works correctly
func TestConfigClone(t *testing.T) {
	// Create a config
	original := DefaultConfig()
	original.Token = "test-token"
	original.DefaultOwner = "test-owner"
	original.DefaultRepo = "test-repo"
	
	// Clone it
	clone := original.Clone()
	
	// Verify they're equal
	assert.Equal(t, original, clone)
	
	// Modify the clone
	clone.Token = "modified-token"
	clone.EnabledFeatures = append(clone.EnabledFeatures, "new-feature")
	
	// Verify original hasn't changed
	assert.Equal(t, "test-token", original.Token)
	assert.NotContains(t, original.EnabledFeatures, "new-feature")
	
	// Verify the lengths differ
	assert.Equal(t, len(original.EnabledFeatures)+1, len(clone.EnabledFeatures))
}

// TestValidateConfig tests the configuration validation logic
func TestValidateConfig(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name          string
		config        Config
		expectValid   bool
		errorContains string
	}{
		{
			name: "valid config with token",
			config: Config{
				Token:        "test-token",
				Timeout:      10 * time.Second,
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
				EnabledFeatures: []string{
					FeatureIssues, FeaturePullRequests, FeatureRepositories, FeatureComments,
				},
				Resilience: adapterConfig.DefaultAdapterConfig().Resilience,
				Security: adapterConfig.DefaultAdapterConfig().Security,
				Observability: adapterConfig.DefaultAdapterConfig().Observability,
			},
			expectValid: true,
		},
		{
			name: "valid config with GitHub app auth",
			config: Config{
				AppID:        "12345",
				InstallID:    "67890",
				PrivateKey:   "private-key",
				Timeout:      10 * time.Second,
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
				EnabledFeatures: []string{
					FeatureIssues, FeaturePullRequests, FeatureRepositories, FeatureComments,
				},
				Resilience: adapterConfig.DefaultAdapterConfig().Resilience,
				Security: adapterConfig.DefaultAdapterConfig().Security,
				Observability: adapterConfig.DefaultAdapterConfig().Observability,
			},
			expectValid: true,
		},
		{
			name: "invalid - missing authentication",
			config: Config{
				Timeout:      10 * time.Second,
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
				EnabledFeatures: []string{
					FeatureIssues, FeaturePullRequests, FeatureRepositories, FeatureComments,
				},
				Resilience: adapterConfig.DefaultAdapterConfig().Resilience,
				Security: adapterConfig.DefaultAdapterConfig().Security,
				Observability: adapterConfig.DefaultAdapterConfig().Observability,
			},
			expectValid:   false,
			errorContains: "authentication is required",
		},
		{
			name: "invalid - partial app auth",
			config: Config{
				AppID:        "12345",
				// Missing InstallID
				// Missing PrivateKey
				Timeout:      10 * time.Second,
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
				EnabledFeatures: []string{
					FeatureIssues, FeaturePullRequests, FeatureRepositories, FeatureComments,
				},
				Resilience: adapterConfig.DefaultAdapterConfig().Resilience,
				Security: adapterConfig.DefaultAdapterConfig().Security,
				Observability: adapterConfig.DefaultAdapterConfig().Observability,
			},
			expectValid:   false,
			errorContains: "authentication is required",
		},
		{
			name: "invalid - negative timeout",
			config: Config{
				Token:        "test-token",
				Timeout:      -1 * time.Second,
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
				EnabledFeatures: []string{
					FeatureIssues, FeaturePullRequests, FeatureRepositories, FeatureComments,
				},
				Resilience: adapterConfig.DefaultAdapterConfig().Resilience,
				Security: adapterConfig.DefaultAdapterConfig().Security,
				Observability: adapterConfig.DefaultAdapterConfig().Observability,
			},
			expectValid:   false,
			errorContains: "timeout must be positive",
		},
		{
			name: "invalid - timeout too small",
			config: Config{
				Token:        "test-token",
				Timeout:      500 * time.Millisecond, // Less than MinimumTimeout
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
				EnabledFeatures: []string{
					FeatureIssues, FeaturePullRequests, FeatureRepositories, FeatureComments,
				},
				Resilience: adapterConfig.DefaultAdapterConfig().Resilience,
				Security: adapterConfig.DefaultAdapterConfig().Security,
				Observability: adapterConfig.DefaultAdapterConfig().Observability,
			},
			expectValid:   false,
			errorContains: "timeout must be at least",
		},
		{
			name: "invalid - timeout too large",
			config: Config{
				Token:        "test-token",
				Timeout:      120 * time.Second, // More than MaximumTimeout
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
				EnabledFeatures: []string{
					FeatureIssues, FeaturePullRequests, FeatureRepositories, FeatureComments,
				},
				Resilience: adapterConfig.DefaultAdapterConfig().Resilience,
				Security: adapterConfig.DefaultAdapterConfig().Security,
				Observability: adapterConfig.DefaultAdapterConfig().Observability,
			},
			expectValid:   false,
			errorContains: "timeout must not exceed",
		},
		{
			name: "invalid - missing default repo for repo features",
			config: Config{
				Token:        "test-token",
				Timeout:      10 * time.Second,
				// Missing DefaultOwner and DefaultRepo
				EnabledFeatures: []string{
					FeatureIssues, FeaturePullRequests, FeatureComments,
				},
				Resilience: adapterConfig.DefaultAdapterConfig().Resilience,
				Security: adapterConfig.DefaultAdapterConfig().Security,
				Observability: adapterConfig.DefaultAdapterConfig().Observability,
			},
			expectValid:   false,
			errorContains: "default owner and repository are required",
		},
		{
			name: "invalid - empty features",
			config: Config{
				Token:        "test-token",
				Timeout:      10 * time.Second,
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
				EnabledFeatures: []string{}, // No features enabled
				Resilience: adapterConfig.DefaultAdapterConfig().Resilience,
				Security: adapterConfig.DefaultAdapterConfig().Security,
				Observability: adapterConfig.DefaultAdapterConfig().Observability,
			},
			expectValid:   false,
			errorContains: "at least one feature must be enabled",
		},
		{
			name: "invalid - unknown feature",
			config: Config{
				Token:        "test-token",
				Timeout:      10 * time.Second,
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
				EnabledFeatures: []string{
					FeatureIssues, "unknown-feature", // Unknown feature
				},
				Resilience: adapterConfig.DefaultAdapterConfig().Resilience,
				Security: adapterConfig.DefaultAdapterConfig().Security,
				Observability: adapterConfig.DefaultAdapterConfig().Observability,
			},
			expectValid:   false,
			errorContains: "unknown feature",
		},
		{
			name: "invalid - base URL without upload URL",
			config: Config{
				Token:        "test-token",
				BaseURL:      "https://github.example.com/api/v3",
				// Missing UploadURL
				Timeout:      10 * time.Second,
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
				EnabledFeatures: []string{
					FeatureRepositories, // Only repositories feature enabled
				},
				Resilience: adapterConfig.DefaultAdapterConfig().Resilience,
				Security: adapterConfig.DefaultAdapterConfig().Security,
				Observability: adapterConfig.DefaultAdapterConfig().Observability,
			},
			expectValid:   false,
			errorContains: "upload URL must be provided",
		},
		{
			name: "valid - no repo features",
			config: Config{
				Token:        "test-token",
				Timeout:      10 * time.Second,
				// No DefaultOwner and DefaultRepo
				EnabledFeatures: []string{
					FeatureRepositories, // Only repositories feature enabled
				},
				Resilience: adapterConfig.DefaultAdapterConfig().Resilience,
				Security: adapterConfig.DefaultAdapterConfig().Security,
				Observability: adapterConfig.DefaultAdapterConfig().Observability,
			},
			expectValid: true,
		},
	}
	
	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Validate config
			valid, errors := ValidateConfig(tc.config)
			
			// Check result
			if tc.expectValid {
				assert.True(t, valid, "Expected config to be valid, but got errors: %v", errors)
				assert.Empty(t, errors)
			} else {
				assert.False(t, valid, "Expected config to be invalid, but it passed validation")
				assert.NotEmpty(t, errors)
				
				if tc.errorContains != "" {
					// Convert errors slice to a single string for easier checking
					errorsStr := ""
					for _, err := range errors {
						errorsStr += err + "; "
					}
					assert.Contains(t, errorsStr, tc.errorContains, 
						"Error message should contain expected text")
				}
			}
		})
	}
}

// TestResilienceConfig tests the resilience configuration handling
func TestResilienceConfig(t *testing.T) {
	// Create a config with custom resilience settings
	config := DefaultConfig()
	config.Token = "test-token"
	config.DefaultOwner = "test-owner"
	config.DefaultRepo = "test-repo"
	
	// Modify resilience config
	config.Resilience.CircuitBreaker.Enabled = true
	config.Resilience.CircuitBreaker.FailureThreshold = 0.5
	config.Resilience.CircuitBreaker.ResetTimeout = 30 * time.Second
	
	config.Resilience.RateLimiter.Enabled = true
	config.Resilience.RateLimiter.RequestsPerSecond = 10
	
	// Validate config
	valid, errors := ValidateConfig(config)
	
	// Check results
	assert.True(t, valid, "Expected config to be valid, but got errors: %v", errors)
	assert.Empty(t, errors)
	
	// Verify that resilience config is properly handled
	circuitBreakerConfig := config.Resilience.CircuitBreaker.GetCircuitBreakerConfig("github")
	assert.Equal(t, "github", circuitBreakerConfig.Name)
	assert.Equal(t, 0.5, circuitBreakerConfig.FailureThreshold)
	assert.Equal(t, 30*time.Second, circuitBreakerConfig.ResetTimeout)
	
	rateLimiterConfig := config.Resilience.RateLimiter.GetRateLimiterConfig("github")
	assert.Equal(t, "github", rateLimiterConfig.Name)
	assert.Equal(t, 10, rateLimiterConfig.RequestsPerSecond)
}

// TestIsFeatureEnabled tests the IsFeatureEnabled method
func TestIsFeatureEnabled(t *testing.T) {
	// Create a config with specific features
	config := DefaultConfig()
	config.EnabledFeatures = []string{FeatureIssues, FeatureRepositories}
	
	// Test enabled features
	assert.True(t, config.IsFeatureEnabled(FeatureIssues), "Issues feature should be enabled")
	assert.True(t, config.IsFeatureEnabled(FeatureRepositories), "Repositories feature should be enabled")
	
	// Test disabled features
	assert.False(t, config.IsFeatureEnabled(FeaturePullRequests), "Pull requests feature should be disabled")
	assert.False(t, config.IsFeatureEnabled(FeatureComments), "Comments feature should be disabled")
	assert.False(t, config.IsFeatureEnabled("unknown"), "Unknown feature should be disabled")
}

// TestGetTimeout tests the GetTimeout method
func TestGetTimeout(t *testing.T) {
	// Test with valid timeout
	configValid := DefaultConfig()
	configValid.Timeout = 15 * time.Second
	assert.Equal(t, 15*time.Second, configValid.GetTimeout(), "Should return configured timeout")
	
	// Test with zero timeout
	configZero := DefaultConfig()
	configZero.Timeout = 0
	assert.Equal(t, DefaultTimeout, configZero.GetTimeout(), "Should return default timeout for zero")
	
	// Test with negative timeout
	configNegative := DefaultConfig()
	configNegative.Timeout = -5 * time.Second
	assert.Equal(t, DefaultTimeout, configNegative.GetTimeout(), "Should return default timeout for negative")
}
