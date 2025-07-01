# Agent Registration Guide

> **Purpose**: Step-by-step guide for registering AI agents with the DevOps MCP platform
> **Audience**: Developers integrating AI agents with different neural network models
> **Scope**: Registration process, configuration, model selection, capability declaration

## Overview

This guide explains how to register AI agents with the DevOps MCP platform. Agents can be powered by various neural networks (GPT-4, Claude, Bedrock models, custom models) and specialized for different tasks (code analysis, documentation, DevOps automation, performance optimization).

## Quick Start

### Basic Agent Registration

```go
// Register a simple agent
agent := &agents.Agent{
    ID:      "my-code-agent",
    Name:    "Code Analysis Agent",
    Type:    agents.AgentTypeCodeAnalysis,
    Model:   agents.ModelGPT4,
    Status:  agents.StatusActive,
}

// Connect and register
client := mcp.NewClient("ws://localhost:8080/ws")
err := client.RegisterAgent(context.Background(), agent)
```

## Agent Architecture

### Agent Components

```
┌─────────────────────────────────────────────────────────┐
│                    Agent Instance                        │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │   Identity  │  │ Capabilities │  │    Model     │  │
│  │  - ID       │  │  - Tasks     │  │  - Provider  │  │
│  │  - Name     │  │  - Skills    │  │  - Version   │  │
│  │  - Type     │  │  - Languages │  │  - Config    │  │
│  └─────────────┘  └──────────────┘  └──────────────┘  │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │ Connection  │  │    State     │  │ Constraints  │  │
│  │  - WebSocket│  │  - Workload  │  │  - Cost      │  │
│  │  - Protocol │  │  - Health    │  │  - Latency   │  │
│  │  - Auth     │  │  - Metrics   │  │  - Quality   │  │
│  └─────────────┘  └──────────────┘  └──────────────┘  │
└─────────────────────────────────────────────────────────┘
```

## Registration Process

### 1. Define Agent Configuration

```go
// Complete agent configuration
config := &agents.AgentConfig{
    AgentID:           "code-reviewer-001",
    Version:           1,
    EmbeddingStrategy: agents.StrategyBalanced,
    ModelPreferences: []agents.ModelPreference{
        {
            TaskType:       agents.TaskTypeCodeAnalysis,
            PrimaryModels:  []string{"gpt-4", "claude-2"},
            FallbackModels: []string{"gpt-3.5-turbo"},
            Weight:         0.8,
        },
        {
            TaskType:       agents.TaskTypeGeneralQA,
            PrimaryModels:  []string{"claude-2"},
            FallbackModels: []string{"gpt-3.5-turbo"},
            Weight:         0.2,
        },
    },
    Constraints: agents.AgentConstraints{
        MaxCostPerMonthUSD: 100.0,
        MaxLatencyP99Ms:    5000,
        MinAvailabilitySLA: 0.99,
        RateLimits: agents.RateLimitConfig{
            RequestsPerMinute:  60,
            TokensPerHour:      100000,
            ConcurrentRequests: 10,
        },
        QualityThresholds: agents.QualityConfig{
            MinCosineSimilarity:   0.8,
            MinEmbeddingMagnitude: 0.5,
            AcceptableErrorRate:   0.01,
        },
    },
    FallbackBehavior: agents.FallbackConfig{
        MaxRetries:      3,
        InitialDelayMs:  1000,
        MaxDelayMs:      10000,
        ExponentialBase: 2.0,
        QueueOnFailure:  true,
        CircuitBreaker: agents.CircuitConfig{
            Enabled:          true,
            FailureThreshold: 5,
            SuccessThreshold: 2,
            TimeoutSeconds:   30,
        },
    },
    Metadata: map[string]interface{}{
        "team":        "platform",
        "environment": "production",
        "region":      "us-east-1",
    },
    IsActive:  true,
    CreatedBy: "platform-team",
}
```

### 2. Declare Agent Capabilities

```go
// Define what the agent can do
capabilities := []agents.Capability{
    {
        Name:       "syntax_analysis",
        Confidence: 0.95,
        TaskTypes:  []agents.TaskType{agents.TaskTypeCodeReview, agents.TaskTypeBugDetection},
        Languages:  []string{"go", "python", "javascript", "typescript"},
        Specialties: []string{"error-handling", "concurrency", "performance"},
    },
    {
        Name:       "security_scanning",
        Confidence: 0.88,
        TaskTypes:  []agents.TaskType{agents.TaskTypeSecurityAudit},
        Specialties: []string{"OWASP", "dependency-check", "secret-detection"},
    },
    {
        Name:       "code_documentation",
        Confidence: 0.92,
        TaskTypes:  []agents.TaskType{agents.TaskTypeDocGeneration},
        Languages:  []string{"go", "python"},
        Specialties: []string{"API-docs", "inline-comments", "README"},
    },
}

// Attach to agent
agent := &agents.Agent{
    ID:           config.AgentID,
    Capabilities: capabilities,
    Config:       config,
}
```

### 3. Connect via WebSocket

```go
// WebSocket connection with authentication
type AgentClient struct {
    conn   *websocket.Conn
    agent  *agents.Agent
    config *agents.AgentConfig
}

func NewAgentClient(url, apiKey string) (*AgentClient, error) {
    // Setup headers
    headers := http.Header{
        "Authorization": []string{fmt.Sprintf("Bearer %s", apiKey)},
        "X-Agent-ID":    []string{agent.ID},
    }
    
    // Connect
    conn, _, err := websocket.DefaultDialer.Dial(url, headers)
    if err != nil {
        return nil, fmt.Errorf("websocket dial failed: %w", err)
    }
    
    return &AgentClient{
        conn: conn,
    }, nil
}
```

### 4. Send Registration Message

```go
// Registration message format
type RegistrationMessage struct {
    Type         string                 `json:"type"`
    AgentID      string                 `json:"agent_id"`
    Capabilities []agents.Capability    `json:"capabilities"`
    Config       *agents.AgentConfig    `json:"config"`
    Metadata     map[string]interface{} `json:"metadata"`
}

func (c *AgentClient) Register(ctx context.Context) error {
    // Create registration message
    msg := RegistrationMessage{
        Type:         "agent.register",
        AgentID:      c.agent.ID,
        Capabilities: c.agent.Capabilities,
        Config:       c.config,
        Metadata: map[string]interface{}{
            "version":    "1.0.0",
            "sdk":        "go",
            "timestamp":  time.Now().Unix(),
        },
    }
    
    // Send registration
    if err := c.conn.WriteJSON(msg); err != nil {
        return fmt.Errorf("registration send failed: %w", err)
    }
    
    // Wait for acknowledgment
    var response map[string]interface{}
    if err := c.conn.ReadJSON(&response); err != nil {
        return fmt.Errorf("registration response failed: %w", err)
    }
    
    if response["type"] != "agent.registered" {
        return fmt.Errorf("registration failed: %v", response["error"])
    }
    
    return nil
}
```

## Model Integration

### 1. OpenAI Models (GPT-4, GPT-3.5)

```go
// OpenAI model configuration
type OpenAIAgent struct {
    BaseAgent
    client *openai.Client
    model  string
}

func NewOpenAIAgent(apiKey string, model string) *OpenAIAgent {
    return &OpenAIAgent{
        client: openai.NewClient(apiKey),
        model:  model, // "gpt-4", "gpt-3.5-turbo"
    }
}

func (a *OpenAIAgent) Process(ctx context.Context, task Task) (*Result, error) {
    // Build messages
    messages := []openai.ChatCompletionMessage{
        {
            Role:    openai.ChatMessageRoleSystem,
            Content: a.buildSystemPrompt(),
        },
        {
            Role:    openai.ChatMessageRoleUser,
            Content: task.Content,
        },
    }
    
    // Create completion
    resp, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:       a.model,
        Messages:    messages,
        Temperature: 0.7,
        MaxTokens:   2000,
    })
    
    if err != nil {
        return nil, err
    }
    
    return &Result{
        Content: resp.Choices[0].Message.Content,
        Model:   a.model,
        Usage:   resp.Usage,
    }, nil
}
```

### 2. Anthropic Models (Claude)

```go
// Claude model configuration
type ClaudeAgent struct {
    BaseAgent
    client *anthropic.Client
    model  string
}

func NewClaudeAgent(apiKey string, model string) *ClaudeAgent {
    return &ClaudeAgent{
        client: anthropic.NewClient(apiKey),
        model:  model, // "claude-2", "claude-instant-1"
    }
}

func (a *ClaudeAgent) Process(ctx context.Context, task Task) (*Result, error) {
    // Create completion
    resp, err := a.client.CreateMessage(ctx, anthropic.MessageRequest{
        Model:     a.model,
        Messages:  a.buildMessages(task),
        MaxTokens: 2000,
        System:    a.buildSystemPrompt(),
    })
    
    if err != nil {
        return nil, err
    }
    
    return &Result{
        Content: resp.Content[0].Text,
        Model:   a.model,
        Usage:   resp.Usage,
    }, nil
}
```

### 3. AWS Bedrock Models

```go
// Bedrock model configuration
type BedrockAgent struct {
    BaseAgent
    client   *bedrockruntime.Client
    modelID  string
    provider string
}

func NewBedrockAgent(cfg aws.Config, modelID string) *BedrockAgent {
    return &BedrockAgent{
        client:   bedrockruntime.NewFromConfig(cfg),
        modelID:  modelID,
        provider: extractProvider(modelID),
    }
}

func (a *BedrockAgent) Process(ctx context.Context, task Task) (*Result, error) {
    // Build request based on provider
    var payload []byte
    var err error
    
    switch a.provider {
    case "amazon":
        payload, err = a.buildTitanRequest(task)
    case "anthropic":
        payload, err = a.buildClaudeRequest(task)
    case "cohere":
        payload, err = a.buildCohereRequest(task)
    default:
        return nil, fmt.Errorf("unsupported provider: %s", a.provider)
    }
    
    if err != nil {
        return nil, err
    }
    
    // Invoke model
    resp, err := a.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
        ModelId:     aws.String(a.modelID),
        Body:        payload,
        ContentType: aws.String("application/json"),
    })
    
    if err != nil {
        return nil, err
    }
    
    // Parse response based on provider
    return a.parseResponse(resp.Body, a.provider)
}

// Example model IDs:
// - "amazon.titan-text-express-v1"
// - "anthropic.claude-v2"
// - "cohere.command-text-v14"
```

### 4. Custom Models

```go
// Custom model integration
type CustomModelAgent struct {
    BaseAgent
    endpoint string
    apiKey   string
    client   *http.Client
}

func NewCustomModelAgent(endpoint, apiKey string) *CustomModelAgent {
    return &CustomModelAgent{
        endpoint: endpoint,
        apiKey:   apiKey,
        client: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

func (a *CustomModelAgent) Process(ctx context.Context, task Task) (*Result, error) {
    // Build request
    reqBody := map[string]interface{}{
        "prompt":      task.Content,
        "max_tokens":  2000,
        "temperature": 0.7,
        "metadata":    task.Metadata,
    }
    
    body, err := json.Marshal(reqBody)
    if err != nil {
        return nil, err
    }
    
    // Create HTTP request
    req, err := http.NewRequestWithContext(ctx, "POST", a.endpoint, bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.apiKey))
    req.Header.Set("Content-Type", "application/json")
    
    // Send request
    resp, err := a.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    // Parse response
    var result CustomModelResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    
    return &Result{
        Content: result.Text,
        Model:   "custom",
        Usage:   result.Usage,
    }, nil
}
```

## Advanced Registration

### 1. Multi-Model Agent

```go
// Agent that can use multiple models
type MultiModelAgent struct {
    models   map[string]ModelProvider
    selector ModelSelector
    config   *agents.AgentConfig
}

func (m *MultiModelAgent) Register(ctx context.Context) error {
    // Register with all supported models
    capabilities := []agents.Capability{}
    
    for modelName, provider := range m.models {
        caps := provider.GetCapabilities()
        for i := range caps {
            caps[i].Metadata = map[string]interface{}{
                "model": modelName,
            }
        }
        capabilities = append(capabilities, caps...)
    }
    
    // Register agent with combined capabilities
    return m.registerWithCapabilities(ctx, capabilities)
}

func (m *MultiModelAgent) Process(ctx context.Context, task Task) (*Result, error) {
    // Select best model for task
    model := m.selector.SelectModel(task, m.config.ModelPreferences)
    provider := m.models[model]
    
    // Process with selected model
    return provider.Process(ctx, task)
}
```

### 2. Specialized Agent Registration

```go
// Code analysis specialist
func RegisterCodeAnalysisAgent(mcp *MCPClient) error {
    agent := &agents.Agent{
        ID:   "code-specialist-001",
        Name: "Code Analysis Specialist",
        Type: agents.AgentTypeCodeAnalysis,
        Capabilities: []agents.Capability{
            {
                Name:       "ast_analysis",
                Confidence: 0.98,
                TaskTypes:  []agents.TaskType{agents.TaskTypeCodeReview},
                Languages:  []string{"go", "java", "python"},
                Specialties: []string{"complexity", "patterns", "smells"},
            },
            {
                Name:       "performance_profiling",
                Confidence: 0.90,
                TaskTypes:  []agents.TaskType{agents.TaskTypePerformance},
                Specialties: []string{"cpu", "memory", "goroutines"},
            },
        },
        Config: &agents.AgentConfig{
            ModelPreferences: []agents.ModelPreference{
                {
                    TaskType:      agents.TaskTypeCodeAnalysis,
                    PrimaryModels: []string{"gpt-4", "claude-2"},
                },
            },
        },
    }
    
    return mcp.RegisterAgent(context.Background(), agent)
}

// Documentation specialist
func RegisterDocumentationAgent(mcp *MCPClient) error {
    agent := &agents.Agent{
        ID:   "docs-specialist-001",
        Name: "Documentation Specialist",
        Type: agents.AgentTypeDocumentation,
        Capabilities: []agents.Capability{
            {
                Name:       "api_documentation",
                Confidence: 0.95,
                TaskTypes:  []agents.TaskType{agents.TaskTypeDocGeneration},
                Specialties: []string{"openapi", "swagger", "rest"},
            },
            {
                Name:       "tutorial_creation",
                Confidence: 0.92,
                TaskTypes:  []agents.TaskType{agents.TaskTypeDocGeneration},
                Specialties: []string{"guides", "tutorials", "examples"},
            },
        },
    }
    
    return mcp.RegisterAgent(context.Background(), agent)
}
```

### 3. Dynamic Capability Updates

```go
// Update agent capabilities at runtime
type DynamicAgent struct {
    *AgentClient
    capabilities []agents.Capability
}

func (d *DynamicAgent) UpdateCapabilities(ctx context.Context, newCaps []agents.Capability) error {
    // Send capability update message
    msg := map[string]interface{}{
        "type":         "agent.capabilities.update",
        "agent_id":     d.agent.ID,
        "capabilities": newCaps,
    }
    
    if err := d.conn.WriteJSON(msg); err != nil {
        return err
    }
    
    // Update local state
    d.capabilities = newCaps
    
    return nil
}

// Example: Add new language support
func (d *DynamicAgent) AddLanguageSupport(ctx context.Context, language string) error {
    // Find code analysis capability
    for i, cap := range d.capabilities {
        if cap.Name == "syntax_analysis" {
            // Add new language
            d.capabilities[i].Languages = append(cap.Languages, language)
            break
        }
    }
    
    return d.UpdateCapabilities(ctx, d.capabilities)
}
```

## Health and Status Reporting

### 1. Health Check Implementation

```go
// Agent health reporting
func (c *AgentClient) StartHealthReporting(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            health := c.checkHealth()
            msg := map[string]interface{}{
                "type":      "agent.health",
                "agent_id":  c.agent.ID,
                "status":    health.Status,
                "metrics":   health.Metrics,
                "timestamp": time.Now().Unix(),
            }
            
            if err := c.conn.WriteJSON(msg); err != nil {
                // Handle error
                c.handleHealthError(err)
            }
            
        case <-ctx.Done():
            return
        }
    }
}

type HealthStatus struct {
    Status  string
    Metrics map[string]interface{}
}

func (c *AgentClient) checkHealth() HealthStatus {
    return HealthStatus{
        Status: "healthy",
        Metrics: map[string]interface{}{
            "cpu_usage":        c.getCPUUsage(),
            "memory_usage":     c.getMemoryUsage(),
            "active_tasks":     c.getActiveTaskCount(),
            "completed_tasks":  c.getCompletedTaskCount(),
            "error_rate":       c.getErrorRate(),
            "avg_latency_ms":   c.getAverageLatency(),
        },
    }
}
```

### 2. Workload Reporting

```go
// Report current workload
func (c *AgentClient) ReportWorkload(ctx context.Context) error {
    workload := &WorkloadReport{
        AgentID:      c.agent.ID,
        ActiveTasks:  c.getActiveTasks(),
        QueuedTasks:  c.getQueuedTasks(),
        Capacity:     c.getCapacity(),
        Utilization:  c.getUtilization(),
    }
    
    msg := map[string]interface{}{
        "type":     "agent.workload",
        "agent_id": c.agent.ID,
        "workload": workload,
    }
    
    return c.conn.WriteJSON(msg)
}
```

## Error Handling

### 1. Registration Failures

```go
// Handle registration errors
func (c *AgentClient) RegisterWithRetry(ctx context.Context, maxRetries int) error {
    var lastErr error
    
    for i := 0; i < maxRetries; i++ {
        err := c.Register(ctx)
        if err == nil {
            return nil
        }
        
        lastErr = err
        
        // Check error type
        switch {
        case isAuthError(err):
            return fmt.Errorf("authentication failed: %w", err)
            
        case isCapabilityError(err):
            // Fix capabilities and retry
            c.validateAndFixCapabilities()
            
        case isNetworkError(err):
            // Exponential backoff
            time.Sleep(time.Duration(math.Pow(2, float64(i))) * time.Second)
            
        default:
            return err
        }
    }
    
    return fmt.Errorf("registration failed after %d retries: %w", maxRetries, lastErr)
}
```

### 2. Connection Recovery

```go
// Automatic reconnection
func (c *AgentClient) MaintainConnection(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
            
        default:
            // Check connection health
            if err := c.ping(); err != nil {
                // Reconnect
                if err := c.reconnect(); err != nil {
                    time.Sleep(5 * time.Second)
                    continue
                }
                
                // Re-register
                if err := c.Register(ctx); err != nil {
                    time.Sleep(5 * time.Second)
                    continue
                }
            }
            
            time.Sleep(30 * time.Second)
        }
    }
}
```

## Best Practices

### 1. Registration Checklist

- [ ] Define clear agent identity and purpose
- [ ] Declare all capabilities with accurate confidence scores
- [ ] Configure appropriate model preferences
- [ ] Set realistic constraints (cost, latency, quality)
- [ ] Implement proper error handling and retry logic
- [ ] Setup health and workload reporting
- [ ] Test failover behavior
- [ ] Monitor registration success rate

### 2. Security Considerations

```go
// Secure registration
func SecureRegistration(agent *agents.Agent) error {
    // 1. Validate API credentials
    if err := validateAPIKey(agent.APIKey); err != nil {
        return err
    }
    
    // 2. Use TLS for WebSocket
    url := "wss://mcp.example.com/ws" // Note: wss:// not ws://
    
    // 3. Implement rate limiting
    limiter := rate.NewLimiter(rate.Every(time.Second), 10)
    if !limiter.Allow() {
        return ErrRateLimited
    }
    
    // 4. Sign registration request
    signature := signRequest(agent)
    
    // 5. Include security headers
    headers := map[string]string{
        "X-Agent-Signature": signature,
        "X-Request-ID":      uuid.New().String(),
    }
    
    return registerWithSecurity(url, agent, headers)
}
```

### 3. Performance Optimization

```go
// Optimize agent performance
type OptimizedAgent struct {
    *BaseAgent
    cache       *AgentCache
    pool        *WorkerPool
    batcher     *RequestBatcher
}

func (o *OptimizedAgent) Configure() {
    // 1. Enable response caching
    o.cache = NewAgentCache(1000, 1*time.Hour)
    
    // 2. Use worker pool for parallel processing
    o.pool = NewWorkerPool(10)
    
    // 3. Batch similar requests
    o.batcher = NewRequestBatcher(100, 100*time.Millisecond)
    
    // 4. Configure connection pooling
    o.configureConnectionPool(20, 100)
}
```

## Monitoring

### 1. Registration Metrics

```go
var (
    agentRegistrations = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "agent_registrations_total",
            Help: "Total number of agent registrations",
        },
        []string{"agent_type", "model", "status"},
    )
    
    registrationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "agent_registration_duration_seconds",
            Help: "Time taken to register an agent",
        },
        []string{"agent_type"},
    )
)
```

### 2. Capability Tracking

```go
// Track capability usage
func TrackCapabilityUsage(agent *agents.Agent, capability string) {
    capabilityUsage.WithLabelValues(
        agent.ID,
        agent.Type,
        capability,
    ).Inc()
}
```

## Troubleshooting

### Common Issues

1. **Registration Timeout**
   ```go
   // Increase timeout
   client.SetTimeout(30 * time.Second)
   ```

2. **Invalid Capabilities**
   ```go
   // Validate before registration
   if err := agent.ValidateCapabilities(); err != nil {
       log.Printf("Invalid capabilities: %v", err)
   }
   ```

3. **Model Not Available**
   ```go
   // Check model availability
   models := client.GetAvailableModels()
   if !contains(models, desiredModel) {
       // Use fallback
   }
   ```

4. **Authentication Failures**
   ```go
   // Refresh token
   token, err := refreshAuthToken()
   client.SetAuthToken(token)
   ```

## Next Steps

1. Review [Task Routing Algorithms](./task-routing-algorithms.md) for task distribution
2. Explore [Multi-Agent Collaboration](./multi-agent-collaboration.md) for coordination
3. See [Agent Specialization Patterns](./agent-specialization-patterns.md) for design patterns
4. Check [Agent WebSocket Protocol](./agent-websocket-protocol.md) for protocol details

## Resources

- [AI Agent Orchestration Guide](./ai-agent-orchestration.md)
- [Building Custom AI Agents](./building-custom-ai-agents.md)
- [Agent Integration Examples](./agent-integration-examples.md)
- [Agent SDK Guide](./agent-sdk-guide.md)