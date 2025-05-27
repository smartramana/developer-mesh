package safety

import (
	"errors"
	"strings"
)

// Common safety errors
var (
	// ErrOperationNotAllowed indicates an operation is not allowed by the adapter
	ErrOperationNotAllowed = errors.New("operation not allowed by security policy")

	// ErrMissingPermission indicates insufficient permissions for the operation
	ErrMissingPermission = errors.New("insufficient permissions for this operation")

	// ErrRestrictedOperation indicates that an operation is restricted for safety reasons
	ErrRestrictedOperation = errors.New("operation restricted for safety reasons")
)

// Checker defines the interface for operation safety checks
type Checker interface {
	// IsSafeOperation determines if an operation is safe to perform based on security policies
	IsSafeOperation(operation string, params map[string]interface{}) (bool, error)
}

// DefaultCheck provides a default implementation of safety checks
func DefaultCheck(operation string, params map[string]interface{}) (bool, error) {
	// Check for dangerous operations containing "delete" in the name
	if strings.Contains(strings.ToLower(operation), "delete") ||
		strings.Contains(strings.ToLower(operation), "remove") {
		return false, ErrOperationNotAllowed
	}

	// By default, consider operations safe
	return true, nil
}

// GitHubChecker implements safety checks for GitHub operations
type GitHubChecker struct{}

// IsSafeOperation implements the Checker interface for GitHub
func (c *GitHubChecker) IsSafeOperation(operation string, params map[string]interface{}) (bool, error) {
	// List of restricted GitHub operations
	restrictedOps := []string{
		"delete_repository",
		"delete_team",
		"delete_organization",
		"delete_branch_protection",
		"delete_webhook",
	}

	// List of explicitly allowed "dangerous" operations
	allowedDangerousOps := []string{
		"archive_repository",
		"lock_issue",
		"close_issue",
		"close_pull_request",
	}

	// Check if operation is explicitly restricted
	for _, restrictedOp := range restrictedOps {
		if operation == restrictedOp {
			return false, ErrRestrictedOperation
		}
	}

	// Check if operation contains "delete" but is not in allowed dangerous operations
	if strings.Contains(operation, "delete") {
		for _, allowedOp := range allowedDangerousOps {
			if operation == allowedOp {
				return true, nil
			}
		}
		return false, ErrRestrictedOperation
	}

	// All other operations are considered safe
	return true, nil
}

// NewGitHubChecker creates a new GitHub safety checker
func NewGitHubChecker() *GitHubChecker {
	return &GitHubChecker{}
}

// ArtifactoryChecker implements safety checks for Artifactory operations
type ArtifactoryChecker struct{}

// IsSafeOperation implements the Checker interface for Artifactory
func (c *ArtifactoryChecker) IsSafeOperation(operation string, params map[string]interface{}) (bool, error) {
	// List of restricted Artifactory operations
	restrictedOps := []string{
		"upload_artifact",
		"delete_artifact",
		"delete_repository",
		"delete_build",
		"move_artifact",
		"copy_artifact",
		"update_artifact",
		"deploy_artifact",
		"promote_build",
	}

	// List of explicitly allowed operations (read-only)
	readOnlyOps := []string{
		"get_artifact",
		"search_artifacts",
		"get_artifact_properties",
		"get_artifact_statistics",
		"get_repository_info",
		"get_builds",
		"get_build_info",
		"get_storage_info",
	}

	// Check if operation is explicitly restricted
	for _, restrictedOp := range restrictedOps {
		if operation == restrictedOp {
			return false, ErrRestrictedOperation
		}
	}

	// Check if operation is a read operation (starts with "get_" or "search_")
	if strings.HasPrefix(operation, "get_") || strings.HasPrefix(operation, "search_") {
		return true, nil
	}

	// Check if operation is explicitly allowed
	for _, allowedOp := range readOnlyOps {
		if operation == allowedOp {
			return true, nil
		}
	}

	// Any other operations that aren't explicitly allowed are considered unsafe
	return false, ErrRestrictedOperation
}

// NewArtifactoryChecker creates a new Artifactory safety checker
func NewArtifactoryChecker() *ArtifactoryChecker {
	return &ArtifactoryChecker{}
}

// HarnessChecker implements safety checks for Harness operations
type HarnessChecker struct{}

// IsSafeOperation implements the Checker interface for Harness
func (c *HarnessChecker) IsSafeOperation(operation string, params map[string]interface{}) (bool, error) {
	// List of restricted Harness operations
	restrictedOps := []string{
		"delete_feature_flag",
		"delete_pipeline",
		"delete_service",
		"delete_environment",
		"delete_connector",
		"delete_secret",
		"delete_template",
	}

	// List of restricted operation prefixes
	restrictedPrefixes := []string{
		"delete_prod_",
		"delete_production_",
	}

	// Check if operation is explicitly restricted
	for _, restrictedOp := range restrictedOps {
		if operation == restrictedOp {
			return false, ErrRestrictedOperation
		}
	}

	// Check restricted prefixes
	for _, prefix := range restrictedPrefixes {
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

// NewHarnessChecker creates a new Harness safety checker
func NewHarnessChecker() *HarnessChecker {
	return &HarnessChecker{}
}

// DefaultAdapterChecker implements a default safety checker that allows all operations
type DefaultAdapterChecker struct{}

// IsSafeOperation implements the Checker interface with a permissive default
func (c *DefaultAdapterChecker) IsSafeOperation(operation string, params map[string]interface{}) (bool, error) {
	return true, nil
}

// GetCheckerForAdapter returns the appropriate safety checker for a given adapter
func GetCheckerForAdapter(adapterName string) Checker {
	switch adapterName {
	case "github":
		return NewGitHubChecker()
	case "artifactory":
		return NewArtifactoryChecker()
	case "harness":
		return NewHarnessChecker()
	default:
		// Return a dummy checker that allows everything for other adapters
		return &DefaultAdapterChecker{}
	}
}
