package repository

import (
	"context"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/jmoiron/sqlx"
)

// WebhookEventRepository defines the interface for webhook event storage
type WebhookEventRepository interface {
	// Create stores a new webhook event
	Create(ctx context.Context, event *models.WebhookEvent) error

	// GetByID retrieves a webhook event by ID
	GetByID(ctx context.Context, id string) (*models.WebhookEvent, error)

	// GetByToolID retrieves webhook events for a specific tool
	GetByToolID(ctx context.Context, toolID string, limit, offset int) ([]*models.WebhookEvent, error)

	// GetByTenantID retrieves webhook events for a specific tenant
	GetByTenantID(ctx context.Context, tenantID string, limit, offset int) ([]*models.WebhookEvent, error)

	// GetPending retrieves pending webhook events for processing
	GetPending(ctx context.Context, limit int) ([]*models.WebhookEvent, error)

	// UpdateStatus updates the status of a webhook event
	UpdateStatus(ctx context.Context, id, status string, processedAt *time.Time, errorMsg string) error

	// Delete removes a webhook event
	Delete(ctx context.Context, id string) error

	// DeleteOlderThan removes webhook events older than the specified time
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}

// webhookEventRepository is the SQL implementation
type webhookEventRepository struct {
	db *sqlx.DB
}

// NewWebhookEventRepository creates a new webhook event repository
func NewWebhookEventRepository(db *sqlx.DB) WebhookEventRepository {
	return &webhookEventRepository{db: db}
}

// Create stores a new webhook event
func (r *webhookEventRepository) Create(ctx context.Context, event *models.WebhookEvent) error {
	query := `
		INSERT INTO webhook_events (
			id, tool_id, tenant_id, event_type, payload, headers, 
			source_ip, received_at, status, metadata
		) VALUES (
			:id, :tool_id, :tenant_id, :event_type, :payload, :headers,
			:source_ip, :received_at, :status, :metadata
		)`

	_, err := r.db.NamedExecContext(ctx, query, event)
	return err
}

// GetByID retrieves a webhook event by ID
func (r *webhookEventRepository) GetByID(ctx context.Context, id string) (*models.WebhookEvent, error) {
	var event models.WebhookEvent
	query := `SELECT * FROM webhook_events WHERE id = $1`
	err := r.db.GetContext(ctx, &event, query, id)
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// GetByToolID retrieves webhook events for a specific tool
func (r *webhookEventRepository) GetByToolID(ctx context.Context, toolID string, limit, offset int) ([]*models.WebhookEvent, error) {
	var events []*models.WebhookEvent
	query := `
		SELECT * FROM webhook_events 
		WHERE tool_id = $1 
		ORDER BY received_at DESC 
		LIMIT $2 OFFSET $3`

	err := r.db.SelectContext(ctx, &events, query, toolID, limit, offset)
	return events, err
}

// GetByTenantID retrieves webhook events for a specific tenant
func (r *webhookEventRepository) GetByTenantID(ctx context.Context, tenantID string, limit, offset int) ([]*models.WebhookEvent, error) {
	var events []*models.WebhookEvent
	query := `
		SELECT * FROM webhook_events 
		WHERE tenant_id = $1 
		ORDER BY received_at DESC 
		LIMIT $2 OFFSET $3`

	err := r.db.SelectContext(ctx, &events, query, tenantID, limit, offset)
	return events, err
}

// GetPending retrieves pending webhook events for processing
func (r *webhookEventRepository) GetPending(ctx context.Context, limit int) ([]*models.WebhookEvent, error) {
	var events []*models.WebhookEvent
	query := `
		SELECT * FROM webhook_events 
		WHERE status = 'pending' 
		ORDER BY received_at ASC 
		LIMIT $1`

	err := r.db.SelectContext(ctx, &events, query, limit)
	return events, err
}

// UpdateStatus updates the status of a webhook event
func (r *webhookEventRepository) UpdateStatus(ctx context.Context, id, status string, processedAt *time.Time, errorMsg string) error {
	query := `
		UPDATE webhook_events 
		SET status = $2, processed_at = $3, error = $4 
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id, status, processedAt, errorMsg)
	return err
}

// Delete removes a webhook event
func (r *webhookEventRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM webhook_events WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// DeleteOlderThan removes webhook events older than the specified time
func (r *webhookEventRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	query := `DELETE FROM webhook_events WHERE received_at < $1`
	result, err := r.db.ExecContext(ctx, query, before)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
