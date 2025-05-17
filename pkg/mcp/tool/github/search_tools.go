package github

import (
	"github.com/S-Corkum/devops-mcp/pkg/mcp/tool"
)

// searchCodeTool returns a tool for searching code in GitHub repositories
func (p *GitHubToolProvider) searchCodeTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_search_code",
			Description: "This is a tool from the github MCP server.\nSearch for code across GitHub repositories",
			Parameters: tool.ParameterSchema{
				Type: "object",
				Properties: map[string]tool.PropertySchema{
					"q": {
						Type:        "string",
						Description: "Search query using GitHub's search syntax",
					},
					"order": {
						Type:        "string",
						Description: "Order of results",
						Enum:        []string{"asc", "desc"},
					},
					"page": {
						Type:        "number",
						Description: "Page number for pagination (1-based)",
						Default:     1,
					},
					"per_page": {
						Type:        "number",
						Description: "Results per page (1-100)",
						Default:     30,
					},
				},
				Required: []string{"q"},
			},
			Tags: []string{"github", "search"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("searchCode", params)
		},
	}
}

// searchIssuesTool returns a tool for searching issues and pull requests in GitHub
func (p *GitHubToolProvider) searchIssuesTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_search_issues",
			Description: "This is a tool from the github MCP server.\nSearch for issues and pull requests across GitHub repositories",
			Parameters: tool.ParameterSchema{
				Type: "object",
				Properties: map[string]tool.PropertySchema{
					"q": {
						Type:        "string",
						Description: "Search query using GitHub's search syntax",
					},
					"sort": {
						Type:        "string",
						Description: "Sort field",
						Enum:        []string{"comments", "reactions", "reactions-+1", "reactions--1", "reactions-smile", "reactions-thinking_face", "reactions-heart", "reactions-tada", "interactions", "created", "updated"},
					},
					"order": {
						Type:        "string",
						Description: "Order of results",
						Enum:        []string{"asc", "desc"},
					},
					"page": {
						Type:        "number",
						Description: "Page number for pagination (1-based)",
						Default:     1,
					},
					"per_page": {
						Type:        "number",
						Description: "Results per page (1-100)",
						Default:     30,
					},
				},
				Required: []string{"q"},
			},
			Tags: []string{"github", "search"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("searchIssues", params)
		},
	}
}

// searchRepositoriesTool returns a tool for searching GitHub repositories
func (p *GitHubToolProvider) searchRepositoriesTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_search_repositories",
			Description: "This is a tool from the github MCP server.\nSearch for GitHub repositories",
			Parameters: tool.ParameterSchema{
				Type: "object",
				Properties: map[string]tool.PropertySchema{
					"query": {
						Type:        "string",
						Description: "Search query (see GitHub search syntax)",
					},
					"page": {
						Type:        "number",
						Description: "Page number for pagination (default: 1)",
						Default:     1,
					},
					"perPage": {
						Type:        "number",
						Description: "Number of results per page (default: 30, max: 100)",
						Default:     30,
					},
				},
				Required: []string{"query"},
			},
			Tags: []string{"github", "search"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("searchRepositories", params)
		},
	}
}

// searchUsersTool returns a tool for searching GitHub users
func (p *GitHubToolProvider) searchUsersTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_search_users",
			Description: "This is a tool from the github MCP server.\nSearch for users on GitHub",
			Parameters: tool.ParameterSchema{
				Type: "object",
				Properties: map[string]tool.PropertySchema{
					"q": {
						Type:        "string",
						Description: "Search query using GitHub's search syntax",
					},
					"sort": {
						Type:        "string",
						Description: "Sort field",
						Enum:        []string{"followers", "repositories", "joined"},
					},
					"order": {
						Type:        "string",
						Description: "Order of results",
						Enum:        []string{"asc", "desc"},
					},
					"page": {
						Type:        "number",
						Description: "Page number for pagination (1-based)",
						Default:     1,
					},
					"per_page": {
						Type:        "number",
						Description: "Results per page (1-100)",
						Default:     30,
					},
				},
				Required: []string{"q"},
			},
			Tags: []string{"github", "search"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("searchUsers", params)
		},
	}
}
