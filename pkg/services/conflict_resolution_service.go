package services

import (
	"context"
	"fmt"
	"sync"
	"time"
	
	"github.com/google/uuid"
	"github.com/pkg/errors"
	
	"github.com/S-Corkum/devops-mcp/pkg/collaboration"
	"github.com/S-Corkum/devops-mcp/pkg/collaboration/crdt"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository/interfaces"
)

// ConflictResolutionService manages conflict resolution for collaborative entities
type ConflictResolutionService interface {
	// Document conflict resolution
	ResolveDocumentConflict(ctx context.Context, documentID uuid.UUID, operations []collaboration.DocumentOperation) (*models.Document, error)
	GetDocumentCRDT(ctx context.Context, documentID uuid.UUID) (*collaboration.DocumentCRDT, error)
	ApplyDocumentOperation(ctx context.Context, documentID uuid.UUID, operation *collaboration.CRDTOperation) error
	SyncDocument(ctx context.Context, documentID uuid.UUID, remoteCRDT *collaboration.DocumentCRDT) error
	
	// Workspace state conflict resolution
	ResolveWorkspaceStateConflict(ctx context.Context, workspaceID uuid.UUID, operations []models.StateOperation) (*models.WorkspaceState, error)
	GetWorkspaceStateCRDT(ctx context.Context, workspaceID uuid.UUID) (*collaboration.StateCRDT, error)
	ApplyStateOperation(ctx context.Context, workspaceID uuid.UUID, operation *collaboration.StateOperation) error
	SyncWorkspaceState(ctx context.Context, workspaceID uuid.UUID, remoteState *collaboration.StateCRDT) error
	
	// Task conflict resolution
	ResolveTaskConflict(ctx context.Context, taskID uuid.UUID, conflicts []models.TaskConflict) (*models.Task, error)
	
	// General conflict detection
	DetectConflicts(ctx context.Context, entityType string, entityID uuid.UUID) ([]*models.ConflictInfo, error)
	GetConflictHistory(ctx context.Context, entityType string, entityID uuid.UUID) ([]*models.ConflictResolution, error)
	
	// Vector clock management
	GetVectorClock(ctx context.Context, nodeID string) (crdt.VectorClock, error)
	UpdateVectorClock(ctx context.Context, nodeID string, clock crdt.VectorClock) error
}

type conflictResolutionService struct {
	BaseService
	
	// Repositories
	documentRepo  interfaces.DocumentRepository
	workspaceRepo interfaces.WorkspaceRepository
	taskRepo      interfaces.TaskRepository
	
	// CRDT storage
	documentCRDTs  sync.Map // documentID -> *collaboration.DocumentCRDT
	stateCRDTs     sync.Map // workspaceID -> *collaboration.StateCRDT
	
	// Vector clock storage
	vectorClocks   sync.Map // nodeID -> crdt.VectorClock
	
	// Node identification
	nodeID         crdt.NodeID
}

// NewConflictResolutionService creates a new conflict resolution service
func NewConflictResolutionService(
	config ServiceConfig,
	documentRepo interfaces.DocumentRepository,
	workspaceRepo interfaces.WorkspaceRepository,
	taskRepo interfaces.TaskRepository,
	nodeID string,
) ConflictResolutionService {
	return &conflictResolutionService{
		BaseService:   NewBaseService(config),
		documentRepo:  documentRepo,
		workspaceRepo: workspaceRepo,
		taskRepo:      taskRepo,
		nodeID:        crdt.NodeID(nodeID),
	}
}

// Document conflict resolution

func (s *conflictResolutionService) ResolveDocumentConflict(ctx context.Context, documentID uuid.UUID, operations []collaboration.DocumentOperation) (*models.Document, error) {
	ctx, span := s.config.Tracer(ctx, "ConflictResolutionService.ResolveDocumentConflict")
	defer span.End()
	
	// Get or create document CRDT
	docCRDT, err := s.GetDocumentCRDT(ctx, documentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get document CRDT")
	}
	
	// Apply operations using CRDT logic
	for _, op := range operations {
		crdtOp := s.convertToCRDTOperation(op)
		if err := docCRDT.ApplyOperation(crdtOp); err != nil {
			s.config.Logger.Warn("Failed to apply operation", map[string]interface{}{
				"document_id": documentID,
				"operation_id": op.ID,
				"error": err.Error(),
			})
			// Continue with other operations
		}
	}
	
	// Get resolved content
	content := docCRDT.GetContent()
	
	// Update document in repository
	doc, err := s.documentRepo.Get(ctx, documentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get document")
	}
	
	doc.Content = content
	doc.Version++
	doc.UpdatedAt = time.Now()
	
	if err := s.documentRepo.Update(ctx, doc); err != nil {
		return nil, errors.Wrap(err, "failed to update document")
	}
	
	// Publish resolution event
	// Log resolution event
	s.config.Logger.Info("Document conflict resolved", map[string]interface{}{
		"document_id": documentID,
		"version": doc.Version,
		"operations_count": len(operations),
	})
	
	// Convert SharedDocument to Document for return
	result := &models.Document{
		ID:          doc.ID,
		TenantID:    doc.TenantID,
		WorkspaceID: &doc.WorkspaceID,
		Title:       doc.Title,
		Type:        doc.Type,
		Content:     doc.Content,
		ContentType: doc.ContentType,
		CreatedBy:   doc.CreatedBy,
		Version:     int(doc.Version),
		Permissions: doc.Metadata, // Use metadata as permissions not available
		Metadata:    doc.Metadata,
		CreatedAt:   doc.CreatedAt,
		UpdatedAt:   doc.UpdatedAt,
		DeletedAt:   doc.DeletedAt,
	}
	return result, nil
}

func (s *conflictResolutionService) GetDocumentCRDT(ctx context.Context, documentID uuid.UUID) (*collaboration.DocumentCRDT, error) {
	// Check cache
	if val, ok := s.documentCRDTs.Load(documentID); ok {
		return val.(*collaboration.DocumentCRDT), nil
	}
	
	// Create new CRDT
	docCRDT := collaboration.NewDocumentCRDT(documentID, s.nodeID)
	
	// Load existing operations from repository if available
	doc, err := s.documentRepo.Get(ctx, documentID)
	if err == nil && doc != nil {
		// Initialize CRDT with current content
		// This is a simplified approach - in production, you'd load historical operations
		if doc.Content != "" {
			op, _ := docCRDT.Insert(0, doc.Content)
			if op != nil {
				// Store the initial operation
			}
		}
	}
	
	s.documentCRDTs.Store(documentID, docCRDT)
	return docCRDT, nil
}

func (s *conflictResolutionService) ApplyDocumentOperation(ctx context.Context, documentID uuid.UUID, operation *collaboration.CRDTOperation) error {
	docCRDT, err := s.GetDocumentCRDT(ctx, documentID)
	if err != nil {
		return errors.Wrap(err, "failed to get document CRDT")
	}
	
	if err := docCRDT.ApplyOperation(operation); err != nil {
		return errors.Wrap(err, "failed to apply operation")
	}
	
	// Persist the updated state
	content := docCRDT.GetContent()
	doc, err := s.documentRepo.Get(ctx, documentID)
	if err != nil {
		return errors.Wrap(err, "failed to get document")
	}
	
	doc.Content = content
	doc.UpdatedAt = time.Now()
	
	return s.documentRepo.Update(ctx, doc)
}

func (s *conflictResolutionService) SyncDocument(ctx context.Context, documentID uuid.UUID, remoteCRDT *collaboration.DocumentCRDT) error {
	localCRDT, err := s.GetDocumentCRDT(ctx, documentID)
	if err != nil {
		return errors.Wrap(err, "failed to get local document CRDT")
	}
	
	// Merge remote CRDT into local
	if err := localCRDT.Merge(remoteCRDT); err != nil {
		return errors.Wrap(err, "failed to merge CRDTs")
	}
	
	// Update document with merged content
	content := localCRDT.GetContent()
	doc, err := s.documentRepo.Get(ctx, documentID)
	if err != nil {
		return errors.Wrap(err, "failed to get document")
	}
	
	doc.Content = content
	doc.UpdatedAt = time.Now()
	
	return s.documentRepo.Update(ctx, doc)
}

// Workspace state conflict resolution

func (s *conflictResolutionService) ResolveWorkspaceStateConflict(ctx context.Context, workspaceID uuid.UUID, operations []models.StateOperation) (*models.WorkspaceState, error) {
	ctx, span := s.config.Tracer(ctx, "ConflictResolutionService.ResolveWorkspaceStateConflict")
	defer span.End()
	
	// Get or create state CRDT
	stateCRDT, err := s.GetWorkspaceStateCRDT(ctx, workspaceID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get state CRDT")
	}
	
	// Apply operations
	for _, op := range operations {
		stateOp := s.convertToStateOperation(op)
		if err := stateCRDT.ApplyOperation(&stateOp); err != nil {
			s.config.Logger.Warn("Failed to apply state operation", map[string]interface{}{
				"workspace_id": workspaceID,
				"operation_type": op.Type,
				"error": err.Error(),
			})
		}
	}
	
	// Get resolved state
	stateMap := stateCRDT.GetState()
	
	// Update workspace state
	workspace, err := s.workspaceRepo.Get(ctx, workspaceID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get workspace")
	}
	
	workspace.State = models.JSONMap(stateMap)
	workspace.UpdatedAt = time.Now()
	
	if err := s.workspaceRepo.Update(ctx, workspace); err != nil {
		return nil, errors.Wrap(err, "failed to update workspace")
	}
	
	// Create WorkspaceState object
	state := &models.WorkspaceState{
		WorkspaceID: workspaceID,
		Data:        stateMap,
	}
	
	return state, nil
}

func (s *conflictResolutionService) GetWorkspaceStateCRDT(ctx context.Context, workspaceID uuid.UUID) (*collaboration.StateCRDT, error) {
	// Check cache
	if val, ok := s.stateCRDTs.Load(workspaceID); ok {
		return val.(*collaboration.StateCRDT), nil
	}
	
	// Create new CRDT
	stateCRDT := collaboration.NewStateCRDT(workspaceID, s.nodeID)
	
	// Load existing state from repository
	workspace, err := s.workspaceRepo.Get(ctx, workspaceID)
	if err == nil && workspace != nil && workspace.State != nil {
		// Initialize CRDT with current state
		for key, value := range workspace.State {
			stateCRDT.Set(key, value)
		}
	}
	
	s.stateCRDTs.Store(workspaceID, stateCRDT)
	return stateCRDT, nil
}

func (s *conflictResolutionService) ApplyStateOperation(ctx context.Context, workspaceID uuid.UUID, operation *collaboration.StateOperation) error {
	stateCRDT, err := s.GetWorkspaceStateCRDT(ctx, workspaceID)
	if err != nil {
		return errors.Wrap(err, "failed to get state CRDT")
	}
	
	if err := stateCRDT.ApplyOperation(operation); err != nil {
		return errors.Wrap(err, "failed to apply operation")
	}
	
	// Persist the updated state
	stateMap := stateCRDT.GetState()
	workspace, err := s.workspaceRepo.Get(ctx, workspaceID)
	if err != nil {
		return errors.Wrap(err, "failed to get workspace")
	}
	
	workspace.State = models.JSONMap(stateMap)
	workspace.UpdatedAt = time.Now()
	
	return s.workspaceRepo.Update(ctx, workspace)
}

func (s *conflictResolutionService) SyncWorkspaceState(ctx context.Context, workspaceID uuid.UUID, remoteState *collaboration.StateCRDT) error {
	localState, err := s.GetWorkspaceStateCRDT(ctx, workspaceID)
	if err != nil {
		return errors.Wrap(err, "failed to get local state CRDT")
	}
	
	// Merge remote state into local
	if err := localState.Merge(remoteState); err != nil {
		return errors.Wrap(err, "failed to merge state CRDTs")
	}
	
	// Update workspace with merged state
	stateMap := localState.GetState()
	workspace, err := s.workspaceRepo.Get(ctx, workspaceID)
	if err != nil {
		return errors.Wrap(err, "failed to get workspace")
	}
	
	workspace.State = models.JSONMap(stateMap)
	workspace.UpdatedAt = time.Now()
	
	return s.workspaceRepo.Update(ctx, workspace)
}

// Task conflict resolution

func (s *conflictResolutionService) ResolveTaskConflict(ctx context.Context, taskID uuid.UUID, conflicts []models.TaskConflict) (*models.Task, error) {
	ctx, span := s.config.Tracer(ctx, "ConflictResolutionService.ResolveTaskConflict")
	defer span.End()
	
	// Get current task
	task, err := s.taskRepo.Get(ctx, taskID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get task")
	}
	
	// Apply conflict resolution rules
	for _, conflict := range conflicts {
		switch conflict.Type {
		case "status":
			// Priority: completed > failed > in_progress > assigned > pending
			task.Status = s.resolveTaskStatus(task.Status, conflict.Values)
			
		case "assignment":
			// Last assignment wins
			if len(conflict.Values) > 0 {
				if assignment, ok := conflict.Values[len(conflict.Values)-1].(string); ok {
					task.AssignedTo = &assignment
				}
			}
			
		case "priority":
			// Highest priority wins
			task.Priority = s.resolveTaskPriority(task.Priority, conflict.Values)
			
		default:
			s.config.Logger.Warn("Unknown conflict type", map[string]interface{}{
				"task_id": taskID,
				"conflict_type": conflict.Type,
			})
		}
	}
	
	// Update task
	task.UpdatedAt = time.Now()
	task.Version++
	
	if err := s.taskRepo.Update(ctx, task); err != nil {
		return nil, errors.Wrap(err, "failed to update task")
	}
	
	// Record resolution
	s.recordConflictResolution(ctx, "task", taskID, conflicts)
	
	return task, nil
}

// General conflict detection

func (s *conflictResolutionService) DetectConflicts(ctx context.Context, entityType string, entityID uuid.UUID) ([]*models.ConflictInfo, error) {
	// This would analyze version history and concurrent modifications
	// For now, return empty as this requires version tracking implementation
	return []*models.ConflictInfo{}, nil
}

func (s *conflictResolutionService) GetConflictHistory(ctx context.Context, entityType string, entityID uuid.UUID) ([]*models.ConflictResolution, error) {
	// This would retrieve historical conflict resolutions
	// For now, return empty as this requires persistence
	return []*models.ConflictResolution{}, nil
}

// Vector clock management

func (s *conflictResolutionService) GetVectorClock(ctx context.Context, nodeID string) (crdt.VectorClock, error) {
	if val, ok := s.vectorClocks.Load(nodeID); ok {
		return val.(crdt.VectorClock), nil
	}
	
	// Create new vector clock
	clock := crdt.NewVectorClock()
	s.vectorClocks.Store(nodeID, clock)
	return clock, nil
}

func (s *conflictResolutionService) UpdateVectorClock(ctx context.Context, nodeID string, clock crdt.VectorClock) error {
	s.vectorClocks.Store(nodeID, clock)
	return nil
}

// Helper methods

func (s *conflictResolutionService) convertToCRDTOperation(op collaboration.DocumentOperation) *collaboration.CRDTOperation {
	return &collaboration.CRDTOperation{
		ID:        op.ID,
		Type:      op.Type,
		Position:  0, // Would need to extract from path
		Content:   fmt.Sprintf("%v", op.Value),
		NodeID:    crdt.NodeID(op.AgentID),
		Clock:     s.convertVectorClock(op.VectorClock),
		Timestamp: op.AppliedAt,
	}
}

func (s *conflictResolutionService) convertToStateOperation(op models.StateOperation) collaboration.StateOperation {
	return collaboration.StateOperation{
		ID:        uuid.New(),
		Type:      op.Type,
		Path:      op.Path,
		Value:     op.Value,
		NodeID:    s.nodeID, // Use service's node ID
		Clock:     crdt.NewVectorClock(), // Would need proper clock
		Timestamp: time.Now(),
	}
}

func (s *conflictResolutionService) convertVectorClock(clock map[string]int) crdt.VectorClock {
	vc := crdt.NewVectorClock()
	for nodeID, value := range clock {
		vc[crdt.NodeID(nodeID)] = uint64(value)
	}
	return vc
}

func (s *conflictResolutionService) resolveTaskStatus(current models.TaskStatus, conflictValues []interface{}) models.TaskStatus {
	// Define priority order
	statusPriority := map[models.TaskStatus]int{
		models.TaskStatusCompleted:  5,
		models.TaskStatusFailed:     4,
		models.TaskStatusInProgress: 3,
		models.TaskStatusAssigned:   2,
		models.TaskStatusPending:    1,
	}
	
	highestPriority := statusPriority[current]
	result := current
	
	for _, val := range conflictValues {
		if status, ok := val.(models.TaskStatus); ok {
			if priority, exists := statusPriority[status]; exists && priority > highestPriority {
				highestPriority = priority
				result = status
			}
		}
	}
	
	return result
}

func (s *conflictResolutionService) resolveTaskPriority(current models.TaskPriority, conflictValues []interface{}) models.TaskPriority {
	// Define priority order
	priorityOrder := map[models.TaskPriority]int{
		models.TaskPriorityCritical: 4,
		models.TaskPriorityHigh:     3,
		models.TaskPriorityNormal:   2,
		models.TaskPriorityLow:      1,
	}
	
	highestPriority := priorityOrder[current]
	result := current
	
	for _, val := range conflictValues {
		if priority, ok := val.(models.TaskPriority); ok {
			if order, exists := priorityOrder[priority]; exists && order > highestPriority {
				highestPriority = order
				result = priority
			}
		}
	}
	
	return result
}

func (s *conflictResolutionService) recordConflictResolution(ctx context.Context, entityType string, entityID uuid.UUID, conflicts []models.TaskConflict) {
	// Record the resolution for audit
	// Log the resolution for audit
	s.config.Logger.Info("Conflict resolved", map[string]interface{}{
		"entity_type": entityType,
		"entity_id": entityID,
		"conflicts_count": len(conflicts),
		"resolved_at": time.Now(),
	})
}