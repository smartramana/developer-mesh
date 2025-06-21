package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/google/uuid"
	"gopkg.in/yaml.v2"
)

// ConfigFileRuleLoader loads rules from configuration files
type ConfigFileRuleLoader struct {
	configPath string
	logger     observability.Logger
}

// NewConfigFileRuleLoader creates a new configuration file rule loader
func NewConfigFileRuleLoader(configPath string, logger observability.Logger) RuleLoader {
	return &ConfigFileRuleLoader{
		configPath: configPath,
		logger:     logger,
	}
}

// LoadRules loads rules from configuration files
func (l *ConfigFileRuleLoader) LoadRules(ctx context.Context) ([]Rule, error) {
	// Check if path exists
	info, err := os.Stat(l.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to access config path: %w", err)
	}

	var rules []Rule

	if info.IsDir() {
		// Load all rule files from directory
		err = filepath.Walk(l.configPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && (filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml" || filepath.Ext(path) == ".json") {
				fileRules, err := l.loadRulesFromFile(path)
				if err != nil {
					l.logger.Warn("Failed to load rules from file", map[string]interface{}{
						"file":  path,
						"error": err.Error(),
					})
					return nil // Continue with other files
				}
				rules = append(rules, fileRules...)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to walk directory: %w", err)
		}
	} else {
		// Load from single file
		rules, err = l.loadRulesFromFile(l.configPath)
		if err != nil {
			return nil, err
		}
	}

	return rules, nil
}

// loadRulesFromFile loads rules from a single file
func (l *ConfigFileRuleLoader) loadRulesFromFile(path string) ([]Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var rules []Rule

	switch filepath.Ext(path) {
	case ".json":
		if err := json.Unmarshal(data, &rules); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &rules); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported file format: %s", filepath.Ext(path))
	}

	// Ensure all rules have IDs
	for i := range rules {
		if rules[i].ID == uuid.Nil {
			rules[i].ID = uuid.New()
		}
	}

	return rules, nil
}

// DatabaseRuleLoader loads rules from database
type DatabaseRuleLoader struct {
	db     *database.Database
	logger observability.Logger
}

// NewDatabaseRuleLoader creates a new database rule loader
func NewDatabaseRuleLoader(db *database.Database, logger observability.Logger) RuleLoader {
	return &DatabaseRuleLoader{
		db:     db,
		logger: logger,
	}
}

// LoadRules loads rules from database
func (l *DatabaseRuleLoader) LoadRules(ctx context.Context) ([]Rule, error) {
	query := `
		SELECT id, name, category, expression, priority, enabled, metadata
		FROM rules
		WHERE enabled = true
		ORDER BY priority ASC
	`

	rows, err := l.db.GetDB().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query rules: %w", err)
	}
	defer rows.Close()

	var rules []Rule
	for rows.Next() {
		var rule Rule
		var metadataJSON []byte

		err := rows.Scan(
			&rule.ID,
			&rule.Name,
			&rule.Category,
			&rule.Expression,
			&rule.Priority,
			&rule.Enabled,
			&metadataJSON,
		)
		if err != nil {
			l.logger.Warn("Failed to scan rule", map[string]interface{}{
				"error": err.Error(),
			})
			continue
		}

		// Parse metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &rule.Metadata); err != nil {
				l.logger.Warn("Failed to parse rule metadata", map[string]interface{}{
					"rule":  rule.Name,
					"error": err.Error(),
				})
			}
		}

		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rules: %w", err)
	}

	return rules, nil
}

// ConfigFilePolicyLoader loads policies from configuration files
type ConfigFilePolicyLoader struct {
	configPath string
	logger     observability.Logger
}

// NewConfigFilePolicyLoader creates a new configuration file policy loader
func NewConfigFilePolicyLoader(configPath string, logger observability.Logger) PolicyLoader {
	return &ConfigFilePolicyLoader{
		configPath: configPath,
		logger:     logger,
	}
}

// LoadPolicies loads policies from configuration files
func (l *ConfigFilePolicyLoader) LoadPolicies(ctx context.Context) ([]Policy, error) {
	// Check if path exists
	info, err := os.Stat(l.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to access config path: %w", err)
	}

	var policies []Policy

	if info.IsDir() {
		// Load all policy files from directory
		err = filepath.Walk(l.configPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && (filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml" || filepath.Ext(path) == ".json") {
				filePolicies, err := l.loadPoliciesFromFile(path)
				if err != nil {
					l.logger.Warn("Failed to load policies from file", map[string]interface{}{
						"file":  path,
						"error": err.Error(),
					})
					return nil // Continue with other files
				}
				policies = append(policies, filePolicies...)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to walk directory: %w", err)
		}
	} else {
		// Load from single file
		policies, err = l.loadPoliciesFromFile(l.configPath)
		if err != nil {
			return nil, err
		}
	}

	return policies, nil
}

// loadPoliciesFromFile loads policies from a single file
func (l *ConfigFilePolicyLoader) loadPoliciesFromFile(path string) ([]Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var policies []Policy

	switch filepath.Ext(path) {
	case ".json":
		if err := json.Unmarshal(data, &policies); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &policies); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported file format: %s", filepath.Ext(path))
	}

	// Ensure all policies have IDs
	for i := range policies {
		if policies[i].ID == uuid.Nil {
			policies[i].ID = uuid.New()
		}
	}

	return policies, nil
}

// DatabasePolicyLoader loads policies from database
type DatabasePolicyLoader struct {
	db     *database.Database
	logger observability.Logger
}

// NewDatabasePolicyLoader creates a new database policy loader
func NewDatabasePolicyLoader(db *database.Database, logger observability.Logger) PolicyLoader {
	return &DatabasePolicyLoader{
		db:     db,
		logger: logger,
	}
}

// LoadPolicies loads policies from database
func (l *DatabasePolicyLoader) LoadPolicies(ctx context.Context) ([]Policy, error) {
	query := `
		SELECT id, name, resource, rules, defaults, version
		FROM policies
		WHERE enabled = true
		ORDER BY name ASC
	`

	rows, err := l.db.GetDB().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query policies: %w", err)
	}
	defer rows.Close()

	var policies []Policy
	for rows.Next() {
		var policy Policy
		var rulesJSON []byte
		var defaultsJSON []byte

		err := rows.Scan(
			&policy.ID,
			&policy.Name,
			&policy.Resource,
			&rulesJSON,
			&defaultsJSON,
			&policy.Version,
		)
		if err != nil {
			l.logger.Warn("Failed to scan policy", map[string]interface{}{
				"error": err.Error(),
			})
			continue
		}

		// Parse rules
		if len(rulesJSON) > 0 {
			if err := json.Unmarshal(rulesJSON, &policy.Rules); err != nil {
				l.logger.Warn("Failed to parse policy rules", map[string]interface{}{
					"policy": policy.Name,
					"error":  err.Error(),
				})
			}
		}

		// Parse defaults
		if len(defaultsJSON) > 0 {
			if err := json.Unmarshal(defaultsJSON, &policy.Defaults); err != nil {
				l.logger.Warn("Failed to parse policy defaults", map[string]interface{}{
					"policy": policy.Name,
					"error":  err.Error(),
				})
			}
		}

		policies = append(policies, policy)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating policies: %w", err)
	}

	return policies, nil
}