package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/sony/gobreaker"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/events"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/resilience"
	"github.com/developer-mesh/developer-mesh/pkg/rules"
)

// ServiceConfig provides common configuration for all services
type ServiceConfig struct {
	// Resilience
	CircuitBreaker *gobreaker.Settings
	RetryPolicy    resilience.RetryPolicy
	TimeoutPolicy  resilience.TimeoutPolicy
	BulkheadPolicy resilience.BulkheadPolicy

	// Rate Limiting
	RateLimiter  RateLimiter
	QuotaManager QuotaManager

	// Security
	Authorizer        auth.Authorizer
	Sanitizer         Sanitizer
	EncryptionService EncryptionService

	// Observability
	Logger  observability.Logger
	Metrics observability.MetricsClient
	Tracer  observability.StartSpanFunc

	// Business Rules
	RuleEngine    rules.Engine
	PolicyManager rules.PolicyManager
}

// BaseService provides common functionality for all services
type BaseService struct {
	config         ServiceConfig
	eventStore     events.EventStore
	eventPublisher events.EventPublisher
	healthChecker  HealthChecker
}

// NewBaseService creates a new base service
func NewBaseService(config ServiceConfig) BaseService {
	return BaseService{
		config: config,
	}
}

// SetEventStore sets the event store
func (s *BaseService) SetEventStore(store events.EventStore) {
	s.eventStore = store
}

// SetEventPublisher sets the event publisher
func (s *BaseService) SetEventPublisher(publisher events.EventPublisher) {
	s.eventPublisher = publisher
}

// SetHealthChecker sets the health checker
func (s *BaseService) SetHealthChecker(checker HealthChecker) {
	s.healthChecker = checker
}

// Transaction represents a distributed transaction
type Transaction interface {
	Commit() error
	Rollback() error
	GetID() uuid.UUID
}

// Compensator manages saga compensations
type Compensator struct {
	compensations []func() error
	logger        observability.Logger
}

// NewCompensator creates a new compensator
func NewCompensator(logger observability.Logger) *Compensator {
	return &Compensator{
		compensations: make([]func() error, 0),
		logger:        logger,
	}
}

// AddCompensation adds a compensation function
func (c *Compensator) AddCompensation(fn func() error) {
	c.compensations = append([]func() error{fn}, c.compensations...)
}

// Compensate runs all compensations
func (c *Compensator) Compensate(ctx context.Context) error {
	var errs []error
	for _, compensation := range c.compensations {
		if err := compensation(); err != nil {
			c.logger.Error("Compensation failed", map[string]interface{}{
				"error": err.Error(),
			})
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Wrap(errs[0], "compensation failed")
	}
	return nil
}

// WithTransaction executes a function within a transaction with saga support
func (s *BaseService) WithTransaction(ctx context.Context, fn func(ctx context.Context, tx Transaction) error) error {
	// Start distributed transaction
	tx, err := s.startDistributedTransaction(ctx)
	if err != nil {
		return err
	}

	// Setup compensation
	compensator := NewCompensator(s.config.Logger)
	type compensatorKey struct{}
	ctx = context.WithValue(ctx, compensatorKey{}, compensator)

	// Execute function
	err = fn(ctx, tx)
	if err != nil {
		// Run compensations
		if compErr := compensator.Compensate(ctx); compErr != nil {
			s.config.Logger.Error("Compensation failed", map[string]interface{}{
				"error":          compErr.Error(),
				"original_error": err.Error(),
			})
		}
		if rbErr := tx.Rollback(); rbErr != nil {
			s.config.Logger.Error("Rollback failed", map[string]interface{}{
				"error": rbErr.Error(),
			})
		}
		return err
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		if compErr := compensator.Compensate(ctx); compErr != nil {
			s.config.Logger.Error("Compensation failed", map[string]interface{}{
				"error": compErr.Error(),
			})
		}
		return err
	}

	return nil
}

// AggregateRoot represents an aggregate root for event sourcing
type AggregateRoot interface {
	GetID() uuid.UUID
	GetType() string
	GetVersion() int
}

// PublishEvent publishes an event with versioning and metadata
func (s *BaseService) PublishEvent(ctx context.Context, eventType string, aggregate AggregateRoot, data interface{}) error {
	if s.eventPublisher == nil {
		// Event publishing not configured, skip
		return nil
	}

	event := &events.DomainEvent{
		ID:            uuid.New(),
		Type:          eventType,
		AggregateID:   aggregate.GetID(),
		AggregateType: aggregate.GetType(),
		Version:       aggregate.GetVersion(),
		Timestamp:     time.Now(),
		Data:          data,
		Metadata: events.Metadata{
			TenantID:      auth.GetTenantID(ctx),
			UserID:        auth.GetUserID(ctx),
			CorrelationID: observability.GetCorrelationID(ctx),
			CausationID:   observability.GetCausationID(ctx),
		},
	}

	// Store event if event store is configured
	if s.eventStore != nil {
		if err := s.eventStore.Append(ctx, event); err != nil {
			return err
		}
	}

	// Publish for projections
	return s.eventPublisher.Publish(ctx, event)
}

// CheckRateLimit enforces rate limiting per tenant/agent
func (s *BaseService) CheckRateLimit(ctx context.Context, resource string) error {
	if s.config.RateLimiter == nil {
		// Rate limiting not configured, allow
		return nil
	}

	tenantID := auth.GetTenantID(ctx)
	agentID := auth.GetAgentID(ctx)

	// Check tenant limit
	if err := s.config.RateLimiter.Check(ctx, fmt.Sprintf("tenant:%s:%s", tenantID, resource)); err != nil {
		return ErrRateLimitExceeded
	}

	// Check agent limit
	if err := s.config.RateLimiter.Check(ctx, fmt.Sprintf("agent:%s:%s", agentID, resource)); err != nil {
		return ErrRateLimitExceeded
	}

	return nil
}

// CheckQuota verifies resource quota
func (s *BaseService) CheckQuota(ctx context.Context, resource string, amount int64) error {
	if s.config.QuotaManager == nil {
		// Quota management not configured, allow
		return nil
	}

	tenantID := auth.GetTenantID(ctx)

	quota, err := s.config.QuotaManager.GetQuota(ctx, tenantID, resource)
	if err != nil {
		return err
	}

	usage, err := s.config.QuotaManager.GetUsage(ctx, tenantID, resource)
	if err != nil {
		return err
	}

	if usage+amount > quota {
		return ErrQuotaExceeded
	}

	return s.config.QuotaManager.IncrementUsage(ctx, tenantID, resource, amount)
}

// startDistributedTransaction starts a new distributed transaction
func (s *BaseService) startDistributedTransaction(ctx context.Context) (Transaction, error) {
	// Create a distributed transaction coordinator
	txID := uuid.New()

	// Record transaction start
	s.config.Logger.Info("Starting distributed transaction", map[string]interface{}{
		"transaction_id": txID,
		"tenant_id":      auth.GetTenantID(ctx),
	})

	// Create transaction with proper implementation
	dtx := &distributedTransaction{
		id:            txID,
		participants:  make(map[string]TransactionParticipant),
		state:         TxStateActive,
		logger:        s.config.Logger,
		metrics:       s.config.Metrics,
		timeout:       5 * time.Minute,
		createdAt:     time.Now(),
		compensations: make([]func() error, 0),
	}

	// Set context deadline for transaction
	dtx.ctx, dtx.cancel = context.WithTimeout(ctx, dtx.timeout)

	// Record metric
	s.config.Metrics.IncrementCounterWithLabels("distributed_transaction.started", 1, map[string]string{
		"tenant_id": auth.GetTenantID(ctx).String(),
	})

	return dtx, nil
}

// Transaction states
type TransactionState int

const (
	TxStateActive TransactionState = iota
	TxStatePreparing
	TxStateCommitting
	TxStateAborting
	TxStateCommitted
	TxStateAborted
)

// TransactionParticipant represents a participant in a distributed transaction
type TransactionParticipant interface {
	Prepare(ctx context.Context) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	GetID() string
}

// distributedTransaction implements a simplified 2PC distributed transaction
type distributedTransaction struct {
	id            uuid.UUID
	participants  map[string]TransactionParticipant
	state         TransactionState
	logger        observability.Logger
	metrics       observability.MetricsClient
	timeout       time.Duration
	createdAt     time.Time
	ctx           context.Context
	cancel        context.CancelFunc
	compensations []func() error
	mu            sync.Mutex
}

func (t *distributedTransaction) GetID() uuid.UUID {
	return t.id
}

func (t *distributedTransaction) RegisterParticipant(participant TransactionParticipant) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state != TxStateActive {
		return fmt.Errorf("cannot register participant: transaction not active")
	}

	participantID := participant.GetID()
	if _, exists := t.participants[participantID]; exists {
		return fmt.Errorf("participant already registered: %s", participantID)
	}

	t.participants[participantID] = participant

	t.logger.Info("Participant registered", map[string]interface{}{
		"transaction_id":     t.id,
		"participant_id":     participantID,
		"total_participants": len(t.participants),
	})

	return nil
}

func (t *distributedTransaction) AddCompensation(compensation func() error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.compensations = append(t.compensations, compensation)
}

func (t *distributedTransaction) Commit() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state != TxStateActive {
		return fmt.Errorf("cannot commit: transaction not in active state")
	}

	startTime := time.Now()

	// Phase 1: Prepare
	t.state = TxStatePreparing
	t.logger.Info("Starting prepare phase", map[string]interface{}{
		"transaction_id": t.id,
		"participants":   len(t.participants),
	})

	prepareErrors := make(map[string]error)
	var wg sync.WaitGroup
	var prepareMu sync.Mutex

	for id, participant := range t.participants {
		wg.Add(1)
		go func(pID string, p TransactionParticipant) {
			defer wg.Done()

			if err := p.Prepare(t.ctx); err != nil {
				prepareMu.Lock()
				prepareErrors[pID] = err
				prepareMu.Unlock()

				t.logger.Error("Participant prepare failed", map[string]interface{}{
					"transaction_id": t.id,
					"participant_id": pID,
					"error":          err.Error(),
				})
			}
		}(id, participant)
	}

	wg.Wait()

	// Check prepare results
	if len(prepareErrors) > 0 {
		t.logger.Warn("Prepare phase failed, rolling back", map[string]interface{}{
			"transaction_id": t.id,
			"failed_count":   len(prepareErrors),
		})

		// Abort transaction
		t.state = TxStateAborting
		t.doRollback()
		t.state = TxStateAborted

		return fmt.Errorf("prepare phase failed: %d participants failed", len(prepareErrors))
	}

	// Phase 2: Commit
	t.state = TxStateCommitting
	t.logger.Info("Starting commit phase", map[string]interface{}{
		"transaction_id": t.id,
	})

	commitErrors := make(map[string]error)
	wg = sync.WaitGroup{}
	var commitMu sync.Mutex

	for id, participant := range t.participants {
		wg.Add(1)
		go func(pID string, p TransactionParticipant) {
			defer wg.Done()

			if err := p.Commit(t.ctx); err != nil {
				commitMu.Lock()
				commitErrors[pID] = err
				commitMu.Unlock()

				t.logger.Error("Participant commit failed", map[string]interface{}{
					"transaction_id": t.id,
					"participant_id": pID,
					"error":          err.Error(),
				})
			}
		}(id, participant)
	}

	wg.Wait()

	// Even if some commits fail, transaction is considered committed
	// Failed participants need to handle recovery
	if len(commitErrors) > 0 {
		t.logger.Warn("Some participants failed to commit", map[string]interface{}{
			"transaction_id": t.id,
			"failed_count":   len(commitErrors),
		})
	}

	t.state = TxStateCommitted

	// Record metrics
	duration := time.Since(startTime)
	t.metrics.RecordHistogram("distributed_transaction.duration", duration.Seconds(), map[string]string{
		"phase":   "commit",
		"success": fmt.Sprintf("%t", len(commitErrors) == 0),
	})

	t.logger.Info("Distributed transaction committed", map[string]interface{}{
		"transaction_id": t.id,
		"duration":       duration.String(),
		"participants":   len(t.participants),
		"failures":       len(commitErrors),
	})

	// Clean up
	t.cancel()

	return nil
}

func (t *distributedTransaction) Rollback() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state == TxStateAborted || t.state == TxStateCommitted {
		return fmt.Errorf("cannot rollback: transaction already %v", t.state)
	}

	t.state = TxStateAborting
	t.doRollback()
	t.state = TxStateAborted

	// Clean up
	t.cancel()

	return nil
}

func (t *distributedTransaction) doRollback() {
	startTime := time.Now()

	t.logger.Info("Starting rollback", map[string]interface{}{
		"transaction_id": t.id,
		"participants":   len(t.participants),
		"compensations":  len(t.compensations),
	})

	// Rollback all participants
	var wg sync.WaitGroup
	for id, participant := range t.participants {
		wg.Add(1)
		go func(pID string, p TransactionParticipant) {
			defer wg.Done()

			if err := p.Rollback(t.ctx); err != nil {
				t.logger.Error("Participant rollback failed", map[string]interface{}{
					"transaction_id": t.id,
					"participant_id": pID,
					"error":          err.Error(),
				})
			}
		}(id, participant)
	}

	wg.Wait()

	// Execute compensations in reverse order
	for i := len(t.compensations) - 1; i >= 0; i-- {
		if err := t.compensations[i](); err != nil {
			t.logger.Error("Compensation failed", map[string]interface{}{
				"transaction_id":     t.id,
				"compensation_index": i,
				"error":              err.Error(),
			})
		}
	}

	// Record metrics
	duration := time.Since(startTime)
	t.metrics.RecordHistogram("distributed_transaction.duration", duration.Seconds(), map[string]string{
		"phase": "rollback",
	})

	t.logger.Info("Distributed transaction rolled back", map[string]interface{}{
		"transaction_id": t.id,
		"duration":       duration.String(),
	})
}

// timePtr returns a pointer to a time
func timePtr(t time.Time) *time.Time {
	return &t
}

// TxOption configures a distributed transaction
type TxOption func(*TxConfig)

// TxConfig holds transaction configuration
type TxConfig struct {
	Timeout        time.Duration
	IsolationLevel string
	RetryPolicy    resilience.RetryPolicy
}

// defaultTxConfig provides default transaction configuration
var defaultTxConfig = TxConfig{
	Timeout:        30 * time.Second,
	IsolationLevel: "read_committed",
}

// WithTxTimeout sets the transaction timeout
func WithTxTimeout(timeout time.Duration) TxOption {
	return func(cfg *TxConfig) {
		cfg.Timeout = timeout
	}
}

// WithTxIsolationLevel sets the transaction isolation level
func WithTxIsolationLevel(level string) TxOption {
	return func(cfg *TxConfig) {
		cfg.IsolationLevel = level
	}
}

// WithTxRetryPolicy sets the transaction retry policy
func WithTxRetryPolicy(policy resilience.RetryPolicy) TxOption {
	return func(cfg *TxConfig) {
		cfg.RetryPolicy = policy
	}
}

// StartDistributedTransaction starts a new distributed transaction with options
func (s *BaseService) StartDistributedTransaction(ctx context.Context, opts ...TxOption) (context.Context, func(), func()) {
	// Generate transaction ID
	txID := uuid.New()

	// Apply options
	config := defaultTxConfig
	for _, opt := range opts {
		opt(&config)
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, config.Timeout)

	// Add transaction ID to context
	type txIDKey struct{}
	ctx = context.WithValue(ctx, txIDKey{}, txID)

	// Add isolation level to context
	type txIsolationKey struct{}
	ctx = context.WithValue(ctx, txIsolationKey{}, config.IsolationLevel)

	// Add transaction start time to context
	type txStartTimeKey struct{}
	ctx = context.WithValue(ctx, txStartTimeKey{}, time.Now())

	// Log transaction start
	s.config.Logger.Info("Transaction started", map[string]interface{}{
		"tx_id":           txID,
		"timeout":         config.Timeout,
		"isolation_level": config.IsolationLevel,
		"tenant_id":       auth.GetTenantID(ctx),
		"user_id":         auth.GetUserID(ctx),
	})

	// Record metric
	s.config.Metrics.IncrementCounterWithLabels("transaction.started", 1, map[string]string{
		"isolation_level": config.IsolationLevel,
	})

	// Create commit function
	commit := func() {
		// Calculate transaction duration
		startTime, _ := ctx.Value(txStartTimeKey{}).(time.Time)
		duration := time.Since(startTime)

		// Log commit
		s.config.Logger.Info("Transaction committed", map[string]interface{}{
			"tx_id":    txID,
			"duration": duration,
		})

		// Record metrics
		s.config.Metrics.RecordHistogram("transaction.duration", duration.Seconds(), map[string]string{
			"status":          "committed",
			"isolation_level": config.IsolationLevel,
		})
		s.config.Metrics.IncrementCounterWithLabels("transaction.committed", 1, map[string]string{
			"isolation_level": config.IsolationLevel,
		})
	}

	// Create rollback function
	rollback := func() {
		// Calculate transaction duration
		startTime, _ := ctx.Value(txStartTimeKey{}).(time.Time)
		duration := time.Since(startTime)

		// Log rollback
		s.config.Logger.Error("Transaction rolled back", map[string]interface{}{
			"tx_id":    txID,
			"duration": duration,
		})

		// Record metrics
		s.config.Metrics.RecordHistogram("transaction.duration", duration.Seconds(), map[string]string{
			"status":          "rolled_back",
			"isolation_level": config.IsolationLevel,
		})
		s.config.Metrics.IncrementCounterWithLabels("transaction.rolled_back", 1, map[string]string{
			"isolation_level": config.IsolationLevel,
		})

		// Cancel context
		cancel()
	}

	return ctx, commit, rollback
}
