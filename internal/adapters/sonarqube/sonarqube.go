package sonarqube

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters"
	"github.com/S-Corkum/mcp-server/pkg/models"
)

// Config holds configuration for the SonarQube adapter
type Config struct {
	BaseURL        string        `mapstructure:"base_url"`
	Username       string        `mapstructure:"username"`
	Password       string        `mapstructure:"password"`
	Token          string        `mapstructure:"token"`
	WebhookSecret  string        `mapstructure:"webhook_secret"`
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
	MaxRetries     int           `mapstructure:"max_retries"`
	RetryDelay     time.Duration `mapstructure:"retry_delay"`
}

// Adapter implements the SonarQube integration
type Adapter struct {
	client       *http.Client
	config       Config
	subscribers  map[string][]func(interface{})
	subscriberMu sync.RWMutex
	baseAdapter  adapters.BaseAdapter
}

// NewAdapter creates a new SonarQube adapter
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
	// Test connection by calling SonarQube API
	return a.baseAdapter.CallWithRetry(func() error {
		return a.testConnection(ctx)
	})
}

// testConnection verifies that we can connect to the SonarQube API
func (a *Adapter) testConnection(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/system/status", a.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	// Add authorization header
	a.addAuthHeader(req)

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sonarqube API error: status code %d", resp.StatusCode)
	}

	return nil
}

// addAuthHeader adds authentication header to the request
func (a *Adapter) addAuthHeader(req *http.Request) {
	if a.config.Token != "" {
		// Use token-based authentication
		req.Header.Add("Authorization", "Bearer "+a.config.Token)
	} else {
		// Use basic authentication
		auth := a.config.Username + ":" + a.config.Password
		encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
		req.Header.Add("Authorization", "Basic "+encodedAuth)
	}
}

// GetData retrieves data from SonarQube
func (a *Adapter) GetData(ctx context.Context, query interface{}) (interface{}, error) {
	queryParams, ok := query.(models.SonarQubeQuery)
	if !ok {
		return nil, fmt.Errorf("invalid query type, expected SonarQubeQuery")
	}

	var result interface{}
	var err error

	// Execute the query with retry logic
	err = a.baseAdapter.CallWithRetry(func() error {
		// Build the request URL and query parameters based on query type
		apiPath, queryString := a.buildQueryParams(queryParams)
		requestURL := fmt.Sprintf("%s%s?%s", a.config.BaseURL, apiPath, queryString)

		// Create request
		req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
		if err != nil {
			return err
		}

		// Add authorization header
		a.addAuthHeader(req)
		req.Header.Add("Content-Type", "application/json")

		// Execute request
		resp, err := a.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode >= 400 {
			return fmt.Errorf("sonarqube API error: %s", resp.Status)
		}

		// Parse response based on query type
		switch queryParams.Type {
		case models.SonarQubeQueryTypeProject:
			var project models.SonarQubeProject
			if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
				return err
			}
			result = project

		case models.SonarQubeQueryTypeQualityGate:
			var qualityGate models.SonarQubeQualityGate
			if err := json.NewDecoder(resp.Body).Decode(&qualityGate); err != nil {
				return err
			}
			result = qualityGate

		case models.SonarQubeQueryTypeIssues:
			var issues models.SonarQubeIssues
			if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
				return err
			}
			result = issues

		case models.SonarQubeQueryTypeMetrics:
			var metrics models.SonarQubeMetrics
			if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
				return err
			}
			result = metrics

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

// buildQueryParams constructs the API endpoint and query parameters based on query parameters
func (a *Adapter) buildQueryParams(query models.SonarQubeQuery) (string, string) {
	var apiPath string
	params := url.Values{}

	switch query.Type {
	case models.SonarQubeQueryTypeProject:
		apiPath = "/api/projects/search"
		if query.ProjectKey != "" {
			params.Add("projects", query.ProjectKey)
		}
		if query.Organization != "" {
			params.Add("organization", query.Organization)
		}

	case models.SonarQubeQueryTypeQualityGate:
		if query.ProjectKey != "" {
			apiPath = "/api/qualitygates/project_status"
			params.Add("projectKey", query.ProjectKey)
		} else {
			apiPath = "/api/qualitygates/list"
		}

	case models.SonarQubeQueryTypeIssues:
		apiPath = "/api/issues/search"
		if query.ProjectKey != "" {
			params.Add("componentKeys", query.ProjectKey)
		}
		if query.Severity != "" {
			params.Add("severities", query.Severity)
		}
		if query.Status != "" {
			params.Add("statuses", query.Status)
		}

	case models.SonarQubeQueryTypeMetrics:
		apiPath = "/api/measures/component"
		if query.ProjectKey != "" {
			params.Add("component", query.ProjectKey)
		}
		if query.MetricKeys != "" {
			params.Add("metricKeys", query.MetricKeys)
		}

	default:
		apiPath = "/api/unknown"
	}

	return apiPath, params.Encode()
}

// HandleWebhook processes SonarQube webhook events
func (a *Adapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	// Verify webhook signature if secret is configured
	// This would be done at the API layer

	// Parse the event
	event, err := a.parseWebhookEvent(payload)
	if err != nil {
		return err
	}

	// Determine event type from the payload
	sonarEventType := a.determineEventType(event)

	// Notify subscribers
	a.notifySubscribers(sonarEventType, event)

	return nil
}

// parseWebhookEvent parses a SonarQube webhook event
func (a *Adapter) parseWebhookEvent(payload []byte) (interface{}, error) {
	var event models.SonarQubeWebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return event, nil
}

// determineEventType determines the event type from the payload
func (a *Adapter) determineEventType(event interface{}) string {
	webhookEvent, ok := event.(models.SonarQubeWebhookEvent)
	if !ok {
		return "unknown"
	}

	// Determine event type based on the payload
	if webhookEvent.Task != nil {
		return "task_completed"
	} else if webhookEvent.QualityGate != nil {
		return "quality_gate"
	}

	return "unknown"
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
	// Check SonarQube API status
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/api/system/status", a.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Sprintf("unhealthy: %v", err)
	}

	// Add authorization header
	a.addAuthHeader(req)

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
