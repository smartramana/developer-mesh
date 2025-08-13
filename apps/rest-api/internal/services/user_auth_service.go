package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

// UserAuthService handles user authentication and management
type UserAuthService struct {
	db       *sqlx.DB
	authSvc  *auth.Service
	emailSvc EmailService
	logger   observability.Logger
}

// NewUserAuthService creates a new user auth service
func NewUserAuthService(db *sqlx.DB, authSvc *auth.Service, emailSvc EmailService, logger observability.Logger) *UserAuthService {
	return &UserAuthService{
		db:       db,
		authSvc:  authSvc,
		emailSvc: emailSvc,
		logger:   logger,
	}
}

// LoginRequest represents a user login request
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents a successful login response
type LoginResponse struct {
	User         *User         `json:"user"`
	Organization *Organization `json:"organization"`
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	ExpiresIn    int           `json:"expires_in"`
}

// InviteUserRequest represents a user invitation request
type InviteUserRequest struct {
	Email string `json:"email" binding:"required,email"`
	Name  string `json:"name" binding:"required,min=2,max=100"`
	Role  string `json:"role" binding:"required,oneof=admin member readonly"`
}

// AcceptInvitationRequest represents accepting an invitation
type AcceptInvitationRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

// PasswordResetRequest represents a password reset request
type PasswordResetRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// PasswordResetConfirmRequest represents confirming a password reset
type PasswordResetConfirmRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// Login authenticates a user and returns tokens
func (s *UserAuthService) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	// Fetch user with organization
	query := `
		SELECT 
			u.id, u.organization_id, u.tenant_id, u.email, u.name, 
			u.password_hash, u.role, u.status, u.email_verified,
			u.failed_login_attempts, u.locked_until,
			o.id as org_id, o.name as org_name, o.slug as org_slug,
			o.subscription_tier, o.max_users, o.max_agents
		FROM mcp.users u
		LEFT JOIN mcp.organizations o ON u.organization_id = o.id
		WHERE u.email = $1`

	var user struct {
		ID                  uuid.UUID      `db:"id"`
		OrganizationID      *uuid.UUID     `db:"organization_id"`
		TenantID            uuid.UUID      `db:"tenant_id"`
		Email               string         `db:"email"`
		Name                string         `db:"name"`
		PasswordHash        string         `db:"password_hash"`
		Role                string         `db:"role"`
		Status              string         `db:"status"`
		EmailVerified       bool           `db:"email_verified"`
		FailedLoginAttempts int            `db:"failed_login_attempts"`
		LockedUntil         *time.Time     `db:"locked_until"`
		OrgID               *uuid.UUID     `db:"org_id"`
		OrgName             sql.NullString `db:"org_name"`
		OrgSlug             sql.NullString `db:"org_slug"`
		SubscriptionTier    sql.NullString `db:"subscription_tier"`
		MaxUsers            sql.NullInt32  `db:"max_users"`
		MaxAgents           sql.NullInt32  `db:"max_agents"`
	}

	err := s.db.Get(&user, query, req.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			s.logFailedLogin(ctx, nil, nil, req.Email, "user_not_found")
			return nil, errors.New("invalid email or password")
		}
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	// Check if account is locked
	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		s.logFailedLogin(ctx, &user.ID, user.OrganizationID, req.Email, "account_locked")
		return nil, fmt.Errorf("account locked until %s", user.LockedUntil.Format(time.RFC3339))
	}

	// Check account status
	if user.Status != "active" {
		s.logFailedLogin(ctx, &user.ID, user.OrganizationID, req.Email, "account_"+user.Status)
		return nil, fmt.Errorf("account is %s", user.Status)
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		// Increment failed login attempts
		s.incrementFailedAttempts(ctx, user.ID)
		s.logFailedLogin(ctx, &user.ID, user.OrganizationID, req.Email, "invalid_password")
		return nil, errors.New("invalid email or password")
	}

	// Reset failed login attempts
	s.resetFailedAttempts(ctx, user.ID)

	// Update last login
	updateQuery := `UPDATE mcp.users SET last_login_at = $1, failed_login_attempts = 0 WHERE id = $2`
	if _, err := s.db.Exec(updateQuery, time.Now(), user.ID); err != nil {
		s.logger.Warn("Failed to update last login", map[string]interface{}{
			"user_id": user.ID,
			"error":   err.Error(),
		})
	}

	// Build response
	userResp := &User{
		ID:             user.ID,
		OrganizationID: user.OrganizationID,
		TenantID:       user.TenantID,
		Email:          user.Email,
		Name:           user.Name,
		Role:           user.Role,
		Status:         user.Status,
		EmailVerified:  user.EmailVerified,
	}

	var orgResp *Organization
	if user.OrgID != nil {
		orgResp = &Organization{
			ID:               *user.OrgID,
			Name:             user.OrgName.String,
			Slug:             user.OrgSlug.String,
			SubscriptionTier: user.SubscriptionTier.String,
			MaxUsers:         int(user.MaxUsers.Int32),
			MaxAgents:        int(user.MaxAgents.Int32),
		}
	}

	// Generate JWT tokens
	authUser := &auth.User{
		ID:       user.ID,
		TenantID: user.TenantID,
		Email:    user.Email,
		Scopes:   s.getScopesForRole(user.Role),
		AuthType: auth.TypeJWT,
	}

	accessToken, err := s.authSvc.GenerateJWT(ctx, authUser)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token
	refreshToken := generateSecureToken()
	refreshTokenHash := hashToken(refreshToken)

	// Store refresh token
	sessionID := uuid.New()
	sessionQuery := `
		INSERT INTO mcp.user_sessions (
			id, user_id, refresh_token_hash, expires_at, created_at, last_activity
		) VALUES ($1, $2, $3, $4, $5, $5)`

	expiresAt := time.Now().Add(30 * 24 * time.Hour) // 30 days
	if _, err := s.db.Exec(sessionQuery, sessionID, user.ID, refreshTokenHash, expiresAt, time.Now()); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Log successful login
	s.logSuccessfulLogin(ctx, &user.ID, user.OrganizationID, req.Email)

	return &LoginResponse{
		User:         userResp,
		Organization: orgResp,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    86400, // 24 hours in seconds
	}, nil
}

// InviteUser invites a new user to an organization
func (s *UserAuthService) InviteUser(ctx context.Context, inviterID uuid.UUID, orgID uuid.UUID, req *InviteUserRequest) error {
	// Get inviter details
	var inviter struct {
		Name     string    `db:"name"`
		Role     string    `db:"role"`
		TenantID uuid.UUID `db:"tenant_id"`
	}

	inviterQuery := `SELECT name, role, tenant_id FROM mcp.users WHERE id = $1 AND organization_id = $2`
	if err := s.db.Get(&inviter, inviterQuery, inviterID, orgID); err != nil {
		return fmt.Errorf("failed to get inviter details: %w", err)
	}

	// Check if inviter has permission (must be owner or admin)
	if inviter.Role != "owner" && inviter.Role != "admin" {
		return errors.New("insufficient permissions to invite users")
	}

	// Check organization user limit
	var userCount int
	countQuery := `SELECT COUNT(*) FROM mcp.users WHERE organization_id = $1`
	if err := s.db.Get(&userCount, countQuery, orgID); err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	var maxUsers int
	limitQuery := `SELECT max_users FROM mcp.organizations WHERE id = $1`
	if err := s.db.Get(&maxUsers, limitQuery, orgID); err != nil {
		return fmt.Errorf("failed to get user limit: %w", err)
	}

	if userCount >= maxUsers {
		return fmt.Errorf("organization has reached maximum user limit (%d)", maxUsers)
	}

	// Check if user already exists
	var exists bool
	existsQuery := `SELECT EXISTS(SELECT 1 FROM mcp.users WHERE email = $1)`
	if err := s.db.Get(&exists, existsQuery, req.Email); err != nil {
		return fmt.Errorf("failed to check user existence: %w", err)
	}
	if exists {
		return errors.New("user with this email already exists")
	}

	// Check for existing invitation
	existingInviteQuery := `SELECT EXISTS(SELECT 1 FROM mcp.user_invitations WHERE organization_id = $1 AND email = $2 AND accepted_at IS NULL)`
	if err := s.db.Get(&exists, existingInviteQuery, orgID, req.Email); err != nil {
		return fmt.Errorf("failed to check existing invitation: %w", err)
	}
	if exists {
		return errors.New("invitation already sent to this email")
	}

	// Create invitation token
	inviteToken := generateSecureToken()
	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7 days

	// Get organization name for email
	var orgName string
	orgQuery := `SELECT name FROM mcp.organizations WHERE id = $1`
	if err := s.db.Get(&orgName, orgQuery, orgID); err != nil {
		return fmt.Errorf("failed to get organization name: %w", err)
	}

	// Insert invitation
	inviteQuery := `
		INSERT INTO mcp.user_invitations (
			organization_id, email, role, invitation_token, 
			invited_by, expires_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`

	if _, err := s.db.Exec(inviteQuery,
		orgID, req.Email, req.Role, inviteToken,
		inviterID, expiresAt, time.Now()); err != nil {
		return fmt.Errorf("failed to create invitation: %w", err)
	}

	// Send invitation email (async)
	go func() {
		if s.emailSvc != nil {
			_ = s.emailSvc.SendInvitationEmail(context.Background(),
				req.Email, inviter.Name, orgName, inviteToken)
		}
	}()

	s.logger.Info("User invited successfully", map[string]interface{}{
		"inviter_id":      inviterID,
		"organization_id": orgID,
		"invited_email":   req.Email,
		"role":            req.Role,
	})

	return nil
}

// AcceptInvitation accepts a user invitation and creates the user account
func (s *UserAuthService) AcceptInvitation(ctx context.Context, req *AcceptInvitationRequest) (*User, error) {
	// Validate password
	if err := validatePassword(req.Password); err != nil {
		return nil, fmt.Errorf("password validation failed: %w", err)
	}

	// Find invitation
	var invitation struct {
		ID             uuid.UUID  `db:"id"`
		OrganizationID uuid.UUID  `db:"organization_id"`
		Email          string     `db:"email"`
		Role           string     `db:"role"`
		ExpiresAt      time.Time  `db:"expires_at"`
		AcceptedAt     *time.Time `db:"accepted_at"`
	}

	inviteQuery := `
		SELECT id, organization_id, email, role, expires_at, accepted_at
		FROM mcp.user_invitations
		WHERE invitation_token = $1`

	if err := s.db.Get(&invitation, inviteQuery, req.Token); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("invalid or expired invitation")
		}
		return nil, fmt.Errorf("failed to fetch invitation: %w", err)
	}

	// Check if already accepted
	if invitation.AcceptedAt != nil {
		return nil, errors.New("invitation already accepted")
	}

	// Check expiration
	if time.Now().After(invitation.ExpiresAt) {
		return nil, errors.New("invitation has expired")
	}

	// Get organization's tenant ID
	var tenantID uuid.UUID
	tenantQuery := `SELECT tenant_id FROM mcp.organization_tenants WHERE organization_id = $1 LIMIT 1`
	if err := s.db.Get(&tenantID, tenantQuery, invitation.OrganizationID); err != nil {
		return nil, fmt.Errorf("failed to get tenant ID: %w", err)
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Start transaction
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			s.logger.Error("Failed to rollback transaction", map[string]interface{}{"error": err.Error()})
		}
	}()

	// Create user
	userID := uuid.New()
	now := time.Now()

	user := &User{
		ID:                userID,
		OrganizationID:    &invitation.OrganizationID,
		TenantID:          tenantID,
		Email:             invitation.Email,
		Name:              invitation.Email, // Default to email, can be updated later
		PasswordHash:      string(passwordHash),
		Role:              invitation.Role,
		Status:            "active",
		EmailVerified:     true, // Already verified via invitation
		EmailVerifiedAt:   &now,
		PasswordChangedAt: &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	createUserQuery := `
		INSERT INTO mcp.users (
			id, organization_id, tenant_id, email, name, password_hash,
			role, status, email_verified, email_verified_at, 
			password_changed_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	if _, err := tx.Exec(createUserQuery,
		user.ID, user.OrganizationID, user.TenantID, user.Email, user.Name,
		user.PasswordHash, user.Role, user.Status, user.EmailVerified,
		user.EmailVerifiedAt, user.PasswordChangedAt, user.CreatedAt, user.UpdatedAt); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Mark invitation as accepted
	updateInviteQuery := `UPDATE mcp.user_invitations SET accepted_at = $1 WHERE id = $2`
	if _, err := tx.Exec(updateInviteQuery, time.Now(), invitation.ID); err != nil {
		return nil, fmt.Errorf("failed to update invitation: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("Invitation accepted successfully", map[string]interface{}{
		"user_id":         user.ID,
		"organization_id": invitation.OrganizationID,
		"email":           user.Email,
	})

	return user, nil
}

// Helper methods

func (s *UserAuthService) incrementFailedAttempts(ctx context.Context, userID uuid.UUID) {
	query := `
		UPDATE mcp.users 
		SET failed_login_attempts = failed_login_attempts + 1,
		    locked_until = CASE 
		        WHEN failed_login_attempts >= 4 THEN $1
		        ELSE locked_until
		    END
		WHERE id = $2`

	lockedUntil := time.Now().Add(15 * time.Minute)
	if _, err := s.db.Exec(query, lockedUntil, userID); err != nil {
		s.logger.Warn("Failed to increment login attempts", map[string]interface{}{
			"user_id": userID,
			"error":   err.Error(),
		})
	}
}

func (s *UserAuthService) resetFailedAttempts(ctx context.Context, userID uuid.UUID) {
	query := `UPDATE mcp.users SET failed_login_attempts = 0, locked_until = NULL WHERE id = $1`
	if _, err := s.db.Exec(query, userID); err != nil {
		s.logger.Warn("Failed to reset login attempts", map[string]interface{}{
			"user_id": userID,
			"error":   err.Error(),
		})
	}
}

func (s *UserAuthService) logFailedLogin(ctx context.Context, userID, orgID *uuid.UUID, email, reason string) {
	query := `
		INSERT INTO mcp.auth_audit_log (
			user_id, organization_id, event_type, event_details, success, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)`

	details := fmt.Sprintf(`{"email":"%s","reason":"%s"}`, email, reason)
	if _, err := s.db.Exec(query, userID, orgID, "login_failed", details, false, time.Now()); err != nil {
		s.logger.Warn("Failed to log failed login", map[string]interface{}{"error": err.Error()})
	}
}

func (s *UserAuthService) logSuccessfulLogin(ctx context.Context, userID, orgID *uuid.UUID, email string) {
	query := `
		INSERT INTO mcp.auth_audit_log (
			user_id, organization_id, event_type, event_details, success, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)`

	details := fmt.Sprintf(`{"email":"%s"}`, email)
	if _, err := s.db.Exec(query, userID, orgID, "login_success", details, true, time.Now()); err != nil {
		s.logger.Warn("Failed to log successful login", map[string]interface{}{"error": err.Error()})
	}
}

func (s *UserAuthService) getScopesForRole(role string) []string {
	switch role {
	case "owner":
		return []string{"read", "write", "admin", "billing"}
	case "admin":
		return []string{"read", "write", "admin"}
	case "member":
		return []string{"read", "write"}
	case "readonly":
		return []string{"read"}
	default:
		return []string{"read"}
	}
}
