package rules

import (
	"context"

	"github.com/google/uuid"
)

// Engine evaluates business rules
type Engine interface {
	Evaluate(ctx context.Context, ruleName string, data interface{}) (*Decision, error)
	GetRules(ctx context.Context, category string, filters map[string]interface{}) ([]Rule, error)
	RegisterRule(ctx context.Context, rule Rule) error
	UpdateRule(ctx context.Context, ruleID uuid.UUID, updates map[string]interface{}) error

	// Production methods for dynamic rule management
	LoadRules(ctx context.Context, rules []Rule) error
	StartHotReload(ctx context.Context) error
	StopHotReload() error
	SetRuleLoader(loader RuleLoader) error
}

// RuleLoader defines how rules are loaded from external sources
type RuleLoader interface {
	LoadRules(ctx context.Context) ([]Rule, error)
}

// Rule represents a business rule
type Rule struct {
	ID         uuid.UUID              `json:"id"`
	Name       string                 `json:"name"`
	Category   string                 `json:"category"`
	Expression string                 `json:"expression"`
	Priority   int                    `json:"priority"`
	Enabled    bool                   `json:"enabled"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// Decision represents the result of rule evaluation
type Decision struct {
	Allowed  bool                   `json:"allowed"`
	Reason   string                 `json:"reason"`
	Score    float64                `json:"score"`
	Metadata map[string]interface{} `json:"metadata"`
}

// PolicyManager manages dynamic policies
type PolicyManager interface {
	GetPolicy(ctx context.Context, policyName string) (*Policy, error)
	GetDefaults(ctx context.Context, resource, resourceType string) (Defaults, error)
	GetRules(ctx context.Context, policyName string) ([]Rule, error)
	UpdatePolicy(ctx context.Context, policy *Policy) error
	ValidatePolicy(ctx context.Context, policy *Policy) error

	// Production methods for dynamic policy management
	LoadPolicies(ctx context.Context, policies []Policy) error
	StartHotReload(ctx context.Context) error
	StopHotReload() error
	SetPolicyLoader(loader PolicyLoader) error
}

// PolicyLoader defines how policies are loaded from external sources
type PolicyLoader interface {
	LoadPolicies(ctx context.Context) ([]Policy, error)
}

// Policy represents a business policy
type Policy struct {
	ID       uuid.UUID              `json:"id"`
	Name     string                 `json:"name"`
	Resource string                 `json:"resource"`
	Rules    []PolicyRule           `json:"rules"`
	Defaults map[string]interface{} `json:"defaults"`
	Version  int                    `json:"version"`
}

// PolicyRule represents a rule within a policy
type PolicyRule struct {
	Condition string   `json:"condition"`
	Effect    string   `json:"effect"` // "allow" or "deny"
	Actions   []string `json:"actions"`
	Resources []string `json:"resources"`
}

// Defaults represents default values
type Defaults interface {
	GetString(key string, defaultValue string) string
	GetInt(key string, defaultValue int) int
	GetFloat(key string, defaultValue float64) float64
	GetBool(key string, defaultValue bool) bool
	GetMap(key string) map[string]interface{}
}

// ValidationResult represents the result of a validation rule
type ValidationResult struct {
	Valid   bool   `json:"valid"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Rule interface for validation
type ValidationRule interface {
	Evaluate(ctx context.Context, data interface{}) (*ValidationResult, error)
}
