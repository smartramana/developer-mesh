// Package testdata provides test fixtures and helpers for GitHub adapter tests.
// These fixtures help create consistent test data across different test files.
package testdata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/go-github/v53/github"
)

// Default values for test data
const (
	DefaultTestUser  = "test-user"
	DefaultTestOwner = "test-owner"
	DefaultTestRepo  = "test-repo"
)

// CreateMockIssue creates a mock GitHub issue for testing.
// It includes common issue fields with reasonable defaults.
//
// Parameters:
//   - id: Issue number/ID
//   - title: Issue title
//   - body: Issue description (optional)
//
// Returns:
//   - Mock GitHub Issue suitable for testing
func CreateMockIssue(id int, title, body string) *github.Issue {
	now := time.Now()
	createdAt := now.Add(-24 * time.Hour)
	
	return &github.Issue{
		ID:     github.Int64(int64(id)),
		Number: github.Int(id),
		Title:  github.String(title),
		Body:   github.String(body),
		State:  github.String("open"),
		User: &github.User{
			Login: github.String(DefaultTestUser),
			ID:    github.Int64(1),
		},
		Labels: []*github.Label{
			{
				Name:  github.String("bug"),
				Color: github.String("fc2929"),
			},
		},
		URL:       github.String(fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d", DefaultTestOwner, DefaultTestRepo, id)),
		HTMLURL:   github.String(fmt.Sprintf("https://github.com/%s/%s/issues/%d", DefaultTestOwner, DefaultTestRepo, id)),
		CreatedAt: &github.Timestamp{Time: createdAt},
		UpdatedAt: &github.Timestamp{Time: now},
	}
}

// CreateMockRepository creates a mock GitHub repository for testing.
// It includes common repository fields with reasonable defaults.
//
// Parameters:
//   - id: Repository ID
//   - name: Repository name
//   - owner: Repository owner login
//
// Returns:
//   - Mock GitHub Repository suitable for testing
func CreateMockRepository(id int64, name, owner string) *github.Repository {
	now := time.Now()
	createdAt := now.Add(-720 * time.Hour) // 30 days ago
	
	return &github.Repository{
		ID:   github.Int64(id),
		Name: github.String(name),
		Owner: &github.User{
			Login: github.String(owner),
			ID:    github.Int64(1),
		},
		FullName:    github.String(owner + "/" + name),
		Description: github.String("Test repository for " + name),
		Private:     github.Bool(false),
		Fork:        github.Bool(false),
		HTMLURL:     github.String("https://github.com/" + owner + "/" + name),
		CloneURL:    github.String("https://github.com/" + owner + "/" + name + ".git"),
		Language:    github.String("Go"),
		DefaultBranch: github.String("main"),
		CreatedAt:   &github.Timestamp{Time: createdAt},
		UpdatedAt:   &github.Timestamp{Time: now},
		PushedAt:    &github.Timestamp{Time: now.Add(-48 * time.Hour)},
	}
}

// CreateMockPullRequest creates a mock GitHub pull request for testing.
// It includes common pull request fields with reasonable defaults.
//
// Parameters:
//   - id: Pull request number/ID
//   - title: Pull request title
//   - body: Pull request description
//   - head: Head branch name
//   - base: Base branch name
//
// Returns:
//   - Mock GitHub PullRequest suitable for testing
func CreateMockPullRequest(id int, title, body, head, base string) *github.PullRequest {
	now := time.Now()
	createdAt := now.Add(-24 * time.Hour)
	
	return &github.PullRequest{
		ID:     github.Int64(int64(id)),
		Number: github.Int(id),
		Title:  github.String(title),
		Body:   github.String(body),
		State:  github.String("open"),
		User: &github.User{
			Login: github.String(DefaultTestUser),
			ID:    github.Int64(1),
		},
		Head: &github.PullRequestBranch{
			Ref:  github.String(head),
			SHA:  github.String("abc123def456"),
			User: &github.User{Login: github.String(DefaultTestUser)},
			Repo: &github.Repository{
				Name:  github.String(DefaultTestRepo),
				Owner: &github.User{Login: github.String(DefaultTestUser)},
			},
		},
		Base: &github.PullRequestBranch{
			Ref:  github.String(base),
			SHA:  github.String("789012ghi345"),
			User: &github.User{Login: github.String(DefaultTestUser)},
			Repo: &github.Repository{
				Name:  github.String(DefaultTestRepo),
				Owner: &github.User{Login: github.String(DefaultTestUser)},
			},
		},
		Merged:    github.Bool(false),
		Mergeable: github.Bool(true),
		URL:       github.String(fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", DefaultTestOwner, DefaultTestRepo, id)),
		HTMLURL:   github.String(fmt.Sprintf("https://github.com/%s/%s/pull/%d", DefaultTestOwner, DefaultTestRepo, id)),
		CreatedAt: &github.Timestamp{Time: createdAt},
		UpdatedAt: &github.Timestamp{Time: now},
	}
}

// CreateMockIssueComment creates a mock GitHub issue comment for testing.
// It includes common comment fields with reasonable defaults.
//
// Parameters:
//   - id: Comment ID
//   - body: Comment text content
//
// Returns:
//   - Mock GitHub IssueComment suitable for testing
func CreateMockIssueComment(id int64, body string) *github.IssueComment {
	now := time.Now()
	createdAt := now.Add(-12 * time.Hour)
	
	return &github.IssueComment{
		ID:   github.Int64(id),
		Body: github.String(body),
		User: &github.User{
			Login: github.String(DefaultTestUser),
			ID:    github.Int64(1),
		},
		URL:       github.String(fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/comments/%d", DefaultTestOwner, DefaultTestRepo, id)),
		HTMLURL:   github.String(fmt.Sprintf("https://github.com/%s/%s/issues/1#issuecomment-%d", DefaultTestOwner, DefaultTestRepo, id)),
		CreatedAt: &github.Timestamp{Time: createdAt},
		UpdatedAt: &github.Timestamp{Time: now},
	}
}

// CreateMockPullRequestMergeResult creates a mock GitHub PR merge result for testing.
// It simulates the result of a pull request merge operation.
//
// Parameters:
//   - merged: Whether the merge was successful
//   - message: Message returned with the merge result
//
// Returns:
//   - Mock GitHub PullRequestMergeResult suitable for testing
func CreateMockPullRequestMergeResult(merged bool, message string) *github.PullRequestMergeResult {
	return &github.PullRequestMergeResult{
		Merged:  github.Bool(merged),
		Message: github.String(message),
		SHA:     github.String("abc123def456"),
	}
}

// CreateMockWebhookPayload creates a mock webhook payload for testing.
// It generates a properly formatted webhook payload JSON for the given event type.
//
// Parameters:
//   - eventType: Type of GitHub event (e.g., "issues", "pull_request")
//   - data: Data to include in the payload
//
// Returns:
//   - Byte array containing the JSON payload
//   - Error if serialization fails
func CreateMockWebhookPayload(eventType string, data interface{}) ([]byte, error) {
	var payload interface{}
	
	switch eventType {
	case "issues":
		// Create an issue event payload
		if issue, ok := data.(*github.Issue); ok {
			payload = map[string]interface{}{
				"action": "opened",
				"issue":  issue,
				"repository": map[string]interface{}{
					"full_name": DefaultTestOwner + "/" + DefaultTestRepo,
					"owner": map[string]string{
						"login": DefaultTestOwner,
					},
				},
				"sender": map[string]string{
					"login": DefaultTestUser,
				},
			}
		} else {
			payload = data
		}
	case "pull_request":
		// Create a pull request event payload
		if pr, ok := data.(*github.PullRequest); ok {
			payload = map[string]interface{}{
				"action":       "opened",
				"pull_request": pr,
				"repository": map[string]interface{}{
					"full_name": DefaultTestOwner + "/" + DefaultTestRepo,
					"owner": map[string]string{
						"login": DefaultTestOwner,
					},
				},
				"sender": map[string]string{
					"login": DefaultTestUser,
				},
			}
		} else {
			payload = data
		}
	default:
		// For other event types, use the data as-is
		payload = data
	}
	
	return json.Marshal(payload)
}

// CreateMockResponse creates a mock HTTP response for testing.
// It simulates an HTTP response from the GitHub API.
//
// Parameters:
//   - statusCode: HTTP status code to include in the response
//
// Returns:
//   - Mock GitHub Response suitable for testing
func CreateMockResponse(statusCode int) *github.Response {
	return &github.Response{
		Response: &http.Response{
			StatusCode: statusCode,
			Header:     http.Header{},
		},
	}
}

// CreateMockRateLimitError creates a mock rate limit error
func CreateMockRateLimitError() *github.RateLimitError {
	return &github.RateLimitError{
		Rate: github.Rate{
			Limit:     5000,
			Remaining: 0,
			Reset:     github.Timestamp{Time: time.Now().Add(1 * time.Hour)},
		},
		Message: "API rate limit exceeded",
	}
}

// CreateMockErrorResponse creates a mock error response with a specific status code.
// It simulates a GitHub API error response.
//
// Parameters:
//   - statusCode: HTTP status code
//   - message: Error message
//
// Returns:
//   - Mock GitHub ErrorResponse suitable for testing
func CreateMockErrorResponse(statusCode int, message string) *github.ErrorResponse {
	return &github.ErrorResponse{
		Response: &http.Response{
			StatusCode: statusCode,
			Header:     http.Header{},
		},
		Message: message,
		Errors: []github.Error{
			{
				Resource: "issue",
				Field:    "title",
				Code:     "missing_field",
				Message:  message,
			},
		},
	}
}
