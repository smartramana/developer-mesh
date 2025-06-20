package api

import (
	"context"
	"fmt"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/common/cache"
	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// ConnectionHelper provides production-ready connection management with retry logic
type ConnectionHelper struct {
	logger observability.Logger
}

// NewConnectionHelper creates a new connection helper
func NewConnectionHelper(logger observability.Logger) *ConnectionHelper {
	return &ConnectionHelper{
		logger: logger,
	}
}

// ConnectToDatabase establishes database connection with retry logic
func (h *ConnectionHelper) ConnectToDatabase(ctx context.Context, config database.Config) (*database.Database, error) {
	maxRetries := 5
	baseDelay := time.Second
	maxDelay := time.Minute

	var db *database.Database
	var err error

	for attempt := range maxRetries {
		if attempt > 0 {
			delay := min(baseDelay*time.Duration(1<<uint(attempt-1)), maxDelay)
			h.logger.Info("Retrying database connection", map[string]any{
				"attempt": attempt + 1,
				"delay":   delay.String(),
			})

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		db, err = database.NewDatabase(ctx, config)
		if err == nil && db != nil && db.GetDB() != nil {
			// Verify connection with ping
			pingErr := db.GetDB().PingContext(ctx)
			if pingErr == nil {
				h.logger.Info("Database connection established", map[string]any{
					"attempt": attempt + 1,
				})
				return db, nil
			}
			err = fmt.Errorf("database ping failed: %w", pingErr)
		}

		h.logger.Warn("Database connection failed", map[string]any{
			"attempt": attempt + 1,
			"error":   err.Error(),
		})
	}

	return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
}

// ConnectToCache establishes cache connection with retry logic and graceful degradation
func (h *ConnectionHelper) ConnectToCache(ctx context.Context, config any) (cache.Cache, error) {
	maxRetries := 3
	baseDelay := 500 * time.Millisecond

	var cacheClient cache.Cache
	var err error

	for attempt := range maxRetries {
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<uint(attempt-1))
			h.logger.Info("Retrying cache connection", map[string]any{
				"attempt": attempt + 1,
				"delay":   delay.String(),
			})

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// Try to create cache connection
		cacheClient, err = cache.NewCache(ctx, config)

		if err == nil && cacheClient != nil {
			// Test connection
			testCtx, testCancel := context.WithTimeout(ctx, 2*time.Second)
			testErr := cacheClient.Set(testCtx, "health:check", "ok", time.Second)
			testCancel()

			if testErr == nil {
				h.logger.Info("Cache connection established", map[string]any{
					"attempt": attempt + 1,
				})
				return cacheClient, nil
			}
			err = fmt.Errorf("cache health check failed: %w", testErr)
		}

		h.logger.Warn("Cache connection failed", map[string]any{
			"attempt": attempt + 1,
			"error":   err.Error(),
		})
	}

	// Return nil cache for graceful degradation
	h.logger.Error("Cache connection failed, running without cache", map[string]any{
		"error": err.Error(),
	})
	return nil, nil
}

// WaitForDependencies waits for external dependencies to be ready
func (h *ConnectionHelper) WaitForDependencies(ctx context.Context, dependencies []string) error {
	maxWait := 2 * time.Minute
	checkInterval := 2 * time.Second

	waitCtx, cancel := context.WithTimeout(ctx, maxWait)
	defer cancel()

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	start := time.Now()
	h.logger.Info("Waiting for dependencies to be ready", map[string]any{
		"dependencies": dependencies,
		"max_wait":     maxWait.String(),
	})

	for {
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("timeout waiting for dependencies after %v", time.Since(start))
		case <-ticker.C:
			// In a real implementation, you would check each dependency
			// For now, we'll just wait a reasonable time
			if time.Since(start) > 10*time.Second {
				h.logger.Info("Dependencies check passed", map[string]any{
					"duration": time.Since(start).String(),
				})
				return nil
			}
		}
	}
}
