package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/S-Corkum/devops-mcp/pkg/auth"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

// Define context key types to avoid collisions
type contextKey string

const (
	contextKeyTenantID  contextKey = "tenant_id"
	contextKeyUserID    contextKey = "user_id"
	contextKeyClaims    contextKey = "claims"
	contextKeyRequestID contextKey = "request_id"
	contextKeyMethod    contextKey = "method"
)

// MessageHandler processes a specific message type
type MessageHandler func(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error)

// RegisterHandlers sets up all message handlers
func (s *Server) RegisterHandlers() {
	s.handlers = map[string]MessageHandler{
		// Core protocol
		"initialize":          s.handleInitialize,
		"protocol.set_binary": s.handleSetBinaryProtocol,
		"protocol.get_info":   s.handleProtocolGetInfo,

		// Testing and diagnostics
		"echo":      s.handleEcho,
		"ping":      s.handlePing,
		"benchmark": s.handleBenchmark,

		// Tool operations
		"tool.list":    s.handleToolList,
		"tool.execute": s.handleToolExecute,
		"tool.cancel":  s.handleToolCancel,

		// Context management
		"context.create":     s.handleContextCreate,
		"context.get":        s.handleContextGet,
		"context.update":     s.handleContextUpdate,
		"context.append":     s.handleContextAppend,
		"context.get_limits": s.handleContextGetLimits,
		"context.get_stats":  s.handleContextGetStats,
		"context.truncate":   s.handleContextTruncate,

		// Context window management
		"window.setTokens":     s.handleWindowSetTokens,
		"window.getTokenUsage": s.handleWindowGetTokenUsage,

		// Session management
		"session.create":       s.handleSessionCreate,
		"session.get":          s.handleSessionGet,
		"session.update_state": s.handleSessionUpdateState,
		"session.add_message":  s.handleSessionAddMessage,
		"session.get_history":  s.handleSessionGetHistory,
		"session.branch":       s.handleSessionBranch,
		"session.recover":      s.handleSessionRecover,
		"session.export":       s.handleSessionExport,
		"session.list":         s.handleSessionList,
		"session.set_active":   s.handleSessionSetActive,
		"session.get_metrics":  s.handleSessionGetMetrics,

		// Subscription management
		"subscribe":            s.handleSubscribe,
		"unsubscribe":          s.handleUnsubscribe,
		"subscription.list":    s.handleSubscriptionList,
		"subscription.status":  s.handleSubscriptionStatus,
		"subscription.restore": s.handleSubscriptionRestore,
		"event.subscribe":      s.handleEventSubscribe,
		"event.unsubscribe":    s.handleEventUnsubscribe,

		// Workflow management
		"workflow.create":                s.handleWorkflowCreate,
		"workflow.create_collaborative":  s.handleWorkflowCreateCollaborative,
		"workflow.execute":               s.handleWorkflowExecute,
		"workflow.execute_collaborative": s.handleWorkflowExecuteCollaborative,
		"workflow.status":                s.handleWorkflowStatus,
		"workflow.cancel":                s.handleWorkflowCancel,
		"workflow.list":                  s.handleWorkflowList,
		"workflow.get":                   s.handleWorkflowGet,
		"workflow.resume":                s.handleWorkflowResume,
		"workflow.complete_task":         s.handleWorkflowCompleteTask,

		// Agent management
		"agent.register":      s.handleAgentRegister,
		"agent.discover":      s.handleAgentDiscover,
		"agent.delegate":      s.handleAgentDelegate,
		"agent.collaborate":   s.handleAgentCollaborate,
		"agent.status":        s.handleAgentStatus,
		"agent.update_status": s.handleAgentUpdateStatus,

		// Task management
		"task.create":             s.handleTaskCreate,
		"task.create_auto_assign": s.handleTaskCreateAutoAssign,
		"task.create_distributed": s.handleTaskCreateDistributed,
		"task.status":             s.handleTaskStatus,
		"task.cancel":             s.handleTaskCancel,
		"task.list":               s.handleTaskList,
		"task.delegate":           s.handleTaskDelegate,
		"task.accept":             s.handleTaskAccept,
		"task.complete":           s.handleTaskComplete,
		"task.fail":               s.handleTaskFail,
		"task.submit_result":      s.handleTaskSubmitResult,

		// Workspace management
		"workspace.create":       s.handleWorkspaceCreate,
		"workspace.join":         s.handleWorkspaceJoin,
		"workspace.leave":        s.handleWorkspaceLeave,
		"workspace.broadcast":    s.handleWorkspaceBroadcast,
		"workspace.list_members": s.handleWorkspaceListMembers,
		"workspace.get_state":    s.handleWorkspaceGetState,
		"workspace.update_state": s.handleWorkspaceUpdateState,

		// Document management
		"document.create_shared": s.handleDocumentCreateShared,
		"document.update":        s.handleDocumentUpdate,
		"document.apply_change":  s.handleDocumentApplyChange,

		// Streaming
		"stream.binary": s.handleStreamBinary,

		// Metrics
		"metrics.record": s.handleMetricsRecord,

		// Conflict Resolution
		"document.sync":       s.handleDocumentSync,
		"workspace.sync":      s.handleWorkspaceStateSync,
		"conflict.detect":     s.handleConflictDetect,
		"vector_clock.get":    s.handleVectorClockGet,
		"vector_clock.update": s.handleVectorClockUpdate,
	}
}

// Handler dependencies are already integrated into the Server struct in server.go:
// - handlers map[string]MessageHandler
// - toolRegistry ToolRegistry
// - contextManager ContextManager
// - eventBus EventBus

// processMessage handles incoming WebSocket messages
func (s *Server) processMessage(ctx context.Context, conn *Connection, msg *ws.Message) ([]byte, error) {
	// Validate message
	if msg.Type != ws.MessageTypeRequest {
		return nil, fmt.Errorf("invalid message type: %d", msg.Type)
	}

	// Validate message ID
	if msg.ID == "" {
		return s.createErrorResponse("", ws.ErrCodeInvalidMessage, "Message ID is required")
	}

	// Validate method
	if msg.Method == "" {
		return s.createErrorResponse(msg.ID, ws.ErrCodeInvalidMessage, "Method is required")
	}

	// Get handler
	handler, ok := s.handlers[msg.Method]
	if !ok {
		return s.createErrorResponse(msg.ID, ws.ErrCodeMethodNotFound, "Method not found")
	}

	// Check authorization
	if conn.state != nil && conn.state.Claims != nil {
		// Add claims to context using auth package functions
		ctx = auth.WithTenantID(ctx, uuid.MustParse(conn.state.Claims.TenantID))
		ctx = auth.WithUserID(ctx, conn.state.Claims.UserID)
		ctx = context.WithValue(ctx, contextKeyClaims, conn.state.Claims)
		
		// Debug logging
		s.logger.Info("Context enriched with auth", map[string]interface{}{
			"user_id":   conn.state.Claims.UserID,
			"tenant_id": conn.state.Claims.TenantID,
			"method":    msg.Method,
		})

		// Check method-specific permissions
		if err := s.checkMethodPermission(conn.state.Claims, msg.Method); err != nil {
			s.logger.Warn("Authorization failed", map[string]interface{}{
				"method":  msg.Method,
				"user_id": conn.state.Claims.UserID,
				"error":   err.Error(),
			})
			return s.createErrorResponse(msg.ID, ws.ErrCodeAuthFailed, "Unauthorized")
		}
	} else if s.config.Security.RequireAuth {
		// If auth is required but no claims present
		return s.createErrorResponse(msg.ID, ws.ErrCodeAuthFailed, "Authentication required")
	}

	// Convert params to json.RawMessage if needed
	var params json.RawMessage
	if msg.Params != nil {
		paramBytes, err := json.Marshal(msg.Params)
		if err != nil {
			return s.createErrorResponse(msg.ID, ws.ErrCodeInvalidParams, "Invalid parameters")
		}
		params = paramBytes
	}

	// Add request metadata to context
	ctx = context.WithValue(ctx, contextKeyRequestID, msg.ID)
	ctx = context.WithValue(ctx, contextKeyMethod, msg.Method)

	// Record method call metric
	if s.metricsCollector != nil {
		start := time.Now()
		defer func() {
			s.metricsCollector.RecordMessage("processed", msg.Method, conn.TenantID, time.Since(start))
		}()
	}

	// Execute handler
	result, err := handler(ctx, conn, params)
	if err != nil {
		s.logger.Error("Handler error", map[string]interface{}{
			"method":        msg.Method,
			"error":         err.Error(),
			"connection_id": conn.ID,
		})
		return s.createErrorResponse(msg.ID, ws.ErrCodeServerError, err.Error())
	}

	// Create response using pooled object
	response := GetMessage()
	defer PutMessage(response)

	response.ID = msg.ID
	response.Type = ws.MessageTypeResponse
	response.Result = result

	return json.Marshal(response)
}

// checkMethodPermission checks if the user has permission to call a method
func (s *Server) checkMethodPermission(claims *auth.Claims, method string) error {
	// Define method permission mappings
	readOnlyMethods := map[string]bool{
		"echo":                   true,
		"ping":                   true,
		"protocol.get_info":      true,
		"context.get":            true,
		"context.get_limits":     true,
		"context.get_stats":      true,
		"tool.list":              true,
		"session.get":            true,
		"session.get_history":    true,
		"session.list":           true,
		"subscription.list":      true,
		"subscription.status":    true,
		"workflow.status":        true,
		"workflow.list":          true,
		"workflow.get":           true,
		"agent.status":           true,
		"task.status":            true,
		"task.list":              true,
		"workspace.list_members": true,
		"workspace.get_state":    true,
		"window.getTokenUsage":   true,
		"session.get_metrics":    true,
		"vector_clock.get":       true,
	}

	adminOnlyMethods := map[string]bool{
		"agent.register": true,
		"metrics.record": true,
	}

	// Check admin-only methods
	if adminOnlyMethods[method] {
		for _, scope := range claims.Scopes {
			if scope == "admin" {
				return nil
			}
		}
		return fmt.Errorf("admin permission required for method: %s", method)
	}

	// Check if user has write permission for write methods
	if !readOnlyMethods[method] {
		// Check for write scope
		hasWriteScope := false
		for _, scope := range claims.Scopes {
			if scope == "write" || scope == "admin" {
				hasWriteScope = true
				break
			}
		}
		if !hasWriteScope {
			return fmt.Errorf("write permission required for method: %s", method)
		}
	}

	return nil
}

// createErrorResponse creates an error response message
func (s *Server) createErrorResponse(id string, code int, message string) ([]byte, error) {
	response := GetMessage()
	defer PutMessage(response)

	response.ID = id
	response.Type = ws.MessageTypeError
	response.Error = &ws.Error{
		Code:    code,
		Message: message,
	}

	return json.Marshal(response)
}

// Protocol handlers

// handleProtocolGetInfo returns protocol information
func (s *Server) handleProtocolGetInfo(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"version": "1.0.0",
		"capabilities": []string{
			"binary_protocol",
			"streaming",
			"collaboration",
			"conflict_resolution",
		},
		"binary_enabled": conn.IsBinaryMode(),
	}, nil
}

// Testing and diagnostic handlers

// handleEcho echoes back the input
func (s *Server) handleEcho(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var echoParams map[string]interface{}
	if err := json.Unmarshal(params, &echoParams); err != nil {
		return nil, err
	}
	return echoParams, nil
}

// handlePing responds to ping requests
func (s *Server) handlePing(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"pong":      true,
		"timestamp": time.Now().Unix(),
	}, nil
}

// handleBenchmark performs a benchmark test
func (s *Server) handleBenchmark(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var benchParams struct {
		Iterations int `json:"iterations"`
		DataSize   int `json:"data_size"`
	}

	if err := json.Unmarshal(params, &benchParams); err != nil {
		benchParams.Iterations = 1000
		benchParams.DataSize = 1024
	}

	start := time.Now()

	// Simulate some work
	data := make([]byte, benchParams.DataSize)
	for i := 0; i < benchParams.Iterations; i++ {
		_ = data
	}

	duration := time.Since(start)

	return map[string]interface{}{
		"iterations":  benchParams.Iterations,
		"data_size":   benchParams.DataSize,
		"duration_ms": duration.Milliseconds(),
		"ops_per_sec": float64(benchParams.Iterations) / duration.Seconds(),
	}, nil
}

// handleInitialize handles the initialize method
func (s *Server) handleInitialize(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var initParams struct {
		Version      string   `json:"version"`
		Name         string   `json:"name"`
		Capabilities []string `json:"capabilities"`
	}

	if err := json.Unmarshal(params, &initParams); err != nil {
		return nil, err
	}

	// Store agent capabilities if provided
	if len(initParams.Capabilities) > 0 && s.agentRegistry != nil {
		s.logger.Debug("Registering agent with capabilities", map[string]interface{}{
			"agent_id":     conn.AgentID,
			"agent_name":   initParams.Name,
			"capabilities": initParams.Capabilities,
			"tenant_id":    conn.TenantID,
		})
		
		// Register agent with capabilities
		registration := &AgentRegistration{
			ID:           conn.AgentID,
			Name:         initParams.Name,
			Capabilities: initParams.Capabilities,
			ConnectionID: conn.ID,
			TenantID:     conn.TenantID,
		}
		if _, err := s.agentRegistry.RegisterAgent(ctx, registration); err != nil {
			s.logger.Warn("Failed to register agent capabilities", map[string]interface{}{
				"agent_id":     conn.AgentID,
				"capabilities": initParams.Capabilities,
				"error":        err.Error(),
			})
		} else {
			s.logger.Debug("Successfully registered agent", map[string]interface{}{
				"agent_id":     conn.AgentID,
				"capabilities": initParams.Capabilities,
			})
		}
	} else {
		s.logger.Debug("No capabilities to register", map[string]interface{}{
			"agent_id":              conn.AgentID,
			"has_capabilities":      len(initParams.Capabilities) > 0,
			"has_agent_registry":    s.agentRegistry != nil,
		})
	}

	// Return server capabilities
	return map[string]interface{}{
		"version": "1.0.0",
		"capabilities": map[string]interface{}{
			"tools":            true,
			"context":          true,
			"events":           true,
			"binary":           true,
			"sessions":         true,
			"workflows":        true,
			"agents":           true,
			"tasks":            true,
			"workspaces":       true,
			"subscriptions":    true,
			"token_management": true,
		},
		"limits": map[string]interface{}{
			"max_context_tokens":   200000,
			"max_message_size":     10 * 1024 * 1024, // 10MB
			"max_subscriptions":    100,
			"max_concurrent_tasks": 10,
		},
	}, nil
}

// handleToolList handles the tool.list method
func (s *Server) handleToolList(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	if s.toolRegistry != nil {
		tools, err := s.toolRegistry.GetToolsForAgent(conn.AgentID)
		if err != nil {
			return nil, err
		}

		// Convert tools to response format
		var toolList []map[string]interface{}
		for _, tool := range tools {
			toolList = append(toolList, map[string]interface{}{
				"id":          tool.ID,
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.Parameters,
			})
		}

		return map[string]interface{}{
			"tools": toolList,
		}, nil
	}

	// Mock response when tool registry not available
	return map[string]interface{}{
		"tools": []map[string]interface{}{
			{
				"id":          "github.list_repos",
				"name":        "List GitHub Repositories",
				"description": "Lists repositories for a GitHub organization",
				"parameters": map[string]interface{}{
					"org": map[string]string{
						"type":        "string",
						"description": "GitHub organization name",
						"required":    "true",
					},
				},
			},
		},
	}, nil
}

// handleToolExecute handles the tool.execute method
func (s *Server) handleToolExecute(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var execParams struct {
		Tool      string                 `json:"tool"`
		Name      string                 `json:"name"` // Alternative parameter name
		Args      map[string]interface{} `json:"args"`
		Arguments map[string]interface{} `json:"arguments"` // Alternative parameter name
	}

	if err := json.Unmarshal(params, &execParams); err != nil {
		return nil, err
	}

	// Handle alternative parameter names
	toolID := execParams.Tool
	if toolID == "" {
		toolID = execParams.Name
	}

	args := execParams.Args
	if args == nil {
		args = execParams.Arguments
	}

	if s.toolRegistry != nil {
		result, err := s.toolRegistry.ExecuteTool(ctx, conn.AgentID, toolID, args)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"tool":   toolID,
			"status": "completed",
			"result": result,
		}, nil
	}

	// Send tool.started notification
	if s.notificationManager != nil {
		s.notificationManager.BroadcastNotification(ctx, "tool.events", "subscription.event", map[string]interface{}{
			"subscription_id": "", // Will be filled by the notification manager
			"event": map[string]interface{}{
				"type":      "tool.started",
				"tool":      toolID,
				"args":      args,
				"agent_id":  conn.AgentID,
				"timestamp": time.Now().Unix(),
			},
		})
	}

	// Mock response when tool registry not available
	// Handle specific test tools
	switch toolID {
	case "test_runner":
		// Mock test runner tool for functional tests
		testSuite := "unit"
		if suite, ok := args["test_suite"].(string); ok {
			testSuite = suite
		}

		// Simulate test execution
		time.Sleep(100 * time.Millisecond)

		result := map[string]interface{}{
			"suite":    testSuite,
			"tests":    10,
			"passed":   10,
			"failed":   0,
			"duration": "100ms",
		}

		// Send tool.completed notification
		if s.notificationManager != nil {
			s.notificationManager.BroadcastNotification(ctx, "tool.events", "subscription.event", map[string]interface{}{
				"subscription_id": "", // Will be filled by the notification manager
				"event": map[string]interface{}{
					"type":      "tool.completed",
					"tool":      toolID,
					"result":    result,
					"agent_id":  conn.AgentID,
					"timestamp": time.Now().Unix(),
				},
			})
		}

		return map[string]interface{}{
			"tool":   toolID,
			"status": "completed",
			"result": result,
		}, nil

	case "echo_context":
		// Get active session context
		sessionContext := "Default context"
		if s.conversationManager != nil {
			activeSessionID := conn.GetActiveSession()
			if activeSessionID != "" {
				session, err := s.conversationManager.GetSession(ctx, activeSessionID)
				if err == nil && session.State != nil {
					if ctx, ok := session.State["context"]; ok {
						sessionContext = fmt.Sprintf("%v", ctx)
					}
				}
			}
		}
		return map[string]interface{}{
			"context": sessionContext,
		}, nil

	default:
		// Generic tool execution
		result := map[string]interface{}{
			"message": "Tool executed successfully",
			"args":    args,
		}

		// Send tool.completed notification
		if s.notificationManager != nil {
			s.notificationManager.BroadcastNotification(ctx, "tool.events", "subscription.event", map[string]interface{}{
				"subscription_id": "", // Will be filled by the notification manager
				"event": map[string]interface{}{
					"type":      "tool.completed",
					"tool":      toolID,
					"result":    result,
					"agent_id":  conn.AgentID,
					"timestamp": time.Now().Unix(),
				},
			})
		}

		return map[string]interface{}{
			"tool":   toolID,
			"status": "completed",
			"result": result,
		}, nil
	}
}

// handleContextCreate handles the context.create method
func (s *Server) handleContextCreate(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var createParams struct {
		Name        string `json:"name"`
		Content     string `json:"content"`
		ReturnStats bool   `json:"return_stats"`
	}

	if err := json.Unmarshal(params, &createParams); err != nil {
		return nil, err
	}

	// Create context through context manager
	if s.contextManager == nil {
		// Mock response when context manager not available
		contextID := fmt.Sprintf("ctx_%d", time.Now().UnixNano())

		result := map[string]interface{}{
			"id":         contextID,
			"name":       createParams.Name,
			"agent_id":   conn.AgentID,
			"tenant_id":  conn.TenantID,
			"created_at": time.Now().Format(time.RFC3339),
			"updated_at": time.Now().Format(time.RFC3339),
		}

		if createParams.ReturnStats {
			tokenCount := len(createParams.Content) / 4
			result["token_count"] = tokenCount
		}

		return result, nil
	}

	context, err := s.contextManager.CreateContext(
		ctx,
		conn.AgentID,
		conn.TenantID,
		createParams.Name,
		createParams.Content,
	)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"id":         context.ID,
		"name":       context.Name,
		"agent_id":   context.AgentID,
		"tenant_id":  conn.TenantID, // Use connection's tenant ID
		"created_at": context.CreatedAt.Format(time.RFC3339),
		"updated_at": context.UpdatedAt.Format(time.RFC3339),
	}

	// Add token stats if requested
	if createParams.ReturnStats {
		// Simple token estimation (in production use proper tokenizer)
		tokenCount := len(createParams.Content) / 4
		result["token_count"] = tokenCount
	}

	return result, nil
}

// handleContextGet handles the context.get method
func (s *Server) handleContextGet(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var getParams struct {
		ContextID string `json:"context_id"`
	}

	if err := json.Unmarshal(params, &getParams); err != nil {
		return nil, err
	}

	// Get context through context manager
	if s.contextManager == nil {
		// Mock response when context manager not available
		return map[string]interface{}{
			"id":         getParams.ContextID,
			"agent_id":   conn.AgentID,
			"content":    []map[string]interface{}{},
			"created_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-01T00:00:00Z",
		}, nil
	}

	context, err := s.contextManager.GetContext(ctx, getParams.ContextID)
	if err != nil {
		return nil, err
	}

	// Convert context items to simple format
	var content []map[string]interface{}
	for _, item := range context.Content {
		content = append(content, map[string]interface{}{
			"role":      item.Role,
			"content":   item.Content,
			"timestamp": item.Timestamp.Format(time.RFC3339),
			"tokens":    item.Tokens,
		})
	}

	return map[string]interface{}{
		"id":             context.ID,
		"name":           context.Name,
		"agent_id":       context.AgentID,
		"content":        content,
		"current_tokens": context.CurrentTokens,
		"max_tokens":     context.MaxTokens,
		"created_at":     context.CreatedAt.Format(time.RFC3339),
		"updated_at":     context.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// handleContextUpdate handles the context.update method
func (s *Server) handleContextUpdate(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var updateParams struct {
		ContextID string `json:"context_id"`
		Content   string `json:"content"`
	}

	if err := json.Unmarshal(params, &updateParams); err != nil {
		return nil, err
	}

	// Update context through context manager
	if s.contextManager == nil {
		// Mock response when context manager not available
		return map[string]interface{}{
			"id":         updateParams.ContextID,
			"updated_at": time.Now().Format(time.RFC3339),
		}, nil
	}

	context, err := s.contextManager.UpdateContext(ctx, updateParams.ContextID, updateParams.Content)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":             context.ID,
		"current_tokens": context.CurrentTokens,
		"updated_at":     context.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// handleEventSubscribe handles the event.subscribe method
func (s *Server) handleEventSubscribe(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var subParams struct {
		Events []string `json:"events"`
	}

	if err := json.Unmarshal(params, &subParams); err != nil {
		return nil, err
	}

	if s.eventBus != nil {
		err := s.eventBus.Subscribe(conn.ID, subParams.Events)
		if err != nil {
			return nil, err
		}
	}

	return map[string]interface{}{
		"subscribed": subParams.Events,
		"status":     "active",
	}, nil
}

// handleSetBinaryProtocol enables/disables binary protocol
func (s *Server) handleSetBinaryProtocol(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var binaryParams struct {
		Enabled     bool `json:"enabled"`
		Compression struct {
			Enabled   bool `json:"enabled"`
			Threshold int  `json:"threshold"`
		} `json:"compression"`
	}

	if err := json.Unmarshal(params, &binaryParams); err != nil {
		return nil, err
	}

	// Update connection settings
	conn.SetBinaryMode(binaryParams.Enabled)
	if binaryParams.Compression.Enabled {
		conn.SetCompressionThreshold(binaryParams.Compression.Threshold)
	}

	return map[string]interface{}{
		"binary_enabled":      binaryParams.Enabled,
		"compression_enabled": binaryParams.Compression.Enabled,
		"status":              "protocol_updated",
	}, nil
}

// Context window management handlers
func (s *Server) handleContextGetLimits(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	agentConfig := s.getAgentConfig(conn.AgentID)

	return map[string]interface{}{
		"max_tokens":        agentConfig.MaxContextTokens,
		"warning_threshold": int(float64(agentConfig.MaxContextTokens) * 0.9),
		"current_usage":     conn.GetTokenUsage(),
		"model":             agentConfig.Model,
	}, nil
}

// handleToolCancel cancels a running tool execution
func (s *Server) handleToolCancel(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var cancelParams struct {
		ExecutionID string `json:"execution_id"`
	}

	if err := json.Unmarshal(params, &cancelParams); err != nil {
		return nil, err
	}

	if s.toolRegistry != nil {
		err := s.toolRegistry.CancelExecution(ctx, cancelParams.ExecutionID)
		if err != nil {
			return nil, err
		}
	}

	return map[string]interface{}{
		"execution_id": cancelParams.ExecutionID,
		"status":       "cancelled",
	}, nil
}

// handleContextAppend appends content to an existing context
func (s *Server) handleContextAppend(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var appendParams struct {
		ContextID string `json:"context_id"`
		Content   string `json:"content"`
	}

	if err := json.Unmarshal(params, &appendParams); err != nil {
		return nil, err
	}

	if s.contextManager != nil {
		context, err := s.contextManager.AppendToContext(ctx, appendParams.ContextID, appendParams.Content)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"id":             context.ID,
			"current_tokens": context.CurrentTokens,
			"updated_at":     context.UpdatedAt.Format(time.RFC3339),
		}, nil
	}

	// Mock response
	return map[string]interface{}{
		"id":         appendParams.ContextID,
		"updated_at": time.Now().Format(time.RFC3339),
	}, nil
}

// handleContextGetStats returns statistics for a context
func (s *Server) handleContextGetStats(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var statsParams struct {
		ContextID string `json:"context_id"`
	}

	if err := json.Unmarshal(params, &statsParams); err != nil {
		return nil, err
	}

	if s.contextManager != nil {
		stats, err := s.contextManager.GetContextStats(ctx, statsParams.ContextID)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"context_id":       statsParams.ContextID,
			"total_tokens":     stats.TotalTokens,
			"message_count":    stats.MessageCount,
			"tool_invocations": stats.ToolInvocations,
			"created_at":       stats.CreatedAt.Format(time.RFC3339),
			"last_accessed":    stats.LastAccessed.Format(time.RFC3339),
		}, nil
	}

	// Mock response
	return map[string]interface{}{
		"context_id":       statsParams.ContextID,
		"total_tokens":     1000,
		"message_count":    10,
		"tool_invocations": 5,
		"created_at":       time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		"last_accessed":    time.Now().Format(time.RFC3339),
	}, nil
}

func (s *Server) handleContextTruncate(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var truncateParams struct {
		ContextID      string `json:"context_id"`
		MaxTokens      int    `json:"max_tokens"`
		PreserveRecent bool   `json:"preserve_recent"`
	}

	if err := json.Unmarshal(params, &truncateParams); err != nil {
		return nil, err
	}

	// Truncate context through context manager
	if s.contextManager == nil {
		// Mock response when context manager not available
		return map[string]interface{}{
			"context_id":      truncateParams.ContextID,
			"new_token_count": truncateParams.MaxTokens,
			"removed_tokens":  100,
			"truncated_at":    time.Now().Format(time.RFC3339),
		}, nil
	}

	truncatedContext, removedTokens, err := s.contextManager.TruncateContext(
		ctx,
		truncateParams.ContextID,
		truncateParams.MaxTokens,
		truncateParams.PreserveRecent,
	)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"context_id":      truncatedContext.ID,
		"new_token_count": truncatedContext.TokenCount,
		"removed_tokens":  removedTokens,
		"truncated_at":    time.Now().Format(time.RFC3339),
	}, nil
}

func (s *Server) handleWindowSetTokens(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var windowParams struct {
		MaxTokens int `json:"max_tokens"`
	}

	if err := json.Unmarshal(params, &windowParams); err != nil {
		return nil, err
	}

	// Update connection's token window
	conn.SetMaxTokens(windowParams.MaxTokens)

	return map[string]interface{}{
		"max_tokens":    windowParams.MaxTokens,
		"current_usage": conn.GetTokenUsage(),
		"updated_at":    time.Now().Format(time.RFC3339),
	}, nil
}

func (s *Server) handleWindowGetTokenUsage(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	usage := conn.GetTokenUsage()
	maxTokens := conn.GetMaxTokens()

	return map[string]interface{}{
		"current_tokens":   usage,
		"max_tokens":       maxTokens,
		"percentage_used":  float64(usage) / float64(maxTokens) * 100,
		"tokens_remaining": maxTokens - usage,
		"breakdown": map[string]interface{}{
			"system_prompt": conn.GetSystemPromptTokens(),
			"conversation":  conn.GetConversationTokens(),
			"tools":         conn.GetToolTokens(),
		},
	}, nil
}

// Session management handlers
func (s *Server) handleSessionCreate(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var sessionParams struct {
		SessionID      string                 `json:"session_id"`
		Name           string                 `json:"name"`
		AgentProfile   map[string]interface{} `json:"agent_profile"`
		InitialContext map[string]interface{} `json:"initial_context"`
		State          map[string]interface{} `json:"state"`
		Persistent     bool                   `json:"persistent"`
		TTLSeconds     int                    `json:"ttl_seconds"`
		TrackMetrics   bool                   `json:"track_metrics"`
		Tags           []string               `json:"tags"`
	}

	if err := json.Unmarshal(params, &sessionParams); err != nil {
		return nil, err
	}

	// Generate session ID if not provided
	if sessionParams.SessionID == "" {
		sessionParams.SessionID = uuid.New().String()
	}

	session, err := s.conversationManager.CreateSession(ctx, &SessionConfig{
		ID:             sessionParams.SessionID,
		Name:           sessionParams.Name,
		AgentID:        conn.AgentID,
		TenantID:       conn.TenantID,
		AgentProfile:   sessionParams.AgentProfile,
		InitialContext: sessionParams.InitialContext,
		State:          sessionParams.State,
		Persistent:     sessionParams.Persistent,
		TTL:            time.Duration(sessionParams.TTLSeconds) * time.Second,
		TrackMetrics:   sessionParams.TrackMetrics,
		Tags:           sessionParams.Tags,
	})
	if err != nil {
		return nil, err
	}

	// Set as active session for connection
	conn.SetActiveSession(session.ID)

	return map[string]interface{}{
		"session_id":    session.ID,
		"name":          session.Name,
		"created_at":    session.CreatedAt.Format(time.RFC3339),
		"agent_profile": session.AgentProfile,
		"state":         session.State,
		"persistent":    session.Persistent,
	}, nil
}

func (s *Server) handleSessionGet(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var getParams struct {
		SessionID string `json:"session_id"`
	}

	if err := json.Unmarshal(params, &getParams); err != nil {
		return nil, err
	}

	session, err := s.conversationManager.GetSession(ctx, getParams.SessionID)
	if err != nil {
		return nil, err
	}

	// Check if session is expired
	if session.IsExpired() {
		return map[string]interface{}{
			"session_id": session.ID,
			"status":     "expired",
			"expired_at": session.ExpiresAt.Format(time.RFC3339),
		}, nil
	}

	return map[string]interface{}{
		"session_id":        session.ID,
		"name":              session.Name,
		"agent_id":          session.AgentID,
		"agent_profile":     session.AgentProfile,
		"state":             session.State,
		"created_at":        session.CreatedAt.Format(time.RFC3339),
		"updated_at":        session.UpdatedAt.Format(time.RFC3339),
		"message_count":     len(session.Messages),
		"token_count":       session.TokenCount,
		"persistent":        session.Persistent,
		"parent_session_id": session.ParentSessionID,
	}, nil
}

func (s *Server) handleSessionUpdateState(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var updateParams struct {
		SessionID string                 `json:"session_id"`
		State     map[string]interface{} `json:"state"`
	}

	if err := json.Unmarshal(params, &updateParams); err != nil {
		return nil, err
	}

	session, err := s.conversationManager.UpdateSessionState(ctx, updateParams.SessionID, updateParams.State)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"session_id": session.ID,
		"state":      session.State,
		"updated_at": session.UpdatedAt.Format(time.RFC3339),
	}, nil
}

func (s *Server) handleSessionAddMessage(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var msgParams struct {
		SessionID string                 `json:"session_id"`
		Message   map[string]interface{} `json:"message"`
	}

	if err := json.Unmarshal(params, &msgParams); err != nil {
		return nil, err
	}

	// Add message to session
	message, err := s.conversationManager.AddMessage(ctx, msgParams.SessionID, msgParams.Message)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"message_id":  message.ID,
		"session_id":  msgParams.SessionID,
		"timestamp":   message.Timestamp.Format(time.RFC3339),
		"token_count": message.TokenCount,
	}, nil
}

func (s *Server) handleSessionGetHistory(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var historyParams struct {
		SessionID string `json:"session_id"`
		Limit     int    `json:"limit"`
		Offset    int    `json:"offset"`
	}

	if err := json.Unmarshal(params, &historyParams); err != nil {
		return nil, err
	}

	session, err := s.conversationManager.GetSession(ctx, historyParams.SessionID)
	if err != nil {
		return nil, err
	}

	// Get messages with pagination
	messages := session.GetMessages(historyParams.Limit, historyParams.Offset)

	return map[string]interface{}{
		"session_id":        session.ID,
		"messages":          messages,
		"total_messages":    len(session.Messages),
		"parent_session_id": session.ParentSessionID,
	}, nil
}

func (s *Server) handleSessionBranch(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var branchParams struct {
		ParentSessionID string `json:"parent_session_id"`
		BranchPoint     int    `json:"branch_point"`
		BranchName      string `json:"branch_name"`
	}

	if err := json.Unmarshal(params, &branchParams); err != nil {
		return nil, err
	}

	branchSession, err := s.conversationManager.BranchSession(
		ctx,
		branchParams.ParentSessionID,
		branchParams.BranchPoint,
		branchParams.BranchName,
	)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"branch_session_id": branchSession.ID,
		"parent_session_id": branchSession.ParentSessionID,
		"branch_name":       branchSession.Name,
		"branch_point":      branchParams.BranchPoint,
		"created_at":        branchSession.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (s *Server) handleSessionRecover(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var recoverParams struct {
		SessionID string `json:"session_id"`
	}

	if err := json.Unmarshal(params, &recoverParams); err != nil {
		return nil, err
	}

	session, err := s.conversationManager.RecoverSession(ctx, recoverParams.SessionID)
	if err != nil {
		return nil, err
	}

	// Restore session to connection
	conn.SetActiveSession(session.ID)

	return map[string]interface{}{
		"recovered":     true,
		"session_id":    session.ID,
		"state":         session.State,
		"message_count": len(session.Messages),
		"last_activity": session.UpdatedAt.Format(time.RFC3339),
	}, nil
}

func (s *Server) handleSessionExport(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var exportParams struct {
		SessionID string   `json:"session_id"`
		Format    string   `json:"format"`
		Include   []string `json:"include"`
	}

	if err := json.Unmarshal(params, &exportParams); err != nil {
		return nil, err
	}

	exportData, downloadURL, err := s.conversationManager.ExportSession(
		ctx,
		exportParams.SessionID,
		exportParams.Format,
		exportParams.Include,
	)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"export":       exportData,
		"download_url": downloadURL,
		"expires_at":   time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		"format":       exportParams.Format,
	}, nil
}

func (s *Server) handleSessionList(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var listParams struct {
		Filter map[string]interface{} `json:"filter"`
		SortBy string                 `json:"sort_by"`
		Limit  int                    `json:"limit"`
		Offset int                    `json:"offset"`
	}

	if err := json.Unmarshal(params, &listParams); err != nil {
		return nil, err
	}

	sessions, total, err := s.conversationManager.ListSessions(
		ctx,
		conn.AgentID,
		listParams.Filter,
		listParams.SortBy,
		listParams.Limit,
		listParams.Offset,
	)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"sessions": sessions,
		"total":    total,
		"limit":    listParams.Limit,
		"offset":   listParams.Offset,
	}, nil
}

func (s *Server) handleSessionSetActive(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var activeParams struct {
		SessionID string `json:"session_id"`
	}

	if err := json.Unmarshal(params, &activeParams); err != nil {
		return nil, err
	}

	// Verify session exists
	session, err := s.conversationManager.GetSession(ctx, activeParams.SessionID)
	if err != nil {
		return nil, err
	}

	// Set as active session
	conn.SetActiveSession(session.ID)

	return map[string]interface{}{
		"session_id":       session.ID,
		"previous_session": conn.GetPreviousSession(),
		"state":            session.State,
	}, nil
}

func (s *Server) handleSessionGetMetrics(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var metricsParams struct {
		SessionID string `json:"session_id"`
	}

	if err := json.Unmarshal(params, &metricsParams); err != nil {
		return nil, err
	}

	metrics, err := s.conversationManager.GetSessionMetrics(ctx, metricsParams.SessionID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"session_id":       metricsParams.SessionID,
		"duration_seconds": metrics.Duration.Seconds(),
		"operation_count":  metrics.OperationCount,
		"token_usage":      metrics.TokenUsage,
		"tool_usage":       metrics.ToolUsage,
		"error_count":      metrics.ErrorCount,
		"created_at":       metrics.CreatedAt.Format(time.RFC3339),
		"last_activity":    metrics.LastActivity.Format(time.RFC3339),
	}, nil
}

// Subscription handlers
func (s *Server) handleSubscribe(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var subParams struct {
		Resource string                 `json:"resource"`
		Filter   map[string]interface{} `json:"filter"`
	}

	if err := json.Unmarshal(params, &subParams); err != nil {
		return nil, err
	}

	subscriptionID, err := s.subscriptionManager.Subscribe(
		conn.ID,
		subParams.Resource,
		subParams.Filter,
	)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"subscription_id": subscriptionID,
		"resource":        subParams.Resource,
		"filter":          subParams.Filter,
		"status":          "active",
	}, nil
}

func (s *Server) handleUnsubscribe(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var unsubParams struct {
		SubscriptionID string `json:"subscription_id"`
	}

	if err := json.Unmarshal(params, &unsubParams); err != nil {
		return nil, err
	}

	err := s.subscriptionManager.Unsubscribe(conn.ID, unsubParams.SubscriptionID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"subscription_id": unsubParams.SubscriptionID,
		"status":          "unsubscribed",
	}, nil
}

func (s *Server) handleEventUnsubscribe(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var unsubParams struct {
		Events []string `json:"events"`
	}

	if err := json.Unmarshal(params, &unsubParams); err != nil {
		return nil, err
	}

	if s.eventBus != nil {
		err := s.eventBus.UnsubscribeEvents(conn.ID, unsubParams.Events)
		if err != nil {
			return nil, err
		}
	}

	return map[string]interface{}{
		"unsubscribed": unsubParams.Events,
		"status":       "success",
	}, nil
}

// handleSubscriptionList lists active subscriptions
func (s *Server) handleSubscriptionList(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	subscriptions := s.subscriptionManager.ListSubscriptions(conn.ID)

	return map[string]interface{}{
		"subscriptions": subscriptions,
		"count":         len(subscriptions),
	}, nil
}

// handleSubscriptionStatus gets status of a subscription
func (s *Server) handleSubscriptionStatus(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var statusParams struct {
		SubscriptionID string `json:"subscription_id"`
	}

	if err := json.Unmarshal(params, &statusParams); err != nil {
		return nil, err
	}

	status, err := s.subscriptionManager.GetSubscriptionStatus(conn.ID, statusParams.SubscriptionID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"subscription_id": statusParams.SubscriptionID,
		"status":          status.Status,
		"resource":        status.Resource,
		"filter":          status.Filter,
		"created_at":      status.CreatedAt.Format(time.RFC3339),
		"last_event":      status.LastEvent.Format(time.RFC3339),
		"event_count":     status.EventCount,
	}, nil
}

// handleSubscriptionRestore restores subscriptions after reconnect
func (s *Server) handleSubscriptionRestore(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var restoreParams struct {
		Subscriptions []struct {
			ID       string                 `json:"id"`
			Resource string                 `json:"resource"`
			Filter   map[string]interface{} `json:"filter"`
		} `json:"subscriptions"`
	}

	if err := json.Unmarshal(params, &restoreParams); err != nil {
		return nil, err
	}

	restored := []string{}
	failed := []map[string]interface{}{}

	for _, sub := range restoreParams.Subscriptions {
		// Restore by creating a new subscription with the old ID
		restoredID, err := s.subscriptionManager.Subscribe(conn.ID, sub.Resource, sub.Filter)
		if err != nil {
			failed = append(failed, map[string]interface{}{
				"id":    sub.ID,
				"error": err.Error(),
			})
		} else {
			restored = append(restored, restoredID)
		}
	}

	return map[string]interface{}{
		"restored": restored,
		"failed":   failed,
		"status":   "complete",
	}, nil
}

// Workflow handlers
func (s *Server) handleWorkflowCreate(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var workflowParams struct {
		ID    string                   `json:"id"`
		Name  string                   `json:"name"`
		Steps []map[string]interface{} `json:"steps"`
	}

	if err := json.Unmarshal(params, &workflowParams); err != nil {
		return nil, err
	}

	workflow, err := s.workflowEngine.CreateWorkflow(ctx, &WorkflowDefinition{
		ID:       workflowParams.ID,
		Name:     workflowParams.Name,
		Steps:    workflowParams.Steps,
		AgentID:  conn.AgentID,
		TenantID: conn.TenantID,
	})
	if err != nil {
		return nil, err
	}

	// Subscribe connection to workflow notifications
	if s.notificationManager != nil {
		s.notificationManager.Subscribe(conn.ID, "workflow:"+workflow.ID)
	}

	return map[string]interface{}{
		"workflow_id": workflow.ID,
		"name":        workflow.Name,
		"steps":       len(workflow.Steps),
		"status":      "created",
		"created_at":  workflow.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (s *Server) handleWorkflowExecute(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var execParams struct {
		WorkflowID string                 `json:"workflow_id"`
		Input      map[string]interface{} `json:"input"`
		Stream     bool                   `json:"stream"`     // Auto-subscribe to notifications
		Sync       bool                   `json:"sync"`       // Wait for completion (with timeout)
		Timeout    int                    `json:"timeout_ms"` // Sync timeout in milliseconds (default 30s)
	}

	if err := json.Unmarshal(params, &execParams); err != nil {
		return nil, err
	}

	// Set default timeout for sync mode
	if execParams.Sync && execParams.Timeout == 0 {
		execParams.Timeout = 30000 // 30 seconds default
	}

	// If streaming is requested, subscribe to workflow notifications
	if execParams.Stream && s.notificationManager != nil {
		s.notificationManager.Subscribe(conn.ID, "workflow:"+execParams.WorkflowID)
		s.notificationManager.Subscribe(conn.ID, "execution:"+execParams.WorkflowID)
	}

	// Use workflow service if available (it has proper authorization)
	var execution *WorkflowExecution
	var err error
	
	if s.workflowService != nil {
		// Parse workflow ID as UUID for the service
		workflowID, parseErr := uuid.Parse(execParams.WorkflowID)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid workflow ID: %w", parseErr)
		}
		
		// Prepare context for workflow execution
		executionContext := models.JSONMap(execParams.Input)
		if executionContext == nil {
			executionContext = make(models.JSONMap)
		}
		
		// Execute using workflow service with proper authorization
		workflowExecution, execErr := s.workflowService.ExecuteWorkflow(ctx, workflowID, executionContext, uuid.New().String())
		if execErr != nil {
			return nil, execErr
		}
		
		// Convert to expected format
		execution = &WorkflowExecution{
			ID:          workflowExecution.ID.String(),
			WorkflowID:  workflowExecution.WorkflowID.String(),
			Status:      string(workflowExecution.Status),
			CurrentStep: 0,
			TotalSteps:  0,
			Input:       execParams.Input,
			StepResults: make(map[string]interface{}),
			StartedAt:   workflowExecution.StartedAt,
		}
		err = nil
	} else {
		// Fall back to workflow engine if service not available
		execution, err = s.workflowEngine.ExecuteWorkflow(ctx, execParams.WorkflowID, execParams.Input)
		if err != nil {
			return nil, err
		}
	}

	// Get workflow to extract step order
	workflow, _ := s.workflowEngine.GetWorkflow(ctx, execParams.WorkflowID)
	var executionOrder []string
	if workflow != nil {
		for _, step := range workflow.Steps {
			if stepID, ok := step["id"].(string); ok {
				executionOrder = append(executionOrder, stepID)
			}
		}
	}

	// If sync mode requested, wait for completion
	if execParams.Sync {
		timeout := time.Duration(execParams.Timeout) * time.Millisecond
		execCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Poll for completion
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-execCtx.Done():
				return nil, fmt.Errorf("workflow execution timeout after %v", timeout)
			case <-ticker.C:
				status, err := s.workflowEngine.GetExecutionStatus(ctx, execution.ID)
				if err != nil {
					return nil, err
				}

				if status.Status == "completed" || status.Status == "failed" || status.Status == "cancelled" {
					return map[string]interface{}{
						"execution_id":    status.ID,
						"workflow_id":     status.WorkflowID,
						"status":          status.Status,
						"execution_order": executionOrder,
						"started_at":      status.StartedAt.Format(time.RFC3339),
						"completed_at":    status.CompletedAt.Format(time.RFC3339),
						"execution_time":  status.ExecutionTime.Seconds(),
						"step_results":    status.StepResults,
					}, nil
				}
			}
		}
	}

	// Default async response
	return map[string]interface{}{
		"execution_id":    execution.ID,
		"workflow_id":     execution.WorkflowID,
		"status":          execution.Status,
		"execution_order": executionOrder,
		"started_at":      execution.StartedAt.Format(time.RFC3339),
		"streaming":       execParams.Stream,
	}, nil
}

func (s *Server) handleWorkflowStatus(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var statusParams struct {
		ExecutionID string `json:"execution_id"`
	}

	if err := json.Unmarshal(params, &statusParams); err != nil {
		return nil, err
	}

	// Try workflowService first (for collaborative workflows)
	if s.workflowService != nil {
		executionID, err := uuid.Parse(statusParams.ExecutionID)
		if err == nil {
			status, err := s.workflowService.GetExecutionStatus(ctx, executionID)
			if err == nil {
				// Convert from models.ExecutionStatus to response format
				completedAt := ""
				estimatedEnd := ""
				if status.EstimatedEnd != nil {
					estimatedEnd = status.EstimatedEnd.Format(time.RFC3339)
				}

				executionTime := time.Since(status.StartedAt).Seconds()

				// Get current step from array
				currentStep := ""
				if len(status.CurrentSteps) > 0 {
					currentStep = status.CurrentSteps[0]
				}

				return map[string]interface{}{
					"execution_id":    status.ExecutionID.String(),
					"workflow_id":     status.WorkflowID.String(),
					"status":          status.Status,
					"current_step":    currentStep,
					"current_steps":   status.CurrentSteps,
					"total_steps":     status.TotalSteps,
					"completed_steps": status.CompletedSteps,
					"progress":        status.Progress,
					"started_at":      status.StartedAt.Format(time.RFC3339),
					"completed_at":    completedAt,
					"estimated_end":   estimatedEnd,
					"execution_time":  executionTime,
					"metrics":         status.Metrics,
				}, nil
			}
			// If workflowService couldn't find it, log and continue to workflowEngine
			if s.logger != nil {
				s.logger.Debug("Execution not found in workflowService", map[string]interface{}{
					"execution_id": executionID.String(),
					"error":        err.Error(),
				})
			}
		}
	}

	// Fall back to workflowEngine for non-collaborative workflows
	if s.workflowEngine != nil {
		status, err := s.workflowEngine.GetExecutionStatus(ctx, statusParams.ExecutionID)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"execution_id":   status.ID,
			"workflow_id":    status.WorkflowID,
			"status":         status.Status,
			"current_step":   status.CurrentStep,
			"total_steps":    status.TotalSteps,
			"started_at":     status.StartedAt.Format(time.RFC3339),
			"completed_at":   status.CompletedAt.Format(time.RFC3339),
			"execution_time": status.ExecutionTime.Seconds(),
			"step_results":   status.StepResults,
		}, nil
	}

	return nil, fmt.Errorf("execution not found: %s", statusParams.ExecutionID)
}

func (s *Server) handleWorkflowCancel(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var cancelParams struct {
		ExecutionID string `json:"execution_id"`
		Reason      string `json:"reason"`
	}

	if err := json.Unmarshal(params, &cancelParams); err != nil {
		return nil, err
	}

	err := s.workflowEngine.CancelExecution(ctx, cancelParams.ExecutionID, cancelParams.Reason)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"execution_id": cancelParams.ExecutionID,
		"status":       "cancelled",
		"reason":       cancelParams.Reason,
		"cancelled_at": time.Now().Format(time.RFC3339),
	}, nil
}

func (s *Server) handleWorkflowList(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var listParams struct {
		Status string `json:"status"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}

	if err := json.Unmarshal(params, &listParams); err != nil {
		return nil, err
	}

	workflows, total, err := s.workflowEngine.ListWorkflows(
		ctx,
		conn.AgentID,
		listParams.Status,
		listParams.Limit,
		listParams.Offset,
	)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"workflows": workflows,
		"total":     total,
		"limit":     listParams.Limit,
		"offset":    listParams.Offset,
	}, nil
}

// Agent handlers
func (s *Server) handleAgentRegister(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var registerParams struct {
		Name         string                 `json:"name"`
		Capabilities []string               `json:"capabilities"`
		Metadata     map[string]interface{} `json:"metadata"`
	}

	if err := json.Unmarshal(params, &registerParams); err != nil {
		return nil, err
	}

	agent, err := s.agentRegistry.RegisterAgent(ctx, &AgentRegistration{
		ID:           conn.AgentID,
		Name:         registerParams.Name,
		Capabilities: registerParams.Capabilities,
		Metadata:     registerParams.Metadata,
		ConnectionID: conn.ID,
		TenantID:     conn.TenantID,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"agent_id":      agent.ID,
		"name":          agent.Name,
		"capabilities":  agent.Capabilities,
		"registered_at": agent.RegisteredAt.Format(time.RFC3339),
	}, nil
}

func (s *Server) handleAgentDiscover(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var discoverParams struct {
		Capabilities []string `json:"required_capabilities"`
		ExcludeSelf  bool     `json:"exclude_self"`
	}

	if err := json.Unmarshal(params, &discoverParams); err != nil {
		return nil, err
	}

	agents, err := s.agentRegistry.DiscoverAgents(
		ctx,
		conn.TenantID,
		discoverParams.Capabilities,
		discoverParams.ExcludeSelf,
		conn.AgentID,
	)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"agents": agents,
		"count":  len(agents),
	}, nil
}

func (s *Server) handleAgentDelegate(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var delegateParams struct {
		TargetAgentID string                 `json:"target_agent_id"`
		Task          map[string]interface{} `json:"task"`
		Timeout       int                    `json:"timeout_seconds"`
	}

	if err := json.Unmarshal(params, &delegateParams); err != nil {
		return nil, err
	}

	result, err := s.agentRegistry.DelegateTask(
		ctx,
		conn.AgentID,
		delegateParams.TargetAgentID,
		delegateParams.Task,
		time.Duration(delegateParams.Timeout)*time.Second,
	)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"task_id":      result.TaskID,
		"target_agent": delegateParams.TargetAgentID,
		"status":       result.Status,
		"delegated_at": result.DelegatedAt.Format(time.RFC3339),
	}, nil
}

func (s *Server) handleAgentCollaborate(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var collabParams struct {
		AgentIDs []string               `json:"agent_ids"`
		Task     map[string]interface{} `json:"task"`
		Strategy string                 `json:"strategy"` // parallel, sequential, consensus
	}

	if err := json.Unmarshal(params, &collabParams); err != nil {
		return nil, err
	}

	collaboration, err := s.agentRegistry.InitiateCollaboration(
		ctx,
		conn.AgentID,
		collabParams.AgentIDs,
		collabParams.Task,
		collabParams.Strategy,
	)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"collaboration_id":     collaboration.ID,
		"participating_agents": collaboration.Agents,
		"strategy":             collaboration.Strategy,
		"status":               collaboration.Status,
		"initiated_at":         collaboration.InitiatedAt.Format(time.RFC3339),
	}, nil
}

func (s *Server) handleAgentStatus(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var statusParams struct {
		AgentID string `json:"agent_id"`
	}

	if err := json.Unmarshal(params, &statusParams); err != nil {
		return nil, err
	}

	// Default to self if no agent ID provided
	agentID := statusParams.AgentID
	if agentID == "" {
		agentID = conn.AgentID
	}

	status, err := s.agentRegistry.GetAgentStatus(ctx, agentID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"agent_id":     status.ID,
		"name":         status.Name,
		"status":       status.Status,
		"capabilities": status.Capabilities,
		"active_tasks": status.ActiveTasks,
		"last_seen":    status.LastSeen.Format(time.RFC3339),
		"health":       status.Health,
	}, nil
}

// Task handlers
func (s *Server) handleTaskCreate(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var taskParams struct {
		Type        string                 `json:"type"`
		Parameters  map[string]interface{} `json:"parameters"`
		Priority    string                 `json:"priority"`
		MaxRetries  int                    `json:"max_retries"`
		TimeoutSecs int                    `json:"timeout_seconds"`
	}

	if err := json.Unmarshal(params, &taskParams); err != nil {
		return nil, err
	}

	task, err := s.taskManager.CreateTask(ctx, &TaskDefinition{
		Type:       taskParams.Type,
		Parameters: taskParams.Parameters,
		Priority:   taskParams.Priority,
		MaxRetries: taskParams.MaxRetries,
		Timeout:    time.Duration(taskParams.TimeoutSecs) * time.Second,
		AgentID:    conn.AgentID,
		TenantID:   conn.TenantID,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"task_id":    task.ID,
		"type":       task.Type,
		"status":     task.Status,
		"priority":   task.Priority,
		"created_at": task.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (s *Server) handleTaskStatus(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var statusParams struct {
		TaskID string `json:"task_id"`
	}

	if err := json.Unmarshal(params, &statusParams); err != nil {
		return nil, err
	}

	task, err := s.taskManager.GetTask(ctx, statusParams.TaskID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"task_id":      task.ID,
		"type":         task.Type,
		"status":       task.Status,
		"progress":     task.Progress,
		"result":       task.Result,
		"error":        task.Error,
		"created_at":   task.CreatedAt.Format(time.RFC3339),
		"started_at":   task.StartedAt.Format(time.RFC3339),
		"completed_at": task.CompletedAt.Format(time.RFC3339),
		"attempts":     task.Attempts,
	}, nil
}

func (s *Server) handleTaskCancel(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var cancelParams struct {
		TaskID string `json:"task_id"`
		Reason string `json:"reason"`
	}

	if err := json.Unmarshal(params, &cancelParams); err != nil {
		return nil, err
	}

	err := s.taskManager.CancelTask(ctx, cancelParams.TaskID, cancelParams.Reason)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"task_id":      cancelParams.TaskID,
		"status":       "cancelled",
		"reason":       cancelParams.Reason,
		"cancelled_at": time.Now().Format(time.RFC3339),
	}, nil
}

func (s *Server) handleTaskList(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var listParams struct {
		Status string `json:"status"`
		Type   string `json:"type"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}

	if err := json.Unmarshal(params, &listParams); err != nil {
		return nil, err
	}

	tasks, total, err := s.taskManager.ListTasks(
		ctx,
		conn.AgentID,
		listParams.Status,
		listParams.Type,
		listParams.Limit,
		listParams.Offset,
	)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tasks":  tasks,
		"total":  total,
		"limit":  listParams.Limit,
		"offset": listParams.Offset,
	}, nil
}

// Workspace handlers
func (s *Server) handleWorkspaceCreate(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var wsParams struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Type        string   `json:"type"` // private, team, public
		Members     []string `json:"members"`
	}

	if err := json.Unmarshal(params, &wsParams); err != nil {
		return nil, err
	}

	workspace, err := s.workspaceManager.CreateWorkspace(ctx, &WorkspaceConfig{
		Name:        wsParams.Name,
		Description: wsParams.Description,
		Type:        wsParams.Type,
		OwnerID:     conn.AgentID,
		TenantID:    conn.TenantID,
		Members:     wsParams.Members,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"workspace_id": workspace.ID,
		"name":         workspace.Name,
		"type":         workspace.Type,
		"owner_id":     workspace.OwnerID,
		"created_at":   workspace.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (s *Server) handleWorkspaceJoin(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var joinParams struct {
		WorkspaceID string `json:"workspace_id"`
		Role        string `json:"role"` // member, moderator, admin
	}

	if err := json.Unmarshal(params, &joinParams); err != nil {
		return nil, err
	}

	member, err := s.workspaceManager.JoinWorkspace(
		ctx,
		joinParams.WorkspaceID,
		conn.AgentID,
		joinParams.Role,
	)
	if err != nil {
		return nil, err
	}

	// Prepare response first
	response := map[string]interface{}{
		"workspace_id": joinParams.WorkspaceID,
		"member_id":    member.ID,
		"role":         member.Role,
		"joined_at":    member.JoinedAt.Format(time.RFC3339),
	}

	// Subscribe to workspace events in a goroutine after a small delay
	// This ensures the response is sent before any notifications
	go func() {
		// Small delay to ensure response is processed first
		time.Sleep(10 * time.Millisecond)
		_ = s.subscriptionManager.SubscribeToWorkspace(conn.ID, joinParams.WorkspaceID)
	}()

	return response, nil
}

func (s *Server) handleWorkspaceLeave(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var leaveParams struct {
		WorkspaceID string `json:"workspace_id"`
	}

	if err := json.Unmarshal(params, &leaveParams); err != nil {
		return nil, err
	}

	err := s.workspaceManager.LeaveWorkspace(ctx, leaveParams.WorkspaceID, conn.AgentID)
	if err != nil {
		return nil, err
	}

	// Unsubscribe from workspace events
	_ = s.subscriptionManager.UnsubscribeFromWorkspace(conn.ID, leaveParams.WorkspaceID)

	return map[string]interface{}{
		"workspace_id": leaveParams.WorkspaceID,
		"left_at":      time.Now().Format(time.RFC3339),
	}, nil
}

func (s *Server) handleWorkspaceBroadcast(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var broadcastParams struct {
		WorkspaceID string                 `json:"workspace_id"`
		Event       string                 `json:"event"`
		Data        map[string]interface{} `json:"data"`
	}

	if err := json.Unmarshal(params, &broadcastParams); err != nil {
		return nil, err
	}

	// Verify sender is member of workspace
	isMember, err := s.workspaceManager.IsMember(ctx, broadcastParams.WorkspaceID, conn.AgentID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, fmt.Errorf("not a member of workspace")
	}

	// Broadcast to all workspace members
	recipients, err := s.workspaceManager.BroadcastToWorkspace(
		ctx,
		broadcastParams.WorkspaceID,
		conn.AgentID,
		broadcastParams.Event,
		broadcastParams.Data,
	)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"workspace_id": broadcastParams.WorkspaceID,
		"event":        broadcastParams.Event,
		"recipients":   recipients,
		"broadcast_at": time.Now().Format(time.RFC3339),
	}, nil
}

func (s *Server) handleWorkspaceListMembers(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var listParams struct {
		WorkspaceID string `json:"workspace_id"`
	}

	if err := json.Unmarshal(params, &listParams); err != nil {
		return nil, err
	}

	members, err := s.workspaceManager.ListMembers(ctx, listParams.WorkspaceID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"workspace_id": listParams.WorkspaceID,
		"members":      members,
		"count":        len(members),
	}, nil
}

// handleStreamBinary handles binary streaming
func (s *Server) handleStreamBinary(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var streamParams struct {
		StreamID string `json:"stream_id"`
		Data     []byte `json:"data"`
		Sequence int    `json:"sequence"`
		Final    bool   `json:"final"`
	}

	if err := json.Unmarshal(params, &streamParams); err != nil {
		return nil, err
	}

	// In a real implementation, this would handle binary data streaming
	// For now, just acknowledge receipt

	return map[string]interface{}{
		"stream_id": streamParams.StreamID,
		"sequence":  streamParams.Sequence,
		"received":  len(streamParams.Data),
		"final":     streamParams.Final,
	}, nil
}

// handleMetricsRecord handles metrics recording
func (s *Server) handleMetricsRecord(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var metricsParams struct {
		Metric    string            `json:"metric"`
		Value     float64           `json:"value"`
		Tags      map[string]string `json:"tags"`
		Timestamp int64             `json:"timestamp"`
	}

	if err := json.Unmarshal(params, &metricsParams); err != nil {
		return nil, err
	}

	// Record metric
	if s.metrics != nil {
		tags := metricsParams.Tags
		if tags == nil {
			tags = make(map[string]string)
		}
		tags["agent_id"] = conn.AgentID
		tags["tenant_id"] = conn.TenantID

		s.metrics.RecordGauge(metricsParams.Metric, metricsParams.Value, tags)
	}

	return map[string]interface{}{
		"metric":    metricsParams.Metric,
		"recorded":  true,
		"timestamp": time.Now().Unix(),
	}, nil
}

// Interfaces for dependencies
type ToolRegistry interface {
	GetToolsForAgent(agentID string) ([]Tool, error)
	ExecuteTool(ctx context.Context, agentID, toolID string, args map[string]interface{}) (interface{}, error)
	CancelExecution(ctx context.Context, executionID string) error
	GetExecutionStatus(ctx context.Context, executionID string) (*ToolExecutionStatus, error)
}

type ContextManager interface {
	GetContext(ctx context.Context, contextID string) (*models.Context, error)
	UpdateContext(ctx context.Context, contextID string, content string) (*models.Context, error)
	TruncateContext(ctx context.Context, contextID string, maxTokens int, preserveRecent bool) (*TruncatedContext, int, error)
	CreateContext(ctx context.Context, agentID, tenantID, name, content string) (*models.Context, error)
	AppendToContext(ctx context.Context, contextID string, content string) (*models.Context, error)
	GetContextStats(ctx context.Context, contextID string) (*ContextStats, error)
}

type EventBus interface {
	Subscribe(connectionID string, events []string) error
	Unsubscribe(connectionID string) error
	UnsubscribeEvents(connectionID string, events []string) error
	Publish(event string, data interface{}) error
}

// Helper method to get agent configuration
func (s *Server) getAgentConfig(agentID string) *AgentConfig {
	// This would normally fetch from database/config
	return &AgentConfig{
		MaxContextTokens: 200000,
		Model:            "claude-3-opus",
	}
}

type AgentConfig struct {
	MaxContextTokens int
	Model            string
}
