package github

import (
	"net/http"
)

// WebhookValidator defines methods for validating GitHub webhooks
type WebhookValidator interface {
	// VerifySignature verifies the signature of a webhook payload against the secret
	VerifySignature(payload []byte, signature string) error
	
	// GetWebhookSecret returns the webhook secret configured for the adapter
	GetWebhookSecret() string
	
	// SetWebhookSecret sets the webhook secret for the adapter
	SetWebhookSecret(secret string)
	
	// ValidateWebhook validates a webhook request against signature, delivery ID, and payload schema
	ValidateWebhook(eventType string, payload []byte, headers http.Header) error
	
	// ValidateWebhookWithIP validates a webhook request including source IP validation
	ValidateWebhookWithIP(eventType string, payload []byte, headers http.Header, remoteAddr string) error
}

// AsWebhookValidator tries to convert the adapter to a WebhookValidator interface
func AsWebhookValidator(adapter interface{}) (WebhookValidator, bool) {
	validator, ok := adapter.(WebhookValidator)
	return validator, ok
}
