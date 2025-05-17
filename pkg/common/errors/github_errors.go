package errors

// GitHub error checking functions

// IsGitHubRateLimitError checks if the error is a GitHub rate limit error
func IsGitHubRateLimitError(err error) bool {
	if githubErr, ok := err.(*GitHubError); ok {
		return githubErr.Code == "rate_limit_exceeded" || githubErr.Status == 429
	}
	if adapterErr, ok := err.(*AdapterError); ok {
		return adapterErr.ErrorCode == "RATE_LIMIT_EXCEEDED" || adapterErr.ErrorType == ErrorTypeLimitExceeded
	}
	return false
}

// IsGitHubNotFoundError checks if the error is a GitHub not found error
func IsGitHubNotFoundError(err error) bool {
	if githubErr, ok := err.(*GitHubError); ok {
		return githubErr.Code == "not_found" || githubErr.Status == 404
	}
	if adapterErr, ok := err.(*AdapterError); ok {
		if adapterErr.ErrorType == ErrorTypeNotFound {
			return true
		}
		
		// Also check status code if available
		if statusCode, ok := adapterErr.Context["status_code"].(int); ok {
			return statusCode == 404
		}
	}
	return false
}

// IsGitHubAuthenticationError checks if the error is a GitHub authentication error
func IsGitHubAuthenticationError(err error) bool {
	if githubErr, ok := err.(*GitHubError); ok {
		return githubErr.Code == "invalid_authentication" || githubErr.Status == 401
	}
	if adapterErr, ok := err.(*AdapterError); ok {
		if adapterErr.ErrorType == ErrorTypeUnauthorized {
			return true
		}
		
		// Also check status code if available
		if statusCode, ok := adapterErr.Context["status_code"].(int); ok {
			return statusCode == 401
		}
	}
	return false
}

// IsGitHubPermissionError checks if the error is a GitHub permission error
func IsGitHubPermissionError(err error) bool {
	if githubErr, ok := err.(*GitHubError); ok {
		return githubErr.Code == "permission_denied" || githubErr.Status == 403
	}
	if adapterErr, ok := err.(*AdapterError); ok {
		if adapterErr.ErrorType == ErrorTypeForbidden {
			return true
		}
		
		// Also check status code if available
		if statusCode, ok := adapterErr.Context["status_code"].(int); ok {
			return statusCode == 403
		}
	}
	return false
}

// IsGitHubValidationError checks if the error is a GitHub validation error
func IsGitHubValidationError(err error) bool {
	if githubErr, ok := err.(*GitHubError); ok {
		return githubErr.Code == "invalid_payload" || githubErr.Status == 422
	}
	if adapterErr, ok := err.(*AdapterError); ok {
		if adapterErr.ErrorType == ErrorTypeBadRequest {
			return true
		}
		
		// Also check status code if available
		if statusCode, ok := adapterErr.Context["status_code"].(int); ok {
			return statusCode == 422
		}
	}
	return false
}

// IsGitHubServerError checks if the error is a GitHub server error
func IsGitHubServerError(err error) bool {
	if githubErr, ok := err.(*GitHubError); ok {
		return githubErr.Status >= 500 && githubErr.Status < 600
	}
	if adapterErr, ok := err.(*AdapterError); ok {
		if statusCode, ok := adapterErr.Context["status_code"].(int); ok {
			return statusCode >= 500 && statusCode < 600
		}
	}
	return false
}

// Note: FromWebhookError is already defined in errors.go, so we're not redefining it here
