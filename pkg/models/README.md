# Models Package

## Overview

The `models` package contains all domain models for the DevOps MCP AI Agent Orchestration Platform. It provides type-safe, production-ready data structures with built-in state machines, validation, and metrics collection for multi-agent coordination and distributed task processing.

## Core Domain Models

### Agent Models

The agent models represent AI agents in the system with comprehensive lifecycle management:

```go
// Agent represents an AI agent with capabilities and workload tracking
type Agent struct {
    ID           uuid.UUID        `json:"id"`
    Name         string          `json:"name"`
    Status       AgentStatus     `json:"status"`
    Capabilities []AgentCapability `json:"capabilities"`
    Workload     *AgentWorkload  `json:"workload,omitempty"`
    Health       *AgentHealth    `json:"health,omitempty"`
}

// Agent States (complete state machine)
- AgentStatusOffline   → Starting
- AgentStatusStarting  → Active/Offline
- AgentStatusActive    → Draining/Inactive/Offline
- AgentStatusDraining  → Inactive
- AgentStatusInactive  → Active/Offline
```

**Key Features:**
- Complete state machine with transition validation
- Workload tracking (tasks in progress, completed, average time)
- Health monitoring with last heartbeat
- Capability-based routing (compute, storage, network, orchestrate, analyze, secure)
- Automatic metrics collection on state changes

### Task Models

Tasks represent units of work that can be assigned to agents:

```go
// Task represents a work item that can be processed by agents
type Task struct {
    ID             uuid.UUID              `json:"id"`
    Type           string                `json:"type"`
    Status         TaskStatus            `json:"status"`
    Priority       TaskPriority          `json:"priority"`
    AssignedAgent  *uuid.UUID            `json:"assigned_agent"`
    Parameters     JSONMap               `json:"parameters"`
    Result         JSONMap               `json:"result"`
    Error          *string               `json:"error"`
    RetryCount     int                   `json:"retry_count"`
    MaxRetries     int                   `json:"max_retries"`
}

// Task States
- TaskStatusPending    → Assigned/Cancelled
- TaskStatusAssigned   → InProgress/Cancelled
- TaskStatusInProgress → Completed/Failed/Cancelled
- TaskStatusCompleted  (terminal)
- TaskStatusFailed     → Pending (retry)
- TaskStatusCancelled  (terminal)
```

**Advanced Features:**
- Task delegation between agents
- Parent-child task relationships
- Retry logic with exponential backoff
- Priority-based scheduling
- Flexible JSONMap for parameters/results

### Workflow Models

Workflows coordinate multi-agent task execution with various patterns:

```go
// Workflow represents a multi-agent workflow definition
type Workflow struct {
    ID              uuid.UUID           `json:"id"`
    Name            string             `json:"name"`
    Type            WorkflowType       `json:"type"`
    Status          WorkflowStatus     `json:"status"`
    Steps           []WorkflowStep     `json:"steps"`
    ExecutionPlan   *WorkflowExecution `json:"execution_plan"`
    ResourceUsage   *ResourceUsage     `json:"resource_usage"`
}

// Workflow Types
- WorkflowTypeSequential  // Steps run one after another
- WorkflowTypeParallel    // Steps run simultaneously
- WorkflowTypeDAG         // Directed Acyclic Graph
- WorkflowTypeSaga        // Distributed transaction pattern
- WorkflowTypeStateMachine // State-based execution
- WorkflowTypeEventDriven  // Event-triggered execution
```

**Workflow States:**
```
Pending → Running → Completed/Failed/Cancelled
Paused ↔ Running
Failed → Pending (retry)
```

**Step States:**
```
Pending → Running → Completed/Failed/Skipped
Blocked → Pending
Failed → Pending (retry)
```

### Document Models

Documents support collaborative editing with conflict resolution:

```go
// Document represents a collaborative document
type Document struct {
    ID          uuid.UUID      `json:"id"`
    Type        DocumentType   `json:"type"`
    Name        string        `json:"name"`
    Content     string        `json:"content"`
    Version     int64         `json:"version"`
    UpdatedAt   time.Time     `json:"updated_at"`
    UpdatedBy   uuid.UUID     `json:"updated_by"`
}

// Document Types
- DocumentTypeMarkdown
- DocumentTypeJSON/YAML
- DocumentTypeCode/Script
- DocumentTypeDiagram/PlantUML
- DocumentTypeRunbook/Template
- DocumentTypeDocumentation/Notebook
```

**Access Control Roles:**
- Owner: Full control
- Admin: Manage members and settings
- Editor: Read/write access
- Commenter: Read and comment
- Viewer: Read-only
- Guest: Limited read access

## Binary WebSocket Protocol

High-performance binary protocol for agent communication:

```go
// Header is the binary protocol header (24 bytes)
type Header struct {
    Magic      [4]byte  // "DMCP" magic number
    Version    uint8    // Protocol version
    Type       uint8    // Message type
    Method     uint16   // Method enum
    Flags      uint8    // Compression, encryption flags
    Reserved   [3]byte  // Future use
    PayloadLen uint32   // Payload size
    RequestID  uint64   // Request identifier
}

// Message Types
- TypeRequest      // Client request
- TypeResponse     // Server response
- TypeNotification // Async notification
- TypeError        // Error response
- TypePing/Pong    // Keepalive
```

**Performance Features:**
- Binary encoding for efficiency
- Optional gzip compression
- Method enums for fast routing
- Connection state management
- Automatic reconnection support

## Distributed Task Coordination

Support for complex multi-agent task patterns:

```go
// DistributedTask coordinates work across multiple agents
type DistributedTask struct {
    ID               uuid.UUID            `json:"id"`
    CoordinationMode CoordinationMode     `json:"coordination_mode"`
    CompletionMode   CompletionMode       `json:"completion_mode"`
    Partitions       []TaskPartition      `json:"partitions"`
    ExecutionPlan    *ExecutionPlan       `json:"execution_plan"`
    Progress         *TaskProgress        `json:"progress"`
}

// Coordination Modes
- ModeParallel      // All tasks run simultaneously
- ModeSequential    // Tasks run in order
- ModePipeline      // Data flows through stages
- ModeMapReduce     // Map-reduce pattern
- ModeLeaderElect   // Leader election pattern

// Completion Modes  
- CompletionAll       // All must complete
- CompletionAny       // First to complete wins
- CompletionMajority  // >50% must complete
- CompletionThreshold // Custom threshold
- CompletionBestOf    // Best result wins
```

## State Machine Features

All stateful models include:

1. **Validation**: Ensure valid state transitions
2. **Metrics**: Automatic collection on transitions
3. **Events**: State change notifications
4. **History**: Transition audit trail
5. **Concurrency**: Safe concurrent access

Example usage:
```go
// Transition agent state with validation
if agent.CanTransitionTo(AgentStatusActive) {
    oldStatus := agent.Status
    agent.TransitionTo(AgentStatusActive)
    
    // Metrics automatically collected
    agentStatusTransitions.WithLabelValues(
        string(oldStatus), 
        string(agent.Status),
    ).Inc()
}
```

## JSONMap Utility

Flexible map type for dynamic data:

```go
type JSONMap map[string]interface{}

// Helper methods
func (j JSONMap) GetString(key string) (string, bool)
func (j JSONMap) GetInt(key string) (int, bool) 
func (j JSONMap) GetBool(key string) (bool, bool)
func (j JSONMap) GetStringSlice(key string) ([]string, bool)
func (j JSONMap) GetMap(key string) (JSONMap, bool)
```

## Integration Models

### GitHub Integration

Query models for GitHub operations:

```go
// RepositoryQuery for repository operations
type RepositoryQuery struct {
    Owner      string
    Repo       string
    Path       string
    Branch     string
    Since      *time.Time
    PageSize   int
}

// Supports: repos, PRs, issues, commits, files
```

### Entity Relationships

Track relationships between entities:

```go
// Relationship between entities
type Relationship struct {
    ID           string
    FromEntity   EntityReference
    ToEntity     EntityReference
    Type         RelationshipType
    Properties   map[string]interface{}
    Strength     float64
}

// Relationship Types
- References, Contains, Creates
- Modifies, Implements, Tests
- Documents, Depends, Conflicts
```

## Best Practices

### 1. State Management

Always use provided transition methods:
```go
// Good
if agent.CanTransitionTo(newStatus) {
    agent.TransitionTo(newStatus)
}

// Bad  
agent.Status = newStatus // Bypasses validation
```

### 2. Version Control

Use version fields for optimistic locking:
```go
// Update with version check
doc.Version++
err := repo.UpdateDocument(ctx, doc)
if err == ErrVersionConflict {
    // Handle conflict
}
```

### 3. Metrics Collection

Leverage built-in metrics:
```go
// Metrics automatically collected for:
- State transitions
- Task assignments
- Workflow execution
- Error rates
```

### 4. Binary Protocol

Use provided encoders/decoders:
```go
// Encode message
data, err := msg.Encode()

// Decode message  
msg, err := DecodeMessage(data)
```

## Testing

The package includes comprehensive test coverage:

```bash
# Run tests
go test ./pkg/models/...

# With coverage
go test -cover ./pkg/models/...

# Benchmark binary protocol
go test -bench=. ./pkg/models/websocket/
```

## Migration Guide

When updating models:

1. Add new fields with `omitempty` tag
2. Maintain backward compatibility
3. Update validation rules
4. Add migration logic if needed
5. Update tests

## Performance Considerations

- **Binary Protocol**: ~70% smaller than JSON
- **State Machines**: O(1) transition validation
- **JSONMap**: Lazy parsing for efficiency
- **Metrics**: Minimal overhead (<1μs)

## Future Enhancements

- [ ] Protocol buffer support
- [ ] GraphQL schema generation
- [ ] OpenAPI spec generation
- [ ] Event sourcing support
- [ ] CQRS patterns

## References

- [System Overview](../../docs/architecture/system-overview.md)
- [Binary Protocol Spec](../../docs/api-reference/websocket-protocol.md)
- [State Machine Patterns](../../docs/patterns/state-machines.md)