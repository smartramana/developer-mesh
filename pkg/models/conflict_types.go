package models

// TaskConflict represents a conflict in task updates
type TaskConflict struct {
	Type   string        `json:"type"`   // status, assignment, priority
	Values []interface{} `json:"values"` // Conflicting values from different agents
}