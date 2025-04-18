// Package github provides an adapter for interacting with GitHub repositories,
// issues, pull requests, and other GitHub features.
package github

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/S-Corkum/mcp-server/internal/adapters/core"
	"github.com/S-Corkum/mcp-server/internal/adapters/events"
	"github.com/S-Corkum/mcp-server/internal/adapters/providers/github/mocks"
	"github.com/S-Corkum/mcp-server/internal/observability"
)

// MockAdapterFactory is a mock implementation of the AdapterFactory interface
type MockAdapterFactory struct {
	mock.Mock
}

// RegisterAdapterCreator mocks the registration of an adapter creator
func (m *MockAdapterFactory) RegisterAdapterCreator(adapterType string, creator core.AdapterCreator) {
	m.Called(adapterType, creator)
}

// Create mocks the creation of an adapter
func (m *MockAdapterFactory) Create(ctx context.Context, adapterType string, config interface{}) (core.Adapter, error) {
	args := m.Called(ctx, adapterType, config)
	if adapter, ok := args.Get(0).(core.Adapter); ok {
		return adapter, args.Error(1)
	}
	return nil, args.Error(1)
}

// TestRegisterAdapter tests the adapter registration process
func TestRegisterAdapter(t *testing.T) {
	t.Run("successful registration", func(t *testing.T) {
		// Create mock factory and dependencies
		mockFactory := new(MockAdapterFactory)
		eventBus := events.NewEventBus()
		metricsClient := observability.NewMetricsClient()
		logger := mocks.NewLogger() // Use our mock logger

		// Set up expectations for the mock
		mockFactory.On("RegisterAdapterCreator", "github", mock.AnythingOfType("core.AdapterCreator")).Return()

		// Call the function being tested
		err := RegisterAdapter(mockFactory, eventBus, metricsClient, logger)
		
		// Verify the result
		require.NoError(t, err, "RegisterAdapter should succeed with valid dependencies")
		
		// Verify the expectations were met
		mockFactory.AssertExpectations(t)
	})
	
	t.Run("nil factory", func(t *testing.T) {
		// Create dependencies except factory
		eventBus := events.NewEventBus()
		metricsClient := observability.NewMetricsClient()
		logger := mocks.NewLogger()
		
		// Call with nil factory
		err := RegisterAdapter(nil, eventBus, metricsClient, logger)
		
		// Verify error
		assert.Error(t, err, "RegisterAdapter should fail with nil factory")
		assert.Contains(t, err.Error(), "factory cannot be nil", "Error should mention nil factory")
	})
	
	t.Run("nil logger", func(t *testing.T) {
		// Create dependencies except logger
		mockFactory := new(MockAdapterFactory)
		eventBus := events.NewEventBus()
		metricsClient := observability.NewMetricsClient()
		
		// Call with nil logger
		err := RegisterAdapter(mockFactory, eventBus, metricsClient, nil)
		
		// Verify error
		assert.Error(t, err, "RegisterAdapter should fail with nil logger")
		assert.Contains(t, err.Error(), "logger cannot be nil", "Error should mention nil logger")
	})
}

// TestAdapterCreation tests the adapter creation process
func TestAdapterCreation(t *testing.T) {
	// Create real factory and dependencies
	factory := core.NewAdapterFactory()
	eventBus := events.NewEventBus()
	metricsClient := observability.NewMetricsClient()
	logger := mocks.NewLogger() // Use our mock logger

	// Register the GitHub adapter
	err := RegisterAdapter(factory, eventBus, metricsClient, logger)
	require.NoError(t, err, "Adapter registration should succeed")

	// Define test cases
	testCases := []struct {
		name        string
		config      interface{}
		expectError bool
		errorContains string
	}{
		{
			name: "valid github config",
			config: Config{
				Token:        "test-token",
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
				EnabledFeatures: []string{
					"issues", "pull_requests", "repositories", "comments",
				},
			},
			expectError: false,
		},
		{
			name:        "invalid config type",
			config:      "not a config",
			expectError: true,
			errorContains: "invalid configuration type",
		},
		{
			name: "invalid github config - missing auth",
			config: Config{
				// Missing token and app auth
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
			},
			expectError: true,
			errorContains: "invalid GitHub adapter configuration",
		},
		{
			name: "invalid github config - missing default repo with features",
			config: Config{
				Token:        "test-token",
				// Missing default owner/repo but has features that require them
				EnabledFeatures: []string{"issues"},
			},
			expectError: true,
			errorContains: "default owner and repository",
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Attempt to create the adapter with context
			ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
			defer cancel()
			
			adapter, err := factory.Create(ctx, "github", tc.config)

			// Check results
			if tc.expectError {
				assert.Error(t, err, "Should get error for %s", tc.name)
				assert.Nil(t, adapter, "Adapter should be nil when error occurs")
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains, 
						"Error message should contain expected text")
				}
			} else {
				assert.NoError(t, err, "Should not get error for %s", tc.name)
				require.NotNil(t, adapter, "Adapter should not be nil")

				// Verify adapter type
				assert.Equal(t, "github", adapter.GetType(), "Adapter type should be 'github'")

				// Verify it's the correct implementation
				_, ok := adapter.(*GitHubAdapter)
				assert.True(t, ok, "Adapter should be of type *GitHubAdapter")
				
				// Test closing the adapter
				closeErr := adapter.Close()
				assert.NoError(t, closeErr, "Closing adapter should succeed")
			}
		})
	}
}

// TestProviderRegistration tests the registration in the providers package
func TestProviderRegistration(t *testing.T) {
	// This is a integration test that verifies the adapter can be registered and created
	// through the providers package

	// Create a new factory
	factory := core.NewAdapterFactory()
	eventBus := events.NewEventBus()
	metricsClient := observability.NewMetricsClient()
	logger := mocks.NewLogger() // Use our mock logger

	// Register the GitHub adapter directly
	err := RegisterAdapter(factory, eventBus, metricsClient, logger)
	require.NoError(t, err, "Adapter registration should succeed")

	// Create valid config
	config := DefaultConfig()
	config.Token = "test-token"
	config.DefaultOwner = "test-owner"
	config.DefaultRepo = "test-repo"

	// Create the adapter with context
	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
	defer cancel()
	
	adapter, err := factory.Create(ctx, "github", config)

	// Verify results
	require.NoError(t, err, "Adapter creation should succeed")
	require.NotNil(t, adapter, "Adapter should not be nil")
	assert.Equal(t, "github", adapter.GetType(), "Adapter type should be 'github'")

	// Verify adapter implementation
	githubAdapter, ok := adapter.(*GitHubAdapter)
	assert.True(t, ok, "Adapter should be of type *GitHubAdapter")
	assert.NotNil(t, githubAdapter.client, "GitHub client should be initialized")
	
	// Test adapter cleanup
	closeErr := adapter.Close()
	assert.NoError(t, closeErr, "Closing adapter should succeed")
}

// Default timeout for tests
var defaultTestTimeout = 5 * time.Second
