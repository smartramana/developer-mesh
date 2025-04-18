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
		"transfer_repository",
		"add_organization_member",
		"update_repository_visibility",
		"set_team_permissions",
		"add_collaborator_admin",
		"set_admin_permissions",
		"modify_security_settings",
		"disable_branch_protection",
		"modify_default_branch",
	}
	
	// AllowedDangerousOperations lists operations that would normally be dangerous but are explicitly allowed
	AllowedDangerousOperations = []string{
		"archive_repository",
		"lock_issue",
		"close_issue",
		"close_pull_request",
		"delete_webhook",
		"remove_team_member",
		"merge_pull_request",
	}

	// DangerousOperationPrefixes lists prefixes of operations that should be considered dangerous
	DangerousOperationPrefixes = []string{
		"delete_",
		"remove_",
		"force_",
		"update_security_",
		"modify_access_",
		"set_admin_",
		"transfer_",
	}

	// AllowedDeleteOperations specifies which delete operations are explicitly allowed
	AllowedDeleteOperations = []string{
		"delete_webhook",
		"delete_comment",
		"delete_label",
		"delete_milestone",
		"delete_project_column",
		"delete_project_card",
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
	
	// Check if operation is explicitly allowed dangerous operation
	for _, allowedOp := range AllowedDangerousOperations {
		if operation == allowedOp {
			return true, nil
		}
	}
	
	// Check allowed delete operations
	if strings.HasPrefix(operation, "delete_") {
		for _, allowedDelete := range AllowedDeleteOperations {
			if operation == allowedDelete {
				return true, nil
			}
		}
		return false, ErrRestrictedOperation
	}

	// Check for dangerous operation prefixes
	for _, prefix := range DangerousOperationPrefixes {
		if strings.HasPrefix(operation, prefix) {
			// The operation starts with a dangerous prefix and wasn't explicitly allowed
			return false, ErrRestrictedOperation
		}
	}
	
	// All other operations are considered safe
	return true, nil
}

// IsSafeRepository checks if operations on a specific repository should be allowed
// This could be extended to check against an allowlist/blocklist of repositories
func IsSafeRepository(owner, repo string) (bool, error) {
	// Implement repository-specific safety rules if needed
	// For now, all repositories are considered safe
	return true, nil
}

// IsSafeWebhookURL checks if a webhook URL is safe to register
func IsSafeWebhookURL(url string) (bool, error) {
	// Implement checks for webhook URLs
	// For example, verify the URL is in an allowed domain list
	// or reject URLs pointing to known malicious domains
	
	// For now, simply check that the URL uses HTTPS
	if !strings.HasPrefix(url, "https://") {
		return false, errors.New("webhook URLs must use HTTPS for security")
	}
	
	return true, nil
}

// IsSafeBranchOperation checks if an operation on a branch is safe
func IsSafeBranchOperation(operation, branch string) (bool, error) {
	// Check if the operation affects a protected branch like main/master
	protectedBranches := []string{"main", "master", "develop", "production", "staging"}
	
	// For dangerous operations on protected branches, reject
	if strings.HasPrefix(operation, "delete_") || strings.HasPrefix(operation, "force_") {
		for _, protectedBranch := range protectedBranches {
			if branch == protectedBranch {
				return false, errors.New("operation not allowed on protected branch")
			}
		}
	}
	
	return true, nil
}
