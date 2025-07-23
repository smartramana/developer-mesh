// Package github provides GitHub tools for the MCP application
package github

import (
	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/core/tool"
)

// Issue tools

func (p *GitHubToolProvider) getIssueTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "get_issue",
			Description: "Get details of a GitHub issue",
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
					"issue_number": {
						Type:        "number",
						Description: "Issue number",
					},
				},
				Required: []string{"owner", "repo", "issue_number"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("get_issue", params)
		},
	}
}

func (p *GitHubToolProvider) listIssuesTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "list_issues",
			Description: "List issues in a GitHub repository",
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
					"state": {
						Type:        "string",
						Description: "Issue state (open, closed, all)",
						Enum:        []string{"open", "closed", "all"},
					},
				},
				Required: []string{"owner", "repo"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("list_issues", params)
		},
	}
}

func (p *GitHubToolProvider) createIssueTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "create_issue",
			Description: "Create a new issue in a GitHub repository",
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
					"title": {
						Type:        "string",
						Description: "Issue title",
					},
					"body": {
						Type:        "string",
						Description: "Issue body",
					},
				},
				Required: []string{"owner", "repo", "title"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("create_issue", params)
		},
	}
}

func (p *GitHubToolProvider) updateIssueTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "update_issue",
			Description: "Update an issue in a GitHub repository",
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
					"issue_number": {
						Type:        "number",
						Description: "Issue number",
					},
					"title": {
						Type:        "string",
						Description: "New issue title",
					},
					"body": {
						Type:        "string",
						Description: "New issue body",
					},
					"state": {
						Type:        "string",
						Description: "New issue state (open, closed)",
						Enum:        []string{"open", "closed"},
					},
				},
				Required: []string{"owner", "repo", "issue_number"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("update_issue", params)
		},
	}
}

func (p *GitHubToolProvider) addIssueCommentTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "add_issue_comment",
			Description: "Add a comment to an existing issue",
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
					"issue_number": {
						Type:        "number",
						Description: "Issue number",
					},
					"body": {
						Type:        "string",
						Description: "Comment text",
					},
				},
				Required: []string{"owner", "repo", "issue_number", "body"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("add_issue_comment", params)
		},
	}
}

// Pull request tools

func (p *GitHubToolProvider) getPullRequestTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "get_pull_request",
			Description: "Get details of a pull request",
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
			return p.executeAction("get_pull_request", params)
		},
	}
}

func (p *GitHubToolProvider) listPullRequestsTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "list_pull_requests",
			Description: "List pull requests in a repository",
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
					"state": {
						Type:        "string",
						Description: "Pull request state (open, closed, all)",
						Enum:        []string{"open", "closed", "all"},
					},
				},
				Required: []string{"owner", "repo"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("list_pull_requests", params)
		},
	}
}

func (p *GitHubToolProvider) createPullRequestTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "create_pull_request",
			Description: "Create a new pull request",
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
					"title": {
						Type:        "string",
						Description: "Pull request title",
					},
					"body": {
						Type:        "string",
						Description: "Pull request description",
					},
					"head": {
						Type:        "string",
						Description: "The name of the branch where your changes are implemented",
					},
					"base": {
						Type:        "string",
						Description: "The name of the branch you want the changes pulled into",
					},
				},
				Required: []string{"owner", "repo", "title", "head", "base"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("create_pull_request", params)
		},
	}
}

func (p *GitHubToolProvider) mergePullRequestTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "merge_pull_request",
			Description: "Merge a pull request",
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
					"merge_method": {
						Type:        "string",
						Description: "Merge method to use",
						Enum:        []string{"merge", "squash", "rebase"},
					},
				},
				Required: []string{"owner", "repo", "pull_number"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("merge_pull_request", params)
		},
	}
}
