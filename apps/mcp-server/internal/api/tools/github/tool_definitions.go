// Package github provides GitHub tools for the MCP application
package github

import (
	"mcp-server/internal/core/tool"
)

// Repository tools

func (p *GitHubToolProvider) getRepositoryTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "get_repository",
			Description: "Get details of a GitHub repository",
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
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("get_repository", params)
		},
	}
}

func (p *GitHubToolProvider) listRepositoriesTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "list_repositories",
			Description: "List repositories for a user or organization",
			Parameters: tool.ParameterSchema{
				Type: "object",
				Properties: map[string]tool.PropertySchema{
					"username": {
						Type:        "string",
						Description: "GitHub username or organization name",
					},
				},
				Required: []string{"username"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("list_repositories", params)
		},
	}
}

func (p *GitHubToolProvider) createRepositoryTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "create_repository",
			Description: "Create a new GitHub repository",
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
				},
				Required: []string{"name"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("create_repository", params)
		},
	}
}

func (p *GitHubToolProvider) updateRepositoryTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "update_repository",
			Description: "Update a GitHub repository",
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
				},
				Required: []string{"owner", "repo"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("update_repository", params)
		},
	}
}

func (p *GitHubToolProvider) deleteRepositoryTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "delete_repository",
			Description: "Delete a GitHub repository",
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
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("delete_repository", params)
		},
	}
}
