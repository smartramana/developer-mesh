package artifactory

import (
	"errors"
	"strings"
)

var (
	// ErrRestrictedOperation indicates that an operation is restricted for safety reasons
	ErrRestrictedOperation = errors.New("operation restricted for safety reasons")
	
	// RestrictedOperations lists Artifactory operations that are restricted
	RestrictedOperations = []string{
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
	
	// ReadOnlyOperations lists operations that are explicitly allowed (read-only operations)
	ReadOnlyOperations = []string{
		"get_artifact",
		"search_artifacts",
		"get_artifact_properties",
		"get_artifact_statistics",
		"get_repository_info",
		"get_builds",
		"get_build_info",
		"get_storage_info",
	}
)

// IsSafeOperation checks if an Artifactory operation is safe to perform
func IsSafeOperation(operation string) (bool, error) {
	// Check if operation is explicitly restricted
	for _, restrictedOp := range RestrictedOperations {
		if operation == restrictedOp {
			return false, ErrRestrictedOperation
		}
	}
	
	// Check if operation is a read operation (starts with "get_" or "search_")
	if strings.HasPrefix(operation, "get_") || strings.HasPrefix(operation, "search_") {
		return true, nil
	}
	
	// Check if operation is explicitly allowed
	for _, allowedOp := range ReadOnlyOperations {
		if operation == allowedOp {
			return true, nil
		}
	}
	
	// Any other operations that aren't explicitly allowed are considered unsafe
	return false, ErrRestrictedOperation
}
