package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/rules"
)

// Define context key types to avoid collisions
type contextKey string

const (
	systemOperationKey contextKey = "system_operation"
	operationTypeKey   contextKey = "operation_type"
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
	// Check progress of all tracked tasks
	ctx := context.Background()
	tasksToCheck := make(map[uuid.UUID]time.Time)

	// Collect all tasks to check
	t.tasks.Range(func(key, value interface{}) bool {
		taskID := key.(uuid.UUID)
		startTime := value.(time.Time)
		tasksToCheck[taskID] = startTime
		return true
	})

	// Check each task
	for taskID, startTime := range tasksToCheck {
		elapsed := time.Since(startTime)

		// Fetch current task status
		task, err := t.service.Get(ctx, taskID)
		if err != nil {
			t.service.config.Logger.Error("Failed to fetch task for progress check", map[string]interface{}{
				"task_id": taskID,
				"error":   err.Error(),
			})
			continue
		}

		// Skip if task is already completed or failed
		if task.Status == models.TaskStatusCompleted || task.Status == models.TaskStatusFailed {
			t.Untrack(taskID)
			continue
		}

		// Calculate progress percentage based on elapsed time vs timeout
		var progressPercent float64
		if task.TimeoutSeconds > 0 {
			progressPercent = float64(elapsed.Seconds()) / float64(task.TimeoutSeconds) * 100
			if progressPercent > 100 {
				progressPercent = 100
			}
		}

		// Update progress for in-progress tasks
		if task.Status == models.TaskStatusInProgress {
			// Extract current progress from Result field
			var oldProgress int
			if task.Result != nil {
				if progressVal, ok := task.Result["progress"].(float64); ok {
					oldProgress = int(progressVal)
				}
			}

			newProgress := int(progressPercent)

			// Only update if progress has changed significantly (5% threshold)
			if newProgress-oldProgress >= 5 {
				progressMessage := fmt.Sprintf("Task running for %s", elapsed.Round(time.Second).String())
				if err := t.service.UpdateProgress(ctx, taskID, newProgress, progressMessage); err != nil {
					t.service.config.Logger.Error("Failed to update task progress", map[string]interface{}{
						"task_id":      taskID,
						"old_progress": oldProgress,
						"new_progress": newProgress,
						"error":        err.Error(),
					})
				} else {
					t.service.config.Logger.Info("Task progress updated", map[string]interface{}{
						"task_id":  taskID,
						"progress": newProgress,
						"elapsed":  elapsed.String(),
					})
				}
			}
		}

		// Check warning thresholds
		warningThreshold := 30 * time.Minute
		criticalThreshold := 60 * time.Minute

		// Apply custom thresholds based on task timeout
		if task.TimeoutSeconds > 0 {
			taskTimeout := time.Duration(task.TimeoutSeconds) * time.Second
			warningThreshold = taskTimeout * 75 / 100  // 75% of timeout
			criticalThreshold = taskTimeout * 90 / 100 // 90% of timeout
		}

		// Emit metrics and logs based on thresholds
		if elapsed > criticalThreshold {
			t.service.config.Logger.Error("Task exceeded critical threshold", map[string]interface{}{
				"task_id":            taskID,
				"task_type":          task.Type,
				"elapsed":            elapsed.String(),
				"critical_threshold": criticalThreshold.String(),
				"status":             task.Status,
			})

			t.service.config.Metrics.IncrementCounterWithLabels("task.progress.critical", 1, map[string]string{
				"tenant_id": task.TenantID.String(),
				"task_type": task.Type,
			})

			// Consider triggering intervention or timeout
			if task.TimeoutSeconds > 0 && elapsed > time.Duration(task.TimeoutSeconds)*time.Second {
				// Task has exceeded its timeout - attempt to mark as failed
				timeoutError := fmt.Sprintf("Task timed out after %s (timeout: %ds)", elapsed.String(), task.TimeoutSeconds)
				// For timeout handling, we need to update the task status directly
				// since FailTask requires the task to be assigned to the calling agent
				task.Status = models.TaskStatusTimeout
				task.Error = timeoutError
				task.CompletedAt = timePtr(time.Now())

				if err := t.service.Update(ctx, task); err != nil {
					t.service.config.Logger.Error("Failed to mark task as timed out", map[string]interface{}{
						"task_id": taskID,
						"error":   err.Error(),
					})
				} else {
					t.service.config.Logger.Info("Task marked as timed out", map[string]interface{}{
						"task_id": taskID,
						"elapsed": elapsed.String(),
						"timeout": task.TimeoutSeconds,
					})

					// Record timeout metric
					t.service.config.Metrics.IncrementCounterWithLabels("task.timeout", 1, map[string]string{
						"tenant_id": task.TenantID.String(),
						"task_type": task.Type,
					})
				}
				t.Untrack(taskID)
			}

		} else if elapsed > warningThreshold {
			t.service.config.Logger.Warn("Task exceeded warning threshold", map[string]interface{}{
				"task_id":           taskID,
				"task_type":         task.Type,
				"elapsed":           elapsed.String(),
				"warning_threshold": warningThreshold.String(),
				"status":            task.Status,
				"progress":          progressPercent,
			})

			t.service.config.Metrics.IncrementCounterWithLabels("task.progress.warning", 1, map[string]string{
				"tenant_id": task.TenantID.String(),
				"task_type": task.Type,
			})
		}

		// Record progress metrics
		t.service.config.Metrics.RecordGauge("task.progress.percentage", progressPercent, map[string]string{
			"tenant_id": task.TenantID.String(),
			"task_type": task.Type,
			"task_id":   taskID.String(),
		})

		t.service.config.Metrics.RecordHistogram("task.progress.elapsed_seconds", elapsed.Seconds(), map[string]string{
			"tenant_id": task.TenantID.String(),
			"task_type": task.Type,
		})
	}
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
	// Create a system context for background operations
	// This bypasses authorization checks since it's an internal system operation
	ctx := context.WithValue(context.Background(), systemOperationKey, true)
	ctx = context.WithValue(ctx, operationTypeKey, "task_rebalancing")

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
	// Implement majority vote logic for consensus among multiple task results
	if len(results) == 0 {
		return nil
	}

	// For single result, return immediately
	if len(results) == 1 {
		for _, result := range results {
			return result
		}
	}

	// Count occurrences of each result
	votes := make(map[string]int)
	resultsByHash := make(map[string]interface{})

	for _, result := range results {
		// Create a deterministic hash for the result
		resultHash := a.hashResult(result)
		votes[resultHash]++
		resultsByHash[resultHash] = result
	}

	// Find the result with the most votes
	var winningHash string
	maxVotes := 0
	totalVotes := len(results)

	for hash, voteCount := range votes {
		if voteCount > maxVotes {
			maxVotes = voteCount
			winningHash = hash
		}
	}

	// Check if we have a clear majority (more than 50%)
	if maxVotes > totalVotes/2 {
		return resultsByHash[winningHash]
	}

	// No clear majority - check for plurality (most votes but not majority)
	// In case of tie, we need a deterministic tie-breaker
	if maxVotes > 1 {
		// Multiple results got the same highest vote count
		var tiedHashes []string
		for hash, voteCount := range votes {
			if voteCount == maxVotes {
				tiedHashes = append(tiedHashes, hash)
			}
		}

		// Sort hashes for deterministic selection
		sort.Strings(tiedHashes)

		// Return the first one (deterministic)
		return resultsByHash[tiedHashes[0]]
	}

	// All results are different - no consensus
	// Return a special result indicating no consensus
	return map[string]interface{}{
		"consensus": false,
		"reason":    "no majority agreement",
		"votes":     votes,
		"total":     totalVotes,
		"required":  totalVotes/2 + 1,
		"results":   results, // Include all results for analysis
	}
}

// hashResult creates a deterministic hash for a result to enable voting
func (a *ResultAggregator) hashResult(result interface{}) string {
	// Handle different result types
	switch v := result.(type) {
	case string:
		return fmt.Sprintf("string:%s", v)
	case int, int32, int64, float32, float64:
		return fmt.Sprintf("number:%v", v)
	case bool:
		return fmt.Sprintf("bool:%v", v)
	case map[string]interface{}:
		// For maps, create a sorted JSON representation
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var parts []string
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s:%v", k, v[k]))
		}
		return fmt.Sprintf("map:%s", strings.Join(parts, ","))
	case []interface{}:
		// For arrays, concatenate elements
		var parts []string
		for _, elem := range v {
			parts = append(parts, fmt.Sprintf("%v", elem))
		}
		return fmt.Sprintf("array:%s", strings.Join(parts, ","))
	default:
		// For complex types, use JSON encoding
		if data, err := json.Marshal(result); err == nil {
			return fmt.Sprintf("json:%s", string(data))
		}
		// Fallback to string representation
		return fmt.Sprintf("unknown:%v", result)
	}
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
	// Publish events through event bus if configured
	if s.service.eventPublisher == nil {
		// Event publisher not configured, log events instead
		for _, event := range s.events {
			s.service.config.Logger.Info("Saga event (no publisher configured)", map[string]interface{}{
				"event_type": fmt.Sprintf("%T", event),
				"event":      event,
			})
		}
		return nil
	}

	// Publish each event
	var publishErrors []error
	for _, event := range s.events {
		// Determine event type and aggregate info based on event type
		var eventType string
		var aggregate AggregateRoot

		switch e := event.(type) {
		case models.TaskCreatedEvent:
			eventType = "task.created"
			aggregate = &taskAggregate{
				id:      e.Task.ID,
				version: 1, // New task
			}
		case models.SubtaskCreatedEvent:
			eventType = "subtask.created"
			aggregate = &taskAggregate{
				id:      e.ParentTask.ID,
				version: 1, // Version would be tracked properly in production
			}
		default:
			// Unknown event type - log and skip
			s.service.config.Logger.Warn("Unknown saga event type", map[string]interface{}{
				"event_type": fmt.Sprintf("%T", event),
				"event":      event,
			})
			continue
		}

		// Publish the event
		if err := s.service.PublishEvent(ctx, eventType, aggregate, event); err != nil {
			publishErrors = append(publishErrors, err)
			s.service.config.Logger.Error("Failed to publish saga event", map[string]interface{}{
				"event_type": eventType,
				"error":      err.Error(),
			})

			// Record failure metric
			s.service.config.Metrics.IncrementCounterWithLabels("saga.event.publish.failed", 1, map[string]string{
				"event_type": eventType,
				"saga_type":  "task_creation",
			})
		} else {
			s.service.config.Logger.Info("Saga event published", map[string]interface{}{
				"event_type": eventType,
			})

			// Record success metric
			s.service.config.Metrics.IncrementCounterWithLabels("saga.event.publish.success", 1, map[string]string{
				"event_type": eventType,
				"saga_type":  "task_creation",
			})
		}
	}

	// Return error if any events failed to publish
	if len(publishErrors) > 0 {
		return fmt.Errorf("failed to publish %d events: %v", len(publishErrors), publishErrors)
	}

	return nil
}

// taskAggregate implements AggregateRoot for task events
type taskAggregate struct {
	id      uuid.UUID
	version int
}

func (t *taskAggregate) GetID() uuid.UUID {
	return t.id
}

func (t *taskAggregate) GetType() string {
	return "task"
}

func (t *taskAggregate) GetVersion() int {
	return t.version
}
