package github

import (
	"github.com/S-Corkum/devops-mcp/pkg/mcp/tool"
)

// listCommitsTool returns a tool for listing commits in a GitHub repository
func (p *GitHubToolProvider) listCommitsTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_list_commits",
			Description: "This is a tool from the github MCP server.\nGet list of commits of a branch in a GitHub repository",
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
						Description: "Branch name or commit SHA to get commits from",
					},
					"page": {
						Type:        "number",
						Description: "Page number for pagination",
					},
					"perPage": {
						Type:        "number",
						Description: "Number of results per page",
					},
				},
				Required: []string{"owner", "repo"},
			},
			Tags: []string{"github", "commit"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("listCommits", params)
		},
	}
}
