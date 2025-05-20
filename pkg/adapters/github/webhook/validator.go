package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/common/errors"
	"github.com/xeipuuv/gojsonschema"
)

// IPRange represents an IP address range
type IPRange struct {
	CIDR   string
	IPNet  *net.IPNet
}

// Validator validates webhook requests
type Validator struct {
	secret         string
	schemaCatalog  map[string]*gojsonschema.Schema
	schemaLoader   gojsonschema.JSONLoader
	deliveryCache  DeliveryCache
	ipRanges       []IPRange
	validateIPs    bool
	ipRangesLastUpdated time.Time
	// DisableSignatureValidation allows tests to bypass signature validation entirely
	disableSignatureValidation bool
}

// DeliveryCache defines the interface for caching delivery IDs
type DeliveryCache interface {
	// Has checks if a delivery ID exists in the cache
	Has(deliveryID string) bool
	
	// Add adds a delivery ID to the cache
	Add(deliveryID string, timestamp time.Time) error
	
	// GC performs garbage collection on the cache
	GC() error
}

// NewValidator creates a new webhook validator
func NewValidator(secret string, deliveryCache DeliveryCache) *Validator {
	validator := &Validator{
		secret:        secret,
		schemaCatalog: make(map[string]*gojsonschema.Schema),
		deliveryCache: deliveryCache,
		ipRanges:      []IPRange{},
		validateIPs:   false,
	}
	
	return validator
}

// EnableIPValidation enables IP validation and fetches GitHub's IP ranges
func (v *Validator) EnableIPValidation() error {
	v.validateIPs = true
	return v.updateGitHubIPRanges()
}

// DisableIPValidation disables IP validation
func (v *Validator) DisableIPValidation() {
	v.validateIPs = false
}

// updateGitHubIPRanges fetches and updates GitHub's published IP ranges
func (v *Validator) updateGitHubIPRanges() error {
	// Check if we updated recently (within the last hour)
	if time.Since(v.ipRangesLastUpdated) < time.Hour {
		return nil
	}
	
	// Fetch GitHub's Meta API
	resp, err := http.Get("https://api.github.com/meta")
	if err != nil {
		return fmt.Errorf("failed to fetch GitHub IP ranges: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch GitHub IP ranges: HTTP %d", resp.StatusCode)
	}
	
	// Parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read GitHub IP ranges response: %w", err)
	}
	
	var meta struct {
		Hooks []string `json:"hooks"`
	}
	
	if err := json.Unmarshal(body, &meta); err != nil {
		return fmt.Errorf("failed to parse GitHub IP ranges: %w", err)
	}
	
	// Parse IP ranges
	var ipRanges []IPRange
	for _, cidr := range meta.Hooks {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return fmt.Errorf("failed to parse GitHub IP range %s: %w", cidr, err)
		}
		
		ipRanges = append(ipRanges, IPRange{
			CIDR:  cidr,
			IPNet: ipNet,
		})
	}
	
	// Update IP ranges
	v.ipRanges = ipRanges
	v.ipRangesLastUpdated = time.Now()
	
	return nil
}

// ValidateSourceIP validates the source IP of a webhook request
func (v *Validator) ValidateSourceIP(sourceIP string) error {
	// Skip validation if disabled
	if !v.validateIPs {
		return nil
	}
	
	// Update IP ranges if needed
	if time.Since(v.ipRangesLastUpdated) > time.Hour {
		if err := v.updateGitHubIPRanges(); err != nil {
			// Log error but don't fail the request if we can't update IP ranges
			// This prevents valid webhooks from being rejected due to temporary issues
			return nil
		}
	}
	
	// Parse source IP
	ip := net.ParseIP(sourceIP)
	if ip == nil {
		return errors.NewGitHubError(
			errors.ErrInvalidWebhook,
			0,
			fmt.Sprintf("invalid source IP: %s", sourceIP),
		).WithContext("validation", "ip_format")
	}
	
	// Check if IP is in any of the GitHub IP ranges
	for _, ipRange := range v.ipRanges {
		if ipRange.IPNet.Contains(ip) {
			return nil
		}
	}
	
	// Create a detailed error with context
	err := errors.NewGitHubError(
		errors.ErrInvalidWebhook,
		0,
		fmt.Sprintf("source IP %s is not in GitHub's IP ranges", sourceIP),
	)
	err.WithContext("validation", "ip_not_allowed")
	
	// Add the allowed IP ranges for context
	if len(v.ipRanges) > 0 {
		var allowedRanges []string
		for _, ipRange := range v.ipRanges {
			allowedRanges = append(allowedRanges, ipRange.CIDR)
		}
		err.WithContext("allowed_ip_ranges", strings.Join(allowedRanges[:3], ", ") + "...")
	}
	
	return err
}

// SetSecret sets the webhook secret
func (v *Validator) SetSecret(secret string) {
	v.secret = secret
}

// SetDeliveryCache sets the delivery cache
func (v *Validator) SetDeliveryCache(cache DeliveryCache) {
	v.deliveryCache = cache
}

// RegisterSchema registers a JSON schema for an event type
func (v *Validator) RegisterSchema(eventType string, schema []byte) error {
	loader := gojsonschema.NewBytesLoader(schema)
	s, err := gojsonschema.NewSchema(loader)
	if err != nil {
		return fmt.Errorf("failed to load schema for %s: %w", eventType, err)
	}
	
	v.schemaCatalog[eventType] = s
	return nil
}

// DisableSignatureValidation disables webhook signature validation (for testing only)
func (v *Validator) DisableSignatureValidation() {
	v.disableSignatureValidation = true
}

// EnableSignatureValidation enables webhook signature validation (default behavior)
func (v *Validator) EnableSignatureValidation() {
	v.disableSignatureValidation = false
}

// ValidateSignature validates the signature of a webhook request
func (v *Validator) ValidateSignature(payload []byte, signature string) error {
	// Skip validation if explicitly disabled
	if v.disableSignatureValidation {
		return nil
	}
	
	// Skip validation if no secret is configured
	if v.secret == "" {
		return nil
	}
	
	// Remove 'sha256=' prefix if present
	signature = strings.TrimPrefix(signature, "sha256=")
	
	// Calculate expected HMAC signature
	mac := hmac.New(sha256.New, []byte(v.secret))
	mac.Write(payload)
	expectedMAC := mac.Sum(nil)
	
	// Decode the provided signature
	providedMAC, err := hex.DecodeString(signature)
	if err != nil {
		return errors.NewGitHubError(
			errors.ErrInvalidSignature,
			0,
			"invalid signature format",
		).WithContext("error", err.Error()).
			WithContext("signature_length", fmt.Sprintf("%d", len(signature))).
			WithResource("webhook", "signature")
	}
	
	// Constant-time comparison to prevent timing attacks
	if !hmac.Equal(providedMAC, expectedMAC) {
		return errors.NewGitHubError(
			errors.ErrInvalidSignature,
			0,
			"webhook signature verification failed",
		).WithContext("signature_length", fmt.Sprintf("%d", len(signature))).
			WithContext("expected_length", fmt.Sprintf("%d", len(expectedMAC))).
			WithResource("webhook", "signature")
	}
	
	return nil
}

// ValidateDeliveryID validates the delivery ID of a webhook request
func (v *Validator) ValidateDeliveryID(deliveryID string) error {
	if v.deliveryCache == nil {
		// If no delivery cache is configured, skip verification
		return nil
	}
	
	// Check if delivery ID has been seen before
	if v.deliveryCache.Has(deliveryID) {
		return errors.NewGitHubError(
			errors.ErrDuplicateDelivery,
			0,
			fmt.Sprintf("duplicate delivery ID: %s", deliveryID),
		)
	}
	
	// Add delivery ID to cache
	if err := v.deliveryCache.Add(deliveryID, time.Now()); err != nil {
		return errors.NewGitHubError(
			errors.ErrInvalidWebhook,
			0,
			"failed to add delivery ID to cache",
		).WithContext("error", err.Error())
	}
	
	return nil
}

// ValidateHeaders validates the headers of a webhook request
func (v *Validator) ValidateHeaders(headers http.Header) error {
	// Check for required headers
	requiredHeaders := []string{
		"X-GitHub-Event",
		"X-GitHub-Delivery",
		"X-Hub-Signature-256",
	}
	
	for _, header := range requiredHeaders {
		if headers.Get(header) == "" {
			return errors.NewGitHubError(
				errors.ErrInvalidWebhook,
				0,
				fmt.Sprintf("missing required header: %s", header),
			)
		}
	}
	
	return nil
}

// ValidatePayload validates the payload of a webhook request against its JSON schema
func (v *Validator) ValidatePayload(eventType string, payload []byte) error {
	// Get schema for event type
	schema, ok := v.schemaCatalog[eventType]
	if !ok {
		// No schema registered for this event type
		return nil
	}
	
	// Validate payload against schema
	documentLoader := gojsonschema.NewBytesLoader(payload)
	result, err := schema.Validate(documentLoader)
	if err != nil {
		return errors.NewGitHubError(
			errors.ErrInvalidPayload,
			0,
			"failed to validate payload",
		).WithContext("error", err.Error())
	}
	
	// Check for validation errors
	if !result.Valid() {
		// Collect validation errors
		var errMsgs []string
		for _, err := range result.Errors() {
			errMsgs = append(errMsgs, err.String())
		}
		
		errorMsg := strings.Join(errMsgs, "; ")
		
		return errors.NewGitHubError(
			errors.ErrInvalidPayload,
			0,
			"payload validation failed",
		).WithContext("validation_errors", errorMsg)
	}
	
	return nil
}

// Validate validates a webhook request
func (v *Validator) Validate(eventType string, payload []byte, headers http.Header) error {
	// Default empty remote address
	return v.ValidateWithIP(eventType, payload, headers, "")
}

// ValidateWithIP validates a webhook request with a remote IP address
func (v *Validator) ValidateWithIP(eventType string, payload []byte, headers http.Header, remoteAddr string) error {
	// Skip all validation if signature validation is disabled (for testing)
	if v.disableSignatureValidation {
		return nil
	}
	// Track the first error we encounter
	var validationErr error
	
	// Validate headers
	if err := v.ValidateHeaders(headers); err != nil {
		validationErr = errors.FromWebhookError(err, eventType)
		if githubErr, ok := validationErr.(*errors.GitHubError); ok {
			githubErr.WithContext("stage", "headers")
		}
		return validationErr
	}
	
	// Validate signature
	signature := headers.Get("X-Hub-Signature-256")
	if err := v.ValidateSignature(payload, signature); err != nil {
		validationErr = errors.FromWebhookError(err, eventType)
		if githubErr, ok := validationErr.(*errors.GitHubError); ok {
			githubErr.WithContext("stage", "signature")
		}
		return validationErr
	}
	
	// Validate delivery ID
	deliveryID := headers.Get("X-GitHub-Delivery")
	if err := v.ValidateDeliveryID(deliveryID); err != nil {
		validationErr = errors.FromWebhookError(err, eventType)
		if githubErr, ok := validationErr.(*errors.GitHubError); ok {
			githubErr.WithContext("stage", "delivery_id")
			githubErr.WithContext("delivery_id", deliveryID)
		}
		return validationErr
	}
	
	// Validate payload schema
	if err := v.ValidatePayload(eventType, payload); err != nil {
		validationErr = errors.FromWebhookError(err, eventType)
		if githubErr, ok := validationErr.(*errors.GitHubError); ok {
			githubErr.WithContext("stage", "payload")
		}
		return validationErr
	}
	
	// Validate source IP if enabled
	if v.validateIPs && remoteAddr != "" {
		// Extract IP from remote address (remove port if present)
		ipParts := strings.Split(remoteAddr, ":")
		sourceIP := ipParts[0]
		
		if err := v.ValidateSourceIP(sourceIP); err != nil {
			validationErr = errors.FromWebhookError(err, eventType)
			if githubErr, ok := validationErr.(*errors.GitHubError); ok {
				githubErr.WithContext("stage", "source_ip")
				githubErr.WithContext("ip", sourceIP)
			}
			return validationErr
		}
	}
	
	return nil
}

// InMemoryDeliveryCache is a simple in-memory implementation of DeliveryCache
type InMemoryDeliveryCache struct {
	cache    map[string]time.Time
	maxAge   time.Duration
}

// NewInMemoryDeliveryCache creates a new in-memory delivery cache
func NewInMemoryDeliveryCache(maxAge time.Duration) *InMemoryDeliveryCache {
	return &InMemoryDeliveryCache{
		cache:  make(map[string]time.Time),
		maxAge: maxAge,
	}
}

// Has checks if a delivery ID exists in the cache
func (c *InMemoryDeliveryCache) Has(deliveryID string) bool {
	_, ok := c.cache[deliveryID]
	return ok
}

// Add adds a delivery ID to the cache
func (c *InMemoryDeliveryCache) Add(deliveryID string, timestamp time.Time) error {
	c.cache[deliveryID] = timestamp
	return nil
}

// GC performs garbage collection on the cache
func (c *InMemoryDeliveryCache) GC() error {
	cutoff := time.Now().Add(-c.maxAge)
	
	for id, timestamp := range c.cache {
		if timestamp.Before(cutoff) {
			delete(c.cache, id)
		}
	}
	
	return nil
}

// JSONSchemas returns a map of event types to JSON schemas
func JSONSchemas() map[string][]byte {
	schemas := make(map[string][]byte)
	
	// Push event schema
	schemas["push"] = []byte(`{
		"type": "object",
		"required": ["ref", "repository", "pusher"],
		"properties": {
			"ref": { "type": "string" },
			"repository": {
				"type": "object",
				"required": ["id", "name", "full_name"],
				"properties": {
					"id": { "type": "integer" },
					"name": { "type": "string" },
					"full_name": { "type": "string" }
				}
			},
			"pusher": {
				"type": "object",
				"required": ["name"],
				"properties": {
					"name": { "type": "string" }
				}
			}
		}
	}`)
	
	// Pull request event schema
	schemas["pull_request"] = []byte(`{
		"type": "object",
		"required": ["action", "pull_request", "repository"],
		"properties": {
			"action": { "type": "string" },
			"pull_request": {
				"type": "object",
				"required": ["id", "number", "title", "state"],
				"properties": {
					"id": { "type": "integer" },
					"number": { "type": "integer" },
					"title": { "type": "string" },
					"state": { "type": "string" }
				}
			},
			"repository": {
				"type": "object",
				"required": ["id", "name", "full_name"],
				"properties": {
					"id": { "type": "integer" },
					"name": { "type": "string" },
					"full_name": { "type": "string" }
				}
			}
		}
	}`)
	
	// Issue event schema
	schemas["issues"] = []byte(`{
		"type": "object",
		"required": ["action", "issue", "repository"],
		"properties": {
			"action": { "type": "string" },
			"issue": {
				"type": "object",
				"required": ["id", "number", "title", "state"],
				"properties": {
					"id": { "type": "integer" },
					"number": { "type": "integer" },
					"title": { "type": "string" },
					"state": { "type": "string" }
				}
			},
			"repository": {
				"type": "object",
				"required": ["id", "name", "full_name"],
				"properties": {
					"id": { "type": "integer" },
					"name": { "type": "string" },
					"full_name": { "type": "string" }
				}
			}
		}
	}`)
	
	// Release event schema
	schemas["release"] = []byte(`{
		"type": "object",
		"required": ["action", "release", "repository"],
		"properties": {
			"action": { "type": "string" },
			"release": {
				"type": "object",
				"required": ["id", "tag_name", "name"],
				"properties": {
					"id": { "type": "integer" },
					"tag_name": { "type": "string" },
					"name": { "type": "string" }
				}
			},
			"repository": {
				"type": "object",
				"required": ["id", "name", "full_name"],
				"properties": {
					"id": { "type": "integer" },
					"name": { "type": "string" },
					"full_name": { "type": "string" }
				}
			}
		}
	}`)
	
	return schemas
}
