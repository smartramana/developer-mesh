package factory

import (
	"context"
	"testing"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters/github"
	"github.com/S-Corkum/mcp-server/internal/adapters/harness"
	"github.com/S-Corkum/mcp-server/internal/adapters/sonarqube"
	"github.com/S-Corkum/mcp-server/internal/adapters/artifactory"
	"github.com/S-Corkum/mcp-server/internal/adapters/xray"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock the adapter initialization to skip external API calls
type factoryWrapper struct {
	*Factory
}

func (f *factoryWrapper) CreateAdapter(ctx context.Context, adapterType string) (interface{}, error) {
	var adapter interface{}
	var err error

	// Get configuration for the adapter type
	config, ok := f.configs[adapterType]
	if !ok {
		return nil, nil
	}

	// Create adapter based on type but skip initialization for testing
	switch adapterType {
	case "github":
		githubConfig, ok := config.(github.Config)
		if !ok {
			return nil, nil
		}
		adapter, err = github.NewAdapter(githubConfig)

	case "harness":
		harnessConfig, ok := config.(harness.Config)
		if !ok {
			return nil, nil
		}
		adapter, err = harness.NewAdapter(harnessConfig)

	case "sonarqube":
		sonarqubeConfig, ok := config.(sonarqube.Config)
		if !ok {
			return nil, nil
		}
		adapter, err = sonarqube.NewAdapter(sonarqubeConfig)

	case "artifactory":
		artifactoryConfig, ok := config.(artifactory.Config)
		if !ok {
			return nil, nil
		}
		adapter, err = artifactory.NewAdapter(artifactoryConfig)

	case "xray":
		xrayConfig, ok := config.(xray.Config)
		if !ok {
			return nil, nil
		}
		adapter, err = xray.NewAdapter(xrayConfig)

	default:
		return nil, nil
	}

	return adapter, err
}

func TestNewFactory(t *testing.T) {
	configs := map[string]interface{}{
		"github": github.Config{
			MockResponses: true,
		},
	}
	
	factory := NewFactory(configs)
	assert.NotNil(t, factory)
	assert.Equal(t, configs, factory.configs)
}

func TestCreateAdapterTypes(t *testing.T) {
	// Set up a factory with mock configurations for testing
	configs := map[string]interface{}{
		"github": github.Config{
			MockResponses:  true,
			RequestTimeout: 5 * time.Second,
			MaxRetries:     3,
			RetryDelay:     1 * time.Second,
		},
		"harness": harness.Config{
			APIToken:       "test-token",
			AccountID:      "test-account",
			BaseURL:        "http://localhost:9999", // Non-existent for testing
			RequestTimeout: 5 * time.Second,
			MaxRetries:     0, // Don't retry in tests
			RetryDelay:     1 * time.Second,
		},
		"sonarqube": sonarqube.Config{
			BaseURL:        "http://localhost:9999", // Non-existent for testing
			RequestTimeout: 5 * time.Second,
			MaxRetries:     0, // Don't retry in tests
			RetryDelay:     1 * time.Second,
		},
		"artifactory": artifactory.Config{
			BaseURL:        "http://localhost:9999", // Non-existent for testing
			RequestTimeout: 5 * time.Second,
			MaxRetries:     0, // Don't retry in tests
			RetryDelay:     1 * time.Second,
		},
		"xray": xray.Config{
			BaseURL:        "http://localhost:9999", // Non-existent for testing
			RequestTimeout: 5 * time.Second,
			MaxRetries:     0, // Don't retry in tests
			RetryDelay:     1 * time.Second,
		},
	}
	
	factoryW := &factoryWrapper{NewFactory(configs)}
	ctx := context.Background()
	
	t.Run("GitHub Adapter", func(t *testing.T) {
		adapter, err := factoryW.CreateAdapter(ctx, "github")
		require.NoError(t, err)
		assert.NotNil(t, adapter)
		
		// Check type of adapter
		_, ok := adapter.(*github.Adapter)
		assert.True(t, ok, "Expected GitHub adapter")
	})
	
	t.Run("Harness Adapter", func(t *testing.T) {
		adapter, err := factoryW.CreateAdapter(ctx, "harness")
		require.NoError(t, err)
		assert.NotNil(t, adapter)
		
		// Check type of adapter
		_, ok := adapter.(*harness.Adapter)
		assert.True(t, ok, "Expected Harness adapter")
	})
	
	t.Run("SonarQube Adapter", func(t *testing.T) {
		adapter, err := factoryW.CreateAdapter(ctx, "sonarqube")
		require.NoError(t, err)
		assert.NotNil(t, adapter)
		
		// Check type of adapter
		_, ok := adapter.(*sonarqube.Adapter)
		assert.True(t, ok, "Expected SonarQube adapter")
	})
	
	t.Run("Artifactory Adapter", func(t *testing.T) {
		adapter, err := factoryW.CreateAdapter(ctx, "artifactory") 
		require.NoError(t, err)
		assert.NotNil(t, adapter)
		
		// Check type of adapter
		_, ok := adapter.(*artifactory.Adapter)
		assert.True(t, ok, "Expected Artifactory adapter")
	})
	
	t.Run("Xray Adapter", func(t *testing.T) {
		adapter, err := factoryW.CreateAdapter(ctx, "xray")
		require.NoError(t, err)
		assert.NotNil(t, adapter)
		
		// Check type of adapter
		_, ok := adapter.(*xray.Adapter)
		assert.True(t, ok, "Expected Xray adapter")
	})
	
	t.Run("Unknown Adapter", func(t *testing.T) {
		adapter, err := factoryW.CreateAdapter(ctx, "unknown")
		assert.Nil(t, err) // Our wrapper returns nil for unknown
		assert.Nil(t, adapter)
	})
}

func TestCreateAdapterConfiguration(t *testing.T) {
	// Test configuration validation
	t.Run("Invalid Configuration", func(t *testing.T) {
		// Create a factory with invalid configuration type
		invalidConfigs := map[string]interface{}{
			"github": "not a valid config",
		}
		
		factory := NewFactory(invalidConfigs)
		ctx := context.Background()
		adapter, err := factory.CreateAdapter(ctx, "github")
		assert.Error(t, err)
		assert.Nil(t, adapter)
		assert.Contains(t, err.Error(), "invalid configuration type")
	})
}
