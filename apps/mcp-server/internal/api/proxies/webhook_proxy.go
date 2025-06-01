package proxies

import (
	"context"
	"net/http"

	"github.com/S-Corkum/devops-mcp/pkg/client/rest"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// WebhookAPIProxy implements webhook handling via the REST API
type WebhookAPIProxy struct {
	client *rest.WebhookClient
	logger observability.Logger
}

// WebhookRepository defines the interface for webhook handling
type WebhookRepository interface {
	ProcessWebhook(ctx context.Context, provider string, headers http.Header, body []byte) error
	ValidateIP(ctx context.Context, provider string, ipAddress string) (bool, error)
}

// NewWebhookAPIProxy creates a new webhook proxy that delegates to the REST API
func NewWebhookAPIProxy(client *rest.WebhookClient, logger observability.Logger) WebhookRepository {
	if logger == nil {
		logger = observability.NewLogger("webhook-api-proxy")
	}

	return &WebhookAPIProxy{
		client: client,
		logger: logger,
	}
}

// ProcessWebhook delegates webhook processing to the REST API
func (p *WebhookAPIProxy) ProcessWebhook(ctx context.Context, provider string, headers http.Header, body []byte) error {
	p.logger.Debug("Processing webhook via REST API", map[string]interface{}{
		"provider":  provider,
		"body_size": len(body),
	})

	return p.client.ProcessWebhook(ctx, provider, headers, body)
}

// ValidateIP delegates IP validation to the REST API
func (p *WebhookAPIProxy) ValidateIP(ctx context.Context, provider string, ipAddress string) (bool, error) {
	p.logger.Debug("Validating webhook IP via REST API", map[string]interface{}{
		"provider":   provider,
		"ip_address": ipAddress,
	})

	return p.client.ValidateIP(ctx, provider, ipAddress)
}

// MockWebhookRepository provides a mock implementation for testing
type MockWebhookRepository struct {
	logger observability.Logger
}

// NewMockWebhookRepository creates a new mock webhook repository
func NewMockWebhookRepository(logger observability.Logger) WebhookRepository {
	if logger == nil {
		logger = observability.NewLogger("mock-webhook-repository")
	}

	return &MockWebhookRepository{
		logger: logger,
	}
}

// ProcessWebhook mock implementation
func (m *MockWebhookRepository) ProcessWebhook(ctx context.Context, provider string, headers http.Header, body []byte) error {
	m.logger.Debug("Mock webhook processing", map[string]interface{}{
		"provider":  provider,
		"body_size": len(body),
	})

	return nil
}

// ValidateIP mock implementation
func (m *MockWebhookRepository) ValidateIP(ctx context.Context, provider string, ipAddress string) (bool, error) {
	m.logger.Debug("Mock IP validation", map[string]interface{}{
		"provider":   provider,
		"ip_address": ipAddress,
	})

	// Always return valid in mock
	return true, nil
}
