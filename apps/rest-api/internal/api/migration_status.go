package api

import (
	"context"
	"sync"
	"time"
)

// MigrationStatus tracks the status of database migrations
type MigrationStatus struct {
	mu          sync.RWMutex
	inProgress  bool
	completed   bool
	version     string
	startedAt   time.Time
	completedAt time.Time
	error       error
}

// GlobalMigrationStatus is a package-level variable to track migration status
var GlobalMigrationStatus = &MigrationStatus{}

// SetInProgress marks migrations as in progress
func (m *MigrationStatus) SetInProgress() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inProgress = true
	m.completed = false
	m.startedAt = time.Now()
}

// SetCompleted marks migrations as completed
func (m *MigrationStatus) SetCompleted(version string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inProgress = false
	m.completed = true
	m.version = version
	m.completedAt = time.Now()
	m.error = nil
}

// SetFailed marks migrations as failed
func (m *MigrationStatus) SetFailed(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inProgress = false
	m.completed = false
	m.error = err
}

// IsReady returns true if migrations are completed successfully
func (m *MigrationStatus) IsReady() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.completed && !m.inProgress && m.error == nil
}

// GetStatus returns the current migration status
func (m *MigrationStatus) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := map[string]interface{}{
		"in_progress": m.inProgress,
		"completed":   m.completed,
		"ready":       m.IsReady(),
	}

	if m.version != "" {
		status["version"] = m.version
	}

	if !m.startedAt.IsZero() {
		status["started_at"] = m.startedAt.Format(time.RFC3339)
	}

	if !m.completedAt.IsZero() {
		status["completed_at"] = m.completedAt.Format(time.RFC3339)
		status["duration"] = m.completedAt.Sub(m.startedAt).String()
	}

	if m.error != nil {
		status["error"] = m.error.Error()
	}

	return status
}

// WaitForReady waits for migrations to complete with timeout
func (m *MigrationStatus) WaitForReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if m.IsReady() {
				return nil
			}
			if time.Now().After(deadline) {
				return context.DeadlineExceeded
			}
		}
	}
}
