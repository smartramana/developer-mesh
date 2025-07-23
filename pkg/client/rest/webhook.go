package rest

import (
	"context"
	"fmt"
	"net/http"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// WebhookConfig contains the configuration for webhook handling
type WebhookConfig struct {
	Enabled       bool
	Secret        string
	Endpoint      string
	AllowedEvents []string
	ValidateIP    bool
	IPWhitelist   []string
}

// WebhookClient handles webhook processing through the REST API
type WebhookClient struct {
	client *RESTClient
	logger observability.Logger
	config WebhookConfig
}

// NewWebhookClient creates a new WebhookClient with the given configuration
func NewWebhookClient(client *RESTClient, config WebhookConfig, logger observability.Logger) *WebhookClient {
	if logger == nil {
		logger = observability.NewLogger("webhook-client")
	}

	return &WebhookClient{
		client: client,
		config: config,
		logger: logger,
	}
}

// ProcessWebhook handles an incoming webhook request
func (c *WebhookClient) ProcessWebhook(ctx context.Context, provider string, headers http.Header, body []byte) error {
	endpoint := fmt.Sprintf("/api/webhooks/%s", provider)

	req := WebhookRequest{
		Provider: provider,
		Headers:  make(map[string][]string),
		Body:     body,
	}

	// Copy headers we care about
	for k, v := range headers {
		req.Headers[k] = v
	}

	var resp struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}

	err := c.client.Post(ctx, endpoint, req, &resp)
	if err != nil {
		return fmt.Errorf("webhook processing failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("webhook processing error: %s", resp.Error)
	}

	return nil
}

// ValidateIP checks if the given IP is allowed to send webhooks
func (c *WebhookClient) ValidateIP(ctx context.Context, provider string, ipAddress string) (bool, error) {
	endpoint := fmt.Sprintf("/api/webhooks/%s/validate-ip", provider)

	req := struct {
		IPAddress string `json:"ip_address"`
	}{
		IPAddress: ipAddress,
	}

	var resp struct {
		Valid bool   `json:"valid"`
		Error string `json:"error,omitempty"`
	}

	err := c.client.Post(ctx, endpoint, req, &resp)
	if err != nil {
		return false, fmt.Errorf("IP validation failed: %w", err)
	}

	return resp.Valid, nil
}

// WebhookRequest represents a webhook request to be processed
type WebhookRequest struct {
	Provider string              `json:"provider"`
	Headers  map[string][]string `json:"headers"`
	Body     []byte              `json:"body"`
}
