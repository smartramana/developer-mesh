package github

import (
	"context"
	"fmt"

	"github.com/S-Corkum/devops-mcp/pkg/adapters/github"
	"github.com/S-Corkum/devops-mcp/pkg/mcp/tool"
)

// GitHubToolProvider provides GitHub tools for the MCP
type GitHubToolProvider struct {
	adapter *github.GitHubAdapter
}

// NewGitHubToolProvider creates a new GitHubToolProvider
func NewGitHubToolProvider(adapter *github.GitHubAdapter) *GitHubToolProvider {
	return &GitHubToolProvider{
		adapter: adapter,
	}
}

// RegisterTools registers all GitHub tools with the registry
func (p *GitHubToolProvider) RegisterTools(registry *tool.ToolRegistry) error {
	// Register repository tools
	if err := registry.RegisterTool(p.getRepositoryTool()); err != nil {
		return err
	}
	if err := registry.RegisterTool(p.listRepositoriesTool()); err != nil {
		return err
	}
	if err := registry.RegisterTool(p.createRepositoryTool()); err != nil {
		return err
	}
	if err := registry.RegisterTool(p.updateRepositoryTool()); err != nil {
		return err
	}
	if err := registry.RegisterTool(p.deleteRepositoryTool()); err != nil {
		return err
	}
	
	// Register issue tools
	if err := registry.RegisterTool(p.getIssueTool()); err != nil {
		return err
	}
	if err := registry.RegisterTool(p.listIssuesTool()); err != nil {
		return err
	}
	if err := registry.RegisterTool(p.createIssueTool()); err != nil {
		return err
	}
	if err := registry.RegisterTool(p.updateIssueTool()); err != nil {
		return err
	}
	if err := registry.RegisterTool(p.addIssueCommentTool()); err != nil {
		return err
	}
	
	// Register pull request tools
	if err := registry.RegisterTool(p.getPullRequestTool()); err != nil {
		return err
	}
	if err := registry.RegisterTool(p.listPullRequestsTool()); err != nil {
		return err
	}
	if err := registry.RegisterTool(p.createPullRequestTool()); err != nil {
		return err
	}
	if err := registry.RegisterTool(p.mergePullRequestTool()); err != nil {
		return err
	}
	if err := registry.RegisterTool(p.getPullRequestFilesTool()); err != nil {
		return err
	}
	if err := registry.RegisterTool(p.getPullRequestCommentsTool()); err != nil {
		return err
	}
	
	// Register branch tools
	if err := registry.RegisterTool(p.createBranchTool()); err != nil {
		return err
	}
	
	// Register file tools
	if err := registry.RegisterTool(p.getFileContentsTool()); err != nil {
		return err
	}
	if err := registry.RegisterTool(p.createOrUpdateFileTool()); err != nil {
		return err
	}
	if err := registry.RegisterTool(p.pushFilesTool()); err != nil {
		return err
	}
	
	// Register search tools
	if err := registry.RegisterTool(p.searchCodeTool()); err != nil {
		return err
	}
	if err := registry.RegisterTool(p.searchIssuesTool()); err != nil {
		return err
	}
	if err := registry.RegisterTool(p.searchRepositoriesTool()); err != nil {
		return err
	}
	if err := registry.RegisterTool(p.searchUsersTool()); err != nil {
		return err
	}
	
	// Register commit tools
	if err := registry.RegisterTool(p.listCommitsTool()); err != nil {
		return err
	}
	
	// Register fork tool
	if err := registry.RegisterTool(p.forkRepositoryTool()); err != nil {
		return err
	}
	
	return nil
}

// executeAction is a helper function to execute GitHub adapter actions
func (p *GitHubToolProvider) executeAction(action string, params map[string]interface{}) (interface{}, error) {
	// Create context for the operation
	ctx := context.Background()
	
	// Default contextID if not provided
	contextID := "default"
	if cidValue, exists := params["_context_id"]; exists {
		if cid, ok := cidValue.(string); ok && cid != "" {
			contextID = cid
		}
		// Remove internal parameter before passing to adapter
		delete(params, "_context_id")
	}
	
	// Execute the action through the adapter
	result, err := p.adapter.ExecuteAction(ctx, contextID, action, params)
	if err != nil {
		return nil, fmt.Errorf("GitHub action '%s' failed: %w", action, err)
	}
	
	return result, nil
}

// Tool definitions follow below
// Each method returns a fully configured tool with definition and handler
