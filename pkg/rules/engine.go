package rules

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/observability"
	// "github.com/Knetic/govaluate" // Removed - using simple expression evaluation instead
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// Config for rule engine
type Config struct {
	HotReload      bool          `mapstructure:"hot_reload"`
	ReloadInterval time.Duration `mapstructure:"reload_interval"`
	CacheDuration  time.Duration `mapstructure:"cache_duration"`
	MaxRules       int           `mapstructure:"max_rules"`
}

// engine implements the Engine interface
type engine struct {
	mu              sync.RWMutex
	rules           map[string]*Rule
	rulesByCategory map[string][]*Rule
	evaluator       map[string]func(map[string]interface{}) (interface{}, error)
	config          Config
	logger          observability.Logger
	metrics         observability.MetricsClient
	loadFunc        func() ([]Rule, error)
	stopChan        chan struct{}
}

// NewEngine creates a new rule engine
func NewEngine(config Config, logger observability.Logger, metrics observability.MetricsClient) Engine {
	if config.ReloadInterval == 0 {
		config.ReloadInterval = 30 * time.Second
	}
	if config.CacheDuration == 0 {
		config.CacheDuration = 5 * time.Minute
	}
	if config.MaxRules == 0 {
		config.MaxRules = 1000
	}

	return &engine{
		rules:           make(map[string]*Rule),
		rulesByCategory: make(map[string][]*Rule),
		evaluator:       make(map[string]func(map[string]interface{}) (interface{}, error)),
		config:          config,
		logger:          logger,
		metrics:         metrics,
		stopChan:        make(chan struct{}),
	}
}

// LoadRules loads rules in bulk
func (e *engine) LoadRules(ctx context.Context, rules []Rule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Clear existing rules
	e.rules = make(map[string]*Rule)
	e.rulesByCategory = make(map[string][]*Rule)
	e.evaluator = make(map[string]func(map[string]interface{}) (interface{}, error))

	// Load new rules
	successCount := 0
	for _, rule := range rules {
		if err := e.addRuleInternal(rule); err != nil {
			e.logger.Warn("Failed to add rule", map[string]interface{}{
				"rule":  rule.Name,
				"error": err.Error(),
			})
			continue
		}
		successCount++
	}

	e.logger.Info("Rules loaded", map[string]interface{}{
		"total":   len(rules),
		"success": successCount,
		"failed":  len(rules) - successCount,
	})

	e.metrics.RecordGauge("rules.total", float64(successCount), nil)
	return nil
}

// SetRuleLoader sets the rule loader for dynamic loading
func (e *engine) SetRuleLoader(loader RuleLoader) error {
	if loader == nil {
		return fmt.Errorf("rule loader cannot be nil")
	}

	e.mu.Lock()
	e.loadFunc = func() ([]Rule, error) {
		return loader.LoadRules(context.Background())
	}
	e.mu.Unlock()

	return nil
}

// StartHotReload starts the hot reload goroutine
func (e *engine) StartHotReload(ctx context.Context) error {
	if !e.config.HotReload {
		return fmt.Errorf("hot reload is disabled")
	}

	e.mu.Lock()
	if e.loadFunc == nil {
		e.mu.Unlock()
		return fmt.Errorf("no rule loader configured")
	}
	e.mu.Unlock()

	go func() {
		ticker := time.NewTicker(e.config.ReloadInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-e.stopChan:
				return
			case <-ticker.C:
				if err := e.reload(); err != nil {
					e.logger.Error("Failed to reload rules", map[string]interface{}{
						"error": err.Error(),
					})
					e.metrics.IncrementCounterWithLabels("rules.reload.error", 1.0, map[string]string{
						"error": "load_failed",
					})
				} else {
					e.metrics.IncrementCounterWithLabels("rules.reload.success", 1.0, nil)
				}
			}
		}
	}()

	return nil
}

// StopHotReload stops the hot reload goroutine
func (e *engine) StopHotReload() error {
	select {
	case <-e.stopChan:
		// Already closed
		return fmt.Errorf("hot reload already stopped")
	default:
		close(e.stopChan)
		return nil
	}
}

// reload reloads rules from external source
func (e *engine) reload() error {
	if e.loadFunc == nil {
		return nil
	}

	rules, err := e.loadFunc()
	if err != nil {
		return errors.Wrap(err, "failed to load rules")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Clear existing rules
	e.rules = make(map[string]*Rule)
	e.rulesByCategory = make(map[string][]*Rule)
	e.evaluator = make(map[string]func(map[string]interface{}) (interface{}, error))

	// Load new rules
	for _, rule := range rules {
		if err := e.addRuleInternal(rule); err != nil {
			e.logger.Warn("Failed to add rule", map[string]interface{}{
				"rule":  rule.Name,
				"error": err.Error(),
			})
			continue
		}
	}

	e.logger.Info("Rules reloaded", map[string]interface{}{
		"count": len(e.rules),
	})

	return nil
}

// Evaluate evaluates a rule against data
func (e *engine) Evaluate(ctx context.Context, ruleName string, data interface{}) (*Decision, error) {
	start := time.Now()
	defer func() {
		e.metrics.RecordDuration("rules.evaluate.duration", time.Since(start))
	}()

	e.mu.RLock()
	rule, exists := e.rules[ruleName]
	evaluator, hasEvaluator := e.evaluator[ruleName]
	e.mu.RUnlock()

	if !exists {
		e.metrics.IncrementCounterWithLabels("rules.evaluate.error", 1.0, map[string]string{
			"error": "not_found",
		})
		return nil, fmt.Errorf("rule not found: %s", ruleName)
	}

	if !rule.Enabled {
		return &Decision{
			Allowed: false,
			Reason:  "Rule is disabled",
		}, nil
	}

	if !hasEvaluator {
		e.metrics.IncrementCounterWithLabels("rules.evaluate.error", 1.0, map[string]string{
			"error": "no_evaluator",
		})
		return nil, fmt.Errorf("no evaluator for rule: %s", ruleName)
	}

	// Convert data to parameters map
	params, err := e.convertToParams(data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert data to parameters")
	}

	// Evaluate expression
	result, err := evaluator(params)
	if err != nil {
		e.metrics.IncrementCounterWithLabels("rules.evaluate.error", 1.0, map[string]string{
			"error": "evaluation_failed",
		})
		return nil, errors.Wrap(err, "failed to evaluate rule")
	}

	// Convert result to decision
	decision := &Decision{
		Metadata: rule.Metadata,
	}

	switch v := result.(type) {
	case bool:
		decision.Allowed = v
		if v {
			decision.Reason = "Rule passed"
		} else {
			decision.Reason = "Rule failed"
		}
	case float64:
		decision.Score = v
		decision.Allowed = v > 0.5
		decision.Reason = fmt.Sprintf("Score: %.2f", v)
	default:
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}

	e.metrics.IncrementCounterWithLabels("rules.evaluate.success", 1.0, map[string]string{
		"rule":    ruleName,
		"allowed": fmt.Sprintf("%t", decision.Allowed),
	})

	return decision, nil
}

// GetRules retrieves rules by category and filters
func (e *engine) GetRules(ctx context.Context, category string, filters map[string]interface{}) ([]Rule, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var rules []Rule

	if category != "" {
		if categoryRules, exists := e.rulesByCategory[category]; exists {
			for _, rule := range categoryRules {
				if e.matchesFilters(rule, filters) {
					rules = append(rules, *rule)
				}
			}
		}
	} else {
		for _, rule := range e.rules {
			if e.matchesFilters(rule, filters) {
				rules = append(rules, *rule)
			}
		}
	}

	return rules, nil
}

// RegisterRule registers a new rule
func (e *engine) RegisterRule(ctx context.Context, rule Rule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.rules) >= e.config.MaxRules {
		return fmt.Errorf("maximum number of rules (%d) reached", e.config.MaxRules)
	}

	if rule.ID == uuid.Nil {
		rule.ID = uuid.New()
	}

	if err := e.addRuleInternal(rule); err != nil {
		return err
	}

	e.logger.Info("Rule registered", map[string]interface{}{
		"rule_id":  rule.ID,
		"name":     rule.Name,
		"category": rule.Category,
	})

	e.metrics.IncrementCounterWithLabels("rules.register.success", 1.0, map[string]string{
		"category": rule.Category,
	})

	return nil
}

// UpdateRule updates an existing rule
func (e *engine) UpdateRule(ctx context.Context, ruleID uuid.UUID, updates map[string]interface{}) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var rule *Rule
	for _, r := range e.rules {
		if r.ID == ruleID {
			rule = r
			break
		}
	}

	if rule == nil {
		return fmt.Errorf("rule not found: %s", ruleID)
	}

	// Apply updates
	if name, ok := updates["name"].(string); ok {
		rule.Name = name
	}
	if category, ok := updates["category"].(string); ok {
		// Remove from old category
		e.removeFromCategory(rule)
		rule.Category = category
		// Add to new category
		e.addToCategory(rule)
	}
	if expression, ok := updates["expression"].(string); ok {
		// Parse new expression
		evaluator, err := e.parseExpression(expression)
		if err != nil {
			return errors.Wrap(err, "invalid expression")
		}
		rule.Expression = expression
		e.evaluator[rule.Name] = evaluator
	}
	if priority, ok := updates["priority"].(int); ok {
		rule.Priority = priority
	}
	if enabled, ok := updates["enabled"].(bool); ok {
		rule.Enabled = enabled
	}
	if metadata, ok := updates["metadata"].(map[string]interface{}); ok {
		rule.Metadata = metadata
	}

	e.logger.Info("Rule updated", map[string]interface{}{
		"rule_id": ruleID,
		"updates": len(updates),
	})

	e.metrics.IncrementCounterWithLabels("rules.update.success", 1.0, nil)

	return nil
}

// addRuleInternal adds a rule internally
func (e *engine) addRuleInternal(rule Rule) error {
	if rule.Name == "" {
		return fmt.Errorf("rule name is required")
	}

	if rule.Expression == "" {
		return fmt.Errorf("rule expression is required")
	}

	// Parse expression
	evaluator, err := e.parseExpression(rule.Expression)
	if err != nil {
		return errors.Wrap(err, "invalid expression")
	}

	// Store rule
	e.rules[rule.Name] = &rule
	e.evaluator[rule.Name] = evaluator

	// Add to category index
	e.addToCategory(&rule)

	return nil
}

// addToCategory adds rule to category index
func (e *engine) addToCategory(rule *Rule) {
	if rule.Category != "" {
		e.rulesByCategory[rule.Category] = append(e.rulesByCategory[rule.Category], rule)
	}
}

// removeFromCategory removes rule from category index
func (e *engine) removeFromCategory(rule *Rule) {
	if rule.Category == "" {
		return
	}

	rules := e.rulesByCategory[rule.Category]
	for i, r := range rules {
		if r.ID == rule.ID {
			e.rulesByCategory[rule.Category] = append(rules[:i], rules[i+1:]...)
			break
		}
	}
}

// matchesFilters checks if rule matches the given filters
func (e *engine) matchesFilters(rule *Rule, filters map[string]interface{}) bool {
	if filters == nil || len(filters) == 0 {
		return true
	}

	for key, value := range filters {
		switch key {
		case "enabled":
			if enabled, ok := value.(bool); ok && rule.Enabled != enabled {
				return false
			}
		case "priority":
			if priority, ok := value.(int); ok && rule.Priority != priority {
				return false
			}
		case "metadata":
			if metadata, ok := value.(map[string]interface{}); ok {
				for k, v := range metadata {
					if rule.Metadata[k] != v {
						return false
					}
				}
			}
		}
	}

	return true
}

// convertToParams converts data to parameters map for evaluation
func (e *engine) convertToParams(data interface{}) (map[string]interface{}, error) {
	switch v := data.(type) {
	case map[string]interface{}:
		return v, nil
	case map[string]string:
		params := make(map[string]interface{})
		for k, val := range v {
			params[k] = val
		}
		return params, nil
	default:
		// Use reflection for struct types
		return map[string]interface{}{"data": data}, nil
	}
}

// parseExpression parses a rule expression and returns an evaluator function
func (e *engine) parseExpression(expression string) (func(map[string]interface{}) (interface{}, error), error) {
	// Production implementation with basic expression support
	// Supports: ==, !=, <, >, <=, >=, &&, ||
	// Examples: "status == 'active'", "priority > 3", "workload < 10"

	return func(params map[string]interface{}) (interface{}, error) {
		// Simple expression parser for MVP
		result, err := e.evaluateExpression(expression, params)
		if err != nil {
			return false, err
		}
		return result, nil
	}, nil
}

// evaluateExpression evaluates a simple expression
func (e *engine) evaluateExpression(expr string, params map[string]interface{}) (interface{}, error) {
	// Remove spaces
	expr = strings.TrimSpace(expr)

	// Handle AND operations
	if strings.Contains(expr, " && ") {
		parts := strings.Split(expr, " && ")
		for _, part := range parts {
			result, err := e.evaluateExpression(part, params)
			if err != nil {
				return false, err
			}
			if boolResult, ok := result.(bool); !ok || !boolResult {
				return false, nil
			}
		}
		return true, nil
	}

	// Handle OR operations
	if strings.Contains(expr, " || ") {
		parts := strings.Split(expr, " || ")
		for _, part := range parts {
			result, err := e.evaluateExpression(part, params)
			if err != nil {
				return false, err
			}
			if boolResult, ok := result.(bool); ok && boolResult {
				return true, nil
			}
		}
		return false, nil
	}

	// Parse comparison operations
	var field, op, value string
	if strings.Contains(expr, " == ") {
		parts := strings.SplitN(expr, " == ", 2)
		field, op, value = parts[0], "==", parts[1]
	} else if strings.Contains(expr, " != ") {
		parts := strings.SplitN(expr, " != ", 2)
		field, op, value = parts[0], "!=", parts[1]
	} else if strings.Contains(expr, " >= ") {
		parts := strings.SplitN(expr, " >= ", 2)
		field, op, value = parts[0], ">=", parts[1]
	} else if strings.Contains(expr, " <= ") {
		parts := strings.SplitN(expr, " <= ", 2)
		field, op, value = parts[0], "<=", parts[1]
	} else if strings.Contains(expr, " > ") {
		parts := strings.SplitN(expr, " > ", 2)
		field, op, value = parts[0], ">", parts[1]
	} else if strings.Contains(expr, " < ") {
		parts := strings.SplitN(expr, " < ", 2)
		field, op, value = parts[0], "<", parts[1]
	} else {
		return false, fmt.Errorf("unsupported expression: %s", expr)
	}

	// Clean field and value
	field = strings.TrimSpace(field)
	value = strings.TrimSpace(value)

	// Get field value from params
	fieldValue, exists := params[field]
	if !exists {
		return false, nil
	}

	// Parse value (handle strings and numbers)
	var parsedValue interface{}
	if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
		// String value
		parsedValue = strings.Trim(value, "'")
	} else if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		// String value with double quotes
		parsedValue = strings.Trim(value, "\"")
	} else if v, err := strconv.ParseFloat(value, 64); err == nil {
		// Numeric value
		parsedValue = v
	} else if v, err := strconv.ParseBool(value); err == nil {
		// Boolean value
		parsedValue = v
	} else {
		// Default to string
		parsedValue = value
	}

	// Perform comparison
	return e.compare(fieldValue, op, parsedValue)
}

// compare performs comparison between two values
func (e *engine) compare(left interface{}, op string, right interface{}) (bool, error) {
	switch op {
	case "==":
		return e.equals(left, right), nil
	case "!=":
		return !e.equals(left, right), nil
	case ">":
		return e.greaterThan(left, right)
	case "<":
		return e.lessThan(left, right)
	case ">=":
		gt, err := e.greaterThan(left, right)
		if err != nil {
			return false, err
		}
		return gt || e.equals(left, right), nil
	case "<=":
		lt, err := e.lessThan(left, right)
		if err != nil {
			return false, err
		}
		return lt || e.equals(left, right), nil
	default:
		return false, fmt.Errorf("unsupported operator: %s", op)
	}
}

// equals checks if two values are equal
func (e *engine) equals(left, right interface{}) bool {
	// Handle numeric comparisons
	leftNum, leftIsNum := e.toFloat64(left)
	rightNum, rightIsNum := e.toFloat64(right)
	if leftIsNum && rightIsNum {
		return leftNum == rightNum
	}

	// String comparison
	return fmt.Sprintf("%v", left) == fmt.Sprintf("%v", right)
}

// greaterThan checks if left > right
func (e *engine) greaterThan(left, right interface{}) (bool, error) {
	leftNum, leftIsNum := e.toFloat64(left)
	rightNum, rightIsNum := e.toFloat64(right)
	if !leftIsNum || !rightIsNum {
		return false, fmt.Errorf("cannot compare non-numeric values with >")
	}
	return leftNum > rightNum, nil
}

// lessThan checks if left < right
func (e *engine) lessThan(left, right interface{}) (bool, error) {
	leftNum, leftIsNum := e.toFloat64(left)
	rightNum, rightIsNum := e.toFloat64(right)
	if !leftIsNum || !rightIsNum {
		return false, fmt.Errorf("cannot compare non-numeric values with <")
	}
	return leftNum < rightNum, nil
}

// toFloat64 converts a value to float64 if possible
func (e *engine) toFloat64(val interface{}) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}
