package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Common GitHub API error types
var (
	// General GitHub errors
	ErrGitHubAPI          = errors.New("github api error")
	ErrInvalidCredentials = errors.New("invalid github credentials")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrResourceNotFound   = errors.New("resource not found")
	ErrRateLimitExceeded  = errors.New("rate limit exceeded")
	ErrServerError        = errors.New("github server error")
	ErrValidationFailed   = errors.New("validation failed")
	ErrServiceUnavailable = errors.New("github service unavailable")

	// REST API specific errors
	ErrRESTRequest  = errors.New("rest request failed")
	ErrRESTResponse = errors.New("invalid rest response")
	
	// GraphQL specific errors
	ErrGraphQLRequest  = errors.New("graphql request failed")
	ErrGraphQLResponse = errors.New("invalid graphql response")
	
	// Webhook specific errors
	ErrInvalidWebhook    = errors.New("invalid webhook")
	ErrInvalidSignature  = errors.New("invalid webhook signature")
	ErrInvalidPayload    = errors.New("invalid webhook payload")
	ErrDuplicateDelivery = errors.New("duplicate webhook delivery")
)

// GitHubError represents a GitHub API error with context
type GitHubError struct {
	// Original error
	Err error
	
	// HTTP status code (if applicable)
	StatusCode int
	
	// Response message (if any)
	Message string
	
	// Documentation URL
	DocumentationURL string
	
	// Context information
	Resource    string            // Resource being accessed (repo, issue, etc.)
	ResourceID  string            // ID of the resource
	Operation   string            // Operation being performed (GET, POST, etc.)
	RequestPath string            // API path that was requested
	Context     map[string]string // Additional context information
}

// Error returns the error message
func (e *GitHubError) Error() string {
	msg := fmt.Sprintf("%s", e.Err)
	
	if e.StatusCode > 0 {
		msg = fmt.Sprintf("%s (HTTP %d)", msg, e.StatusCode)
	}
	
	if e.Message != "" {
		msg = fmt.Sprintf("%s: %s", msg, e.Message)
	}
	
	if e.Resource != "" {
		if e.ResourceID != "" {
			msg = fmt.Sprintf("%s [%s: %s]", msg, e.Resource, e.ResourceID)
		} else {
			msg = fmt.Sprintf("%s [%s]", msg, e.Resource)
		}
	}
	
	return msg
}

// Unwrap returns the wrapped error
func (e *GitHubError) Unwrap() error {
	return e.Err
}

// Is checks if the target error matches this error
func (e *GitHubError) Is(target error) bool {
	return errors.Is(e.Err, target)
}

// NewGitHubError creates a new GitHub error
func NewGitHubError(err error, statusCode int, message string) *GitHubError {
	return &GitHubError{
		Err:        err,
		StatusCode: statusCode,
		Message:    message,
		Context:    make(map[string]string),
	}
}

// WithResource adds resource information to the error
func (e *GitHubError) WithResource(resource, id string) *GitHubError {
	e.Resource = resource
	e.ResourceID = id
	return e
}

// WithOperation adds operation information to the error
func (e *GitHubError) WithOperation(operation, path string) *GitHubError {
	e.Operation = operation
	e.RequestPath = path
	return e
}

// WithContext adds context information to the error
func (e *GitHubError) WithContext(key, value string) *GitHubError {
	e.Context[key] = value
	return e
}

// WithDocumentation adds documentation URL to the error
func (e *GitHubError) WithDocumentation(url string) *GitHubError {
	e.DocumentationURL = url
	return e
}

// FromHTTPError creates a GitHubError from an HTTP status code and message
func FromHTTPError(statusCode int, message, documentationURL string) *GitHubError {
	var baseErr error
	
	switch statusCode {
	case http.StatusUnauthorized:
		baseErr = ErrInvalidCredentials
	case http.StatusForbidden:
		if message != "" && (
			message == "API rate limit exceeded" || 
			message == "You have exceeded a secondary rate limit" ||
			message == "You have triggered an abuse detection mechanism") {
			baseErr = ErrRateLimitExceeded
		} else {
			baseErr = ErrPermissionDenied
		}
	case http.StatusNotFound:
		baseErr = ErrResourceNotFound
	case http.StatusUnprocessableEntity:
		baseErr = ErrValidationFailed
	case http.StatusInternalServerError:
		baseErr = ErrServerError
	case http.StatusServiceUnavailable:
		baseErr = ErrServiceUnavailable
	default:
		if statusCode >= 400 && statusCode < 500 {
			baseErr = ErrGitHubAPI
		} else if statusCode >= 500 {
			baseErr = ErrServerError
		} else {
			baseErr = ErrGitHubAPI
		}
	}
	
	err := NewGitHubError(baseErr, statusCode, message)
	if documentationURL != "" {
		err.WithDocumentation(documentationURL)
	}
	
	return err
}

// FromWebhookError creates a GitHubError from a webhook validation error
func FromWebhookError(err error, eventType string) *GitHubError {
	var baseErr error
	
	switch {
	case errors.Is(err, ErrInvalidSignature) || 
		 errors.Is(err, ErrInvalidWebhook) ||
		 errors.Is(err, ErrInvalidPayload):
		baseErr = err
	case errors.Is(err, ErrDuplicateDelivery):
		baseErr = ErrDuplicateDelivery
	default:
		baseErr = ErrInvalidWebhook
	}
	
	githubErr := NewGitHubError(baseErr, 0, err.Error())
	githubErr.WithResource("webhook", eventType)
	
	return githubErr
}

// IsGitHubRateLimitError checks if the error is a GitHub rate limit error
func IsGitHubRateLimitError(err error) bool {
	var githubErr *GitHubError
	if errors.As(err, &githubErr) {
		return errors.Is(githubErr.Err, ErrRateLimitExceeded)
	}
	return errors.Is(err, ErrRateLimitExceeded)
}

// IsGitHubNotFoundError checks if the error is a GitHub not found error
func IsGitHubNotFoundError(err error) bool {
	var githubErr *GitHubError
	if errors.As(err, &githubErr) {
		return errors.Is(githubErr.Err, ErrResourceNotFound)
	}
	return errors.Is(err, ErrResourceNotFound)
}

// IsGitHubAuthenticationError checks if the error is a GitHub authentication error
func IsGitHubAuthenticationError(err error) bool {
	var githubErr *GitHubError
	if errors.As(err, &githubErr) {
		return errors.Is(githubErr.Err, ErrInvalidCredentials)
	}
	return errors.Is(err, ErrInvalidCredentials)
}

// IsGitHubPermissionError checks if the error is a GitHub permission error
func IsGitHubPermissionError(err error) bool {
	var githubErr *GitHubError
	if errors.As(err, &githubErr) {
		return errors.Is(githubErr.Err, ErrPermissionDenied)
	}
	return errors.Is(err, ErrPermissionDenied)
}

// IsGitHubValidationError checks if the error is a GitHub validation error
func IsGitHubValidationError(err error) bool {
	var githubErr *GitHubError
	if errors.As(err, &githubErr) {
		return errors.Is(githubErr.Err, ErrValidationFailed)
	}
	return errors.Is(err, ErrValidationFailed)
}

// IsGitHubServerError checks if the error is a GitHub server error
func IsGitHubServerError(err error) bool {
	var githubErr *GitHubError
	if errors.As(err, &githubErr) {
		return errors.Is(githubErr.Err, ErrServerError) || 
			   errors.Is(githubErr.Err, ErrServiceUnavailable)
	}
	return errors.Is(err, ErrServerError) || 
		   errors.Is(err, ErrServiceUnavailable)
}
