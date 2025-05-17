package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/S-Corkum/devops-mcp/internal/adapters/resilience"
	"github.com/S-Corkum/devops-mcp/internal/observability"
)

func TestGraphQLClient_RateLimitHandling(t *testing.T) {
	logger := observability.NewLogger("test.graphqlclient")
metrics := observability.NewMetricsClient()
rateLimiter := resilience.NewRateLimiter(resilience.RateLimiterConfig{
	Name:  "test",
	Rate:  1,
	Burst: 1,
})
gqlClient := NewGraphQLClient(&Config{}, &http.Client{}, rateLimiter, logger, metrics)

	ctx := context.Background()
	// Simulate rate limit exceeded
	for rateLimiter.Allow() {
		// Exhaust the rate limiter
	}
	if err := gqlClient.Query(ctx, "query { viewer { login } }", nil, nil); err == nil {
		t.Error("Expected error when rate limit exceeded, got nil")
	}
}
