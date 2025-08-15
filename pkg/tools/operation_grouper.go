package tools

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// OperationGroup represents a group of related operations
type OperationGroup struct {
	Name        string                       // Group name (e.g., "repos", "issues", "users")
	DisplayName string                       // Human-friendly name (e.g., "Repository Management")
	Description string                       // Group description
	Operations  map[string]*GroupedOperation // Operations in this group
	Tags        []string                     // Tags associated with this group
	Priority    int                          // Priority for ordering groups
}

// GroupedOperation represents an operation within a group
type GroupedOperation struct {
	OperationID string
	Method      string
	Path        string
	Operation   *openapi3.Operation
	PathItem    *openapi3.PathItem
}

// OperationGrouper handles the grouping of OpenAPI operations
type OperationGrouper struct {
	// Configuration
	MaxOperationsPerGroup int
	MinOperationsPerGroup int
	GroupingStrategy      GroupingStrategy

	// Internal state
	spec       *openapi3.T
	groups     map[string]*OperationGroup
	unassigned []*GroupedOperation
}

// GroupingStrategy defines how operations should be grouped
type GroupingStrategy string

const (
	// GroupByTags groups operations by their OpenAPI tags
	GroupByTags GroupingStrategy = "tags"

	// GroupByPaths groups operations by path segments
	GroupByPaths GroupingStrategy = "paths"

	// GroupByResources groups operations by resource type
	GroupByResources GroupingStrategy = "resources"

	// GroupByHybrid uses tags first, then falls back to paths
	GroupByHybrid GroupingStrategy = "hybrid"
)

// NewOperationGrouper creates a new operation grouper
func NewOperationGrouper() *OperationGrouper {
	return &OperationGrouper{
		MaxOperationsPerGroup: 100, // Reasonable limit per group
		MinOperationsPerGroup: 1,   // Allow single-operation groups
		GroupingStrategy:      GroupByHybrid,
		groups:                make(map[string]*OperationGroup),
		unassigned:            make([]*GroupedOperation, 0),
	}
}

// GroupOperations groups operations from an OpenAPI spec
func (g *OperationGrouper) GroupOperations(spec *openapi3.T) (map[string]*OperationGroup, error) {
	if spec == nil {
		return nil, fmt.Errorf("OpenAPI spec is nil")
	}

	g.spec = spec
	g.groups = make(map[string]*OperationGroup)
	g.unassigned = make([]*GroupedOperation, 0)

	// Extract all operations from the spec
	operations := g.extractOperations(spec)

	// Group operations based on strategy
	switch g.GroupingStrategy {
	case GroupByTags:
		g.groupByTags(operations)
	case GroupByPaths:
		g.groupByPaths(operations)
	case GroupByResources:
		g.groupByResources(operations)
	case GroupByHybrid:
		g.groupByHybrid(operations)
	default:
		g.groupByHybrid(operations)
	}

	// Handle unassigned operations
	if len(g.unassigned) > 0 {
		g.handleUnassignedOperations()
	}

	// Post-process groups
	g.postProcessGroups()

	return g.groups, nil
}

// extractOperations extracts all operations from the spec
func (g *OperationGrouper) extractOperations(spec *openapi3.T) []*GroupedOperation {
	operations := make([]*GroupedOperation, 0)

	for path, pathItem := range spec.Paths.Map() {
		if pathItem == nil {
			continue
		}

		for method, operation := range pathItem.Operations() {
			if operation == nil {
				continue
			}

			// Generate operation ID if missing
			operationID := operation.OperationID
			if operationID == "" {
				operationID = g.generateOperationID(method, path)
			}

			operations = append(operations, &GroupedOperation{
				OperationID: operationID,
				Method:      method,
				Path:        path,
				Operation:   operation,
				PathItem:    pathItem,
			})
		}
	}

	return operations
}

// groupByTags groups operations by their OpenAPI tags
func (g *OperationGrouper) groupByTags(operations []*GroupedOperation) {
	for _, op := range operations {
		if len(op.Operation.Tags) == 0 {
			g.unassigned = append(g.unassigned, op)
			continue
		}

		// Use the first tag as the primary group
		primaryTag := op.Operation.Tags[0]
		groupName := g.normalizeGroupName(primaryTag)

		// Get or create group
		group, exists := g.groups[groupName]
		if !exists {
			group = &OperationGroup{
				Name:        groupName,
				DisplayName: g.generateDisplayName(primaryTag),
				Description: fmt.Sprintf("Operations related to %s", primaryTag),
				Operations:  make(map[string]*GroupedOperation),
				Tags:        []string{primaryTag},
			}
			g.groups[groupName] = group
		}

		// Add operation to group if not at limit
		if len(group.Operations) < g.MaxOperationsPerGroup {
			group.Operations[op.OperationID] = op

			// Add additional tags
			for _, tag := range op.Operation.Tags[1:] {
				if !g.containsString(group.Tags, tag) {
					group.Tags = append(group.Tags, tag)
				}
			}
		} else {
			g.unassigned = append(g.unassigned, op)
		}
	}
}

// groupByPaths groups operations by path segments
func (g *OperationGrouper) groupByPaths(operations []*GroupedOperation) {
	for _, op := range operations {
		resource := g.extractResourceFromPath(op.Path)
		if resource == "" {
			g.unassigned = append(g.unassigned, op)
			continue
		}

		groupName := g.normalizeGroupName(resource)

		// Get or create group
		group, exists := g.groups[groupName]
		if !exists {
			group = &OperationGroup{
				Name:        groupName,
				DisplayName: g.generateDisplayName(resource),
				Description: fmt.Sprintf("Operations for %s resources", resource),
				Operations:  make(map[string]*GroupedOperation),
				Tags:        []string{},
			}
			g.groups[groupName] = group
		}

		// Add operation to group if not at limit
		if len(group.Operations) < g.MaxOperationsPerGroup {
			group.Operations[op.OperationID] = op
		} else {
			g.unassigned = append(g.unassigned, op)
		}
	}
}

// groupByResources groups operations by resource type
func (g *OperationGrouper) groupByResources(operations []*GroupedOperation) {
	// Analyze paths to identify resource patterns
	resourcePatterns := g.identifyResourcePatterns(operations)

	for _, op := range operations {
		assigned := false

		// Try to match against resource patterns
		for resource, pattern := range resourcePatterns {
			if pattern.MatchString(op.Path) {
				groupName := g.normalizeGroupName(resource)

				// Get or create group
				group, exists := g.groups[groupName]
				if !exists {
					group = &OperationGroup{
						Name:        groupName,
						DisplayName: g.generateDisplayName(resource),
						Description: fmt.Sprintf("Operations for %s", resource),
						Operations:  make(map[string]*GroupedOperation),
						Tags:        []string{},
					}
					g.groups[groupName] = group
				}

				// Add operation to group if not at limit
				if len(group.Operations) < g.MaxOperationsPerGroup {
					group.Operations[op.OperationID] = op
					assigned = true
					break
				}
			}
		}

		if !assigned {
			g.unassigned = append(g.unassigned, op)
		}
	}
}

// groupByHybrid uses tags first, then falls back to paths
func (g *OperationGrouper) groupByHybrid(operations []*GroupedOperation) {
	// First pass: group by tags
	tagged := make([]*GroupedOperation, 0)
	untagged := make([]*GroupedOperation, 0)

	for _, op := range operations {
		if len(op.Operation.Tags) > 0 {
			tagged = append(tagged, op)
		} else {
			untagged = append(untagged, op)
		}
	}

	// Group tagged operations
	g.groupByTags(tagged)

	// Second pass: group untagged operations by paths
	for _, op := range untagged {
		resource := g.extractResourceFromPath(op.Path)
		if resource == "" {
			g.unassigned = append(g.unassigned, op)
			continue
		}

		groupName := g.normalizeGroupName(resource)

		// Try to find existing group or create new one
		group, exists := g.groups[groupName]
		if !exists {
			group = &OperationGroup{
				Name:        groupName,
				DisplayName: g.generateDisplayName(resource),
				Description: fmt.Sprintf("Operations for %s", resource),
				Operations:  make(map[string]*GroupedOperation),
				Tags:        []string{},
			}
			g.groups[groupName] = group
		}

		// Add operation to group if not at limit
		if len(group.Operations) < g.MaxOperationsPerGroup {
			group.Operations[op.OperationID] = op
		} else {
			g.unassigned = append(g.unassigned, op)
		}
	}
}

// handleUnassignedOperations handles operations that couldn't be grouped
func (g *OperationGrouper) handleUnassignedOperations() {
	if len(g.unassigned) == 0 {
		return
	}

	// Create a general/misc group for unassigned operations
	generalGroup := &OperationGroup{
		Name:        "general",
		DisplayName: "General Operations",
		Description: "Miscellaneous operations",
		Operations:  make(map[string]*GroupedOperation),
		Tags:        []string{},
		Priority:    999, // Low priority
	}

	for _, op := range g.unassigned {
		if len(generalGroup.Operations) < g.MaxOperationsPerGroup {
			generalGroup.Operations[op.OperationID] = op
		}
	}

	if len(generalGroup.Operations) > 0 {
		g.groups["general"] = generalGroup
	}
}

// postProcessGroups performs post-processing on groups
func (g *OperationGrouper) postProcessGroups() {
	// Remove groups that are too small if configured
	if g.MinOperationsPerGroup > 1 {
		smallGroups := make([]string, 0)
		orphanedOps := make([]*GroupedOperation, 0)

		for name, group := range g.groups {
			if len(group.Operations) < g.MinOperationsPerGroup {
				smallGroups = append(smallGroups, name)
				for _, op := range group.Operations {
					orphanedOps = append(orphanedOps, op)
				}
			}
		}

		// Remove small groups
		for _, name := range smallGroups {
			delete(g.groups, name)
		}

		// Reassign orphaned operations to general group
		if len(orphanedOps) > 0 {
			generalGroup, exists := g.groups["general"]
			if !exists {
				generalGroup = &OperationGroup{
					Name:        "general",
					DisplayName: "General Operations",
					Description: "Miscellaneous operations",
					Operations:  make(map[string]*GroupedOperation),
					Tags:        []string{},
					Priority:    999,
				}
				g.groups["general"] = generalGroup
			}

			for _, op := range orphanedOps {
				if len(generalGroup.Operations) < g.MaxOperationsPerGroup {
					generalGroup.Operations[op.OperationID] = op
				}
			}
		}
	}

	// Set priorities based on operation count
	g.setPriorities()
}

// setPriorities sets priorities for groups based on importance
func (g *OperationGrouper) setPriorities() {
	// Common high-priority group names
	highPriority := []string{"auth", "authentication", "users", "account", "core"}
	mediumPriority := []string{"repos", "repositories", "projects", "issues", "tickets"}

	for name, group := range g.groups {
		// Check for high priority
		for _, hp := range highPriority {
			if strings.Contains(strings.ToLower(name), hp) {
				group.Priority = 1
				break
			}
		}

		// Check for medium priority
		if group.Priority == 0 {
			for _, mp := range mediumPriority {
				if strings.Contains(strings.ToLower(name), mp) {
					group.Priority = 5
					break
				}
			}
		}

		// Default priority based on operation count
		if group.Priority == 0 {
			if len(group.Operations) > 20 {
				group.Priority = 10
			} else if len(group.Operations) > 10 {
				group.Priority = 20
			} else {
				group.Priority = 50
			}
		}
	}
}

// extractResourceFromPath extracts the resource name from a path
func (g *OperationGrouper) extractResourceFromPath(path string) string {
	// Remove leading slash and split
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")

	// Look for the first meaningful segment
	for _, part := range parts {
		// Skip version indicators and parameters
		if strings.HasPrefix(part, "v") && len(part) <= 3 {
			continue
		}
		if strings.HasPrefix(part, "{") {
			continue
		}
		if part == "api" {
			continue
		}

		// Return the first meaningful part
		if len(part) > 2 {
			// Remove trailing 's' for plural to singular
			if strings.HasSuffix(part, "s") && len(part) > 3 {
				return strings.TrimSuffix(part, "s")
			}
			return part
		}
	}

	return ""
}

// identifyResourcePatterns identifies common resource patterns in paths
func (g *OperationGrouper) identifyResourcePatterns(operations []*GroupedOperation) map[string]*regexp.Regexp {
	patterns := make(map[string]*regexp.Regexp)

	// Count resource occurrences
	resourceCounts := make(map[string]int)
	for _, op := range operations {
		resource := g.extractResourceFromPath(op.Path)
		if resource != "" {
			resourceCounts[resource]++
		}
	}

	// Create patterns for common resources
	for resource, count := range resourceCounts {
		if count >= 2 { // At least 2 operations for a pattern
			// Create a pattern that matches the resource in the path
			pattern := fmt.Sprintf(`(?i)/%s(?:/|$|\{)`, regexp.QuoteMeta(resource))
			if re, err := regexp.Compile(pattern); err == nil {
				patterns[resource] = re
			}
		}
	}

	return patterns
}

// normalizeGroupName normalizes a name for use as a group identifier
func (g *OperationGrouper) normalizeGroupName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace spaces and special characters with underscores
	re := regexp.MustCompile(`[^a-z0-9]+`)
	name = re.ReplaceAllString(name, "_")

	// Remove leading/trailing underscores
	name = strings.Trim(name, "_")

	// Ensure it's not empty
	if name == "" {
		name = "general"
	}

	return name
}

// generateDisplayName generates a human-friendly display name
func (g *OperationGrouper) generateDisplayName(name string) string {
	// Split by underscores or hyphens
	parts := regexp.MustCompile(`[_-]`).Split(name, -1)

	// Capitalize each part
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}

	return strings.Join(parts, " ")
}

// generateOperationID generates an operation ID from method and path
func (g *OperationGrouper) generateOperationID(method, path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	cleanParts := []string{strings.ToLower(method)}

	for _, part := range parts {
		// Skip parameters and version indicators
		if strings.HasPrefix(part, "{") || part == "v1" || part == "v2" || part == "api" {
			continue
		}
		cleanParts = append(cleanParts, part)
	}

	return strings.Join(cleanParts, "_")
}

// containsString checks if a slice contains a string
func (g *OperationGrouper) containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// GetSortedGroups returns groups sorted by priority and name
func (g *OperationGrouper) GetSortedGroups() []*OperationGroup {
	groups := make([]*OperationGroup, 0, len(g.groups))
	for _, group := range g.groups {
		groups = append(groups, group)
	}

	// Sort by priority, then by name
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Priority != groups[j].Priority {
			return groups[i].Priority < groups[j].Priority
		}
		return groups[i].Name < groups[j].Name
	})

	return groups
}
