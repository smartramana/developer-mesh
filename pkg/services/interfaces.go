package services

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// RateLimiter provides rate limiting functionality
type RateLimiter interface {
	Check(ctx context.Context, key string) error
	CheckWithLimit(ctx context.Context, key string, limit int, window time.Duration) error
	GetRemaining(ctx context.Context, key string) (int, error)
	Reset(ctx context.Context, key string) error
}

// QuotaManager manages resource quotas
type QuotaManager interface {
	GetQuota(ctx context.Context, tenantID uuid.UUID, resource string) (int64, error)
	GetUsage(ctx context.Context, tenantID uuid.UUID, resource string) (int64, error)
	IncrementUsage(ctx context.Context, tenantID uuid.UUID, resource string, amount int64) error
	SetQuota(ctx context.Context, tenantID uuid.UUID, resource string, limit int64) error
	GetQuotaStatus(ctx context.Context, tenantID uuid.UUID) (*QuotaStatus, error)
}

// QuotaStatus represents the quota status for a tenant
type QuotaStatus struct {
	TenantID uuid.UUID
	Quotas   map[string]QuotaInfo
}

// QuotaInfo contains quota information for a resource
type QuotaInfo struct {
	Resource  string
	Limit     int64
	Used      int64
	Available int64
	Period    string
}

// Sanitizer provides input sanitization
type Sanitizer interface {
	SanitizeString(input string) string
	SanitizeJSON(input interface{}) (interface{}, error)
	SanitizeHTML(input string) string
}

// EncryptionService provides encryption functionality
type EncryptionService interface {
	Encrypt(ctx context.Context, data []byte) ([]byte, error)
	Decrypt(ctx context.Context, data []byte) ([]byte, error)
	EncryptString(ctx context.Context, data string) (string, error)
	DecryptString(ctx context.Context, data string) (string, error)
}

// HealthChecker provides health check functionality
type HealthChecker interface {
	Check(ctx context.Context) error
	RegisterCheck(name string, check func(context.Context) error)
}

// NotificationService handles notifications
type NotificationService interface {
	NotifyTaskAssigned(ctx context.Context, agentID string, task interface{}) error
	NotifyTaskCompleted(ctx context.Context, agentID string, task interface{}) error
	NotifyWorkflowStarted(ctx context.Context, workflow interface{}) error
	NotifyWorkflowCompleted(ctx context.Context, workflow interface{}) error
	NotifyStepStarted(ctx context.Context, executionID uuid.UUID, stepID string) error
	NotifyStepCompleted(ctx context.Context, executionID uuid.UUID, stepID string, output interface{}) error
	BroadcastToWorkspace(ctx context.Context, workspaceID uuid.UUID, message interface{}) error
}

// EventBus provides event broadcasting
type EventBus interface {
	Subscribe(topic string, handler func(event interface{})) (unsubscribe func())
	Publish(topic string, event interface{})
	Close()
}

// StateStore provides state persistence
type StateStore interface {
	GetState(ctx context.Context, key string) (interface{}, error)
	SetState(ctx context.Context, key string, value interface{}) error
	GetStateForUpdate(ctx context.Context, key string) (interface{}, error)
	SaveState(ctx context.Context, key string, state interface{}) error
}