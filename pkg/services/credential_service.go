package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository/credential"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	"github.com/google/uuid"
)

// CredentialService handles user credential operations with encryption
type CredentialService struct {
	repo       credential.Repository
	encryption *security.EncryptionService
	logger     observability.Logger
}

// NewCredentialService creates a new credential service
func NewCredentialService(
	repo credential.Repository,
	encryption *security.EncryptionService,
	logger observability.Logger,
) *CredentialService {
	return &CredentialService{
		repo:       repo,
		encryption: encryption,
		logger:     logger,
	}
}

// StoreCredentials stores encrypted credentials for a user
func (s *CredentialService) StoreCredentials(
	ctx context.Context,
	tenantID, userID string,
	payload *models.CredentialPayload,
	ipAddress, userAgent string,
) (*models.CredentialResponse, error) {
	// Validate inputs
	if tenantID == "" || userID == "" {
		return nil, fmt.Errorf("tenant_id and user_id are required")
	}
	if len(payload.Credentials) == 0 {
		return nil, fmt.Errorf("credentials are required")
	}

	// Convert credentials map to JSON
	credentialsJSON, err := json.Marshal(payload.Credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Encrypt credentials with tenant-specific key
	encrypted, err := s.encryption.EncryptCredential(string(credentialsJSON), tenantID)
	if err != nil {
		s.logger.Error("Failed to encrypt credentials", map[string]interface{}{
			"error":        err.Error(),
			"tenant_id":    tenantID,
			"user_id":      userID,
			"service_type": payload.ServiceType,
		})
		return nil, fmt.Errorf("failed to encrypt credentials: %w", err)
	}

	// Create credential record
	cred := &models.UserCredential{
		ID:                   uuid.New().String(),
		TenantID:             tenantID,
		UserID:               userID,
		ServiceType:          payload.ServiceType,
		EncryptedCredentials: encrypted,
		EncryptionKeyVersion: 1, // Track key version for future rotation
		IsActive:             true,
		Metadata:             payload.Metadata,
		ExpiresAt:            payload.ExpiresAt,
	}

	// Store in database (upsert)
	if err := s.repo.Create(ctx, cred); err != nil {
		s.logger.Error("Failed to store credentials", map[string]interface{}{
			"error":        err.Error(),
			"tenant_id":    tenantID,
			"user_id":      userID,
			"service_type": payload.ServiceType,
		})
		return nil, fmt.Errorf("failed to store credentials: %w", err)
	}

	// Log audit trail
	audit := &models.UserCredentialAudit{
		ID:           uuid.New().String(),
		CredentialID: cred.ID,
		TenantID:     tenantID,
		UserID:       userID,
		ServiceType:  payload.ServiceType,
		Operation:    "created",
		Success:      true,
		IPAddress:    stringToNullString(ipAddress),
		UserAgent:    stringToNullString(userAgent),
	}
	if err := s.repo.AuditLog(ctx, audit); err != nil {
		// Don't fail operation if audit fails, just log
		s.logger.Warn("Failed to log audit trail", map[string]interface{}{
			"error": err.Error(),
		})
	}

	s.logger.Info("Credentials stored successfully", map[string]interface{}{
		"tenant_id":    tenantID,
		"user_id":      userID,
		"service_type": payload.ServiceType,
	})

	return cred.ToResponse(), nil
}

// GetCredentials retrieves and decrypts credentials for a user
func (s *CredentialService) GetCredentials(
	ctx context.Context,
	tenantID, userID string,
	serviceType models.ServiceType,
) (*models.DecryptedCredentials, error) {
	// Fetch encrypted credentials from database
	cred, err := s.repo.Get(ctx, tenantID, userID, serviceType)
	if err != nil {
		return nil, fmt.Errorf("credentials not found: %w", err)
	}

	// Check if expired
	if cred.ExpiresAt != nil && time.Now().After(*cred.ExpiresAt) {
		return nil, fmt.Errorf("credentials have expired")
	}

	// Decrypt credentials
	decrypted, err := s.encryption.DecryptCredential(cred.EncryptedCredentials, tenantID)
	if err != nil {
		s.logger.Error("Failed to decrypt credentials", map[string]interface{}{
			"error":        err.Error(),
			"tenant_id":    tenantID,
			"user_id":      userID,
			"service_type": serviceType,
		})
		return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	// Parse JSON
	var credMap map[string]string
	if err := json.Unmarshal([]byte(decrypted), &credMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}

	// Record usage
	if err := s.repo.RecordUsage(ctx, cred.ID); err != nil {
		// Don't fail operation if usage tracking fails
		s.logger.Warn("Failed to record credential usage", map[string]interface{}{
			"error": err.Error(),
		})
	}

	return &models.DecryptedCredentials{
		ServiceType: serviceType,
		Credentials: credMap,
		Metadata:    cred.Metadata,
	}, nil
}

// ListCredentials returns all configured credentials for a user (without sensitive data)
func (s *CredentialService) ListCredentials(
	ctx context.Context,
	tenantID, userID string,
) ([]*models.CredentialResponse, error) {
	// Fetch credentials from database
	creds, err := s.repo.List(ctx, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}

	// Convert to response format (strips encrypted data)
	responses := make([]*models.CredentialResponse, len(creds))
	for i, cred := range creds {
		responses[i] = cred.ToResponse()
	}

	return responses, nil
}

// DeleteCredentials removes credentials for a specific service
func (s *CredentialService) DeleteCredentials(
	ctx context.Context,
	tenantID, userID string,
	serviceType models.ServiceType,
	ipAddress, userAgent string,
) error {
	// Get credential ID for audit trail
	cred, err := s.repo.Get(ctx, tenantID, userID, serviceType)
	if err != nil {
		return fmt.Errorf("credentials not found: %w", err)
	}

	// Soft delete credentials
	if err := s.repo.Delete(ctx, tenantID, userID, serviceType); err != nil {
		return fmt.Errorf("failed to delete credentials: %w", err)
	}

	// Log audit trail
	audit := &models.UserCredentialAudit{
		ID:           uuid.New().String(),
		CredentialID: cred.ID,
		TenantID:     tenantID,
		UserID:       userID,
		ServiceType:  serviceType,
		Operation:    "deleted",
		Success:      true,
		IPAddress:    stringToNullString(ipAddress),
		UserAgent:    stringToNullString(userAgent),
	}
	if err := s.repo.AuditLog(ctx, audit); err != nil {
		// Don't fail operation if audit fails, just log
		s.logger.Warn("Failed to log audit trail", map[string]interface{}{
			"error": err.Error(),
		})
	}

	s.logger.Info("Credentials deleted successfully", map[string]interface{}{
		"tenant_id":    tenantID,
		"user_id":      userID,
		"service_type": serviceType,
	})

	return nil
}

// GetAllUserCredentials retrieves all credentials for a user and decrypts them
// This is used by edge-mcp to populate passthrough auth
func (s *CredentialService) GetAllUserCredentials(
	ctx context.Context,
	tenantID, userID string,
) (map[models.ServiceType]*models.DecryptedCredentials, error) {
	// Fetch all credentials for user
	creds, err := s.repo.List(ctx, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}

	// Decrypt each credential
	decryptedMap := make(map[models.ServiceType]*models.DecryptedCredentials)
	for _, cred := range creds {
		// Skip expired credentials
		if cred.ExpiresAt != nil && time.Now().After(*cred.ExpiresAt) {
			continue
		}

		// Decrypt credentials
		decrypted, err := s.encryption.DecryptCredential(cred.EncryptedCredentials, tenantID)
		if err != nil {
			s.logger.Error("Failed to decrypt credential", map[string]interface{}{
				"error":        err.Error(),
				"service_type": cred.ServiceType,
			})
			continue // Skip this credential but continue with others
		}

		// Parse JSON
		var credMap map[string]string
		if err := json.Unmarshal([]byte(decrypted), &credMap); err != nil {
			s.logger.Error("Failed to unmarshal credential", map[string]interface{}{
				"error":        err.Error(),
				"service_type": cred.ServiceType,
			})
			continue
		}

		decryptedMap[cred.ServiceType] = &models.DecryptedCredentials{
			ServiceType: cred.ServiceType,
			Credentials: credMap,
			Metadata:    cred.Metadata,
		}

		// Record usage asynchronously (don't block)
		go func(id string) {
			if err := s.repo.RecordUsage(context.Background(), id); err != nil {
				s.logger.Warn("Failed to record credential usage", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}(cred.ID)
	}

	return decryptedMap, nil
}

// ValidateCredentials tests if credentials work by making a test API call
// This is called before storing to ensure credentials are valid
func (s *CredentialService) ValidateCredentials(
	ctx context.Context,
	serviceType models.ServiceType,
	credentials map[string]string,
) error {
	// TODO: Implement validation for each service type
	// For now, just check that credentials are not empty
	switch serviceType {
	case models.ServiceTypeGitHub:
		if _, ok := credentials["token"]; !ok {
			return fmt.Errorf("github token is required")
		}
		// TODO: Make test API call to GitHub to validate token

	case models.ServiceTypeHarness:
		if _, ok := credentials["token"]; !ok {
			if _, ok := credentials["api_key"]; !ok {
				return fmt.Errorf("harness token or api_key is required")
			}
		}
		// TODO: Make test API call to Harness to validate token

	case models.ServiceTypeAWS:
		if _, ok := credentials["access_key"]; !ok {
			return fmt.Errorf("aws access_key is required")
		}
		if _, ok := credentials["secret_key"]; !ok {
			return fmt.Errorf("aws secret_key is required")
		}
		// TODO: Make test AWS call to validate credentials

	default:
		// For generic services, just ensure credentials exist
		if len(credentials) == 0 {
			return fmt.Errorf("credentials cannot be empty")
		}
	}

	return nil
}

// Helper function to convert string to sql.NullString
func stringToNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}
