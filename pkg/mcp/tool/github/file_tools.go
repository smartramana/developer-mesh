package github

import (
	"github.com/S-Corkum/devops-mcp/pkg/mcp/tool"
)

// createBranchTool returns a tool for creating a branch in a GitHub repository
func (p *GitHubToolProvider) createBranchTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_create_branch",
			Description: "This is a tool from the github MCP server.\nCreate a new branch in a GitHub repository",
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
						Description: "Optional: source branch to create from (defaults to the repository's default branch)",
					},
				},
				Required: []string{"owner", "repo", "branch"},
			},
			Tags: []string{"github", "branch"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("createBranch", params)
		},
	}
}

// getFileContentsTool returns a tool for getting file contents from a GitHub repository
func (p *GitHubToolProvider) getFileContentsTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_get_file_contents",
			Description: "This is a tool from the github MCP server.\nGet the contents of a file or directory from a GitHub repository",
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
						Description: "Path to the file or directory",
					},
					"branch": {
						Type:        "string",
						Description: "Branch to get contents from",
					},
				},
				Required: []string{"owner", "repo", "path"},
			},
			Tags: []string{"github", "file"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("getFileContents", params)
		},
	}
}

// createOrUpdateFileTool returns a tool for creating or updating a file in a GitHub repository
func (p *GitHubToolProvider) createOrUpdateFileTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_create_or_update_file",
			Description: "This is a tool from the github MCP server.\nCreate or update a single file in a GitHub repository",
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
					"message": {
						Type:        "string",
						Description: "Commit message",
					},
					"content": {
						Type:        "string",
						Description: "Content of the file",
					},
					"branch": {
						Type:        "string",
						Description: "Branch to create/update the file in",
					},
					"sha": {
						Type:        "string",
						Description: "SHA of the file being replaced (required when updating existing files)",
					},
				},
				Required: []string{"owner", "repo", "path", "message", "content"},
			},
			Tags: []string{"github", "file"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("createOrUpdateFile", params)
		},
	}
}

// pushFilesTool returns a tool for pushing multiple files to a GitHub repository
func (p *GitHubToolProvider) pushFilesTool() *tool.Tool {
	return &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "mcp0_push_files",
			Description: "This is a tool from the github MCP server.\nPush multiple files to a GitHub repository in a single commit",
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
									Type: "string",
								},
								"content": {
									Type: "string",
								},
							},
						},
					},
				},
				Required: []string{"owner", "repo", "branch", "message", "files"},
			},
			Tags: []string{"github", "file"},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return p.executeAction("pushFiles", params)
		},
	}
}
