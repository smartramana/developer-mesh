package harness

import (
	"errors"
	"strings"
)

var (
	// ErrRestrictedOperation indicates that an operation is restricted for safety reasons
	ErrRestrictedOperation = errors.New("operation restricted for safety reasons")
	
	// RestrictedOperations lists Harness operations that are restricted
	RestrictedOperations = []string{
		"delete_feature_flag",
		"delete_pipeline",
		"delete_service",
		"delete_environment",
		"delete_connector",
		"delete_secret",
		"delete_template",
	}
	
	// RestrictedPrefixes lists prefixes of operations that should be restricted
	RestrictedPrefixes = []string{
		"delete_prod_",
		"delete_production_",
	}
)

// IsSafeOperation checks if a Harness operation is safe to perform
// It also requires environment context for production-related restrictions
func IsSafeOperation(operation string, params map[string]interface{}) (bool, error) {
	// Check if operation is explicitly restricted
	for _, restrictedOp := range RestrictedOperations {
		if operation == restrictedOp {
			return false, ErrRestrictedOperation
		}
	}
	
	// Check restricted prefixes
	for _, prefix := range RestrictedPrefixes {
		if strings.HasPrefix(operation, prefix) {
			return false, ErrRestrictedOperation
		}
	}
	
	// Special case for feature flags - check if targeting production
	if strings.Contains(operation, "feature_flag") && 
	   (strings.Contains(operation, "delete") || 
	    strings.Contains(operation, "toggle") || 
	    strings.Contains(operation, "update")) {
		
		// Check if environment is production
		if env, ok := params["environment"].(string); ok {
			if strings.ToLower(env) == "production" || strings.ToLower(env) == "prod" {
				return false, errors.New("modifying production feature flags is restricted")
			}
		}
		
		// Check if environment_id matches production patterns
		if envID, ok := params["environment_id"].(string); ok {
			if strings.Contains(strings.ToLower(envID), "prod") {
				return false, errors.New("modifying production feature flags is restricted")
			}
		}
	}
	
	// All other operations are considered safe
	return true, nil
}
