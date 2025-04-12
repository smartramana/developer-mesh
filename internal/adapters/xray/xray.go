package xray

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters"
	"github.com/S-Corkum/mcp-server/pkg/models"
)

// Config holds configuration for the JFrog Xray adapter
type Config struct {
	BaseURL        string        `mapstructure:"base_url"`
	Username       string        `mapstructure:"username"`
	Password       string        `mapstructure:"password"`
	ApiKey         string        `mapstructure:"api_key"`
	AccessToken    string        `mapstructure:"access_token"`
	WebhookSecret  string        `mapstructure:"webhook_secret"`
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
	MaxRetries     int           `mapstructure:"max_retries"`
	RetryDelay     time.Duration `mapstructure:"retry_delay"`
}

// Adapter implements the JFrog Xray integration
type Adapter struct {
	client       *http.Client
	config       Config
	subscribers  map[string][]func(interface{})
	subscriberMu sync.RWMutex
	baseAdapter  adapters.BaseAdapter
}

// NewAdapter creates a new JFrog Xray adapter
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
	// Test connection by calling Xray API
	return a.baseAdapter.CallWithRetry(func() error {
		return a.testConnection(ctx)
	})
}

// testConnection verifies that we can connect to the Xray API
func (a *Adapter) testConnection(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/system/ping", a.config.BaseURL)
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
		return fmt.Errorf("xray API error: status code %d", resp.StatusCode)
	}

	return nil
}

// addAuthHeader adds authentication header to the request
func (a *Adapter) addAuthHeader(req *http.Request) {
	if a.config.AccessToken != "" {
		// Use Bearer token authentication
		req.Header.Add("Authorization", "Bearer "+a.config.AccessToken)
	} else if a.config.ApiKey != "" {
		// Use API key authentication
		req.Header.Add("X-JFrog-Art-Api", a.config.ApiKey)
	} else {
		// Use basic authentication
		auth := a.config.Username + ":" + a.config.Password
		encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
		req.Header.Add("Authorization", "Basic "+encodedAuth)
	}
}

// GetData retrieves data from JFrog Xray
func (a *Adapter) GetData(ctx context.Context, query interface{}) (interface{}, error) {
	queryParams, ok := query.(models.XrayQuery)
	if !ok {
		return nil, fmt.Errorf("invalid query type, expected XrayQuery")
	}

	var result interface{}
	var err error

	// Execute the query with retry logic
	err = a.baseAdapter.CallWithRetry(func() error {
		var req *http.Request
		var err error

		switch queryParams.Type {
		case models.XrayQueryTypeSummary:
			// Summary API requires POST with JSON body
			req, err = a.createSummaryRequest(ctx, queryParams)
		case models.XrayQueryTypeVulnerabilities:
			// Vulnerabilities query
			req, err = a.createVulnerabilitiesRequest(ctx, queryParams)
		case models.XrayQueryTypeLicenses:
			// Licenses query
			req, err = a.createLicensesRequest(ctx, queryParams)
		case models.XrayQueryTypeScans:
			// Scans query
			req, err = a.createScansRequest(ctx, queryParams)
		default:
			return fmt.Errorf("unsupported query type: %s", queryParams.Type)
		}

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
			bodyBytes, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("xray API error: %s - %s", resp.Status, string(bodyBytes))
		}

		// Parse response based on query type
		switch queryParams.Type {
		case models.XrayQueryTypeSummary:
			var summary models.XraySummary
			if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
				return err
			}
			result = summary

		case models.XrayQueryTypeVulnerabilities:
			var vulnerabilities models.XrayVulnerabilities
			if err := json.NewDecoder(resp.Body).Decode(&vulnerabilities); err != nil {
				return err
			}
			result = vulnerabilities

		case models.XrayQueryTypeLicenses:
			var licenses models.XrayLicenses
			if err := json.NewDecoder(resp.Body).Decode(&licenses); err != nil {
				return err
			}
			result = licenses

		case models.XrayQueryTypeScans:
			var scans models.XrayScans
			if err := json.NewDecoder(resp.Body).Decode(&scans); err != nil {
				return err
			}
			result = scans
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// createSummaryRequest creates a request for the summary API
func (a *Adapter) createSummaryRequest(ctx context.Context, query models.XrayQuery) (*http.Request, error) {
	url := fmt.Sprintf("%s/api/v1/summary/artifact", a.config.BaseURL)

	// Build request body
	requestBody := map[string]interface{}{
		"component_details": map[string]string{
			"component_id": query.ArtifactPath,
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	return http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(bodyBytes)))
}

// createVulnerabilitiesRequest creates a request for the vulnerabilities API
func (a *Adapter) createVulnerabilitiesRequest(ctx context.Context, query models.XrayQuery) (*http.Request, error) {
	url := fmt.Sprintf("%s/api/v1/vulnerabilities", a.config.BaseURL)

	if query.CVE != "" {
		url = fmt.Sprintf("%s/%s", url, query.CVE)
	}

	return http.NewRequestWithContext(ctx, "GET", url, nil)
}

// createLicensesRequest creates a request for the licenses API
func (a *Adapter) createLicensesRequest(ctx context.Context, query models.XrayQuery) (*http.Request, error) {
	url := fmt.Sprintf("%s/api/v1/licenses", a.config.BaseURL)

	if query.LicenseID != "" {
		url = fmt.Sprintf("%s/%s", url, query.LicenseID)
	}

	return http.NewRequestWithContext(ctx, "GET", url, nil)
}

// createScansRequest creates a request for the scans API
func (a *Adapter) createScansRequest(ctx context.Context, query models.XrayQuery) (*http.Request, error) {
	url := fmt.Sprintf("%s/api/v1/scans", a.config.BaseURL)

	// Build request body for scan query
	requestBody := map[string]interface{}{
		"artifact_path": query.ArtifactPath,
	}

	if query.BuildName != "" && query.BuildNumber != "" {
		requestBody["build_name"] = query.BuildName
		requestBody["build_number"] = query.BuildNumber
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	return http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(bodyBytes)))
}

// HandleWebhook processes JFrog Xray webhook events
func (a *Adapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	// Verify webhook signature if secret is configured
	// This would be done at the API layer

	// Parse the event
	event, err := a.parseWebhookEvent(payload)
	if err != nil {
		return err
	}

	// Determine event type from the payload
	xrayEventType := a.determineEventType(event)

	// Notify subscribers
	a.notifySubscribers(xrayEventType, event)

	return nil
}

// parseWebhookEvent parses a JFrog Xray webhook event
func (a *Adapter) parseWebhookEvent(payload []byte) (interface{}, error) {
	var event models.XrayWebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return event, nil
}

// determineEventType determines the event type from the payload
func (a *Adapter) determineEventType(event interface{}) string {
	webhookEvent, ok := event.(models.XrayWebhookEvent)
	if !ok {
		return "unknown"
	}

	return webhookEvent.EventType
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
	// Check JFrog Xray API status
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/api/v1/system/ping", a.config.BaseURL)
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
