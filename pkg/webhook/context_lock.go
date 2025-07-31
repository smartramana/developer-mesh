package webhook

import (
	"context"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/redis"
	"github.com/google/uuid"
)

// Lock-related constants
const (
	lockKeyPrefix  = "context:lock:"
	lockTTL        = 30 * time.Second
	lockRetryDelay = 100 * time.Millisecond
	maxLockRetries = 10
)

// ContextLock represents a distributed lock for context operations
type ContextLock struct {
	lockID          string
	contextID       string
	tenantID        string
	client          *redis.StreamsClient
	acquiredAt      time.Time
	metricsRecorder *ContextLifecycleMetrics
}

// AcquireContextLock attempts to acquire a distributed lock for a context
func (m *ContextLifecycleManager) AcquireContextLock(ctx context.Context, tenantID, contextID string) (*ContextLock, error) {
	startTime := time.Now()
	lockID := uuid.New().String()
	lockKey := fmt.Sprintf("%s%s:%s", lockKeyPrefix, tenantID, contextID)

	client := m.redisClient.GetClient()

	// Try to acquire lock with retries
	for i := 0; i < maxLockRetries; i++ {
		success, err := client.SetNX(ctx, lockKey, lockID, lockTTL).Result()
		if err != nil {
			m.metricsRecorder.RecordLockAcquisition(tenantID, false, time.Since(startTime))
			return nil, fmt.Errorf("failed to acquire lock: %w", err)
		}

		if success {
			duration := time.Since(startTime)
			m.metricsRecorder.RecordLockAcquisition(tenantID, true, duration)
			m.metricsRecorder.RecordLockContention(tenantID, i)

			m.logger.Debug("Acquired context lock", map[string]interface{}{
				"lock_id":    lockID,
				"context_id": contextID,
				"tenant_id":  tenantID,
				"attempt":    i + 1,
				"duration":   duration.String(),
			})

			lock := &ContextLock{
				lockID:          lockID,
				contextID:       contextID,
				tenantID:        tenantID,
				client:          m.redisClient,
				acquiredAt:      time.Now(),
				metricsRecorder: m.metricsRecorder,
			}
			return lock, nil
		}

		// Wait before retry
		select {
		case <-ctx.Done():
			m.metricsRecorder.RecordLockAcquisition(tenantID, false, time.Since(startTime))
			return nil, ctx.Err()
		case <-time.After(lockRetryDelay):
			continue
		}
	}

	m.metricsRecorder.RecordLockAcquisition(tenantID, false, time.Since(startTime))
	m.metricsRecorder.RecordLockContention(tenantID, maxLockRetries)
	return nil, fmt.Errorf("failed to acquire lock after %d retries", maxLockRetries)
}

// Release releases the distributed lock
func (l *ContextLock) Release(ctx context.Context) error {
	// Record lock hold time
	if l.metricsRecorder != nil && !l.acquiredAt.IsZero() {
		holdTime := time.Since(l.acquiredAt)
		l.metricsRecorder.RecordLockHoldTime(l.tenantID, holdTime)
	}

	lockKey := fmt.Sprintf("%s%s:%s", lockKeyPrefix, l.tenantID, l.contextID)
	client := l.client.GetClient()

	// Use Lua script to ensure we only delete our own lock
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	result, err := client.Eval(ctx, script, []string{lockKey}, l.lockID).Result()
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	if result.(int64) == 0 {
		return fmt.Errorf("lock was already released or expired")
	}

	return nil
}

// ExtendLock extends the TTL of the lock
func (l *ContextLock) ExtendLock(ctx context.Context, duration time.Duration) error {
	lockKey := fmt.Sprintf("%s%s:%s", lockKeyPrefix, l.tenantID, l.contextID)
	client := l.client.GetClient()

	// Verify we still own the lock and extend it
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("expire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`

	result, err := client.Eval(ctx, script, []string{lockKey}, l.lockID, int(duration.Seconds())).Result()
	if err != nil {
		return fmt.Errorf("failed to extend lock: %w", err)
	}

	if result.(int64) == 0 {
		return fmt.Errorf("lock is no longer owned by this instance")
	}

	return nil
}

// TryAcquireContextLock attempts to acquire a lock without retries
func (m *ContextLifecycleManager) TryAcquireContextLock(ctx context.Context, tenantID, contextID string) (*ContextLock, error) {
	lockID := uuid.New().String()
	lockKey := fmt.Sprintf("%s%s:%s", lockKeyPrefix, tenantID, contextID)

	client := m.redisClient.GetClient()

	success, err := client.SetNX(ctx, lockKey, lockID, lockTTL).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	if !success {
		return nil, fmt.Errorf("lock is already held")
	}

	return &ContextLock{
		lockID:          lockID,
		contextID:       contextID,
		tenantID:        tenantID,
		client:          m.redisClient,
		acquiredAt:      time.Now(),
		metricsRecorder: m.metricsRecorder,
	}, nil
}
