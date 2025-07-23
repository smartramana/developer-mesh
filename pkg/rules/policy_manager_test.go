package rules

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyManagerConcurrentLoadPolicies(t *testing.T) {
	// Create test dependencies
	logger := observability.NewLogger("test")
	cacheClient := cache.NewMemoryCache(1000, 5*time.Minute)
	metricsClient := observability.NewMetricsClient()

	// Create policy manager
	config := PolicyManagerConfig{
		MaxPolicies: 1000,
	}
	pm := NewPolicyManager(config, cacheClient, logger, metricsClient)

	// Create test policies
	createTestPolicies := func(prefix string, count int) []Policy {
		policies := make([]Policy, count)
		for i := 0; i < count; i++ {
			policies[i] = Policy{
				Name:     prefix + "_policy_" + string(rune(i)),
				Resource: "test_resource",
				Rules: []PolicyRule{
					{
						Effect:    "allow",
						Actions:   []string{"read"},
						Resources: []string{"*"},
						Condition: "true",
					},
				},
			}
		}
		return policies
	}

	// Test concurrent LoadPolicies calls
	t.Run("concurrent LoadPolicies", func(t *testing.T) {
		ctx := context.Background()
		numGoroutines := 10
		policiesPerGoroutine := 10

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		// Channel to collect errors
		errCh := make(chan error, numGoroutines)

		// Start time
		start := time.Now()

		// Launch concurrent LoadPolicies
		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				defer wg.Done()

				policies := createTestPolicies("goroutine_"+string(rune(idx)), policiesPerGoroutine)
				if err := pm.LoadPolicies(ctx, policies); err != nil {
					errCh <- err
				}
			}(i)
		}

		// Wait for completion or timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success - all goroutines completed
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out - possible deadlock")
		}

		close(errCh)

		// Check for errors
		for err := range errCh {
			t.Errorf("LoadPolicies error: %v", err)
		}

		// Verify timing - should complete quickly
		elapsed := time.Since(start)
		assert.Less(t, elapsed, 2*time.Second, "LoadPolicies took too long: %v", elapsed)
	})

	// Test concurrent ValidatePolicy during LoadPolicies
	t.Run("concurrent ValidatePolicy during LoadPolicies", func(t *testing.T) {
		ctx := context.Background()

		// Start LoadPolicies in background
		go func() {
			policies := createTestPolicies("background", 50)
			_ = pm.LoadPolicies(ctx, policies)
		}()

		// Give LoadPolicies a moment to start
		time.Sleep(10 * time.Millisecond)

		// Try to validate policies concurrently
		var wg sync.WaitGroup
		numValidations := 20
		wg.Add(numValidations)

		for i := 0; i < numValidations; i++ {
			go func(idx int) {
				defer wg.Done()

				policy := &Policy{
					Name:     "validate_policy_" + string(rune(idx)),
					Resource: "test_resource",
					Rules: []PolicyRule{
						{
							Effect:    "allow",
							Actions:   []string{"read"},
							Resources: []string{"*"},
							Condition: "true",
						},
					},
				}

				// This should not deadlock
				err := pm.ValidatePolicy(ctx, policy)
				assert.NoError(t, err)
			}(i)
		}

		// Wait for completion or timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success - no deadlock
		case <-time.After(3 * time.Second):
			t.Fatal("ValidatePolicy deadlocked during LoadPolicies")
		}
	})

	// Test policy count limit enforcement
	t.Run("policy count limit", func(t *testing.T) {
		ctx := context.Background()

		// Create a manager with low limit
		limitedConfig := config
		limitedConfig.MaxPolicies = 5
		limitedPM := NewPolicyManager(limitedConfig, cacheClient, logger, metricsClient)

		// Load policies up to limit
		policies := createTestPolicies("limited", 5)
		err := limitedPM.LoadPolicies(ctx, policies)
		require.NoError(t, err)

		// Try to add one more via AddPolicy (which uses ValidatePolicy)
		extraPolicy := Policy{
			Name:     "extra_policy",
			Resource: "test_resource",
			Rules: []PolicyRule{
				{
					Effect:    "allow",
					Actions:   []string{"read"},
					Resources: []string{"*"},
					Condition: "true",
				},
			},
		}

		// AddPolicy doesn't exist, we'll just validate
		err = limitedPM.ValidatePolicy(ctx, &extraPolicy)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "maximum number of policies")
	})
}
