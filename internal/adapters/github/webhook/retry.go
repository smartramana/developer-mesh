package webhook

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/internal/observability"
)

// RetryConfig holds configuration for the retry manager
type RetryConfig struct {
	// MaxRetries is the maximum number of retries
	MaxRetries int
	
	// InitialBackoff is the initial backoff duration
	InitialBackoff time.Duration
	
	// MaxBackoff is the maximum backoff duration
	MaxBackoff time.Duration
	
	// BackoffFactor is the factor to multiply backoff on each retry
	BackoffFactor float64
	
	// Jitter is the jitter factor to randomize backoff
	Jitter float64
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     1 * time.Hour,
		BackoffFactor:  2.0,
		Jitter:         0.2,
	}
}

// RetryStatus represents the status of a retried event
type RetryStatus string

const (
	// RetryStatusPending indicates the event is pending retry
	RetryStatusPending RetryStatus = "pending"
	
	// RetryStatusInProgress indicates the event is being retried
	RetryStatusInProgress RetryStatus = "in_progress"
	
	// RetryStatusSuccess indicates the event was successfully processed
	RetryStatusSuccess RetryStatus = "success"
	
	// RetryStatusFailed indicates the event processing failed after all retries
	RetryStatusFailed RetryStatus = "failed"
	
	// RetryStatusCancelled indicates the event retry was cancelled
	RetryStatusCancelled RetryStatus = "cancelled"
)

// RetryStorage defines the interface for storing retry information
type RetryStorage interface {
	// Store stores retry information for an event
	Store(ctx context.Context, event Event, status RetryStatus, retryCount int, nextRetry time.Time, err error) error
	
	// Get gets retry information for an event
	Get(ctx context.Context, deliveryID string) (*RetryInfo, error)
	
	// List lists all retry information matching a filter
	List(ctx context.Context, filter RetryFilter) ([]*RetryInfo, error)
	
	// Update updates retry information for an event
	Update(ctx context.Context, info *RetryInfo) error
	
	// Delete deletes retry information for an event
	Delete(ctx context.Context, deliveryID string) error
}

// RetryInfo holds information about a retried event
type RetryInfo struct {
	// DeliveryID is the GitHub delivery ID
	DeliveryID string
	
	// EventType is the GitHub event type
	EventType string
	
	// Event is the parsed webhook event
	Event Event
	
	// Status is the retry status
	Status RetryStatus
	
	// RetryCount is the number of retries attempted
	RetryCount int
	
	// NextRetry is the time of the next retry
	NextRetry time.Time
	
	// LastError is the error from the last retry attempt
	LastError string
	
	// CreatedAt is the time when the retry info was created
	CreatedAt time.Time
	
	// UpdatedAt is the time when the retry info was last updated
	UpdatedAt time.Time
}

// RetryFilter defines a filter for listing retry information
type RetryFilter struct {
	// DeliveryIDs are the delivery IDs to filter by
	DeliveryIDs []string
	
	// EventTypes are the event types to filter by
	EventTypes []string
	
	// Statuses are the statuses to filter by
	Statuses []RetryStatus
	
	// MinRetryCount is the minimum retry count to filter by
	MinRetryCount int
	
	// MaxRetryCount is the maximum retry count to filter by
	MaxRetryCount int
	
	// Since is the minimum creation time to filter by
	Since time.Time
	
	// Until is the maximum creation time to filter by
	Until time.Time
}

// RetryHandler defines the handler for retrying events
type RetryHandler func(ctx context.Context, event Event) error

// RetryManager manages the retrying of webhook events
type RetryManager struct {
	config   *RetryConfig
	storage  RetryStorage
	handler  RetryHandler
	logger   *observability.Logger
	queue    chan *RetryInfo
	mutex    sync.RWMutex
	closed   bool
	timer    *time.Timer
	wg       sync.WaitGroup
}

// NewRetryManager creates a new retry manager
func NewRetryManager(config *RetryConfig, storage RetryStorage, handler RetryHandler, logger *observability.Logger) *RetryManager {
	if config == nil {
		config = DefaultRetryConfig()
	}
	
	return &RetryManager{
		config:  config,
		storage: storage,
		handler: handler,
		logger:  logger,
		queue:   make(chan *RetryInfo, 100),
	}
}

// Start starts the retry manager
func (m *RetryManager) Start(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if m.closed {
		return fmt.Errorf("retry manager is closed")
	}
	
	// Start workers
	m.wg.Add(2)
	go m.processQueue(ctx)
	go m.schedulePendingRetries(ctx)
	
	return nil
}

// Close closes the retry manager
func (m *RetryManager) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if m.closed {
		return nil
	}
	
	m.closed = true
	close(m.queue)
	
	// Wait for workers to finish
	m.wg.Wait()
	
	return nil
}

// ScheduleRetry schedules a webhook event for retry
func (m *RetryManager) ScheduleRetry(ctx context.Context, event Event, err error) error {
	m.mutex.RLock()
	closed := m.closed
	m.mutex.RUnlock()
	
	if closed {
		return fmt.Errorf("retry manager is closed")
	}
	
	// Get retry info for the event
	info, getErr := m.storage.Get(ctx, event.DeliveryID)
	if getErr != nil {
		// If not found, create new retry info
		info = &RetryInfo{
			DeliveryID: event.DeliveryID,
			EventType:  event.Type,
			Event:      event,
			Status:     RetryStatusPending,
			RetryCount: 0,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
	}
	
	// Update retry info
	info.RetryCount++
	info.Status = RetryStatusPending
	if err != nil {
		info.LastError = err.Error()
	}
	info.UpdatedAt = time.Now()
	
	// Calculate next retry time with exponential backoff
	if info.RetryCount > m.config.MaxRetries {
		// Max retries reached
		info.Status = RetryStatusFailed
		info.NextRetry = time.Time{}
	} else {
		// Calculate backoff duration
		backoff := m.calculateBackoff(info.RetryCount)
		info.NextRetry = time.Now().Add(backoff)
	}
	
	// Store retry info
	storeErr := m.storage.Store(ctx, event, info.Status, info.RetryCount, info.NextRetry, err)
	if storeErr != nil {
		return fmt.Errorf("failed to store retry info: %w", storeErr)
	}
	
	m.logger.Info("Scheduled webhook event for retry", map[string]interface{}{
		"eventType":  event.Type,
		"deliveryID": event.DeliveryID,
		"retryCount": info.RetryCount,
		"nextRetry":  info.NextRetry,
	})
	
	// If event should be retried now, add to queue
	if info.Status == RetryStatusPending && !info.NextRetry.After(time.Now()) {
		select {
		case m.queue <- info:
			// Successfully queued
		default:
			// Queue is full
			m.logger.Warn("Retry queue is full", map[string]interface{}{
				"eventType":  event.Type,
				"deliveryID": event.DeliveryID,
			})
		}
	}
	
	return nil
}

// Retry retries a webhook event immediately
func (m *RetryManager) Retry(ctx context.Context, deliveryID string) error {
	m.mutex.RLock()
	closed := m.closed
	m.mutex.RUnlock()
	
	if closed {
		return fmt.Errorf("retry manager is closed")
	}
	
	// Get retry info for the event
	info, err := m.storage.Get(ctx, deliveryID)
	if err != nil {
		return fmt.Errorf("failed to get retry info: %w", err)
	}
	
	// Update retry info
	info.Status = RetryStatusPending
	info.NextRetry = time.Now()
	info.UpdatedAt = time.Now()
	
	// Store updated retry info
	if err := m.storage.Update(ctx, info); err != nil {
		return fmt.Errorf("failed to update retry info: %w", err)
	}
	
	// Add to queue
	select {
	case m.queue <- info:
		// Successfully queued
	default:
		// Queue is full
		return fmt.Errorf("retry queue is full")
	}
	
	return nil
}

// Cancel cancels a scheduled retry
func (m *RetryManager) Cancel(ctx context.Context, deliveryID string) error {
	// Get retry info for the event
	info, err := m.storage.Get(ctx, deliveryID)
	if err != nil {
		return fmt.Errorf("failed to get retry info: %w", err)
	}
	
	// Update status
	info.Status = RetryStatusCancelled
	info.UpdatedAt = time.Now()
	
	// Store updated retry info
	if err := m.storage.Update(ctx, info); err != nil {
		return fmt.Errorf("failed to update retry info: %w", err)
	}
	
	return nil
}

// GetStatus gets the status of a retried event
func (m *RetryManager) GetStatus(ctx context.Context, deliveryID string) (*RetryInfo, error) {
	return m.storage.Get(ctx, deliveryID)
}

// List lists all retry information matching a filter
func (m *RetryManager) List(ctx context.Context, filter RetryFilter) ([]*RetryInfo, error) {
	return m.storage.List(ctx, filter)
}

// addJitter adds jitter to a duration to avoid thundering herd problems
func addJitter(duration time.Duration, jitter float64) time.Duration {
	if jitter <= 0 {
		return duration
	}
	
	// Calculate jitter range (duration * jitter)
	jitterRange := float64(duration) * jitter
	
	// Generate random jitter within range [-jitterRange/2, jitterRange/2]
	randomJitter := (rand.Float64()-0.5) * jitterRange
	
	// Apply jitter
	return time.Duration(float64(duration) + randomJitter)
}

// calculateBackoff calculates the backoff duration for a retry
func (m *RetryManager) calculateBackoff(retryCount int) time.Duration {
	// Calculate base backoff
	backoff := m.config.InitialBackoff * time.Duration(math.Pow(m.config.BackoffFactor, float64(retryCount-1)))
	
	// Apply jitter
	if m.config.Jitter > 0 {
		backoff = addJitter(backoff, m.config.Jitter)
	}
	
	// Cap at max backoff
	if backoff > m.config.MaxBackoff {
		backoff = m.config.MaxBackoff
	}
	
	return backoff
}

// processQueue processes the retry queue
func (m *RetryManager) processQueue(ctx context.Context) {
	defer m.wg.Done()
	
	for info := range m.queue {
		m.processRetry(ctx, info)
	}
}

// processRetry processes a single retry
func (m *RetryManager) processRetry(ctx context.Context, info *RetryInfo) {
	// Update status
	info.Status = RetryStatusInProgress
	info.UpdatedAt = time.Now()
	
	// Store updated retry info
	if err := m.storage.Update(ctx, info); err != nil {
		m.logger.Error("Failed to update retry info", map[string]interface{}{
			"eventType":  info.EventType,
			"deliveryID": info.DeliveryID,
			"error":      err.Error(),
		})
		return
	}
	
	// Execute handler
	m.logger.Info("Executing webhook event retry", map[string]interface{}{
		"eventType":  info.EventType,
		"deliveryID": info.DeliveryID,
		"retryCount": info.RetryCount,
	})
	
	err := m.handler(ctx, info.Event)
	
	// Update status
	if err != nil {
		// Handler failed
		info.LastError = err.Error()
		
		// Check if max retries reached
		if info.RetryCount >= m.config.MaxRetries {
			info.Status = RetryStatusFailed
		} else {
			// Schedule next retry
			info.Status = RetryStatusPending
			info.NextRetry = time.Now().Add(m.calculateBackoff(info.RetryCount + 1))
		}
	} else {
		// Handler succeeded
		info.Status = RetryStatusSuccess
		info.LastError = ""
	}
	
	info.UpdatedAt = time.Now()
	
	// Store updated retry info
	if err := m.storage.Update(ctx, info); err != nil {
		m.logger.Error("Failed to update retry info", map[string]interface{}{
			"eventType":  info.EventType,
			"deliveryID": info.DeliveryID,
			"error":      err.Error(),
		})
		return
	}
	
	// Log result
	if err != nil {
		m.logger.Error("Webhook event retry failed", map[string]interface{}{
			"eventType":  info.EventType,
			"deliveryID": info.DeliveryID,
			"retryCount": info.RetryCount,
			"error":      err.Error(),
		})
	} else {
		m.logger.Info("Webhook event retry succeeded", map[string]interface{}{
			"eventType":  info.EventType,
			"deliveryID": info.DeliveryID,
			"retryCount": info.RetryCount,
		})
	}
}

// schedulePendingRetries schedules pending retries
func (m *RetryManager) schedulePendingRetries(ctx context.Context) {
	defer m.wg.Done()
	
	m.timer = time.NewTimer(5 * time.Second)
	defer m.timer.Stop()
	
	for {
		// Wait for timer
		select {
		case <-ctx.Done():
			// Context cancelled
			return
		case <-m.timer.C:
			// Timer expired
		}
		
		// Check if manager is closed
		m.mutex.RLock()
		closed := m.closed
		m.mutex.RUnlock()
		
		if closed {
			return
		}
		
		// Find pending retries
		now := time.Now()
		filter := RetryFilter{
			Statuses: []RetryStatus{RetryStatusPending},
			Until:    now,
		}
		
		infos, err := m.storage.List(ctx, filter)
		if err != nil {
			m.logger.Error("Failed to list pending retries", map[string]interface{}{
				"error": err.Error(),
			})
		} else {
			// Add to queue
			for _, info := range infos {
				select {
				case m.queue <- info:
					// Successfully queued
				default:
					// Queue is full
					m.logger.Warn("Retry queue is full", map[string]interface{}{
						"eventType":  info.EventType,
						"deliveryID": info.DeliveryID,
					})
				}
			}
		}
		
		// Reset timer for next check
		m.timer.Reset(5 * time.Second)
	}
}

// InMemoryRetryStorage is a simple in-memory implementation of RetryStorage
type InMemoryRetryStorage struct {
	retries map[string]*RetryInfo
	mutex   sync.RWMutex
}

// NewInMemoryRetryStorage creates a new in-memory retry storage
func NewInMemoryRetryStorage() *InMemoryRetryStorage {
	return &InMemoryRetryStorage{
		retries: make(map[string]*RetryInfo),
	}
}

// Store stores retry information for an event
func (s *InMemoryRetryStorage) Store(ctx context.Context, event Event, status RetryStatus, retryCount int, nextRetry time.Time, err error) error {
	info := &RetryInfo{
		DeliveryID: event.DeliveryID,
		EventType:  event.Type,
		Event:      event,
		Status:     status,
		RetryCount: retryCount,
		NextRetry:  nextRetry,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	
	if err != nil {
		info.LastError = err.Error()
	}
	
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.retries[event.DeliveryID] = info
	return nil
}

// Get gets retry information for an event
func (s *InMemoryRetryStorage) Get(ctx context.Context, deliveryID string) (*RetryInfo, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	info, ok := s.retries[deliveryID]
	if !ok {
		return nil, fmt.Errorf("no retry info found for delivery ID: %s", deliveryID)
	}
	
	return info, nil
}

// List lists all retry information matching a filter
func (s *InMemoryRetryStorage) List(ctx context.Context, filter RetryFilter) ([]*RetryInfo, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	var result []*RetryInfo
	
	for _, info := range s.retries {
		// Apply filter
		if !s.matchesFilter(info, filter) {
			continue
		}
		
		result = append(result, info)
	}
	
	return result, nil
}

// matchesFilter checks if retry info matches a filter
func (s *InMemoryRetryStorage) matchesFilter(info *RetryInfo, filter RetryFilter) bool {
	// Check delivery IDs
	if len(filter.DeliveryIDs) > 0 {
		matched := false
		for _, id := range filter.DeliveryIDs {
			if id == info.DeliveryID {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	
	// Check event types
	if len(filter.EventTypes) > 0 {
		matched := false
		for _, t := range filter.EventTypes {
			if t == info.EventType {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	
	// Check statuses
	if len(filter.Statuses) > 0 {
		matched := false
		for _, s := range filter.Statuses {
			if s == info.Status {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	
	// Check retry count
	if filter.MinRetryCount > 0 && info.RetryCount < filter.MinRetryCount {
		return false
	}
	if filter.MaxRetryCount > 0 && info.RetryCount > filter.MaxRetryCount {
		return false
	}
	
	// Check creation time
	if !filter.Since.IsZero() && info.CreatedAt.Before(filter.Since) {
		return false
	}
	if !filter.Until.IsZero() && info.CreatedAt.After(filter.Until) {
		return false
	}
	
	return true
}

// Update updates retry information for an event
func (s *InMemoryRetryStorage) Update(ctx context.Context, info *RetryInfo) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	// Check if retry info exists
	if _, ok := s.retries[info.DeliveryID]; !ok {
		return fmt.Errorf("no retry info found for delivery ID: %s", info.DeliveryID)
	}
	
	// Update timestamp
	info.UpdatedAt = time.Now()
	
	// Store updated info
	s.retries[info.DeliveryID] = info
	
	return nil
}

// Delete deletes retry information for an event
func (s *InMemoryRetryStorage) Delete(ctx context.Context, deliveryID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	// Check if retry info exists
	if _, ok := s.retries[deliveryID]; !ok {
		return fmt.Errorf("no retry info found for delivery ID: %s", deliveryID)
	}
	
	// Delete retry info
	delete(s.retries, deliveryID)
	
	return nil
}
