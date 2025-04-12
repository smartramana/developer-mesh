package artifactory

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters"
	"github.com/S-Corkum/mcp-server/pkg/models"
)

// Config holds configuration for the Artifactory adapter
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

// Adapter implements the Artifactory integration
type Adapter struct {
	client       *http.Client
	config       Config
	subscribers  map[string][]func(interface{})
	subscriberMu sync.RWMutex
	baseAdapter  adapters.BaseAdapter
}

// NewAdapter creates a new Artifactory adapter
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
	// Test connection by calling Artifactory API
	return a.baseAdapter.CallWithRetry(func() error {
		return a.testConnection(ctx)
	})
}

// testConnection verifies that we can connect to the Artifactory API
func (a *Adapter) testConnection(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/system/ping", a.config.BaseURL)
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
		return fmt.Errorf("artifactory API error: status code %d", resp.StatusCode)
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

// GetData retrieves data from Artifactory
func (a *Adapter) GetData(ctx context.Context, query interface{}) (interface{}, error) {
	queryParams, ok := query.(models.ArtifactoryQuery)
	if !ok {
		return nil, fmt.Errorf("invalid query type, expected ArtifactoryQuery")
	}

	var result interface{}
	var err error

	// Execute the query with retry logic
	err = a.baseAdapter.CallWithRetry(func() error {
		// Build the request URL based on query type
		apiPath, queryString := a.buildQueryParams(queryParams)
		requestURL := fmt.Sprintf("%s%s?%s", a.config.BaseURL, apiPath, queryString)

		// Create request
		req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
		if err != nil {
			return err
		}

		// Add headers
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
			return fmt.Errorf("artifactory API error: %s - %s", resp.Status, string(bodyBytes))
		}

		// Parse response based on query type
		switch queryParams.Type {
		case models.ArtifactoryQueryTypeRepository:
			var repositories models.ArtifactoryRepositories
			if err := json.NewDecoder(resp.Body).Decode(&repositories); err != nil {
				return err
			}
			result = repositories

		case models.ArtifactoryQueryTypeArtifact:
			var artifact models.ArtifactoryArtifact
			if err := json.NewDecoder(resp.Body).Decode(&artifact); err != nil {
				return err
			}
			result = artifact

		case models.ArtifactoryQueryTypeBuild:
			var build models.ArtifactoryBuild
			if err := json.NewDecoder(resp.Body).Decode(&build); err != nil {
				return err
			}
			result = build

		case models.ArtifactoryQueryTypeStorage:
			var storage models.ArtifactoryStorage
			if err := json.NewDecoder(resp.Body).Decode(&storage); err != nil {
				return err
			}
			result = storage

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
func (a *Adapter) buildQueryParams(query models.ArtifactoryQuery) (string, string) {
	var apiPath string
	params := url.Values{}

	switch query.Type {
	case models.ArtifactoryQueryTypeRepository:
		apiPath = "/api/repositories"
		if query.RepoType != "" {
			params.Add("type", query.RepoType)
		}
		if query.PackageType != "" {
			params.Add("packageType", query.PackageType)
		}

	case models.ArtifactoryQueryTypeArtifact:
		apiPath = fmt.Sprintf("/api/storage/%s/%s", query.RepoKey, query.Path)
		params.Add("stats", "1")
		params.Add("properties", "1")

	case models.ArtifactoryQueryTypeBuild:
		if query.BuildName != "" && query.BuildNumber != "" {
			apiPath = fmt.Sprintf("/api/build/%s/%s", query.BuildName, query.BuildNumber)
		} else {
			apiPath = "/api/build"
		}

	case models.ArtifactoryQueryTypeStorage:
		apiPath = "/api/storageinfo"

	default:
		apiPath = "/api/unknown"
	}

	return apiPath, params.Encode()
}

// DownloadArtifact downloads an artifact from Artifactory
func (a *Adapter) DownloadArtifact(ctx context.Context, repoKey, path string) ([]byte, error) {
	var data []byte
	var err error

	// Execute with retry logic
	err = a.baseAdapter.CallWithRetry(func() error {
		// Build the request URL
		requestURL := fmt.Sprintf("%s/%s/%s", a.config.BaseURL, repoKey, path)

		// Create request
		req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
		if err != nil {
			return err
		}

		// Add authorization header
		a.addAuthHeader(req)

		// Execute request
		resp, err := a.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode >= 400 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("artifactory download error: %s - %s", resp.Status, string(bodyBytes))
		}

		// Read response body
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return data, nil
}

// HandleWebhook processes Artifactory webhook events
func (a *Adapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	// Verify webhook signature if secret is configured
	// This would be done at the API layer

	// Parse the event
	event, err := a.parseWebhookEvent(payload)
	if err != nil {
		return err
	}

	// Determine event type from the payload
	artifactoryEventType := a.determineEventType(event)

	// Notify subscribers
	a.notifySubscribers(artifactoryEventType, event)

	return nil
}

// parseWebhookEvent parses an Artifactory webhook event
func (a *Adapter) parseWebhookEvent(payload []byte) (interface{}, error) {
	var event models.ArtifactoryWebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return event, nil
}

// determineEventType determines the event type from the payload
func (a *Adapter) determineEventType(event interface{}) string {
	webhookEvent, ok := event.(models.ArtifactoryWebhookEvent)
	if !ok {
		return "unknown"
	}

	return webhookEvent.Domain
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
	// Check Artifactory API status
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/api/system/ping", a.config.BaseURL)
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
