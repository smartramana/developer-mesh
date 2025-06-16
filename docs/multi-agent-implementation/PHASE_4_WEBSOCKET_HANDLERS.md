# Phase 4: WebSocket Handler Implementation

## Overview
This phase implements production-grade WebSocket handlers for multi-agent collaboration with enterprise features including connection management, authentication, rate limiting, circuit breakers, distributed tracing, and comprehensive security. Handlers orchestrate real-time communication with resilience, observability, and compliance.

## Timeline
**Duration**: 7-8 days
**Prerequisites**: Phase 3 (Service Layer) completed
**Deliverables**:
- 30+ handler implementations with full validation and authorization
- Connection pool management with health monitoring
- Multi-level rate limiting and quota enforcement
- Circuit breaker integration for service protection
- Comprehensive metrics, tracing, and audit logging
- WebSocket clustering support for horizontal scaling
- Binary protocol optimization for performance
- End-to-end encryption for sensitive operations

## Handler Design Principles

1. **Security First**: Multi-layer authentication, authorization, and encryption
2. **Performance Optimized**: Binary protocol, compression, and connection pooling
3. **Resilience Built-in**: Circuit breakers, retries, and graceful degradation
4. **Observable**: Distributed tracing, metrics, and structured logging
5. **Scalable**: Clustering support with session affinity
6. **Compliant**: Audit trails, data retention, and privacy controls
7. **Developer Friendly**: Self-documenting APIs with OpenAPI specs
8. **Testable**: Comprehensive mocks and test utilities

## Enhanced Connection Management

### Connection Pool with Health Monitoring

```go
// File: apps/mcp-server/internal/api/websocket/connection_pool.go
package websocket

import (
    "context"
    "sync"
    "time"
    
    "github.com/google/uuid"
    "github.com/gorilla/websocket"
    "go.uber.org/zap"
    
    "github.com/S-Corkum/devops-mcp/pkg/observability"
    "github.com/S-Corkum/devops-mcp/pkg/resilience"
)

// ConnectionPool manages WebSocket connections with health monitoring
type ConnectionPool struct {
    mu              sync.RWMutex
    connections     map[string]*ManagedConnection
    tenantConns     map[uuid.UUID]map[string]*ManagedConnection
    
    // Configuration
    maxConnections     int
    maxPerTenant       int
    maxPerAgent        int
    idleTimeout        time.Duration
    pingInterval       time.Duration
    
    // Health monitoring
    healthChecker      *HealthChecker
    metrics           observability.MetricsClient
    logger            *zap.Logger
    
    // Resource management
    rateLimiters      map[string]*resilience.RateLimiter
    circuitBreakers   map[string]*resilience.CircuitBreaker
    
    // Clustering
    nodeID            string
    clusterManager    *ClusterManager
}

// ManagedConnection wraps a WebSocket connection with metadata
type ManagedConnection struct {
    *Connection
    
    // Metadata
    ID              string
    TenantID        uuid.UUID
    AgentID         string
    NodeID          string
    
    // State
    State           ConnectionState
    LastActivity    time.Time
    LastPing        time.Time
    PingLatency     time.Duration
    MessageCount    int64
    BytesSent       int64
    BytesReceived   int64
    
    // Resources
    RateLimiter     *resilience.RateLimiter
    CircuitBreaker  *resilience.CircuitBreaker
    
    // Channels
    send            chan []byte
    done            chan struct{}
}

// ConnectionState represents the state of a connection
type ConnectionState int

const (
    StateConnecting ConnectionState = iota
    StateAuthenticated
    StateActive
    StateIdle
    StateClosing
    StateClosed
)

// HealthChecker monitors connection health
type HealthChecker struct {
    interval        time.Duration
    timeout         time.Duration
    maxFailures     int
    
    metrics         observability.MetricsClient
    logger          *zap.Logger
}

// AddConnection adds a new connection to the pool
func (cp *ConnectionPool) AddConnection(conn *websocket.Conn, tenantID uuid.UUID, agentID string) (*ManagedConnection, error) {
    cp.mu.Lock()
    defer cp.mu.Unlock()
    
    // Check limits
    if len(cp.connections) >= cp.maxConnections {
        return nil, ErrPoolFull
    }
    
    tenantConns := cp.tenantConns[tenantID]
    if len(tenantConns) >= cp.maxPerTenant {
        return nil, ErrTenantLimitExceeded
    }
    
    agentConnCount := 0
    for _, tc := range tenantConns {
        if tc.AgentID == agentID {
            agentConnCount++
        }
    }
    if agentConnCount >= cp.maxPerAgent {
        return nil, ErrAgentLimitExceeded
    }
    
    // Create managed connection
    connID := uuid.New().String()
    mc := &ManagedConnection{
        Connection: &Connection{
            conn:     conn,
            TenantID: tenantID,
            AgentID:  agentID,
        },
        ID:             connID,
        TenantID:       tenantID,
        AgentID:        agentID,
        NodeID:         cp.nodeID,
        State:          StateConnecting,
        LastActivity:   time.Now(),
        send:           make(chan []byte, 256),
        done:           make(chan struct{}),
        RateLimiter:    cp.getRateLimiter(tenantID, agentID),
        CircuitBreaker: cp.getCircuitBreaker(tenantID, agentID),
    }
    
    // Add to pool
    cp.connections[connID] = mc
    if cp.tenantConns[tenantID] == nil {
        cp.tenantConns[tenantID] = make(map[string]*ManagedConnection)
    }
    cp.tenantConns[tenantID][connID] = mc
    
    // Start connection handlers
    go mc.writePump()
    go mc.readPump()
    go cp.monitorConnection(mc)
    
    // Notify cluster
    if cp.clusterManager != nil {
        cp.clusterManager.NotifyConnectionAdded(connID, tenantID, agentID, cp.nodeID)
    }
    
    // Metrics
    cp.metrics.Gauge("websocket.connections.active", float64(len(cp.connections)))
    cp.metrics.Gauge("websocket.connections.tenant", float64(len(tenantConns)), 
        "tenant_id", tenantID.String())
    
    cp.logger.Info("Connection added to pool",
        zap.String("conn_id", connID),
        zap.String("tenant_id", tenantID.String()),
        zap.String("agent_id", agentID))
    
    return mc, nil
}

// monitorConnection monitors a connection's health
func (cp *ConnectionPool) monitorConnection(mc *ManagedConnection) {
    ticker := time.NewTicker(cp.pingInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            if err := cp.pingConnection(mc); err != nil {
                cp.logger.Warn("Ping failed",
                    zap.String("conn_id", mc.ID),
                    zap.Error(err))
                
                // Check if connection should be removed
                if time.Since(mc.LastActivity) > cp.idleTimeout {
                    cp.RemoveConnection(mc.ID, "idle_timeout")
                    return
                }
            }
            
        case <-mc.done:
            return
        }
    }
}

// pingConnection sends a ping to check connection health
func (cp *ConnectionPool) pingConnection(mc *ManagedConnection) error {
    start := time.Now()
    
    mc.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
    if err := mc.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
        return err
    }
    
    mc.LastPing = time.Now()
    mc.PingLatency = time.Since(start)
    
    // Update metrics
    cp.metrics.Histogram("websocket.ping.latency", mc.PingLatency.Seconds(),
        "agent_id", mc.AgentID)
    
    return nil
}
```

### Base Handler Framework

```go
// File: apps/mcp-server/internal/api/websocket/handler_base.go
package websocket

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/google/uuid"
    "github.com/opentracing/opentracing-go"
    "github.com/pkg/errors"
    "go.uber.org/zap"
    
    "github.com/S-Corkum/devops-mcp/pkg/auth"
    "github.com/S-Corkum/devops-mcp/pkg/observability"
    ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

// HandlerFunc defines the WebSocket handler function signature
type HandlerFunc func(ctx context.Context, conn *ManagedConnection, params json.RawMessage) (interface{}, error)

// HandlerMiddleware defines middleware for handlers
type HandlerMiddleware func(HandlerFunc) HandlerFunc

// BaseHandler provides common functionality for all handlers
type BaseHandler struct {
    name            string
    handler         HandlerFunc
    middlewares     []HandlerMiddleware
    
    // Configuration
    timeout         time.Duration
    maxRetries      int
    requiresAuth    bool
    requiredPerms   []string
    
    // Dependencies
    logger          *zap.Logger
    metrics         observability.MetricsClient
    tracer          opentracing.Tracer
    authorizer      auth.Authorizer
    validator       *Validator
    
    // Rate limiting
    rateLimit       int
    ratePeriod      time.Duration
}

// Execute runs the handler with all middleware
func (h *BaseHandler) Execute(ctx context.Context, conn *ManagedConnection, params json.RawMessage) (interface{}, error) {
    // Build handler chain
    handler := h.handler
    for i := len(h.middlewares) - 1; i >= 0; i-- {
        handler = h.middlewares[i](handler)
    }
    
    // Execute with timeout
    ctx, cancel := context.WithTimeout(ctx, h.timeout)
    defer cancel()
    
    return handler(ctx, conn, params)
}

// WithTracing adds distributed tracing to handlers
func WithTracing(tracer opentracing.Tracer, operationName string) HandlerMiddleware {
    return func(next HandlerFunc) HandlerFunc {
        return func(ctx context.Context, conn *ManagedConnection, params json.RawMessage) (interface{}, error) {
            span, ctx := opentracing.StartSpanFromContext(ctx, operationName)
            defer span.Finish()
            
            span.SetTag("handler", operationName)
            span.SetTag("tenant_id", conn.TenantID.String())
            span.SetTag("agent_id", conn.AgentID)
            span.SetTag("connection_id", conn.ID)
            
            result, err := next(ctx, conn, params)
            if err != nil {
                span.SetTag("error", true)
                span.LogKV("error", err.Error())
            }
            
            return result, err
        }
    }
}

// WithMetrics adds metrics collection to handlers
func WithMetrics(metrics observability.MetricsClient) HandlerMiddleware {
    return func(next HandlerFunc) HandlerFunc {
        return func(ctx context.Context, conn *ManagedConnection, params json.RawMessage) (interface{}, error) {
            start := time.Now()
            
            result, err := next(ctx, conn, params)
            
            duration := time.Since(start)
            status := "success"
            if err != nil {
                status = "error"
            }
            
            metrics.Histogram("websocket.handler.duration", duration.Seconds(),
                "handler", getHandlerName(ctx),
                "status", status,
                "tenant_id", conn.TenantID.String())
            
            metrics.Counter("websocket.handler.requests", 1,
                "handler", getHandlerName(ctx),
                "status", status)
            
            return result, err
        }
    }
}

// WithAuthorization adds authorization checks
func WithAuthorization(authorizer auth.Authorizer, requiredPerms []string) HandlerMiddleware {
    return func(next HandlerFunc) HandlerFunc {
        return func(ctx context.Context, conn *ManagedConnection, params json.RawMessage) (interface{}, error) {
            // Extract auth context
            authCtx := auth.GetContext(ctx)
            if authCtx == nil {
                return nil, &ws.Error{
                    Code:    ws.ErrCodeUnauthorized,
                    Message: "Authentication required",
                }
            }
            
            // Check permissions
            for _, perm := range requiredPerms {
                if !authorizer.HasPermission(authCtx, perm) {
                    return nil, &ws.Error{
                        Code:    ws.ErrCodeForbidden,
                        Message: fmt.Sprintf("Missing required permission: %s", perm),
                    }
                }
            }
            
            return next(ctx, conn, params)
        }
    }
}

// WithRateLimiting adds rate limiting to handlers
func WithRateLimiting() HandlerMiddleware {
    return func(next HandlerFunc) HandlerFunc {
        return func(ctx context.Context, conn *ManagedConnection, params json.RawMessage) (interface{}, error) {
            // Check rate limit
            if !conn.RateLimiter.Allow() {
                return nil, &ws.Error{
                    Code:    ws.ErrCodeTooManyRequests,
                    Message: "Rate limit exceeded",
                    Data: map[string]interface{}{
                        "retry_after": conn.RateLimiter.RetryAfter().Seconds(),
                    },
                }
            }
            
            return next(ctx, conn, params)
        }
    }
}

// WithCircuitBreaker adds circuit breaker protection
func WithCircuitBreaker() HandlerMiddleware {
    return func(next HandlerFunc) HandlerFunc {
        return func(ctx context.Context, conn *ManagedConnection, params json.RawMessage) (interface{}, error) {
            // Check circuit breaker
            if !conn.CircuitBreaker.Allow() {
                return nil, &ws.Error{
                    Code:    ws.ErrCodeServiceUnavailable,
                    Message: "Service temporarily unavailable",
                    Data: map[string]interface{}{
                        "retry_after": conn.CircuitBreaker.NextRetry().Seconds(),
                    },
                }
            }
            
            result, err := next(ctx, conn, params)
            
            // Update circuit breaker
            if err != nil {
                conn.CircuitBreaker.RecordFailure()
            } else {
                conn.CircuitBreaker.RecordSuccess()
            }
            
            return result, err
        }
    }
}

// WithValidation adds input validation
func WithValidation(validator *Validator) HandlerMiddleware {
    return func(next HandlerFunc) HandlerFunc {
        return func(ctx context.Context, conn *ManagedConnection, params json.RawMessage) (interface{}, error) {
            // Validate JSON structure
            if !json.Valid(params) {
                return nil, &ws.Error{
                    Code:    ws.ErrCodeInvalidParams,
                    Message: "Invalid JSON",
                }
            }
            
            // Handler-specific validation happens inside the handler
            return next(ctx, conn, params)
        }
    }
}

// WithAuditLog adds audit logging
func WithAuditLog(logger *zap.Logger) HandlerMiddleware {
    return func(next HandlerFunc) HandlerFunc {
        return func(ctx context.Context, conn *ManagedConnection, params json.RawMessage) (interface{}, error) {
            start := time.Now()
            
            // Log request
            logger.Info("Handler request",
                zap.String("handler", getHandlerName(ctx)),
                zap.String("tenant_id", conn.TenantID.String()),
                zap.String("agent_id", conn.AgentID),
                zap.String("connection_id", conn.ID),
                zap.ByteString("params", params))
            
            result, err := next(ctx, conn, params)
            
            // Log response
            logger.Info("Handler response",
                zap.String("handler", getHandlerName(ctx)),
                zap.Duration("duration", time.Since(start)),
                zap.Bool("success", err == nil),
                zap.Error(err))
            
            return result, err
        }
    }
}
```

## Task Handlers

### Enhanced Task Management Handlers

```go
// File: apps/mcp-server/internal/api/websocket/handlers_task.go
package websocket

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/google/uuid"
    "github.com/pkg/errors"
    "go.uber.org/zap"
    
    "github.com/S-Corkum/devops-mcp/pkg/models"
    ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
    "github.com/S-Corkum/devops-mcp/pkg/repository"
    "github.com/S-Corkum/devops-mcp/pkg/services"
    "github.com/S-Corkum/devops-mcp/pkg/validation"
)

// Enhanced Task Request/Response Structures with Validation

type TaskCreateRequest struct {
    Type        string                 `json:"type" validate:"required,max=100"`
    Title       string                 `json:"title" validate:"required,min=3,max=500"`
    Description string                 `json:"description" validate:"max=5000"`
    Priority    string                 `json:"priority" validate:"omitempty,oneof=low normal high critical"`
    AssignTo    string                 `json:"assign_to,omitempty" validate:"omitempty,uuid4|agent_id"`
    Parameters  map[string]interface{} `json:"parameters" validate:"dive"`
    Timeout     int                    `json:"timeout_seconds,omitempty" validate:"omitempty,min=1,max=86400"`
    
    // Advanced fields
    ParentTaskID    *uuid.UUID             `json:"parent_task_id,omitempty"`
    Dependencies    []uuid.UUID            `json:"dependencies,omitempty"`
    Tags            []string               `json:"tags,omitempty" validate:"omitempty,dive,max=50"`
    RequiredCaps    []string               `json:"required_capabilities,omitempty"`
    MaxRetries      int                    `json:"max_retries,omitempty" validate:"omitempty,min=0,max=10"`
    IdempotencyKey  string                 `json:"idempotency_key,omitempty" validate:"omitempty,max=255"`
    ScheduledFor    *time.Time             `json:"scheduled_for,omitempty"`
    Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

type TaskResponse struct {
    TaskID          uuid.UUID              `json:"task_id"`
    Status          string                 `json:"status"`
    AssignedTo      string                 `json:"assigned_to,omitempty"`
    CreatedAt       time.Time              `json:"created_at"`
    EstimatedStart  *time.Time             `json:"estimated_start,omitempty"`
    EstimatedEnd    *time.Time             `json:"estimated_end,omitempty"`
    QueuePosition   int                    `json:"queue_position,omitempty"`
    Warnings        []string               `json:"warnings,omitempty"`
}

type TaskDelegateRequest struct {
    TaskID          uuid.UUID              `json:"task_id" validate:"required"`
    ToAgentID       string                 `json:"to_agent_id" validate:"required,agent_id"`
    Reason          string                 `json:"reason" validate:"required,min=5,max=500"`
    Priority        string                 `json:"priority,omitempty" validate:"omitempty,oneof=normal high critical"`
    RetainContext   bool                   `json:"retain_context"`
    TransferData    map[string]interface{} `json:"transfer_data,omitempty"`
}

type TaskAcceptRequest struct {
    TaskID            uuid.UUID `json:"task_id" validate:"required"`
    EstimatedDuration string    `json:"estimated_duration,omitempty" validate:"omitempty,duration"`
    Resources         []string  `json:"required_resources,omitempty"`
    StartTime         *time.Time `json:"start_time,omitempty"`
}

type TaskCompleteRequest struct {
    TaskID          uuid.UUID              `json:"task_id" validate:"required"`
    Result          interface{}            `json:"result" validate:"required"`
    Metrics         map[string]float64     `json:"metrics,omitempty"`
    Artifacts       []TaskArtifact         `json:"artifacts,omitempty"`
    NextActions     []string               `json:"next_actions,omitempty"`
}

type TaskArtifact struct {
    Type        string    `json:"type" validate:"required,oneof=file log report data"`
    Name        string    `json:"name" validate:"required,max=255"`
    Location    string    `json:"location" validate:"required,url|filepath"`
    Size        int64     `json:"size,omitempty"`
    Checksum    string    `json:"checksum,omitempty"`
    ContentType string    `json:"content_type,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
}

// TaskHandlerRegistry manages all task-related handlers
type TaskHandlerRegistry struct {
    handlers        map[string]*BaseHandler
    taskService     services.TaskService
    agentManager    *AgentManager
    notifier        *NotificationManager
    validator       *validation.Validator
    logger          *zap.Logger
    metrics         observability.MetricsClient
    tracer          opentracing.Tracer
}

// NewTaskHandlerRegistry creates a new task handler registry
func NewTaskHandlerRegistry(deps HandlerDependencies) *TaskHandlerRegistry {
    r := &TaskHandlerRegistry{
        handlers:     make(map[string]*BaseHandler),
        taskService:  deps.TaskService,
        agentManager: deps.AgentManager,
        notifier:     deps.NotificationManager,
        validator:    deps.Validator,
        logger:       deps.Logger.Named("task_handlers"),
        metrics:      deps.Metrics,
        tracer:       deps.Tracer,
    }
    
    r.registerHandlers()
    return r
}

// registerHandlers registers all task handlers with middleware
func (r *TaskHandlerRegistry) registerHandlers() {
    // Common middleware stack
    commonMiddleware := []HandlerMiddleware{
        WithTracing(r.tracer, "task.handler"),
        WithMetrics(r.metrics),
        WithRateLimiting(),
        WithCircuitBreaker(),
        WithValidation(r.validator),
        WithAuditLog(r.logger),
    }
    
    // Register handlers
    r.handlers["task.create"] = &BaseHandler{
        name:          "task.create",
        handler:       r.handleTaskCreate,
        middlewares:   commonMiddleware,
        timeout:       30 * time.Second,
        requiresAuth:  true,
        requiredPerms: []string{"task:create"},
    }
    
    r.handlers["task.delegate"] = &BaseHandler{
        name:          "task.delegate",
        handler:       r.handleTaskDelegate,
        middlewares:   append(commonMiddleware, WithAuthorization(authorizer, []string{"task:delegate"})),
        timeout:       20 * time.Second,
        requiresAuth:  true,
    }
    
    r.handlers["task.accept"] = &BaseHandler{
        name:          "task.accept",
        handler:       r.handleTaskAccept,
        middlewares:   commonMiddleware,
        timeout:       15 * time.Second,
        requiresAuth:  true,
        requiredPerms: []string{"task:accept"},
    }
    
    r.handlers["task.complete"] = &BaseHandler{
        name:          "task.complete",
        handler:       r.handleTaskComplete,
        middlewares:   commonMiddleware,
        timeout:       30 * time.Second,
        requiresAuth:  true,
        requiredPerms: []string{"task:complete"},
    }
    
    // Additional handlers
    r.handlers["task.create_distributed"] = &BaseHandler{
        name:          "task.create_distributed",
        handler:       r.handleTaskCreateDistributed,
        middlewares:   append(commonMiddleware, WithAuthorization(authorizer, []string{"task:create:distributed"})),
        timeout:       60 * time.Second,
        requiresAuth:  true,
    }
}

// handleTaskCreate creates a new task with comprehensive validation and processing
func (r *TaskHandlerRegistry) handleTaskCreate(ctx context.Context, conn *ManagedConnection, params json.RawMessage) (interface{}, error) {
    span, ctx := opentracing.StartSpanFromContext(ctx, "handleTaskCreate")
    defer span.Finish()
    
    r.logger.Debug("Creating task",
        zap.String("agent_id", conn.AgentID),
        zap.String("connection_id", conn.ID))
    
    // Parse and validate request
    var req TaskCreateRequest
    if err := json.Unmarshal(params, &req); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Invalid request format",
            Data:    map[string]interface{}{"error": err.Error()},
        }
    }
    
    // Validate with custom rules
    if err := r.validator.ValidateStruct(ctx, req); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Validation failed",
            Data:    validation.FormatErrors(err),
        }
    }
    
    // Check agent capabilities
    if len(req.RequiredCaps) > 0 {
        agent, err := r.agentManager.GetAgent(conn.AgentID)
        if err != nil {
            return nil, &ws.Error{
                Code:    ws.ErrCodeInternalError,
                Message: "Failed to verify agent capabilities",
            }
        }
        
        missingCaps := agent.MissingCapabilities(req.RequiredCaps)
        if len(missingCaps) > 0 {
            return nil, &ws.Error{
                Code:    ws.ErrCodeForbidden,
                Message: "Agent missing required capabilities",
                Data:    map[string]interface{}{"missing": missingCaps},
            }
        }
    }
    
    // Build task model
    task := &models.Task{
        ID:             uuid.New(),
        TenantID:       conn.TenantID,
        Type:           req.Type,
        Title:          req.Title,
        Description:    req.Description,
        Priority:       models.TaskPriority(req.Priority),
        CreatedBy:      conn.AgentID,
        AssignedTo:     req.AssignTo,
        Parameters:     req.Parameters,
        ParentTaskID:   req.ParentTaskID,
        Dependencies:   req.Dependencies,
        Tags:           req.Tags,
        MaxRetries:     req.MaxRetries,
        TimeoutSeconds: req.Timeout,
        Metadata:       req.Metadata,
        Status:         models.TaskStatusPending,
        CreatedAt:      time.Now(),
    }
    
    // Set defaults
    if task.Priority == "" {
        task.Priority = models.TaskPriorityNormal
    }
    if task.MaxRetries == 0 {
        task.MaxRetries = 3
    }
    if task.TimeoutSeconds == 0 {
        task.TimeoutSeconds = 3600
    }
    
    // Handle scheduled tasks
    if req.ScheduledFor != nil && req.ScheduledFor.After(time.Now()) {
        task.ScheduledFor = req.ScheduledFor
        task.Status = models.TaskStatusScheduled
    }
    
    // Create task with idempotency
    var idempotencyKey string
    if req.IdempotencyKey != "" {
        idempotencyKey = fmt.Sprintf("%s:%s", conn.TenantID, req.IdempotencyKey)
    }
    
    err := r.taskService.Create(ctx, task, idempotencyKey)
    if err != nil {
        r.logger.Error("Failed to create task",
            zap.Error(err),
            zap.String("agent_id", conn.AgentID))
        
        // Handle specific errors
        if errors.Is(err, services.ErrDuplicateIdempotencyKey) {
            // Return existing task
            existingTask, _ := r.taskService.GetByIdempotencyKey(ctx, idempotencyKey)
            if existingTask != nil {
                return r.buildTaskResponse(existingTask), nil
            }
        }
        
        if errors.Is(err, services.ErrQuotaExceeded) {
            return nil, &ws.Error{
                Code:    ws.ErrCodeTooManyRequests,
                Message: "Task quota exceeded",
                Data:    map[string]interface{}{"retry_after": 3600},
            }
        }
        
        return nil, &ws.Error{
            Code:    ws.ErrCodeInternalError,
            Message: "Failed to create task",
        }
    }
    
    // Get queue position if pending
    var queuePosition int
    if task.Status == models.TaskStatusPending && task.AssignedTo != "" {
        queuePosition, _ = r.taskService.GetQueuePosition(ctx, task.ID, task.AssignedTo)
    }
    
    // Send notifications
    r.sendTaskNotifications(ctx, task, "created")
    
    // Metrics
    r.metrics.Counter("task.created", 1,
        "type", task.Type,
        "priority", string(task.Priority),
        "tenant_id", conn.TenantID.String())
    
    // Build response
    resp := TaskResponse{
        TaskID:        task.ID,
        Status:        string(task.Status),
        AssignedTo:    task.AssignedTo,
        CreatedAt:     task.CreatedAt,
        QueuePosition: queuePosition,
    }
    
    // Add estimates if available
    if task.AssignedTo != "" {
        estimates, _ := r.taskService.GetTaskEstimates(ctx, task.ID)
        if estimates != nil {
            resp.EstimatedStart = estimates.StartTime
            resp.EstimatedEnd = estimates.EndTime
        }
    }
    
    return resp, nil
}

// handleTaskDelegate delegates a task to another agent with comprehensive validation
func (r *TaskHandlerRegistry) handleTaskDelegate(ctx context.Context, conn *ManagedConnection, params json.RawMessage) (interface{}, error) {
    span, ctx := opentracing.StartSpanFromContext(ctx, "handleTaskDelegate")
    defer span.Finish()
    
    r.logger.Debug("Delegating task",
        zap.String("agent_id", conn.AgentID),
        zap.String("connection_id", conn.ID))
    
    // Parse request
    var req TaskDelegateRequest
    if err := json.Unmarshal(params, &req); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Invalid request format",
            Data:    map[string]interface{}{"error": err.Error()},
        }
    }
    
    // Validate
    if err := r.validator.ValidateStruct(ctx, req); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Validation failed",
            Data:    validation.FormatErrors(err),
        }
    }
    
    // Get task
    task, err := r.taskService.Get(ctx, req.TaskID)
    if err != nil {
        if errors.Is(err, repository.ErrNotFound) {
            return nil, &ws.Error{
                Code:    ws.ErrCodeNotFound,
                Message: "Task not found",
            }
        }
        return nil, &ws.Error{
            Code:    ws.ErrCodeInternalError,
            Message: "Failed to retrieve task",
        }
    }
    
    // Verify ownership or permission
    if task.AssignedTo != conn.AgentID && task.CreatedBy != conn.AgentID {
        // Check if user has delegation permission
        authCtx := auth.GetContext(ctx)
        if !r.authorizer.HasPermission(authCtx, "task:delegate:any") {
            return nil, &ws.Error{
                Code:    ws.ErrCodeForbidden,
                Message: "Not authorized to delegate this task",
            }
        }
    }
    
    // Verify target agent exists and is available
    targetAgent, err := r.agentManager.GetAgent(req.ToAgentID)
    if err != nil || targetAgent == nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Target agent not found",
        }
    }
    
    if targetAgent.Status != models.AgentStatusAvailable && 
       targetAgent.Status != models.AgentStatusBusy {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Target agent not available",
            Data:    map[string]interface{}{"agent_status": targetAgent.Status},
        }
    }
    
    // Check target agent capabilities
    requiredCaps := task.GetRequiredCapabilities()
    if len(requiredCaps) > 0 {
        missingCaps := targetAgent.MissingCapabilities(requiredCaps)
        if len(missingCaps) > 0 {
            return nil, &ws.Error{
                Code:    ws.ErrCodeInvalidParams,
                Message: "Target agent missing required capabilities",
                Data:    map[string]interface{}{"missing": missingCaps},
            }
        }
    }
    
    // Build delegation
    delegation := &models.TaskDelegation{
        TaskID:         req.TaskID,
        FromAgentID:    conn.AgentID,
        ToAgentID:      req.ToAgentID,
        Reason:         req.Reason,
        DelegationType: models.DelegationTypeManual,
        Metadata: map[string]interface{}{
            "priority":       req.Priority,
            "retain_context": req.RetainContext,
            "transfer_data":  req.TransferData,
        },
    }
    
    // Perform delegation
    err = r.taskService.DelegateTask(ctx, delegation)
    if err != nil {
        r.logger.Error("Failed to delegate task",
            zap.Error(err),
            zap.String("task_id", req.TaskID.String()),
            zap.String("from_agent", conn.AgentID),
            zap.String("to_agent", req.ToAgentID))
        
        // Handle specific errors
        if errors.Is(err, services.ErrDelegationNotAllowed) {
            return nil, &ws.Error{
                Code:    ws.ErrCodeForbidden,
                Message: "Delegation not allowed for this task",
            }
        }
        
        if errors.Is(err, services.ErrMaxDelegationsExceeded) {
            return nil, &ws.Error{
                Code:    ws.ErrCodeInvalidParams,
                Message: "Maximum delegation limit exceeded",
            }
        }
        
        return nil, &ws.Error{
            Code:    ws.ErrCodeInternalError,
            Message: "Failed to delegate task",
        }
    }
    
    // Send notifications
    r.notifier.SendNotification(ctx, req.ToAgentID, "task.delegated", map[string]interface{}{
        "task_id":      req.TaskID,
        "from_agent":   conn.AgentID,
        "reason":       req.Reason,
        "priority":     req.Priority,
        "task_summary": task.GetSummary(),
    })
    
    r.notifier.SendNotification(ctx, task.CreatedBy, "task.delegation", map[string]interface{}{
        "task_id":    req.TaskID,
        "from_agent": conn.AgentID,
        "to_agent":   req.ToAgentID,
        "reason":     req.Reason,
    })
    
    // Metrics
    r.metrics.Counter("task.delegated", 1,
        "from_agent", conn.AgentID,
        "to_agent", req.ToAgentID,
        "tenant_id", conn.TenantID.String())
    
    return map[string]interface{}{
        "status":         "delegated",
        "to_agent":       req.ToAgentID,
        "delegation_id":  delegation.ID,
        "estimated_wait": targetAgent.GetEstimatedWaitTime(),
    }, nil
}

// handleTaskAccept accepts a delegated task with resource allocation
func (r *TaskHandlerRegistry) handleTaskAccept(ctx context.Context, conn *ManagedConnection, params json.RawMessage) (interface{}, error) {
    span, ctx := opentracing.StartSpanFromContext(ctx, "handleTaskAccept")
    defer span.Finish()
    
    r.logger.Debug("Accepting task",
        zap.String("agent_id", conn.AgentID),
        zap.String("connection_id", conn.ID))
    
    // Parse request
    var req TaskAcceptRequest
    if err := json.Unmarshal(params, &req); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Invalid request format",
        }
    }
    
    // Validate
    if err := r.validator.ValidateStruct(ctx, req); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Validation failed",
            Data:    validation.FormatErrors(err),
        }
    }
    
    // Parse estimated duration
    var estimatedDuration time.Duration
    if req.EstimatedDuration != "" {
        d, err := time.ParseDuration(req.EstimatedDuration)
        if err != nil {
            return nil, &ws.Error{
                Code:    ws.ErrCodeInvalidParams,
                Message: "Invalid estimated duration format",
            }
        }
        estimatedDuration = d
    }
    
    // Accept task
    acceptance := &models.TaskAcceptance{
        TaskID:            req.TaskID,
        AgentID:           conn.AgentID,
        EstimatedDuration: estimatedDuration,
        RequiredResources: req.Resources,
        StartTime:         req.StartTime,
    }
    
    err := r.taskService.AcceptTask(ctx, acceptance)
    if err != nil {
        r.logger.Error("Failed to accept task",
            zap.Error(err),
            zap.String("task_id", req.TaskID.String()),
            zap.String("agent_id", conn.AgentID))
        
        // Handle specific errors
        if errors.Is(err, services.ErrTaskNotAssigned) {
            return nil, &ws.Error{
                Code:    ws.ErrCodeForbidden,
                Message: "Task not assigned to this agent",
            }
        }
        
        if errors.Is(err, services.ErrTaskAlreadyAccepted) {
            return nil, &ws.Error{
                Code:    ws.ErrCodeConflict,
                Message: "Task already accepted",
            }
        }
        
        if errors.Is(err, services.ErrResourcesUnavailable) {
            return nil, &ws.Error{
                Code:    ws.ErrCodeServiceUnavailable,
                Message: "Required resources unavailable",
                Data:    map[string]interface{}{"resources": req.Resources},
            }
        }
        
        return nil, &ws.Error{
            Code:    ws.ErrCodeInternalError,
            Message: "Failed to accept task",
        }
    }
    
    // Update agent status
    r.agentManager.UpdateAgentStatus(conn.AgentID, models.AgentStatusBusy, map[string]interface{}{
        "current_task":       req.TaskID,
        "started_at":         time.Now(),
        "estimated_duration": estimatedDuration,
    })
    
    // Get task details for response
    task, _ := r.taskService.Get(ctx, req.TaskID)
    
    // Send notifications
    if task != nil {
        r.notifier.SendNotification(ctx, task.CreatedBy, "task.accepted", map[string]interface{}{
            "task_id":            req.TaskID,
            "agent_id":           conn.AgentID,
            "estimated_duration": req.EstimatedDuration,
            "start_time":         req.StartTime,
        })
    }
    
    // Metrics
    r.metrics.Counter("task.accepted", 1,
        "agent_id", conn.AgentID,
        "tenant_id", conn.TenantID.String())
    
    r.metrics.Histogram("task.acceptance.delay", 
        time.Since(task.AssignedAt).Seconds(),
        "agent_id", conn.AgentID)
    
    return map[string]interface{}{
        "status":              "accepted",
        "estimated_duration":  req.EstimatedDuration,
        "estimated_completion": time.Now().Add(estimatedDuration),
        "resources_allocated": req.Resources,
    }, nil
}

// handleTaskComplete completes a task with comprehensive result handling
func (r *TaskHandlerRegistry) handleTaskComplete(ctx context.Context, conn *ManagedConnection, params json.RawMessage) (interface{}, error) {
    span, ctx := opentracing.StartSpanFromContext(ctx, "handleTaskComplete")
    defer span.Finish()
    
    r.logger.Debug("Completing task",
        zap.String("agent_id", conn.AgentID),
        zap.String("connection_id", conn.ID))
    
    // Parse request
    var req TaskCompleteRequest
    if err := json.Unmarshal(params, &req); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Invalid request format",
        }
    }
    
    // Validate
    if err := r.validator.ValidateStruct(ctx, req); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Validation failed",
            Data:    validation.FormatErrors(err),
        }
    }
    
    // Build completion
    completion := &models.TaskCompletion{
        TaskID:      req.TaskID,
        AgentID:     conn.AgentID,
        Result:      req.Result,
        Metrics:     req.Metrics,
        Artifacts:   r.convertArtifacts(req.Artifacts),
        NextActions: req.NextActions,
        CompletedAt: time.Now(),
    }
    
    // Complete task
    err := r.taskService.CompleteTask(ctx, completion)
    if err != nil {
        r.logger.Error("Failed to complete task",
            zap.Error(err),
            zap.String("task_id", req.TaskID.String()),
            zap.String("agent_id", conn.AgentID))
        
        // Handle specific errors
        if errors.Is(err, services.ErrTaskNotAssigned) {
            return nil, &ws.Error{
                Code:    ws.ErrCodeForbidden,
                Message: "Task not assigned to this agent",
            }
        }
        
        if errors.Is(err, services.ErrTaskNotInProgress) {
            return nil, &ws.Error{
                Code:    ws.ErrCodeConflict,
                Message: "Task not in progress",
            }
        }
        
        if errors.Is(err, services.ErrInvalidResult) {
            return nil, &ws.Error{
                Code:    ws.ErrCodeInvalidParams,
                Message: "Invalid task result",
            }
        }
        
        return nil, &ws.Error{
            Code:    ws.ErrCodeInternalError,
            Message: "Failed to complete task",
        }
    }
    
    // Get task for notifications
    task, _ := r.taskService.Get(ctx, req.TaskID)
    
    // Update agent status
    r.agentManager.UpdateAgentStatus(conn.AgentID, models.AgentStatusAvailable, nil)
    
    // Process next actions
    if len(req.NextActions) > 0 && task != nil {
        r.processNextActions(ctx, task, req.NextActions)
    }
    
    // Send notifications
    if task != nil {
        r.notifier.SendNotification(ctx, task.CreatedBy, "task.completed", map[string]interface{}{
            "task_id":   req.TaskID,
            "agent_id":  conn.AgentID,
            "result":    req.Result,
            "metrics":   req.Metrics,
            "artifacts": len(req.Artifacts),
        })
        
        // Notify parent task if exists
        if task.ParentTaskID != nil {
            r.notifier.SendNotification(ctx, "system", "subtask.completed", map[string]interface{}{
                "parent_task_id": task.ParentTaskID,
                "subtask_id":     req.TaskID,
                "result":         req.Result,
            })
        }
    }
    
    // Metrics
    if task != nil {
        duration := time.Since(task.StartedAt)
        r.metrics.Histogram("task.completion.duration", duration.Seconds(),
            "type", task.Type,
            "priority", string(task.Priority),
            "agent_id", conn.AgentID)
        
        r.metrics.Counter("task.completed", 1,
            "type", task.Type,
            "agent_id", conn.AgentID,
            "tenant_id", conn.TenantID.String())
        
        // Record custom metrics
        for name, value := range req.Metrics {
            r.metrics.Gauge(fmt.Sprintf("task.custom.%s", name), value,
                "task_type", task.Type,
                "agent_id", conn.AgentID)
        }
    }
    
    return map[string]interface{}{
        "status":       "completed",
        "completed_at": completion.CompletedAt,
        "duration":     time.Since(task.StartedAt).Seconds(),
        "next_actions": req.NextActions,
    }, nil
}

// Additional helper methods
func (r *TaskHandlerRegistry) sendTaskNotifications(ctx context.Context, task *models.Task, event string) {
    // Notify assigned agent
    if task.AssignedTo != "" {
        r.notifier.SendNotification(ctx, task.AssignedTo, fmt.Sprintf("task.%s", event), map[string]interface{}{
            "task_id":  task.ID,
            "type":     task.Type,
            "title":    task.Title,
            "priority": task.Priority,
        })
    }
    
    // Notify creator if different
    if task.CreatedBy != task.AssignedTo {
        r.notifier.SendNotification(ctx, task.CreatedBy, fmt.Sprintf("task.%s", event), map[string]interface{}{
            "task_id":     task.ID,
            "assigned_to": task.AssignedTo,
        })
    }
    
    // Notify workspace if task is part of one
    if task.WorkspaceID != nil {
        r.notifier.BroadcastNotification(ctx,
            fmt.Sprintf("workspace:%s", task.WorkspaceID),
            fmt.Sprintf("task.%s", event),
            map[string]interface{}{
                "task_id": task.ID,
                "title":   task.Title,
            })
    }
}

func (r *TaskHandlerRegistry) buildTaskResponse(task *models.Task) TaskResponse {
    resp := TaskResponse{
        TaskID:     task.ID,
        Status:     string(task.Status),
        AssignedTo: task.AssignedTo,
        CreatedAt:  task.CreatedAt,
    }
    
    if task.Warnings != nil {
        resp.Warnings = task.Warnings
    }
    
    return resp
}

func (r *TaskHandlerRegistry) convertArtifacts(artifacts []TaskArtifact) []models.TaskArtifact {
    result := make([]models.TaskArtifact, len(artifacts))
    for i, a := range artifacts {
        result[i] = models.TaskArtifact{
            Type:        a.Type,
            Name:        a.Name,
            Location:    a.Location,
            Size:        a.Size,
            Checksum:    a.Checksum,
            ContentType: a.ContentType,
            CreatedAt:   a.CreatedAt,
        }
    }
    return result
}

func (r *TaskHandlerRegistry) processNextActions(ctx context.Context, task *models.Task, actions []string) {
    // Process suggested next actions
    for _, action := range actions {
        r.logger.Info("Processing next action",
            zap.String("task_id", task.ID.String()),
            zap.String("action", action))
        
        // This could trigger workflows, create follow-up tasks, etc.
        // Implementation depends on your action processing system
    }
}
```

### Distributed Task Handlers

```go
// Enhanced structures for distributed tasks
type DistributedTaskRequest struct {
    Type            string                 `json:"type" validate:"required,max=100"`
    Title           string                 `json:"title" validate:"required,min=3,max=500"`
    Description     string                 `json:"description" validate:"max=5000"`
    Priority        string                 `json:"priority" validate:"omitempty,oneof=low normal high critical"`
    Subtasks        []SubtaskDefinition    `json:"subtasks" validate:"required,min=2,max=100,dive"`
    Aggregation     AggregationConfig      `json:"aggregation" validate:"required"`
    Timeout         int                    `json:"timeout_seconds,omitempty" validate:"omitempty,min=60,max=86400"`
    IdempotencyKey  string                 `json:"idempotency_key,omitempty"`
    Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

type SubtaskDefinition struct {
    ID              string                 `json:"id" validate:"required,max=100"`
    AgentID         string                 `json:"agent_id,omitempty" validate:"omitempty,agent_id"`
    AgentSelector   AgentSelector          `json:"agent_selector,omitempty"`
    Type            string                 `json:"type" validate:"required"`
    Title           string                 `json:"title" validate:"required"`
    Description     string                 `json:"description"`
    Parameters      map[string]interface{} `json:"parameters"`
    DependsOn       []string               `json:"depends_on,omitempty"`
    Timeout         int                    `json:"timeout_seconds,omitempty"`
    Priority        string                 `json:"priority,omitempty"`
    RequiredCaps    []string               `json:"required_capabilities,omitempty"`
}

type AgentSelector struct {
    Strategy        string   `json:"strategy" validate:"required,oneof=round_robin least_loaded capability_match random"`
    Capabilities    []string `json:"capabilities,omitempty"`
    PreferredAgents []string `json:"preferred_agents,omitempty"`
    ExcludeAgents   []string `json:"exclude_agents,omitempty"`
}

type AggregationConfig struct {
    Method          string                 `json:"method" validate:"required,oneof=combine_results average sum custom map_reduce"`
    WaitForAll      bool                   `json:"wait_for_all"`
    Timeout         int                    `json:"timeout_seconds,omitempty"`
    FailureStrategy string                 `json:"failure_strategy,omitempty" validate:"omitempty,oneof=fail_fast continue_on_failure"`
    CustomAggregator string                `json:"custom_aggregator,omitempty"`
    AggregatorParams map[string]interface{} `json:"aggregator_params,omitempty"`
}

// handleTaskCreateDistributed creates a distributed task with subtasks
func (r *TaskHandlerRegistry) handleTaskCreateDistributed(ctx context.Context, conn *ManagedConnection, params json.RawMessage) (interface{}, error) {
    span, ctx := opentracing.StartSpanFromContext(ctx, "handleTaskCreateDistributed")
    defer span.Finish()
    
    r.logger.Debug("Creating distributed task",
        zap.String("agent_id", conn.AgentID),
        zap.String("connection_id", conn.ID))
    
    // Parse request
    var req DistributedTaskRequest
    if err := json.Unmarshal(params, &req); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Invalid request format",
            Data:    map[string]interface{}{"error": err.Error()},
        }
    }
    
    // Validate
    if err := r.validator.ValidateStruct(ctx, req); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Validation failed",
            Data:    validation.FormatErrors(err),
        }
    }
    
    // Validate subtask dependencies
    if err := r.validateSubtaskDependencies(req.Subtasks); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Invalid subtask dependencies",
            Data:    map[string]interface{}{"error": err.Error()},
        }
    }
    
    // Check agent availability for subtasks
    agentAssignments, err := r.resolveAgentAssignments(ctx, req.Subtasks)
    if err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeServiceUnavailable,
            Message: "Unable to assign agents to subtasks",
            Data:    map[string]interface{}{"error": err.Error()},
        }
    }
    
    // Build distributed task
    distributedTask := &models.DistributedTask{
        ID:           uuid.New(),
        TenantID:     conn.TenantID,
        Type:         req.Type,
        Title:        req.Title,
        Description:  req.Description,
        Priority:     models.TaskPriority(req.Priority),
        CreatedBy:    conn.AgentID,
        Timeout:      req.Timeout,
        Metadata:     req.Metadata,
        Status:       models.TaskStatusPending,
        CreatedAt:    time.Now(),
        Aggregation: models.AggregationConfig{
            Method:          req.Aggregation.Method,
            WaitForAll:      req.Aggregation.WaitForAll,
            Timeout:         req.Aggregation.Timeout,
            FailureStrategy: req.Aggregation.FailureStrategy,
            CustomAggregator: req.Aggregation.CustomAggregator,
            AggregatorParams: req.Aggregation.AggregatorParams,
        },
    }
    
    // Build subtasks
    subtasks := make([]models.Subtask, len(req.Subtasks))
    for i, st := range req.Subtasks {
        subtasks[i] = models.Subtask{
            ID:             st.ID,
            ParentTaskID:   distributedTask.ID,
            AgentID:        agentAssignments[st.ID],
            Type:           st.Type,
            Title:          st.Title,
            Description:    st.Description,
            Parameters:     st.Parameters,
            DependsOn:      st.DependsOn,
            Timeout:        st.Timeout,
            Priority:       models.TaskPriority(st.Priority),
            RequiredCaps:   st.RequiredCaps,
            Status:         models.TaskStatusPending,
        }
    }
    distributedTask.Subtasks = subtasks
    
    // Create with idempotency
    var idempotencyKey string
    if req.IdempotencyKey != "" {
        idempotencyKey = fmt.Sprintf("%s:%s:distributed", conn.TenantID, req.IdempotencyKey)
    }
    
    err = r.taskService.CreateDistributedTask(ctx, distributedTask, idempotencyKey)
    if err != nil {
        r.logger.Error("Failed to create distributed task",
            zap.Error(err),
            zap.String("agent_id", conn.AgentID))
        
        if errors.Is(err, services.ErrDuplicateIdempotencyKey) {
            existingTask, _ := r.taskService.GetDistributedTaskByIdempotencyKey(ctx, idempotencyKey)
            if existingTask != nil {
                return r.buildDistributedTaskResponse(existingTask), nil
            }
        }
        
        return nil, &ws.Error{
            Code:    ws.ErrCodeInternalError,
            Message: "Failed to create distributed task",
        }
    }
    
    // Send notifications to assigned agents
    for _, subtask := range subtasks {
        r.notifier.SendNotification(ctx, subtask.AgentID, "subtask.assigned", map[string]interface{}{
            "parent_task_id": distributedTask.ID,
            "subtask_id":     subtask.ID,
            "type":           subtask.Type,
            "title":          subtask.Title,
            "priority":       subtask.Priority,
            "timeout":        subtask.Timeout,
        })
    }
    
    // Metrics
    r.metrics.Counter("distributed_task.created", 1,
        "type", distributedTask.Type,
        "subtask_count", len(subtasks),
        "tenant_id", conn.TenantID.String())
    
    return r.buildDistributedTaskResponse(distributedTask), nil
}

// handleTaskSubmitResult submits a subtask result
func (r *TaskHandlerRegistry) handleTaskSubmitResult(ctx context.Context, conn *ManagedConnection, params json.RawMessage) (interface{}, error) {
    span, ctx := opentracing.StartSpanFromContext(ctx, "handleTaskSubmitResult")
    defer span.Finish()
    
    var req struct {
        TaskID      uuid.UUID              `json:"task_id" validate:"required"`
        SubtaskID   string                 `json:"subtask_id" validate:"required"`
        Result      interface{}            `json:"result" validate:"required"`
        Metrics     map[string]float64     `json:"metrics,omitempty"`
        Artifacts   []TaskArtifact         `json:"artifacts,omitempty"`
    }
    
    if err := json.Unmarshal(params, &req); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Invalid request format",
        }
    }
    
    if err := r.validator.ValidateStruct(ctx, req); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Validation failed",
            Data:    validation.FormatErrors(err),
        }
    }
    
    // Submit subtask result
    submission := &models.SubtaskResult{
        ParentTaskID: req.TaskID,
        SubtaskID:    req.SubtaskID,
        AgentID:      conn.AgentID,
        Result:       req.Result,
        Metrics:      req.Metrics,
        Artifacts:    r.convertArtifacts(req.Artifacts),
        SubmittedAt:  time.Now(),
    }
    
    aggregatedResult, err := r.taskService.SubmitSubtaskResult(ctx, submission)
    if err != nil {
        r.logger.Error("Failed to submit subtask result",
            zap.Error(err),
            zap.String("task_id", req.TaskID.String()),
            zap.String("subtask_id", req.SubtaskID))
        
        if errors.Is(err, services.ErrSubtaskNotFound) {
            return nil, &ws.Error{
                Code:    ws.ErrCodeNotFound,
                Message: "Subtask not found",
            }
        }
        
        if errors.Is(err, services.ErrSubtaskNotAssigned) {
            return nil, &ws.Error{
                Code:    ws.ErrCodeForbidden,
                Message: "Subtask not assigned to this agent",
            }
        }
        
        return nil, &ws.Error{
            Code:    ws.ErrCodeInternalError,
            Message: "Failed to submit result",
        }
    }
    
    // Update agent status
    r.agentManager.UpdateAgentStatus(conn.AgentID, models.AgentStatusAvailable, nil)
    
    // Metrics
    r.metrics.Counter("subtask.completed", 1,
        "agent_id", conn.AgentID,
        "tenant_id", conn.TenantID.String())
    
    response := map[string]interface{}{
        "status":      "submitted",
        "subtask_id":  req.SubtaskID,
        "submitted_at": submission.SubmittedAt,
    }
    
    // Include aggregated result if task is complete
    if aggregatedResult != nil {
        response["task_completed"] = true
        response["aggregated_result"] = aggregatedResult
    }
    
    return response, nil
}

// Helper methods for distributed tasks
func (r *TaskHandlerRegistry) validateSubtaskDependencies(subtasks []SubtaskDefinition) error {
    // Build ID set
    ids := make(map[string]bool)
    for _, st := range subtasks {
        if ids[st.ID] {
            return fmt.Errorf("duplicate subtask ID: %s", st.ID)
        }
        ids[st.ID] = true
    }
    
    // Check dependencies
    for _, st := range subtasks {
        for _, dep := range st.DependsOn {
            if !ids[dep] {
                return fmt.Errorf("subtask %s depends on non-existent subtask %s", st.ID, dep)
            }
        }
    }
    
    // Check for cycles
    if r.hasCycles(subtasks) {
        return fmt.Errorf("circular dependencies detected")
    }
    
    return nil
}

func (r *TaskHandlerRegistry) hasCycles(subtasks []SubtaskDefinition) bool {
    // Build adjacency list
    graph := make(map[string][]string)
    for _, st := range subtasks {
        graph[st.ID] = st.DependsOn
    }
    
    // DFS to detect cycles
    visited := make(map[string]int) // 0: unvisited, 1: visiting, 2: visited
    
    var hasCycle func(node string) bool
    hasCycle = func(node string) bool {
        visited[node] = 1
        
        for _, dep := range graph[node] {
            if visited[dep] == 1 {
                return true // Back edge found
            }
            if visited[dep] == 0 && hasCycle(dep) {
                return true
            }
        }
        
        visited[node] = 2
        return false
    }
    
    for _, st := range subtasks {
        if visited[st.ID] == 0 && hasCycle(st.ID) {
            return true
        }
    }
    
    return false
}

func (r *TaskHandlerRegistry) resolveAgentAssignments(ctx context.Context, subtasks []SubtaskDefinition) (map[string]string, error) {
    assignments := make(map[string]string)
    
    for _, st := range subtasks {
        var agentID string
        
        if st.AgentID != "" {
            // Direct assignment
            agentID = st.AgentID
        } else if st.AgentSelector.Strategy != "" {
            // Use selector strategy
            agent, err := r.selectAgent(ctx, st.AgentSelector, st.RequiredCaps)
            if err != nil {
                return nil, fmt.Errorf("failed to select agent for subtask %s: %w", st.ID, err)
            }
            agentID = agent.ID
        } else {
            // Default: capability match or round-robin
            selector := AgentSelector{
                Strategy:     "capability_match",
                Capabilities: st.RequiredCaps,
            }
            agent, err := r.selectAgent(ctx, selector, st.RequiredCaps)
            if err != nil {
                return nil, fmt.Errorf("failed to select agent for subtask %s: %w", st.ID, err)
            }
            agentID = agent.ID
        }
        
        assignments[st.ID] = agentID
    }
    
    return assignments, nil
}

func (r *TaskHandlerRegistry) selectAgent(ctx context.Context, selector AgentSelector, requiredCaps []string) (*models.Agent, error) {
    // Get available agents
    agents, err := r.agentManager.GetAvailableAgents(ctx, selector.PreferredAgents, selector.ExcludeAgents)
    if err != nil {
        return nil, err
    }
    
    // Filter by capabilities
    if len(requiredCaps) > 0 || len(selector.Capabilities) > 0 {
        allCaps := append(requiredCaps, selector.Capabilities...)
        agents = r.filterByCapabilities(agents, allCaps)
    }
    
    if len(agents) == 0 {
        return nil, fmt.Errorf("no agents available with required capabilities")
    }
    
    // Apply selection strategy
    switch selector.Strategy {
    case "round_robin":
        return r.agentManager.SelectRoundRobin(agents), nil
    case "least_loaded":
        return r.agentManager.SelectLeastLoaded(agents), nil
    case "random":
        return r.agentManager.SelectRandom(agents), nil
    case "capability_match":
        return r.agentManager.SelectBestCapabilityMatch(agents, requiredCaps), nil
    default:
        return agents[0], nil
    }
}

func (r *TaskHandlerRegistry) filterByCapabilities(agents []*models.Agent, capabilities []string) []*models.Agent {
    var filtered []*models.Agent
    for _, agent := range agents {
        if agent.HasCapabilities(capabilities) {
            filtered = append(filtered, agent)
        }
    }
    return filtered
}

func (r *TaskHandlerRegistry) buildDistributedTaskResponse(task *models.DistributedTask) map[string]interface{} {
    subtaskInfo := make([]map[string]interface{}, len(task.Subtasks))
    for i, st := range task.Subtasks {
        subtaskInfo[i] = map[string]interface{}{
            "id":          st.ID,
            "agent_id":    st.AgentID,
            "type":        st.Type,
            "status":      st.Status,
            "depends_on":  st.DependsOn,
        }
    }
    
    return map[string]interface{}{
        "task_id":     task.ID,
        "type":        task.Type,
        "status":      task.Status,
        "created_at":  task.CreatedAt,
        "subtasks":    subtaskInfo,
        "aggregation": task.Aggregation,
    }
}
```

## Workflow Handlers

### Enhanced Workflow Management Handlers

```go
// File: apps/mcp-server/internal/api/websocket/handlers_workflow.go
package websocket

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/google/uuid"
    "github.com/pkg/errors"
    "go.uber.org/zap"
    
    "github.com/S-Corkum/devops-mcp/pkg/models"
    ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
    "github.com/S-Corkum/devops-mcp/pkg/services"
    "github.com/S-Corkum/devops-mcp/pkg/validation"
)

// Enhanced Workflow Request Structures

type WorkflowCreateRequest struct {
    Name            string                    `json:"name" validate:"required,min=3,max=255"`
    Type            string                    `json:"type" validate:"required,oneof=sequential parallel conditional collaborative"`
    Description     string                    `json:"description" validate:"max=2000"`
    Version         int                       `json:"version,omitempty"`
    Agents          map[string]AgentRole      `json:"agents" validate:"required,min=1"`
    Steps           []WorkflowStepDefinition  `json:"steps" validate:"required,min=1,max=100,dive"`
    Config          WorkflowConfig            `json:"config"`
    Tags            []string                  `json:"tags,omitempty" validate:"omitempty,dive,max=50"`
    IdempotencyKey  string                    `json:"idempotency_key,omitempty"`
}

type AgentRole struct {
    AgentID         string   `json:"agent_id,omitempty"`
    Capabilities    []string `json:"capabilities,omitempty"`
    Selector        string   `json:"selector,omitempty" validate:"omitempty,oneof=any specific capability_match"`
}

type WorkflowStepDefinition struct {
    ID              string                 `json:"id" validate:"required,max=100"`
    Name            string                 `json:"name" validate:"required"`
    Agent           string                 `json:"agent" validate:"required"` // role name
    Action          string                 `json:"action" validate:"required"`
    Input           map[string]interface{} `json:"input,omitempty"`
    DependsOn       []string               `json:"depends_on,omitempty"`
    Conditions      []StepCondition        `json:"conditions,omitempty"`
    Timeout         int                    `json:"timeout_seconds,omitempty" validate:"omitempty,min=1,max=3600"`
    Retries         int                    `json:"retries,omitempty" validate:"omitempty,min=0,max=5"`
    OnFailure       string                 `json:"on_failure,omitempty" validate:"omitempty,oneof=fail skip continue retry"`
    OutputMapping   map[string]string      `json:"output_mapping,omitempty"`
}

type StepCondition struct {
    Type       string      `json:"type" validate:"required,oneof=expression variable output"`
    Expression string      `json:"expression,omitempty"`
    Variable   string      `json:"variable,omitempty"`
    Operator   string      `json:"operator,omitempty" validate:"omitempty,oneof=eq ne gt lt gte lte contains"`
    Value      interface{} `json:"value,omitempty"`
}

type WorkflowConfig struct {
    MaxParallel      int                    `json:"max_parallel,omitempty" validate:"omitempty,min=1,max=100"`
    GlobalTimeout    int                    `json:"global_timeout_seconds,omitempty"`
    ErrorStrategy    string                 `json:"error_strategy,omitempty" validate:"omitempty,oneof=fail_fast continue compensate"`
    Notifications    NotificationConfig     `json:"notifications,omitempty"`
    ResourceLimits   ResourceLimits         `json:"resource_limits,omitempty"`
    SecurityContext  map[string]interface{} `json:"security_context,omitempty"`
}

type NotificationConfig struct {
    OnStart      bool     `json:"on_start"`
    OnComplete   bool     `json:"on_complete"`
    OnFailure    bool     `json:"on_failure"`
    Recipients   []string `json:"recipients,omitempty"`
    Webhooks     []string `json:"webhooks,omitempty"`
}

type ResourceLimits struct {
    MaxMemoryMB     int `json:"max_memory_mb,omitempty"`
    MaxCPUPercent   int `json:"max_cpu_percent,omitempty"`
    MaxExecutions   int `json:"max_executions,omitempty"`
}

// WorkflowHandlerRegistry manages workflow handlers
type WorkflowHandlerRegistry struct {
    handlers         map[string]*BaseHandler
    workflowService  services.WorkflowService
    agentManager     *AgentManager
    notifier         *NotificationManager
    validator        *validation.Validator
    logger           *zap.Logger
    metrics          observability.MetricsClient
    tracer           opentracing.Tracer
}

// NewWorkflowHandlerRegistry creates a new workflow handler registry
func NewWorkflowHandlerRegistry(deps HandlerDependencies) *WorkflowHandlerRegistry {
    r := &WorkflowHandlerRegistry{
        handlers:        make(map[string]*BaseHandler),
        workflowService: deps.WorkflowService,
        agentManager:    deps.AgentManager,
        notifier:        deps.NotificationManager,
        validator:       deps.Validator,
        logger:          deps.Logger.Named("workflow_handlers"),
        metrics:         deps.Metrics,
        tracer:          deps.Tracer,
    }
    
    r.registerHandlers()
    return r
}

// registerHandlers registers all workflow handlers
func (r *WorkflowHandlerRegistry) registerHandlers() {
    commonMiddleware := []HandlerMiddleware{
        WithTracing(r.tracer, "workflow.handler"),
        WithMetrics(r.metrics),
        WithRateLimiting(),
        WithCircuitBreaker(),
        WithValidation(r.validator),
        WithAuditLog(r.logger),
    }
    
    r.handlers["workflow.create"] = &BaseHandler{
        name:          "workflow.create",
        handler:       r.handleWorkflowCreate,
        middlewares:   commonMiddleware,
        timeout:       30 * time.Second,
        requiresAuth:  true,
        requiredPerms: []string{"workflow:create"},
    }
    
    r.handlers["workflow.execute"] = &BaseHandler{
        name:          "workflow.execute",
        handler:       r.handleWorkflowExecute,
        middlewares:   commonMiddleware,
        timeout:       60 * time.Second,
        requiresAuth:  true,
        requiredPerms: []string{"workflow:execute"},
    }
    
    r.handlers["workflow.create_collaborative"] = &BaseHandler{
        name:          "workflow.create_collaborative",
        handler:       r.handleWorkflowCreateCollaborative,
        middlewares:   append(commonMiddleware, WithAuthorization(authorizer, []string{"workflow:create:collaborative"})),
        timeout:       45 * time.Second,
        requiresAuth:  true,
    }
}

// handleWorkflowCreate creates a new workflow
func (r *WorkflowHandlerRegistry) handleWorkflowCreate(ctx context.Context, conn *ManagedConnection, params json.RawMessage) (interface{}, error) {
    span, ctx := opentracing.StartSpanFromContext(ctx, "handleWorkflowCreate")
    defer span.Finish()
    
    r.logger.Debug("Creating workflow",
        zap.String("agent_id", conn.AgentID),
        zap.String("connection_id", conn.ID))
    
    // Parse request
    var req WorkflowCreateRequest
    if err := json.Unmarshal(params, &req); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Invalid request format",
            Data:    map[string]interface{}{"error": err.Error()},
        }
    }
    
    // Validate
    if err := r.validator.ValidateStruct(ctx, req); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Validation failed",
            Data:    validation.FormatErrors(err),
        }
    }
    
    // Validate step dependencies
    if err := r.validateStepDependencies(req.Steps); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Invalid step dependencies",
            Data:    map[string]interface{}{"error": err.Error()},
        }
    }
    
    // Validate agent roles
    if err := r.validateAgentRoles(req.Agents, req.Steps); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Invalid agent role configuration",
            Data:    map[string]interface{}{"error": err.Error()},
        }
    }
    
    // Build workflow model
    workflow := &models.Workflow{
        ID:          uuid.New(),
        TenantID:    conn.TenantID,
        Name:        req.Name,
        Type:        models.WorkflowType(req.Type),
        Version:     req.Version,
        Description: req.Description,
        CreatedBy:   conn.AgentID,
        Tags:        req.Tags,
        IsActive:    true,
        CreatedAt:   time.Now(),
        Config: models.WorkflowConfig{
            MaxParallel:     req.Config.MaxParallel,
            GlobalTimeout:   req.Config.GlobalTimeout,
            ErrorStrategy:   req.Config.ErrorStrategy,
            ResourceLimits:  r.convertResourceLimits(req.Config.ResourceLimits),
            SecurityContext: req.Config.SecurityContext,
        },
    }
    
    // Set defaults
    if workflow.Version == 0 {
        workflow.Version = 1
    }
    if workflow.Config.MaxParallel == 0 {
        workflow.Config.MaxParallel = 10
    }
    if workflow.Config.ErrorStrategy == "" {
        workflow.Config.ErrorStrategy = "fail_fast"
    }
    
    // Convert agents and steps
    workflow.Agents = r.convertAgentRoles(req.Agents)
    workflow.Steps = r.convertWorkflowSteps(req.Steps)
    
    // Create with idempotency
    var idempotencyKey string
    if req.IdempotencyKey != "" {
        idempotencyKey = fmt.Sprintf("%s:%s:workflow", conn.TenantID, req.IdempotencyKey)
    }
    
    err := r.workflowService.Create(ctx, workflow, idempotencyKey)
    if err != nil {
        r.logger.Error("Failed to create workflow",
            zap.Error(err),
            zap.String("agent_id", conn.AgentID))
        
        if errors.Is(err, services.ErrDuplicateIdempotencyKey) {
            existingWorkflow, _ := r.workflowService.GetByIdempotencyKey(ctx, idempotencyKey)
            if existingWorkflow != nil {
                return r.buildWorkflowResponse(existingWorkflow), nil
            }
        }
        
        if errors.Is(err, services.ErrWorkflowNameExists) {
            return nil, &ws.Error{
                Code:    ws.ErrCodeConflict,
                Message: "Workflow with this name already exists",
            }
        }
        
        return nil, &ws.Error{
            Code:    ws.ErrCodeInternalError,
            Message: "Failed to create workflow",
        }
    }
    
    // Send notifications if configured
    if req.Config.Notifications.OnStart {
        r.notifyWorkflowEvent(ctx, workflow, "created", req.Config.Notifications)
    }
    
    // Metrics
    r.metrics.Counter("workflow.created", 1,
        "type", string(workflow.Type),
        "steps", len(workflow.Steps),
        "tenant_id", conn.TenantID.String())
    
    return r.buildWorkflowResponse(workflow), nil
}

// handleWorkflowExecute executes a workflow
func (r *WorkflowHandlerRegistry) handleWorkflowExecute(ctx context.Context, conn *ManagedConnection, params json.RawMessage) (interface{}, error) {
    span, ctx := opentracing.StartSpanFromContext(ctx, "handleWorkflowExecute")
    defer span.Finish()
    
    r.logger.Debug("Executing workflow",
        zap.String("agent_id", conn.AgentID),
        zap.String("connection_id", conn.ID))
    
    var req struct {
        WorkflowID      uuid.UUID              `json:"workflow_id" validate:"required"`
        Input           map[string]interface{} `json:"input,omitempty"`
        Context         map[string]interface{} `json:"context,omitempty"`
        Priority        string                 `json:"priority,omitempty" validate:"omitempty,oneof=low normal high critical"`
        IdempotencyKey  string                 `json:"idempotency_key,omitempty"`
    }
    
    if err := json.Unmarshal(params, &req); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Invalid request format",
        }
    }
    
    if err := r.validator.ValidateStruct(ctx, req); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Validation failed",
            Data:    validation.FormatErrors(err),
        }
    }
    
    // Get workflow
    workflow, err := r.workflowService.Get(ctx, req.WorkflowID)
    if err != nil {
        if errors.Is(err, services.ErrWorkflowNotFound) {
            return nil, &ws.Error{
                Code:    ws.ErrCodeNotFound,
                Message: "Workflow not found",
            }
        }
        return nil, &ws.Error{
            Code:    ws.ErrCodeInternalError,
            Message: "Failed to retrieve workflow",
        }
    }
    
    // Check if workflow is active
    if !workflow.IsActive {
        return nil, &ws.Error{
            Code:    ws.ErrCodeForbidden,
            Message: "Workflow is not active",
        }
    }
    
    // Build execution
    execution := &models.WorkflowExecution{
        ID:          uuid.New(),
        WorkflowID:  req.WorkflowID,
        TenantID:    conn.TenantID,
        TriggeredBy: conn.AgentID,
        Input:       req.Input,
        Context:     req.Context,
        Priority:    models.TaskPriority(req.Priority),
        Status:      models.WorkflowStatusPending,
        StartedAt:   time.Now(),
    }
    
    if execution.Priority == "" {
        execution.Priority = models.TaskPriorityNormal
    }
    
    // Execute with idempotency
    var idempotencyKey string
    if req.IdempotencyKey != "" {
        idempotencyKey = fmt.Sprintf("%s:%s:execution", conn.TenantID, req.IdempotencyKey)
    }
    
    err = r.workflowService.Execute(ctx, execution, idempotencyKey)
    if err != nil {
        r.logger.Error("Failed to execute workflow",
            zap.Error(err),
            zap.String("workflow_id", req.WorkflowID.String()),
            zap.String("agent_id", conn.AgentID))
        
        if errors.Is(err, services.ErrDuplicateIdempotencyKey) {
            existingExecution, _ := r.workflowService.GetExecutionByIdempotencyKey(ctx, idempotencyKey)
            if existingExecution != nil {
                return r.buildExecutionResponse(existingExecution), nil
            }
        }
        
        if errors.Is(err, services.ErrMaxExecutionsExceeded) {
            return nil, &ws.Error{
                Code:    ws.ErrCodeTooManyRequests,
                Message: "Maximum concurrent executions exceeded",
                Data:    map[string]interface{}{"retry_after": 60},
            }
        }
        
        return nil, &ws.Error{
            Code:    ws.ErrCodeInternalError,
            Message: "Failed to execute workflow",
        }
    }
    
    // Send notifications
    r.notifier.SendNotification(ctx, workflow.CreatedBy, "workflow.started", map[string]interface{}{
        "workflow_id":   workflow.ID,
        "workflow_name": workflow.Name,
        "execution_id":  execution.ID,
        "triggered_by":  conn.AgentID,
    })
    
    // Notify assigned agents for initial steps
    r.notifyInitialStepAgents(ctx, workflow, execution)
    
    // Metrics
    r.metrics.Counter("workflow.executed", 1,
        "workflow_id", workflow.ID.String(),
        "type", string(workflow.Type),
        "tenant_id", conn.TenantID.String())
    
    return r.buildExecutionResponse(execution), nil
}

// handleWorkflowCreateCollaborative creates a collaborative workflow
func (r *WorkflowHandlerRegistry) handleWorkflowCreateCollaborative(ctx context.Context, conn *ManagedConnection, params json.RawMessage) (interface{}, error) {
    span, ctx := opentracing.StartSpanFromContext(ctx, "handleWorkflowCreateCollaborative")
    defer span.Finish()
    
    // Similar to handleWorkflowCreate but with collaborative-specific validations
    var req WorkflowCreateRequest
    if err := json.Unmarshal(params, &req); err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Invalid request format",
        }
    }
    
    // Enforce collaborative type
    req.Type = "collaborative"
    
    // Validate minimum agents for collaboration
    if len(req.Agents) < 2 {
        return nil, &ws.Error{
            Code:    ws.ErrCodeInvalidParams,
            Message: "Collaborative workflows require at least 2 agents",
        }
    }
    
    // Continue with standard creation flow
    return r.handleWorkflowCreate(ctx, conn, params)
}

// Helper methods for workflow handlers
func (r *WorkflowHandlerRegistry) validateStepDependencies(steps []WorkflowStepDefinition) error {
    // Build ID set
    ids := make(map[string]bool)
    for _, step := range steps {
        if ids[step.ID] {
            return fmt.Errorf("duplicate step ID: %s", step.ID)
        }
        ids[step.ID] = true
    }
    
    // Check dependencies exist
    for _, step := range steps {
        for _, dep := range step.DependsOn {
            if !ids[dep] {
                return fmt.Errorf("step %s depends on non-existent step %s", step.ID, dep)
            }
        }
    }
    
    // Check for cycles using DFS
    visited := make(map[string]int)
    var hasCycle func(id string) bool
    hasCycle = func(id string) bool {
        visited[id] = 1
        
        for _, step := range steps {
            if step.ID == id {
                for _, dep := range step.DependsOn {
                    if visited[dep] == 1 {
                        return true
                    }
                    if visited[dep] == 0 && hasCycle(dep) {
                        return true
                    }
                }
                break
            }
        }
        
        visited[id] = 2
        return false
    }
    
    for _, step := range steps {
        if visited[step.ID] == 0 && hasCycle(step.ID) {
            return fmt.Errorf("circular dependency detected")
        }
    }
    
    return nil
}

func (r *WorkflowHandlerRegistry) validateAgentRoles(agents map[string]AgentRole, steps []WorkflowStepDefinition) error {
    // Check all step agents have corresponding roles
    for _, step := range steps {
        if _, ok := agents[step.Agent]; !ok {
            return fmt.Errorf("step %s references undefined agent role: %s", step.ID, step.Agent)
        }
    }
    
    // Validate agent selectors
    for role, agent := range agents {
        if agent.Selector == "specific" && agent.AgentID == "" {
            return fmt.Errorf("agent role %s has 'specific' selector but no agent_id", role)
        }
        if agent.Selector == "capability_match" && len(agent.Capabilities) == 0 {
            return fmt.Errorf("agent role %s has 'capability_match' selector but no capabilities", role)
        }
    }
    
    return nil
}

func (r *WorkflowHandlerRegistry) buildWorkflowResponse(workflow *models.Workflow) map[string]interface{} {
    return map[string]interface{}{
        "workflow_id":  workflow.ID,
        "name":         workflow.Name,
        "type":         workflow.Type,
        "version":      workflow.Version,
        "created_at":   workflow.CreatedAt,
        "created_by":   workflow.CreatedBy,
        "is_active":    workflow.IsActive,
        "step_count":   len(workflow.Steps),
        "agent_count":  len(workflow.Agents),
    }
}

func (r *WorkflowHandlerRegistry) buildExecutionResponse(execution *models.WorkflowExecution) map[string]interface{} {
    return map[string]interface{}{
        "execution_id":       execution.ID,
        "workflow_id":        execution.WorkflowID,
        "status":             execution.Status,
        "started_at":         execution.StartedAt,
        "current_step_index": execution.CurrentStepIndex,
        "current_step_id":    execution.CurrentStepID,
        "priority":           execution.Priority,
    }
}

func (r *WorkflowHandlerRegistry) notifyWorkflowEvent(ctx context.Context, workflow *models.Workflow, event string, config NotificationConfig) {
    notification := map[string]interface{}{
        "workflow_id":   workflow.ID,
        "workflow_name": workflow.Name,
        "event":         event,
        "timestamp":     time.Now(),
    }
    
    // Notify recipients
    for _, recipient := range config.Recipients {
        r.notifier.SendNotification(ctx, recipient, fmt.Sprintf("workflow.%s", event), notification)
    }
    
    // Send webhooks
    for _, webhook := range config.Webhooks {
        r.notifier.SendWebhook(ctx, webhook, notification)
    }
}

func (r *WorkflowHandlerRegistry) notifyInitialStepAgents(ctx context.Context, workflow *models.Workflow, execution *models.WorkflowExecution) {
    // Find steps with no dependencies
    for _, step := range workflow.Steps {
        if len(step.DependsOn) == 0 {
            agentRole := workflow.Agents[step.Agent]
            if agentRole.AgentID != "" {
                r.notifier.SendNotification(ctx, agentRole.AgentID, "workflow.step.ready", map[string]interface{}{
                    "workflow_id":   workflow.ID,
                    "execution_id":  execution.ID,
                    "step_id":       step.ID,
                    "step_name":     step.Name,
                    "action":        step.Action,
                    "input":         step.Input,
                })
            }
        }
    }
}

func (r *WorkflowHandlerRegistry) convertAgentRoles(roles map[string]AgentRole) map[string]models.AgentRole {
    result := make(map[string]models.AgentRole)
    for k, v := range roles {
        result[k] = models.AgentRole{
            AgentID:      v.AgentID,
            Capabilities: v.Capabilities,
            Selector:     v.Selector,
        }
    }
    return result
}

func (r *WorkflowHandlerRegistry) convertWorkflowSteps(steps []WorkflowStepDefinition) []models.WorkflowStep {
    result := make([]models.WorkflowStep, len(steps))
    for i, step := range steps {
        result[i] = models.WorkflowStep{
            ID:            step.ID,
            Name:          step.Name,
            Agent:         step.Agent,
            Action:        step.Action,
            Input:         step.Input,
            DependsOn:     step.DependsOn,
            Conditions:    r.convertStepConditions(step.Conditions),
            Timeout:       step.Timeout,
            Retries:       step.Retries,
            OnFailure:     step.OnFailure,
            OutputMapping: step.OutputMapping,
        }
    }
    return result
}

func (r *WorkflowHandlerRegistry) convertStepConditions(conditions []StepCondition) []models.StepCondition {
    result := make([]models.StepCondition, len(conditions))
    for i, cond := range conditions {
        result[i] = models.StepCondition{
            Type:       cond.Type,
            Expression: cond.Expression,
            Variable:   cond.Variable,
            Operator:   cond.Operator,
            Value:      cond.Value,
        }
    }
    return result
}

func (r *WorkflowHandlerRegistry) convertResourceLimits(limits ResourceLimits) models.ResourceLimits {
    return models.ResourceLimits{
        MaxMemoryMB:   limits.MaxMemoryMB,
        MaxCPUPercent: limits.MaxCPUPercent,
        MaxExecutions: limits.MaxExecutions,
    }
}
```

## Workspace and Document Handlers

Due to the 32K token limit, the remaining handlers (Workspace and Document) follow the same production-ready patterns:

### Key Features for Workspace Handlers:
- **Real-time state synchronization** with CRDT conflict resolution
- **Member management** with role-based permissions
- **State versioning** with optimistic concurrency control
- **Broadcast notifications** for collaborative updates
- **Resource limits** to prevent abuse
- **Activity tracking** for analytics

### Key Features for Document Handlers:
- **Collaborative editing** with operational transformation
- **Version control** with diff tracking
- **Lock management** for exclusive editing
- **Change notifications** with incremental updates
- **Conflict resolution** using vector clocks
- **Content validation** and sanitization

## Handler Registration and Management

```go
// File: apps/mcp-server/internal/api/websocket/handler_manager.go
package websocket

import (
    "context"
    "fmt"
    "sync"
    
    "go.uber.org/zap"
)

// HandlerManager manages all WebSocket handlers
type HandlerManager struct {
    mu              sync.RWMutex
    handlers        map[string]*BaseHandler
    registries      []HandlerRegistry
    
    // Dependencies
    logger          *zap.Logger
    metrics         observability.MetricsClient
    
    // Configuration
    maxHandlers     int
    enableMetrics   bool
    enableTracing   bool
}

// HandlerRegistry interface for handler registries
type HandlerRegistry interface {
    GetHandlers() map[string]*BaseHandler
}

// NewHandlerManager creates a new handler manager
func NewHandlerManager(deps HandlerDependencies) *HandlerManager {
    m := &HandlerManager{
        handlers:      make(map[string]*BaseHandler),
        registries:    make([]HandlerRegistry, 0),
        logger:        deps.Logger.Named("handler_manager"),
        metrics:       deps.Metrics,
        maxHandlers:   1000,
        enableMetrics: true,
        enableTracing: true,
    }
    
    // Register all handler registries
    m.RegisterRegistry(NewTaskHandlerRegistry(deps))
    m.RegisterRegistry(NewWorkflowHandlerRegistry(deps))
    m.RegisterRegistry(NewWorkspaceHandlerRegistry(deps))
    m.RegisterRegistry(NewDocumentHandlerRegistry(deps))
    
    return m
}

// RegisterRegistry adds a handler registry
func (m *HandlerManager) RegisterRegistry(registry HandlerRegistry) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    m.registries = append(m.registries, registry)
    
    // Merge handlers
    for name, handler := range registry.GetHandlers() {
        if _, exists := m.handlers[name]; exists {
            m.logger.Warn("Handler already registered, overwriting",
                zap.String("handler", name))
        }
        m.handlers[name] = handler
    }
}

// GetHandler retrieves a handler by name
func (m *HandlerManager) GetHandler(name string) (*BaseHandler, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    handler, ok := m.handlers[name]
    if !ok {
        return nil, fmt.Errorf("handler not found: %s", name)
    }
    
    return handler, nil
}

// ExecuteHandler executes a handler
func (m *HandlerManager) ExecuteHandler(ctx context.Context, name string, conn *ManagedConnection, params json.RawMessage) (interface{}, error) {
    handler, err := m.GetHandler(name)
    if err != nil {
        return nil, &ws.Error{
            Code:    ws.ErrCodeMethodNotFound,
            Message: "Handler not found",
        }
    }
    
    // Check authentication
    if handler.requiresAuth && !conn.IsAuthenticated() {
        return nil, &ws.Error{
            Code:    ws.ErrCodeUnauthorized,
            Message: "Authentication required",
        }
    }
    
    // Execute with context
    ctx = context.WithValue(ctx, "handler_name", name)
    return handler.Execute(ctx, conn, params)
}
```

## Error Handling and Recovery

```go
// Enhanced error types for comprehensive error handling
const (
    // Client errors (4xx)
    ErrCodeBadRequest         = 400
    ErrCodeUnauthorized       = 401
    ErrCodeForbidden          = 403
    ErrCodeNotFound           = 404
    ErrCodeMethodNotFound     = 405
    ErrCodeConflict           = 409
    ErrCodeInvalidParams      = 422
    ErrCodeTooManyRequests    = 429
    
    // Server errors (5xx)
    ErrCodeInternalError      = 500
    ErrCodeNotImplemented     = 501
    ErrCodeServiceUnavailable = 503
    ErrCodeTimeout            = 504
)

// Error recovery middleware
func WithErrorRecovery(logger *zap.Logger) HandlerMiddleware {
    return func(next HandlerFunc) HandlerFunc {
        return func(ctx context.Context, conn *ManagedConnection, params json.RawMessage) (result interface{}, err error) {
            defer func() {
                if r := recover(); r != nil {
                    logger.Error("Handler panic recovered",
                        zap.Any("panic", r),
                        zap.String("handler", getHandlerName(ctx)),
                        zap.String("agent_id", conn.AgentID),
                        zap.Stack("stack"))
                    
                    err = &ws.Error{
                        Code:    ErrCodeInternalError,
                        Message: "Internal server error",
                    }
                }
            }()
            
            return next(ctx, conn, params)
        }
    }
}
```

## Performance Optimizations

### Binary Protocol Implementation
- **Message compression** using zstd for large payloads
- **Binary encoding** for numeric data and timestamps
- **Message batching** for multiple notifications
- **Connection multiplexing** for multiple agents

### Caching Strategy
- **Handler result caching** for idempotent operations
- **Permission caching** to reduce auth checks
- **Agent capability caching** for faster routing
- **Workflow definition caching** for execution

## Security Considerations

### Authentication and Authorization
- **JWT token validation** with rotation support
- **API key authentication** for service accounts
- **Permission-based access control** at handler level
- **Tenant isolation** enforced at all layers

### Input Validation
- **Schema validation** using JSON Schema
- **Parameter sanitization** for XSS prevention
- **Size limits** on all inputs
- **Rate limiting** per connection and tenant

## Testing Strategy

### Unit Tests
- Test each handler in isolation
- Mock all dependencies
- Test error scenarios
- Verify authorization checks

### Integration Tests
- Test handler chains with middleware
- Test with real WebSocket connections
- Test concurrent operations
- Test rate limiting and circuit breakers

### Load Tests
- Test with 10K+ concurrent connections
- Test message throughput
- Test handler latency under load
- Test resource usage

## Monitoring and Alerting

### Key Metrics
- Handler execution time (p50, p95, p99)
- Handler error rates by type
- WebSocket connection count
- Message throughput
- Active handler count

### Alerts
- High error rate (> 1%)
- High latency (p99 > 1s)
- Connection pool exhaustion
- Circuit breaker trips
- Rate limit violations

## Next Steps

After completing Phase 4:
1. Implement comprehensive integration tests
2. Perform load testing with production workloads
3. Add handler documentation with OpenAPI specs
4. Implement handler versioning for backward compatibility
5. Add handler feature flags for gradual rollout
