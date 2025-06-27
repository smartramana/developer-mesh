package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// notificationServiceImpl handles notifications across the system
// The interface is defined in interfaces.go

// notificationService implements NotificationService
type notificationService struct {
	BaseService
	notifications chan *notification
}

type notification struct {
	ID        uuid.UUID
	Type      string
	Target    string
	Message   interface{}
	Timestamp time.Time
}

// NewNotificationService creates a new notification service
func NewNotificationService(config ServiceConfig) NotificationService {
	ns := &notificationService{
		BaseService:   NewBaseService(config),
		notifications: make(chan *notification, 1000),
	}

	// Start notification processor
	go ns.processNotifications()

	return ns
}

func (s *notificationService) processNotifications() {
	for n := range s.notifications {
		// Process notification asynchronously
		s.config.Logger.Info("Processing notification", map[string]interface{}{
			"id":     n.ID,
			"type":   n.Type,
			"target": n.Target,
		})

		// In production, this would send to WebSocket connections,
		// message queues, webhooks, etc.
	}
}

func (s *notificationService) NotifyTaskAssigned(ctx context.Context, agentID string, task interface{}) error {
	n := &notification{
		ID:     uuid.New(),
		Type:   "task.assigned",
		Target: agentID,
		Message: map[string]interface{}{
			"task":     task,
			"agent_id": agentID,
		},
		Timestamp: time.Now(),
	}

	select {
	case s.notifications <- n:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("notification queue full")
	}
}

func (s *notificationService) NotifyTaskCompleted(ctx context.Context, agentID string, task interface{}) error {
	n := &notification{
		ID:     uuid.New(),
		Type:   "task.completed",
		Target: agentID,
		Message: map[string]interface{}{
			"task":     task,
			"agent_id": agentID,
		},
		Timestamp: time.Now(),
	}

	select {
	case s.notifications <- n:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("notification queue full")
	}
}

func (s *notificationService) NotifyTaskFailed(ctx context.Context, taskID uuid.UUID, agentID string, reason string) error {
	n := &notification{
		ID:     uuid.New(),
		Type:   "task.failed",
		Target: agentID,
		Message: map[string]interface{}{
			"task_id":  taskID.String(),
			"agent_id": agentID,
			"reason":   reason,
		},
		Timestamp: time.Now(),
	}

	select {
	case s.notifications <- n:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("notification queue full")
	}
}

func (s *notificationService) NotifyWorkflowStarted(ctx context.Context, workflow interface{}) error {
	n := &notification{
		ID:     uuid.New(),
		Type:   "workflow.started",
		Target: "broadcast",
		Message: map[string]interface{}{
			"workflow": workflow,
		},
		Timestamp: time.Now(),
	}

	select {
	case s.notifications <- n:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("notification queue full")
	}
}

func (s *notificationService) NotifyWorkflowCompleted(ctx context.Context, workflow interface{}) error {
	n := &notification{
		ID:     uuid.New(),
		Type:   "workflow.completed",
		Target: "broadcast",
		Message: map[string]interface{}{
			"workflow": workflow,
		},
		Timestamp: time.Now(),
	}

	select {
	case s.notifications <- n:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("notification queue full")
	}
}

func (s *notificationService) NotifyWorkflowFailed(ctx context.Context, workflowID uuid.UUID, reason string) error {
	n := &notification{
		ID:     uuid.New(),
		Type:   "workflow.failed",
		Target: "broadcast",
		Message: map[string]interface{}{
			"workflow_id": workflowID.String(),
			"reason":      reason,
		},
		Timestamp: time.Now(),
	}

	select {
	case s.notifications <- n:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("notification queue full")
	}
}

func (s *notificationService) BroadcastToAgents(ctx context.Context, agentIDs []string, message interface{}) error {
	for _, agentID := range agentIDs {
		n := &notification{
			ID:        uuid.New(),
			Type:      "broadcast",
			Target:    agentID,
			Message:   message,
			Timestamp: time.Now(),
		}

		select {
		case s.notifications <- n:
			// Continue to next agent
		case <-time.After(1 * time.Second):
			s.config.Logger.Warn("Failed to queue notification for agent", map[string]interface{}{
				"agent_id": agentID,
			})
		}
	}

	return nil
}

// NotifyStepStarted notifies when a workflow step starts
func (s *notificationService) NotifyStepStarted(ctx context.Context, executionID uuid.UUID, stepID string) error {
	n := &notification{
		ID:     uuid.New(),
		Type:   "step.started",
		Target: "broadcast",
		Message: map[string]interface{}{
			"execution_id": executionID.String(),
			"step_id":      stepID,
		},
		Timestamp: time.Now(),
	}

	select {
	case s.notifications <- n:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("notification queue full")
	}
}

// NotifyStepCompleted notifies when a workflow step completes
func (s *notificationService) NotifyStepCompleted(ctx context.Context, executionID uuid.UUID, stepID string, output interface{}) error {
	n := &notification{
		ID:     uuid.New(),
		Type:   "step.completed",
		Target: "broadcast",
		Message: map[string]interface{}{
			"execution_id": executionID.String(),
			"step_id":      stepID,
			"output":       output,
		},
		Timestamp: time.Now(),
	}

	select {
	case s.notifications <- n:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("notification queue full")
	}
}

// BroadcastToWorkspace broadcasts a message to all members of a workspace
func (s *notificationService) BroadcastToWorkspace(ctx context.Context, workspaceID uuid.UUID, message interface{}) error {
	n := &notification{
		ID:        uuid.New(),
		Type:      "workspace.broadcast",
		Target:    workspaceID.String(),
		Message:   message,
		Timestamp: time.Now(),
	}

	select {
	case s.notifications <- n:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("notification queue full")
	}
}

// NotifyWorkflowUpdated notifies when a workflow is updated
func (s *notificationService) NotifyWorkflowUpdated(ctx context.Context, workflow interface{}) error {
	n := &notification{
		ID:        uuid.New(),
		Type:      "workflow.updated",
		Target:    "broadcast",
		Message:   workflow,
		Timestamp: time.Now(),
	}

	select {
	case s.notifications <- n:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("notification queue full")
	}
}

// NotifyResourceDeleted notifies when a resource is deleted
func (s *notificationService) NotifyResourceDeleted(ctx context.Context, resourceType string, resourceID uuid.UUID) error {
	n := &notification{
		ID:     uuid.New(),
		Type:   fmt.Sprintf("%s.deleted", resourceType),
		Target: "broadcast",
		Message: map[string]interface{}{
			"resource_type": resourceType,
			"resource_id":   resourceID.String(),
		},
		Timestamp: time.Now(),
	}

	select {
	case s.notifications <- n:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("notification queue full")
	}
}
