package harness

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSafeOperation_Safety(t *testing.T) {
	t.Run("Safe Operations", func(t *testing.T) {
		safeOperations := []string{
			"trigger_pipeline",
			"get_pipeline_status",
			"create_environment",
			"update_service",
			"random_operation", // Unknown operations should default to safe
		}

		for _, op := range safeOperations {
			safe, err := IsSafeOperation(op, make(map[string]interface{}))
			assert.True(t, safe, "Expected operation '%s' to be safe", op)
			assert.NoError(t, err)
		}
	})

	t.Run("Explicitly Restricted Operations", func(t *testing.T) {
		restrictedOperations := []string{
			"delete_feature_flag",
			"delete_pipeline",
			"delete_service",
			"delete_environment",
			"delete_connector",
			"delete_secret",
			"delete_template",
		}

		for _, op := range restrictedOperations {
			safe, err := IsSafeOperation(op, make(map[string]interface{}))
			assert.False(t, safe, "Expected operation '%s' to be restricted", op)
			assert.Error(t, err)
			assert.Equal(t, ErrRestrictedOperation, err)
		}
	})

	t.Run("Restricted Operation Prefixes", func(t *testing.T) {
		prefixedOperations := []string{
			"delete_prod_service",
			"delete_production_environment",
		}

		for _, op := range prefixedOperations {
			safe, err := IsSafeOperation(op, make(map[string]interface{}))
			assert.False(t, safe, "Expected operation '%s' to be restricted", op)
			assert.Error(t, err)
			assert.Equal(t, ErrRestrictedOperation, err)
		}
	})

	t.Run("Feature Flag Production Restrictions", func(t *testing.T) {
		// Test feature flag operations in production environment
		featureFlagOps := []string{
			"toggle_feature_flag",
			"update_feature_flag",
			"delete_feature_flag",
		}

		// Test with production environment name
		prodParams := map[string]interface{}{
			"environment": "Production",
		}

		for _, op := range featureFlagOps {
			safe, err := IsSafeOperation(op, prodParams)
			if op == "delete_feature_flag" {
				// delete_feature_flag is always restricted regardless of environment
				assert.False(t, safe)
				assert.Error(t, err)
				assert.Equal(t, ErrRestrictedOperation, err)
			} else if op == "toggle_feature_flag" || op == "update_feature_flag" {
				assert.False(t, safe, "Expected operation '%s' to be restricted in production", op)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "production feature flags is restricted")
			}
		}

		// Test with prod in environment_id
		prodIDParams := map[string]interface{}{
			"environment_id": "prod-us-east",
		}

		for _, op := range featureFlagOps {
			safe, err := IsSafeOperation(op, prodIDParams)
			if op == "delete_feature_flag" {
				// delete_feature_flag is always restricted regardless of environment
				assert.False(t, safe)
				assert.Error(t, err)
				assert.Equal(t, ErrRestrictedOperation, err)
			} else if op == "toggle_feature_flag" || op == "update_feature_flag" {
				assert.False(t, safe, "Expected operation '%s' to be restricted in production", op)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "production feature flags is restricted")
			}
		}

		// Test with non-production environment
		nonProdParams := map[string]interface{}{
			"environment": "development",
		}

		for _, op := range featureFlagOps {
			if op == "delete_feature_flag" {
				// delete_feature_flag is always restricted regardless of environment
				safe, err := IsSafeOperation(op, nonProdParams)
				assert.False(t, safe)
				assert.Error(t, err)
				assert.Equal(t, ErrRestrictedOperation, err)
			} else {
				safe, err := IsSafeOperation(op, nonProdParams)
				assert.True(t, safe, "Expected operation '%s' to be allowed in non-production", op)
				assert.NoError(t, err)
			}
		}
	})
}
