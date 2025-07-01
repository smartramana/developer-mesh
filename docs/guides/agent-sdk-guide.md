# MCP Agent SDK Guide

> **Purpose**: Comprehensive guide for using the MCP SDK to develop AI agents
> **Audience**: Developers building agents using the official MCP SDK
> **Scope**: SDK installation, core APIs, utilities, best practices, and advanced features

## Table of Contents

1. [Getting Started](#getting-started)
2. [SDK Architecture](#sdk-architecture)
3. [Core Components](#core-components)
4. [Agent Development](#agent-development)
5. [WebSocket Client](#websocket-client)
6. [Task Processing](#task-processing)
7. [State Management](#state-management)
8. [Error Handling](#error-handling)
9. [Testing Utilities](#testing-utilities)
10. [Advanced Features](#advanced-features)
11. [Migration Guide](#migration-guide)

## Getting Started

### Installation

```bash
# Install the MCP Agent SDK
go get github.com/S-Corkum/devops-mcp/sdk/agent@latest

# Install CLI tools
go install github.com/S-Corkum/devops-mcp/sdk/agent/cmd/mcp-agent@latest
```

### Quick Start

```go
package main

import (
    "context"
    "log"
    
    "github.com/S-Corkum/devops-mcp/sdk/agent"
    "github.com/S-Corkum/devops-mcp/sdk/agent/capabilities"
)

func main() {
    // Create agent configuration
    config := agent.Config{
        AgentID:   "my-agent-001",
        AgentType: "analyzer",
        ServerURL: "wss://mcp.example.com/ws",
        APIKey:    "your-api-key",
    }
    
    // Create new agent
    myAgent, err := agent.New(config)
    if err != nil {
        log.Fatal(err)
    }
    
    // Define capabilities
    myAgent.AddCapability(capabilities.Capability{
        Name:       "code_analysis",
        Confidence: 0.95,
        Languages:  []string{"go", "python"},
    })
    
    // Set task handler
    myAgent.OnTask(func(ctx context.Context, task agent.Task) error {
        log.Printf("Processing task: %s", task.ID)
        
        // Process task...
        result := processTask(task)
        
        // Complete task
        return myAgent.CompleteTask(task.ID, result)
    })
    
    // Start agent
    if err := myAgent.Start(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

## SDK Architecture

### Package Structure

```
github.com/S-Corkum/devops-mcp/sdk/agent/
├── agent.go              # Core agent implementation
├── client.go             # WebSocket client
├── config.go             # Configuration structures
├── capabilities/         # Capability definitions
│   ├── types.go
│   └── registry.go
├── tasks/               # Task processing
│   ├── handler.go
│   ├── processor.go
│   └── queue.go
├── state/               # State management
│   ├── store.go
│   └── crdt.go
├── protocol/            # Protocol implementation
│   ├── messages.go
│   ├── binary.go
│   └── codec.go
├── metrics/             # Metrics collection
│   ├── collector.go
│   └── prometheus.go
├── testing/             # Testing utilities
│   ├── mock.go
│   └── server.go
└── cmd/                 # CLI tools
    └── mcp-agent/
```

### Design Principles

1. **Simple API**: Easy to use for common cases
2. **Extensible**: Hooks and interfaces for customization
3. **Type-Safe**: Strong typing with generics where appropriate
4. **Performance**: Optimized for high-throughput scenarios
5. **Testable**: Built-in testing utilities and mocks

## Core Components

### Agent Interface

```go
type Agent interface {
    // Lifecycle
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    
    // Registration
    AddCapability(cap capabilities.Capability)
    RemoveCapability(name string)
    UpdateCapability(cap capabilities.Capability)
    
    // Task handling
    OnTask(handler TaskHandler)
    CompleteTask(taskID string, result interface{}) error
    FailTask(taskID string, err error) error
    
    // State
    GetState() State
    SetState(state State)
    
    // Metrics
    Metrics() Metrics
}

type TaskHandler func(ctx context.Context, task Task) error
```

### Configuration

```go
type Config struct {
    // Required fields
    AgentID   string `json:"agent_id" validate:"required"`
    AgentType string `json:"agent_type" validate:"required"`
    ServerURL string `json:"server_url" validate:"required,url"`
    APIKey    string `json:"api_key" validate:"required"`
    
    // Optional fields
    Name        string        `json:"name,omitempty"`
    MaxTasks    int           `json:"max_tasks,omitempty"`
    Timeout     time.Duration `json:"timeout,omitempty"`
    RetryConfig *RetryConfig  `json:"retry,omitempty"`
    TLS         *TLSConfig    `json:"tls,omitempty"`
    
    // Advanced options
    BinaryProtocol bool            `json:"binary_protocol,omitempty"`
    Compression    CompressionType `json:"compression,omitempty"`
    BufferSize     int             `json:"buffer_size,omitempty"`
}

// Load configuration from file
func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    
    var config Config
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, err
    }
    
    if err := config.Validate(); err != nil {
        return nil, err
    }
    
    return &config, nil
}
```

## Agent Development

### Creating a Custom Agent

```go
package myagent

import (
    "context"
    "fmt"
    
    "github.com/S-Corkum/devops-mcp/sdk/agent"
    "github.com/S-Corkum/devops-mcp/sdk/agent/capabilities"
)

// MyCustomAgent extends the base agent
type MyCustomAgent struct {
    *agent.BaseAgent
    
    // Custom fields
    processor *DataProcessor
    cache     *Cache
}

// NewMyCustomAgent creates a new custom agent
func NewMyCustomAgent(config agent.Config) (*MyCustomAgent, error) {
    // Create base agent
    base, err := agent.NewBase(config)
    if err != nil {
        return nil, err
    }
    
    return &MyCustomAgent{
        BaseAgent: base,
        processor: NewDataProcessor(),
        cache:     NewCache(),
    }, nil
}

// Start initializes and starts the agent
func (a *MyCustomAgent) Start(ctx context.Context) error {
    // Register capabilities
    a.registerCapabilities()
    
    // Set up task handlers
    a.OnTask(a.handleTask)
    
    // Start background workers
    go a.backgroundWorker(ctx)
    
    // Start base agent
    return a.BaseAgent.Start(ctx)
}

func (a *MyCustomAgent) registerCapabilities() {
    a.AddCapability(capabilities.Capability{
        Name:        "data_processing",
        Confidence:  0.95,
        Description: "Advanced data processing and analysis",
        Constraints: capabilities.Constraints{
            MaxInputSize:  10 * 1024 * 1024, // 10MB
            MaxOutputSize: 5 * 1024 * 1024,  // 5MB
            Timeout:       5 * time.Minute,
        },
    })
}

func (a *MyCustomAgent) handleTask(ctx context.Context, task agent.Task) error {
    // Check cache
    if result, found := a.cache.Get(task.CacheKey()); found {
        return a.CompleteTask(task.ID, result)
    }
    
    // Process task
    result, err := a.processor.Process(ctx, task.Input)
    if err != nil {
        return a.FailTask(task.ID, err)
    }
    
    // Cache result
    a.cache.Set(task.CacheKey(), result)
    
    // Complete task
    return a.CompleteTask(task.ID, result)
}
```

### Capability Management

```go
import "github.com/S-Corkum/devops-mcp/sdk/agent/capabilities"

// Define custom capabilities
var (
    CodeReviewCap = capabilities.Capability{
        Name:       "code_review",
        Confidence: 0.92,
        Languages:  []string{"go", "python", "javascript"},
        Specialties: []string{"security", "performance", "style"},
        Constraints: capabilities.Constraints{
            MaxInputSize: 1 * 1024 * 1024, // 1MB
            Timeout:      2 * time.Minute,
        },
    }
    
    SecurityAuditCap = capabilities.Capability{
        Name:       "security_audit",
        Confidence: 0.88,
        Specialties: []string{"owasp", "cve", "vulnerability"},
        Requirements: []string{"code_access", "dependency_list"},
    }
)

// Dynamic capability updates
func (a *MyCustomAgent) UpdateCapabilities() {
    // Update based on performance
    metrics := a.Metrics()
    
    if metrics.SuccessRate("code_review") > 0.95 {
        // Increase confidence
        cap := a.GetCapability("code_review")
        cap.Confidence = min(cap.Confidence * 1.05, 1.0)
        a.UpdateCapability(cap)
    }
    
    // Add new capability after training
    if a.isTrainingComplete("rust") {
        a.AddCapability(capabilities.Capability{
            Name:       "rust_analysis",
            Confidence: 0.80,
            Languages:  []string{"rust"},
        })
    }
}

// Capability-based task routing
func (a *MyCustomAgent) CanHandleTask(task agent.Task) bool {
    required := task.RequiredCapabilities
    
    for _, req := range required {
        cap := a.GetCapability(req.Name)
        if cap == nil || cap.Confidence < req.MinConfidence {
            return false
        }
    }
    
    return true
}
```

## WebSocket Client

### Advanced WebSocket Configuration

```go
import (
    "github.com/S-Corkum/devops-mcp/sdk/agent/client"
    "github.com/gorilla/websocket"
)

// Configure WebSocket client
wsConfig := client.WebSocketConfig{
    URL:    "wss://mcp.example.com/ws",
    APIKey: "your-api-key",
    
    // Connection options
    DialTimeout:     10 * time.Second,
    HandshakeTimeout: 10 * time.Second,
    
    // Compression
    EnableCompression: true,
    CompressionLevel:  websocket.CompressionLevel(6),
    
    // Keepalive
    PingInterval: 30 * time.Second,
    PongTimeout:  10 * time.Second,
    
    // Reconnection
    ReconnectInterval: 5 * time.Second,
    MaxReconnectDelay: 5 * time.Minute,
    ReconnectBackoff:  2.0,
    
    // Binary protocol
    UseBinaryProtocol: true,
    
    // TLS
    TLSConfig: &tls.Config{
        MinVersion: tls.VersionTLS13,
    },
}

// Create client with custom config
wsClient, err := client.NewWebSocketClient(wsConfig)
if err != nil {
    return err
}

// Set event handlers
wsClient.OnConnect(func() {
    log.Println("Connected to MCP server")
})

wsClient.OnDisconnect(func(err error) {
    log.Printf("Disconnected: %v", err)
})

wsClient.OnMessage(func(msg client.Message) {
    log.Printf("Received: %s", msg.Method)
})

// Connect
if err := wsClient.Connect(); err != nil {
    return err
}
```

### Binary Protocol Implementation

```go
import "github.com/S-Corkum/devops-mcp/sdk/agent/protocol"

// Enable binary protocol for performance
agent.UseBinaryProtocol(true)

// Custom message encoding
type CustomMessage struct {
    Type    protocol.MessageType
    Payload interface{}
}

func (m CustomMessage) Encode() ([]byte, error) {
    // Use SDK's binary encoder
    encoder := protocol.NewBinaryEncoder()
    
    if err := encoder.WriteHeader(protocol.Header{
        Magic:    protocol.MagicNumber,
        Version:  1,
        Type:     m.Type,
        Method:   protocol.MethodCustom,
    }); err != nil {
        return nil, err
    }
    
    // Encode payload
    if err := encoder.WritePayload(m.Payload); err != nil {
        return nil, err
    }
    
    return encoder.Bytes(), nil
}

// Message batching for efficiency
batcher := protocol.NewMessageBatcher(protocol.BatchConfig{
    MaxBatchSize:     100,
    MaxBatchBytes:    1024 * 1024, // 1MB
    FlushInterval:    100 * time.Millisecond,
    CompressionThreshold: 1024, // Compress if > 1KB
})

// Add messages to batch
batcher.Add(msg1)
batcher.Add(msg2)

// Flush batch
if err := batcher.Flush(); err != nil {
    return err
}
```

## Task Processing

### Task Processor Framework

```go
import "github.com/S-Corkum/devops-mcp/sdk/agent/tasks"

// Create task processor with middleware
processor := tasks.NewProcessor(
    tasks.WithConcurrency(10),
    tasks.WithTimeout(5 * time.Minute),
    tasks.WithRetry(3, time.Second),
    tasks.WithCircuitBreaker(5, 30 * time.Second),
)

// Add middleware
processor.Use(
    tasks.LoggingMiddleware(),
    tasks.MetricsMiddleware(),
    tasks.TracingMiddleware(),
    tasks.RateLimitMiddleware(100), // 100 tasks/sec
)

// Register task handlers
processor.Register("code_review", handleCodeReview)
processor.Register("security_scan", handleSecurityScan)
processor.Register("performance_analysis", handlePerformance)

// Custom middleware
func CustomMiddleware() tasks.Middleware {
    return func(next tasks.Handler) tasks.Handler {
        return func(ctx context.Context, task agent.Task) error {
            // Pre-processing
            start := time.Now()
            
            // Call next handler
            err := next(ctx, task)
            
            // Post-processing
            duration := time.Since(start)
            log.Printf("Task %s took %v", task.ID, duration)
            
            return err
        }
    }
}

// Task handler with context
func handleCodeReview(ctx context.Context, task agent.Task) error {
    // Extract task data
    var request CodeReviewRequest
    if err := task.UnmarshalInput(&request); err != nil {
        return fmt.Errorf("invalid request: %w", err)
    }
    
    // Process with timeout
    ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
    defer cancel()
    
    review, err := performCodeReview(ctx, request)
    if err != nil {
        return err
    }
    
    // Return result
    return task.Complete(review)
}
```

### Task Queue Management

```go
// Priority queue for tasks
queue := tasks.NewPriorityQueue(tasks.QueueConfig{
    MaxSize:     1000,
    MaxMemory:   100 * 1024 * 1024, // 100MB
    EvictionPolicy: tasks.LRU,
})

// Add task with priority
queue.Push(tasks.PriorityTask{
    Task:     task,
    Priority: tasks.PriorityHigh,
})

// Process tasks by priority
processor := tasks.NewQueueProcessor(queue, handler)
processor.Start(ctx)

// Monitor queue metrics
metrics := queue.Metrics()
log.Printf("Queue size: %d, Memory: %d bytes", 
    metrics.Size, metrics.MemoryUsage)
```

## State Management

### CRDT-Based State Synchronization

```go
import "github.com/S-Corkum/devops-mcp/sdk/agent/state"

// Create CRDT state store
store := state.NewCRDTStore(state.StoreConfig{
    AgentID: "agent-001",
    SyncInterval: 5 * time.Second,
})

// G-Counter for distributed counting
counter := store.GCounter("tasks_completed")
counter.Increment(1)

// OR-Set for distributed sets
activeSet := store.ORSet("active_tasks")
activeSet.Add("task-123")
activeSet.Add("task-456")

// LWW-Map for last-write-wins map
config := store.LWWMap("agent_config")
config.Set("max_tasks", 10)
config.Set("timeout", "5m")

// Vector clock for causality tracking
clock := store.VectorClock()
clock.Increment("agent-001")

// Sync with other agents
if err := store.Sync(); err != nil {
    log.Printf("Sync failed: %v", err)
}

// Subscribe to state changes
store.OnChange(func(key string, value interface{}) {
    log.Printf("State changed: %s = %v", key, value)
})
```

### Persistent State

```go
// State persistence with snapshots
persister := state.NewPersister(state.PersisterConfig{
    Path:             "/var/lib/mcp-agent/state",
    SnapshotInterval: 5 * time.Minute,
    Compression:      true,
})

// Save state
if err := persister.SaveSnapshot(store); err != nil {
    return err
}

// Load state
if err := persister.LoadSnapshot(store); err != nil {
    return err
}

// Automatic persistence
store.EnablePersistence(persister)
```

## Error Handling

### Comprehensive Error Handling

```go
import "github.com/S-Corkum/devops-mcp/sdk/agent/errors"

// Define custom errors
var (
    ErrTaskTimeout = errors.New("task_timeout", "Task execution timed out")
    ErrInvalidInput = errors.New("invalid_input", "Task input validation failed")
    ErrResourceLimit = errors.New("resource_limit", "Resource limit exceeded")
)

// Error with context
err := errors.Wrap(ErrTaskTimeout, "processing task %s", taskID).
    WithCode(errors.CodeTimeout).
    WithMetadata(map[string]interface{}{
        "task_id": taskID,
        "timeout": timeout,
    })

// Error handling middleware
func ErrorHandlingMiddleware() tasks.Middleware {
    return func(next tasks.Handler) tasks.Handler {
        return func(ctx context.Context, task agent.Task) error {
            err := next(ctx, task)
            
            if err == nil {
                return nil
            }
            
            // Classify error
            switch errors.Code(err) {
            case errors.CodeTimeout:
                // Retry with extended timeout
                return retryWithBackoff(ctx, task, err)
                
            case errors.CodeRateLimit:
                // Queue for later
                return queueForRetry(task, err)
                
            case errors.CodeInvalidInput:
                // Fail immediately
                return task.Fail(err)
                
            default:
                // Log and fail
                log.Printf("Unexpected error: %+v", err)
                return task.Fail(err)
            }
        }
    }
}

// Circuit breaker for error prevention
breaker := errors.NewCircuitBreaker(errors.BreakerConfig{
    FailureThreshold: 5,
    SuccessThreshold: 2,
    Timeout:          30 * time.Second,
})

// Use circuit breaker
err := breaker.Execute(func() error {
    return riskyOperation()
})

if err == errors.ErrCircuitOpen {
    // Circuit is open, use fallback
    return fallbackOperation()
}
```

## Testing Utilities

### Mock Agent for Testing

```go
import "github.com/S-Corkum/devops-mcp/sdk/agent/testing"

func TestTaskHandler(t *testing.T) {
    // Create mock agent
    mockAgent := testing.NewMockAgent(t)
    
    // Set expectations
    mockAgent.ExpectTask(agent.Task{
        ID:   "test-001",
        Type: "code_review",
        Input: map[string]interface{}{
            "code": "func main() {}",
        },
    }).Return(map[string]interface{}{
        "issues": 0,
        "score":  100,
    }, nil)
    
    // Create handler
    handler := NewCodeReviewHandler(mockAgent)
    
    // Test
    result, err := handler.Process(context.Background(), testTask)
    assert.NoError(t, err)
    assert.Equal(t, 0, result.Issues)
    
    // Verify expectations
    mockAgent.AssertExpectations()
}

// Test server for integration tests
func TestAgentIntegration(t *testing.T) {
    // Start test MCP server
    server := testing.NewTestServer(t, testing.ServerConfig{
        Port: 8080,
        TLS:  false,
    })
    defer server.Stop()
    
    // Create agent
    agent := createTestAgent(server.URL())
    
    // Start agent
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    go agent.Start(ctx)
    
    // Wait for registration
    server.WaitForAgent(agent.ID, 5*time.Second)
    
    // Send test task
    task := server.SendTask(agent.ID, testing.Task{
        Type:  "test",
        Input: "test data",
    })
    
    // Wait for completion
    result := server.WaitForTaskCompletion(task.ID, 10*time.Second)
    assert.NotNil(t, result)
}
```

### Benchmarking

```go
import "github.com/S-Corkum/devops-mcp/sdk/agent/testing"

func BenchmarkTaskProcessing(b *testing.B) {
    agent := createBenchmarkAgent()
    ctx := context.Background()
    
    b.ResetTimer()
    
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            task := generateTask()
            if err := agent.ProcessTask(ctx, task); err != nil {
                b.Fatal(err)
            }
        }
    })
    
    b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "tasks/sec")
}

// Load testing
func TestAgentLoad(t *testing.T) {
    load := testing.NewLoadTest(testing.LoadConfig{
        Duration:       5 * time.Minute,
        TasksPerSecond: 100,
        Concurrency:    10,
    })
    
    agent := createTestAgent()
    
    results := load.Run(agent)
    
    assert.Less(t, results.ErrorRate, 0.01)
    assert.Less(t, results.P99Latency, 100*time.Millisecond)
    assert.Greater(t, results.Throughput, 90.0)
}
```

## Advanced Features

### Plugin System

```go
import "github.com/S-Corkum/devops-mcp/sdk/agent/plugins"

// Define plugin interface
type AnalyzerPlugin interface {
    Name() string
    Analyze(ctx context.Context, data []byte) (interface{}, error)
}

// Register plugins
registry := plugins.NewRegistry()
registry.Register("security", NewSecurityPlugin())
registry.Register("performance", NewPerformancePlugin())

// Use plugins in agent
func (a *MyAgent) HandleAnalysis(ctx context.Context, task agent.Task) error {
    // Get requested analyzers
    analyzers := task.GetStringSlice("analyzers")
    
    results := make(map[string]interface{})
    
    // Run each analyzer
    for _, name := range analyzers {
        plugin := registry.Get(name)
        if plugin == nil {
            continue
        }
        
        result, err := plugin.Analyze(ctx, task.Input)
        if err != nil {
            log.Printf("Plugin %s failed: %v", name, err)
            continue
        }
        
        results[name] = result
    }
    
    return task.Complete(results)
}

// Hot-reload plugins
watcher := plugins.NewWatcher("/etc/mcp-agent/plugins")
watcher.OnChange(func(event plugins.Event) {
    if event.Type == plugins.EventAdd {
        plugin, err := plugins.Load(event.Path)
        if err == nil {
            registry.Register(plugin.Name(), plugin)
        }
    }
})
```

### Distributed Tracing

```go
import (
    "github.com/S-Corkum/devops-mcp/sdk/agent/tracing"
    "go.opentelemetry.io/otel"
)

// Initialize tracing
tracer := tracing.NewTracer(tracing.Config{
    ServiceName: "my-agent",
    Endpoint:    "http://jaeger:14268/api/traces",
    SampleRate:  0.1,
})

// Trace task processing
func (a *MyAgent) ProcessTask(ctx context.Context, task agent.Task) error {
    ctx, span := tracer.Start(ctx, "process_task",
        trace.WithAttributes(
            attribute.String("task.id", task.ID),
            attribute.String("task.type", task.Type),
        ),
    )
    defer span.End()
    
    // Process with tracing
    result, err := a.process(ctx, task)
    
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return err
    }
    
    span.SetAttributes(
        attribute.Int("result.size", len(result)),
    )
    
    return nil
}
```

### Multi-Model Support

```go
import "github.com/S-Corkum/devops-mcp/sdk/agent/models"

// Model manager for multi-model agents
manager := models.NewModelManager()

// Register models
manager.Register("gpt-4", models.OpenAIProvider{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    Model:  "gpt-4-1106-preview",
})

manager.Register("claude", models.AnthropicProvider{
    APIKey: os.Getenv("ANTHROPIC_API_KEY"),
    Model:  "claude-3-opus-20240229",
})

manager.Register("llama", models.BedrockProvider{
    Region: "us-east-1",
    Model:  "meta.llama2-70b-chat-v1",
})

// Select model based on task
func (a *MyAgent) SelectModel(task agent.Task) models.Model {
    // Cost-optimized selection
    if task.Priority == agent.PriorityLow {
        return manager.Get("llama") // Cheaper option
    }
    
    // Capability-based selection
    if task.RequiresCapability("reasoning") {
        return manager.Get("claude")
    }
    
    // Default to GPT-4
    return manager.Get("gpt-4")
}

// Use selected model
model := a.SelectModel(task)
response, err := model.Complete(ctx, models.Request{
    Messages: []models.Message{
        {Role: "system", Content: systemPrompt},
        {Role: "user", Content: task.Input},
    },
    MaxTokens:   2000,
    Temperature: 0.7,
})
```

## Migration Guide

### Migrating from v1 to v2

```go
// v1 code
agent := agent.New(agentID, agentType)
agent.Connect(serverURL, apiKey)
agent.RegisterHandler(handler)
agent.Start()

// v2 code
config := agent.Config{
    AgentID:   agentID,
    AgentType: agentType,
    ServerURL: serverURL,
    APIKey:    apiKey,
}

agent, err := agent.New(config)
if err != nil {
    return err
}

agent.OnTask(handler)

if err := agent.Start(context.Background()); err != nil {
    return err
}
```

### Breaking Changes

1. **Configuration**: Now uses structured config
2. **Context**: All methods now require context
3. **Error Handling**: Returns errors instead of panicking
4. **Task Interface**: Simplified with helper methods
5. **Metrics**: Now uses OpenTelemetry

### Compatibility Layer

```go
import "github.com/S-Corkum/devops-mcp/sdk/agent/compat"

// Use v1 compatibility layer
agent := compat.NewV1Agent(agentID, agentType)
agent.Connect(serverURL, apiKey)
agent.RegisterHandler(handler)
agent.Start()
```

## Best Practices

### 1. Configuration Management

```go
// Use environment variables with defaults
config := agent.Config{
    AgentID:   getEnv("AGENT_ID", "default-agent"),
    AgentType: getEnv("AGENT_TYPE", "general"),
    ServerURL: getEnv("MCP_SERVER_URL", "wss://localhost:8080/ws"),
    APIKey:    mustGetEnv("MCP_API_KEY"), // Required
}

// Validate configuration
if err := config.Validate(); err != nil {
    log.Fatalf("Invalid config: %v", err)
}
```

### 2. Graceful Shutdown

```go
// Handle shutdown signals
ctx, cancel := context.WithCancel(context.Background())

sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

go func() {
    <-sigChan
    log.Println("Shutting down...")
    cancel()
}()

// Start with context
if err := agent.Start(ctx); err != nil {
    log.Fatal(err)
}

// Graceful shutdown
shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
defer shutdownCancel()

if err := agent.Stop(shutdownCtx); err != nil {
    log.Printf("Shutdown error: %v", err)
}
```

### 3. Resource Management

```go
// Set resource limits
agent.SetResourceLimits(agent.ResourceLimits{
    MaxMemory:      2 * 1024 * 1024 * 1024, // 2GB
    MaxCPU:         2.0,                     // 2 cores
    MaxConcurrency: 10,                      // 10 concurrent tasks
})

// Monitor resource usage
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        metrics := agent.ResourceMetrics()
        if metrics.MemoryUsage > 0.9 {
            log.Warn("High memory usage: %.2f%%", metrics.MemoryUsage*100)
        }
    }
}()
```

### 4. Debugging

```go
// Enable debug logging
if debug := os.Getenv("DEBUG"); debug == "true" {
    agent.EnableDebugLogging()
}

// Request/response logging
agent.OnRequest(func(msg agent.Message) {
    log.Printf(">> %s %s", msg.Method, msg.ID)
})

agent.OnResponse(func(msg agent.Message) {
    log.Printf("<< %s %s", msg.Method, msg.ID)
})

// Performance profiling
import _ "net/http/pprof"

go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

## CLI Tools

### Agent Generator

```bash
# Generate new agent project
mcp-agent init my-agent \
    --type analyzer \
    --capabilities code_review,security \
    --model gpt-4

# Project structure created:
# my-agent/
# ├── cmd/agent/main.go
# ├── internal/agent/agent.go
# ├── internal/handlers/handlers.go
# ├── configs/agent.yaml
# ├── Dockerfile
# ├── Makefile
# └── README.md
```

### Agent Manager

```bash
# List running agents
mcp-agent list

# Start agent
mcp-agent start --config agent.yaml

# Stop agent
mcp-agent stop agent-001

# View agent logs
mcp-agent logs agent-001 --follow

# Test agent
mcp-agent test --config agent.yaml --task testdata/task.json
```

## Next Steps

1. Review [Agent Integration Examples](./agent-integration-examples.md) for complete examples
2. See [Agent WebSocket Protocol](./agent-websocket-protocol.md) for protocol details
3. Check [Agent Integration Troubleshooting](./agent-integration-troubleshooting.md) for debugging
4. Explore [Building Custom AI Agents](./building-custom-ai-agents.md) for advanced patterns

## Resources

- [SDK API Documentation](https://pkg.go.dev/github.com/S-Corkum/devops-mcp/sdk/agent)
- [Example Projects](https://github.com/S-Corkum/mcp-agent-examples)
- [SDK Changelog](https://github.com/S-Corkum/devops-mcp/blob/main/sdk/CHANGELOG.md)
- [Community Forum](https://forum.mcp.dev/c/sdk)