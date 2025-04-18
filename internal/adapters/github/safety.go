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
		"modify_access_token",
		"update_security_policy",
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
	
	// ProtectedBranches lists branch names that should be protected from destructive operations
	ProtectedBranches = []string{
		"main",
		"master",
		"develop",
		"production",
		"staging",
		"release",
		"gh-pages",
	}
)

// IsSafeOperation checks if a GitHub operation is safe to perform
func IsSafeOperation(operation string) (bool, error) {
	// Check if operation is explicitly restricted
	for _, restrictedOp := range RestrictedOperations {
		if operation == restrictedOp {
			return false, errors.New("operation is explicitly restricted: " + operation)
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
		return false, errors.New("delete operation not explicitly allowed: " + operation)
	}

	// Check for dangerous operation prefixes
	for _, prefix := range DangerousOperationPrefixes {
		if strings.HasPrefix(operation, prefix) {
			// The operation starts with a dangerous prefix and wasn't explicitly allowed
			return false, errors.New("operation has dangerous prefix: " + prefix)
		}
	}
	
	// All other operations are considered safe
	return true, nil
}

// IsSafeRepository checks if operations on a specific repository should be allowed
// This can be extended to check against an allowlist/blocklist of repositories
func IsSafeRepository(owner, repo string) (bool, error) {
	// Add specific repository safety checks here if needed
	
	// Check if repository is in a list of protected repositories
	protectedRepos := map[string]bool{
		"infrastructure": true,
		"security":       true,
		"config":         true,
		"iam":            true,
		"auth":           true,
		"secrets":        true,
	}
	
	// Check if the repo name is in the protected list
	if protectedRepos[strings.ToLower(repo)] {
		return false, errors.New("repository is protected from automated operations: " + owner + "/" + repo)
	}
	
	// For now, all repositories are considered safe
	return true, nil
}

// IsSafeWebhookURL checks if a webhook URL is safe to register
func IsSafeWebhookURL(url string) (bool, error) {
	// Require HTTPS for security
	if !strings.HasPrefix(url, "https://") {
		return false, errors.New("webhook URLs must use HTTPS for security")
	}
	
	// Check against allowed domains if needed
	allowedDomains := []string{
		"api.github.com",
		"hooks.slack.com",
		"jenkins.",
		"travis-ci.",
		"circleci.",
		"gitlab.",
		"jira.",
		"atlassian.",
	}
	
	// Allow internal/company domains
	internalDomains := []string{
		"company.com",
		"company.io",
		"company.net",
		"company-internal.com",
	}
	
	// Check if URL is from allowed external domain
	for _, domain := range allowedDomains {
		if strings.Contains(url, domain) {
			return true, nil
		}
	}
	
	// Check if URL is from internal domain
	for _, domain := range internalDomains {
		if strings.Contains(url, domain) {
			return true, nil
		}
	}
	
	// If we have explicit domain checks, we should return an error if none match
	// For now, just return true for any HTTPS URL
	return true, nil
}

// IsSafeBranchOperation checks if an operation on a branch is safe
func IsSafeBranchOperation(operation, branch string) (bool, error) {
	// Check if the operation affects a protected branch
	// For dangerous operations on protected branches, reject
	if (strings.HasPrefix(operation, "delete_") || 
		strings.HasPrefix(operation, "force_") || 
		strings.HasPrefix(operation, "modify_") ||
		operation == "set_default_branch") {
		
		// Check against protected branch list
		for _, protectedBranch := range ProtectedBranches {
			if strings.EqualFold(branch, protectedBranch) {
				return false, errors.New("operation not allowed on protected branch: " + branch)
			}
		}
	}
	
	return true, nil
}

// IsSafeTeamOperation checks if an operation on a team is safe
func IsSafeTeamOperation(operation, team string) (bool, error) {
	// Check for protected teams
	protectedTeams := []string{
		"owners",
		"admins",
		"administrators",
		"security",
		"security-team",
	}
	
	// Check if operation affects a protected team
	if (strings.HasPrefix(operation, "delete_") || 
		strings.HasPrefix(operation, "modify_") ||
		strings.HasPrefix(operation, "update_") ||
		strings.HasPrefix(operation, "change_")) {
		
		// Check against protected team list
		for _, protectedTeam := range protectedTeams {
			if strings.EqualFold(team, protectedTeam) {
				return false, errors.New("operation not allowed on protected team: " + team)
			}
		}
	}
	
	return true, nil
}

// IsSafePermissionLevel checks if a permission level is safe to assign
func IsSafePermissionLevel(permission string) (bool, error) {
	// Convert to lowercase for comparison
	permissionLower := strings.ToLower(permission)
	
	// Check if permission is admin/high-level
	if permissionLower == "admin" || permissionLower == "administrator" || 
	   permissionLower == "owner" || permissionLower == "maintain" {
		return false, errors.New("assigning high-level permissions requires manual review")
	}
	
	// Allow read/write permissions
	if permissionLower == "read" || permissionLower == "write" || 
	   permissionLower == "pull" || permissionLower == "push" || 
	   permissionLower == "triage" {
		return true, nil
	}
	
	// Unknown permission level
	return false, errors.New("unknown permission level: " + permission)
}

// IsSafeOrganizationMember checks if a user is safe to add to an organization
func IsSafeOrganizationMember(username string) (bool, error) {
	// This is a placeholder for a more sophisticated implementation
	// In a real-world scenario, you might want to check against:
	// - A list of allowed/vetted users
	// - An HR/identity system API
	// - A specific email domain
	
	// For now, implement a basic allowlist
	allowedUsers := map[string]bool{
		"user1": true,
		"user2": true,
		// Add more allowed users as needed
	}
	
	// Check if the user is in the allow list
	if allowedUsers[username] {
		return true, nil
	}
	
	// If we have an explicit allowlist, we might want to reject unknown users
	// For this example, we'll return true for any user
	return true, nil
}
