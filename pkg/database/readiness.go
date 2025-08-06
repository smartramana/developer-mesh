package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// ReadinessChecker checks if database tables are ready
type ReadinessChecker struct {
	db             *sqlx.DB
	requiredTables []string
	logger         func(string, ...interface{})
}

// NewReadinessChecker creates a new readiness checker
func NewReadinessChecker(db *sqlx.DB) *ReadinessChecker {
	return &ReadinessChecker{
		db: db,
		requiredTables: []string{
			"events",
			"contexts",
			"context_items",
			"integrations",
			"agents",
			"models",
			"tasks",
			"workflows",
			"workspaces",
			"workspace_members",
			"shared_documents",
			"tool_configurations",
			"webhook_configs",
			"webhook_dlq",
			"embeddings",
		},
		logger: log.Printf,
	}
}

// SetLogger sets a custom logger function
func (r *ReadinessChecker) SetLogger(logger func(string, ...interface{})) {
	r.logger = logger
}

// TablesExist checks if all required tables exist
func (r *ReadinessChecker) TablesExist(ctx context.Context) (bool, error) {
	query := `
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_schema = 'mcp'
		AND table_name = ANY($1)
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, pq.Array(r.requiredTables)).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check tables: %w", err)
	}

	return count == len(r.requiredTables), nil
}

// WaitForTables waits for all required tables to be created
func (r *ReadinessChecker) WaitForTables(ctx context.Context) error {
	r.logger("Waiting for database tables to be ready...")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(120 * time.Second) // 2 minutes timeout

	attempt := 0
	for {
		select {
		case <-ticker.C:
			attempt++
			exists, err := r.TablesExist(ctx)
			if err != nil {
				r.logger("Failed to check tables (attempt %d): %v", attempt, err)
				continue
			}

			if exists {
				r.logger("All required tables are ready")
				return nil
			}

			// Check which tables are missing for better logging
			missing := r.getMissingTables(ctx)
			r.logger("Waiting for tables (attempt %d), missing: %v", attempt, missing)

		case <-timeout:
			missing := r.getMissingTables(ctx)
			return fmt.Errorf("timeout waiting for tables after 120s, missing tables: %v", missing)

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// WaitForTablesWithBackoff waits with exponential backoff
func (r *ReadinessChecker) WaitForTablesWithBackoff(ctx context.Context) error {
	r.logger("Waiting for database tables with exponential backoff...")

	maxRetries := 10
	baseDelay := 1 * time.Second
	maxDelay := 32 * time.Second

	for i := 0; i < maxRetries; i++ {
		exists, err := r.TablesExist(ctx)
		if err != nil {
			r.logger("Failed to check tables (attempt %d/%d): %v", i+1, maxRetries, err)
		} else if exists {
			r.logger("All required tables are ready after %d attempts", i+1)
			return nil
		} else {
			missing := r.getMissingTables(ctx)
			r.logger("Tables not ready (attempt %d/%d), missing: %v", i+1, maxRetries, missing)
		}

		if i < maxRetries-1 {
			delay := baseDelay * (1 << uint(i)) // Exponential backoff: 1s, 2s, 4s, 8s, 16s, 32s
			if delay > maxDelay {
				delay = maxDelay
			}
			r.logger("Waiting %v before next attempt...", delay)

			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	missing := r.getMissingTables(ctx)
	return fmt.Errorf("failed to find required tables after %d attempts, missing: %v", maxRetries, missing)
}

// getMissingTables returns a list of tables that don't exist
func (r *ReadinessChecker) getMissingTables(ctx context.Context) []string {
	query := `
		SELECT table_name
		FROM unnest($1::text[]) AS required(table_name)
		WHERE NOT EXISTS (
			SELECT 1 
			FROM information_schema.tables 
			WHERE table_schema = 'mcp' 
			AND table_name = required.table_name
		)
	`

	var missing []string
	rows, err := r.db.QueryContext(ctx, query, pq.Array(r.requiredTables))
	if err != nil {
		r.logger("Failed to get missing tables: %v", err)
		return []string{"unknown"}
	}
	defer func() {
		if err := rows.Close(); err != nil {
			r.logger("Failed to close rows: %v", err)
		}
	}()

	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err == nil {
			missing = append(missing, table)
		}
	}

	return missing
}

// CheckSpecificTable checks if a specific table exists
func (r *ReadinessChecker) CheckSpecificTable(ctx context.Context, tableName string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.tables 
			WHERE table_schema = 'mcp' 
			AND table_name = $1
		)
	`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, tableName).Scan(&exists)
	return exists, err
}

// HealthCheck performs a health check on the database
func (r *ReadinessChecker) HealthCheck(ctx context.Context) error {
	// First check basic connectivity
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Then check if tables exist
	exists, err := r.TablesExist(ctx)
	if err != nil {
		return fmt.Errorf("failed to verify tables: %w", err)
	}

	if !exists {
		missing := r.getMissingTables(ctx)
		return fmt.Errorf("missing required tables: %v", missing)
	}

	return nil
}
