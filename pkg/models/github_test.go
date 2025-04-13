package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitHubQueryTypes(t *testing.T) {
	// Test GitHubQueryType constants
	assert.Equal(t, GitHubQueryType("repository"), GitHubQueryTypeRepository)
	assert.Equal(t, GitHubQueryType("pull_requests"), GitHubQueryTypePullRequests)
	assert.Equal(t, GitHubQueryType("issues"), GitHubQueryTypeIssues)
	assert.Equal(t, GitHubQueryType("commits"), GitHubQueryTypeCommits)
}

func TestGitHubQuery(t *testing.T) {
	// Test creating and accessing GitHubQuery
	query := GitHubQuery{
		Type:   GitHubQueryTypeRepository,
		Owner:  "test-owner",
		Repo:   "test-repo",
		State:  "open",
		Branch: "main",
		ID:     "123456",
		Number: 42,
	}
	
	// Verify fields
	assert.Equal(t, GitHubQueryTypeRepository, query.Type)
	assert.Equal(t, "test-owner", query.Owner)
	assert.Equal(t, "test-repo", query.Repo)
	assert.Equal(t, "open", query.State)
	assert.Equal(t, "main", query.Branch)
	assert.Equal(t, "123456", query.ID)
	assert.Equal(t, 42, query.Number)
}

func TestGitHubQueryRepositoryType(t *testing.T) {
	// Test using repository query type
	query := GitHubQuery{
		Type:  GitHubQueryTypeRepository,
		Owner: "test-owner",
		Repo:  "test-repo",
	}
	
	assert.Equal(t, GitHubQueryTypeRepository, query.Type)
	assert.Equal(t, "test-owner", query.Owner)
	assert.Equal(t, "test-repo", query.Repo)
}

func TestGitHubQueryPullRequestsType(t *testing.T) {
	// Test using pull requests query type
	query := GitHubQuery{
		Type:   GitHubQueryTypePullRequests,
		Owner:  "test-owner",
		Repo:   "test-repo",
		State:  "open",
		Branch: "main",
	}
	
	assert.Equal(t, GitHubQueryTypePullRequests, query.Type)
	assert.Equal(t, "test-owner", query.Owner)
	assert.Equal(t, "test-repo", query.Repo)
	assert.Equal(t, "open", query.State)
	assert.Equal(t, "main", query.Branch)
}

func TestGitHubQueryIssuesType(t *testing.T) {
	// Test using issues query type
	query := GitHubQuery{
		Type:   GitHubQueryTypeIssues,
		Owner:  "test-owner",
		Repo:   "test-repo",
		State:  "closed",
		Number: 123,
	}
	
	assert.Equal(t, GitHubQueryTypeIssues, query.Type)
	assert.Equal(t, "test-owner", query.Owner)
	assert.Equal(t, "test-repo", query.Repo)
	assert.Equal(t, "closed", query.State)
	assert.Equal(t, 123, query.Number)
}

func TestGitHubQueryCommitsType(t *testing.T) {
	// Test using commits query type
	query := GitHubQuery{
		Type:   GitHubQueryTypeCommits,
		Owner:  "test-owner",
		Repo:   "test-repo",
		Branch: "feature/branch",
	}
	
	assert.Equal(t, GitHubQueryTypeCommits, query.Type)
	assert.Equal(t, "test-owner", query.Owner)
	assert.Equal(t, "test-repo", query.Repo)
	assert.Equal(t, "feature/branch", query.Branch)
}
