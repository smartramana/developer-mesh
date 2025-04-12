package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters"
	"github.com/S-Corkum/mcp-server/pkg/models"
)

// Config holds configuration for the Harness adapter
type Config struct {
	APIToken       string        `mapstructure:"api_token"`
	AccountID      string        `mapstructure:"account_id"`
	WebhookSecret  string        `mapstructure:"webhook_secret"`
	BaseURL        string        `mapstructure:"base_url"`
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
	MaxRetries     int           `mapstructure:"max_retries"`
	RetryDelay     time.Duration `mapstructure:"retry_delay"`
}

// Adapter implements the Harness integration
type Adapter struct {
	client       *http.Client
	config       Config
	subscribers  map[string][]func(interface{})
	subscriberMu sync.RWMutex
	baseAdapter  adapters.BaseAdapter
}

// NewAdapter creates a new Harness adapter
func NewAdapter(cfg Config) (*Adapter, error) {
	// Configure HTTP client with timeout
	httpClient := &http.Client{
		Timeout: cfg.RequestTimeout,
	}

	adapter := &Adapter{
		client:      httpClient,
		config:      cfg,
		subscribers: make(map[string][]func(interface{})),
		baseAdapter: adapters.BaseAdapter{
			RetryMax:   cfg.MaxRetries,
			RetryDelay: cfg.RetryDelay,
		},
	}

	return adapter, nil
}

// Initialize initializes the adapter
func (a *Adapter) Initialize(ctx context.Context, config interface{}) error {
	// Test connection by calling Harness API
	return a.baseAdapter.CallWithRetry(func() error {
		return a.testConnection(ctx)
	})
}

// testConnection verifies that we can connect to the Harness API
func (a *Adapter) testConnection(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/ping", a.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Add("X-Api-Key", a.config.APIToken)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-Account-Id", a.config.AccountID)

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("harness API error: status code %d", resp.StatusCode)
	}

	return nil
}

// GetData retrieves data from Harness
func (a *Adapter) GetData(ctx context.Context, query interface{}) (interface{}, error) {
	queryParams, ok := query.(models.HarnessQuery)
	if !ok {
		return nil, fmt.Errorf("invalid query type, expected HarnessQuery")
	}

	var result interface{}
	var err error

	// Execute the query with retry logic
	err = a.baseAdapter.CallWithRetry(func() error {
		// Build the request URL based on query type
		url := fmt.Sprintf("%s/api/v1/%s", a.config.BaseURL, a.buildEndpoint(queryParams))

		// Create request
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}

		// Add headers
		req.Header.Add("X-Api-Key", a.config.APIToken)
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("X-Account-Id", a.config.AccountID)

		// Execute request
		resp, err := a.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode >= 400 {
			return fmt.Errorf("harness API error: %s", resp.Status)
		}

		// Parse response based on query type
		switch queryParams.Type {
		case models.HarnessQueryTypePipeline:
			var pipeline models.HarnessPipeline
			if err := json.NewDecoder(resp.Body).Decode(&pipeline); err != nil {
				return err
			}
			result = pipeline

		case models.HarnessQueryTypeCIBuild:
			var ciBuild models.HarnessCIBuild
			if err := json.NewDecoder(resp.Body).Decode(&ciBuild); err != nil {
				return err
			}
			result = ciBuild

		case models.HarnessQueryTypeCDDeployment:
			var cdDeployment models.HarnessCDDeployment
			if err := json.NewDecoder(resp.Body).Decode(&cdDeployment); err != nil {
				return err
			}
			result = cdDeployment

		case models.HarnessQueryTypeSTOExperiment:
			var stoExperiment models.HarnessSTOExperiment
			if err := json.NewDecoder(resp.Body).Decode(&stoExperiment); err != nil {
				return err
			}
			result = stoExperiment

		case models.HarnessQueryTypeFeatureFlag:
			var featureFlag models.HarnessFeatureFlag
			if err := json.NewDecoder(resp.Body).Decode(&featureFlag); err != nil {
				return err
			}
			result = featureFlag

		default:
			return fmt.Errorf("unsupported query type: %s", queryParams.Type)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// buildEndpoint constructs the API endpoint based on query parameters
func (a *Adapter) buildEndpoint(query models.HarnessQuery) string {
	switch query.Type {
	case models.HarnessQueryTypePipeline:
		return fmt.Sprintf("pipelines/%s", query.ID)
	case models.HarnessQueryTypeCIBuild:
		return fmt.Sprintf("ci/builds/%s", query.ID)
	case models.HarnessQueryTypeCDDeployment:
		return fmt.Sprintf("cd/deployments/%s", query.ID)
	case models.HarnessQueryTypeSTOExperiment:
		return fmt.Sprintf("sto/experiments/%s", query.ID)
	case models.HarnessQueryTypeFeatureFlag:
		return fmt.Sprintf("ff/flags/%s", query.ID)
	default:
		return ""
	}
}

// HandleWebhook processes Harness webhook events
func (a *Adapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	// Verify webhook signature if secret is configured
	// This would be done at the API layer

	// Parse the event
	event, err := a.parseWebhookEvent(eventType, payload)
	if err != nil {
		return err
	}

	// Notify subscribers
	a.notifySubscribers(eventType, event)

	return nil
}

// parseWebhookEvent parses a Harness webhook event
func (a *Adapter) parseWebhookEvent(eventType string, payload []byte) (interface{}, error) {
	switch eventType {
	case "ci.build":
		var event models.HarnessCIBuildEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		return event, nil

	case "cd.deployment":
		var event models.HarnessCDDeploymentEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		return event, nil

	case "sto.experiment":
		var event models.HarnessSTOExperimentEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		return event, nil

	case "ff.change":
		var event models.HarnessFeatureFlagEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		return event, nil

	default:
		// For unknown event types, return raw payload
		var event map[string]interface{}
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		return event, nil
	}
}

// Subscribe adds a callback for a specific event type
func (a *Adapter) Subscribe(eventType string, callback func(interface{})) error {
	a.subscriberMu.Lock()
	defer a.subscriberMu.Unlock()

	a.subscribers[eventType] = append(a.subscribers[eventType], callback)
	return nil
}

// notifySubscribers notifies subscribers of an event
func (a *Adapter) notifySubscribers(eventType string, event interface{}) {
	a.subscriberMu.RLock()
	defer a.subscriberMu.RUnlock()

	for _, callback := range a.subscribers[eventType] {
		go callback(event)
	}
}

// Health returns the health status of the adapter
func (a *Adapter) Health() string {
	// Check Harness API status
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/api/v1/ping", a.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Sprintf("unhealthy: %v", err)
	}

	req.Header.Add("X-Api-Key", a.config.APIToken)
	req.Header.Add("X-Account-Id", a.config.AccountID)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Sprintf("unhealthy: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("unhealthy: status code %d", resp.StatusCode)
	}

	return "healthy"
}

// Close gracefully shuts down the adapter
func (a *Adapter) Close() error {
	// Nothing specific to clean up for this adapter
	return nil
}
