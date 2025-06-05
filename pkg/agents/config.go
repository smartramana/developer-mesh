package agents

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EmbeddingStrategy represents the strategy for selecting embedding models
type EmbeddingStrategy string

const (
	StrategyBalanced EmbeddingStrategy = "balanced" // Balance between cost and quality
	StrategyQuality  EmbeddingStrategy = "quality"  // Prioritize quality over cost
	StrategySpeed    EmbeddingStrategy = "speed"    // Prioritize speed/latency
	StrategyCost     EmbeddingStrategy = "cost"     // Minimize cost
)

// TaskType represents the type of task for embedding optimization
type TaskType string

const (
	TaskTypeCodeAnalysis TaskType = "code_analysis"
	TaskTypeGeneralQA    TaskType = "general_qa"
	TaskTypeMultilingual TaskType = "multilingual"
	TaskTypeResearch     TaskType = "research"
)

// AgentConfig represents the complete configuration for an AI agent
type AgentConfig struct {
	ID               uuid.UUID            `json:"id" db:"id"`
	AgentID          string               `json:"agent_id" db:"agent_id" validate:"required,min=3,max=255"`
	Version          int                  `json:"version" db:"version"`
	EmbeddingStrategy EmbeddingStrategy   `json:"embedding_strategy" db:"embedding_strategy" validate:"required,oneof=balanced quality speed cost"`
	ModelPreferences []ModelPreference    `json:"model_preferences" db:"model_preferences" validate:"required,dive"`
	Constraints      AgentConstraints     `json:"constraints" db:"constraints" validate:"required"`
	FallbackBehavior FallbackConfig       `json:"fallback_behavior" db:"fallback_behavior"`
	Metadata         map[string]interface{} `json:"metadata" db:"metadata"`
	IsActive         bool                 `json:"is_active" db:"is_active"`
	CreatedAt        time.Time            `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at" db:"updated_at"`
	CreatedBy        string               `json:"created_by" db:"created_by"`
}

// ModelPreference defines which models an agent prefers for specific tasks
type ModelPreference struct {
	TaskType       TaskType `json:"task_type" validate:"required,oneof=code_analysis general_qa multilingual research"`
	PrimaryModels  []string `json:"primary_models" validate:"required,min=1"`
	FallbackModels []string `json:"fallback_models"`
	Weight         float64  `json:"weight" validate:"min=0,max=1"` // For weighted selection
}

// AgentConstraints defines operational constraints for an agent
type AgentConstraints struct {
	MaxCostPerMonthUSD  float64          `json:"max_cost_per_month_usd" validate:"min=0"`
	MaxLatencyP99Ms     int              `json:"max_latency_p99_ms" validate:"min=0"`
	MinAvailabilitySLA  float64          `json:"min_availability_sla" validate:"min=0,max=1"`
	RateLimits          RateLimitConfig  `json:"rate_limits"`
	QualityThresholds   QualityConfig    `json:"quality_thresholds"`
}

// RateLimitConfig defines rate limiting constraints
type RateLimitConfig struct {
	RequestsPerMinute int `json:"requests_per_minute" validate:"min=0"`
	TokensPerHour     int `json:"tokens_per_hour" validate:"min=0"`
	ConcurrentRequests int `json:"concurrent_requests" validate:"min=0"`
}

// QualityConfig defines quality thresholds
type QualityConfig struct {
	MinCosineSimilarity   float64 `json:"min_cosine_similarity" validate:"min=0,max=1"`
	MinEmbeddingMagnitude float64 `json:"min_embedding_magnitude" validate:"min=0"`
	AcceptableErrorRate   float64 `json:"acceptable_error_rate" validate:"min=0,max=1"`
}

// FallbackConfig defines behavior when primary models fail
type FallbackConfig struct {
	MaxRetries          int           `json:"max_retries" validate:"min=0,max=10"`
	InitialDelayMs      int           `json:"initial_delay_ms" validate:"min=0"`
	MaxDelayMs          int           `json:"max_delay_ms" validate:"min=0"`
	ExponentialBase     float64       `json:"exponential_base" validate:"min=1,max=10"`
	QueueOnFailure      bool          `json:"queue_on_failure"`
	QueueTimeoutMs      int           `json:"queue_timeout_ms" validate:"min=0"`
	CircuitBreaker      CircuitConfig `json:"circuit_breaker"`
}

// CircuitConfig defines circuit breaker settings
type CircuitConfig struct {
	Enabled             bool   `json:"enabled"`
	FailureThreshold    int    `json:"failure_threshold" validate:"min=1"`
	SuccessThreshold    int    `json:"success_threshold" validate:"min=1"`
	TimeoutSeconds      int    `json:"timeout_seconds" validate:"min=1"`
	HalfOpenRequests    int    `json:"half_open_requests" validate:"min=1"`
}

// ConfigFilter for querying agent configurations
type ConfigFilter struct {
	AgentID   *string            `json:"agent_id,omitempty"`
	IsActive  *bool              `json:"is_active,omitempty"`
	Strategy  *EmbeddingStrategy `json:"strategy,omitempty"`
	CreatedBy *string            `json:"created_by,omitempty"`
	Limit     int                `json:"limit,omitempty"`
	Offset    int                `json:"offset,omitempty"`
}

// ConfigUpdateRequest represents a request to update agent configuration
type ConfigUpdateRequest struct {
	EmbeddingStrategy *EmbeddingStrategy    `json:"embedding_strategy,omitempty"`
	ModelPreferences  []ModelPreference     `json:"model_preferences,omitempty"`
	Constraints       *AgentConstraints     `json:"constraints,omitempty"`
	FallbackBehavior  *FallbackConfig       `json:"fallback_behavior,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	UpdatedBy         string                `json:"updated_by" validate:"required"`
}

// Validate validates the agent configuration
func (c *AgentConfig) Validate() error {
	if c.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}

	if len(c.ModelPreferences) == 0 {
		return fmt.Errorf("at least one model preference is required")
	}

	// Validate each model preference
	taskTypes := make(map[TaskType]bool)
	for _, pref := range c.ModelPreferences {
		if taskTypes[pref.TaskType] {
			return fmt.Errorf("duplicate task type: %s", pref.TaskType)
		}
		taskTypes[pref.TaskType] = true

		if len(pref.PrimaryModels) == 0 {
			return fmt.Errorf("at least one primary model is required for task type: %s", pref.TaskType)
		}
	}

	// Validate constraints
	if c.Constraints.MaxCostPerMonthUSD < 0 {
		return fmt.Errorf("max_cost_per_month_usd must be non-negative")
	}

	if c.Constraints.MinAvailabilitySLA < 0 || c.Constraints.MinAvailabilitySLA > 1 {
		return fmt.Errorf("min_availability_sla must be between 0 and 1")
	}

	return nil
}

// ToJSON converts the configuration to JSON
func (c *AgentConfig) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

// FromJSON populates the configuration from JSON
func (c *AgentConfig) FromJSON(data []byte) error {
	return json.Unmarshal(data, c)
}

// GetModelsForTask returns the models configured for a specific task type
func (c *AgentConfig) GetModelsForTask(taskType TaskType) (primary []string, fallback []string) {
	for _, pref := range c.ModelPreferences {
		if pref.TaskType == taskType {
			return pref.PrimaryModels, pref.FallbackModels
		}
	}
	
	// If no specific preference, look for a default
	for _, pref := range c.ModelPreferences {
		if pref.TaskType == TaskTypeGeneralQA {
			return pref.PrimaryModels, pref.FallbackModels
		}
	}
	
	return nil, nil
}

// Clone creates a deep copy of the configuration
func (c *AgentConfig) Clone() *AgentConfig {
	clone := *c
	
	// Deep copy slices
	clone.ModelPreferences = make([]ModelPreference, len(c.ModelPreferences))
	for i, pref := range c.ModelPreferences {
		clone.ModelPreferences[i] = ModelPreference{
			TaskType: pref.TaskType,
			Weight:   pref.Weight,
		}
		clone.ModelPreferences[i].PrimaryModels = append([]string(nil), pref.PrimaryModels...)
		clone.ModelPreferences[i].FallbackModels = append([]string(nil), pref.FallbackModels...)
	}
	
	// Deep copy metadata
	if c.Metadata != nil {
		clone.Metadata = make(map[string]interface{})
		for k, v := range c.Metadata {
			clone.Metadata[k] = v
		}
	}
	
	return &clone
}