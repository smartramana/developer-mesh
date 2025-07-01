# Agents Package

> **Purpose**: AI agent management and orchestration for the DevOps MCP platform
> **Status**: Production Ready
> **Dependencies**: WebSocket connections, capability registry, workload tracking, collaboration protocols

## Overview

The agents package provides comprehensive AI agent lifecycle management, from registration and capability discovery to task assignment and multi-agent collaboration. It supports heterogeneous AI models and enables dynamic orchestration of distributed AI workloads.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Agent Management System                   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Agent Registry ──► Agent Manager ──► Task Router          │
│       │                  │               │                  │
│       │                  ├── Health      ├── Round Robin   │
│       │                  ├── Metrics     ├── Least Loaded  │
│       │                  └── State       └── Capability    │
│       │                                                     │
│       └──► Capability Index ──► Collaboration Engine       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Agent Interface

```go
// Agent represents an AI agent instance
type Agent interface {
    // ID returns unique agent identifier
    ID() string
    
    // Model returns the AI model (e.g., "claude-3-opus", "gpt-4")
    Model() string
    
    // Capabilities returns agent capabilities
    Capabilities() []Capability
    
    // Status returns current agent status
    Status() AgentStatus
    
    // Metrics returns performance metrics
    Metrics() *AgentMetrics
    
    // Execute runs a task
    Execute(ctx context.Context, task *Task) (*TaskResult, error)
    
    // HealthCheck verifies agent health
    HealthCheck(ctx context.Context) error
}

// AgentStatus represents agent state
type AgentStatus string

const (
    StatusRegistering AgentStatus = "registering"
    StatusActive      AgentStatus = "active"
    StatusBusy        AgentStatus = "busy"
    StatusDraining    AgentStatus = "draining"
    StatusOffline     AgentStatus = "offline"
    StatusError       AgentStatus = "error"
)

// Capability defines what an agent can do
type Capability struct {
    Name        string                 `json:"name"`
    Type        CapabilityType         `json:"type"`
    Version     string                 `json:"version"`
    Parameters  map[string]interface{} `json:"parameters"`
    Constraints *Constraints           `json:"constraints,omitempty"`
}

// AgentMetrics tracks agent performance
type AgentMetrics struct {
    TasksCompleted   int64         `json:"tasks_completed"`
    TasksFailed      int64         `json:"tasks_failed"`
    AverageLatency   time.Duration `json:"average_latency"`
    SuccessRate      float64       `json:"success_rate"`
    CurrentLoad      float64       `json:"current_load"`
    TokensProcessed  int64         `json:"tokens_processed"`
    CostIncurred     float64       `json:"cost_incurred"`
    LastHealthCheck  time.Time     `json:"last_health_check"`
}
```

### 2. Agent Manager

```go
// AgentManager orchestrates all agents
type AgentManager struct {
    registry    *AgentRegistry
    router      TaskRouter
    health      *HealthMonitor
    metrics     *MetricsCollector
    config      *Config
    mu          sync.RWMutex
}

// NewAgentManager creates an agent manager
func NewAgentManager(config *Config) *AgentManager {
    return &AgentManager{
        registry: NewAgentRegistry(),
        router:   NewCapabilityBasedRouter(),
        health:   NewHealthMonitor(config.HealthCheckInterval),
        metrics:  NewMetricsCollector(),
        config:   config,
    }
}

// RegisterAgent adds a new agent
func (m *AgentManager) RegisterAgent(ctx context.Context, agent Agent) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // Validate agent
    if err := m.validateAgent(agent); err != nil {
        return fmt.Errorf("agent validation failed: %w", err)
    }
    
    // Health check
    if err := agent.HealthCheck(ctx); err != nil {
        return fmt.Errorf("agent health check failed: %w", err)
    }
    
    // Register in registry
    if err := m.registry.Register(agent); err != nil {
        return fmt.Errorf("registry error: %w", err)
    }
    
    // Start monitoring
    m.health.Monitor(agent)
    
    // Update capability index
    m.updateCapabilityIndex(agent)
    
    // Emit event
    m.emitAgentRegistered(agent)
    
    logger.Info("agent registered",
        "agent_id", agent.ID(),
        "model", agent.Model(),
        "capabilities", len(agent.Capabilities()),
    )
    
    return nil
}

// AssignTask routes a task to an appropriate agent
func (m *AgentManager) AssignTask(ctx context.Context, task *Task) (*Assignment, error) {
    // Find capable agents
    candidates := m.findCapableAgents(task)
    if len(candidates) == 0 {
        return nil, ErrNoCapableAgent
    }
    
    // Apply routing strategy
    agent := m.router.SelectAgent(task, candidates)
    if agent == nil {
        return nil, ErrRoutingFailed
    }
    
    // Create assignment
    assignment := &Assignment{
        ID:        uuid.New().String(),
        TaskID:    task.ID,
        AgentID:   agent.ID(),
        CreatedAt: time.Now(),
        Status:    AssignmentPending,
    }
    
    // Update agent workload
    if err := m.updateAgentWorkload(agent, task); err != nil {
        return nil, err
    }
    
    return assignment, nil
}
```

### 3. Agent Registry

```go
// AgentRegistry maintains agent inventory
type AgentRegistry struct {
    agents       map[string]Agent
    byModel      map[string][]string // model -> agent IDs
    byCapability map[string][]string // capability -> agent IDs
    mu           sync.RWMutex
}

// Register adds an agent to registry
func (r *AgentRegistry) Register(agent Agent) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    id := agent.ID()
    
    // Check duplicate
    if _, exists := r.agents[id]; exists {
        return ErrAgentAlreadyRegistered
    }
    
    // Add to maps
    r.agents[id] = agent
    
    // Index by model
    model := agent.Model()
    r.byModel[model] = append(r.byModel[model], id)
    
    // Index by capabilities
    for _, cap := range agent.Capabilities() {
        r.byCapability[cap.Name] = append(r.byCapability[cap.Name], id)
    }
    
    return nil
}

// FindByCapability returns agents with specific capability
func (r *AgentRegistry) FindByCapability(capability string) []Agent {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    agentIDs, exists := r.byCapability[capability]
    if !exists {
        return nil
    }
    
    agents := make([]Agent, 0, len(agentIDs))
    for _, id := range agentIDs {
        if agent, exists := r.agents[id]; exists {
            agents = append(agents, agent)
        }
    }
    
    return agents
}

// GetMetrics returns registry metrics
func (r *AgentRegistry) GetMetrics() *RegistryMetrics {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    metrics := &RegistryMetrics{
        TotalAgents:      len(r.agents),
        AgentsByModel:    make(map[string]int),
        AgentsByStatus:   make(map[AgentStatus]int),
        UniqueCapabilities: len(r.byCapability),
    }
    
    // Count by model
    for model, agents := range r.byModel {
        metrics.AgentsByModel[model] = len(agents)
    }
    
    // Count by status
    for _, agent := range r.agents {
        status := agent.Status()
        metrics.AgentsByStatus[status]++
    }
    
    return metrics
}
```

### 4. WebSocket Agent Implementation

```go
// WebSocketAgent represents a remote AI agent
type WebSocketAgent struct {
    id           string
    model        string
    capabilities []Capability
    conn         *websocket.Conn
    status       AgentStatus
    metrics      *AgentMetrics
    mu           sync.RWMutex
}

// NewWebSocketAgent creates a WebSocket-connected agent
func NewWebSocketAgent(conn *websocket.Conn, info *AgentInfo) *WebSocketAgent {
    return &WebSocketAgent{
        id:           info.ID,
        model:        info.Model,
        capabilities: info.Capabilities,
        conn:         conn,
        status:       StatusActive,
        metrics:      NewAgentMetrics(),
    }
}

// Execute sends task to remote agent
func (a *WebSocketAgent) Execute(ctx context.Context, task *Task) (*TaskResult, error) {
    a.mu.Lock()
    a.status = StatusBusy
    a.mu.Unlock()
    
    defer func() {
        a.mu.Lock()
        a.status = StatusActive
        a.mu.Unlock()
    }()
    
    // Create execution request
    request := &ExecutionRequest{
        ID:        uuid.New().String(),
        TaskID:    task.ID,
        TaskType:  task.Type,
        Payload:   task.Payload,
        Timeout:   task.Timeout,
        Timestamp: time.Now(),
    }
    
    // Send via WebSocket
    if err := a.conn.WriteJSON(request); err != nil {
        return nil, fmt.Errorf("send request failed: %w", err)
    }
    
    // Wait for response
    responseChan := make(chan *TaskResult, 1)
    errorChan := make(chan error, 1)
    
    go func() {
        var result TaskResult
        if err := a.conn.ReadJSON(&result); err != nil {
            errorChan <- err
            return
        }
        responseChan <- &result
    }()
    
    // Handle timeout
    select {
    case result := <-responseChan:
        a.updateMetrics(result)
        return result, nil
        
    case err := <-errorChan:
        a.metrics.TasksFailed++
        return nil, err
        
    case <-ctx.Done():
        a.metrics.TasksFailed++
        return nil, ctx.Err()
        
    case <-time.After(task.Timeout):
        a.metrics.TasksFailed++
        return nil, ErrTaskTimeout
    }
}

// HealthCheck pings the agent
func (a *WebSocketAgent) HealthCheck(ctx context.Context) error {
    // Send ping
    ping := &Message{
        Type:      "ping",
        Timestamp: time.Now(),
    }
    
    if err := a.conn.WriteJSON(ping); err != nil {
        a.mu.Lock()
        a.status = StatusError
        a.mu.Unlock()
        return err
    }
    
    // Wait for pong
    pongChan := make(chan bool, 1)
    go func() {
        var msg Message
        if err := a.conn.ReadJSON(&msg); err == nil && msg.Type == "pong" {
            pongChan <- true
        }
    }()
    
    select {
    case <-pongChan:
        a.mu.Lock()
        a.status = StatusActive
        a.metrics.LastHealthCheck = time.Now()
        a.mu.Unlock()
        return nil
        
    case <-time.After(5 * time.Second):
        a.mu.Lock()
        a.status = StatusError
        a.mu.Unlock()
        return ErrHealthCheckTimeout
    }
}
```

### 5. Task Routing

```go
// TaskRouter selects agents for tasks
type TaskRouter interface {
    SelectAgent(task *Task, candidates []Agent) Agent
}

// CapabilityBasedRouter routes based on capabilities and performance
type CapabilityBasedRouter struct {
    weights RouterWeights
}

type RouterWeights struct {
    CapabilityMatch float64
    Performance     float64
    CurrentLoad     float64
    Cost            float64
}

func (r *CapabilityBasedRouter) SelectAgent(task *Task, candidates []Agent) Agent {
    if len(candidates) == 0 {
        return nil
    }
    
    scores := make([]ScoredAgent, 0, len(candidates))
    
    for _, agent := range candidates {
        score := r.calculateScore(task, agent)
        scores = append(scores, ScoredAgent{
            Agent: agent,
            Score: score,
        })
    }
    
    // Sort by score descending
    sort.Slice(scores, func(i, j int) bool {
        return scores[i].Score > scores[j].Score
    })
    
    return scores[0].Agent
}

func (r *CapabilityBasedRouter) calculateScore(task *Task, agent Agent) float64 {
    score := 0.0
    
    // Capability match score
    capScore := r.calculateCapabilityScore(task, agent)
    score += capScore * r.weights.CapabilityMatch
    
    // Performance score
    metrics := agent.Metrics()
    perfScore := metrics.SuccessRate
    score += perfScore * r.weights.Performance
    
    // Load score (inverse - lower load is better)
    loadScore := 1.0 - metrics.CurrentLoad
    score += loadScore * r.weights.CurrentLoad
    
    // Cost score (inverse - lower cost is better)
    costScore := 1.0 / (1.0 + r.estimateCost(task, agent))
    score += costScore * r.weights.Cost
    
    return score
}

// RoundRobinRouter distributes tasks evenly
type RoundRobinRouter struct {
    current uint64
}

func (r *RoundRobinRouter) SelectAgent(task *Task, candidates []Agent) Agent {
    if len(candidates) == 0 {
        return nil
    }
    
    index := atomic.AddUint64(&r.current, 1) % uint64(len(candidates))
    return candidates[index]
}

// LeastLoadedRouter selects agent with lowest load
type LeastLoadedRouter struct{}

func (r *LeastLoadedRouter) SelectAgent(task *Task, candidates []Agent) Agent {
    if len(candidates) == 0 {
        return nil
    }
    
    var selected Agent
    minLoad := math.MaxFloat64
    
    for _, agent := range candidates {
        load := agent.Metrics().CurrentLoad
        if load < minLoad {
            minLoad = load
            selected = agent
        }
    }
    
    return selected
}
```

### 6. Multi-Agent Collaboration

```go
// CollaborationSession manages multi-agent tasks
type CollaborationSession struct {
    ID           string
    Strategy     CollaborationStrategy
    Participants []Agent
    Task         *CollaborativeTask
    State        *CollaborationState
    Results      chan *PartialResult
    mu           sync.RWMutex
}

// CollaborationStrategy defines how agents work together
type CollaborationStrategy interface {
    Execute(ctx context.Context, session *CollaborationSession) (*CollaborationResult, error)
}

// MapReduceStrategy for parallel processing
type MapReduceStrategy struct {
    mapper  func(*Task) []*SubTask
    reducer func([]*TaskResult) *TaskResult
}

func (s *MapReduceStrategy) Execute(ctx context.Context, session *CollaborationSession) (*CollaborationResult, error) {
    // Map phase - split task
    subTasks := s.mapper(session.Task.Task)
    
    // Assign subtasks to agents
    assignments := make([]*Assignment, 0, len(subTasks))
    for i, subTask := range subTasks {
        agent := session.Participants[i%len(session.Participants)]
        assignment := &Assignment{
            Agent:   agent,
            SubTask: subTask,
        }
        assignments = append(assignments, assignment)
    }
    
    // Execute in parallel
    results := make([]*TaskResult, len(assignments))
    var wg sync.WaitGroup
    
    for i, assignment := range assignments {
        wg.Add(1)
        go func(idx int, assign *Assignment) {
            defer wg.Done()
            
            result, err := assign.Agent.Execute(ctx, assign.SubTask)
            if err != nil {
                logger.Error("subtask failed", 
                    "agent", assign.Agent.ID(),
                    "error", err,
                )
                return
            }
            results[idx] = result
        }(i, assignment)
    }
    
    wg.Wait()
    
    // Reduce phase - combine results
    finalResult := s.reducer(results)
    
    return &CollaborationResult{
        SessionID: session.ID,
        Result:    finalResult,
        Metrics:   s.calculateMetrics(assignments, results),
    }, nil
}

// ConsensusStrategy for agreement-based tasks
type ConsensusStrategy struct {
    threshold float64 // Agreement threshold (e.g., 0.8 = 80%)
    maxRounds int
}

func (s *ConsensusStrategy) Execute(ctx context.Context, session *CollaborationSession) (*CollaborationResult, error) {
    task := session.Task
    
    for round := 0; round < s.maxRounds; round++ {
        // Collect proposals from all agents
        proposals := make([]*Proposal, 0, len(session.Participants))
        
        for _, agent := range session.Participants {
            result, err := agent.Execute(ctx, task.Task)
            if err != nil {
                continue
            }
            
            proposal := &Proposal{
                AgentID: agent.ID(),
                Result:  result,
                Round:   round,
            }
            proposals = append(proposals, proposal)
        }
        
        // Check for consensus
        if consensus := s.findConsensus(proposals); consensus != nil {
            return &CollaborationResult{
                SessionID: session.ID,
                Result:    consensus,
                Rounds:    round + 1,
            }, nil
        }
        
        // Share proposals for next round
        task = s.createRefinedTask(task, proposals)
    }
    
    return nil, ErrNoConsensusReached
}

// ChainStrategy for sequential processing
type ChainStrategy struct {
    steps []ChainStep
}

type ChainStep struct {
    Name         string
    AgentMatcher func([]Agent) Agent
    Transform    func(*TaskResult) *Task
}

func (s *ChainStrategy) Execute(ctx context.Context, session *CollaborationSession) (*CollaborationResult, error) {
    var currentResult *TaskResult
    currentTask := session.Task.Task
    
    for _, step := range s.steps {
        // Select agent for this step
        agent := step.AgentMatcher(session.Participants)
        if agent == nil {
            return nil, fmt.Errorf("no agent for step %s", step.Name)
        }
        
        // Execute step
        result, err := agent.Execute(ctx, currentTask)
        if err != nil {
            return nil, fmt.Errorf("step %s failed: %w", step.Name, err)
        }
        
        currentResult = result
        
        // Transform for next step
        if step.Transform != nil {
            currentTask = step.Transform(result)
        }
    }
    
    return &CollaborationResult{
        SessionID: session.ID,
        Result:    currentResult,
        Steps:     len(s.steps),
    }, nil
}
```

### 7. Agent Health Monitoring

```go
// HealthMonitor tracks agent health
type HealthMonitor struct {
    agents    map[string]*MonitoredAgent
    interval  time.Duration
    mu        sync.RWMutex
    ctx       context.Context
    cancel    context.CancelFunc
}

type MonitoredAgent struct {
    Agent           Agent
    LastCheck       time.Time
    ConsecutiveFails int
    HealthHistory   []HealthRecord
}

func (m *HealthMonitor) Monitor(agent Agent) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    monitored := &MonitoredAgent{
        Agent:         agent,
        LastCheck:     time.Now(),
        HealthHistory: make([]HealthRecord, 0, 100),
    }
    
    m.agents[agent.ID()] = monitored
    
    // Start health check routine
    go m.healthCheckLoop(agent.ID())
}

func (m *HealthMonitor) healthCheckLoop(agentID string) {
    ticker := time.NewTicker(m.interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            m.performHealthCheck(agentID)
            
        case <-m.ctx.Done():
            return
        }
    }
}

func (m *HealthMonitor) performHealthCheck(agentID string) {
    m.mu.RLock()
    monitored, exists := m.agents[agentID]
    m.mu.RUnlock()
    
    if !exists {
        return
    }
    
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    start := time.Now()
    err := monitored.Agent.HealthCheck(ctx)
    duration := time.Since(start)
    
    record := HealthRecord{
        Timestamp: time.Now(),
        Success:   err == nil,
        Duration:  duration,
        Error:     err,
    }
    
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // Update monitoring data
    monitored.LastCheck = time.Now()
    monitored.HealthHistory = append(monitored.HealthHistory, record)
    
    // Trim history
    if len(monitored.HealthHistory) > 100 {
        monitored.HealthHistory = monitored.HealthHistory[1:]
    }
    
    // Update failure count
    if err != nil {
        monitored.ConsecutiveFails++
        
        // Mark unhealthy after threshold
        if monitored.ConsecutiveFails >= 3 {
            m.markUnhealthy(monitored.Agent)
        }
    } else {
        monitored.ConsecutiveFails = 0
    }
}
```

## Agent SDK

### Creating Custom Agents

```go
// CustomAgent implements a specialized agent
type CustomAgent struct {
    BaseAgent
    processor TaskProcessor
}

// BaseAgent provides common functionality
type BaseAgent struct {
    id           string
    model        string
    capabilities []Capability
    status       AgentStatus
    metrics      *AgentMetrics
}

// Example: Code generation agent
func NewCodeGenerationAgent(model string) *CustomAgent {
    return &CustomAgent{
        BaseAgent: BaseAgent{
            id:    uuid.New().String(),
            model: model,
            capabilities: []Capability{
                {Name: "code-generation", Type: CapabilityTypeGeneration},
                {Name: "code-review", Type: CapabilityTypeAnalysis},
                {Name: "refactoring", Type: CapabilityTypeTransformation},
            },
            status:  StatusActive,
            metrics: NewAgentMetrics(),
        },
        processor: &CodeProcessor{model: model},
    }
}

func (a *CustomAgent) Execute(ctx context.Context, task *Task) (*TaskResult, error) {
    start := time.Now()
    
    // Process based on task type
    var result interface{}
    var err error
    
    switch task.Type {
    case "generate-code":
        result, err = a.processor.GenerateCode(ctx, task.Payload)
    case "review-code":
        result, err = a.processor.ReviewCode(ctx, task.Payload)
    case "refactor-code":
        result, err = a.processor.RefactorCode(ctx, task.Payload)
    default:
        err = ErrUnsupportedTaskType
    }
    
    // Update metrics
    a.updateMetrics(err == nil, time.Since(start))
    
    if err != nil {
        return nil, err
    }
    
    return &TaskResult{
        TaskID:    task.ID,
        AgentID:   a.id,
        Result:    result,
        Duration:  time.Since(start),
        Timestamp: time.Now(),
    }, nil
}
```

## Monitoring & Metrics

```go
var (
    agentsRegistered = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_agents_registered_total",
            Help: "Total number of agents registered",
        },
        []string{"model"},
    )
    
    agentsActive = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "mcp_agents_active",
            Help: "Number of active agents",
        },
        []string{"model", "status"},
    )
    
    tasksAssigned = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_tasks_assigned_total",
            Help: "Total number of tasks assigned",
        },
        []string{"agent_model", "task_type"},
    )
    
    taskExecutionDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "mcp_task_execution_duration_seconds",
            Help:    "Task execution duration",
            Buckets: prometheus.ExponentialBuckets(0.1, 2, 15),
        },
        []string{"agent_model", "task_type"},
    )
    
    agentUtilization = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "mcp_agent_utilization_ratio",
            Help: "Agent utilization ratio (0-1)",
        },
        []string{"agent_id", "model"},
    )
)
```

## Configuration

### Environment Variables

```bash
# Agent Manager
AGENT_HEALTH_CHECK_INTERVAL=30s
AGENT_MAX_CONCURRENT_TASKS=10
AGENT_TASK_TIMEOUT=5m

# Routing
AGENT_ROUTING_STRATEGY=capability_based
AGENT_ROUTING_WEIGHTS_CAPABILITY=0.4
AGENT_ROUTING_WEIGHTS_PERFORMANCE=0.3
AGENT_ROUTING_WEIGHTS_LOAD=0.2
AGENT_ROUTING_WEIGHTS_COST=0.1

# Collaboration
AGENT_COLLABORATION_TIMEOUT=10m
AGENT_CONSENSUS_THRESHOLD=0.8
AGENT_CONSENSUS_MAX_ROUNDS=5
```

### Configuration File

```yaml
agents:
  manager:
    health_check_interval: 30s
    max_concurrent_tasks: 10
    task_timeout: 5m
    
  routing:
    strategy: capability_based
    weights:
      capability: 0.4
      performance: 0.3
      load: 0.2
      cost: 0.1
      
  collaboration:
    timeout: 10m
    strategies:
      consensus:
        threshold: 0.8
        max_rounds: 5
      map_reduce:
        max_parallel: 20
        
  models:
    allowed:
      - claude-3-opus
      - claude-3-sonnet
      - gpt-4
      - titan-v2
    cost_limits:
      per_task: 0.10
      per_session: 1.00
```

## Best Practices

1. **Agent Design**: Create specialized agents with clear capabilities
2. **Health Monitoring**: Implement comprehensive health checks
3. **Load Balancing**: Distribute work based on agent capacity
4. **Error Handling**: Gracefully handle agent failures with fallbacks
5. **Cost Management**: Track and limit AI model usage costs
6. **Performance**: Monitor latency and optimize routing decisions
7. **Collaboration**: Use appropriate strategies for multi-agent tasks
8. **Testing**: Test agent implementations with various workloads

---

Package Version: 1.0.0
Last Updated: 2024-01-10