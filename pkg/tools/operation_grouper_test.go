package tools

import (
	"fmt"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperationGrouper_GroupOperations(t *testing.T) {
	tests := []struct {
		name           string
		spec           *openapi3.T
		strategy       GroupingStrategy
		expectedGroups []string
		minGroups      int
	}{
		{
			name: "group by tags",
			spec: &openapi3.T{
				OpenAPI: "3.0.0",
				Info: &openapi3.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: openapi3.NewPaths(
					openapi3.WithPath("/users", &openapi3.PathItem{
						Get: &openapi3.Operation{
							Tags:    []string{"users"},
							Summary: "List users",
						},
						Post: &openapi3.Operation{
							Tags:    []string{"users"},
							Summary: "Create user",
						},
					}),
					openapi3.WithPath("/repos", &openapi3.PathItem{
						Get: &openapi3.Operation{
							Tags:    []string{"repositories"},
							Summary: "List repositories",
						},
					}),
				),
			},
			strategy:       GroupByTags,
			expectedGroups: []string{"users", "repositories"},
			minGroups:      2,
		},
		{
			name: "group by paths when no tags",
			spec: &openapi3.T{
				OpenAPI: "3.0.0",
				Info: &openapi3.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: openapi3.NewPaths(
					openapi3.WithPath("/users", &openapi3.PathItem{
						Get: &openapi3.Operation{
							Summary: "List users",
						},
					}),
					openapi3.WithPath("/users/{id}", &openapi3.PathItem{
						Get: &openapi3.Operation{
							Summary: "Get user",
						},
					}),
					openapi3.WithPath("/repos", &openapi3.PathItem{
						Get: &openapi3.Operation{
							Summary: "List repositories",
						},
					}),
				),
			},
			strategy:       GroupByPaths,
			expectedGroups: []string{"user", "repo"},
			minGroups:      2,
		},
		{
			name: "hybrid grouping",
			spec: &openapi3.T{
				OpenAPI: "3.0.0",
				Info: &openapi3.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: openapi3.NewPaths(
					openapi3.WithPath("/users", &openapi3.PathItem{
						Get: &openapi3.Operation{
							Tags:    []string{"users"},
							Summary: "List users",
						},
					}),
					openapi3.WithPath("/repos", &openapi3.PathItem{
						Get: &openapi3.Operation{
							// No tags - should group by path
							Summary: "List repositories",
						},
					}),
				),
			},
			strategy:  GroupByHybrid,
			minGroups: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grouper := NewOperationGrouper()
			grouper.GroupingStrategy = tt.strategy

			groups, err := grouper.GroupOperations(tt.spec)

			assert.NoError(t, err)
			assert.GreaterOrEqual(t, len(groups), tt.minGroups)

			// Check expected groups exist
			for _, expectedGroup := range tt.expectedGroups {
				_, exists := groups[expectedGroup]
				assert.True(t, exists, "Expected group %s not found", expectedGroup)
			}

			// Verify each group has operations
			for name, group := range groups {
				assert.NotEmpty(t, group.Operations, "Group %s has no operations", name)
				assert.NotEmpty(t, group.Name)
				assert.NotEmpty(t, group.DisplayName)
			}
		})
	}
}

func TestOperationGrouper_HandleLargeSpec(t *testing.T) {
	// Create a spec with many operations
	spec := &openapi3.T{
		OpenAPI: "3.0.0",
		Info: &openapi3.Info{
			Title:   "Large API",
			Version: "1.0.0",
		},
		Paths: openapi3.NewPaths(),
	}

	// Add many operations with different tags
	tags := []string{"users", "repos", "issues", "pulls", "actions", "packages"}
	for _, tag := range tags {
		for j := 0; j < 20; j++ {
			path := fmt.Sprintf("/%s/operation%d", tag, j)
			spec.Paths.Set(path, &openapi3.PathItem{
				Get: &openapi3.Operation{
					Tags:        []string{tag},
					OperationID: fmt.Sprintf("%s_get_%d", tag, j),
					Summary:     fmt.Sprintf("Operation %d for %s", j, tag),
				},
			})
		}
	}

	grouper := NewOperationGrouper()
	grouper.MaxOperationsPerGroup = 50

	groups, err := grouper.GroupOperations(spec)

	require.NoError(t, err)
	assert.Len(t, groups, len(tags))

	// Verify each group respects the max operations limit
	for name, group := range groups {
		assert.LessOrEqual(t, len(group.Operations), grouper.MaxOperationsPerGroup,
			"Group %s exceeds max operations", name)
	}
}

func TestOperationGrouper_PriorityOrdering(t *testing.T) {
	spec := &openapi3.T{
		OpenAPI: "3.0.0",
		Info: &openapi3.Info{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Paths: openapi3.NewPaths(
			openapi3.WithPath("/auth/login", &openapi3.PathItem{
				Post: &openapi3.Operation{
					Tags:    []string{"authentication"},
					Summary: "Login",
				},
			}),
			openapi3.WithPath("/misc/info", &openapi3.PathItem{
				Get: &openapi3.Operation{
					Tags:    []string{"miscellaneous"},
					Summary: "Get info",
				},
			}),
			openapi3.WithPath("/users", &openapi3.PathItem{
				Get: &openapi3.Operation{
					Tags:    []string{"users"},
					Summary: "List users",
				},
			}),
		),
	}

	grouper := NewOperationGrouper()
	_, err := grouper.GroupOperations(spec)

	require.NoError(t, err)

	// Get sorted groups
	sortedGroups := grouper.GetSortedGroups()

	assert.NotEmpty(t, sortedGroups)

	// Authentication should have higher priority (lower number)
	var authPriority, miscPriority int
	for _, group := range sortedGroups {
		if group.Name == "authentication" {
			authPriority = group.Priority
		}
		if group.Name == "miscellaneous" {
			miscPriority = group.Priority
		}
	}

	assert.Less(t, authPriority, miscPriority,
		"Authentication should have higher priority than miscellaneous")
}

func TestOperationGrouper_NoOperations(t *testing.T) {
	spec := &openapi3.T{
		OpenAPI: "3.0.0",
		Info: &openapi3.Info{
			Title:   "Empty API",
			Version: "1.0.0",
		},
		Paths: openapi3.NewPaths(),
	}

	grouper := NewOperationGrouper()
	groups, err := grouper.GroupOperations(spec)

	assert.NoError(t, err)
	assert.Empty(t, groups)
}

func TestOperationGrouper_GeneralGroup(t *testing.T) {
	spec := &openapi3.T{
		OpenAPI: "3.0.0",
		Info: &openapi3.Info{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Paths: openapi3.NewPaths(
			// Operation without tags and unclear path
			openapi3.WithPath("/v1/{id}", &openapi3.PathItem{
				Get: &openapi3.Operation{
					Summary: "Get by ID",
				},
			}),
		),
	}

	grouper := NewOperationGrouper()
	grouper.GroupingStrategy = GroupByHybrid

	groups, err := grouper.GroupOperations(spec)

	assert.NoError(t, err)
	assert.NotEmpty(t, groups)

	// Should have a general group for unassigned operations
	generalGroup, exists := groups["general"]
	assert.True(t, exists, "General group should exist")
	assert.NotEmpty(t, generalGroup.Operations)
	assert.Equal(t, 999, generalGroup.Priority, "General group should have low priority")
}
