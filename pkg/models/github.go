package models

// GitHubQueryType defines the type of GitHub query
type GitHubQueryType string

const (
	// GitHubQueryTypeRepository represents a repository query
	GitHubQueryTypeRepository GitHubQueryType = "repository"

	// GitHubQueryTypePullRequests represents a pull requests query
	GitHubQueryTypePullRequests GitHubQueryType = "pull_requests"

	// GitHubQueryTypeIssues represents an issues query
	GitHubQueryTypeIssues GitHubQueryType = "issues"

	// GitHubQueryTypeCommits represents a commits query
	GitHubQueryTypeCommits GitHubQueryType = "commits"
)

// GitHubQuery represents a query to the GitHub API
type GitHubQuery struct {
	// Type specifies the type of query
	Type GitHubQueryType `json:"type"`

	// Owner is the repository owner (organization or user)
	Owner string `json:"owner"`

	// Repo is the repository name
	Repo string `json:"repo"`

	// State is used for filtering pull requests or issues
	State string `json:"state,omitempty"`

	// Branch is used for filtering commits or other branch-specific queries
	Branch string `json:"branch,omitempty"`

	// ID is used when querying a specific resource by ID
	ID string `json:"id,omitempty"`

	// Number is used when querying a specific pull request or issue by number
	Number int `json:"number,omitempty"`
}
