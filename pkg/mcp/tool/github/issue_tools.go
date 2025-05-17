package github

import (
	"github.com/S-Corkum/devops-mcp/pkg/mcp/tool"
)

// getIssueTool returns a tool for getting a GitHub issue
func (p *GitHubToolProvider) getIssueTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_get_issue",
			Description: "This is a tool from the github MCP server.\nGet details of a specific issue in a GitHub repository.",
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
			Tags: []string{"github", "issue"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("getIssue", params)
		},
	}
}

// listIssuesTool returns a tool for listing GitHub issues
func (p *GitHubToolProvider) listIssuesTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_list_issues",
			Description: "This is a tool from the github MCP server.\nList issues in a GitHub repository with filtering options",
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
						Description: "Issue state to filter by",
						Enum:        []string{"open", "closed", "all"},
					},
					"sort": {
						Type:        "string",
						Description: "Sorting criteria",
						Enum:        []string{"created", "updated", "comments"},
					},
					"direction": {
						Type:        "string",
						Description: "Sort direction",
						Enum:        []string{"asc", "desc"},
					},
					"since": {
						Type:        "string",
						Description: "Only issues updated at or after this time (ISO 8601 format)",
					},
					"labels": {
						Type:        "array",
						Description: "Filter by label names",
						Items: &tool.PropertySchema{
							Type: "string",
						},
					},
					"page": {
						Type:        "number",
						Description: "Page number for pagination",
					},
					"per_page": {
						Type:        "number",
						Description: "Number of results per page",
					},
				},
				Required: []string{"owner", "repo"},
			},
			Tags: []string{"github", "issue"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("listIssues", params)
		},
	}
}

// createIssueTool returns a tool for creating a GitHub issue
func (p *GitHubToolProvider) createIssueTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_create_issue",
			Description: "This is a tool from the github MCP server.\nCreate a new issue in a GitHub repository",
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
						Description: "Issue body content",
					},
					"assignees": {
						Type:        "array",
						Description: "GitHub usernames to assign to the issue",
						Items: &tool.PropertySchema{
							Type: "string",
						},
					},
					"milestone": {
						Type:        "number",
						Description: "Milestone ID to associate with this issue",
					},
					"labels": {
						Type:        "array",
						Description: "Labels to associate with this issue",
						Items: &tool.PropertySchema{
							Type: "string",
						},
					},
				},
				Required: []string{"owner", "repo", "title"},
			},
			Tags: []string{"github", "issue"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("createIssue", params)
		},
	}
}

// updateIssueTool returns a tool for updating a GitHub issue
func (p *GitHubToolProvider) updateIssueTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_update_issue",
			Description: "This is a tool from the github MCP server.\nUpdate an existing issue in a GitHub repository",
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
						Description: "New issue body content",
					},
					"state": {
						Type:        "string",
						Description: "Issue state",
						Enum:        []string{"open", "closed"},
					},
					"assignees": {
						Type:        "array",
						Description: "GitHub usernames to assign to the issue",
						Items: &tool.PropertySchema{
							Type: "string",
						},
					},
					"milestone": {
						Type:        "number",
						Description: "Milestone ID to associate with this issue",
					},
					"labels": {
						Type:        "array",
						Description: "Labels to associate with this issue",
						Items: &tool.PropertySchema{
							Type: "string",
						},
					},
				},
				Required: []string{"owner", "repo", "issue_number"},
			},
			Tags: []string{"github", "issue"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("updateIssue", params)
		},
	}
}

// addIssueCommentTool returns a tool for adding a comment to a GitHub issue
func (p *GitHubToolProvider) addIssueCommentTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_add_issue_comment",
			Description: "This is a tool from the github MCP server.\nAdd a comment to an existing issue",
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
						Description: "Comment body content",
					},
				},
				Required: []string{"owner", "repo", "issue_number", "body"},
			},
			Tags: []string{"github", "issue", "comment"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("addIssueComment", params)
		},
	}
}
