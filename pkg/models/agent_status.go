package models

import (
	"fmt"
	"time"
)

// AgentStatus represents the operational state of an agent
type AgentStatus string

const (
	AgentStatusActive      AgentStatus = "active"
	AgentStatusInactive    AgentStatus = "inactive"
	AgentStatusMaintenance AgentStatus = "maintenance"
	AgentStatusDraining    AgentStatus = "draining"
	AgentStatusError       AgentStatus = "error"
	AgentStatusOffline     AgentStatus = "offline"
	AgentStatusStarting    AgentStatus = "starting"
	AgentStatusStopping    AgentStatus = "stopping"
)

// Valid state transitions
var validAgentTransitions = map[AgentStatus][]AgentStatus{
	AgentStatusOffline:     {AgentStatusStarting},
	AgentStatusStarting:    {AgentStatusActive, AgentStatusError},
	AgentStatusActive:      {AgentStatusDraining, AgentStatusMaintenance, AgentStatusError, AgentStatusStopping},
	AgentStatusDraining:    {AgentStatusInactive, AgentStatusError},
	AgentStatusInactive:    {AgentStatusActive, AgentStatusMaintenance, AgentStatusStopping},
	AgentStatusMaintenance: {AgentStatusActive, AgentStatusInactive, AgentStatusStopping},
	AgentStatusError:       {AgentStatusStopping, AgentStatusMaintenance},
	AgentStatusStopping:    {AgentStatusOffline},
}

// CanTransitionTo checks if a status transition is valid
func (s AgentStatus) CanTransitionTo(target AgentStatus) bool {
	validTargets, exists := validAgentTransitions[s]
	if !exists {
		return false
	}
	for _, valid := range validTargets {
		if valid == target {
			return true
		}
	}
	return false
}

// TransitionTo performs a validated state transition with metrics
func (s AgentStatus) TransitionTo(target AgentStatus, metrics MetricsClient) (AgentStatus, error) {
	if !s.CanTransitionTo(target) {
		metrics.IncrementCounter("agent.status.transition.invalid", 1, map[string]string{
			"from": string(s),
			"to":   string(target),
		})
		return s, fmt.Errorf("invalid transition from %s to %s", s, target)
	}

	metrics.IncrementCounter("agent.status.transition.success", 1, map[string]string{
		"from": string(s),
		"to":   string(target),
	})

	return target, nil
}

// Validate ensures the status is valid
func (s AgentStatus) Validate() error {
	switch s {
	case AgentStatusActive, AgentStatusInactive, AgentStatusMaintenance,
		AgentStatusDraining, AgentStatusError, AgentStatusOffline,
		AgentStatusStarting, AgentStatusStopping:
		return nil
	default:
		return fmt.Errorf("invalid agent status: %s", s)
	}
}

// AgentCapability defines what an agent can do
type AgentCapability string

const (
	AgentCapabilityCompute     AgentCapability = "compute"
	AgentCapabilityStorage     AgentCapability = "storage"
	AgentCapabilityNetwork     AgentCapability = "network"
	AgentCapabilityOrchestrate AgentCapability = "orchestrate"
	AgentCapabilityAnalyze     AgentCapability = "analyze"
	AgentCapabilitySecure      AgentCapability = "secure"
	AgentCapabilitySpecialized AgentCapability = "specialized"
)

// AgentMetrics tracks real-time agent performance
type AgentMetrics struct {
	CPUUsage      float64   `json:"cpu_usage"`    // 0-100
	MemoryUsage   float64   `json:"memory_usage"` // 0-100
	DiskUsage     float64   `json:"disk_usage"`   // 0-100
	NetworkIO     float64   `json:"network_io"`   // bytes/sec
	TasksActive   int       `json:"tasks_active"`
	TasksQueued   int       `json:"tasks_queued"`
	TasksComplete int64     `json:"tasks_complete"`
	ErrorRate     float64   `json:"error_rate"`    // errors per minute
	ResponseTime  float64   `json:"response_time"` // milliseconds
	LastUpdated   time.Time `json:"last_updated"`
}

// AgentHealth represents health check results
type AgentHealth struct {
	Status    HealthStatus           `json:"status"`
	LastCheck time.Time              `json:"last_check"`
	NextCheck time.Time              `json:"next_check"`
	Checks    map[string]CheckResult `json:"checks"`
	Message   string                 `json:"message,omitempty"`
}

type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

type CheckResult struct {
	Status      HealthStatus  `json:"status"`
	Message     string        `json:"message"`
	LastSuccess time.Time     `json:"last_success"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
}

// WorkloadInfo contains agent workload information
type WorkloadInfo struct {
	AgentID       string    `json:"agent_id"`
	ActiveTasks   int       `json:"active_tasks"`
	QueuedTasks   int       `json:"queued_tasks"`
	CPUUsage      float64   `json:"cpu_usage"`
	MemoryUsage   float64   `json:"memory_usage"`
	WorkloadScore float64   `json:"workload_score"` // Calculated score 0-100
	LastUpdated   time.Time `json:"last_updated"`
}

// Calculate computes the overall workload score
func (w *WorkloadInfo) Calculate() {
	// Weighted calculation: tasks have more weight than resource usage
	taskScore := float64(w.ActiveTasks*2+w.QueuedTasks) * 10 // Normalize to 0-100
	if taskScore > 100 {
		taskScore = 100
	}

	resourceScore := (w.CPUUsage + w.MemoryUsage) / 2

	// 60% weight on tasks, 40% on resources
	w.WorkloadScore = (taskScore * 0.6) + (resourceScore * 0.4)
}

// MetricsClient interface for metrics collection
type MetricsClient interface {
	IncrementCounter(name string, value float64, tags map[string]string)
	RecordGauge(name string, value float64, tags map[string]string)
	RecordHistogram(name string, value float64, tags map[string]string)
	RecordTiming(name string, duration time.Duration, tags map[string]string)
}
