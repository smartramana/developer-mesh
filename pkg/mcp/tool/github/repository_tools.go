package github

import (
	"github.com/S-Corkum/devops-mcp/pkg/mcp/tool"
)

// getRepositoryTool returns a tool for getting a GitHub repository
func (p *GitHubToolProvider) getRepositoryTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_get_repository",
			Description: "This is a tool from the github MCP server.\nGet details of a GitHub repository",
			Parameters: tool.ParameterSchema{
				Type: "object",
				Properties: map[string]tool.PropertySchema{
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
				},
				Required: []string{"owner", "repo"},
			},
			Tags: []string{"github", "repository"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("getRepository", params)
		},
	}
}

// listRepositoriesTool returns a tool for listing GitHub repositories
func (p *GitHubToolProvider) listRepositoriesTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_list_repositories",
			Description: "This is a tool from the github MCP server.\nList repositories for a user or organization on GitHub",
			Parameters: tool.ParameterSchema{
				Type: "object",
				Properties: map[string]tool.PropertySchema{
					"username": {
						Type:        "string",
						Description: "GitHub username to list repositories for (leave empty for authenticated user)",
					},
					"type": {
						Type:        "string",
						Description: "Type of repositories to list",
						Enum:        []string{"all", "owner", "public", "private", "member"},
					},
					"sort": {
						Type:        "string",
						Description: "How to sort the repositories",
						Enum:        []string{"created", "updated", "pushed", "full_name"},
					},
					"direction": {
						Type:        "string",
						Description: "Direction of sort",
						Enum:        []string{"asc", "desc"},
					},
					"page": {
						Type:        "integer",
						Description: "Page number (1-based)",
					},
					"per_page": {
						Type:        "integer",
						Description: "Results per page (max 100)",
					},
				},
			},
			Tags: []string{"github", "repository"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("listRepositories", params)
		},
	}
}

// createRepositoryTool returns a tool for creating a GitHub repository
func (p *GitHubToolProvider) createRepositoryTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_create_repository",
			Description: "This is a tool from the github MCP server.\nCreate a new GitHub repository in your account",
			Parameters: tool.ParameterSchema{
				Type: "object",
				Properties: map[string]tool.PropertySchema{
					"name": {
						Type:        "string",
						Description: "Repository name",
					},
					"description": {
						Type:        "string",
						Description: "Repository description",
					},
					"private": {
						Type:        "boolean",
						Description: "Whether the repository should be private",
					},
					"autoInit": {
						Type:        "boolean",
						Description: "Initialize with README.md",
					},
				},
				Required: []string{"name"},
			},
			Tags: []string{"github", "repository"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("createRepository", params)
		},
	}
}

// updateRepositoryTool returns a tool for updating a GitHub repository
func (p *GitHubToolProvider) updateRepositoryTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_update_repository",
			Description: "This is a tool from the github MCP server.\nUpdate an existing GitHub repository",
			Parameters: tool.ParameterSchema{
				Type: "object",
				Properties: map[string]tool.PropertySchema{
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"name": {
						Type:        "string",
						Description: "New repository name",
					},
					"description": {
						Type:        "string",
						Description: "New repository description",
					},
					"private": {
						Type:        "boolean",
						Description: "Whether the repository should be private",
					},
					"default_branch": {
						Type:        "string",
						Description: "The default branch",
					},
					"has_issues": {
						Type:        "boolean",
						Description: "Enable issues feature",
					},
					"has_projects": {
						Type:        "boolean",
						Description: "Enable projects feature",
					},
					"has_wiki": {
						Type:        "boolean",
						Description: "Enable wiki feature",
					},
					"allow_squash_merge": {
						Type:        "boolean",
						Description: "Allow squash merging",
					},
					"allow_merge_commit": {
						Type:        "boolean",
						Description: "Allow merge commits",
					},
					"allow_rebase_merge": {
						Type:        "boolean",
						Description: "Allow rebase merging",
					},
				},
				Required: []string{"owner", "repo"},
			},
			Tags: []string{"github", "repository"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("updateRepository", params)
		},
	}
}

// deleteRepositoryTool returns a tool for deleting a GitHub repository
func (p *GitHubToolProvider) deleteRepositoryTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_delete_repository",
			Description: "This is a tool from the github MCP server.\nDelete a GitHub repository",
			Parameters: tool.ParameterSchema{
				Type: "object",
				Properties: map[string]tool.PropertySchema{
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
				},
				Required: []string{"owner", "repo"},
			},
			Tags: []string{"github", "repository"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("deleteRepository", params)
		},
	}
}

// forkRepositoryTool returns a tool for forking a GitHub repository
func (p *GitHubToolProvider) forkRepositoryTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_fork_repository",
			Description: "This is a tool from the github MCP server.\nFork a GitHub repository to your account or specified organization",
			Parameters: tool.ParameterSchema{
				Type: "object",
				Properties: map[string]tool.PropertySchema{
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"organization": {
						Type:        "string",
						Description: "Optional: organization to fork to (defaults to your personal account)",
					},
				},
				Required: []string{"owner", "repo"},
			},
			Tags: []string{"github", "repository"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("forkRepository", params)
		},
	}
}
