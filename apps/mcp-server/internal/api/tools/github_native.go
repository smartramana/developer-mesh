package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// GitHubNativeTool provides direct GitHub API access as an MCP tool
type GitHubNativeTool struct {
	client *http.Client
	token  string
}

// NewGitHubNativeTool creates a new GitHub tool
func NewGitHubNativeTool() *GitHubNativeTool {
	return &GitHubNativeTool{
		client: &http.Client{},
		token:  os.Getenv("GITHUB_TOKEN"),
	}
}

// GetToolDefinitions returns MCP tool definitions for GitHub operations
func (g *GitHubNativeTool) GetToolDefinitions() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "github_get_repo",
			"description": "Get GitHub repository information",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"owner": map[string]interface{}{
						"type":        "string",
						"description": "Repository owner",
					},
					"repo": map[string]interface{}{
						"type":        "string",
						"description": "Repository name",
					},
				},
				"required": []string{"owner", "repo"},
			},
		},
		{
			"name":        "github_create_issue",
			"description": "Create a new GitHub issue",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"owner": map[string]interface{}{
						"type":        "string",
						"description": "Repository owner",
					},
					"repo": map[string]interface{}{
						"type":        "string",
						"description": "Repository name",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Issue title",
					},
					"body": map[string]interface{}{
						"type":        "string",
						"description": "Issue body",
					},
				},
				"required": []string{"owner", "repo", "title"},
			},
		},
		{
			"name":        "github_list_issues",
			"description": "List GitHub issues for a repository",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"owner": map[string]interface{}{
						"type":        "string",
						"description": "Repository owner",
					},
					"repo": map[string]interface{}{
						"type":        "string",
						"description": "Repository name",
					},
					"state": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"open", "closed", "all"},
						"default":     "open",
						"description": "Issue state filter",
					},
				},
				"required": []string{"owner", "repo"},
			},
		},
	}
}

// ExecuteTool executes a GitHub tool
func (g *GitHubNativeTool) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	switch toolName {
	case "github_get_repo":
		return g.getRepo(ctx, args)
	case "github_create_issue":
		return g.createIssue(ctx, args)
	case "github_list_issues":
		return g.listIssues(ctx, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

func (g *GitHubNativeTool) getRepo(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)

	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Failed to close response body: %v\n", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API error: %s", string(body))
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (g *GitHubNativeTool) createIssue(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	title, _ := args["title"].(string)
	body, _ := args["body"].(string)

	if owner == "" || repo == "" || title == "" {
		return nil, fmt.Errorf("owner, repo, and title are required")
	}

	payload := map[string]string{
		"title": title,
		"body":  body,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", owner, repo)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}

	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Failed to close response body: %v\n", err)
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("GitHub API error: %s", string(respBody))
	}

	var result interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (g *GitHubNativeTool) listIssues(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	state, _ := args["state"].(string)

	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}

	if state == "" {
		state = "open"
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues?state=%s", owner, repo, state)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Failed to close response body: %v\n", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API error: %s", string(body))
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}
