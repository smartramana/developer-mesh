package testdata

import (
	"time"

	"github.com/google/go-github/v53/github"
)

// CreateMockIssue creates a mock GitHub issue for testing
func CreateMockIssue(id int, title, body string) *github.Issue {
	return &github.Issue{
		ID:     github.Int64(int64(id)),
		Number: github.Int(id),
		Title:  github.String(title),
		Body:   github.String(body),
		State:  github.String("open"),
		CreatedAt: &github.Timestamp{
			Time: time.Now().Add(-24 * time.Hour),
		},
		UpdatedAt: &github.Timestamp{
			Time: time.Now(),
		},
	}
}

// CreateMockRepository creates a mock GitHub repository for testing
func CreateMockRepository(id int64, name, owner string) *github.Repository {
	return &github.Repository{
		ID:   github.Int64(id),
		Name: github.String(name),
		Owner: &github.User{
			Login: github.String(owner),
		},
		FullName: github.String(owner + "/" + name),
		Private:  github.Bool(false),
		CreatedAt: &github.Timestamp{
			Time: time.Now().Add(-720 * time.Hour),
		},
		UpdatedAt: &github.Timestamp{
			Time: time.Now(),
		},
	}
}

// CreateMockPullRequest creates a mock GitHub pull request for testing
func CreateMockPullRequest(id int, title, body, head, base string) *github.PullRequest {
	return &github.PullRequest{
		ID:     github.Int64(int64(id)),
		Number: github.Int(id),
		Title:  github.String(title),
		Body:   github.String(body),
		State:  github.String("open"),
		Head: &github.PullRequestBranch{
			Ref: github.String(head),
		},
		Base: &github.PullRequestBranch{
			Ref: github.String(base),
		},
		CreatedAt: &github.Timestamp{
			Time: time.Now().Add(-24 * time.Hour),
		},
		UpdatedAt: &github.Timestamp{
			Time: time.Now(),
		},
	}
}

// CreateMockIssueComment creates a mock GitHub issue comment for testing
func CreateMockIssueComment(id int64, body string) *github.IssueComment {
	return &github.IssueComment{
		ID:   github.Int64(id),
		Body: github.String(body),
		User: &github.User{
			Login: github.String("test-user"),
		},
		CreatedAt: &github.Timestamp{
			Time: time.Now().Add(-12 * time.Hour),
		},
		UpdatedAt: &github.Timestamp{
			Time: time.Now(),
		},
	}
}

// CreateMockPullRequestMergeResult creates a mock GitHub PR merge result for testing
func CreateMockPullRequestMergeResult(merged bool, message string) *github.PullRequestMergeResult {
	return &github.PullRequestMergeResult{
		Merged:  github.Bool(merged),
		Message: github.String(message),
		SHA:     github.String("abc123def456"),
	}
}

// CreateMockWebhookPayload creates a mock webhook payload for testing
func CreateMockWebhookPayload(eventType string, data interface{}) ([]byte, error) {
	// This would normally create proper webhook payloads
	// For now, we'll use a simple placeholder implementation
	// In a real implementation, this would create properly structured webhook payloads
	return []byte(`{"test": "data"}`), nil
}

// CreateMockResponse creates a mock HTTP response for testing
func CreateMockResponse(statusCode int) *github.Response {
	return &github.Response{
		Response: &github.ResponseHeader{
			StatusCode: statusCode,
		},
	}
}
