package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/gin-gonic/gin"
)

// AgentWebhookConfig holds configuration for the agent webhook
type AgentWebhookConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Secret  string `mapstructure:"secret"`
}

// agentWebhookHandler processes webhooks from AI agents
func (s *Server) agentWebhookHandler(c *gin.Context) {
	// Read the payload
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	// Reset the request body for potential future middleware
	c.Request.Body = io.NopCloser(bytes.NewBuffer(payload))

	// Verify signature if configured
	if s.config.AgentWebhook.Secret != "" {
		signature := c.GetHeader("X-MCP-Signature")
		if signature == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing signature header"})
			return
		}

		// Calculate expected signature
		mac := hmac.New(sha256.New, []byte(s.config.AgentWebhook.Secret))
		mac.Write(payload)
		expectedSignature := hex.EncodeToString(mac.Sum(nil))

		// Use constant-time comparison to prevent timing attacks
		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
			return
		}
	}

	// Parse the event
	var event mcp.Event
	if err := json.Unmarshal(payload, &event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event format"})
		return
	}

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Process the event based on type
	switch event.Type {
	case "context_update":
		err = s.handleContextUpdateEvent(c, event)
	case "agent_status":
		err = s.handleAgentStatusEvent(c, event)
	case "conversation_complete":
		err = s.handleConversationCompleteEvent(c, event)
	default:
		// For unknown events, just log them
		s.engine.events <- event
		err = nil
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to process event: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "processed"})
}

// handleContextUpdateEvent processes a context update event
func (s *Server) handleContextUpdateEvent(c *gin.Context, event mcp.Event) error {
	// Parse context update data
	var updateData struct {
		ContextID string          `json:"context_id"`
		Context   *mcp.Context    `json:"context"`
		Options   map[string]interface{} `json:"options"`
	}

	if data, ok := event.Data.(map[string]interface{}); ok {
		// Extract context_id
		if contextID, ok := data["context_id"].(string); ok {
			updateData.ContextID = contextID
		} else {
			return fmt.Errorf("missing context_id in event data")
		}

		// Extract context
		if contextData, ok := data["context"].(map[string]interface{}); ok {
			// Convert to Context struct
			contextJSON, err := json.Marshal(contextData)
			if err != nil {
				return err
			}

			var context mcp.Context
			if err := json.Unmarshal(contextJSON, &context); err != nil {
				return err
			}
			updateData.Context = &context
		} else {
			return fmt.Errorf("missing context in event data")
		}

		// Extract options
		if options, ok := data["options"].(map[string]interface{}); ok {
			updateData.Options = options
		}
	} else {
		return fmt.Errorf("invalid event data format")
	}

	// Convert options to ContextUpdateOptions
	var options mcp.ContextUpdateOptions
	if updateData.Options != nil {
		if truncate, ok := updateData.Options["truncate"].(bool); ok {
			options.Truncate = truncate
		}
		if strategy, ok := updateData.Options["truncate_strategy"].(string); ok {
			options.TruncateStrategy = strategy
		}
		if params, ok := updateData.Options["relevance_parameters"].(map[string]interface{}); ok {
			options.RelevanceParameters = params
		}
	}

	// Update context
	_, err := s.engine.ContextManager.UpdateContext(c.Request.Context(), updateData.ContextID, updateData.Context, &options)
	return err
}

// handleAgentStatusEvent processes an agent status event
func (s *Server) handleAgentStatusEvent(c *gin.Context, event mcp.Event) error {
	// Log agent status
	s.engine.events <- event
	return nil
}

// handleConversationCompleteEvent processes a conversation complete event
func (s *Server) handleConversationCompleteEvent(c *gin.Context, event mcp.Event) error {
	// Parse context data
	var completeData struct {
		ContextID string `json:"context_id"`
		Summary   string `json:"summary,omitempty"`
	}

	if data, ok := event.Data.(map[string]interface{}); ok {
		if contextID, ok := data["context_id"].(string); ok {
			completeData.ContextID = contextID
		} else {
			return fmt.Errorf("missing context_id in event data")
		}

		if summary, ok := data["summary"].(string); ok {
			completeData.Summary = summary
		}
	}

	// Get the context
	context, err := s.engine.ContextManager.GetContext(c.Request.Context(), completeData.ContextID)
	if err != nil {
		return err
	}

	// Update metadata to mark as completed
	if context.Metadata == nil {
		context.Metadata = make(map[string]interface{})
	}
	context.Metadata["completed"] = true
	context.Metadata["completed_at"] = time.Now()

	// Add summary if provided
	if completeData.Summary != "" {
		context.Metadata["summary"] = completeData.Summary
	}

	// Update context
	_, err = s.engine.ContextManager.UpdateContext(c.Request.Context(), completeData.ContextID, context, nil)
	return err
}
