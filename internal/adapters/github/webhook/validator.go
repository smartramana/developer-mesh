package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/xeipuuv/gojsonschema"
)

// Validator validates webhook requests
type Validator struct {
	secret         string
	schemaCatalog  map[string]*gojsonschema.Schema
	schemaLoader   gojsonschema.JSONLoader
	deliveryCache  DeliveryCache
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
	return &Validator{
		secret:        secret,
		schemaCatalog: make(map[string]*gojsonschema.Schema),
		deliveryCache: deliveryCache,
	}
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

// ValidateSignature validates the signature of a webhook request
func (v *Validator) ValidateSignature(payload []byte, signature string) error {
	if v.secret == "" {
		// If no webhook secret is configured, skip verification
		return nil
	}
	
	// Remove 'sha256=' prefix if present
	signature = strings.TrimPrefix(signature, "sha256=")
	
	// Calculate expected HMAC signature
	mac := hmac.New(sha256.New, []byte(v.secret))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))
	
	// Constant-time comparison to prevent timing attacks
	if !hmac.Equal([]byte(signature), []byte(expectedMAC)) {
		return fmt.Errorf("invalid webhook signature")
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
		return fmt.Errorf("duplicate delivery ID: %s", deliveryID)
	}
	
	// Add delivery ID to cache
	if err := v.deliveryCache.Add(deliveryID, time.Now()); err != nil {
		return fmt.Errorf("failed to add delivery ID to cache: %w", err)
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
			return fmt.Errorf("missing header: %s", header)
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
		return fmt.Errorf("failed to validate payload: %w", err)
	}
	
	// Check for validation errors
	if !result.Valid() {
		// Collect validation errors
		var errMsgs []string
		for _, err := range result.Errors() {
			errMsgs = append(errMsgs, err.String())
		}
		
		return fmt.Errorf("payload validation failed: %s", strings.Join(errMsgs, "; "))
	}
	
	return nil
}

// Validate validates a webhook request
func (v *Validator) Validate(eventType string, payload []byte, headers http.Header) error {
	// Validate headers
	if err := v.ValidateHeaders(headers); err != nil {
		return err
	}
	
	// Validate signature
	signature := headers.Get("X-Hub-Signature-256")
	if err := v.ValidateSignature(payload, signature); err != nil {
		return err
	}
	
	// Validate delivery ID
	deliveryID := headers.Get("X-GitHub-Delivery")
	if err := v.ValidateDeliveryID(deliveryID); err != nil {
		return err
	}
	
	// Validate payload schema
	if err := v.ValidatePayload(eventType, payload); err != nil {
		return err
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
