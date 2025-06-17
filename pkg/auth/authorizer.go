package auth

import (
	"context"
)

// Authorizer provides authorization functionality
type Authorizer interface {
	Authorize(ctx context.Context, permission Permission) Decision
	CheckPermission(ctx context.Context, resource, action string) bool
}

// Permission represents a permission request
type Permission struct {
	Resource   string                 `json:"resource"`
	Action     string                 `json:"action"`
	Conditions map[string]interface{} `json:"conditions,omitempty"`
}

// Decision represents an authorization decision
type Decision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}