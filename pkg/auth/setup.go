package auth

import (
	"os"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/common/cache"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/jmoiron/sqlx"
)

// SetupAuthentication sets up the enhanced authentication service
func SetupAuthentication(db *sqlx.DB, cache cache.Cache, logger observability.Logger, metrics observability.MetricsClient) (*AuthMiddleware, error) {
	// Create base service
	config := &ServiceConfig{
		JWTSecret:         getEnvOrDefault("JWT_SECRET", ""),
		JWTExpiration:     24 * time.Hour,
		APIKeyHeader:      "X-API-Key",
		EnableAPIKeys:     true,
		EnableJWT:         true,
		CacheEnabled:      true,
		CacheTTL:          5 * time.Minute,
		MaxFailedAttempts: 5,
		LockoutDuration:   15 * time.Minute,
	}

	baseService := NewService(config, db, cache, logger)

	// Create rate limiter
	rateLimiter := NewRateLimiter(cache, logger, nil)

	// Create metrics and audit
	metricsCollector := NewMetricsCollector(metrics)
	auditLogger := NewAuditLogger(logger)

	// Create auth middleware
	authMiddleware := NewAuthMiddleware(baseService, rateLimiter, metricsCollector, auditLogger)

	// Load auth configuration based on environment - this handles both config file and env vars
	if err := baseService.LoadAuthConfigBasedOnEnvironment(); err != nil {
		logger.Warn("Failed to load auth configuration from file", map[string]interface{}{
			"error": err.Error(),
		})
	}

	return authMiddleware, nil
}

// getEnvOrDefault gets environment variable or returns default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
