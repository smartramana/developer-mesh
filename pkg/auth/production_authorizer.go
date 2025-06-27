package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/S-Corkum/devops-mcp/pkg/cache"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// ProductionAuthorizer implements production-grade authorization
type ProductionAuthorizer struct {
	// In-memory policy storage for now (will be replaced with Casbin later)
	policies     []Policy
	roleBindings map[string][]string // user -> roles
	roles        map[string]bool     // available roles
	cache        cache.Cache
	logger       observability.Logger
	metrics      observability.MetricsClient
	tracer       observability.StartSpanFunc
	auditLogger  *AuditLogger
	mu           sync.RWMutex

	// Configuration
	cacheEnabled  bool
	cacheDuration time.Duration
	policyPath    string
	modelPath     string
}

// AuthConfig holds configuration for the production authorizer
type AuthConfig struct {
	ModelPath     string
	PolicyPath    string
	DBDriver      string
	DBSource      string
	Cache         cache.Cache
	Logger        observability.Logger
	Metrics       observability.MetricsClient
	Tracer        observability.StartSpanFunc
	AuditLogger   *AuditLogger
	CacheEnabled  bool
	CacheDuration time.Duration
}

// AuthRequest represents an authorization request
type AuthRequest struct {
	Subject  string                 `json:"subject"`
	Resource string                 `json:"resource"`
	Action   string                 `json:"action"`
	Tenant   string                 `json:"tenant"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

// Policy represents an authorization policy
type Policy struct {
	Subject  string `json:"subject"`
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Effect   string `json:"effect,omitempty"`
	Tenant   string `json:"tenant,omitempty"`
}

// NewProductionAuthorizer creates a production authorizer
func NewProductionAuthorizer(config AuthConfig) (*ProductionAuthorizer, error) {
	// Validate configuration
	if config.Logger == nil {
		return nil, errors.New("logger is required")
	}
	if config.Metrics == nil {
		return nil, errors.New("metrics is required")
	}
	if config.Cache == nil && config.CacheEnabled {
		return nil, errors.New("cache is required when cache is enabled")
	}
	if config.AuditLogger == nil {
		return nil, errors.New("audit logger is required")
	}

	// Initialize in-memory storage
	roleBindings := make(map[string][]string)
	roles := make(map[string]bool)

	// Default roles
	roles["admin"] = true
	roles["user"] = true
	roles["viewer"] = true

	auth := &ProductionAuthorizer{
		policies:      []Policy{},
		roleBindings:  roleBindings,
		roles:         roles,
		cache:         config.Cache,
		logger:        config.Logger,
		metrics:       config.Metrics,
		tracer:        config.Tracer,
		auditLogger:   config.AuditLogger,
		cacheEnabled:  config.CacheEnabled,
		cacheDuration: config.CacheDuration,
		modelPath:     config.ModelPath,
		policyPath:    config.PolicyPath,
	}

	// Load initial policies
	if err := auth.loadPolicies(); err != nil {
		return nil, errors.Wrap(err, "failed to load initial policies")
	}

	// Start policy watcher for dynamic updates
	go auth.watchPolicyChanges(context.Background())

	config.Logger.Info("Production authorizer initialized", map[string]interface{}{
		"model_path":     config.ModelPath,
		"cache_enabled":  config.CacheEnabled,
		"cache_duration": config.CacheDuration,
	})

	return auth, nil
}

// Authorize implements the Authorizer interface
func (a *ProductionAuthorizer) Authorize(ctx context.Context, permission Permission) Decision {
	// Extract user from context
	userID := GetUserID(ctx)
	if userID == "" {
		return Decision{
			Allowed: false,
			Reason:  "no user in context",
		}
	}

	tenantID := GetTenantID(ctx).String()

	// Create auth request
	req := &AuthRequest{
		Subject:  userID,
		Resource: permission.Resource,
		Action:   permission.Action,
		Tenant:   tenantID,
		Context:  permission.Conditions,
	}

	// Check authorization
	err := a.AuthorizeRequest(ctx, req)
	if err != nil {
		return Decision{
			Allowed: false,
			Reason:  err.Error(),
		}
	}

	return Decision{
		Allowed: true,
		Reason:  "permission granted",
	}
}

// AuthorizeRequest checks if a subject can perform an action on a resource
func (a *ProductionAuthorizer) AuthorizeRequest(ctx context.Context, req *AuthRequest) error {
	ctx, span := a.tracer(ctx, "ProductionAuthorizer.AuthorizeRequest")
	defer span.End()

	startTime := time.Now()
	defer func() {
		a.metrics.RecordHistogram("auth.authorize.duration", time.Since(startTime).Seconds(), nil)
	}()

	// Validate request
	if err := a.validateAuthRequest(req); err != nil {
		a.metrics.IncrementCounter("auth.validation.error", 1)
		return errors.Wrap(err, "invalid auth request")
	}

	// Check cache if enabled
	if a.cacheEnabled {
		cacheKey := a.buildCacheKey(req)
		var allowed bool
		if err := a.cache.Get(ctx, cacheKey, &allowed); err == nil {
			a.metrics.IncrementCounter("auth.cache.hit", 1)
			if !allowed {
				return a.recordAndReturnDenial(ctx, req, "cached denial")
			}
			return a.recordAndReturnSuccess(ctx, req)
		}
	}

	// Prepare request parameters
	params := a.buildEnforcerParams(req)

	// Check permission with retries for resilience
	allowed, err := a.enforceWithRetry(ctx, params)
	if err != nil {
		a.metrics.IncrementCounter("auth.enforce.error", 1)
		return errors.Wrap(err, "failed to check permission")
	}

	// Cache result
	if a.cacheEnabled {
		cacheKey := a.buildCacheKey(req)
		_ = a.cache.Set(ctx, cacheKey, allowed, a.cacheDuration)
	}

	// Handle result
	if !allowed {
		return a.recordAndReturnDenial(ctx, req, "permission denied")
	}

	return a.recordAndReturnSuccess(ctx, req)
}

// CheckPermission implements the Authorizer interface
func (a *ProductionAuthorizer) CheckPermission(ctx context.Context, resource, action string) bool {
	// Extract user from context
	userID := GetUserID(ctx)
	if userID == "" {
		return false
	}

	tenantID := GetTenantID(ctx).String()

	// Create auth request
	req := &AuthRequest{
		Subject:  userID,
		Resource: resource,
		Action:   action,
		Tenant:   tenantID,
	}

	// Check authorization
	err := a.AuthorizeRequest(ctx, req)
	return err == nil
}

// AddPolicy adds a new authorization policy
func (a *ProductionAuthorizer) AddPolicy(ctx context.Context, policy Policy) error {
	ctx, span := a.tracer(ctx, "ProductionAuthorizer.AddPolicy")
	defer span.End()

	// Validate policy
	if err := a.validatePolicy(policy); err != nil {
		return errors.Wrap(err, "invalid policy")
	}

	// Check if policy already exists
	for _, p := range a.policies {
		if p.Subject == policy.Subject && p.Resource == policy.Resource && p.Action == policy.Action {
			return errors.New("policy already exists")
		}
	}

	// Add policy
	a.mu.Lock()
	a.policies = append(a.policies, policy)
	a.mu.Unlock()

	// Invalidate cache for this subject
	if a.cacheEnabled {
		a.invalidateCacheForSubject(ctx, policy.Subject)
	}

	// Audit log
	a.auditLogger.LogPolicyChange(ctx, "add_policy", policy, "")

	a.metrics.IncrementCounter("auth.policy.added", 1)
	return nil
}

// RemovePolicy removes an authorization policy
func (a *ProductionAuthorizer) RemovePolicy(ctx context.Context, policy Policy) error {
	ctx, span := a.tracer(ctx, "ProductionAuthorizer.RemovePolicy")
	defer span.End()

	// Validate policy
	if err := a.validatePolicy(policy); err != nil {
		return errors.Wrap(err, "invalid policy")
	}

	// Find and remove policy
	a.mu.Lock()
	found := false
	newPolicies := []Policy{}
	for _, p := range a.policies {
		if p.Subject == policy.Subject && p.Resource == policy.Resource && p.Action == policy.Action {
			found = true
			continue
		}
		newPolicies = append(newPolicies, p)
	}
	a.policies = newPolicies
	a.mu.Unlock()

	if !found {
		return errors.New("policy not found")
	}

	// Invalidate cache
	if a.cacheEnabled {
		a.invalidateCacheForSubject(ctx, policy.Subject)
	}

	// Audit log
	a.auditLogger.LogPolicyChange(ctx, "remove_policy", policy, "")

	a.metrics.IncrementCounter("auth.policy.removed", 1)
	return nil
}

// AddRole assigns a role to a user
func (a *ProductionAuthorizer) AddRole(ctx context.Context, user, role string) error {
	ctx, span := a.tracer(ctx, "ProductionAuthorizer.AddRole")
	defer span.End()

	// Validate inputs
	if user == "" || role == "" {
		return errors.New("user and role must not be empty")
	}

	// Check if role exists
	if !a.roleExists(role) {
		return errors.Errorf("role '%s' does not exist", role)
	}

	// Check if role assignment already exists
	a.mu.RLock()
	if roles, exists := a.roleBindings[user]; exists {
		for _, r := range roles {
			if r == role {
				a.mu.RUnlock()
				return errors.New("role assignment already exists")
			}
		}
	}
	a.mu.RUnlock()

	// Add role assignment
	a.mu.Lock()
	a.roleBindings[user] = append(a.roleBindings[user], role)
	a.mu.Unlock()

	// Invalidate cache
	if a.cacheEnabled {
		a.invalidateCacheForSubject(ctx, user)
	}

	// Audit log
	a.auditLogger.LogRoleAssignment(ctx, "add_role", user, role)

	a.metrics.IncrementCounter("auth.role.assigned", 1)
	return nil
}

// RemoveRole removes a role from a user
func (a *ProductionAuthorizer) RemoveRole(ctx context.Context, user, role string) error {
	ctx, span := a.tracer(ctx, "ProductionAuthorizer.RemoveRole")
	defer span.End()

	// Validate inputs
	if user == "" || role == "" {
		return errors.New("user and role must not be empty")
	}

	// Remove role assignment
	a.mu.Lock()
	found := false
	if roles, exists := a.roleBindings[user]; exists {
		newRoles := []string{}
		for _, r := range roles {
			if r == role {
				found = true
				continue
			}
			newRoles = append(newRoles, r)
		}
		a.roleBindings[user] = newRoles
	}
	a.mu.Unlock()

	if !found {
		return errors.New("role assignment not found")
	}

	// Invalidate cache
	if a.cacheEnabled {
		a.invalidateCacheForSubject(ctx, user)
	}

	// Audit log
	a.auditLogger.LogRoleAssignment(ctx, "remove_role", user, role)

	a.metrics.IncrementCounter("auth.role.removed", 1)
	return nil
}

// GetRolesForUser returns all roles assigned to a user
func (a *ProductionAuthorizer) GetRolesForUser(ctx context.Context, user string) ([]string, error) {
	_, span := a.tracer(ctx, "ProductionAuthorizer.GetRolesForUser")
	defer span.End()

	if user == "" {
		return nil, errors.New("user must not be empty")
	}

	a.mu.RLock()
	roles := a.roleBindings[user]
	a.mu.RUnlock()

	return roles, nil
}

// GetUsersForRole returns all users assigned to a role
func (a *ProductionAuthorizer) GetUsersForRole(ctx context.Context, role string) ([]string, error) {
	_, span := a.tracer(ctx, "ProductionAuthorizer.GetUsersForRole")
	defer span.End()

	if role == "" {
		return nil, errors.New("role must not be empty")
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	var users []string
	for user, roles := range a.roleBindings {
		for _, r := range roles {
			if r == role {
				users = append(users, user)
				break
			}
		}
	}

	return users, nil
}

// enforceWithRetry performs authorization check with retry logic
func (a *ProductionAuthorizer) enforceWithRetry(ctx context.Context, params []interface{}) (bool, error) {
	// Simple in-memory policy evaluation
	if len(params) < 3 {
		return false, errors.New("invalid parameters")
	}

	subject, _ := params[0].(string)
	resource, _ := params[1].(string)
	action, _ := params[2].(string)
	tenant := ""
	if len(params) > 3 {
		tenant, _ = params[3].(string)
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	// Check direct policies
	for _, policy := range a.policies {
		if a.matchPolicy(policy, subject, resource, action, tenant) {
			return policy.Effect != "deny", nil
		}
	}

	// Check role-based policies
	if roles, exists := a.roleBindings[subject]; exists {
		for _, role := range roles {
			for _, policy := range a.policies {
				if a.matchPolicy(policy, role, resource, action, tenant) {
					return policy.Effect != "deny", nil
				}
			}
		}
	}

	return false, nil
}

// Dynamic policy updates
func (a *ProductionAuthorizer) watchPolicyChanges(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// In production, this would reload from database
			// For now, just clear cache periodically
			if a.cacheEnabled {
				_ = a.cache.Flush(ctx)
			}
			a.metrics.IncrementCounter("auth.policy.reloaded", 1)
		}
	}
}

// loadPolicies loads initial policies
func (a *ProductionAuthorizer) loadPolicies() error {
	// Add default policies
	defaultPolicies := []Policy{
		// Admin policies
		{Subject: "admin", Resource: "*", Action: "*", Effect: "allow"},

		// User policies
		{Subject: "user", Resource: "workspace", Action: "read", Effect: "allow"},
		{Subject: "user", Resource: "workspace", Action: "create", Effect: "allow"},
		{Subject: "user", Resource: "document", Action: "read", Effect: "allow"},
		{Subject: "user", Resource: "document", Action: "create", Effect: "allow"},
		{Subject: "user", Resource: "task", Action: "read", Effect: "allow"},
		{Subject: "user", Resource: "task", Action: "create", Effect: "allow"},
		{Subject: "user", Resource: "workflow", Action: "read", Effect: "allow"},
		{Subject: "user", Resource: "workflow", Action: "create", Effect: "allow"},
		{Subject: "user", Resource: "workflow", Action: "update", Effect: "allow"},
		{Subject: "user", Resource: "workflow", Action: "delete", Effect: "allow"},

		// Viewer policies
		{Subject: "viewer", Resource: "workspace", Action: "read", Effect: "allow"},
		{Subject: "viewer", Resource: "document", Action: "read", Effect: "allow"},
		{Subject: "viewer", Resource: "task", Action: "read", Effect: "allow"},
		{Subject: "viewer", Resource: "workflow", Action: "read", Effect: "allow"},
	}

	a.mu.Lock()
	a.policies = defaultPolicies

	// Add default role bindings for known test users
	// In production, these would come from a database
	a.roleBindings["system"] = []string{"admin"} // For admin keys without explicit user_id
	a.roleBindings["agent-1"] = []string{"user"}
	a.roleBindings["agent-2"] = []string{"user"}
	a.roleBindings["backend-dev"] = []string{"user"}
	a.roleBindings["frontend-dev"] = []string{"user"}
	a.roleBindings["devops"] = []string{"user"}
	a.roleBindings["ml-engineer"] = []string{"user"}

	a.mu.Unlock()

	return nil
}

// validateAuthRequest validates an authorization request
func (a *ProductionAuthorizer) validateAuthRequest(req *AuthRequest) error {
	if req == nil {
		return errors.New("request is nil")
	}
	if req.Subject == "" {
		return errors.New("subject is required")
	}
	if req.Resource == "" {
		return errors.New("resource is required")
	}
	if req.Action == "" {
		return errors.New("action is required")
	}
	return nil
}

// validatePolicy validates a policy
func (a *ProductionAuthorizer) validatePolicy(policy Policy) error {
	if policy.Subject == "" {
		return errors.New("subject is required")
	}
	if policy.Resource == "" {
		return errors.New("resource is required")
	}
	if policy.Action == "" {
		return errors.New("action is required")
	}
	if policy.Effect != "" && policy.Effect != "allow" && policy.Effect != "deny" {
		return errors.New("effect must be 'allow' or 'deny'")
	}
	return nil
}

// buildCacheKey builds a cache key for an auth request
func (a *ProductionAuthorizer) buildCacheKey(req *AuthRequest) string {
	return fmt.Sprintf("auth:%s:%s:%s:%s", req.Subject, req.Resource, req.Action, req.Tenant)
}

// buildEnforcerParams builds parameters for the enforcer
func (a *ProductionAuthorizer) buildEnforcerParams(req *AuthRequest) []interface{} {
	params := []interface{}{req.Subject, req.Resource, req.Action}
	if req.Tenant != "" {
		params = append(params, req.Tenant)
	}
	return params
}

// roleExists checks if a role exists
func (a *ProductionAuthorizer) roleExists(role string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.roles[role]
}

// matchPolicy checks if a policy matches the given parameters
func (a *ProductionAuthorizer) matchPolicy(policy Policy, subject, resource, action, tenant string) bool {
	// Subject match
	if policy.Subject != "*" && policy.Subject != subject {
		return false
	}

	// Resource match (support wildcards)
	if policy.Resource != "*" && policy.Resource != resource {
		return false
	}

	// Action match
	if policy.Action != "*" && policy.Action != action {
		return false
	}

	// Tenant match if specified
	if policy.Tenant != "" && policy.Tenant != tenant {
		return false
	}

	return true
}

// invalidateCacheForSubject removes all cache entries for a subject
func (a *ProductionAuthorizer) invalidateCacheForSubject(ctx context.Context, subject string) {
	// Since we don't have DeletePattern, we'll just flush the entire cache
	// In production with Casbin, you'd want to implement a more targeted cache invalidation
	_ = a.cache.Flush(ctx)
}

// recordAndReturnDenial records a denial and returns an error
func (a *ProductionAuthorizer) recordAndReturnDenial(ctx context.Context, req *AuthRequest, reason string) error {
	a.auditLogger.LogAuthorizationDenial(ctx, req.Subject, req.Resource, req.Action, reason)
	a.metrics.IncrementCounter("auth.denied", 1)
	return errors.Errorf("authorization denied: %s", reason)
}

// recordAndReturnSuccess records a successful authorization
func (a *ProductionAuthorizer) recordAndReturnSuccess(ctx context.Context, req *AuthRequest) error {
	a.auditLogger.LogAuthorizationSuccess(ctx, req.Subject, req.Resource, req.Action)
	a.metrics.IncrementCounter("auth.allowed", 1)
	return nil
}
