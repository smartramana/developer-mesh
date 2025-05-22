package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// WebhookEvent represents a GitHub webhook event
type WebhookEvent struct {
	ID        string
	Type      string
	Payload   []byte
	Timestamp time.Time
	Headers   map[string]string
}

// WebhookHandler defines the interface for handling webhook events
type WebhookHandler interface {
	// HandleEvent handles a webhook event
	HandleEvent(ctx context.Context, event *WebhookEvent) error
}

// Processor processes webhook events asynchronously
type Processor struct {
	handlers    map[string][]WebhookHandler
	queue       chan *WebhookEvent
	workerCount int
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	logger      observability.Logger
	metrics     observability.MetricsClient
}

// ProcessorConfig holds configuration for the webhook processor
type ProcessorConfig struct {
	QueueSize   int
	WorkerCount int
	Logger      observability.Logger
	Metrics     observability.MetricsClient
}

// NewProcessor creates a new webhook processor
func NewProcessor(config *ProcessorConfig) *Processor {
	// Set defaults if not provided
	queueSize := config.QueueSize
	if queueSize <= 0 {
		queueSize = 100
	}

	workerCount := config.WorkerCount
	if workerCount <= 0 {
		workerCount = 5
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Processor{
		handlers:    make(map[string][]WebhookHandler),
		queue:       make(chan *WebhookEvent, queueSize),
		workerCount: workerCount,
		ctx:         ctx,
		cancel:      cancel,
		logger:      config.Logger,
		metrics:     config.Metrics,
	}
}

// Start starts the webhook processor
func (p *Processor) Start() {
	p.logger.Info("Starting webhook processor", map[string]interface{}{
		"workers": p.workerCount,
	})

	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Stop stops the webhook processor
func (p *Processor) Stop() {
	p.logger.Info("Stopping webhook processor", map[string]interface{}{})
	p.cancel()
	close(p.queue)
	p.wg.Wait()
}

// RegisterHandler registers a webhook handler for a specific event type
func (p *Processor) RegisterHandler(eventType string, handler WebhookHandler) {
	p.handlers[eventType] = append(p.handlers[eventType], handler)
}

// ProcessEvent processes a webhook event asynchronously
func (p *Processor) ProcessEvent(event *WebhookEvent) error {
	// Check if we have handlers for this event type
	if _, ok := p.handlers[event.Type]; !ok && len(p.handlers["*"]) == 0 {
		// No handlers for this event type
		p.logger.Debug("No handlers for event type", map[string]interface{}{
			"type": event.Type,
		})
		return nil
	}

	// Log incoming event
	p.logger.Debug("Queuing webhook event", map[string]interface{}{
		"id": event.ID,
		"type": event.Type,
		"queue_length": len(p.queue),
	})
	
	// Record metric for queue size
	p.metrics.RecordGauge("github.webhook.queue_size", float64(len(p.queue)), map[string]string{
		"event_type": event.Type,
	})

	// Add event to queue with timeout
	select {
	case p.queue <- event:
		// Successfully added to queue
		return nil
	case <-time.After(5 * time.Second):
		// Queue is full or blocked
		p.metrics.RecordCounter("github.webhook.queue_timeout", 1, map[string]string{
			"event_type": event.Type,
		})
		return fmt.Errorf("webhook queue is full")
	}
}

// worker processes webhook events from the queue
func (p *Processor) worker(id int) {
	defer p.wg.Done()

	p.logger.Debug("Starting webhook worker", map[string]interface{}{
		"id": id,
	})

	for {
		select {
		case event, ok := <-p.queue:
			if !ok {
				// Channel closed, exit worker
				return
			}

			// Process the event
			p.handleEvent(event)

		case <-p.ctx.Done():
			// Context cancelled, exit worker
			return
		}
	}
}

// handleEvent handles a webhook event
func (p *Processor) handleEvent(event *WebhookEvent) {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		p.metrics.RecordHistogram("github.webhook.processing_time", duration.Seconds(), map[string]string{
			"event_type": event.Type,
		})
	}()

	// Create a context with timeout for event processing
	ctx, cancel := context.WithTimeout(p.ctx, 30*time.Second)
	defer cancel()

	// Log event processing
	p.logger.Debug("Processing webhook event", map[string]interface{}{
		"id": event.ID,
		"type": event.Type,
	})

	// Track if the event was handled by any handler
	handled := false

	// Call event-specific handlers
	if handlers, ok := p.handlers[event.Type]; ok {
		for _, handler := range handlers {
			err := handler.HandleEvent(ctx, event)
			if err != nil {
				p.logger.Error("Error handling webhook event", map[string]interface{}{
					"id": event.ID,
					"type": event.Type,
					"error": err.Error(),
				})
				p.metrics.RecordCounter("github.webhook.error", 1, map[string]string{
					"event_type": event.Type,
				})
			} else {
				handled = true
			}
		}
	}

	// Call wildcard handlers
	if handlers, ok := p.handlers["*"]; ok {
		for _, handler := range handlers {
			err := handler.HandleEvent(ctx, event)
			if err != nil {
				p.logger.Error("Error handling webhook event with wildcard handler", map[string]interface{}{
					"id": event.ID,
					"type": event.Type,
					"error": err.Error(),
				})
				p.metrics.RecordCounter("github.webhook.error", 1, map[string]string{
					"event_type": event.Type,
					"handler": "wildcard",
				})
			} else {
				handled = true
			}
		}
	}

	// Log event completion
	if handled {
		p.logger.Debug("Webhook event processed successfully", map[string]interface{}{
			"id": event.ID,
			"type": event.Type,
			"duration": time.Since(startTime).String(),
		})
		p.metrics.RecordCounter("github.webhook.processed", 1, map[string]string{
			"event_type": event.Type,
		})
	} else {
		p.logger.Warn("No handlers processed webhook event", map[string]interface{}{
			"id": event.ID,
			"type": event.Type,
		})
		p.metrics.RecordCounter("github.webhook.unhandled", 1, map[string]string{
			"event_type": event.Type,
		})
	}
}

// DefaultEventHandler implements a simple webhook handler
type DefaultEventHandler struct {
	logger  observability.Logger
	metrics observability.MetricsClient
}

// NewDefaultEventHandler creates a new default webhook handler
func NewDefaultEventHandler(logger observability.Logger, metrics observability.MetricsClient) *DefaultEventHandler {
	return &DefaultEventHandler{
		logger:  logger,
		metrics: metrics,
	}
}

// HandleEvent handles a webhook event
func (h *DefaultEventHandler) HandleEvent(ctx context.Context, event *WebhookEvent) error {
	// Log event information
	h.logger.Info("Handling webhook event", map[string]interface{}{
		"id": event.ID,
		"type": event.Type,
	})

	// Parse payload
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	// Process event based on type
	switch event.Type {
	case "push":
		return h.handlePushEvent(ctx, event, payload)
	case "pull_request":
		return h.handlePullRequestEvent(ctx, event, payload)
	case "issues":
		return h.handleIssueEvent(ctx, event, payload)
	default:
		h.logger.Info("Received webhook event", map[string]interface{}{
			"id": event.ID,
			"type": event.Type,
		})
	}

	return nil
}

// handlePushEvent handles a push event
func (h *DefaultEventHandler) handlePushEvent(ctx context.Context, event *WebhookEvent, payload map[string]interface{}) error {
	ref, _ := payload["ref"].(string)
	repo, _ := payload["repository"].(map[string]interface{})
	repoName, _ := repo["full_name"].(string)

	h.logger.Info("Received push event", map[string]interface{}{
		"id": event.ID,
		"repository": repoName,
		"ref": ref,
	})

	h.metrics.RecordCounter("github.webhook.push", 1, map[string]string{
		"repository": repoName,
	})

	return nil
}

// handlePullRequestEvent handles a pull request event
func (h *DefaultEventHandler) handlePullRequestEvent(ctx context.Context, event *WebhookEvent, payload map[string]interface{}) error {
	action, _ := payload["action"].(string)
	pr, _ := payload["pull_request"].(map[string]interface{})
	number, _ := pr["number"].(float64)
	title, _ := pr["title"].(string)
	repo, _ := payload["repository"].(map[string]interface{})
	repoName, _ := repo["full_name"].(string)

	h.logger.Info("Received pull request event", map[string]interface{}{
		"id": event.ID,
		"repository": repoName,
		"number": number,
		"title": title,
		"action": action,
	})

	h.metrics.RecordCounter("github.webhook.pull_request", 1, map[string]string{
		"repository": repoName,
		"action": action,
	})

	return nil
}

// handleIssueEvent handles an issue event
func (h *DefaultEventHandler) handleIssueEvent(ctx context.Context, event *WebhookEvent, payload map[string]interface{}) error {
	action, _ := payload["action"].(string)
	issue, _ := payload["issue"].(map[string]interface{})
	number, _ := issue["number"].(float64)
	title, _ := issue["title"].(string)
	repo, _ := payload["repository"].(map[string]interface{})
	repoName, _ := repo["full_name"].(string)

	h.logger.Info("Received issue event", map[string]interface{}{
		"id": event.ID,
		"repository": repoName,
		"number": number,
		"title": title,
		"action": action,
	})

	h.metrics.RecordCounter("github.webhook.issue", 1, map[string]string{
		"repository": repoName,
		"action": action,
	})

	return nil
}
