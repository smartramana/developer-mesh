package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters"
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
	// Implementation would go here - simplified for brevity
	return nil, fmt.Errorf("not implemented")
}

// HandleWebhook processes Harness webhook events
func (a *Adapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
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
	var event map[string]interface{}
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return event, nil
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
