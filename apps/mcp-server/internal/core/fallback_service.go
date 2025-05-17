package core

import (
	"context"
	"fmt"
	"time"

	"github.com/S-Corkum/devops-mcp/internal/observability"
	"github.com/google/uuid"
)

// FallbackService provides degraded service when primary services fail
type FallbackService struct {
	metricsClient observability.MetricsClient
}

// NewFallbackService creates a new fallback service
func NewFallbackService() *FallbackService {
	return &FallbackService{
		metricsClient: observability.NewMetricsClient(),
	}
}

// EmergencyHealthCheck provides a minimal health check
func (s *FallbackService) EmergencyHealthCheck() map[string]string {
	return map[string]string{
		"service":   "fallback",
		"status":    "active",
		"timestamp": time.Now().Format(time.RFC3339),
	}
}

// GenerateEmergencyID generates a unique ID for emergency use
func (s *FallbackService) GenerateEmergencyID() string {
	return uuid.New().String()
}

// LogEmergencyEvent logs an emergency event
func (s *FallbackService) LogEmergencyEvent(ctx context.Context, eventType, details string) error {
	// In a real implementation, this would log to a persistent store
	// For now, just log to stdout
	fmt.Printf("[EMERGENCY EVENT] %s - %s - %s\n", time.Now().Format(time.RFC3339), eventType, details)
	return nil
}
