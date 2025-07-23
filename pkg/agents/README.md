# Agents Package

> **Purpose**: Agent configuration and preferences management for the Developer Mesh platform
> **Status**: Basic Implementation
> **Dependencies**: Database models, configuration management

## Overview

The agents package provides configuration management for AI agents, including embedding strategies, model preferences, and cost constraints. It works in conjunction with the WebSocket server for actual agent runtime management.

**Note**: This package focuses on agent configuration storage. The actual agent orchestration, registration, and task routing is implemented in the MCP Server's WebSocket handlers (`apps/mcp-server/internal/api/websocket/`).

## Architecture

```
┌─────────────────────────────────────────────────┐
│              Agent Configuration System              │
├─────────────────────────────────────────────────┤
│                                                     │
│   AgentConfigService ────► Repository               │
│          │                     │                    │
│          │                     │                    │
│          ▼                     ▼                    │
│   AgentConfiguration    Database Storage           │
│   - Agent ID                                       │
│   - Embedding Strategy                             │
│   - Model Preferences                              │
│   - Cost Constraints                               │
│                                                     │
└─────────────────────────────────────────────────┘

Actual Agent Runtime (in mcp-server/websocket):
┌─────────────────────────────────────────────────┐
│  WebSocket Handlers ──► Agent Registry (DB/Memory) │
│         │                        │                │
│         │                        ▼                │
│         │                 Task Assignment          │
│         │                 - Round Robin            │
│         │                 - Least Loaded           │
│         ▼                                         │
│  Collaboration Sessions                           │
└─────────────────────────────────────────────────┘
```

## Core Components (Actual Implementation)

### 1. Agent Configuration (pkg/agents/config.go)

```go
// AgentConfiguration stores preferences for an AI agent
type AgentConfiguration struct {
    AgentID            string                 `json:"agent_id"`
    EmbeddingStrategy  string                 `json:"embedding_strategy"`
    ModelPreferences   []ModelPreference      `json:"model_preferences"`
    Constraints        Constraints            `json:"constraints"`
    FallbackBehavior   FallbackBehavior       `json:"fallback_behavior"`
    LastUpdated        time.Time              `json:"last_updated"`
}

// ModelPreference defines model selection criteria
type ModelPreference struct {
    TaskType       string   `json:"task_type"`
    PrimaryModels  []string `json:"primary_models"`
    FallbackModels []string `json:"fallback_models,omitempty"`
}

// Constraints for agent operations
type Constraints struct {
    MaxTokensPerRequest  int     `json:"max_tokens_per_request,omitempty"`
    MaxCostPerMonthUSD   float64 `json:"max_cost_per_month_usd,omitempty"`
    PreferredDimensions  int     `json:"preferred_dimensions,omitempty"`
    AllowDimensionReduction bool `json:"allow_dimension_reduction,omitempty"`
}
```

### 2. Agent Service (pkg/agents/service.go)

```go
// Service manages agent configurations
type Service interface {
    CreateOrUpdateAgentConfig(ctx context.Context, config *AgentConfiguration) error
    GetAgentConfig(ctx context.Context, agentID string) (*AgentConfiguration, error)
    DeleteAgentConfig(ctx context.Context, agentID string) error
    ListAgentConfigs(ctx context.Context) ([]*AgentConfiguration, error)
}
```

### 3. Actual Agent Model (pkg/models/agent.go)

```go
// Agent represents a registered AI agent (database model)
type Agent struct {
    ID           string                 `json:"id"`
    Name         string                 `json:"name"`
    Type         string                 `json:"type"`
    Model        string                 `json:"model,omitempty"`
    Capabilities []string               `json:"capabilities"`
    Metadata     map[string]interface{} `json:"metadata,omitempty"`
    Status       string                 `json:"status"`
    LastSeen     time.Time              `json:"last_seen"`
    CreatedAt    time.Time              `json:"created_at"`
    UpdatedAt    time.Time              `json:"updated_at"`
}
```

### 4. Agent Registry Implementation (apps/mcp-server/internal/api/websocket/)

**Note**: The actual agent runtime management is in the MCP Server, not this package.

```go
// From agent_registry.go - In-memory registry
type InMemoryAgentRegistry struct {
    agents      map[string]*models.Agent
    connections map[string]*websocket.Conn
    mu          sync.RWMutex
}

// From agent_registry_db.go - Database-backed registry
type DBAgentRegistry struct {
    db          *sql.DB
    connections map[string]*websocket.Conn
    cache       map[string]*models.Agent
    mu          sync.RWMutex
}

// RegisterAgent stores agent information
func (r *DBAgentRegistry) RegisterAgent(agent *models.Agent, conn *websocket.Conn) error {
    // Store in database
    _, err := r.db.Exec(`
        INSERT INTO agents (id, name, type, model, capabilities, metadata, status)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        ON CONFLICT (id) DO UPDATE SET
            name = EXCLUDED.name,
            type = EXCLUDED.type,
            model = EXCLUDED.model,
            capabilities = EXCLUDED.capabilities,
            metadata = EXCLUDED.metadata,
            status = EXCLUDED.status,
            last_seen = CURRENT_TIMESTAMP
    `, agent.ID, agent.Name, agent.Type, agent.Model, 
       pq.Array(agent.Capabilities), agent.Metadata, agent.Status)
    
    // Store connection
    r.connections[agent.ID] = conn
    
    return err
}
```

### 5. Task Assignment Implementation (pkg/services/assignment_engine.go)

```go
// AssignmentStrategy defines how tasks are assigned to agents
type AssignmentStrategy interface {
    SelectAgent(task *models.Task, agents []*models.Agent) (*models.Agent, error)
}

// RoundRobinStrategy - implemented ✅
type RoundRobinStrategy struct {
    counter uint64
}

func (s *RoundRobinStrategy) SelectAgent(task *models.Task, agents []*models.Agent) (*models.Agent, error) {
    if len(agents) == 0 {
        return nil, ErrNoAgentsAvailable
    }
    
    index := atomic.AddUint64(&s.counter, 1) % uint64(len(agents))
    return agents[index], nil
}

// LeastLoadedStrategy - implemented ✅
type LeastLoadedStrategy struct {
    getWorkload func(agentID string) (float64, error)
}

func (s *LeastLoadedStrategy) SelectAgent(task *models.Task, agents []*models.Agent) (*models.Agent, error) {
    if len(agents) == 0 {
        return nil, ErrNoAgentsAvailable
    }
    
    var selectedAgent *models.Agent
    minWorkload := math.MaxFloat64
    
    for _, agent := range agents {
        workload, err := s.getWorkload(agent.ID)
        if err != nil {
            continue
        }
        
        if workload < minWorkload {
            minWorkload = workload
            selectedAgent = agent
        }
    }
    
    if selectedAgent == nil {
        return nil, ErrNoSuitableAgent
    }
    
    return selectedAgent, nil
}
```

### 6. WebSocket Agent Registration (apps/mcp-server/internal/api/websocket/handlers.go)

```go
// HandleAgentRegister processes agent registration via WebSocket
func (h *Handler) HandleAgentRegister(ctx context.Context, conn *websocket.Conn, msg json.RawMessage) error {
    var req InitializeRequest
    if err := json.Unmarshal(msg, &req); err != nil {
        return err
    }
    
    // Create agent from request
    agent := &models.Agent{
        ID:           req.ClientInfo.Name, // Agent provides its ID
        Name:         req.ClientInfo.Name,
        Type:         "ai-agent",
        Capabilities: req.Capabilities,
        Status:       "active",
        LastSeen:     time.Now(),
    }
    
    // Register in registry
    if err := h.agentRegistry.RegisterAgent(agent, conn); err != nil {
        return err
    }
    
    // Store connection mapping
    h.connToAgent[conn] = agent.ID
    h.agentToConn[agent.ID] = conn
    
    // Send success response
    return h.sendResponse(conn, "initialized", InitializedResponse{
        ProtocolVersion: "2024-11-05",
        ServerInfo: ServerInfo{
            Name:    "developer-mesh",
            Version: "1.0.0",
        },
    })
}

// Agent discovery by capability
func (h *Handler) findAgentsByCapability(capability string) ([]*models.Agent, error) {
    return h.agentRegistry.GetAgentsByCapability(capability)
}
```

### 7. Collaboration Modes (Defined but not fully implemented)

From pkg/models/distributed_task_complete.go:

```go
// CoordinationMode defines how tasks are coordinated
type CoordinationMode string

const (
    CoordinationModeParallel    CoordinationMode = "parallel"     // Defined ✅
    CoordinationModeSequential  CoordinationMode = "sequential"   // Defined ✅
    CoordinationModePipeline    CoordinationMode = "pipeline"     // Defined ✅
    CoordinationModeMapReduce   CoordinationMode = "map_reduce"   // Defined ✅
    CoordinationModeLeaderElect CoordinationMode = "leader_elect" // Defined ✅
)
```

**Note**: While these coordination modes are defined in the models, the actual execution strategies (MapReduce implementation, consensus algorithms, etc.) are not implemented. Basic collaboration session management exists in the WebSocket handlers.

## Usage Examples

### Configuring an Agent

```go
// Create agent configuration service
service := agents.NewService(repository, logger)

// Configure agent preferences
config := &agents.AgentConfiguration{
    AgentID:           "claude-assistant-001",
    EmbeddingStrategy: "quality", // quality, speed, cost, balanced
    ModelPreferences: []agents.ModelPreference{
        {
            TaskType:       "general_qa",
            PrimaryModels:  []string{"text-embedding-3-large"},
            FallbackModels: []string{"text-embedding-3-small"},
        },
        {
            TaskType:       "code_analysis",
            PrimaryModels:  []string{"amazon.titan-embed-text-v2:0"},
            FallbackModels: []string{"text-embedding-3-large"},
        },
    },
    Constraints: agents.Constraints{
        MaxTokensPerRequest: 8000,
        MaxCostPerMonthUSD:  100.0,
        PreferredDimensions: 1536,
    },
}

// Save configuration
err := service.CreateOrUpdateAgentConfig(ctx, config)
```

### Agent Registration via WebSocket

```javascript
// Client-side agent registration
const ws = new WebSocket('wss://dev-mesh.io/ws', ['mcp.v1']);

ws.onopen = () => {
    // Register agent
    ws.send(JSON.stringify({
        jsonrpc: "2.0",
        method: "initialize",
        params: {
            protocolVersion: "2024-11-05",
            capabilities: {
                tools: {},
                prompts: {},
                resources: {}
            },
            clientInfo: {
                name: "my-ai-agent",
                version: "1.0.0"
            }
        },
        id: 1
    }));
};
```

## What's Actually Implemented

### ✅ Working Features

1. **Agent Configuration Management** (this package)
   - Store agent preferences and constraints
   - Configure embedding strategies
   - Set model preferences by task type
   - Define cost and token limits

2. **WebSocket Agent Registration** (in mcp-server)
   - Agents connect via WebSocket protocol
   - Register with capabilities
   - Store in database or memory
   - Track connection status

3. **Task Assignment**
   - Round-robin strategy
   - Least-loaded strategy
   - Basic capability matching

4. **Agent Discovery**
   - Find agents by capability
   - List all registered agents
   - Track agent status

### ❌ Not Implemented

1. **Advanced Routing**
   - No capability-based scoring algorithm
   - No performance-based routing
   - No cost optimization routing

2. **Multi-Agent Collaboration Execution**
   - Coordination modes are defined but not implemented
   - No MapReduce execution
   - No consensus algorithms
   - No pipeline execution

3. **Health Monitoring**
   - No automated health checks
   - No health history tracking
   - No automatic failover

4. **Metrics & Performance Tracking**
   - Basic metrics only
   - No detailed performance history
   - No cost tracking per agent

## Directory Structure

```
pkg/agents/
├── config.go          # Agent configuration structures
├── repository.go      # Repository interface for persistence
├── service.go         # Service for managing agent configs
└── README.md          # This file

apps/mcp-server/internal/api/websocket/
├── agent_registry.go        # In-memory agent registry
├── agent_registry_db.go     # Database-backed registry
├── handlers.go              # WebSocket message handlers
└── collaboration_handlers.go # Collaboration session management

pkg/services/
└── assignment_engine.go     # Task assignment strategies
```

## Future Enhancements

1. **Move Core Orchestration**: Consolidate agent runtime management from mcp-server into this package
2. **Implement Advanced Routing**: Add capability-based scoring and performance routing
3. **Build Collaboration Strategies**: Implement MapReduce, Consensus, and Pipeline execution
4. **Add Health Monitoring**: Automated health checks and failover
5. **Enhanced Metrics**: Detailed performance and cost tracking
6. **Agent SDK**: Provide base classes for custom agent implementations

## Current Configuration

### Environment Variables

```bash
# Currently used
DATABASE_URL=postgres://user:password@localhost:5432/devops_mcp

# WebSocket configuration (in mcp-server)
WEBSOCKET_MAX_CONNECTIONS=10000
WEBSOCKET_PING_INTERVAL=30s
```

### Agent Configuration Structure

```yaml
# Example agent configuration (stored in database)
agent_id: "claude-assistant-001"
embedding_strategy: "balanced"
model_preferences:
  - task_type: "general_qa"
    primary_models: ["text-embedding-3-small"]
    fallback_models: ["text-embedding-ada-002"]
constraints:
  max_tokens_per_request: 8000
  max_cost_per_month_usd: 100.0
```

## Best Practices

1. **Configuration**: Set appropriate embedding strategies based on use case
2. **Model Selection**: Configure primary and fallback models for reliability
3. **Cost Control**: Set monthly cost limits to prevent overruns
4. **Token Limits**: Configure max tokens based on model capabilities
5. **Connection Management**: Handle WebSocket disconnections gracefully
6. **Capability Declaration**: Be specific about agent capabilities
7. **Error Handling**: Implement retry logic for transient failures
8. **Monitoring**: Track agent registration and task assignment metrics

---

Package Version: 1.0.0
Last Updated: 2024-01-23

**Note**: This documentation reflects the actual implementation. For the envisioned full agent orchestration system, see the future enhancements section.