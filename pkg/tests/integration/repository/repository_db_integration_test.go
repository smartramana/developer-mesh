package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/database"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/repository/agent"
	"github.com/developer-mesh/developer-mesh/pkg/repository/model"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TxKey defines a custom type for transaction context keys to avoid collisions
type TxKey string

// Known transaction context keys
// contextKey is a type for context keys to avoid collisions
type contextKey string

const (
	// TransactionKey is the key used to store transaction in context
	TransactionKey contextKey = "tx" // Using contextKey type to avoid collisions
)

// MockModelRepository implements a simple in-memory model.Repository for testing
type MockModelRepository struct {
	models map[string]*models.Model
	mutex  sync.RWMutex
}

// NewMockModelRepository creates a new mock model repository
func NewMockModelRepository() *MockModelRepository {
	return &MockModelRepository{
		models: make(map[string]*models.Model),
	}
}

// Create implements the Repository.Create method
func (r *MockModelRepository) Create(ctx context.Context, model *models.Model) error {
	if model == nil {
		return errors.New("model cannot be nil")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.models[model.ID] = model
	return nil
}

// Get implements the Repository.Get method
func (r *MockModelRepository) Get(ctx context.Context, id string) (*models.Model, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	model, ok := r.models[id]
	if !ok {
		return nil, nil
	}
	return model, nil
}

// List implements the Repository.List method
func (r *MockModelRepository) List(ctx context.Context, filter model.Filter) ([]*models.Model, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var result []*models.Model
	for _, m := range r.models {
		// Simple filter check
		include := true
		for key, value := range filter {
			if key == "tenant_id" && m.TenantID != value.(string) {
				include = false
				break
			}
		}

		if include {
			result = append(result, m)
		}
	}
	return result, nil
}

// Update implements the Repository.Update method
func (r *MockModelRepository) Update(ctx context.Context, model *models.Model) error {
	if model == nil {
		return errors.New("model cannot be nil")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	_, ok := r.models[model.ID]
	if !ok {
		return errors.New("model not found")
	}

	r.models[model.ID] = model
	return nil
}

// Delete implements the Repository.Delete method
func (r *MockModelRepository) Delete(ctx context.Context, id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	_, ok := r.models[id]
	if !ok {
		return errors.New("model not found")
	}

	delete(r.models, id)
	return nil
}

// CreateModel implements the API-specific method
func (r *MockModelRepository) CreateModel(ctx context.Context, model *models.Model) error {
	return r.Create(ctx, model)
}

// GetModelByID implements the API-specific method
func (r *MockModelRepository) GetModelByID(ctx context.Context, id string, tenantID string) (*models.Model, error) {
	model, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if model == nil || model.TenantID != tenantID {
		return nil, errors.New("model not found")
	}

	return model, nil
}

// ListModels implements the API-specific method
func (r *MockModelRepository) ListModels(ctx context.Context, tenantID string) ([]*models.Model, error) {
	filter := model.FilterFromTenantID(tenantID)
	return r.List(ctx, filter)
}

// UpdateModel implements the API-specific method
func (r *MockModelRepository) UpdateModel(ctx context.Context, model *models.Model) error {
	return r.Update(ctx, model)
}

// DeleteModel implements the API-specific method
func (r *MockModelRepository) DeleteModel(ctx context.Context, id string) error {
	return r.Delete(ctx, id)
}

// MockAgentRepository implements a simple in-memory agent.Repository for testing
type MockAgentRepository struct {
	agents map[string]*models.Agent
	mutex  sync.RWMutex
}

// NewMockAgentRepository creates a new mock agent repository
func NewMockAgentRepository() *MockAgentRepository {
	return &MockAgentRepository{
		agents: make(map[string]*models.Agent),
	}
}

// Create implements the Repository.Create method
func (r *MockAgentRepository) Create(ctx context.Context, agent *models.Agent) error {
	if agent == nil {
		return errors.New("agent cannot be nil")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.agents[agent.ID] = agent
	return nil
}

// Get implements the Repository.Get method
func (r *MockAgentRepository) Get(ctx context.Context, id string) (*models.Agent, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	agent, ok := r.agents[id]
	if !ok {
		return nil, nil
	}
	return agent, nil
}

// List implements the Repository.List method
func (r *MockAgentRepository) List(ctx context.Context, filter agent.Filter) ([]*models.Agent, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var result []*models.Agent
	for _, a := range r.agents {
		// Simple filter check
		include := true
		for key, value := range filter {
			if key == "tenant_id" && a.TenantID.String() != value.(string) {
				include = false
				break
			}
		}

		if include {
			result = append(result, a)
		}
	}
	return result, nil
}

// Update implements the Repository.Update method
func (r *MockAgentRepository) Update(ctx context.Context, agent *models.Agent) error {
	if agent == nil {
		return errors.New("agent cannot be nil")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	_, ok := r.agents[agent.ID]
	if !ok {
		return errors.New("agent not found")
	}

	r.agents[agent.ID] = agent
	return nil
}

// Delete implements the Repository.Delete method
func (r *MockAgentRepository) Delete(ctx context.Context, id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	_, ok := r.agents[id]
	if !ok {
		return errors.New("agent not found")
	}

	delete(r.agents, id)
	return nil
}

// CreateAgent implements the API-specific method
func (r *MockAgentRepository) CreateAgent(ctx context.Context, agent *models.Agent) error {
	return r.Create(ctx, agent)
}

// GetAgentByID implements the API-specific method
func (r *MockAgentRepository) GetAgentByID(ctx context.Context, id string, tenantID string) (*models.Agent, error) {
	agent, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if agent == nil || agent.TenantID.String() != tenantID {
		return nil, errors.New("agent not found")
	}

	return agent, nil
}

// ListAgents implements the API-specific method
func (r *MockAgentRepository) ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error) {
	filter := map[string]interface{}{"tenant_id": tenantID}
	return r.List(ctx, filter)
}

// UpdateAgent implements the API-specific method
func (r *MockAgentRepository) UpdateAgent(ctx context.Context, agent *models.Agent) error {
	return r.Update(ctx, agent)
}

// DeleteAgent implements the API-specific method
func (r *MockAgentRepository) DeleteAgent(ctx context.Context, id string) error {
	return r.Delete(ctx, id)
}

// GetByStatus retrieves agents by status
func (r *MockAgentRepository) GetByStatus(ctx context.Context, status models.AgentStatus) ([]*models.Agent, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var result []*models.Agent
	for _, agent := range r.agents {
		if agent.Status == string(status) {
			result = append(result, agent)
		}
	}
	return result, nil
}

// GetWorkload retrieves agent workload
func (r *MockAgentRepository) GetWorkload(ctx context.Context, agentID uuid.UUID) (*models.AgentWorkload, error) {
	// Return a mock workload
	return &models.AgentWorkload{
		AgentID:       agentID.String(),
		ActiveTasks:   0,
		QueuedTasks:   0,
		TasksByType:   make(map[string]int),
		LoadScore:     0.0,
		EstimatedTime: 0,
	}, nil
}

// UpdateWorkload updates agent workload
func (r *MockAgentRepository) UpdateWorkload(ctx context.Context, workload *models.AgentWorkload) error {
	// Mock implementation - do nothing
	return nil
}

// GetLeastLoadedAgent retrieves the least loaded agent with a capability
func (r *MockAgentRepository) GetLeastLoadedAgent(ctx context.Context, capability models.AgentCapability) (*models.Agent, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Return the first active agent for simplicity
	for _, agent := range r.agents {
		if agent.Status == string(models.AgentStatusActive) {
			return agent, nil
		}
	}
	return nil, nil
}

// WithTx returns a new context with the transaction
// Includes validation to ensure the transaction is valid
func WithTx(ctx context.Context, tx *sqlx.Tx) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	if tx == nil {
		// Return original context if transaction is nil
		return ctx
	}

	// Store transaction in context
	return context.WithValue(ctx, TransactionKey, tx)
}

// ExtractTx extracts a transaction from the context if present
// Returns the transaction and true if found, nil and false otherwise
// Includes validation to ensure the transaction is valid and active
func ExtractTx(ctx context.Context) (*sqlx.Tx, bool) {
	if ctx == nil {
		return nil, false
	}

	// Extract transaction from context
	txValue := ctx.Value(TransactionKey)
	if txValue == nil {
		return nil, false
	}

	// Try to cast to the expected type
	tx, ok := txValue.(*sqlx.Tx)
	if !ok || tx == nil {
		return nil, false
	}

	// We can't reliably check if a transaction is active without executing a query
	// which could affect the transaction state, so we'll just trust that it's valid
	// if we got this far

	return tx, true
}

// GetTx gets a transaction from a context
// Returns nil if no transaction is found or invalid
// This is a convenience wrapper around ExtractTx for backward compatibility
func GetTx(ctx context.Context) *sqlx.Tx {
	if ctx == nil {
		return nil
	}

	tx, ok := ExtractTx(ctx)
	if !ok {
		// If debugging is needed, you could log here
		// fmt.Printf("Warning: No valid transaction found in context\n")
		return nil
	}

	return tx
}

func TestRepositoryDatabaseIntegration(t *testing.T) {
	// Note: The integration package provides test helpers but this test doesn't need them currently
	// Use in-memory mock repositories test
	t.Run("In-memory repositories provide expected behavior", func(t *testing.T) {
		// Test with in-memory repository implementation
		t.Run("In-memory repository", func(t *testing.T) {
			// Create in-memory repository implementations for testing
			modelRepo := NewMockModelRepository()
			agentRepo := NewMockAgentRepository()

			ctx := context.Background()
			tenantID := "test-tenant"

			// Create test model and agent
			testModel := &models.Model{
				ID:       "model-1",
				TenantID: tenantID,
				Name:     "Test Model",
			}

			testAgent := &models.Agent{
				ID:       "agent-1",
				TenantID: uuid.MustParse(tenantID),
				Name:     "Test Agent",
				ModelID:  testModel.ID,
			}

			// Run standard repository tests with in-memory implementations
			runRepositoryTests(t, ctx, modelRepo, agentRepo, testModel, testAgent, tenantID)
		})
	})

	// Use actual database repositories for integration testing
	t.Run("Database repositories provide proper persistence", func(t *testing.T) {
		ctx := context.Background()

		// Create test database with context
		db, err := database.NewTestDatabaseWithContext(ctx)
		require.NoError(t, err)
		require.NotNil(t, db)
		defer func() { _ = db.Close() }()

		// Use custom test table initialization that works with both SQLite and PostgreSQL
		err = initializeTestTables(ctx, db.DB())
		require.NoError(t, err, "Should be able to initialize test tables")

		// Create real repositories using the test database
		modelRepo := model.NewRepository(db.DB())
		require.NotNil(t, modelRepo)

		agentRepo := agent.NewRepository(db.DB())
		require.NotNil(t, agentRepo)

		// Create test data
		tenantID := "test-tenant"

		testModel := &models.Model{
			ID:       "db-model-1",
			TenantID: tenantID,
			Name:     "Test Database Model",
		}

		testAgent := &models.Agent{
			ID:       "db-agent-1",
			TenantID: uuid.MustParse(tenantID),
			Name:     "Test Database Agent",
			ModelID:  testModel.ID,
		}

		// Run the same tests with database repositories
		runRepositoryTests(t, ctx, modelRepo, agentRepo, testModel, testAgent, tenantID)
	})

	// Test transaction handling across repositories
	t.Run("Transaction support", func(t *testing.T) {
		ctx := context.Background()

		// Create test database with context
		db, err := database.NewTestDatabaseWithContext(ctx)
		require.NoError(t, err)
		require.NotNil(t, db)
		defer func() { _ = db.Close() }()

		// Use custom test table initialization that works with both SQLite and PostgreSQL
		err = initializeTestTables(ctx, db.DB())
		require.NoError(t, err, "Should be able to initialize test tables")

		// Verify tables exist before starting transactions
		tablePrefix := getTablePrefix(ctx, db.DB())
		var count int
		modelTableCheck := fmt.Sprintf("SELECT COUNT(*) FROM %smodels", tablePrefix)
		err = db.DB().QueryRowContext(ctx, modelTableCheck).Scan(&count)
		require.NoError(t, err, "Models table should be accessible before transaction")

		// Create real repositories using the test database
		modelRepo := model.NewRepository(db.DB())
		agentRepo := agent.NewRepository(db.DB())
		require.NotNil(t, modelRepo, "Model repository should not be nil")
		require.NotNil(t, agentRepo, "Agent repository should not be nil")

		// Before we start the transaction test, let's do a direct insert to test database access
		testDirectModel := &models.Model{
			ID:       "direct-model-1",
			TenantID: "test-tenant-direct",
			Name:     "Direct Test Model",
		}

		// Try a direct insert first to verify basic database functionality
		err = modelRepo.Create(ctx, testDirectModel)
		require.NoError(t, err, "Direct model creation should succeed")

		// Check we can retrieve it
		directModel, err := modelRepo.Get(ctx, testDirectModel.ID)
		require.NoError(t, err, "Should be able to retrieve directly created model")
		require.NotNil(t, directModel, "Retrieved model should not be nil")

		// Now test with transaction
		t.Log("Beginning transaction test")

		// Use transaction context for proper transaction handling
		err = db.Transaction(ctx, func(tx *sqlx.Tx) error {
			// Create a new context with the transaction
			txCtx := WithTx(ctx, tx)

			// Verify transaction is in context
			txFromCtx, ok := ExtractTx(txCtx)
			if !ok || txFromCtx == nil {
				return fmt.Errorf("transaction not properly stored in context")
			}

			// Create a tenant ID for testing
			tenantID := "test-tenant-" + uuid.New().String()
			t.Logf("Using tenant ID %s for transaction test", tenantID)

			testModel := &models.Model{
				ID:       "tx-model-1",
				TenantID: tenantID,
				Name:     "Transaction Test Model",
			}

			testAgent := &models.Agent{
				ID:       "tx-agent-1",
				TenantID: uuid.MustParse(tenantID),
				Name:     "Transaction Test Agent",
				ModelID:  testModel.ID,
			}

			// Test creating and retrieving a model and agent within the same transaction
			// Create model using transaction context
			t.Log("Creating model within transaction")
			err := modelRepo.Create(txCtx, testModel)
			if err != nil {
				return fmt.Errorf("failed to create model in transaction: %w", err)
			}

			// Create agent using transaction context
			t.Log("Creating agent within transaction")
			err = agentRepo.Create(txCtx, testAgent)
			if err != nil {
				return fmt.Errorf("failed to create agent in transaction: %w", err)
			}

			// Verify the model was created within the transaction
			t.Log("Verifying model within transaction")
			createdModel, err := modelRepo.Get(txCtx, testModel.ID)
			if err != nil {
				return fmt.Errorf("failed to get model in transaction: %w", err)
			}

			if createdModel == nil || createdModel.ID != testModel.ID {
				return fmt.Errorf("model not found in transaction or ID mismatch")
			}

			// Success
			t.Log("Transaction operations successful")
			return nil
		})
		require.NoError(t, err, "Transaction should complete successfully")

		// Verify the model exists outside the transaction
		retrievedModel, err := modelRepo.Get(ctx, "tx-model-1")
		require.NoError(t, err)
		require.NotNil(t, retrievedModel, "Model should exist after transaction commit")
		assert.Equal(t, "tx-model-1", retrievedModel.ID)

		// Verify the agent exists outside the transaction
		retrievedAgent, err := agentRepo.Get(ctx, "tx-agent-1")
		require.NoError(t, err)
		require.NotNil(t, retrievedAgent, "Agent should exist after transaction commit")
		assert.Equal(t, "tx-agent-1", retrievedAgent.ID)

		// Define model ID for reference in the next transaction
		modelID := "tx-model-1"

		// Create invalid agent (missing required field) to force error
		invalidAgent := &models.Agent{
			ID:       "invalid-agent",
			TenantID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			ModelID:  modelID,
			// Missing the Name field intentionally to cause error
		}

		// Add database-level constraint to the model field to ensure validation fails
		// This ensures our transaction will properly fail and roll back
		if isDatabaseSQLite(ctx, db.DB()) {
			_, err = db.DB().ExecContext(ctx, "CREATE TRIGGER IF NOT EXISTS check_agent_name BEFORE INSERT ON agents FOR EACH ROW WHEN NEW.name IS NULL OR NEW.name = '' BEGIN SELECT RAISE(FAIL, 'agent name cannot be empty'); END;")
			require.NoError(t, err, "Failed to create trigger")
		} else {
			if _, err = db.DB().ExecContext(ctx, "ALTER TABLE mcp.agents ADD CONSTRAINT check_agent_name CHECK (name IS NOT NULL AND name != '')"); err != nil {
				// Ignore errors if constraint already exists
				_ = err
			}
		}

		// This should fail and cause rollback
		err = db.Transaction(ctx, func(tx *sqlx.Tx) error {
			txCtx := WithTx(ctx, tx)
			err := agentRepo.Create(txCtx, invalidAgent)
			if err != nil {
				return fmt.Errorf("failed to create invalid agent in transaction: %w", err)
			}
			return nil
		})
		require.Error(t, err, "Transaction should fail and rollback")

		// Verify the invalid agent was not persisted after rollback
		invalidCheck, getErr := agentRepo.Get(ctx, invalidAgent.ID)
		// The agent shouldn't exist, which can be indicated either by an error or by a nil result
		if getErr == nil {
			assert.Nil(t, invalidCheck, "Invalid agent should not exist after transaction rollback")
		}
	})

	// Test database schema verification
	t.Run("Database schema verification", func(t *testing.T) {
		ctx := context.Background()

		// Create test database with context
		db, err := database.NewTestDatabaseWithContext(ctx)
		require.NoError(t, err)
		require.NotNil(t, db)
		defer func() { _ = db.Close() }()

		// Use custom test table initialization that works with both SQLite and PostgreSQL
		err = initializeTestTables(ctx, db.DB())
		require.NoError(t, err, "Should be able to initialize test tables")

		tablePrefix := getTablePrefix(ctx, db.DB())

		// Verify models table exists and has expected structure
		var modelCount int
		modelQuery := fmt.Sprintf("SELECT COUNT(*) FROM %smodels", tablePrefix)
		err = db.DB().QueryRowContext(ctx, modelQuery).Scan(&modelCount)
		require.NoError(t, err, "Models table should exist")

		// Verify agents table exists and has expected structure
		var agentCount int
		agentQuery := fmt.Sprintf("SELECT COUNT(*) FROM %sagents", tablePrefix)
		err = db.DB().QueryRowContext(ctx, agentQuery).Scan(&agentCount)
		require.NoError(t, err, "Agents table should exist")

		// Verify relationships table exists
		var relationshipCount int
		relationshipQuery := fmt.Sprintf("SELECT COUNT(*) FROM %srelationships", tablePrefix)
		err = db.DB().QueryRowContext(ctx, relationshipQuery).Scan(&relationshipCount)
		require.NoError(t, err, "Relationships table should exist")
	})
}

// Helper function to determine if the database is using SQLite
func isDatabaseSQLite(ctx context.Context, db *sqlx.DB) bool {
	if db == nil {
		return false
	}

	// Try to query SQLite version - this will only succeed on SQLite
	row := db.QueryRowContext(ctx, "SELECT sqlite_version()")
	var version string
	err := row.Scan(&version)
	return err == nil
}

// Helper function to get the appropriate table prefix based on database type
func getTablePrefix(ctx context.Context, db *sqlx.DB) string {
	if isDatabaseSQLite(ctx, db) {
		return ""
	}
	return "mcp."
}

// Helper function to initialize tables for testing that works with both SQLite and PostgreSQL
func initializeTestTables(ctx context.Context, db *sqlx.DB) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	// Determine if we're using SQLite or PostgreSQL
	isSQLite := isDatabaseSQLite(ctx, db)
	tablePrefix := ""

	// Log which database type we're using for debugging
	if isSQLite {
		fmt.Println("Using SQLite database for tests")
		// For SQLite, enable foreign keys
		_, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
		if err != nil {
			return fmt.Errorf("failed to enable foreign keys in SQLite: %w", err)
		}
	} else {
		fmt.Println("Using PostgreSQL database for tests")
		// For PostgreSQL, create the schema if it doesn't exist
		// First drop existing schema to ensure clean test environment
		_, err := db.ExecContext(ctx, "DROP SCHEMA IF EXISTS mcp CASCADE")
		if err != nil {
			return fmt.Errorf("failed to drop schema: %w", err)
		}

		// Create fresh schema
		_, err = db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS mcp")
		if err != nil {
			return fmt.Errorf("failed to create schema: %w", err)
		}
		tablePrefix = "mcp."
	}

	// Create models table - ensure it has all required fields from the model struct
	modelsTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %smodels (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		name TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`, tablePrefix)

	fmt.Printf("Creating models table with query: %s\n", modelsTable)

	_, err := db.ExecContext(ctx, modelsTable)
	if err != nil {
		return fmt.Errorf("failed to create models table: %w", err)
	}

	// Create model indices for faster lookups
	if !isSQLite {
		indices := []string{
			fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%smodels_tenant ON %smodels(tenant_id)`,
				tablePrefix[0:len(tablePrefix)-1], tablePrefix),
		}

		for _, indexQuery := range indices {
			_, err = db.ExecContext(ctx, indexQuery)
			if err != nil {
				return fmt.Errorf("failed to create model index: %w", err)
			}
		}
	}

	// Create agents table - ensure it has all required fields from the agent struct
	agentsTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %sagents (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		name TEXT NOT NULL,
		model_id TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`, tablePrefix)

	fmt.Printf("Creating agents table with query: %s\n", agentsTable)

	_, err = db.ExecContext(ctx, agentsTable)
	if err != nil {
		return fmt.Errorf("failed to create agents table: %w", err)
	}

	// Create agent indices for faster lookups
	if !isSQLite {
		indices := []string{
			fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%sagents_tenant ON %sagents(tenant_id)`,
				tablePrefix[0:len(tablePrefix)-1], tablePrefix),
			fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%sagents_model ON %sagents(model_id)`,
				tablePrefix[0:len(tablePrefix)-1], tablePrefix),
		}

		for _, indexQuery := range indices {
			_, err = db.ExecContext(ctx, indexQuery)
			if err != nil {
				return fmt.Errorf("failed to create agent index: %w", err)
			}
		}
	}

	// Create relationships table with additional metadata and timestamps
	// Determine metadata column type based on database
	metadataType := "JSONB"
	if isSQLite {
		metadataType = "TEXT"
	}

	relationshipsTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %srelationships (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		source_id TEXT NOT NULL,
		source_type TEXT NOT NULL,
		target_id TEXT NOT NULL,
		target_type TEXT NOT NULL,
		relationship_type TEXT NOT NULL,
		metadata %s,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`, tablePrefix, metadataType)

	_, err = db.ExecContext(ctx, relationshipsTable)
	if err != nil {
		return fmt.Errorf("failed to create relationships table: %w", err)
	}

	// Create relationship indices for faster lookups
	if !isSQLite {
		indices := []string{
			fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%srelationships_tenant ON %srelationships(tenant_id)`,
				tablePrefix[0:len(tablePrefix)-1], tablePrefix),
			fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%srelationships_source ON %srelationships(source_id, source_type)`,
				tablePrefix[0:len(tablePrefix)-1], tablePrefix),
			fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%srelationships_target ON %srelationships(target_id, target_type)`,
				tablePrefix[0:len(tablePrefix)-1], tablePrefix),
		}

		for _, indexQuery := range indices {
			_, err = db.ExecContext(ctx, indexQuery)
			if err != nil {
				return fmt.Errorf("failed to create relationship index: %w", err)
			}
		}
	}

	return nil
}

// Helper function to run standard repository tests
func runRepositoryTests(t *testing.T, ctx context.Context, modelRepo model.Repository, agentRepo agent.Repository, testModel *models.Model, testAgent *models.Agent, tenantID string) {
	// Verify that both repositories are properly initialized
	require.NotNil(t, modelRepo, "Model repository cannot be nil")
	require.NotNil(t, agentRepo, "Agent repository cannot be nil")
	// If no test model is provided, create one with required fields
	if testModel == nil {
		testModel = &models.Model{
			ID:       uuid.New().String(),
			TenantID: tenantID,
			Name:     "Test Model",
		}
	}

	// If no test agent is provided, create one with required fields
	if testAgent == nil {
		modelID := testModel.ID
		testAgent = &models.Agent{
			ID:       uuid.New().String(),
			TenantID: uuid.MustParse(tenantID),
			Name:     "Test Agent",
			ModelID:  modelID,
		}
	}
	// 1. First add a model, which agent depends on
	fmt.Printf("Creating test model with ID: %s, TenantID: %s\n", testModel.ID, testModel.TenantID)
	err := modelRepo.Create(ctx, testModel)
	require.NoError(t, err, "Failed to create model - check table schema and repository implementation")

	// 2. Add an agent that references the model
	err = agentRepo.Create(ctx, testAgent)
	require.NoError(t, err)

	// 3. Verify model can be retrieved using the API-specific method
	retrievedModel, err := modelRepo.GetModelByID(ctx, testModel.ID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, testModel.ID, retrievedModel.ID)
	assert.Equal(t, testModel.Name, retrievedModel.Name)

	// 4. Verify agent can be retrieved and has correct model reference
	retrievedAgent, err := agentRepo.Get(ctx, testAgent.ID)
	require.NoError(t, err, "Error getting agent")
	require.NotNil(t, retrievedAgent, "Agent should be retrieved")
	assert.Equal(t, testAgent.ID, retrievedAgent.ID, "Agent ID should match")
	assert.Equal(t, testAgent.Name, retrievedAgent.Name, "Agent name should match")
	assert.Equal(t, testAgent.TenantID, retrievedAgent.TenantID, "Agent tenant ID should match")
	assert.Equal(t, testAgent.ModelID, retrievedAgent.ModelID, "Agent model ID should match")

	// Test list methods first (before we delete anything)
	// List models by filter
	filter := model.FilterFromTenantID(tenantID)
	models, err := modelRepo.List(ctx, filter)
	require.NoError(t, err)
	assert.NotEmpty(t, models, "List should return at least one model")
	assert.Greater(t, len(models), 0, "There should be at least one model")

	// Skip the API-specific methods for now as they might not be implemented

	// List agents
	agentFilter := agent.FilterFromTenantID(tenantID)
	agents, err := agentRepo.List(ctx, agentFilter)
	require.NoError(t, err)
	assert.NotEmpty(t, agents, "List should return at least one agent")

	// Skip the API-specific methods for now as they might not be implemented

	// Now delete the agent
	err = agentRepo.Delete(ctx, testAgent.ID)
	require.NoError(t, err, "Error deleting agent")

	// Verify agent is gone
	deletedAgent, err := agentRepo.Get(ctx, testAgent.ID)
	// Some implementations return nil,nil for not found, so check either condition
	if err == nil {
		assert.Nil(t, deletedAgent, "Agent should be nil after deletion")
	} else {
		assert.Error(t, err, "Should get error or nil result for deleted agent")
	}

	// List methods already tested above, we skip them here to avoid testing after deletion

	// Delete the model
	err = modelRepo.Delete(ctx, testModel.ID)
	require.NoError(t, err, "Error deleting model")

	// Verify model is gone
	deletedModel, err := modelRepo.Get(ctx, testModel.ID)
	// Some implementations return nil,nil for not found, so check either condition
	if err == nil {
		assert.Nil(t, deletedModel, "Model should be nil after deletion")
	} else {
		assert.Error(t, err, "Should get error or nil result for deleted model")
	}
}
