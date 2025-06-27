package events

import (
	"time"
)

// BaseEvent contains common fields for all events
type BaseEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	TenantID  string    `json:"tenant_id"`
	AgentID   string    `json:"agent_id"`
	Version   string    `json:"version"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// TaskCreatedEvent is published when a new task is created
type TaskCreatedEvent struct {
	BaseEvent
	TaskID      string  `json:"task_id"`
	TaskType    string  `json:"task_type"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Priority    string  `json:"priority"`
	AssignedTo  *string `json:"assigned_to,omitempty"`
}

// TaskStatusChangedEvent is published when task status changes
type TaskStatusChangedEvent struct {
	BaseEvent
	TaskID    string `json:"task_id"`
	OldStatus string `json:"old_status"`
	NewStatus string `json:"new_status"`
	Reason    string `json:"reason,omitempty"`
}

// TaskDelegatedEvent is published when a task is delegated
type TaskDelegatedEvent struct {
	BaseEvent
	TaskID       string `json:"task_id"`
	FromAgentID  string `json:"from_agent_id"`
	ToAgentID    string `json:"to_agent_id"`
	DelegationID string `json:"delegation_id"`
	Reason       string `json:"reason"`
}

// TaskAcceptedEvent is published when a delegated task is accepted
type TaskAcceptedEvent struct {
	BaseEvent
	TaskID  string `json:"task_id"`
	AgentID string `json:"agent_id"`
}

// TaskRejectedEvent is published when a delegated task is rejected
type TaskRejectedEvent struct {
	BaseEvent
	TaskID  string `json:"task_id"`
	AgentID string `json:"agent_id"`
	Reason  string `json:"reason"`
}

// TaskCompletedEvent is published when a task is completed
type TaskCompletedEvent struct {
	BaseEvent
	TaskID      string      `json:"task_id"`
	AgentID     string      `json:"agent_id"`
	Result      interface{} `json:"result,omitempty"`
	CompletedAt time.Time   `json:"completed_at"`
}

// TaskFailedEvent is published when a task fails
type TaskFailedEvent struct {
	BaseEvent
	TaskID   string `json:"task_id"`
	AgentID  string `json:"agent_id"`
	Error    string `json:"error"`
	FailedAt time.Time `json:"failed_at"`
}

// TaskEscalatedEvent is published when a task is escalated
type TaskEscalatedEvent struct {
	BaseEvent
	TaskID         string `json:"task_id"`
	EscalatedTo    string `json:"escalated_to"`
	EscalationType string `json:"escalation_type"`
	Reason         string `json:"reason"`
}

// WorkspaceMemberAddedEvent is published when a member is added to a workspace
type WorkspaceMemberAddedEvent struct {
	BaseEvent
	WorkspaceID string `json:"workspace_id"`
	MemberID    string `json:"member_id"`
	Role        string `json:"role"`
	AddedBy     string `json:"added_by"`
}

// WorkspaceMemberRemovedEvent is published when a member is removed from a workspace
type WorkspaceMemberRemovedEvent struct {
	BaseEvent
	WorkspaceID string `json:"workspace_id"`
	MemberID    string `json:"member_id"`
	RemovedBy   string `json:"removed_by"`
}

// WorkspaceMemberRoleChangedEvent is published when a member's role is changed
type WorkspaceMemberRoleChangedEvent struct {
	BaseEvent
	WorkspaceID string `json:"workspace_id"`
	MemberID    string `json:"member_id"`
	OldRole     string `json:"old_role"`
	NewRole     string `json:"new_role"`
	ChangedBy   string `json:"changed_by"`
}

// Implement common event methods for BaseEvent
func (e BaseEvent) GetID() string         { return e.ID }
func (e BaseEvent) GetType() string       { return e.Type }
func (e BaseEvent) GetTimestamp() time.Time { return e.Timestamp }
func (e BaseEvent) GetTenantID() string   { return e.TenantID }