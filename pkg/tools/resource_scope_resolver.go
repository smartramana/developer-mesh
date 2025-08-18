package tools

import (
	"fmt"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/getkin/kin-openapi/openapi3"
)

// ResourceScopeResolver filters OpenAPI operations based on resource scope
// This follows 2025 best practices for API orchestration and AI agent interaction
type ResourceScopeResolver struct {
	logger observability.Logger
}

// NewResourceScopeResolver creates a new resource scope resolver
func NewResourceScopeResolver(logger observability.Logger) *ResourceScopeResolver {
	return &ResourceScopeResolver{
		logger: logger,
	}
}

// ResourceScope defines what operations a tool should expose
type ResourceScope struct {
	// Primary resource type (e.g., "issues", "repos", "pulls")
	ResourceType string

	// Secondary resource types this tool can access
	SecondaryResources []string

	// Tag filters - only operations with these tags
	RequiredTags []string

	// Path patterns to include (e.g., "/repos/{owner}/{repo}/issues")
	PathPatterns []string

	// Path patterns to exclude
	ExcludePatterns []string

	// Operation ID prefixes to include (e.g., "issues/", "repos/")
	OperationPrefixes []string

	// Specific operation IDs to always include
	IncludeOperations []string

	// Specific operation IDs to always exclude
	ExcludeOperations []string
}

// ExtractResourceScopeFromToolName intelligently determines resource scope from tool name
// This is the GENERIC approach that works with ANY API
func (r *ResourceScopeResolver) ExtractResourceScopeFromToolName(toolName string) *ResourceScope {
	// Normalize tool name
	normalized := strings.ToLower(toolName)

	// Extract resource type from tool name
	// Common patterns: github_issues, gitlab_merge_requests, jira_tickets
	parts := strings.Split(normalized, "_")
	if len(parts) < 2 {
		// No specific resource scope
		return nil
	}

	// The last part is usually the resource type
	resourceType := parts[len(parts)-1]

	// Handle pluralization (issues -> issue, repos -> repo)
	singularResource := r.singularize(resourceType)

	scope := &ResourceScope{
		ResourceType:       resourceType,
		SecondaryResources: []string{},
		PathPatterns:       []string{},
		OperationPrefixes:  []string{},
	}

	// Build path patterns based on resource type
	// This works generically for any REST API
	scope.PathPatterns = append(scope.PathPatterns,
		fmt.Sprintf("/%s", resourceType),                    // /issues
		fmt.Sprintf("/%s/", resourceType),                   // /issues/
		fmt.Sprintf("/%s/{", singularResource),              // /issue/{id}
		fmt.Sprintf("/{owner}/{repo}/%s", resourceType),     // /{owner}/{repo}/issues
		fmt.Sprintf("/{org}/{project}/%s", resourceType),    // /{org}/{project}/issues
		fmt.Sprintf("/{workspace}/{repo}/%s", resourceType), // Bitbucket style
	)

	// Build operation prefixes
	scope.OperationPrefixes = append(scope.OperationPrefixes,
		resourceType+"/",     // issues/
		resourceType+"-",     // issues-
		singularResource+"/", // issue/
		singularResource+"-", // issue-
	)

	// Handle special resource relationships (following REST best practices)
	switch resourceType {
	case "issues":
		scope.SecondaryResources = []string{"comments", "labels", "milestones", "assignees"}
		scope.RequiredTags = []string{"issues"}
	case "pulls", "pull_requests", "merge_requests":
		scope.SecondaryResources = []string{"reviews", "commits", "files", "comments"}
		scope.RequiredTags = []string{"pulls", "pull-requests", "merge-requests"}
	case "repos", "repositories":
		scope.SecondaryResources = []string{"branches", "tags", "releases", "contributors"}
		scope.RequiredTags = []string{"repos", "repositories"}
	case "users":
		scope.SecondaryResources = []string{"followers", "following", "orgs", "repos"}
		scope.RequiredTags = []string{"users"}
	case "projects":
		scope.SecondaryResources = []string{"boards", "columns", "cards"}
		scope.RequiredTags = []string{"projects"}
	}

	r.logger.Debug("Extracted resource scope from tool name", map[string]interface{}{
		"tool_name":     toolName,
		"resource_type": resourceType,
		"path_patterns": scope.PathPatterns,
		"prefixes":      scope.OperationPrefixes,
	})

	return scope
}

// FilterOperationsByScope filters OpenAPI operations based on resource scope
// This is the CORE GENERIC FUNCTION that works with ANY OpenAPI spec
func (r *ResourceScopeResolver) FilterOperationsByScope(
	spec *openapi3.T,
	scope *ResourceScope,
) map[string]*openapi3.Operation {
	if scope == nil {
		// No scope defined, return all operations
		return r.getAllOperations(spec)
	}

	filtered := make(map[string]*openapi3.Operation)

	for path, pathItem := range spec.Paths.Map() {
		for method, operation := range pathItem.Operations() {
			if operation == nil {
				continue
			}

			// Check if this operation should be included
			if r.shouldIncludeOperation(operation, path, method, scope) {
				opID := operation.OperationID
				if opID == "" {
					opID = fmt.Sprintf("%s_%s", strings.ToLower(method), strings.ReplaceAll(path, "/", "_"))
				}
				filtered[opID] = operation
			}
		}
	}

	r.logger.Info("Filtered operations by resource scope", map[string]interface{}{
		"resource_type":    scope.ResourceType,
		"total_operations": r.countOperations(spec),
		"filtered_count":   len(filtered),
	})

	return filtered
}

// shouldIncludeOperation determines if an operation should be included based on scope
func (r *ResourceScopeResolver) shouldIncludeOperation(
	operation *openapi3.Operation,
	path string,
	method string,
	scope *ResourceScope,
) bool {
	opID := operation.OperationID

	// Check explicit excludes first (highest priority)
	for _, exclude := range scope.ExcludeOperations {
		if opID == exclude {
			return false
		}
	}

	// Check explicit includes (override other rules)
	for _, include := range scope.IncludeOperations {
		if opID == include {
			return true
		}
	}

	// Check path exclusions
	for _, pattern := range scope.ExcludePatterns {
		if r.matchesPattern(path, pattern) {
			return false
		}
	}

	// Score the operation based on multiple factors
	score := 0

	// Check operation ID prefixes
	for _, prefix := range scope.OperationPrefixes {
		if strings.HasPrefix(strings.ToLower(opID), prefix) {
			score += 10
			break
		}
	}

	// Check if operation ID contains resource type
	if strings.Contains(strings.ToLower(opID), scope.ResourceType) ||
		strings.Contains(strings.ToLower(opID), r.singularize(scope.ResourceType)) {
		score += 5
	}

	// Check path patterns
	for _, pattern := range scope.PathPatterns {
		if r.matchesPattern(path, pattern) {
			score += 8
			break
		}
	}

	// Check if path contains resource type
	if strings.Contains(path, "/"+scope.ResourceType) ||
		strings.Contains(path, "/"+r.singularize(scope.ResourceType)) {
		score += 5
	}

	// Check tags
	if operation.Tags != nil && len(scope.RequiredTags) > 0 {
		for _, tag := range operation.Tags {
			for _, required := range scope.RequiredTags {
				if strings.EqualFold(tag, required) {
					score += 7
					break
				}
			}
		}
	}

	// Check secondary resources
	for _, secondary := range scope.SecondaryResources {
		if strings.Contains(path, "/"+secondary) ||
			strings.Contains(strings.ToLower(opID), secondary) {
			score += 3
			break
		}
	}

	// Include if score is above threshold
	// This threshold can be tuned based on API characteristics
	threshold := 5
	return score >= threshold
}

// matchesPattern checks if a path matches a pattern (simple glob-like matching)
func (r *ResourceScopeResolver) matchesPattern(path, pattern string) bool {
	// Handle exact matches
	if path == pattern {
		return true
	}

	// Handle prefix matches
	if strings.HasSuffix(pattern, "/") && strings.HasPrefix(path, pattern) {
		return true
	}

	// Handle patterns with placeholders
	if strings.Contains(pattern, "{") {
		// Convert pattern to a simple regex-like check
		// e.g., "/repos/{owner}/{repo}/issues" matches "/repos/abc/def/issues"
		patternParts := strings.Split(pattern, "/")
		pathParts := strings.Split(path, "/")

		if len(patternParts) != len(pathParts) {
			return false
		}

		for i, part := range patternParts {
			if !strings.Contains(part, "{") && part != pathParts[i] {
				return false
			}
		}
		return true
	}

	// Handle contains matches for simple patterns
	return strings.Contains(path, pattern)
}

// singularize converts plural resource names to singular
// This is a simple implementation - can be enhanced with a proper inflection library
func (r *ResourceScopeResolver) singularize(plural string) string {
	if strings.HasSuffix(plural, "ies") {
		return strings.TrimSuffix(plural, "ies") + "y"
	}
	if strings.HasSuffix(plural, "es") {
		return strings.TrimSuffix(plural, "es")
	}
	if strings.HasSuffix(plural, "s") {
		return strings.TrimSuffix(plural, "s")
	}
	return plural
}

// getAllOperations returns all operations from the spec
func (r *ResourceScopeResolver) getAllOperations(spec *openapi3.T) map[string]*openapi3.Operation {
	operations := make(map[string]*openapi3.Operation)

	for path, pathItem := range spec.Paths.Map() {
		for method, operation := range pathItem.Operations() {
			if operation != nil {
				opID := operation.OperationID
				if opID == "" {
					opID = fmt.Sprintf("%s_%s", strings.ToLower(method), strings.ReplaceAll(path, "/", "_"))
				}
				operations[opID] = operation
			}
		}
	}

	return operations
}

// countOperations counts total operations in the spec
func (r *ResourceScopeResolver) countOperations(spec *openapi3.T) int {
	count := 0
	for _, pathItem := range spec.Paths.Map() {
		for _, operation := range pathItem.Operations() {
			if operation != nil {
				count++
			}
		}
	}
	return count
}

// GetSimplifiedActionName extracts a simple action name from an operation
// This helps AI agents use simple names like "list", "get", "create"
func (r *ResourceScopeResolver) GetSimplifiedActionName(operation *openapi3.Operation, resourceType string) string {
	opID := strings.ToLower(operation.OperationID)

	// Remove resource type prefix
	opID = strings.TrimPrefix(opID, resourceType+"/")
	opID = strings.TrimPrefix(opID, resourceType+"-")
	opID = strings.TrimPrefix(opID, r.singularize(resourceType)+"/")
	opID = strings.TrimPrefix(opID, r.singularize(resourceType)+"-")

	// Common action mappings (following REST conventions)
	actionMappings := map[string]string{
		"list":            "list",
		"get":             "get",
		"create":          "create",
		"update":          "update",
		"delete":          "delete",
		"search":          "search",
		"list-for-repo":   "list",
		"list-for-user":   "list",
		"list-for-org":    "list",
		"get-by-id":       "get",
		"create-for-repo": "create",
		"update-by-id":    "update",
		"delete-by-id":    "delete",
	}

	// Try to find a mapping
	for pattern, simple := range actionMappings {
		if strings.Contains(opID, pattern) {
			return simple
		}
	}

	// Return the cleaned operation ID
	return opID
}
