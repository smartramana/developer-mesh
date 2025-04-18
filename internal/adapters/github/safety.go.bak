package github

import (
	"errors"
	"strings"
)

var (
	// ErrRestrictedOperation indicates that an operation is restricted for safety reasons
	ErrRestrictedOperation = errors.New("operation restricted for safety reasons")
	
	// RestrictedOperations lists GitHub operations that are restricted
	RestrictedOperations = []string{
		"delete_repository",
		"delete_team",
		"delete_organization",
		"delete_branch_protection",
		"delete_webhook",
	}
	
	// AllowedDangerousOperations lists operations that would normally be dangerous but are explicitly allowed
	AllowedDangerousOperations = []string{
		"archive_repository",
		"lock_issue",
		"close_issue",
		"close_pull_request",
	}
)

// IsSafeOperation checks if a GitHub operation is safe to perform
func IsSafeOperation(operation string) (bool, error) {
	// Check if operation is explicitly restricted
	for _, restrictedOp := range RestrictedOperations {
		if operation == restrictedOp {
			return false, ErrRestrictedOperation
		}
	}
	
	// Check if operation contains "delete" but is not in allowed dangerous operations
	if strings.Contains(operation, "delete") {
		for _, allowedOp := range AllowedDangerousOperations {
			if operation == allowedOp {
				return true, nil
			}
		}
		return false, ErrRestrictedOperation
	}
	
	// All other operations are considered safe
	return true, nil
}
