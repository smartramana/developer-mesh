package harness

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewAdapter(t *testing.T) {
	// Test with default configuration
	config := Config{
		APIToken:  "test-token",
		AccountID: "test-account",
	}

	adapter, err := NewAdapter(config)
	assert.NoError(t, err)
	assert.NotNil(t, adapter)
	
	// Verify default values were set
	assert.Equal(t, 30*time.Second, adapter.config.RequestTimeout)
	assert.Equal(t, 3, adapter.config.RetryMax)
	assert.Equal(t, 1*time.Second, adapter.config.RetryDelay)
	assert.Equal(t, "https://app.harness.io", adapter.config.BaseURL)
	assert.Equal(t, "https://app.harness.io/ng/api", adapter.config.APIURL)
	assert.Equal(t, "https://app.harness.io/gateway/api/graphql", adapter.config.GraphQLURL)
	assert.Equal(t, "https://app.harness.io/ccm/api", adapter.config.CCMAPIURL)
	assert.Equal(t, "https://app.harness.io/ccm/graphql", adapter.config.CCMGraphQLURL)
	assert.Equal(t, "default", adapter.config.OrgIdentifier)
	
	// Test with custom configuration
	customConfig := Config{
		APIToken:       "test-token",
		AccountID:      "test-account",
		OrgIdentifier:  "custom-org",
		RequestTimeout: 60 * time.Second,
		RetryMax:       5,
		RetryDelay:     2 * time.Second,
		BaseURL:        "https://custom.harness.io",
	}

	customAdapter, err := NewAdapter(customConfig)
	assert.NoError(t, err)
	assert.NotNil(t, customAdapter)
	
	// Verify custom values were set
	assert.Equal(t, 60*time.Second, customAdapter.config.RequestTimeout)
	assert.Equal(t, 5, customAdapter.config.RetryMax)
	assert.Equal(t, 2*time.Second, customAdapter.config.RetryDelay)
	assert.Equal(t, "https://custom.harness.io", customAdapter.config.BaseURL)
	assert.Equal(t, "https://custom.harness.io/ng/api", customAdapter.config.APIURL)
	assert.Equal(t, "https://custom.harness.io/gateway/api/graphql", customAdapter.config.GraphQLURL)
	assert.Equal(t, "https://custom.harness.io/ccm/api", customAdapter.config.CCMAPIURL)
	assert.Equal(t, "https://custom.harness.io/ccm/graphql", customAdapter.config.CCMGraphQLURL)
	assert.Equal(t, "custom-org", customAdapter.config.OrgIdentifier)
}

func TestIsSafeOperation(t *testing.T) {
	config := Config{
		APIToken:  "test-token",
		AccountID: "test-account",
	}
	
	adapter, err := NewAdapter(config)
	assert.NoError(t, err)
	
	// Test safe operations
	safeOps := []string{
		"trigger_pipeline",
		"stop_pipeline",
		"rollback_deployment",
		"toggle_feature_flag",
		"apply_ccm_recommendation",
		"ignore_ccm_recommendation",
		"create_ccm_budget",
		"update_ccm_budget",
		"ignore_ccm_anomaly",
		"acknowledge_ccm_anomaly",
	}
	
	for _, op := range safeOps {
		safe, err := adapter.IsSafeOperation(op, nil)
		assert.NoError(t, err)
		assert.True(t, safe, "Expected operation '%s' to be safe", op)
	}
	
	// Test unknown operation (defaults to safe)
	safe, err := adapter.IsSafeOperation("unknown_op", nil)
	assert.NoError(t, err)
	assert.True(t, safe, "Expected unknown operation to default to safe")
}

func TestInitializeError(t *testing.T) {
	// Test validation error when API token is missing
	config := Config{
		AccountID: "test-account",
	}
	
	adapter, err := NewAdapter(config)
	assert.NoError(t, err)
	
	ctx := context.Background()
	err = adapter.Initialize(ctx, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API token is required")
	
	// Test validation error when Account ID is missing
	config2 := Config{
		APIToken: "test-token",
	}
	
	adapter2, err := NewAdapter(config2)
	assert.NoError(t, err)
	
	err = adapter2.Initialize(ctx, config2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Account ID is required")
}

func TestHealth(t *testing.T) {
	config := Config{
		APIToken:  "test-token",
		AccountID: "test-account",
	}
	
	adapter, err := NewAdapter(config)
	assert.NoError(t, err)
	
	// Test initial health status
	assert.Equal(t, "initializing", adapter.Health())
	
	// Test setting and getting health status
	adapter.healthStatus = "healthy"
	assert.Equal(t, "healthy", adapter.Health())
}

func TestClose(t *testing.T) {
	config := Config{
		APIToken:  "test-token",
		AccountID: "test-account",
	}
	
	adapter, err := NewAdapter(config)
	assert.NoError(t, err)
	
	// Test close returns no error
	err = adapter.Close()
	assert.NoError(t, err)
}

func TestSubscribe(t *testing.T) {
	config := Config{
		APIToken:  "test-token",
		AccountID: "test-account",
	}
	
	adapter, err := NewAdapter(config)
	assert.NoError(t, err)
	
	// Test Subscribe returns no error
	err = adapter.Subscribe("event", func(data interface{}) {})
	assert.NoError(t, err)
}

func TestHandleWebhook(t *testing.T) {
	config := Config{
		APIToken:  "test-token",
		AccountID: "test-account",
	}
	
	adapter, err := NewAdapter(config)
	assert.NoError(t, err)
	
	// Test HandleWebhook returns no error
	ctx := context.Background()
	err = adapter.HandleWebhook(ctx, "event", []byte("payload"))
	assert.NoError(t, err)
}
