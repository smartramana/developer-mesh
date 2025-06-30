package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// WorkflowSteps represents an array of workflow steps with proper database serialization
type WorkflowSteps []WorkflowStep

// Value implements driver.Valuer for database serialization
func (s WorkflowSteps) Value() (driver.Value, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(s)
}

// Scan implements sql.Scanner for database deserialization
func (s *WorkflowSteps) Scan(value interface{}) error {
	if value == nil {
		*s = WorkflowSteps{}
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("cannot scan type %T into WorkflowSteps", value)
	}

	// Handle empty array
	if len(data) == 0 || string(data) == "[]" {
		*s = WorkflowSteps{}
		return nil
	}

	// Try to unmarshal as array
	var steps []WorkflowStep
	if err := json.Unmarshal(data, &steps); err != nil {
		// For backward compatibility, try to handle object with "steps" key
		var obj map[string]interface{}
		if err := json.Unmarshal(data, &obj); err != nil {
			return fmt.Errorf("failed to unmarshal WorkflowSteps: %w", err)
		}

		// Check if it has a "steps" key with an array
		if stepsData, ok := obj["steps"].([]interface{}); ok {
			steps = make([]WorkflowStep, 0, len(stepsData))
			for _, stepData := range stepsData {
				stepBytes, err := json.Marshal(stepData)
				if err != nil {
					continue
				}
				var step WorkflowStep
				if err := json.Unmarshal(stepBytes, &step); err != nil {
					continue
				}
				steps = append(steps, step)
			}
		}
	}

	*s = steps
	return nil
}

// MarshalJSON implements json.Marshaler
func (s WorkflowSteps) MarshalJSON() ([]byte, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]WorkflowStep(s))
}

// UnmarshalJSON implements json.Unmarshaler
func (s *WorkflowSteps) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" || string(data) == "[]" {
		*s = WorkflowSteps{}
		return nil
	}

	var steps []WorkflowStep
	if err := json.Unmarshal(data, &steps); err != nil {
		// For backward compatibility, try to handle object with "steps" key
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(data, &obj); err != nil {
			return fmt.Errorf("failed to unmarshal WorkflowSteps: %w", err)
		}

		if stepsData, ok := obj["steps"]; ok {
			if err := json.Unmarshal(stepsData, &steps); err != nil {
				return fmt.Errorf("failed to unmarshal steps array: %w", err)
			}
		}
	}

	*s = steps
	return nil
}

// GetByID returns a step by its ID
func (s WorkflowSteps) GetByID(id string) (*WorkflowStep, bool) {
	for i := range s {
		if s[i].ID == id {
			return &s[i], true
		}
	}
	return nil, false
}

// GetInExecutionOrder returns steps in execution order based on dependencies
func (s WorkflowSteps) GetInExecutionOrder() []WorkflowStep {
	// For now, return as-is. In a production system, this would
	// implement topological sorting based on dependencies
	steps := make([]WorkflowStep, len(s))
	copy(steps, s)
	return steps
}

// Validate ensures all steps are valid
func (s WorkflowSteps) Validate() error {
	if len(s) == 0 {
		return nil // Empty steps array is valid
	}

	seenIDs := make(map[string]bool)
	for i, step := range s {
		if step.ID == "" {
			return fmt.Errorf("step at index %d has no ID", i)
		}
		if seenIDs[step.ID] {
			return fmt.Errorf("duplicate step ID: %s", step.ID)
		}
		seenIDs[step.ID] = true

		if step.Name == "" {
			return fmt.Errorf("step %s has no name", step.ID)
		}
		if step.Type == "" {
			return fmt.Errorf("step %s has no type", step.ID)
		}

		// Validate dependencies exist
		for _, dep := range step.Dependencies {
			if !seenIDs[dep] {
				// Check if it exists later in the array
				found := false
				for _, futureStep := range s[i+1:] {
					if futureStep.ID == dep {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("step %s depends on non-existent step %s", step.ID, dep)
				}
			}
		}
	}

	return nil
}
