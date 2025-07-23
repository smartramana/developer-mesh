package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// MetricsCollector collects authentication metrics
type MetricsCollector struct {
	metrics observability.MetricsClient
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(metrics observability.MetricsClient) *MetricsCollector {
	// Metrics will be registered/recorded when they are used
	return &MetricsCollector{
		metrics: metrics,
	}
}

// RecordAuthAttempt records an authentication attempt
func (mc *MetricsCollector) RecordAuthAttempt(ctx context.Context, authType string, success bool, duration time.Duration) {
	labels := map[string]string{
		"auth_type": authType,
		"success":   fmt.Sprintf("%t", success),
	}

	mc.metrics.RecordCounter("auth_attempts_total", 1.0, labels)

	if success {
		mc.metrics.RecordCounter("auth_success_total", 1.0, labels)
	} else {
		mc.metrics.RecordCounter("auth_failure_total", 1.0, labels)
	}

	mc.metrics.RecordHistogram("auth_duration_seconds", duration.Seconds(), labels)
}

// RecordRateLimitExceeded records rate limit exceeded events
func (mc *MetricsCollector) RecordRateLimitExceeded(ctx context.Context, identifier string) {
	mc.metrics.RecordCounter("auth_rate_limit_exceeded_total", 1.0, map[string]string{
		"identifier_type": getIdentifierType(identifier),
	})
}

// UpdateActiveSessions updates the active sessions gauge
func (mc *MetricsCollector) UpdateActiveSessions(count float64) {
	mc.metrics.RecordGauge("auth_active_sessions", count, nil)
}

func getIdentifierType(identifier string) string {
	if strings.HasPrefix(identifier, "user:") {
		return "user"
	}
	return "anonymous"
}
