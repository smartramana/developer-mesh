package github

import (
	"github.com/S-Corkum/devops-mcp/pkg/mcp/tool"
)

// getPullRequestTool returns a tool for getting a GitHub pull request
func (p *GitHubToolProvider) getPullRequestTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_get_pull_request",
			Description: "This is a tool from the github MCP server.\nGet details of a specific pull request",
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
			Tags: []string{"github", "pullrequest"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("getPullRequest", params)
		},
	}
}

// listPullRequestsTool returns a tool for listing GitHub pull requests
func (p *GitHubToolProvider) listPullRequestsTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_list_pull_requests",
			Description: "This is a tool from the github MCP server.\nList and filter repository pull requests",
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
						Description: "State of the pull requests to return",
						Enum:        []string{"open", "closed", "all"},
					},
					"head": {
						Type:        "string",
						Description: "Filter by head user or head organization and branch name",
					},
					"base": {
						Type:        "string",
						Description: "Filter by base branch name",
					},
					"sort": {
						Type:        "string",
						Description: "What to sort results by",
						Enum:        []string{"created", "updated", "popularity", "long-running"},
					},
					"direction": {
						Type:        "string",
						Description: "The direction of the sort",
						Enum:        []string{"asc", "desc"},
					},
					"page": {
						Type:        "number",
						Description: "Page number of the results",
					},
					"per_page": {
						Type:        "number",
						Description: "Results per page (max 100)",
					},
				},
				Required: []string{"owner", "repo"},
			},
			Tags: []string{"github", "pullrequest"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("listPullRequests", params)
		},
	}
}

// createPullRequestTool returns a tool for creating a GitHub pull request
func (p *GitHubToolProvider) createPullRequestTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_create_pull_request",
			Description: "This is a tool from the github MCP server.\nCreate a new pull request in a GitHub repository",
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
					"head": {
						Type:        "string",
						Description: "The name of the branch where your changes are implemented",
					},
					"base": {
						Type:        "string",
						Description: "The name of the branch you want the changes pulled into",
					},
					"body": {
						Type:        "string",
						Description: "Pull request body/description",
					},
					"draft": {
						Type:        "boolean",
						Description: "Whether to create the pull request as a draft",
					},
					"maintainer_can_modify": {
						Type:        "boolean",
						Description: "Whether maintainers can modify the pull request",
					},
				},
				Required: []string{"owner", "repo", "title", "head", "base"},
			},
			Tags: []string{"github", "pullrequest"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("createPullRequest", params)
		},
	}
}

// mergePullRequestTool returns a tool for merging a GitHub pull request
func (p *GitHubToolProvider) mergePullRequestTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_merge_pull_request",
			Description: "This is a tool from the github MCP server.\nMerge a pull request",
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
					"commit_title": {
						Type:        "string",
						Description: "Title for the automatic commit message",
					},
					"commit_message": {
						Type:        "string",
						Description: "Extra detail to append to automatic commit message",
					},
				},
				Required: []string{"owner", "repo", "pull_number"},
			},
			Tags: []string{"github", "pullrequest"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("mergePullRequest", params)
		},
	}
}

// getPullRequestFilesTool returns a tool for getting files changed in a pull request
func (p *GitHubToolProvider) getPullRequestFilesTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_get_pull_request_files",
			Description: "This is a tool from the github MCP server.\nGet the list of files changed in a pull request",
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
			Tags: []string{"github", "pullrequest"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("getPullRequestFiles", params)
		},
	}
}

// getPullRequestCommentsTool returns a tool for getting comments on a pull request
func (p *GitHubToolProvider) getPullRequestCommentsTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_get_pull_request_comments",
			Description: "This is a tool from the github MCP server.\nGet the review comments on a pull request",
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
			Tags: []string{"github", "pullrequest", "comment"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("getPullRequestComments", params)
		},
	}
}

// getPullRequestReviewsTool returns a tool for getting reviews on a pull request
func (p *GitHubToolProvider) getPullRequestReviewsTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_get_pull_request_reviews",
			Description: "This is a tool from the github MCP server.\nGet the reviews on a pull request",
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
			Tags: []string{"github", "pullrequest", "review"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("getPullRequestReviews", params)
		},
	}
}

// createPullRequestReviewTool returns a tool for creating a review on a pull request
func (p *GitHubToolProvider) createPullRequestReviewTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_create_pull_request_review",
			Description: "This is a tool from the github MCP server.\nCreate a review on a pull request",
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
					"event": {
						Type:        "string",
						Description: "The review action to perform",
						Enum:        []string{"APPROVE", "REQUEST_CHANGES", "COMMENT"},
					},
					"body": {
						Type:        "string",
						Description: "The body text of the review",
					},
					"commit_id": {
						Type:        "string",
						Description: "The SHA of the commit that needs a review",
					},
					"comments": {
						Type:        "array",
						Description: "Comments to post as part of the review (specify either position or line, not both)",
						Items: &tool.PropertySchema{
							Type: "object",
							Properties: map[string]tool.PropertySchema{
								"path": {
									Type:        "string",
									Description: "The relative path to the file being commented on",
								},
								"position": {
									Type:        "number",
									Description: "The position in the diff where you want to add a review comment",
								},
								"line": {
									Type:        "number",
									Description: "The line number in the file where you want to add a review comment",
								},
								"body": {
									Type:        "string",
									Description: "Text of the review comment",
								},
							},
						},
					},
				},
				Required: []string{"owner", "repo", "pull_number", "event"},
			},
			Tags: []string{"github", "pullrequest", "review"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("createPullRequestReview", params)
		},
	}
}

// getPullRequestStatusTool returns a tool for getting the status of a pull request
func (p *GitHubToolProvider) getPullRequestStatusTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_get_pull_request_status",
			Description: "This is a tool from the github MCP server.\nGet the combined status of all status checks for a pull request",
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
			Tags: []string{"github", "pullrequest", "status"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("getPullRequestStatus", params)
		},
	}
}

// updatePullRequestBranchTool returns a tool for updating a pull request branch
func (p *GitHubToolProvider) updatePullRequestBranchTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_update_pull_request_branch",
			Description: "This is a tool from the github MCP server.\nUpdate a pull request branch with the latest changes from the base branch",
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
					"expected_head_sha": {
						Type:        "string",
						Description: "The expected SHA of the pull request's HEAD ref",
					},
				},
				Required: []string{"owner", "repo", "pull_number"},
			},
			Tags: []string{"github", "pullrequest"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("updatePullRequestBranch", params)
		},
	}
}
