package core

import (
	"context"

	"github.com/S-Corkum/devops-mcp/apps/mcp-server/internal/config"
	contextManager "github.com/S-Corkum/devops-mcp/apps/mcp-server/internal/core/context"

	"github.com/S-Corkum/devops-mcp/pkg/common/cache"
	"github.com/S-Corkum/devops-mcp/pkg/common/events/system"
	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/storage/providers"
)

// CreateContextManager creates either an optimized or standard context manager based on configuration
func CreateContextManager(
	ctx context.Context,
	db *database.Database,
	cacheClient cache.Cache,
	storage providers.ContextStorage,
	eventBus *system.EventBus,
	logger observability.Logger,
	metricsClient observability.MetricsClient,
	performanceConfig *config.ContextManagerConfig,
) (ContextManagerInterface, error) {
	// If performance config is provided, create optimized manager
	if performanceConfig != nil {
		logger.Info("Creating optimized context manager with performance enhancements", nil)

		// Note: In production, you would create the OptimizedContextManager here
		// For now, we'll use the standard manager as a fallback
		// The optimized manager requires additional setup (Redis client, read replicas, etc.)
		// that would typically be configured at the application level

		logger.Info("Optimized context manager not fully integrated yet, using standard manager", nil)
		return createStandardManager(db, cacheClient, storage, eventBus, logger, metricsClient), nil
	}

	// Create standard context manager
	return createStandardManager(db, cacheClient, storage, eventBus, logger, metricsClient), nil
}

func createStandardManager(
	db *database.Database,
	cacheClient cache.Cache,
	storage providers.ContextStorage,
	eventBus *system.EventBus,
	logger observability.Logger,
	metricsClient observability.MetricsClient,
) ContextManagerInterface {
	return contextManager.NewManager(
		db,
		cacheClient,
		storage,
		eventBus,
		logger,
		metricsClient,
	)
}
