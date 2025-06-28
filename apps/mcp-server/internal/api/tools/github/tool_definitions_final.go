// Package github provides GitHub tools for the MCP application
package github

import (
	"github.com/S-Corkum/devops-mcp/apps/mcp-server/internal/core/tool"
)

// Additional pull request tools

func (p *GitHubToolProvider) getPullRequestFilesTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "get_pull_request_files",
			Description: "Get the files changed in a pull request",
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
					"pull_number": {
						Type:        "number",
						Description: "Pull request number",
					},
				},
				Required: []string{"owner", "repo", "pull_number"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("get_pull_request_files", params)
		},
	}
}

func (p *GitHubToolProvider) getPullRequestCommentsTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "get_pull_request_comments",
			Description: "Get comments on a pull request",
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
					"pull_number": {
						Type:        "number",
						Description: "Pull request number",
					},
				},
				Required: []string{"owner", "repo", "pull_number"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("get_pull_request_comments", params)
		},
	}
}

// Branch tools

func (p *GitHubToolProvider) createBranchTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "create_branch",
			Description: "Create a new branch in a repository",
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
					"branch": {
						Type:        "string",
						Description: "Name for the new branch",
					},
					"from_branch": {
						Type:        "string",
						Description: "Source branch to create from (defaults to the repository's default branch)",
					},
				},
				Required: []string{"owner", "repo", "branch"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("create_branch", params)
		},
	}
}

// File tools

func (p *GitHubToolProvider) getFileContentsTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "get_file_contents",
			Description: "Get the contents of a file from a repository",
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
					"path": {
						Type:        "string",
						Description: "Path to the file",
					},
					"branch": {
						Type:        "string",
						Description: "Branch to get contents from (defaults to the repository's default branch)",
					},
				},
				Required: []string{"owner", "repo", "path"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("get_file_contents", params)
		},
	}
}

func (p *GitHubToolProvider) createOrUpdateFileTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "create_or_update_file",
			Description: "Create or update a file in a repository",
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
					"path": {
						Type:        "string",
						Description: "Path where to create/update the file",
					},
					"content": {
						Type:        "string",
						Description: "Content of the file",
					},
					"message": {
						Type:        "string",
						Description: "Commit message",
					},
					"branch": {
						Type:        "string",
						Description: "Branch to create/update the file in",
					},
				},
				Required: []string{"owner", "repo", "path", "content", "message"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("create_or_update_file", params)
		},
	}
}

func (p *GitHubToolProvider) pushFilesTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "push_files",
			Description: "Push multiple files to a repository in a single commit",
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
					"branch": {
						Type:        "string",
						Description: "Branch to push to (e.g., 'main' or 'master')",
					},
					"message": {
						Type:        "string",
						Description: "Commit message",
					},
					"files": {
						Type:        "array",
						Description: "Array of files to push",
						Items: &tool.PropertySchema{
							Type: "object",
							Properties: map[string]tool.PropertySchema{
								"path": {
									Type:        "string",
									Description: "Path for the file",
								},
								"content": {
									Type:        "string",
									Description: "Content for the file",
								},
							},
						},
					},
				},
				Required: []string{"owner", "repo", "branch", "message", "files"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("push_files", params)
		},
	}
}

// Search tools

func (p *GitHubToolProvider) searchCodeTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "search_code",
			Description: "Search for code across GitHub repositories",
			Parameters: tool.ParameterSchema{
				Type: "object",
				Properties: map[string]tool.PropertySchema{
					"q": {
						Type:        "string",
						Description: "Search query",
					},
				},
				Required: []string{"q"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("search_code", params)
		},
	}
}

func (p *GitHubToolProvider) searchIssuesTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "search_issues",
			Description: "Search for issues and pull requests across GitHub repositories",
			Parameters: tool.ParameterSchema{
				Type: "object",
				Properties: map[string]tool.PropertySchema{
					"q": {
						Type:        "string",
						Description: "Search query",
					},
				},
				Required: []string{"q"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("search_issues", params)
		},
	}
}

func (p *GitHubToolProvider) searchRepositoriesTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "search_repositories",
			Description: "Search for GitHub repositories",
			Parameters: tool.ParameterSchema{
				Type: "object",
				Properties: map[string]tool.PropertySchema{
					"query": {
						Type:        "string",
						Description: "Search query",
					},
				},
				Required: []string{"query"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("search_repositories", params)
		},
	}
}

func (p *GitHubToolProvider) searchUsersTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "search_users",
			Description: "Search for GitHub users",
			Parameters: tool.ParameterSchema{
				Type: "object",
				Properties: map[string]tool.PropertySchema{
					"q": {
						Type:        "string",
						Description: "Search query",
					},
				},
				Required: []string{"q"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("search_users", params)
		},
	}
}

// Commit tools

func (p *GitHubToolProvider) listCommitsTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "list_commits",
			Description: "List commits in a repository",
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
					"sha": {
						Type:        "string",
						Description: "SHA or branch to list commits from",
					},
				},
				Required: []string{"owner", "repo"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("list_commits", params)
		},
	}
}

// Fork tool

func (p *GitHubToolProvider) forkRepositoryTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "fork_repository",
			Description: "Fork a repository to your account",
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
			return p.executeAction("fork_repository", params)
		},
	}
}
