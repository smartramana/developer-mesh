package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/storage"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/developer-mesh/developer-mesh/pkg/tools/adapters"
)

// DiscoveryServiceInterface defines the interface for discovery operations
type DiscoveryServiceInterface interface {
	DiscoverTool(ctx context.Context, config tools.ToolConfig) (*tools.DiscoveryResult, error)
	SaveDiscoveryHints(ctx context.Context, toolID string, hints *adapters.DiscoveryHints) error
	GetDiscoveryMetrics(ctx context.Context) (*DiscoveryMetrics, error)
}

// DiscoveryMetrics contains discovery performance metrics
type DiscoveryMetrics struct {
	TotalAttempts   int64
	SuccessfulCount int64
	FailedCount     int64
	AverageTime     time.Duration
	TopPatterns     []adapters.DiscoveryPattern
}

// EnhancedDiscoveryService implements intelligent discovery with learning
type EnhancedDiscoveryService struct {
	discoveryService *adapters.DiscoveryService
	patternRepo      *storage.DiscoveryPatternRepository
	hintRepo         *storage.DiscoveryHintRepository
	logger           observability.Logger
	metricsClient    observability.MetricsClient
	formatDetector   *adapters.FormatDetector
	learningService  *adapters.LearningDiscoveryService
}

// NewEnhancedDiscoveryService creates a new enhanced discovery service
func NewEnhancedDiscoveryService(
	logger observability.Logger,
	metricsClient observability.MetricsClient,
	patternRepo *storage.DiscoveryPatternRepository,
	hintRepo *storage.DiscoveryHintRepository,
) DiscoveryServiceInterface {
	// Create HTTP client with proper timeouts
	// Increased timeout to 60s to handle large OpenAPI specs like GitHub
	httpClient := &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	// Initialize components following dependency injection
	formatDetector := adapters.NewFormatDetector(httpClient)
	learningService := adapters.NewLearningDiscoveryService(patternRepo)

	// Create base discovery service with all enhancements
	discoveryService := adapters.NewDiscoveryServiceWithStore(
		logger,
		httpClient,
		tools.NewURLValidator(),
		patternRepo, // Use database pattern store
	)

	return &EnhancedDiscoveryService{
		discoveryService: discoveryService,
		patternRepo:      patternRepo,
		hintRepo:         hintRepo,
		logger:           logger,
		metricsClient:    metricsClient,
		formatDetector:   formatDetector,
		learningService:  learningService,
	}
}

// DiscoverTool performs intelligent tool discovery
func (s *EnhancedDiscoveryService) DiscoverTool(ctx context.Context, config tools.ToolConfig) (*tools.DiscoveryResult, error) {
	start := time.Now()

	// Track metrics
	defer func() {
		duration := time.Since(start)
		if s.metricsClient != nil {
			s.metricsClient.RecordHistogram("discovery.duration", float64(duration.Milliseconds()), map[string]string{
				"base_url": config.BaseURL,
			})
		}
	}()

	// Execute discovery with all enhancements
	result, err := s.discoveryService.DiscoverOpenAPISpec(ctx, config)

	// Track success/failure
	if s.metricsClient != nil {
		status := "failure"
		if err == nil && result.Status == tools.DiscoveryStatusSuccess {
			status = "success"
		}
		s.metricsClient.IncrementCounterWithLabels("discovery.attempts", 1, map[string]string{
			"status": status,
		})
	}

	// Log discovery attempt
	s.logger.Info("Tool discovery completed", map[string]interface{}{
		"base_url":    config.BaseURL,
		"status":      result.Status,
		"duration_ms": time.Since(start).Milliseconds(),
		"urls_found":  len(result.DiscoveredURLs),
		"error":       err,
	})

	return result, err
}

// SaveDiscoveryHints saves user-provided discovery hints
func (s *EnhancedDiscoveryService) SaveDiscoveryHints(ctx context.Context, toolID string, hints *adapters.DiscoveryHints) error {
	if err := s.hintRepo.SaveHints(ctx, toolID, hints); err != nil {
		s.logger.Error("Failed to save discovery hints", map[string]interface{}{
			"tool_id": toolID,
			"error":   err.Error(),
		})
		return fmt.Errorf("failed to save discovery hints: %w", err)
	}

	s.logger.Info("Discovery hints saved", map[string]interface{}{
		"tool_id": toolID,
	})

	return nil
}

// GetDiscoveryMetrics retrieves discovery performance metrics
func (s *EnhancedDiscoveryService) GetDiscoveryMetrics(ctx context.Context) (*DiscoveryMetrics, error) {
	// This would aggregate metrics from the metrics client
	// For now, return sample data
	patterns := s.learningService.GetPopularPatterns()

	return &DiscoveryMetrics{
		TotalAttempts:   100,
		SuccessfulCount: 85,
		FailedCount:     15,
		AverageTime:     2 * time.Second,
		TopPatterns:     patterns[:min(5, len(patterns))],
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
