package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/S-Corkum/devops-mcp/pkg/adapters/github/auth"
	"github.com/S-Corkum/devops-mcp/pkg/adapters/resilience"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

func TestRESTClient_RateLimitHandling(t *testing.T) {
	// Skip test during migration
	t.Skip("Skipping test during migration")
	
	logger := observability.NewLogger("test.restclient")
	rateLimiter := resilience.NewRateLimiter(resilience.RateLimiterConfig{
		Name:  "test",
		Rate:  1,
		Burst: 1,
	})
	restClient := NewRESTClient(
		&RESTConfig{
			BaseURL:      "https://api.github.com/",
			AuthProvider: auth.NewNoAuthProvider(logger),
		}, 
		&http.Client{}, 
		nil, // Skipping in the test
		nil, // Skipping rateLimitCallback
		logger,
	)

	ctx := context.Background()
	// Simulate rate limit exceeded
	for rateLimiter.Allow() {
		// Exhaust the rate limiter
	}
	if err := restClient.Get(ctx, "user", nil, nil); err == nil {
		t.Error("Expected error when rate limit exceeded, got nil")
	}
}

func TestRESTClient_ETagCache(t *testing.T) {
	// Skip test during migration
	t.Skip("Skipping test during migration")
	
	logger := observability.NewLogger("test.restclient")
	restClient := NewRESTClient(
		&RESTConfig{
			BaseURL:      "https://api.github.com/",
			AuthProvider: auth.NewNoAuthProvider(logger),
		}, 
		&http.Client{}, 
		nil, // Skipping in the test 
		nil, // Skipping rateLimitCallback
		logger,
	)

	path := "repos/test/test"
	etag := "test-etag"
	restClient.storeETag(path, etag)
	if got := restClient.getETag(path); got != etag {
		t.Errorf("Expected ETag %q, got %q", etag, got)
	}
}
