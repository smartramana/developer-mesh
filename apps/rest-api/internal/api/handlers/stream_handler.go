package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	redisClient "github.com/developer-mesh/developer-mesh/pkg/redis"
	"github.com/gin-gonic/gin"
)

const (
	// Stream name for task events
	TaskEventsStream = "task_events"

	// Consumer group for SSE consumers
	SSEConsumerGroup = "sse_consumers"
)

// StreamHandler handles Server-Sent Events (SSE) endpoints
type StreamHandler struct {
	redisClient *redisClient.StreamsClient
	logger      observability.Logger
}

// NewStreamHandler creates a new handler for SSE endpoints
func NewStreamHandler(redisClient *redisClient.StreamsClient, logger observability.Logger) *StreamHandler {
	return &StreamHandler{
		redisClient: redisClient,
		logger:      logger,
	}
}

// Initialize creates the consumer group if it doesn't exist
func (h *StreamHandler) Initialize(ctx context.Context) error {
	// Create consumer group for task events (create stream if doesn't exist)
	err := h.redisClient.CreateConsumerGroupMkStream(ctx, TaskEventsStream, SSEConsumerGroup, "0")
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		h.logger.Error("Failed to create consumer group", map[string]interface{}{
			"error":  err.Error(),
			"stream": TaskEventsStream,
			"group":  SSEConsumerGroup,
		})
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	h.logger.Info("Stream handler initialized", map[string]interface{}{
		"stream": TaskEventsStream,
		"group":  SSEConsumerGroup,
	})

	return nil
}

// RegisterRoutes registers the SSE stream routes with the router
func (h *StreamHandler) RegisterRoutes(router *gin.Engine) {
	streamGroup := router.Group("/api/v1/stream")
	{
		// Task events stream
		streamGroup.GET("/tasks", h.StreamTasks)

		// Agent-specific events stream
		streamGroup.GET("/agents/:agent_id", h.StreamAgent)

		// Workflow-specific events stream
		streamGroup.GET("/workflows/:workflow_id", h.StreamWorkflow)
	}
}

// StreamTasks streams task events to the client
// @Summary Stream task events
// @Description Subscribe to real-time task events via Server-Sent Events
// @Tags streams
// @Produce text/event-stream
// @Success 200 {string} string "SSE stream"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /stream/tasks [get]
func (h *StreamHandler) StreamTasks(c *gin.Context) {
	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // Disable nginx buffering

	// Get tenant and user from context (set by auth middleware)
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	// Create context for this stream
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Create flusher for SSE
	flusher, ok := c.Writer.(gin.ResponseWriter)
	if !ok {
		c.SSEvent("error", "Streaming not supported")
		return
	}

	// Send initial connection event
	c.SSEvent("connected", gin.H{
		"message":   "Connected to task stream",
		"tenant_id": tenantID,
		"user_id":   userID,
		"timestamp": time.Now().Unix(),
	})
	flusher.Flush()

	// Create unique consumer ID for this connection
	consumerID := fmt.Sprintf("sse-%s-%d", userID, time.Now().Unix())

	// Heartbeat ticker to keep connection alive
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	h.logger.Info("SSE client connected", map[string]interface{}{
		"tenant_id":             tenantID,
		"user_id":               userID,
		"consumer_id":           consumerID,
		"redis_streams_enabled": h.redisClient != nil,
	})

	// If Redis Streams is not available, just send heartbeats
	if h.redisClient == nil {
		h.logger.Warn("Redis Streams not available, sending heartbeat-only", map[string]interface{}{
			"tenant_id": tenantID,
		})

		for {
			select {
			case <-ctx.Done():
				h.logger.Info("SSE client disconnected", map[string]interface{}{
					"tenant_id": tenantID,
				})
				return

			case <-heartbeatTicker.C:
				c.SSEvent("ping", gin.H{
					"timestamp": time.Now().Unix(),
				})
				flusher.Flush()
			}
		}
	}

	// Read ticker for consuming messages
	readTicker := time.NewTicker(1 * time.Second)
	defer readTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			h.logger.Info("SSE client disconnected", map[string]interface{}{
				"tenant_id":   tenantID,
				"consumer_id": consumerID,
			})
			return

		case <-heartbeatTicker.C:
			// Send heartbeat to keep connection alive
			c.SSEvent("ping", gin.H{
				"timestamp": time.Now().Unix(),
			})
			flusher.Flush()

		case <-readTicker.C:
			// Read from Redis Streams
			streams, err := h.redisClient.ReadFromConsumerGroup(
				ctx,
				SSEConsumerGroup,
				consumerID,
				[]string{TaskEventsStream},
				10,    // Read up to 10 messages
				0,     // Don't block
				false, // Don't auto-ack
			)

			if err != nil {
				h.logger.Error("Failed to read from stream", map[string]interface{}{
					"error": err.Error(),
				})
				continue
			}

			// Process each stream
			for _, stream := range streams {
				for _, message := range stream.Messages {
					// Extract event data
					eventType, _ := message.Values["event_type"].(string)
					tenantIDStr, _ := message.Values["tenant_id"].(string)
					payloadStr, _ := message.Values["payload"].(string)

					// Filter by tenant
					if tenantIDStr != tenantID.(string) {
						// Ack message but don't send to client
						_ = h.redisClient.AckMessages(ctx, TaskEventsStream, SSEConsumerGroup, message.ID)
						continue
					}

					// Parse payload
					var eventData map[string]interface{}
					if err := json.Unmarshal([]byte(payloadStr), &eventData); err != nil {
						h.logger.Error("Failed to parse event payload", map[string]interface{}{
							"error":      err.Error(),
							"message_id": message.ID,
						})
						// Ack the message to prevent reprocessing
						_ = h.redisClient.AckMessages(ctx, TaskEventsStream, SSEConsumerGroup, message.ID)
						continue
					}

					// Send event to client
					c.SSEvent(eventType, eventData)
					flusher.Flush()

					h.logger.Debug("Sent event to SSE client", map[string]interface{}{
						"event_type":  eventType,
						"tenant_id":   tenantID,
						"consumer_id": consumerID,
						"message_id":  message.ID,
					})

					// Acknowledge message
					if err := h.redisClient.AckMessages(ctx, TaskEventsStream, SSEConsumerGroup, message.ID); err != nil {
						h.logger.Error("Failed to ack message", map[string]interface{}{
							"error":      err.Error(),
							"message_id": message.ID,
						})
					}
				}
			}
		}
	}
}

// StreamAgent streams agent-specific events to the client
// @Summary Stream agent events
// @Description Subscribe to real-time events for a specific agent via Server-Sent Events
// @Tags streams
// @Produce text/event-stream
// @Param agent_id path string true "Agent ID"
// @Success 200 {string} string "SSE stream"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /stream/agents/{agent_id} [get]
func (h *StreamHandler) StreamAgent(c *gin.Context) {
	agentID := c.Param("agent_id")
	tenantID, _ := c.Get("tenant_id")

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// TODO: Verify agent belongs to tenant

	// Create context for this stream
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Create flusher for SSE
	flusher, ok := c.Writer.(gin.ResponseWriter)
	if !ok {
		c.SSEvent("error", "Streaming not supported")
		return
	}

	// Send initial connection event
	c.SSEvent("connected", gin.H{
		"message":   fmt.Sprintf("Connected to agent %s stream", agentID),
		"agent_id":  agentID,
		"tenant_id": tenantID,
		"timestamp": time.Now().Unix(),
	})
	flusher.Flush()

	// TODO: Subscribe to Redis Streams for agent-specific events
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			c.SSEvent("ping", gin.H{
				"timestamp": time.Now().Unix(),
			})
			flusher.Flush()
		}
	}
}

// StreamWorkflow streams workflow-specific events to the client
// @Summary Stream workflow events
// @Description Subscribe to real-time events for a specific workflow via Server-Sent Events
// @Tags streams
// @Produce text/event-stream
// @Param workflow_id path string true "Workflow ID"
// @Success 200 {string} string "SSE stream"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /stream/workflows/{workflow_id} [get]
func (h *StreamHandler) StreamWorkflow(c *gin.Context) {
	workflowID := c.Param("workflow_id")
	tenantID, _ := c.Get("tenant_id")

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// TODO: Verify workflow belongs to tenant

	// Create context for this stream
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Create flusher for SSE
	flusher, ok := c.Writer.(gin.ResponseWriter)
	if !ok {
		c.SSEvent("error", "Streaming not supported")
		return
	}

	// Send initial connection event
	c.SSEvent("connected", gin.H{
		"message":     fmt.Sprintf("Connected to workflow %s stream", workflowID),
		"workflow_id": workflowID,
		"tenant_id":   tenantID,
		"timestamp":   time.Now().Unix(),
	})
	flusher.Flush()

	// TODO: Subscribe to Redis Streams for workflow-specific events
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			c.SSEvent("ping", gin.H{
				"timestamp": time.Now().Unix(),
			})
			flusher.Flush()
		}
	}
}
