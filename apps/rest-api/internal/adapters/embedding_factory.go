package adapters

import (
	"context"
	"fmt"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/agents"
	"github.com/S-Corkum/devops-mcp/pkg/common/cache"
	"github.com/S-Corkum/devops-mcp/pkg/common/config"
	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/embedding"
	"github.com/S-Corkum/devops-mcp/pkg/embedding/providers"
)

// CreateEmbeddingService creates the multi-agent embedding service
func CreateEmbeddingService(cfg *config.Config, db database.Database, cache cache.Cache) (*embedding.ServiceV2, error) {
	// Initialize providers map
	providerMap := make(map[string]providers.Provider)

	// Check if config exists
	if cfg == nil {
		return nil, fmt.Errorf("configuration is nil")
	}

	// Configure OpenAI if enabled
	if cfg.Embedding.Providers.OpenAI.Enabled && cfg.Embedding.Providers.OpenAI.APIKey != "" {
		openaiCfg := providers.ProviderConfig{
			APIKey:         cfg.Embedding.Providers.OpenAI.APIKey,
			Endpoint:       "https://api.openai.com/v1",
			MaxRetries:     3,
			RetryDelayBase: 100 * time.Millisecond,
		}

		openaiProvider, err := providers.NewOpenAIProvider(openaiCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create OpenAI provider: %w", err)
		}
		providerMap["openai"] = openaiProvider
	}

	// Configure AWS Bedrock if enabled
	if cfg.Embedding.Providers.Bedrock.Enabled {
		bedrockCfg := providers.ProviderConfig{
			Region:   cfg.Embedding.Providers.Bedrock.Region,
			Endpoint: cfg.Embedding.Providers.Bedrock.Endpoint,
		}

		bedrockProvider, err := providers.NewBedrockProvider(bedrockCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create Bedrock provider: %w", err)
		}
		providerMap["bedrock"] = bedrockProvider
	}

	// Configure Google if enabled
	if cfg.Embedding.Providers.Google.Enabled && cfg.Embedding.Providers.Google.APIKey != "" {
		googleCfg := providers.ProviderConfig{
			APIKey:   cfg.Embedding.Providers.Google.APIKey,
			Endpoint: cfg.Embedding.Providers.Google.Endpoint,
		}

		googleProvider, err := providers.NewGoogleProvider(googleCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create Google provider: %w", err)
		}
		providerMap["google"] = googleProvider
	}

	// Require at least one provider
	if len(providerMap) == 0 {
		return nil, fmt.Errorf("at least one embedding provider must be configured (OpenAI, Bedrock, or Google)")
	}

	// Initialize repositories
	sqlxDB := db.DB()
	sqlDB := sqlxDB.DB // Get the underlying *sql.DB from *sqlx.DB

	// Create agent repository and service
	agentRepo := agents.NewPostgresRepository(sqlxDB, "mcp")
	agentService := agents.NewService(agentRepo)

	// Create embedding repository
	embeddingRepo := embedding.NewRepository(sqlDB)

	// For now, metrics repository can be nil - it's optional
	var metricsRepo embedding.MetricsRepository

	// Create embedding cache adapter
	embeddingCache := NewEmbeddingCacheAdapter(cache)

	// Create ServiceV2 - this is our ONLY embedding service
	return embedding.NewServiceV2(embedding.ServiceV2Config{
		Providers:    providerMap,
		AgentService: agentService,
		Repository:   embeddingRepo,
		MetricsRepo:  metricsRepo,
		Cache:        embeddingCache,
	})
}

// EmbeddingCacheAdapter adapts cache.Cache to embedding.EmbeddingCache
type EmbeddingCacheAdapter struct {
	cache cache.Cache
}

func NewEmbeddingCacheAdapter(cache cache.Cache) embedding.EmbeddingCache {
	return &EmbeddingCacheAdapter{cache: cache}
}

func (e *EmbeddingCacheAdapter) Get(ctx context.Context, key string) (*embedding.CachedEmbedding, error) {
	var cached embedding.CachedEmbedding
	err := e.cache.Get(ctx, key, &cached)
	if err != nil {
		return nil, err
	}
	return &cached, nil
}

func (e *EmbeddingCacheAdapter) Set(ctx context.Context, key string, embedding *embedding.CachedEmbedding, ttl time.Duration) error {
	return e.cache.Set(ctx, key, embedding, ttl)
}

func (e *EmbeddingCacheAdapter) Delete(ctx context.Context, key string) error {
	return e.cache.Delete(ctx, key)
}
