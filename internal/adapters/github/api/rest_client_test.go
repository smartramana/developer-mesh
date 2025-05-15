package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/S-Corkum/devops-mcp/internal/adapters/github/auth"
	"github.com/S-Corkum/devops-mcp/internal/adapters/resilience"
	"github.com/S-Corkum/devops-mcp/internal/observability"
)

func TestRESTClient_RateLimitHandling(t *testing.T) {
	logger := observability.NewLogger("test.restclient")
metrics := observability.NewMetricsClient()
rateLimiter := resilience.NewRateLimiter(resilience.RateLimiterConfig{
	Name:  "test",
	Rate:  1,
	Burst: 1,
})
restClient := NewRESTClient(&RESTConfig{
	BaseURL:      "https://api.github.com/",
	AuthProvider: auth.NewNoAuthProvider(logger),
}, &http.Client{}, rateLimiter, logger, metrics)

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
	logger := observability.NewLogger("test.restclient")
metrics := observability.NewMetricsClient()
restClient := NewRESTClient(&RESTConfig{
	BaseURL:      "https://api.github.com/",
	AuthProvider: auth.NewNoAuthProvider(logger),
}, &http.Client{}, resilience.NewRateLimiter(resilience.RateLimiterConfig{
	Name:  "test",
	Rate:  100,
	Burst: 10,
}), logger, metrics)

	path := "repos/test/test"
	etag := "test-etag"
	restClient.storeETag(path, etag)
	if got := restClient.getETag(path); got != etag {
		t.Errorf("Expected ETag %q, got %q", etag, got)
	}
}
