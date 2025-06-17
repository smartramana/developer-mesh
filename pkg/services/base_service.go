package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/sony/gobreaker"

	"github.com/S-Corkum/devops-mcp/pkg/auth"
	"github.com/S-Corkum/devops-mcp/pkg/events"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/resilience"
	"github.com/S-Corkum/devops-mcp/pkg/rules"
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
	ctx = context.WithValue(ctx, "compensator", compensator)

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
	// TODO: Implement distributed transaction start
	// For now, return a mock transaction
	return &mockTransaction{
		id: uuid.New(),
	}, nil
}

// mockTransaction is a temporary implementation
type mockTransaction struct {
	id uuid.UUID
}

func (t *mockTransaction) Commit() error {
	return nil
}

func (t *mockTransaction) Rollback() error {
	return nil
}

func (t *mockTransaction) GetID() uuid.UUID {
	return t.id
}

// timePtr returns a pointer to a time
func timePtr(t time.Time) *time.Time {
	return &t
}