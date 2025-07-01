# Rules Package

> **Purpose**: Business rules engine for policy-based decision making in the DevOps MCP platform
> **Status**: Production Ready
> **Dependencies**: Rule evaluation engine, policy management, decision tables

## Overview

The rules package provides a flexible business rules engine that enables dynamic policy-based decision making. It supports rule composition, decision tables, and integration with AI agent routing, cost controls, and access policies.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Rules Engine Architecture                 │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Rule Sets ──► Rule Engine ──► Decision ──► Actions        │
│      │             │              │            │            │
│      │             ├── Parser     │            ├── Allow    │
│      │             ├── Evaluator  │            ├── Deny     │
│      │             └── Cache      │            ├── Route    │
│      │                            │            └── Transform│
│      └── Rules Repository         │                         │
│                                   └── Decision Log          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Rule Interface

```go
// Rule defines a business rule
type Rule interface {
    // ID returns unique rule identifier
    ID() string
    
    // Name returns human-readable name
    Name() string
    
    // Evaluate applies the rule to given context
    Evaluate(ctx context.Context, facts Facts) (*Decision, error)
    
    // Priority returns rule priority (higher = more important)
    Priority() int
    
    // Enabled checks if rule is active
    Enabled() bool
    
    // Metadata returns rule metadata
    Metadata() map[string]interface{}
}

// Facts represents input data for rules
type Facts map[string]interface{}

// Decision represents rule evaluation outcome
type Decision struct {
    RuleID     string                 `json:"rule_id"`
    Result     Result                 `json:"result"`
    Reason     string                 `json:"reason"`
    Actions    []Action               `json:"actions,omitempty"`
    Confidence float64                `json:"confidence"`
    Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Result represents evaluation result
type Result string

const (
    ResultAllow     Result = "allow"
    ResultDeny      Result = "deny"
    ResultSkip      Result = "skip"
    ResultEscalate  Result = "escalate"
)
```

### 2. Rule Engine

```go
// RuleEngine evaluates rules against facts
type RuleEngine struct {
    rules       []Rule
    mu          sync.RWMutex
    cache       *DecisionCache
    metrics     *Metrics
    logger      *slog.Logger
}

// NewRuleEngine creates a rule engine
func NewRuleEngine(config *Config) *RuleEngine {
    return &RuleEngine{
        rules:   make([]Rule, 0),
        cache:   NewDecisionCache(config.CacheSize),
        metrics: NewMetrics(),
        logger:  slog.Default(),
    }
}

// AddRule registers a rule
func (e *RuleEngine) AddRule(rule Rule) {
    e.mu.Lock()
    defer e.mu.Unlock()
    
    e.rules = append(e.rules, rule)
    
    // Sort by priority
    sort.Slice(e.rules, func(i, j int) bool {
        return e.rules[i].Priority() > e.rules[j].Priority()
    })
    
    e.logger.Info("rule added",
        "rule_id", rule.ID(),
        "name", rule.Name(),
        "priority", rule.Priority(),
    )
}

// Evaluate runs all applicable rules
func (e *RuleEngine) Evaluate(ctx context.Context, facts Facts) (*EngineDecision, error) {
    // Generate cache key
    cacheKey := e.generateCacheKey(facts)
    
    // Check cache
    if cached := e.cache.Get(cacheKey); cached != nil {
        e.metrics.RecordCacheHit()
        return cached, nil
    }
    
    start := time.Now()
    decisions := make([]*Decision, 0)
    
    // Evaluate rules in priority order
    for _, rule := range e.rules {
        if !rule.Enabled() {
            continue
        }
        
        decision, err := rule.Evaluate(ctx, facts)
        if err != nil {
            e.logger.Error("rule evaluation failed",
                "rule_id", rule.ID(),
                "error", err,
            )
            continue
        }
        
        decisions = append(decisions, decision)
        
        // Stop on first deny (fail-fast)
        if decision.Result == ResultDeny {
            break
        }
    }
    
    // Aggregate decisions
    engineDecision := e.aggregateDecisions(decisions)
    
    // Cache result
    e.cache.Set(cacheKey, engineDecision)
    
    // Record metrics
    duration := time.Since(start)
    e.metrics.RecordEvaluation(engineDecision.Result, duration)
    
    return engineDecision, nil
}
```

### 3. Rule Types

#### Condition Rule

```go
// ConditionRule evaluates boolean conditions
type ConditionRule struct {
    id         string
    name       string
    condition  Condition
    priority   int
    enabled    bool
}

// Condition represents a boolean expression
type Condition interface {
    Evaluate(facts Facts) (bool, error)
}

// Example conditions
type AndCondition struct {
    conditions []Condition
}

func (c *AndCondition) Evaluate(facts Facts) (bool, error) {
    for _, cond := range c.conditions {
        result, err := cond.Evaluate(facts)
        if err != nil {
            return false, err
        }
        if !result {
            return false, nil
        }
    }
    return true, nil
}

type ComparisonCondition struct {
    field    string
    operator Operator
    value    interface{}
}

func (c *ComparisonCondition) Evaluate(facts Facts) (bool, error) {
    factValue, exists := facts[c.field]
    if !exists {
        return false, nil
    }
    
    return c.operator.Compare(factValue, c.value)
}
```

#### Script Rule

```go
// ScriptRule evaluates using embedded scripts
type ScriptRule struct {
    id       string
    name     string
    script   string
    language string // "cel", "javascript", "lua"
    priority int
    enabled  bool
    vm       ScriptVM
}

func (r *ScriptRule) Evaluate(ctx context.Context, facts Facts) (*Decision, error) {
    // Create script context
    scriptCtx := r.vm.NewContext()
    
    // Add facts to context
    for k, v := range facts {
        scriptCtx.Set(k, v)
    }
    
    // Execute script
    result, err := r.vm.Execute(ctx, r.script, scriptCtx)
    if err != nil {
        return nil, fmt.Errorf("script execution failed: %w", err)
    }
    
    // Convert result to decision
    return r.convertToDecision(result), nil
}

// Example CEL script rule
func NewCELRule(id, name, expression string) *ScriptRule {
    env, _ := cel.NewEnv(
        cel.Variable("request", cel.DynType),
        cel.Variable("user", cel.DynType),
        cel.Variable("resource", cel.DynType),
    )
    
    ast, _ := env.Compile(expression)
    prg, _ := env.Program(ast)
    
    return &ScriptRule{
        id:       id,
        name:     name,
        script:   expression,
        language: "cel",
        vm:       &CELScriptVM{program: prg},
    }
}
```

#### Decision Table Rule

```go
// DecisionTable implements tabular decision logic
type DecisionTable struct {
    id       string
    name     string
    headers  []Column
    rows     []Row
    priority int
    enabled  bool
}

type Column struct {
    Name      string
    Type      ColumnType
    Condition string // For condition columns
}

type Row struct {
    Conditions []interface{}
    Actions    []Action
}

func (t *DecisionTable) Evaluate(ctx context.Context, facts Facts) (*Decision, error) {
    for _, row := range t.rows {
        if t.matchesRow(row, facts) {
            return &Decision{
                RuleID:  t.id,
                Result:  ResultAllow,
                Actions: row.Actions,
                Reason:  fmt.Sprintf("Matched row in table %s", t.name),
            }, nil
        }
    }
    
    return &Decision{
        RuleID: t.id,
        Result: ResultSkip,
        Reason: "No matching row in decision table",
    }, nil
}

// Example decision table for agent routing
var agentRoutingTable = &DecisionTable{
    id:   "agent-routing",
    name: "Agent Routing Rules",
    headers: []Column{
        {Name: "task_type", Type: ColumnTypeString},
        {Name: "complexity", Type: ColumnTypeString},
        {Name: "cost_limit", Type: ColumnTypeNumber},
    },
    rows: []Row{
        {
            Conditions: []interface{}{"embedding", "low", 0.10},
            Actions: []Action{
                &RouteAction{Model: "titan-embed-v1"},
            },
        },
        {
            Conditions: []interface{}{"generation", "high", 1.00},
            Actions: []Action{
                &RouteAction{Model: "claude-3-opus"},
            },
        },
    },
}
```

## Domain-Specific Rules

### 1. Agent Routing Rules

```go
// AgentRoutingRule determines which agent handles a task
type AgentRoutingRule struct {
    BaseRule
    routingStrategy RoutingStrategy
}

func (r *AgentRoutingRule) Evaluate(ctx context.Context, facts Facts) (*Decision, error) {
    task, ok := facts["task"].(*Task)
    if !ok {
        return nil, fmt.Errorf("missing or invalid task in facts")
    }
    
    agents, ok := facts["available_agents"].([]Agent)
    if !ok {
        return nil, fmt.Errorf("missing available agents")
    }
    
    // Apply routing strategy
    selectedAgent := r.routingStrategy.SelectAgent(task, agents)
    
    if selectedAgent == nil {
        return &Decision{
            RuleID: r.ID(),
            Result: ResultEscalate,
            Reason: "No suitable agent found",
        }, nil
    }
    
    return &Decision{
        RuleID: r.ID(),
        Result: ResultAllow,
        Actions: []Action{
            &AssignAction{
                AgentID: selectedAgent.ID,
                TaskID:  task.ID,
            },
        },
        Metadata: map[string]interface{}{
            "selected_agent": selectedAgent.ID,
            "routing_score":  selectedAgent.Score,
        },
    }, nil
}

// Routing strategies
type CapabilityBasedRouting struct{}

func (s *CapabilityBasedRouting) SelectAgent(task *Task, agents []Agent) *Agent {
    var bestAgent *Agent
    bestScore := 0.0
    
    for _, agent := range agents {
        score := s.calculateScore(task, agent)
        if score > bestScore {
            bestScore = score
            bestAgent = &agent
        }
    }
    
    return bestAgent
}

func (s *CapabilityBasedRouting) calculateScore(task *Task, agent Agent) float64 {
    score := 0.0
    
    // Check capability match
    for _, required := range task.RequiredCapabilities {
        if agent.HasCapability(required) {
            score += 1.0
        }
    }
    
    // Factor in agent load
    score *= (1.0 - agent.CurrentLoad)
    
    // Factor in performance history
    if metrics := agent.PerformanceMetrics[task.Type]; metrics != nil {
        score *= metrics.SuccessRate
    }
    
    return score
}
```

### 2. Cost Control Rules

```go
// CostControlRule enforces spending limits
type CostControlRule struct {
    BaseRule
    limits CostLimits
}

type CostLimits struct {
    SessionLimit  float64
    DailyLimit    float64
    MonthlyLimit  float64
}

func (r *CostControlRule) Evaluate(ctx context.Context, facts Facts) (*Decision, error) {
    session, _ := facts["session_id"].(string)
    estimatedCost, _ := facts["estimated_cost"].(float64)
    
    // Get current spending
    currentSpending, err := r.getSpending(ctx, session)
    if err != nil {
        return nil, err
    }
    
    // Check session limit
    if currentSpending.Session+estimatedCost > r.limits.SessionLimit {
        return &Decision{
            RuleID: r.ID(),
            Result: ResultDeny,
            Reason: fmt.Sprintf("Session cost limit exceeded: $%.2f + $%.2f > $%.2f",
                currentSpending.Session, estimatedCost, r.limits.SessionLimit),
            Actions: []Action{
                &NotifyAction{
                    Type:    "cost_limit_exceeded",
                    Message: "Session spending limit reached",
                },
            },
        }, nil
    }
    
    // Check daily limit
    if currentSpending.Daily+estimatedCost > r.limits.DailyLimit {
        return &Decision{
            RuleID: r.ID(),
            Result: ResultDeny,
            Reason: fmt.Sprintf("Daily cost limit exceeded"),
        }, nil
    }
    
    return &Decision{
        RuleID: r.ID(),
        Result: ResultAllow,
        Metadata: map[string]interface{}{
            "remaining_session_budget": r.limits.SessionLimit - currentSpending.Session,
            "remaining_daily_budget":   r.limits.DailyLimit - currentSpending.Daily,
        },
    }, nil
}
```

### 3. Access Control Rules

```go
// AccessControlRule enforces permissions
type AccessControlRule struct {
    BaseRule
    permissions PermissionChecker
}

func (r *AccessControlRule) Evaluate(ctx context.Context, facts Facts) (*Decision, error) {
    user, _ := facts["user"].(*User)
    resource, _ := facts["resource"].(string)
    action, _ := facts["action"].(string)
    
    if user == nil || resource == "" || action == "" {
        return &Decision{
            RuleID: r.ID(),
            Result: ResultDeny,
            Reason: "Missing required facts: user, resource, or action",
        }, nil
    }
    
    allowed := r.permissions.Check(user, resource, action)
    
    if allowed {
        return &Decision{
            RuleID: r.ID(),
            Result: ResultAllow,
            Metadata: map[string]interface{}{
                "user_roles": user.Roles,
                "resource":   resource,
                "action":     action,
            },
        }, nil
    }
    
    return &Decision{
        RuleID: r.ID(),
        Result: ResultDeny,
        Reason: fmt.Sprintf("User %s lacks permission to %s on %s", 
            user.ID, action, resource),
    }, nil
}
```

### 4. Rate Limiting Rules

```go
// RateLimitRule enforces request rate limits
type RateLimitRule struct {
    BaseRule
    limiter RateLimiter
}

func (r *RateLimitRule) Evaluate(ctx context.Context, facts Facts) (*Decision, error) {
    identifier, _ := facts["identifier"].(string) // user ID, IP, API key
    weight, _ := facts["weight"].(int)
    
    if identifier == "" {
        identifier = "anonymous"
    }
    if weight == 0 {
        weight = 1
    }
    
    allowed := r.limiter.Allow(identifier, weight)
    
    if allowed {
        return &Decision{
            RuleID: r.ID(),
            Result: ResultAllow,
        }, nil
    }
    
    // Get rate limit info
    limit, remaining, resetAt := r.limiter.GetStatus(identifier)
    
    return &Decision{
        RuleID: r.ID(),
        Result: ResultDeny,
        Reason: "Rate limit exceeded",
        Actions: []Action{
            &HTTPResponseAction{
                Headers: map[string]string{
                    "X-RateLimit-Limit":     strconv.Itoa(limit),
                    "X-RateLimit-Remaining": strconv.Itoa(remaining),
                    "X-RateLimit-Reset":     strconv.FormatInt(resetAt.Unix(), 10),
                    "Retry-After":           strconv.Itoa(int(time.Until(resetAt).Seconds())),
                },
            },
        },
    }, nil
}
```

## Rule Actions

```go
// Action represents an outcome of rule evaluation
type Action interface {
    Type() string
    Execute(ctx context.Context) error
}

// Common actions
type RouteAction struct {
    Model    string
    Priority int
}

type AssignAction struct {
    AgentID string
    TaskID  string
}

type TransformAction struct {
    Transformations map[string]interface{}
}

type NotifyAction struct {
    Type       string
    Message    string
    Recipients []string
}

type HTTPResponseAction struct {
    StatusCode int
    Headers    map[string]string
    Body       interface{}
}

type LogAction struct {
    Level   string
    Message string
    Fields  map[string]interface{}
}
```

## Rule Management

### Rule Repository

```go
// RuleRepository manages rule persistence
type RuleRepository interface {
    // Load retrieves a rule by ID
    Load(ctx context.Context, id string) (Rule, error)
    
    // LoadAll retrieves all rules
    LoadAll(ctx context.Context) ([]Rule, error)
    
    // Save persists a rule
    Save(ctx context.Context, rule Rule) error
    
    // Delete removes a rule
    Delete(ctx context.Context, id string) error
    
    // Search finds rules matching criteria
    Search(ctx context.Context, criteria SearchCriteria) ([]Rule, error)
}

// DatabaseRuleRepository stores rules in PostgreSQL
type DatabaseRuleRepository struct {
    db *sql.DB
}

func (r *DatabaseRuleRepository) Save(ctx context.Context, rule Rule) error {
    data, err := json.Marshal(rule)
    if err != nil {
        return err
    }
    
    _, err = r.db.ExecContext(ctx, `
        INSERT INTO rules (id, name, type, priority, enabled, data, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
        ON CONFLICT (id) DO UPDATE SET
            name = EXCLUDED.name,
            priority = EXCLUDED.priority,
            enabled = EXCLUDED.enabled,
            data = EXCLUDED.data,
            updated_at = NOW()
    `, rule.ID(), rule.Name(), reflect.TypeOf(rule).Name(), 
       rule.Priority(), rule.Enabled(), data)
    
    return err
}
```

### Rule Versioning

```go
// VersionedRule supports rule versioning
type VersionedRule struct {
    Rule
    Version   int
    CreatedAt time.Time
    CreatedBy string
    Comment   string
}

// RuleHistory tracks rule changes
type RuleHistory struct {
    RuleID   string
    Versions []VersionedRule
}

func (h *RuleHistory) GetVersion(version int) *VersionedRule {
    for _, v := range h.Versions {
        if v.Version == version {
            return &v
        }
    }
    return nil
}
```

## Testing Rules

```go
// RuleTester helps test rules
type RuleTester struct {
    engine *RuleEngine
}

func (t *RuleTester) TestRule(rule Rule, testCases []TestCase) *TestReport {
    report := &TestReport{
        RuleID:     rule.ID(),
        TotalTests: len(testCases),
    }
    
    for _, tc := range testCases {
        result := t.runTestCase(rule, tc)
        report.Results = append(report.Results, result)
        
        if result.Passed {
            report.PassedTests++
        } else {
            report.FailedTests++
        }
    }
    
    return report
}

type TestCase struct {
    Name           string
    Facts          Facts
    ExpectedResult Result
    ExpectedReason string
}

// Example test
func TestAgentRoutingRule(t *testing.T) {
    rule := NewAgentRoutingRule()
    tester := NewRuleTester(nil)
    
    testCases := []TestCase{
        {
            Name: "Route to capable agent",
            Facts: Facts{
                "task": &Task{
                    Type: "embedding",
                    RequiredCapabilities: []string{"text-embedding"},
                },
                "available_agents": []Agent{
                    {ID: "agent1", Capabilities: []string{"text-embedding"}},
                    {ID: "agent2", Capabilities: []string{"code-generation"}},
                },
            },
            ExpectedResult: ResultAllow,
        },
    }
    
    report := tester.TestRule(rule, testCases)
    assert.Equal(t, len(testCases), report.PassedTests)
}
```

## Monitoring & Metrics

```go
var (
    ruleEvaluations = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_rule_evaluations_total",
            Help: "Total number of rule evaluations",
        },
        []string{"rule_id", "result"},
    )
    
    ruleEvaluationDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "mcp_rule_evaluation_duration_seconds",
            Help:    "Rule evaluation duration",
            Buckets: prometheus.ExponentialBuckets(0.0001, 2, 15),
        },
        []string{"rule_id"},
    )
    
    decisionCacheHitRate = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "mcp_rule_decision_cache_hit_rate",
            Help: "Decision cache hit rate",
        },
    )
)
```

## Configuration

### Environment Variables

```bash
# Rules Engine Configuration
RULES_ENGINE_CACHE_SIZE=10000
RULES_ENGINE_CACHE_TTL=5m
RULES_ENGINE_EVALUATION_TIMEOUT=1s

# Script Rules
RULES_SCRIPT_CEL_ENABLED=true
RULES_SCRIPT_TIMEOUT=100ms

# Cost Control
RULES_COST_SESSION_LIMIT=0.10
RULES_COST_DAILY_LIMIT=10.00
RULES_COST_MONTHLY_LIMIT=100.00

# Rate Limiting
RULES_RATE_LIMIT_PER_MINUTE=60
RULES_RATE_LIMIT_BURST=10
```

### Configuration File

```yaml
rules:
  engine:
    cache_size: 10000
    cache_ttl: 5m
    evaluation_timeout: 1s
    
  scripts:
    cel:
      enabled: true
      timeout: 100ms
      
  domains:
    routing:
      strategy: capability_based
      fallback_agent: default-agent
      
    cost_control:
      session_limit: 0.10
      daily_limit: 10.00
      monthly_limit: 100.00
      
    rate_limiting:
      default:
        rate: 60
        burst: 10
      by_tier:
        free: 
          rate: 10
          burst: 5
        premium:
          rate: 100
          burst: 20
```

## Best Practices

1. **Rule Design**: Keep rules simple and focused on single concerns
2. **Priority Management**: Use clear priority levels for rule ordering
3. **Performance**: Cache decisions for expensive evaluations
4. **Testing**: Thoroughly test rules with various input scenarios
5. **Monitoring**: Track rule performance and decision patterns
6. **Versioning**: Version rules for rollback capability
7. **Documentation**: Document rule logic and expected behavior
8. **Fail-Safe**: Always have default rules for unmatched cases

---

Package Version: 1.0.0
Last Updated: 2024-01-10