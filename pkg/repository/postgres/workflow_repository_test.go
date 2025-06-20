package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/S-Corkum/devops-mcp/pkg/cache"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository/interfaces"
)

// Enhanced mock cache that handles workflow types
type workflowMockCache struct {
	*mockCache
}

func (m *workflowMockCache) Get(ctx context.Context, key string, dest interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getCalls++
	
	if m.getErr != nil {
		return m.getErr
	}
	
	val, exists := m.data[key]
	if !exists {
		return cache.ErrNotFound
	}
	
	// Handle workflow types
	switch d := dest.(type) {
	case *models.Workflow:
		if w, ok := val.(*models.Workflow); ok {
			*d = *w
		}
	case *models.WorkflowExecution:
		if e, ok := val.(*models.WorkflowExecution); ok {
			*d = *e
		}
	case *interfaces.WorkflowStats:
		if s, ok := val.(*interfaces.WorkflowStats); ok {
			*d = *s
		}
	case *[]*models.WorkflowExecution:
		if e, ok := val.([]*models.WorkflowExecution); ok {
			*d = e
		}
	case *string:
		*d = val.(string)
	case *int:
		*d = val.(int)
	}
	
	return nil
}

func newWorkflowMockCache() *workflowMockCache {
	return &workflowMockCache{
		mockCache: newMockCache(),
	}
}

func setupWorkflowRepository(t *testing.T) (*workflowRepository, sqlmock.Sqlmock, *workflowMockCache, *mockMetricsClient) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	
	sqlxDB := sqlx.NewDb(db, "postgres")
	
	cache := newWorkflowMockCache()
	logger := observability.NewStandardLogger("test")
	tracer := observability.NoopStartSpan
	metrics := newMockMetricsClient()
	
	repo := NewWorkflowRepository(sqlxDB, sqlxDB, cache, logger, tracer, metrics).(*workflowRepository)
	
	return repo, mock, cache, metrics
}

func TestWorkflowRepository_Create(t *testing.T) {
	tests := []struct {
		name        string
		workflow    *models.Workflow
		setupMock   func(sqlmock.Sqlmock)
		checkCache  func(*testing.T, *workflowMockCache)
		wantErr     bool
		expectedErr error
	}{
		{
			name: "successful creation",
			workflow: &models.Workflow{
				TenantID:    uuid.New(),
				Name:        "Test Workflow",
				Type:        models.WorkflowTypeSequential,
				Description: "Test description",
				CreatedBy:   "user1",
				Agents:      models.JSONMap{"agent1": map[string]interface{}{"type": "test"}},
				Steps:       models.JSONMap{"step1": map[string]interface{}{"type": "action"}},
				Config:      models.JSONMap{"timeout": 300},
				Tags:        pq.StringArray{"test", "workflow"},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("INSERT INTO workflows").
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "Test Workflow", "Test description", 
						models.WorkflowTypeSequential, 1, "user1", sqlmock.AnyArg(), sqlmock.AnyArg(), 
						sqlmock.AnyArg(), pq.StringArray{"test", "workflow"}, true, 
						sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
			},
			checkCache: func(t *testing.T, c *workflowMockCache) {
				// Should have cleared tenant cache
				assert.Equal(t, 1, c.delCalls)
			},
			wantErr: false,
		},
		{
			name: "duplicate workflow",
			workflow: &models.Workflow{
				ID:       uuid.New(),
				TenantID: uuid.New(),
				Name:     "Duplicate Workflow",
				Type:     models.WorkflowTypeParallel,
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("INSERT INTO workflows").
					WillReturnError(sql.ErrNoRows)
			},
			wantErr:     true,
			expectedErr: interfaces.ErrDuplicate,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock, cache, _ := setupWorkflowRepository(t)
			defer repo.writeDB.Close()
			
			tt.setupMock(mock)
			
			err := repo.Create(context.Background(), tt.workflow)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.Equal(t, tt.expectedErr, err)
				}
			} else {
				assert.NoError(t, err)
				assert.NotEqual(t, uuid.Nil, tt.workflow.ID)
				assert.NotZero(t, tt.workflow.CreatedAt)
				assert.NotZero(t, tt.workflow.UpdatedAt)
				assert.Equal(t, 1, tt.workflow.Version)
			}
			
			if tt.checkCache != nil {
				tt.checkCache(t, cache)
			}
			
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestWorkflowRepository_Get(t *testing.T) {
	workflowID := uuid.New()
	tenantID := uuid.New()
	now := time.Now()
	
	tests := []struct {
		name        string
		id          uuid.UUID
		setupCache  func(*workflowMockCache)
		setupMock   func(sqlmock.Sqlmock)
		want        *models.Workflow
		wantErr     bool
		expectedErr error
	}{
		{
			name: "cache hit",
			id:   workflowID,
			setupCache: func(c *workflowMockCache) {
				workflow := &models.Workflow{
					ID:       workflowID,
					TenantID: tenantID,
					Name:     "Cached Workflow",
				}
				c.data["workflow:"+workflowID.String()] = workflow
			},
			want: &models.Workflow{
				ID:       workflowID,
				TenantID: tenantID,
				Name:     "Cached Workflow",
			},
			wantErr: false,
		},
		{
			name: "cache miss - database hit",
			id:   workflowID,
			setupCache: func(c *workflowMockCache) {
				// No cache entry
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				agentsJSON, _ := json.Marshal(models.JSONMap{"agent1": "config"})
				stepsJSON, _ := json.Marshal(models.JSONMap{"step1": "config"})
				configJSON, _ := json.Marshal(models.JSONMap{"key": "value"})
				
				mock.ExpectQuery("SELECT .* FROM workflows WHERE").
					WithArgs(workflowID).
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "tenant_id", "name", "description", "type",
						"version", "created_by", "agents", "steps",
						"config", "tags", "is_active", "created_at", "updated_at", "deleted_at",
					}).AddRow(
						workflowID, tenantID, "Test Workflow", "Description", "sequential",
						1, "user1", agentsJSON, stepsJSON,
						configJSON, pq.StringArray{"test"}, true, now, now, nil,
					))
			},
			want: &models.Workflow{
				ID:          workflowID,
				TenantID:    tenantID,
				Name:        "Test Workflow",
				Description: "Description",
				Type:        models.WorkflowTypeSequential,
				Version:     1,
				CreatedBy:   "user1",
				Agents:      models.JSONMap{"agent1": "config"},
				Steps:       models.JSONMap{"step1": "config"},
				Config:      models.JSONMap{"key": "value"},
				Tags:        pq.StringArray{"test"},
				IsActive:    true,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			wantErr: false,
		},
		{
			name: "workflow not found",
			id:   workflowID,
			setupCache: func(c *workflowMockCache) {
				// No cache entry
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .* FROM workflows WHERE").
					WithArgs(workflowID).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr:     true,
			expectedErr: interfaces.ErrNotFound,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock, cache, metrics := setupWorkflowRepository(t)
			defer repo.writeDB.Close()
			
			if tt.setupCache != nil {
				tt.setupCache(cache)
			}
			if tt.setupMock != nil {
				tt.setupMock(mock)
			}
			
			got, err := repo.Get(context.Background(), tt.id)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.Equal(t, tt.expectedErr, err)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want.ID, got.ID)
				assert.Equal(t, tt.want.Name, got.Name)
				assert.Equal(t, tt.want.TenantID, got.TenantID)
			}
			
			// Check metrics
			if !tt.wantErr {
				if tt.name == "cache hit" {
					assert.Equal(t, float64(1), metrics.counters["workflow_cache_hits"])
				} else if tt.name == "cache miss - database hit" {
					assert.Equal(t, float64(1), metrics.counters["workflow_cache_misses"])
				}
			}
			
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestWorkflowRepository_Update(t *testing.T) {
	workflowID := uuid.New()
	tenantID := uuid.New()
	
	tests := []struct {
		name        string
		workflow    *models.Workflow
		setupMock   func(sqlmock.Sqlmock)
		wantErr     bool
		expectedErr error
	}{
		{
			name: "successful update",
			workflow: &models.Workflow{
				ID:          workflowID,
				TenantID:    tenantID,
				Name:        "Updated Workflow",
				Description: "Updated description",
				Type:        models.WorkflowTypeParallel,
				Version:     1,
				IsActive:    true,
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE workflows SET").
					WithArgs("Updated Workflow", "Updated description", models.WorkflowTypeParallel,
						sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 
						sqlmock.AnyArg(), // tags
						true, // is_active
						sqlmock.AnyArg(), 2, workflowID, 1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "optimistic lock failure",
			workflow: &models.Workflow{
				ID:      workflowID,
				Version: 1,
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE workflows SET").
					WillReturnResult(sqlmock.NewResult(0, 0))
				// Check existence
				mock.ExpectQuery("SELECT EXISTS").
					WithArgs(workflowID).
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			},
			wantErr:     true,
			expectedErr: interfaces.ErrOptimisticLock,
		},
		{
			name: "workflow not found",
			workflow: &models.Workflow{
				ID:      workflowID,
				Version: 1,
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE workflows SET").
					WillReturnResult(sqlmock.NewResult(0, 0))
				// Check existence
				mock.ExpectQuery("SELECT EXISTS").
					WithArgs(workflowID).
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			},
			wantErr:     true,
			expectedErr: interfaces.ErrNotFound,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock, _, _ := setupWorkflowRepository(t)
			defer repo.writeDB.Close()
			
			tt.setupMock(mock)
			
			oldVersion := tt.workflow.Version
			err := repo.Update(context.Background(), tt.workflow)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.Equal(t, tt.expectedErr, err)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, oldVersion+1, tt.workflow.Version)
				assert.NotZero(t, tt.workflow.UpdatedAt)
			}
			
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestWorkflowRepository_Delete(t *testing.T) {
	workflowID := uuid.New()
	tenantID := uuid.New()
	
	tests := []struct {
		name        string
		id          uuid.UUID
		setupMock   func(sqlmock.Sqlmock)
		wantErr     bool
		expectedErr error
	}{
		{
			name: "successful delete",
			id:   workflowID,
			setupMock: func(mock sqlmock.Sqlmock) {
				// Begin transaction
				mock.ExpectBegin()
				// Check active executions
				mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM workflow_executions").
					WithArgs(workflowID).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				// Get workflow info
				mock.ExpectQuery("SELECT tenant_id, name FROM workflows").
					WithArgs(workflowID).
					WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "name"}).
						AddRow(tenantID, "Test Workflow"))
				// Delete workflow
				mock.ExpectExec("DELETE FROM workflows").
					WithArgs(workflowID).
					WillReturnResult(sqlmock.NewResult(0, 1))
				// Commit
				mock.ExpectCommit()
			},
			wantErr: false,
		},
		{
			name: "active executions exist",
			id:   workflowID,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM workflow_executions").
					WithArgs(workflowID).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
				mock.ExpectRollback()
			},
			wantErr: true,
		},
		{
			name: "workflow not found",
			id:   workflowID,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM workflow_executions").
					WithArgs(workflowID).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery("SELECT tenant_id, name FROM workflows").
					WithArgs(workflowID).
					WillReturnError(sql.ErrNoRows)
				mock.ExpectRollback()
			},
			wantErr:     true,
			expectedErr: interfaces.ErrNotFound,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock, _, _ := setupWorkflowRepository(t)
			defer repo.writeDB.Close()
			
			tt.setupMock(mock)
			
			err := repo.Delete(context.Background(), tt.id)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.Equal(t, tt.expectedErr, err)
				}
			} else {
				assert.NoError(t, err)
			}
			
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestWorkflowRepository_CreateExecution(t *testing.T) {
	workflowID := uuid.New()
	tenantID := uuid.New()
	
	tests := []struct {
		name        string
		execution   *models.WorkflowExecution
		setupMock   func(sqlmock.Sqlmock)
		wantErr     bool
		expectedErr error
	}{
		{
			name: "successful creation",
			execution: &models.WorkflowExecution{
				WorkflowID:  workflowID,
				TenantID:    tenantID,
				Status:      models.WorkflowStatusPending,
				InitiatedBy: "user1",
				Context:     models.JSONMap{"key": "value"},
				State:       models.JSONMap{"step": "init"},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("INSERT INTO workflow_executions").
					WithArgs(sqlmock.AnyArg(), workflowID, tenantID, models.WorkflowStatusPending,
						sqlmock.AnyArg(), sqlmock.AnyArg(), "user1", "",
						sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
			},
			wantErr: false,
		},
		{
			name: "duplicate execution",
			execution: &models.WorkflowExecution{
				ID:         uuid.New(),
				WorkflowID: workflowID,
				TenantID:   tenantID,
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("INSERT INTO workflow_executions").
					WillReturnError(sql.ErrNoRows)
			},
			wantErr:     true,
			expectedErr: interfaces.ErrDuplicate,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock, _, _ := setupWorkflowRepository(t)
			defer repo.writeDB.Close()
			
			tt.setupMock(mock)
			
			err := repo.CreateExecution(context.Background(), tt.execution)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.Equal(t, tt.expectedErr, err)
				}
			} else {
				assert.NoError(t, err)
				assert.NotEqual(t, uuid.Nil, tt.execution.ID)
				assert.NotZero(t, tt.execution.StartedAt)
			}
			
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestWorkflowRepository_GetWorkflowStats(t *testing.T) {
	workflowID := uuid.New()
	period := 24 * time.Hour
	
	tests := []struct {
		name       string
		workflowID uuid.UUID
		period     time.Duration
		setupMock  func(sqlmock.Sqlmock)
		want       *interfaces.WorkflowStats
		wantErr    bool
	}{
		{
			name:       "successful stats retrieval",
			workflowID: workflowID,
			period:     period,
			setupMock: func(mock sqlmock.Sqlmock) {
				// Basic counts query
				mock.ExpectQuery("SELECT.*COUNT\\(\\*\\) as total_runs").
					WithArgs(workflowID, sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"total_runs", "successful_runs", "failed_runs"}).
						AddRow(100, 85, 15))
				
				// Timing query
				mock.ExpectQuery("SELECT.*AVG.*PERCENTILE_CONT").
					WithArgs(workflowID, sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"avg_runtime", "p95_runtime"}).
						AddRow(45.5, 120.8))
				
				// Status breakdown query
				mock.ExpectQuery("SELECT status, COUNT\\(\\*\\) as count").
					WithArgs(workflowID, sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"status", "count"}).
						AddRow("completed", 85).
						AddRow("failed", 10).
						AddRow("timeout", 5))
			},
			want: &interfaces.WorkflowStats{
				TotalRuns:      100,
				SuccessfulRuns: 85,
				FailedRuns:     15,
				AverageRuntime: 45500 * time.Millisecond,
				P95Runtime:     120800 * time.Millisecond,
				ByStatus: map[string]int64{
					"completed": 85,
					"failed":    10,
					"timeout":   5,
				},
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock, _, _ := setupWorkflowRepository(t)
			defer repo.writeDB.Close()
			
			tt.setupMock(mock)
			
			got, err := repo.GetWorkflowStats(context.Background(), tt.workflowID, tt.period)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want.TotalRuns, got.TotalRuns)
				assert.Equal(t, tt.want.SuccessfulRuns, got.SuccessfulRuns)
				assert.Equal(t, tt.want.FailedRuns, got.FailedRuns)
				assert.Equal(t, len(tt.want.ByStatus), len(got.ByStatus))
			}
			
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestWorkflowRepository_ConcurrentOperations(t *testing.T) {
	repo, mock, cache, _ := setupWorkflowRepository(t)
	defer repo.writeDB.Close()
	
	workflowID := uuid.New()
	tenantID := uuid.New()
	now := time.Now()
	
	// Pre-cache the workflow to test concurrent cache reads
	workflow := &models.Workflow{
		ID:          workflowID,
		TenantID:    tenantID,
		Name:        "Cached Workflow",
		Description: "Test",
		Type:        models.WorkflowTypeSequential,
		Version:     1,
		CreatedBy:   "user1",
		Agents:      models.JSONMap{},
		Steps:       models.JSONMap{},
		Config:      models.JSONMap{},
		Tags:        pq.StringArray{},
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	cache.data["workflow:"+workflowID.String()] = workflow
	
	// Run concurrent operations - they should all hit the cache
	errCh := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			result, err := repo.Get(context.Background(), workflowID)
			if err == nil && result.ID != workflowID {
				err = fmt.Errorf("unexpected workflow ID: %s", result.ID)
			}
			errCh <- err
		}()
	}
	
	// Check results
	for i := 0; i < 5; i++ {
		err := <-errCh
		assert.NoError(t, err)
	}
	
	// Verify all operations hit the cache
	assert.Equal(t, 5, cache.getCalls)
	
	// No database queries should have been made
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Benchmark tests
func BenchmarkWorkflowRepository_Get(b *testing.B) {
	repo, _, cache, _ := setupWorkflowRepository(&testing.T{})
	defer repo.writeDB.Close()
	
	workflowID := uuid.New()
	workflow := &models.Workflow{
		ID:       workflowID,
		Name:     "Benchmark Workflow",
		TenantID: uuid.New(),
	}
	cache.data["workflow:"+workflowID.String()] = workflow
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = repo.Get(ctx, workflowID)
	}
}

func BenchmarkWorkflowRepository_Create(b *testing.B) {
	db, mock, err := sqlmock.New()
	require.NoError(b, err)
	defer db.Close()
	
	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewWorkflowRepository(
		sqlxDB, sqlxDB,
		newMockCache(),
		observability.NewStandardLogger("bench"),
		observability.NoopStartSpan,
		newMockMetricsClient(),
	).(*workflowRepository)
	
	// Set up expectations for all iterations
	for i := 0; i < b.N; i++ {
		mock.ExpectQuery("INSERT INTO workflows").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		workflow := &models.Workflow{
			TenantID:  uuid.New(),
			Name:      "Benchmark Workflow",
			Type:      models.WorkflowTypeSequential,
			CreatedBy: "bench",
		}
		_ = repo.Create(ctx, workflow)
	}
}