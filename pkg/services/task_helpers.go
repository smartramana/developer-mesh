package services

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/rules"
)

// ProgressTracker tracks task progress
type ProgressTracker struct {
	service *taskService
	tasks   sync.Map // map[uuid.UUID]time.Time
	ticker  *time.Ticker
	done    chan bool
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(service *taskService) *ProgressTracker {
	return &ProgressTracker{
		service: service,
		ticker:  time.NewTicker(30 * time.Second),
		done:    make(chan bool),
	}
}

// Start starts the progress tracker
func (t *ProgressTracker) Start() {
	for {
		select {
		case <-t.ticker.C:
			t.checkProgress()
		case <-t.done:
			t.ticker.Stop()
			return
		}
	}
}

// Stop stops the progress tracker
func (t *ProgressTracker) Stop() {
	close(t.done)
}

// Track adds a task to track
func (t *ProgressTracker) Track(taskID uuid.UUID) {
	t.tasks.Store(taskID, time.Now())
}

// Untrack removes a task from tracking
func (t *ProgressTracker) Untrack(taskID uuid.UUID) {
	t.tasks.Delete(taskID)
}

func (t *ProgressTracker) checkProgress() {
	// TODO: Implement progress checking logic
	t.tasks.Range(func(key, value interface{}) bool {
		taskID := key.(uuid.UUID)
		startTime := value.(time.Time)

		// Check if task is taking too long
		if time.Since(startTime) > 30*time.Minute {
			// Log warning
			t.service.config.Logger.Warn("Task taking too long", map[string]interface{}{
				"task_id": taskID,
				"elapsed": time.Since(startTime).String(),
			})
		}

		return true
	})
}

// TaskRebalancer handles task rebalancing
type TaskRebalancer struct {
	service    *taskService
	ruleEngine rules.Engine
	ticker     *time.Ticker
	done       chan bool
}

// NewTaskRebalancer creates a new task rebalancer
func NewTaskRebalancer(service *taskService, ruleEngine rules.Engine) *TaskRebalancer {
	return &TaskRebalancer{
		service:    service,
		ruleEngine: ruleEngine,
		ticker:     time.NewTicker(5 * time.Minute),
		done:       make(chan bool),
	}
}

// Start starts the rebalancer
func (r *TaskRebalancer) Start() {
	for {
		select {
		case <-r.ticker.C:
			r.rebalance()
		case <-r.done:
			r.ticker.Stop()
			return
		}
	}
}

// Stop stops the rebalancer
func (r *TaskRebalancer) Stop() {
	close(r.done)
}

func (r *TaskRebalancer) rebalance() {
	ctx := context.Background()
	if err := r.service.RebalanceTasks(ctx); err != nil {
		r.service.config.Logger.Error("Task rebalancing failed", map[string]interface{}{
			"error": err.Error(),
		})
	}
}

// ResultAggregator aggregates results from distributed tasks
type ResultAggregator struct {
	mu      sync.RWMutex
	results map[uuid.UUID]map[uuid.UUID]interface{} // parentTaskID -> subtaskID -> result
}

// NewResultAggregator creates a new result aggregator
func NewResultAggregator() *ResultAggregator {
	return &ResultAggregator{
		results: make(map[uuid.UUID]map[uuid.UUID]interface{}),
	}
}

// AddResult adds a subtask result
func (a *ResultAggregator) AddResult(parentTaskID, subtaskID uuid.UUID, result interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.results[parentTaskID]; !exists {
		a.results[parentTaskID] = make(map[uuid.UUID]interface{})
	}

	a.results[parentTaskID][subtaskID] = result
}

// GetResults gets all results for a parent task
func (a *ResultAggregator) GetResults(parentTaskID uuid.UUID) map[uuid.UUID]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	results, exists := a.results[parentTaskID]
	if !exists {
		return nil
	}

	// Return a copy
	copy := make(map[uuid.UUID]interface{})
	for k, v := range results {
		copy[k] = v
	}

	return copy
}

// AggregateResults aggregates results based on configuration
func (a *ResultAggregator) AggregateResults(parentTaskID uuid.UUID, config models.AggregationConfig) (interface{}, error) {
	results := a.GetResults(parentTaskID)

	switch config.Method {
	case "combine_results":
		return a.combineResults(results), nil
	case "first_complete":
		return a.firstComplete(results), nil
	case "majority_vote":
		return a.majorityVote(results), nil
	default:
		return results, nil
	}
}

func (a *ResultAggregator) combineResults(results map[uuid.UUID]interface{}) interface{} {
	combined := make([]interface{}, 0, len(results))
	for _, result := range results {
		combined = append(combined, result)
	}
	return combined
}

func (a *ResultAggregator) firstComplete(results map[uuid.UUID]interface{}) interface{} {
	for _, result := range results {
		return result // Return first result
	}
	return nil
}

func (a *ResultAggregator) majorityVote(results map[uuid.UUID]interface{}) interface{} {
	// TODO: Implement majority vote logic
	return a.combineResults(results)
}

// Clear clears results for a parent task
func (a *ResultAggregator) Clear(parentTaskID uuid.UUID) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.results, parentTaskID)
}

// TaskCreationSaga handles distributed task creation
type TaskCreationSaga struct {
	service *taskService
	dt      *models.DistributedTask
	events  []interface{}
}

// NewTaskCreationSaga creates a new task creation saga
func NewTaskCreationSaga(service *taskService, dt *models.DistributedTask) *TaskCreationSaga {
	return &TaskCreationSaga{
		service: service,
		dt:      dt,
		events:  make([]interface{}, 0),
	}
}

// CreateMainTask creates the main task
func (s *TaskCreationSaga) CreateMainTask(ctx context.Context, tx Transaction) (*models.Task, error) {
	mainTask := &models.Task{
		Type:        s.dt.Type,
		Title:       s.dt.Title,
		Description: s.dt.Description,
		Priority:    s.dt.Priority,
		Parameters: models.JSONMap{
			"distributed":     true,
			"aggregation":     s.dt.Aggregation,
			"subtask_count":   len(s.dt.Subtasks),
			"parent_task_id":  nil,
			"delegation_type": "distributed",
		},
	}

	// Create task (implementation will handle ID generation, etc.)
	if err := s.service.Create(ctx, mainTask, ""); err != nil {
		return nil, err
	}

	s.events = append(s.events, models.TaskCreatedEvent{Task: mainTask})

	return mainTask, nil
}

// DeleteMainTask deletes the main task (compensation)
func (s *TaskCreationSaga) DeleteMainTask(ctx context.Context, taskID uuid.UUID) error {
	return s.service.Delete(ctx, taskID)
}

// ValidateAgents validates agent availability
func (s *TaskCreationSaga) ValidateAgents(ctx context.Context, subtasks []models.Subtask) (map[string]*models.Agent, error) {
	agentMap := make(map[string]*models.Agent)

	for _, subtask := range subtasks {
		if subtask.AgentID != "" {
			agent, err := s.service.agentService.GetAgent(ctx, subtask.AgentID)
			if err != nil {
				return nil, err
			}

			if agent.Status != "available" {
				return nil, ErrNoEligibleAgents
			}

			agentMap[subtask.AgentID] = agent
		}
	}

	return agentMap, nil
}

// CreateSubtask creates a subtask
func (s *TaskCreationSaga) CreateSubtask(ctx context.Context, tx Transaction, mainTask *models.Task, subtaskDef models.Subtask, agent *models.Agent) (*models.Task, error) {
	subtask := &models.Task{
		Type:         mainTask.Type,
		Title:        subtaskDef.Description,
		Description:  subtaskDef.Description,
		Priority:     mainTask.Priority,
		AssignedTo:   &subtaskDef.AgentID,
		ParentTaskID: &mainTask.ID,
		Parameters:   subtaskDef.Parameters,
	}

	if err := s.service.Create(ctx, subtask, ""); err != nil {
		return nil, err
	}

	s.events = append(s.events, models.SubtaskCreatedEvent{
		ParentTask: mainTask,
		Subtask:    subtask,
	})

	return subtask, nil
}

// DeleteSubtask deletes a subtask (compensation)
func (s *TaskCreationSaga) DeleteSubtask(ctx context.Context, subtaskID uuid.UUID) error {
	return s.service.Delete(ctx, subtaskID)
}

// PublishEvents publishes all saga events
func (s *TaskCreationSaga) PublishEvents(ctx context.Context) error {
	// TODO: Publish events through event bus
	for _, event := range s.events {
		s.service.config.Logger.Info("Saga event", map[string]interface{}{
			"event": event,
		})
	}
	return nil
}
