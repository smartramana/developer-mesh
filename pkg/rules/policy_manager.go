package rules

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/cache"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// PolicyManagerConfig for policy manager
type PolicyManagerConfig struct {
	CacheDuration  time.Duration `mapstructure:"cache_duration"`
	MaxPolicies    int           `mapstructure:"max_policies"`
	EnableCaching  bool          `mapstructure:"enable_caching"`
	HotReload      bool          `mapstructure:"hot_reload"`
	ReloadInterval time.Duration `mapstructure:"reload_interval"`
}

// policyManager implements the PolicyManager interface
type policyManager struct {
	mu         sync.RWMutex
	policies   map[string]*Policy
	cache      cache.Cache
	config     PolicyManagerConfig
	logger     observability.Logger
	metrics    observability.MetricsClient
	loadFunc   func() ([]Policy, error)
	stopChan   chan struct{}
}

// NewPolicyManager creates a new policy manager
func NewPolicyManager(config PolicyManagerConfig, cache cache.Cache, logger observability.Logger, metrics observability.MetricsClient) PolicyManager {
	if config.CacheDuration == 0 {
		config.CacheDuration = 5 * time.Minute
	}
	if config.MaxPolicies == 0 {
		config.MaxPolicies = 1000
	}
	if config.ReloadInterval == 0 {
		config.ReloadInterval = 30 * time.Second
	}

	return &policyManager{
		policies: make(map[string]*Policy),
		cache:    cache,
		config:   config,
		logger:   logger,
		metrics:  metrics,
		stopChan: make(chan struct{}),
	}
}

// LoadPolicies loads policies in bulk
func (pm *policyManager) LoadPolicies(ctx context.Context, policies []Policy) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Clear existing policies
	pm.policies = make(map[string]*Policy)

	// Load new policies
	successCount := 0
	for _, policy := range policies {
		if err := pm.addPolicyInternal(policy); err != nil {
			pm.logger.Warn("Failed to add policy", map[string]interface{}{
				"policy": policy.Name,
				"error":  err.Error(),
			})
			continue
		}
		successCount++
	}

	// Clear cache
	if pm.config.EnableCaching && pm.cache != nil {
		if err := pm.cache.Flush(ctx); err != nil {
			pm.logger.Warn("Failed to flush cache", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	pm.logger.Info("Policies loaded", map[string]interface{}{
		"total":   len(policies),
		"success": successCount,
		"failed":  len(policies) - successCount,
	})

	pm.metrics.RecordGauge("policies.total", float64(successCount), nil)
	return nil
}

// SetPolicyLoader sets the policy loader for dynamic loading
func (pm *policyManager) SetPolicyLoader(loader PolicyLoader) error {
	if loader == nil {
		return fmt.Errorf("policy loader cannot be nil")
	}
	
	pm.mu.Lock()
	pm.loadFunc = func() ([]Policy, error) {
		return loader.LoadPolicies(context.Background())
	}
	pm.mu.Unlock()
	
	return nil
}

// StartHotReload starts the hot reload goroutine
func (pm *policyManager) StartHotReload(ctx context.Context) error {
	if !pm.config.HotReload {
		return fmt.Errorf("hot reload is disabled")
	}
	
	pm.mu.Lock()
	if pm.loadFunc == nil {
		pm.mu.Unlock()
		return fmt.Errorf("no policy loader configured")
	}
	pm.mu.Unlock()

	go func() {
		ticker := time.NewTicker(pm.config.ReloadInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-pm.stopChan:
				return
			case <-ticker.C:
				if err := pm.reload(); err != nil {
					pm.logger.Error("Failed to reload policies", map[string]interface{}{
						"error": err.Error(),
					})
					pm.metrics.IncrementCounterWithLabels("policies.reload.error", 1.0, map[string]string{
						"error": "load_failed",
					})
				} else {
					pm.metrics.IncrementCounterWithLabels("policies.reload.success", 1.0, nil)
				}
			}
		}
	}()
	
	return nil
}

// StopHotReload stops the hot reload goroutine
func (pm *policyManager) StopHotReload() error {
	select {
	case <-pm.stopChan:
		// Already closed
		return fmt.Errorf("hot reload already stopped")
	default:
		close(pm.stopChan)
		return nil
	}
}

// reload reloads policies from external source
func (pm *policyManager) reload() error {
	if pm.loadFunc == nil {
		return nil
	}

	policies, err := pm.loadFunc()
	if err != nil {
		return errors.Wrap(err, "failed to load policies")
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Clear existing policies
	pm.policies = make(map[string]*Policy)

	// Load new policies
	for _, policy := range policies {
		if err := pm.addPolicyInternal(policy); err != nil {
			pm.logger.Warn("Failed to add policy", map[string]interface{}{
				"policy": policy.Name,
				"error":  err.Error(),
			})
			continue
		}
	}

	// Clear cache by flushing
	if pm.config.EnableCaching && pm.cache != nil {
		if err := pm.cache.Flush(context.Background()); err != nil {
			pm.logger.Warn("Failed to flush cache", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	pm.logger.Info("Policies reloaded", map[string]interface{}{
		"count": len(pm.policies),
	})

	return nil
}

// GetPolicy retrieves a policy by name
func (pm *policyManager) GetPolicy(ctx context.Context, policyName string) (*Policy, error) {
	start := time.Now()
	defer func() {
		pm.metrics.RecordDuration("policies.get.duration", time.Since(start))
	}()

	// Check cache first
	if pm.config.EnableCaching && pm.cache != nil {
		cacheKey := fmt.Sprintf("policy:%s", policyName)
		var policy Policy
		err := pm.cache.Get(ctx, cacheKey, &policy)
		if err == nil {
			pm.metrics.IncrementCounterWithLabels("policies.cache.hit", 1.0, nil)
			return &policy, nil
		}
		pm.metrics.IncrementCounterWithLabels("policies.cache.miss", 1.0, nil)
	}

	// Get from memory
	pm.mu.RLock()
	policy, exists := pm.policies[policyName]
	pm.mu.RUnlock()

	if !exists {
		pm.metrics.IncrementCounterWithLabels("policies.get.error", 1.0, map[string]string{
			"error": "not_found",
		})
		return nil, fmt.Errorf("policy not found: %s", policyName)
	}

	// Cache the policy
	if pm.config.EnableCaching && pm.cache != nil {
		cacheKey := fmt.Sprintf("policy:%s", policyName)
		if err := pm.cache.Set(ctx, cacheKey, policy, pm.config.CacheDuration); err != nil {
			pm.logger.Warn("Failed to cache policy", map[string]interface{}{
				"policy": policyName,
				"error":  err.Error(),
			})
		}
	}

	pm.metrics.IncrementCounterWithLabels("policies.get.success", 1.0, nil)
	return policy, nil
}

// GetDefaults retrieves defaults for a resource
func (pm *policyManager) GetDefaults(ctx context.Context, resource, resourceType string) (Defaults, error) {
	// Find policy that matches the resource
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, policy := range pm.policies {
		if policy.Resource == resource || policy.Resource == resourceType {
			return &defaults{
				data: policy.Defaults,
			}, nil
		}
	}

	// Return empty defaults
	return &defaults{
		data: make(map[string]interface{}),
	}, nil
}

// GetRules retrieves rules from a policy
func (pm *policyManager) GetRules(ctx context.Context, policyName string) ([]Rule, error) {
	policy, err := pm.GetPolicy(ctx, policyName)
	if err != nil {
		return nil, err
	}

	// Convert PolicyRules to Rules
	rules := make([]Rule, 0, len(policy.Rules))
	for i, policyRule := range policy.Rules {
		rule := Rule{
			ID:         uuid.New(),
			Name:       fmt.Sprintf("%s_rule_%d", policy.Name, i),
			Category:   policy.Resource,
			Expression: policyRule.Condition,
			Priority:   i,
			Enabled:    true,
			Metadata: map[string]interface{}{
				"effect":    policyRule.Effect,
				"actions":   policyRule.Actions,
				"resources": policyRule.Resources,
			},
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

// UpdatePolicy updates a policy
func (pm *policyManager) UpdatePolicy(ctx context.Context, policy *Policy) error {
	if err := pm.ValidatePolicy(ctx, policy); err != nil {
		return errors.Wrap(err, "policy validation failed")
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if policy exists
	existing, exists := pm.policies[policy.Name]
	if exists {
		// Increment version
		policy.Version = existing.Version + 1
	} else {
		policy.Version = 1
	}

	if policy.ID == uuid.Nil {
		policy.ID = uuid.New()
	}

	// Update policy
	pm.policies[policy.Name] = policy

	// Clear cache for this policy
	if pm.config.EnableCaching && pm.cache != nil {
		cacheKey := fmt.Sprintf("policy:%s", policy.Name)
		if err := pm.cache.Delete(ctx, cacheKey); err != nil {
			pm.logger.Warn("Failed to clear policy cache", map[string]interface{}{
				"policy": policy.Name,
				"error":  err.Error(),
			})
		}
	}

	pm.logger.Info("Policy updated", map[string]interface{}{
		"policy_id": policy.ID,
		"name":      policy.Name,
		"version":   policy.Version,
	})

	pm.metrics.IncrementCounterWithLabels("policies.update.success", 1.0, nil)

	return nil
}

// ValidatePolicy validates a policy
func (pm *policyManager) ValidatePolicy(ctx context.Context, policy *Policy) error {
	if policy == nil {
		return fmt.Errorf("policy is nil")
	}

	if policy.Name == "" {
		return fmt.Errorf("policy name is required")
	}

	if policy.Resource == "" {
		return fmt.Errorf("policy resource is required")
	}

	// Validate rules
	for i, rule := range policy.Rules {
		if rule.Effect != "allow" && rule.Effect != "deny" {
			return fmt.Errorf("invalid effect in rule %d: %s", i, rule.Effect)
		}

		if rule.Condition == "" {
			return fmt.Errorf("condition is required in rule %d", i)
		}

		if len(rule.Actions) == 0 {
			return fmt.Errorf("at least one action is required in rule %d", i)
		}

		if len(rule.Resources) == 0 {
			return fmt.Errorf("at least one resource is required in rule %d", i)
		}
	}

	// Check policy count limit
	pm.mu.RLock()
	policyCount := len(pm.policies)
	pm.mu.RUnlock()

	if policyCount >= pm.config.MaxPolicies && !pm.policyExists(policy.Name) {
		return fmt.Errorf("maximum number of policies (%d) reached", pm.config.MaxPolicies)
	}

	return nil
}

// addPolicyInternal adds a policy internally
func (pm *policyManager) addPolicyInternal(policy Policy) error {
	if err := pm.ValidatePolicy(context.Background(), &policy); err != nil {
		return err
	}

	if policy.ID == uuid.Nil {
		policy.ID = uuid.New()
	}

	pm.policies[policy.Name] = &policy
	return nil
}

// policyExists checks if a policy exists
func (pm *policyManager) policyExists(name string) bool {
	_, exists := pm.policies[name]
	return exists
}

// defaults implements the Defaults interface
type defaults struct {
	data map[string]interface{}
}

func (d *defaults) GetString(key string, defaultValue string) string {
	if val, ok := d.data[key].(string); ok {
		return val
	}
	return defaultValue
}

func (d *defaults) GetInt(key string, defaultValue int) int {
	if val, ok := d.data[key].(int); ok {
		return val
	}
	if val, ok := d.data[key].(float64); ok {
		return int(val)
	}
	return defaultValue
}

func (d *defaults) GetFloat(key string, defaultValue float64) float64 {
	if val, ok := d.data[key].(float64); ok {
		return val
	}
	if val, ok := d.data[key].(int); ok {
		return float64(val)
	}
	return defaultValue
}

func (d *defaults) GetBool(key string, defaultValue bool) bool {
	if val, ok := d.data[key].(bool); ok {
		return val
	}
	return defaultValue
}

func (d *defaults) GetMap(key string) map[string]interface{} {
	if val, ok := d.data[key].(map[string]interface{}); ok {
		return val
	}
	return nil
}