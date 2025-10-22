package credential

import (
	"context"

	"github.com/developer-mesh/developer-mesh/pkg/models"
)

// Repository defines the interface for user credential operations
type Repository interface {
	// Create stores new encrypted credentials for a user
	Create(ctx context.Context, credential *models.UserCredential) error

	// Get retrieves credentials for a specific service
	Get(ctx context.Context, tenantID, userID string, serviceType models.ServiceType) (*models.UserCredential, error)

	// GetByID retrieves credentials by ID
	GetByID(ctx context.Context, id string) (*models.UserCredential, error)

	// List retrieves all credentials for a user
	List(ctx context.Context, tenantID, userID string) ([]*models.UserCredential, error)

	// Update updates existing credentials
	Update(ctx context.Context, credential *models.UserCredential) error

	// Delete removes credentials (soft delete by setting is_active=false)
	Delete(ctx context.Context, tenantID, userID string, serviceType models.ServiceType) error

	// HardDelete permanently removes credentials (for compliance/user deletion)
	HardDelete(ctx context.Context, id string) error

	// RecordUsage updates last_used_at timestamp
	RecordUsage(ctx context.Context, id string) error

	// ListByTenant retrieves all credentials for a tenant (admin operation)
	ListByTenant(ctx context.Context, tenantID string) ([]*models.UserCredential, error)

	// AuditLog records an audit entry for credential operation
	AuditLog(ctx context.Context, audit *models.UserCredentialAudit) error

	// GetAuditHistory retrieves audit history for a credential
	GetAuditHistory(ctx context.Context, credentialID string, limit int) ([]*models.UserCredentialAudit, error)
}
