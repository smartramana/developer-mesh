# Models Package

## Overview

The `models` package contains all domain models for the DevOps MCP AI Agent Orchestration Platform. It provides type-safe data structures for multi-agent coordination and distributed task processing.

## Core Domain Models

### Agent Models

The agent models represent AI agents in the system:

```go
// Agent represents an AI agent in the system
type Agent struct {
    ID           string                 `json:"id" db:"id"`
    TenantID     uuid.UUID              `json:"tenant_id" db:"tenant_id"`
    Name         string                 `json:"name" db:"name"`
    ModelID      string                 `json:"model_id" db:"model_id"`
    Type         string                 `json:"type" db:"type"`
    Status       string                 `json:"status" db:"status"` // available, busy, offline
    Capabilities []string               `json:"capabilities" db:"capabilities"`
    Metadata     map[string]interface{} `json:"metadata" db:"metadata"`
    CreatedAt    time.Time              `json:"created_at" db:"created_at"`
    UpdatedAt    time.Time              `json:"updated_at" db:"updated_at"`
    LastSeenAt   *time.Time             `json:"last_seen_at" db:"last_seen_at"`
}

// Agent States with validation (in agent_status.go)
- AgentStatusOffline   → Starting
- AgentStatusStarting  → Active/Error
- AgentStatusActive    → Draining/Maintenance/Error/Stopping
- AgentStatusDraining  → Inactive/Error
- AgentStatusInactive  → Active/Maintenance/Stopping
- AgentStatusMaintenance → Active/Inactive/Stopping
- AgentStatusError     → Stopping/Maintenance
- AgentStatusStopping  → Offline
```

**Key Features:**
- State machine with transition validation
- Agent capabilities tracking
- Workload metrics tracking (CPU, memory, tasks, etc.)
- Health check results
- Metrics collection integration

### Task Models

Tasks represent units of work that can be assigned to agents:

```go
// Task represents a unit of work in the multi-agent system
type Task struct {
    ID             uuid.UUID    `json:"id" db:"id"`
    TenantID       uuid.UUID    `json:"tenant_id" db:"tenant_id"`
    Type           string       `json:"type" db:"type"`
    Status         TaskStatus   `json:"status" db:"status"`
    Priority       TaskPriority `json:"priority" db:"priority"`
    
    // Agent relationships
    CreatedBy  string  `json:"created_by" db:"created_by"`
    AssignedTo *string `json:"assigned_to,omitempty" db:"assigned_to"`
    
    // Task hierarchy
    ParentTaskID *uuid.UUID `json:"parent_task_id,omitempty" db:"parent_task_id"`
    
    // Task data
    Title       string  `json:"title" db:"title"`
    Description string  `json:"description,omitempty" db:"description"`
    Parameters  JSONMap `json:"parameters" db:"parameters"`
    Result      JSONMap `json:"result,omitempty" db:"result"`
    Error       string  `json:"error,omitempty" db:"error"`
    
    // Execution control
    MaxRetries     int `json:"max_retries" db:"max_retries"`
    RetryCount     int `json:"retry_count" db:"retry_count"`
    TimeoutSeconds int `json:"timeout_seconds" db:"timeout_seconds"`
    
    // Timestamps
    CreatedAt   time.Time  `json:"created_at" db:"created_at"`
    AssignedAt  *time.Time `json:"assigned_at,omitempty" db:"assigned_at"`
    StartedAt   *time.Time `json:"started_at,omitempty" db:"started_at"`
    CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`
    
    // Optimistic locking
    Version int `json:"version" db:"version"`
}

// Task States (no state machine validation in implementation)
- TaskStatusPending
- TaskStatusAssigned
- TaskStatusAccepted
- TaskStatusRejected
- TaskStatusInProgress
- TaskStatusCompleted
- TaskStatusFailed
- TaskStatusCancelled
- TaskStatusTimeout
```

**Features:**
- Task delegation tracking
- Parent-child task relationships
- Retry logic based on retry count
- Priority levels (low, normal, high, critical)
- Timeout enforcement
- Version-based optimistic locking

### Workflow Models

Workflows coordinate multi-agent task execution:

```go
// Workflow represents a multi-agent workflow definition
type Workflow struct {
    ID          uuid.UUID      `json:"id" db:"id"`
    TenantID    uuid.UUID      `json:"tenant_id" db:"tenant_id"`
    Name        string         `json:"name" db:"name"`
    Type        WorkflowType   `json:"type" db:"type"`
    Version     int            `json:"version" db:"version"`
    CreatedBy   string         `json:"created_by" db:"created_by"`
    Agents      JSONMap        `json:"agents" db:"agents"`
    Steps       WorkflowSteps  `json:"steps" db:"steps"`
    Config      JSONMap        `json:"config" db:"config"`
    Description string         `json:"description,omitempty" db:"description"`
    Tags        pq.StringArray `json:"tags,omitempty" db:"tags"`
    IsActive    bool           `json:"is_active" db:"is_active"`
    CreatedAt   time.Time      `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at" db:"updated_at"`
}

// WorkflowExecution represents a running or completed workflow instance
type WorkflowExecution struct {
    ID          uuid.UUID      `json:"id" db:"id"`
    WorkflowID  uuid.UUID      `json:"workflow_id" db:"workflow_id"`
    TenantID    uuid.UUID      `json:"tenant_id" db:"tenant_id"`
    Status      WorkflowStatus `json:"status" db:"status"`
    Context     JSONMap        `json:"context" db:"context"`
    State       JSONMap        `json:"state" db:"state"`
    InitiatedBy string         `json:"initiated_by" db:"initiated_by"`
    Error       string         `json:"error,omitempty" db:"error"`
    StartedAt   time.Time      `json:"started_at" db:"started_at"`
    CompletedAt *time.Time     `json:"completed_at,omitempty" db:"completed_at"`
}

// Workflow Types (implemented)
- WorkflowTypeSequential    // Steps run one after another
- WorkflowTypeParallel      // Steps run simultaneously
- WorkflowTypeConditional   // Conditional execution
- WorkflowTypeCollaborative // Multi-agent collaboration
```

**Workflow Execution States:**
- WorkflowStatusPending
- WorkflowStatusRunning
- WorkflowStatusPaused
- WorkflowStatusCompleted
- WorkflowStatusFailed
- WorkflowStatusCancelled
- WorkflowStatusTimeout

### Document Models

Documents support collaborative editing:

```go
// SharedDocument represents a collaborative document
type SharedDocument struct {
    ID            uuid.UUID  `json:"id" db:"id"`
    WorkspaceID   uuid.UUID  `json:"workspace_id" db:"workspace_id"`
    TenantID      uuid.UUID  `json:"tenant_id" db:"tenant_id"`
    Type          string     `json:"type" db:"type"`
    Title         string     `json:"title" db:"title"`
    Content       string     `json:"content" db:"content"`
    ContentType   string     `json:"content_type" db:"content_type"`
    Version       int64      `json:"version" db:"version"`
    CreatedBy     string     `json:"created_by" db:"created_by"`
    Metadata      JSONMap    `json:"metadata" db:"metadata"`
    LockedBy      *string    `json:"locked_by,omitempty" db:"locked_by"`
    LockedAt      *time.Time `json:"locked_at,omitempty" db:"locked_at"`
    LockExpiresAt *time.Time `json:"lock_expires_at,omitempty" db:"lock_expires_at"`
    CreatedAt     time.Time  `json:"created_at" db:"created_at"`
    UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

// DocumentOperation represents a CRDT operation on a document
type DocumentOperation struct {
    ID                uuid.UUID  `json:"id" db:"id"`
    DocumentID        uuid.UUID  `json:"document_id" db:"document_id"`
    TenantID          uuid.UUID  `json:"tenant_id" db:"tenant_id"`
    AgentID           string     `json:"agent_id" db:"agent_id"`
    OperationType     string     `json:"operation_type" db:"operation_type"`
    OperationData     JSONMap    `json:"operation_data" db:"operation_data"`
    VectorClock       JSONMap    `json:"vector_clock" db:"vector_clock"`
    SequenceNumber    int64      `json:"sequence_number" db:"sequence_number"`
    Timestamp         time.Time  `json:"timestamp" db:"timestamp"`
    ParentOperationID *uuid.UUID `json:"parent_operation_id,omitempty" db:"parent_operation_id"`
    IsApplied         bool       `json:"is_applied" db:"is_applied"`
}
```

**Features:**
- Document locking mechanism
- CRDT operations for conflict-free editing
- Vector clocks for operation ordering
- Document snapshots for version history
- Conflict resolution tracking

## Binary WebSocket Protocol

High-performance binary protocol for agent communication:

```go
// BinaryHeader represents the binary protocol header (24 bytes)
type BinaryHeader struct {
    Magic      uint32 // 0x4D435057 "MCPW"
    Version    uint8  // Protocol version (1)
    Type       uint8  // Message type
    Flags      uint16 // Compression, encryption flags
    SequenceID uint64 // Message sequence ID
    Method     uint16 // Method enum (not string)
    Reserved   uint16 // Padding for alignment
    DataSize   uint32 // Payload size
}

// Method enums for binary protocol
const (
    MethodInitialize       uint16 = 1
    MethodToolList         uint16 = 2
    MethodToolExecute      uint16 = 3
    MethodContextGet       uint16 = 4
    MethodContextUpdate    uint16 = 5
    MethodEventSubscribe   uint16 = 6
    MethodEventUnsubscribe uint16 = 7
    MethodPing             uint16 = 8
    MethodPong             uint16 = 9
)

// Flag bits
const (
    FlagCompressed uint16 = 1 << 0
    FlagEncrypted  uint16 = 1 << 1
    FlagBatch      uint16 = 1 << 2
)
```

**Implementation Features:**
- Magic number "MCPW" (0x4D435057) for protocol identification
- Binary encoding with BigEndian byte order
- Optional compression flag
- Method enums for fast routing
- 1MB maximum payload size
- Sequence ID for message tracking

## Additional Features

### Task Delegation

```go
// TaskDelegation represents a task being delegated between agents
type TaskDelegation struct {
    ID             uuid.UUID      `json:"id" db:"id"`
    TaskID         uuid.UUID      `json:"task_id" db:"task_id"`
    FromAgentID    string         `json:"from_agent_id" db:"from_agent_id"`
    ToAgentID      string         `json:"to_agent_id" db:"to_agent_id"`
    Reason         string         `json:"reason,omitempty" db:"reason"`
    DelegationType DelegationType `json:"delegation_type" db:"delegation_type"`
    Metadata       JSONMap        `json:"metadata" db:"metadata"`
    DelegatedAt    time.Time      `json:"delegated_at" db:"delegated_at"`
}

// Delegation Types
- DelegationManual      // Manual delegation
- DelegationAutomatic   // Automatic delegation
- DelegationFailover    // Failover delegation
- DelegationLoadBalance // Load balance delegation
```

### Workflow Steps

```go
// WorkflowStep represents a step in a workflow
type WorkflowStep struct {
    ID              string                 `json:"id"`
    Name            string                 `json:"name"`
    Description     string                 `json:"description,omitempty"`
    Type            string                 `json:"type"`
    Action          string                 `json:"action"`
    AgentID         string                 `json:"agent_id"`
    Input           map[string]interface{} `json:"input"`
    Config          map[string]interface{} `json:"config"`
    TimeoutSeconds  int                    `json:"timeout_seconds,omitempty"`
    Retries         int                    `json:"retries"`
    RetryPolicy     WorkflowRetryPolicy    `json:"retry_policy,omitempty"`
    ContinueOnError bool                   `json:"continue_on_error"`
    Dependencies    []string               `json:"dependencies"`
    OnFailure       string                 `json:"on_failure,omitempty"`
}
```

### State Machine Features

The agent status model includes state machine validation:

```go
// Valid state transitions
var validAgentTransitions = map[AgentStatus][]AgentStatus{
    AgentStatusOffline:     {AgentStatusStarting},
    AgentStatusStarting:    {AgentStatusActive, AgentStatusError},
    AgentStatusActive:      {AgentStatusDraining, AgentStatusMaintenance, AgentStatusError, AgentStatusStopping},
    // ... etc
}

// CanTransitionTo checks if a status transition is valid
func (s AgentStatus) CanTransitionTo(target AgentStatus) bool

// TransitionTo performs a validated state transition with metrics
func (s AgentStatus) TransitionTo(target AgentStatus, metrics MetricsClient) (AgentStatus, error)
```

## JSONMap Utility

Flexible map type for dynamic data with database serialization:

```go
type JSONMap map[string]interface{}

// Implements driver.Valuer for database serialization
func (m JSONMap) Value() (driver.Value, error)

// Implements sql.Scanner for database deserialization
func (m *JSONMap) Scan(value interface{}) error
```

## Extended Models

### Workflow Templates

```go
// WorkflowTemplate represents a reusable workflow template
type WorkflowTemplate struct {
    ID          uuid.UUID              `json:"id" db:"id"`
    Name        string                 `json:"name" db:"name"`
    Description string                 `json:"description" db:"description"`
    Category    string                 `json:"category" db:"category"`
    Definition  map[string]interface{} `json:"definition" db:"definition"`
    Parameters  []TemplateParameter    `json:"parameters" db:"parameters"`
    CreatedBy   string                 `json:"created_by" db:"created_by"`
    CreatedAt   time.Time              `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time              `json:"updated_at" db:"updated_at"`
}
```

### Execution Tracking

```go
// ExecutionStatus represents detailed workflow execution status
type ExecutionStatus struct {
    ExecutionID    uuid.UUID              `json:"execution_id"`
    WorkflowID     uuid.UUID              `json:"workflow_id"`
    Status         string                 `json:"status"`
    Progress       int                    `json:"progress"`
    CurrentSteps   []string               `json:"current_steps"`
    CompletedSteps int                    `json:"completed_steps"`
    TotalSteps     int                    `json:"total_steps"`
    StartedAt      time.Time              `json:"started_at"`
    UpdatedAt      time.Time              `json:"updated_at"`
    EstimatedEnd   *time.Time             `json:"estimated_end,omitempty"`
    Metrics        map[string]interface{} `json:"metrics,omitempty"`
}
```

## Best Practices

### 1. State Management

For agent status transitions, use the validation methods:
```go
// Good
if status.CanTransitionTo(newStatus) {
    status, err = status.TransitionTo(newStatus, metrics)
}

// Note: Most models don't have built-in state validation
```

### 2. Version Control

Use version fields for optimistic locking (where available):
```go
// Task has version field
task.Version++
// Workflow has version field  
workflow.Version++
```

### 3. Binary Protocol

Use the provided header parsing/writing functions:
```go
// Parse binary header
header, err := ParseBinaryHeader(reader)

// Write binary header
err := WriteBinaryHeader(writer, header)
```

### 4. Type Safety

Use the defined constants for statuses and types:
```go
// Use constants
task.Status = TaskStatusInProgress
workflow.Status = WorkflowStatusRunning

// Not strings
task.Status = "in_progress" // Avoid
```

## Testing

The models package is designed for testing:

```bash
# Run tests
go test ./pkg/models/...

# With coverage
go test -cover ./pkg/models/...
```

## Implementation Status

**Implemented:**
- Basic domain models (Agent, Task, Workflow, Document)
- Binary WebSocket protocol with MCPW magic
- State machine for agent status transitions
- Task delegation tracking
- Workflow execution tracking
- Document collaboration with CRDT operations
- JSONMap utility with DB serialization

**Not Implemented:**
- State validation for tasks and workflows
- Advanced coordination patterns (MapReduce, etc.)
- Protocol buffer support
- GraphQL schema generation
- Comprehensive helper methods on JSONMap

## References

- [System Overview](../../docs/architecture/system-overview.md)
- [WebSocket API Reference](../../docs/api-reference/mcp-server-reference.md)