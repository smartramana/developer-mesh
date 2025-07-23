# Rules Package

> **Purpose**: Simple expression-based rules engine for the Developer Mesh platform
> **Status**: Basic Implementation
> **Dependencies**: Basic expression evaluation, rule storage

## Overview

The rules package provides a basic rules engine with simple expression evaluation. It supports comparison operators (==, !=, <, >, <=, >=) and boolean logic (&&, ||) for rule-based decisions.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Rules Engine Architecture                 │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Rules ──► Engine ──► Expression Evaluator ──► Decision    │
│    │          │                                    │        │
│    │          ├── Simple Parser                    │        │
│    │          └── Basic Cache                      │        │
│    │                                               │        │
│    └── In-Memory Storage                           │        │
│                                                    │        │
│                                          Result: bool/score │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Rule Structure

```go
// Rule represents a simple evaluation rule
type Rule struct {
    ID         uuid.UUID              `json:"id" db:"id"`
    Name       string                 `json:"name" db:"name"`
    Category   string                 `json:"category" db:"category"`
    Expression string                 `json:"expression" db:"expression"`
    Priority   int                    `json:"priority" db:"priority"`
    Enabled    bool                   `json:"enabled" db:"enabled"`
    Metadata   map[string]interface{} `json:"metadata" db:"metadata"`
    CreatedAt  time.Time              `json:"created_at" db:"created_at"`
    UpdatedAt  time.Time              `json:"updated_at" db:"updated_at"`
}

// Decision represents rule evaluation outcome
type Decision struct {
    Allowed  bool                   `json:"allowed"`
    Score    float64                `json:"score,omitempty"`
    Reason   string                 `json:"reason"`
    Metadata map[string]interface{} `json:"metadata,omitempty"`
}
```

### 2. Rule Engine Interface

```go
// Engine defines the rules engine interface
type Engine interface {
    // Rule evaluation
    Evaluate(ctx context.Context, ruleName string, data interface{}) (*Decision, error)
    
    // Rule management
    RegisterRule(ctx context.Context, rule Rule) error
    UpdateRule(ctx context.Context, ruleID uuid.UUID, updates map[string]interface{}) error
    GetRules(ctx context.Context, category string, filters map[string]interface{}) ([]Rule, error)
    
    // Bulk operations
    LoadRules(ctx context.Context, rules []Rule) error
    SetRuleLoader(loader RuleLoader) error
    
    // Hot reload
    StartHotReload(ctx context.Context) error
    StopHotReload() error
}

// RuleLoader interface for dynamic rule loading
type RuleLoader interface {
    LoadRules(ctx context.Context) ([]Rule, error)
}
```

### 3. Expression Evaluation

The rules engine supports simple expression evaluation with the following operators:

```go
// Supported operators:
// Comparison: ==, !=, <, >, <=, >=
// Logical: &&, ||

// Example expressions:
"status == 'active'"
"priority > 3"
"workload < 10"
"status == 'active' && priority > 5"
"category == 'urgent' || priority >= 8"

// Expression evaluation function
func (e *engine) evaluateExpression(expr string, params map[string]interface{}) (interface{}, error) {
    // Simple expression parser for MVP
    // Handles AND/OR operations and comparisons
    // Returns boolean result
}
```

## Example Use Cases

### 1. Status-Based Rules

```go
// Create a rule that checks agent status
statusRule := Rule{
    Name:       "agent-status-check",
    Category:   "agent",
    Expression: "status == 'active'",
    Priority:   10,
    Enabled:    true,
}

// Evaluate the rule
data := map[string]interface{}{
    "status": "active",
}

decision, err := engine.Evaluate(ctx, "agent-status-check", data)
// decision.Allowed = true
```

### 2. Numeric Comparison Rules

```go
// Create a rule that checks workload
workloadRule := Rule{
    Name:       "workload-limit",
    Category:   "capacity",
    Expression: "workload < 10",
    Priority:   5,
    Enabled:    true,
}

// Evaluate with data
data := map[string]interface{}{
    "workload": 15,
}

decision, err := engine.Evaluate(ctx, "workload-limit", data)
// decision.Allowed = false
```

### 3. Combined Conditions

```go
// Create a rule with AND logic
complexRule := Rule{
    Name:       "complex-check",
    Category:   "validation",
    Expression: "status == 'active' && priority > 5",
    Priority:   1,
    Enabled:    true,
}

// Create a rule with OR logic
alternativeRule := Rule{
    Name:       "alternative-check",
    Category:   "validation",
    Expression: "category == 'urgent' || priority >= 8",
    Priority:   1,
    Enabled:    true,
}
```

## Implementation Details

### Expression Parser

The expression parser is a simple implementation that:
1. Handles AND (`&&`) and OR (`||`) operations
2. Supports comparison operators: `==`, `!=`, `<`, `>`, `<=`, `>=`
3. Parses string values (with single or double quotes)
4. Parses numeric values (float64)
5. Parses boolean values

### Limitations

**Current implementation does NOT support:**
- Complex scripting (CEL, JavaScript, Lua)
- Decision tables
- Nested expressions or parentheses
- Function calls within expressions
- Complex data types (arrays, nested objects)
- Regular expressions
- Mathematical operations

**What IS implemented:**
- Basic expression evaluation
- Rule storage and retrieval
- Rule categories and filtering
- Hot reload capability (with external loader)
- Basic metrics tracking

## Rule Management

### In-Memory Storage

The current implementation stores rules in memory:

```go
// Internal storage
type engine struct {
    mu              sync.RWMutex
    rules           map[string]*Rule           // Rules by name
    rulesByCategory map[string][]*Rule         // Rules grouped by category
    evaluator       map[string]func(...)       // Compiled expressions
    // ... other fields
}
```

### Dynamic Rule Loading

Supports external rule loading through the RuleLoader interface:

```go
// Set a rule loader
engine.SetRuleLoader(myLoader)

// Start hot reload (checks for updates periodically)
engine.StartHotReload(ctx)

// Stop hot reload
engine.StopHotReload()
```

## Testing Example

```go
func TestRuleEngine(t *testing.T) {
    // Create engine
    config := Config{
        MaxRules:      100,
        CacheDuration: 5 * time.Minute,
    }
    engine := NewEngine(config, logger, metrics)
    
    // Register a rule
    rule := Rule{
        Name:       "test-rule",
        Expression: "value > 10",
        Priority:   1,
        Enabled:    true,
    }
    engine.RegisterRule(ctx, rule)
    
    // Test evaluation
    tests := []struct {
        name     string
        data     map[string]interface{}
        expected bool
    }{
        {
            name:     "value above threshold",
            data:     map[string]interface{}{"value": 15},
            expected: true,
        },
        {
            name:     "value below threshold",
            data:     map[string]interface{}{"value": 5},
            expected: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            decision, err := engine.Evaluate(ctx, "test-rule", tt.data)
            assert.NoError(t, err)
            assert.Equal(t, tt.expected, decision.Allowed)
        })
    }
}
```

## Metrics Tracking

The engine tracks basic metrics:
- Rule evaluation count and duration
- Cache hit rates
- Rule registration and update counts
- Hot reload success/failure

Metrics are recorded through the observability.MetricsClient interface.

## Configuration

```go
// Config for rule engine
type Config struct {
    HotReload      bool          `mapstructure:"hot_reload"`
    ReloadInterval time.Duration `mapstructure:"reload_interval"`
    CacheDuration  time.Duration `mapstructure:"cache_duration"`
    MaxRules       int           `mapstructure:"max_rules"`
}

// Default values:
// ReloadInterval: 30 seconds
// CacheDuration: 5 minutes
// MaxRules: 1000
```

## Best Practices

1. **Keep Expressions Simple**: The parser only handles basic comparisons and boolean logic
2. **Use Categories**: Group related rules by category for easier management
3. **Set Priorities**: Higher priority rules are stored together by category
4. **Enable/Disable**: Use the enabled flag to temporarily disable rules
5. **Test Thoroughly**: Test all expression paths since there's no complex validation
6. **Monitor Performance**: Track evaluation times for performance optimization

---

Package Version: 1.0.0 (Basic Implementation)
Last Updated: 2024-01-24