# Phase 6: Testing and Monitoring

## Overview
This phase implements enterprise-grade testing strategies and production monitoring for the multi-agent collaboration system with comprehensive coverage including chaos engineering, security testing, contract testing, mutation testing, and full observability stack with SLO/SLI tracking.

## Timeline
**Duration**: 8-10 days
**Prerequisites**: Phases 1-5 completed
**Deliverables**:
- Comprehensive test suites (unit 90%+, integration, functional, E2E)
- Chaos engineering framework with failure injection
- Security testing suite (penetration, fuzzing, vulnerability scanning)
- Contract testing for API compatibility
- Mutation testing for test quality
- Property-based testing for edge cases
- Performance benchmarks and load testing
- Full observability stack (metrics, logs, traces, profiling)
- SLO/SLI definitions and monitoring
- Incident response runbooks
- CI/CD pipeline integration

## Testing Strategy

### 1. Unit Tests (85% Coverage Target)

#### Task Service Tests

```go
// File: pkg/services/task_service_test.go
package services_test

import (
    "context"
    "testing"
    "time"
    
    "github.com/golang/mock/gomock"
    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "github.com/S-Corkum/devops-mcp/pkg/models"
    "github.com/S-Corkum/devops-mcp/pkg/repository"
    "github.com/S-Corkum/devops-mcp/pkg/services"
    "github.com/S-Corkum/devops-mcp/test/mocks"
)

func TestTaskService_CreateDistributedTask(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    
    // Setup mocks
    mockRepo := mocks.NewMockTaskRepository(ctrl)
    mockAgentService := mocks.NewMockAgentService(ctrl)
    mockEventPublisher := mocks.NewMockEventPublisher(ctrl)
    mockNotifier := mocks.NewMockNotificationService(ctrl)
    
    service := services.NewTaskService(
        mockRepo,
        mockAgentService,
        mockEventPublisher,
        mockNotifier,
        testLogger,
        testMetrics,
    )
    
    ctx := context.WithValue(context.Background(), "tenant_id", testTenantID)
    ctx = context.WithValue(ctx, "agent_id", "coordinator-agent")
    
    t.Run("successful distributed task creation", func(t *testing.T) {
        // Test data
        dt := &models.DistributedTask{
            Type:        "parallel_analysis",
            Title:       "Analyze codebase",
            Description: "Parallel code analysis",
            Priority:    "high",
            Subtasks: []models.Subtask{
                {
                    ID:          "subtask-1",
                    AgentID:     "agent-1",
                    Description: "Analyze module A",
                    Parameters:  map[string]interface{}{"module": "A"},
                },
                {
                    ID:          "subtask-2",
                    AgentID:     "agent-2",
                    Description: "Analyze module B",
                    Parameters:  map[string]interface{}{"module": "B"},
                },
            },
            Aggregation: models.AggregationConfig{
                Method:     "combine_results",
                WaitForAll: true,
                Timeout:    3600,
            },
        }
        
        // Expectations
        // Main task creation
        mockRepo.EXPECT().
            Create(gomock.Any(), gomock.Any()).
            DoAndReturn(func(ctx context.Context, task *models.Task) error {
                assert.Equal(t, dt.Type, task.Type)
                assert.Equal(t, dt.Title, task.Title)
                assert.Equal(t, "coordinator-agent", task.CreatedBy)
                assert.Equal(t, true, task.Parameters["distributed"])
                assert.Equal(t, 2, task.Parameters["subtask_count"])
                task.ID = uuid.New()
                return nil
            })
        
        // Subtask creations
        mockRepo.EXPECT().
            Create(gomock.Any(), gomock.Any()).
            DoAndReturn(func(ctx context.Context, task *models.Task) error {
                assert.Contains(t, task.Title, "Analyze codebase - subtask")
                assert.NotNil(t, task.ParentTaskID)
                task.ID = uuid.New()
                return nil
            }).
            Times(2)
        
        // Update main task with subtask IDs
        mockRepo.EXPECT().
            Update(gomock.Any(), gomock.Any()).
            Return(nil)
        
        // Event publishing
        mockEventPublisher.EXPECT().
            Publish(gomock.Any(), gomock.Any()).
            Times(3) // Main task + 2 subtasks
        
        // Notifications
        mockNotifier.EXPECT().
            NotifyTaskAssigned(gomock.Any(), "agent-1", gomock.Any()).
            Return(nil)
        
        mockNotifier.EXPECT().
            NotifyTaskAssigned(gomock.Any(), "agent-2", gomock.Any()).
            Return(nil)
        
        // Execute
        err := service.CreateDistributedTask(ctx, dt)
        
        // Assert
        require.NoError(t, err)
        assert.NotEqual(t, uuid.Nil, dt.ID)
        assert.Len(t, dt.SubtaskIDs, 2)
    })
    
    t.Run("partial failure handling", func(t *testing.T) {
        dt := &models.DistributedTask{
            Type:     "test",
            Title:    "Test task",
            Subtasks: []models.Subtask{{ID: "sub1"}, {ID: "sub2"}},
        }
        
        // Main task succeeds
        mockRepo.EXPECT().
            Create(gomock.Any(), gomock.Any()).
            Return(nil)
        
        // First subtask succeeds
        mockRepo.EXPECT().
            Create(gomock.Any(), gomock.Any()).
            Return(nil)
        
        // Second subtask fails
        mockRepo.EXPECT().
            Create(gomock.Any(), gomock.Any()).
            Return(errors.New("database error"))
        
        // Update should still be called
        mockRepo.EXPECT().
            Update(gomock.Any(), gomock.Any()).
            Return(nil)
        
        mockEventPublisher.EXPECT().Publish(gomock.Any(), gomock.Any()).AnyTimes()
        mockNotifier.EXPECT().NotifyTaskAssigned(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
        
        // Execute
        err := service.CreateDistributedTask(ctx, dt)
        
        // Should succeed but only have 1 subtask
        require.NoError(t, err)
        assert.Len(t, dt.SubtaskIDs, 1)
    })
}

func TestTaskService_DelegateTask(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    
    mockRepo := mocks.NewMockTaskRepository(ctrl)
    mockNotifier := mocks.NewMockNotificationService(ctrl)
    
    service := services.NewTaskService(
        mockRepo,
        nil,
        nil,
        mockNotifier,
        testLogger,
        testMetrics,
    )
    
    ctx := context.Background()
    
    t.Run("successful delegation", func(t *testing.T) {
        taskID := uuid.New()
        task := &models.Task{
            ID:         taskID,
            AssignedTo: "agent-1",
            Status:     models.TaskStatusInProgress,
        }
        
        mockRepo.EXPECT().Get(ctx, taskID).Return(task, nil)
        
        mockRepo.EXPECT().
            CreateDelegation(ctx, gomock.Any()).
            DoAndReturn(func(ctx context.Context, d *models.TaskDelegation) error {
                assert.Equal(t, taskID, d.TaskID)
                assert.Equal(t, "agent-1", d.FromAgentID)
                assert.Equal(t, "agent-2", d.ToAgentID)
                assert.Equal(t, "workload", d.Reason)
                return nil
            })
        
        mockRepo.EXPECT().Update(ctx, task).Return(nil)
        
        mockNotifier.EXPECT().
            NotifyTaskDelegated(ctx, "agent-1", "agent-2", task, "workload").
            Return(nil)
        
        err := service.DelegateTask(ctx, taskID, "agent-1", "agent-2", "workload")
        
        require.NoError(t, err)
        assert.Equal(t, "agent-2", task.AssignedTo)
        assert.Equal(t, models.TaskStatusAssigned, task.Status)
    })
    
    t.Run("unauthorized delegation", func(t *testing.T) {
        taskID := uuid.New()
        task := &models.Task{
            ID:         taskID,
            AssignedTo: "agent-1",
            CreatedBy:  "agent-3",
        }
        
        mockRepo.EXPECT().Get(ctx, taskID).Return(task, nil)
        
        // Trying to delegate a task not assigned to or created by the agent
        err := service.DelegateTask(ctx, taskID, "agent-2", "agent-4", "reason")
        
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "not authorized")
    })
}
```

#### Workflow Service Tests

```go
// File: pkg/services/workflow_service_test.go
package services_test

func TestWorkflowExecutor_ExecuteSequential(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    
    mockTaskService := mocks.NewMockTaskService(ctrl)
    mockNotifier := mocks.NewMockNotificationService(ctrl)
    
    executor := services.NewWorkflowExecutor(
        testLogger,
        testMetrics,
        mockTaskService,
        mockNotifier,
    )
    
    // Test workflow
    workflow := &models.Workflow{
        ID:   uuid.New(),
        Type: models.WorkflowTypeSequential,
        Name: "Test Sequential Workflow",
    }
    
    steps := []models.WorkflowStep{
        {
            ID:     "step1",
            Agent:  "agent-1",
            Action: "analyze",
            Input:  map[string]interface{}{"target": "module_a"},
        },
        {
            ID:        "step2",
            Agent:     "agent-2",
            Action:    "optimize",
            Input:     "$step1.output",
            DependsOn: []string{"step1"},
        },
    }
    workflow.SetSteps(steps)
    
    execution := &models.WorkflowExecution{
        ID:          uuid.New(),
        WorkflowID:  workflow.ID,
        Status:      models.WorkflowStatusPending,
        Input:       map[string]interface{}{"config": "test"},
        Context:     make(map[string]interface{}),
        StepResults: make(map[string]interface{}),
    }
    
    // Expectations for step 1
    task1ID := uuid.New()
    mockTaskService.EXPECT().
        Create(gomock.Any(), gomock.Any()).
        DoAndReturn(func(ctx context.Context, task *models.Task) error {
            assert.Contains(t, task.Title, "step1")
            assert.Equal(t, "agent-1", task.AssignedTo)
            task.ID = task1ID
            return nil
        })
    
    // Simulate task completion for step 1
    go func() {
        time.Sleep(100 * time.Millisecond)
        // Simulate task completion callback
        executor.HandleTaskCompletion(task1ID, map[string]interface{}{
            "analysis": "complete",
            "issues":   3,
        })
    }()
    
    // Expectations for step 2
    task2ID := uuid.New()
    mockTaskService.EXPECT().
        Create(gomock.Any(), gomock.Any()).
        DoAndReturn(func(ctx context.Context, task *models.Task) error {
            assert.Contains(t, task.Title, "step2")
            assert.Equal(t, "agent-2", task.AssignedTo)
            // Verify input transformation
            params := task.Parameters.(map[string]interface{})
            assert.Equal(t, map[string]interface{}{
                "analysis": "complete",
                "issues":   3,
            }, params["step_input"])
            task.ID = task2ID
            return nil
        })
    
    // Simulate task completion for step 2
    go func() {
        time.Sleep(200 * time.Millisecond)
        executor.HandleTaskCompletion(task2ID, map[string]interface{}{
            "optimized": true,
            "improvement": "15%",
        })
    }()
    
    // Notifications
    mockNotifier.EXPECT().
        NotifyWorkflowStepStarted(gomock.Any(), workflow.ID, execution.ID, "step1").
        Return(nil)
    
    mockNotifier.EXPECT().
        NotifyWorkflowStepCompleted(gomock.Any(), workflow.ID, execution.ID, "step1", gomock.Any()).
        Return(nil)
    
    mockNotifier.EXPECT().
        NotifyWorkflowStepStarted(gomock.Any(), workflow.ID, execution.ID, "step2").
        Return(nil)
    
    mockNotifier.EXPECT().
        NotifyWorkflowStepCompleted(gomock.Any(), workflow.ID, execution.ID, "step2", gomock.Any()).
        Return(nil)
    
    mockNotifier.EXPECT().
        NotifyWorkflowCompleted(gomock.Any(), workflow, execution).
        Return(nil)
    
    // Execute workflow
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    executor.Execute(ctx, workflow, execution)
    
    // Verify execution completed
    assert.Equal(t, models.WorkflowStatusCompleted, execution.Status)
    assert.NotNil(t, execution.CompletedAt)
    assert.Len(t, execution.StepResults, 2)
    assert.Equal(t, map[string]interface{}{
        "analysis": "complete",
        "issues":   3,
    }, execution.StepResults["step1"])
    assert.Equal(t, map[string]interface{}{
        "optimized":   true,
        "improvement": "15%",
    }, execution.StepResults["step2"])
}
```

### 2. Integration Tests

#### Multi-Agent Integration Test

```go
// File: test/integration/multi_agent_integration_test.go
package integration_test

import (
    "context"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "github.com/S-Corkum/devops-mcp/test/testutil"
)

func TestMultiAgentTaskDelegation(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // Setup test environment
    env := testutil.SetupIntegrationTest(t)
    defer env.Cleanup()
    
    // Create test agents
    agent1 := env.CreateAgent("agent-1", []string{"code_review"})
    agent2 := env.CreateAgent("agent-2", []string{"code_review", "optimization"})
    agent3 := env.CreateAgent("agent-3", []string{"testing"})
    
    // Connect agents via WebSocket
    conn1 := env.ConnectAgent(agent1)
    conn2 := env.ConnectAgent(agent2)
    conn3 := env.ConnectAgent(agent3)
    
    defer conn1.Close()
    defer conn2.Close()
    defer conn3.Close()
    
    // Agent 2 subscribes to task assignments
    env.Subscribe(conn2, "task.assignments", map[string]interface{}{
        "agent_id": agent2.ID,
    })
    
    t.Run("task delegation flow", func(t *testing.T) {
        // Create task assigned to agent1
        task := env.CreateTask(models.Task{
            Type:        "code_review",
            Title:       "Review critical PR",
            Priority:    "high",
            AssignedTo:  agent1.ID,
        })
        
        // Agent 1 delegates to agent 2
        result := env.SendRequest(conn1, "task.delegate", map[string]interface{}{
            "task_id":     task.ID,
            "to_agent_id": agent2.ID,
            "reason":      "Agent 2 has optimization expertise needed",
        })
        
        assert.Equal(t, "delegated", result["status"])
        
        // Agent 2 should receive notification
        notification := env.WaitForNotification(conn2, "task.assigned", 2*time.Second)
        require.NotNil(t, notification)
        
        assert.Equal(t, task.ID, notification.Params["task_id"])
        assert.Equal(t, agent1.ID, notification.Params["from_agent"])
        
        // Agent 2 accepts the task
        result = env.SendRequest(conn2, "task.accept", map[string]interface{}{
            "task_id": task.ID,
        })
        
        assert.Equal(t, "accepted", result["status"])
        
        // Verify task is now assigned to agent 2
        updatedTask := env.GetTask(task.ID)
        assert.Equal(t, agent2.ID, updatedTask.AssignedTo)
        assert.Equal(t, models.TaskStatusInProgress, updatedTask.Status)
        
        // Verify delegation history
        delegations := env.GetTaskDelegations(task.ID)
        require.Len(t, delegations, 1)
        assert.Equal(t, agent1.ID, delegations[0].FromAgentID)
        assert.Equal(t, agent2.ID, delegations[0].ToAgentID)
    })
}
```

### 3. Functional Tests

#### Complete Multi-Agent Workflow Test

```go
// File: test/functional/api/websocket_collaboration_e2e_test.go
package api_test

var _ = Describe("End-to-End Multi-Agent Collaboration", func() {
    var (
        coordinator *websocket.Conn
        worker1     *websocket.Conn
        worker2     *websocket.Conn
        monitor     *websocket.Conn
    )
    
    BeforeEach(func() {
        // Setup connections
        coordinator = shared.EstablishConnection(wsURL, shared.GetTestAPIKey("coordinator"))
        worker1 = shared.EstablishConnection(wsURL, shared.GetTestAPIKey("worker-1"))
        worker2 = shared.EstablishConnection(wsURL, shared.GetTestAPIKey("worker-2"))
        monitor = shared.EstablishConnection(wsURL, shared.GetTestAPIKey("monitor"))
    })
    
    AfterEach(func() {
        coordinator.Close(websocket.StatusNormalClosure, "")
        worker1.Close(websocket.StatusNormalClosure, "")
        worker2.Close(websocket.StatusNormalClosure, "")
        monitor.Close(websocket.StatusNormalClosure, "")
    })
    
    It("should execute a complete collaborative workflow", func() {
        // Step 1: Register all agents
        agents := []struct {
            conn         *websocket.Conn
            name         string
            capabilities []string
        }{
            {coordinator, "coordinator", []string{"orchestration"}},
            {worker1, "worker-1", []string{"analysis", "code_review"}},
            {worker2, "worker-2", []string{"optimization", "testing"}},
            {monitor, "monitor", []string{"monitoring", "reporting"}},
        }
        
        for _, agent := range agents {
            resp := shared.RegisterAgent(agent.conn, agent.name, agent.capabilities)
            Expect(resp.Error).To(BeNil())
        }
        
        // Step 2: Create a shared workspace
        workspaceResp := shared.SendMessage(coordinator, ws.Message{
            Type:   ws.MessageTypeRequest,
            Method: "workspace.create",
            Params: map[string]interface{}{
                "name": "code-optimization-project",
                "type": "project",
            },
        })
        
        workspaceID := workspaceResp.Result.(map[string]interface{})["workspace_id"].(string)
        
        // All agents join the workspace
        for _, agent := range agents {
            resp := shared.SendMessage(agent.conn, ws.Message{
                Type:   ws.MessageTypeRequest,
                Method: "workspace.join",
                Params: map[string]interface{}{
                    "workspace_id": workspaceID,
                    "agent_name":   agent.name,
                },
            })
            Expect(resp.Error).To(BeNil())
        }
        
        // Step 3: Create a collaborative workflow
        workflowResp := shared.SendMessage(coordinator, ws.Message{
            Type:   ws.MessageTypeRequest,
            Method: "workflow.create_collaborative",
            Params: map[string]interface{}{
                "name": "code-optimization-pipeline",
                "type": "collaborative",
                "agents": map[string]interface{}{
                    "analyzer":  "worker-1",
                    "optimizer": "worker-2",
                    "monitor":   "monitor",
                },
                "steps": []map[string]interface{}{
                    {
                        "id":     "analyze",
                        "agent":  "analyzer",
                        "action": "analyze_code",
                        "input":  map[string]interface{}{"repository": "test-repo"},
                    },
                    {
                        "id":         "optimize",
                        "agent":      "optimizer",
                        "action":     "optimize_code",
                        "depends_on": []string{"analyze"},
                        "input":      "$analyze.output",
                    },
                    {
                        "id":         "report",
                        "agent":      "monitor",
                        "action":     "generate_report",
                        "depends_on": []string{"optimize"},
                        "input":      map[string]interface{}{
                            "analysis":     "$analyze.output",
                            "optimization": "$optimize.output",
                        },
                    },
                },
            },
        })
        
        workflowID := workflowResp.Result.(map[string]interface{})["workflow_id"].(string)
        
        // Step 4: Execute the workflow
        execResp := shared.SendMessage(coordinator, ws.Message{
            Type:   ws.MessageTypeRequest,
            Method: "workflow.execute_collaborative",
            Params: map[string]interface{}{
                "workflow_id": workflowID,
                "input": map[string]interface{}{
                    "target": "src/main.go",
                },
            },
        })
        
        executionID := execResp.Result.(map[string]interface{})["execution_id"].(string)
        
        // Step 5: Workers process their tasks
        // Worker 1 receives and completes analysis task
        Eventually(func() bool {
            msg := shared.ReadMessage(worker1)
            if msg.Type == ws.MessageTypeNotification && msg.Method == "workflow.task_ready" {
                // Complete the analysis
                shared.SendMessage(worker1, ws.Message{
                    Type:   ws.MessageTypeRequest,
                    Method: "workflow.complete_task",
                    Params: map[string]interface{}{
                        "workflow_id":  workflowID,
                        "execution_id": executionID,
                        "step_id":      "analyze",
                        "output": map[string]interface{}{
                            "issues_found": 5,
                            "complexity":   "medium",
                            "suggestions":  []string{"refactor method A", "add tests"},
                        },
                    },
                })
                return true
            }
            return false
        }, 5*time.Second).Should(BeTrue())
        
        // Worker 2 receives and completes optimization task
        Eventually(func() bool {
            msg := shared.ReadMessage(worker2)
            if msg.Type == ws.MessageTypeNotification && msg.Method == "workflow.task_ready" {
                // Complete the optimization
                shared.SendMessage(worker2, ws.Message{
                    Type:   ws.MessageTypeRequest,
                    Method: "workflow.complete_task",
                    Params: map[string]interface{}{
                        "workflow_id":  workflowID,
                        "execution_id": executionID,
                        "step_id":      "optimize",
                        "output": map[string]interface{}{
                            "optimized":     true,
                            "performance":   "+25%",
                            "code_quality":  "improved",
                        },
                    },
                })
                return true
            }
            return false
        }, 5*time.Second).Should(BeTrue())
        
        // Monitor receives and completes reporting task
        Eventually(func() bool {
            msg := shared.ReadMessage(monitor)
            if msg.Type == ws.MessageTypeNotification && msg.Method == "workflow.task_ready" {
                // Generate report
                shared.SendMessage(monitor, ws.Message{
                    Type:   ws.MessageTypeRequest,
                    Method: "workflow.complete_task",
                    Params: map[string]interface{}{
                        "workflow_id":  workflowID,
                        "execution_id": executionID,
                        "step_id":      "report",
                        "output": map[string]interface{}{
                            "report_url":    "https://reports.example.com/123",
                            "summary":       "Code optimized successfully",
                            "total_time":    "45s",
                        },
                    },
                })
                return true
            }
            return false
        }, 5*time.Second).Should(BeTrue())
        
        // Step 6: Verify workflow completion
        Eventually(func() string {
            statusResp := shared.SendMessage(coordinator, ws.Message{
                Type:   ws.MessageTypeRequest,
                Method: "workflow.get_status",
                Params: map[string]interface{}{
                    "execution_id": executionID,
                },
            })
            
            if result, ok := statusResp.Result.(map[string]interface{}); ok {
                return result["status"].(string)
            }
            return ""
        }, 10*time.Second).Should(Equal("completed"))
        
        // Step 7: Update workspace state with results
        shared.SendMessage(coordinator, ws.Message{
            Type:   ws.MessageTypeRequest,
            Method: "workspace.update_state",
            Params: map[string]interface{}{
                "workspace_id": workspaceID,
                "updates": map[string]interface{}{
                    "last_optimization": time.Now().Format(time.RFC3339),
                    "performance_gain":  "+25%",
                    "workflow_status":   "completed",
                },
            },
        })
        
        // Verify all agents see the updated state
        for _, agent := range agents {
            stateResp := shared.SendMessage(agent.conn, ws.Message{
                Type:   ws.MessageTypeRequest,
                Method: "workspace.get_state",
                Params: map[string]interface{}{
                    "workspace_id": workspaceID,
                },
            })
            
            state := stateResp.Result.(map[string]interface{})["state"].(map[string]interface{})
            Expect(state["performance_gain"]).To(Equal("+25%"))
        }
    })
})
```

### 4. Load Tests

#### WebSocket Load Test

```go
// File: test/load/websocket_load_test.go
package load_test

import (
    "context"
    "sync"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "github.com/S-Corkum/devops-mcp/test/testutil"
)

func TestWebSocketLoadHandling(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping load test")
    }
    
    env := testutil.SetupLoadTest(t)
    defer env.Cleanup()
    
    const (
        numAgents        = 100
        numTasks         = 1000
        numWorkspaces    = 10
        testDuration     = 5 * time.Minute
    )
    
    t.Run("concurrent agent operations", func(t *testing.T) {
        ctx, cancel := context.WithTimeout(context.Background(), testDuration)
        defer cancel()
        
        // Metrics collection
        metrics := &LoadTestMetrics{
            ConnectionsEstablished: 0,
            TasksCreated:          0,
            TasksCompleted:        0,
            Errors:                0,
            Latencies:             make([]time.Duration, 0),
        }
        
        var wg sync.WaitGroup
        
        // Create agent workers
        for i := 0; i < numAgents; i++ {
            wg.Add(1)
            go func(agentID int) {
                defer wg.Done()
                
                // Connect agent
                conn, err := env.ConnectAgent(fmt.Sprintf("agent-%d", agentID))
                if err != nil {
                    metrics.RecordError()
                    return
                }
                defer conn.Close()
                
                metrics.RecordConnection()
                
                // Agent work loop
                for {
                    select {
                    case <-ctx.Done():
                        return
                    default:
                        // Create and complete tasks
                        start := time.Now()
                        
                        taskID := env.CreateTask(models.Task{
                            Type:     "load_test",
                            Title:    fmt.Sprintf("Load test task %d", metrics.TasksCreated),
                            Priority: "normal",
                        })
                        
                        if taskID != uuid.Nil {
                            metrics.RecordTaskCreated()
                            
                            // Simulate work
                            time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
                            
                            // Complete task
                            if env.CompleteTask(conn, taskID) {
                                metrics.RecordTaskCompleted()
                            }
                        }
                        
                        metrics.RecordLatency(time.Since(start))
                        
                        // Random delay between operations
                        time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
                    }
                }
            }(i)
        }
        
        // Monitor metrics
        go func() {
            ticker := time.NewTicker(10 * time.Second)
            defer ticker.Stop()
            
            for {
                select {
                case <-ctx.Done():
                    return
                case <-ticker.C:
                    metrics.PrintSummary()
                }
            }
        }()
        
        // Wait for test completion
        wg.Wait()
        
        // Final metrics
        metrics.PrintFinalReport()
        
        // Assertions
        assert.Greater(t, metrics.ConnectionsEstablished, numAgents*0.95) // 95% success rate
        assert.Greater(t, metrics.TasksCompleted, metrics.TasksCreated*0.90) // 90% completion rate
        assert.Less(t, metrics.ErrorRate(), 0.05) // Less than 5% errors
        assert.Less(t, metrics.P99Latency(), 500*time.Millisecond) // P99 under 500ms
    })
}

type LoadTestMetrics struct {
    ConnectionsEstablished int64
    TasksCreated          int64
    TasksCompleted        int64
    Errors                int64
    Latencies             []time.Duration
    mu                    sync.Mutex
}

func (m *LoadTestMetrics) RecordConnection() {
    atomic.AddInt64(&m.ConnectionsEstablished, 1)
}

func (m *LoadTestMetrics) RecordTaskCreated() {
    atomic.AddInt64(&m.TasksCreated, 1)
}

func (m *LoadTestMetrics) RecordTaskCompleted() {
    atomic.AddInt64(&m.TasksCompleted, 1)
}

func (m *LoadTestMetrics) RecordError() {
    atomic.AddInt64(&m.Errors, 1)
}

func (m *LoadTestMetrics) RecordLatency(d time.Duration) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.Latencies = append(m.Latencies, d)
}

func (m *LoadTestMetrics) P99Latency() time.Duration {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    if len(m.Latencies) == 0 {
        return 0
    }
    
    sort.Slice(m.Latencies, func(i, j int) bool {
        return m.Latencies[i] < m.Latencies[j]
    })
    
    index := int(float64(len(m.Latencies)) * 0.99)
    return m.Latencies[index]
}
```

### 5. Chaos Testing

#### Agent Failure Simulation

```go
// File: test/chaos/agent_failure_test.go
package chaos_test

import (
    "context"
    "math/rand"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    
    "github.com/S-Corkum/devops-mcp/test/chaos"
)

func TestAgentFailureRecovery(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping chaos test")
    }
    
    env := chaos.SetupChaosEnvironment(t)
    defer env.Cleanup()
    
    // Create a distributed task with 5 subtasks
    task := env.CreateDistributedTask(5)
    
    // Create 5 agents to handle subtasks
    agents := env.CreateAgents(5)
    
    // Chaos monkey configuration
    chaosConfig := chaos.Config{
        FailureRate:     0.3, // 30% chance of failure
        RecoveryTime:    5 * time.Second,
        NetworkLatency:  100 * time.Millisecond,
        PacketLoss:      0.1, // 10% packet loss
    }
    
    // Start chaos monkey
    monkey := chaos.NewChaosMonkey(env, chaosConfig)
    monkey.Start()
    defer monkey.Stop()
    
    // Monitor task completion
    completed := make(chan bool)
    
    go func() {
        for {
            status := env.GetTaskStatus(task.ID)
            if status.Status == "completed" || status.Status == "failed" {
                completed <- status.Status == "completed"
                return
            }
            time.Sleep(1 * time.Second)
        }
    }()
    
    // Wait for completion with timeout
    select {
    case success := <-completed:
        assert.True(t, success, "Task should complete despite failures")
        
        // Verify all subtasks completed
        for _, subtaskID := range task.SubtaskIDs {
            subtask := env.GetTask(subtaskID)
            assert.Equal(t, "completed", subtask.Status)
        }
        
        // Check delegation history for failovers
        delegations := env.GetAllDelegations()
        failovers := 0
        for _, d := range delegations {
            if d.DelegationType == "failover" {
                failovers++
            }
        }
        
        t.Logf("Task completed with %d failovers", failovers)
        assert.Greater(t, failovers, 0, "Should have some failovers due to chaos")
        
    case <-time.After(5 * time.Minute):
        t.Fatal("Task did not complete within timeout")
    }
}

// ChaosMonkey simulates various failure scenarios
type ChaosMonkey struct {
    env      *chaos.Environment
    config   chaos.Config
    stop     chan bool
    failures []FailureEvent
}

func (cm *ChaosMonkey) Start() {
    cm.stop = make(chan bool)
    
    go func() {
        ticker := time.NewTicker(1 * time.Second)
        defer ticker.Stop()
        
        for {
            select {
            case <-cm.stop:
                return
            case <-ticker.C:
                cm.maybeInjectFailure()
            }
        }
    }()
}

func (cm *ChaosMonkey) maybeInjectFailure() {
    if rand.Float64() < cm.config.FailureRate {
        agents := cm.env.GetActiveAgents()
        if len(agents) == 0 {
            return
        }
        
        // Select random agent to fail
        victim := agents[rand.Intn(len(agents))]
        
        failureType := rand.Intn(4)
        switch failureType {
        case 0:
            cm.disconnectAgent(victim)
        case 1:
            cm.slowdownAgent(victim)
        case 2:
            cm.corruptMessages(victim)
        case 3:
            cm.exhaustResources(victim)
        }
        
        // Schedule recovery
        go func() {
            time.Sleep(cm.config.RecoveryTime)
            cm.recoverAgent(victim)
        }()
    }
}
```

### 6. Security Testing

#### Penetration Testing

```go
// File: test/security/penetration_test.go
package security_test

import (
    "context"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "github.com/S-Corkum/devops-mcp/test/security"
)

func TestAuthenticationBypass(t *testing.T) {
    env := security.SetupSecurityTest(t)
    defer env.Cleanup()
    
    t.Run("invalid JWT token", func(t *testing.T) {
        // Attempt to connect with forged JWT
        forgedToken := security.ForgeJWT(map[string]interface{}{
            "agent_id": "malicious-agent",
            "tenant_id": "victim-tenant",
            "exp": time.Now().Add(1 * time.Hour).Unix(),
        })
        
        _, err := env.ConnectWithToken(forgedToken)
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "invalid signature")
    })
    
    t.Run("expired token reuse", func(t *testing.T) {
        // Create legitimate connection
        conn, token := env.CreateAuthenticatedConnection("test-agent")
        conn.Close()
        
        // Wait for token to expire
        time.Sleep(env.TokenTTL + 1*time.Second)
        
        // Attempt to reuse expired token
        _, err := env.ConnectWithToken(token)
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "token expired")
    })
    
    t.Run("privilege escalation", func(t *testing.T) {
        // Connect as regular agent
        conn := env.ConnectAsAgent("regular-agent", []string{"read"})
        defer conn.Close()
        
        // Attempt admin operation
        err := env.ExecuteAdminOperation(conn, "workflow.delete_all")
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "insufficient privileges")
    })
    
    t.Run("tenant isolation breach", func(t *testing.T) {
        // Create agents in different tenants
        agent1 := env.CreateAgentInTenant("agent1", "tenant1")
        agent2 := env.CreateAgentInTenant("agent2", "tenant2")
        
        // Agent1 tries to access tenant2's data
        conn1 := env.ConnectAgent(agent1)
        defer conn1.Close()
        
        // Attempt cross-tenant access
        _, err := env.AccessResource(conn1, "task", agent2.Tasks[0].ID)
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "access denied")
    })
}

func TestInjectionAttacks(t *testing.T) {
    env := security.SetupSecurityTest(t)
    defer env.Cleanup()
    
    injectionPayloads := []struct {
        name    string
        payload string
        field   string
    }{
        {
            name:    "SQL injection in task title",
            payload: "'; DROP TABLE tasks; --",
            field:   "title",
        },
        {
            name:    "NoSQL injection in parameters",
            payload: `{"$ne": null}`,
            field:   "parameters",
        },
        {
            name:    "Command injection in agent name",
            payload: "agent; rm -rf /; echo",
            field:   "agent_name",
        },
        {
            name:    "LDAP injection in search",
            payload: "*)(uid=*))(|(uid=*",
            field:   "search_query",
        },
        {
            name:    "XSS in description",
            payload: "<script>alert('xss')</script>",
            field:   "description",
        },
        {
            name:    "XXE in XML import",
            payload: `<!DOCTYPE foo [<!ENTITY xxe SYSTEM "file:///etc/passwd">]><task>&xxe;</task>`,
            field:   "import_data",
        },
    }
    
    for _, tc := range injectionPayloads {
        t.Run(tc.name, func(t *testing.T) {
            conn := env.ConnectAgent(env.CreateAgent("test-agent"))
            defer conn.Close()
            
            // Attempt injection
            err := env.SendMaliciousPayload(conn, tc.field, tc.payload)
            
            // Verify payload was sanitized/rejected
            if err == nil {
                // Check if payload was sanitized
                result := env.GetLastCreatedResource(conn)
                assert.NotContains(t, result[tc.field], tc.payload)
            } else {
                // Or rejected entirely
                assert.Contains(t, err.Error(), "invalid input")
            }
            
            // Verify system integrity
            assert.True(t, env.VerifySystemIntegrity())
        })
    }
}

func TestDenialOfService(t *testing.T) {
    env := security.SetupSecurityTest(t)
    defer env.Cleanup()
    
    t.Run("connection flooding", func(t *testing.T) {
        // Attempt to open excessive connections
        connections := make([]*websocket.Conn, 0)
        defer func() {
            for _, conn := range connections {
                conn.Close()
            }
        }()
        
        // Try to open 10000 connections
        for i := 0; i < 10000; i++ {
            conn, err := env.CreateConnection()
            if err != nil {
                // Should hit rate limit
                assert.Contains(t, err.Error(), "rate limit exceeded")
                break
            }
            connections = append(connections, conn)
        }
        
        // Verify rate limiting kicked in
        assert.Less(t, len(connections), 1000) // Should be limited
    })
    
    t.Run("memory exhaustion", func(t *testing.T) {
        conn := env.ConnectAgent(env.CreateAgent("dos-agent"))
        defer conn.Close()
        
        // Try to create massive task
        hugeData := make([]byte, 100*1024*1024) // 100MB
        err := env.CreateTaskWithData(conn, hugeData)
        
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "payload too large")
    })
    
    t.Run("CPU exhaustion via regex", func(t *testing.T) {
        conn := env.ConnectAgent(env.CreateAgent("regex-agent"))
        defer conn.Close()
        
        // Evil regex pattern
        evilPattern := "(a+)+b"
        evilInput := strings.Repeat("a", 100)
        
        start := time.Now()
        err := env.SearchWithPattern(conn, evilPattern, evilInput)
        duration := time.Since(start)
        
        // Should timeout or reject
        assert.True(t, err != nil || duration < 5*time.Second)
    })
}
```

#### Fuzzing Tests

```go
// File: test/security/fuzz_test.go
package security_test

import (
    "testing"
    
    "github.com/dvyukov/go-fuzz"
)

func FuzzTaskCreation(data []byte) int {
    env := security.GetFuzzEnvironment()
    
    // Try to create task with fuzzed data
    err := env.CreateTaskFromBytes(data)
    
    if err != nil {
        // Expected errors are ok
        if security.IsExpectedError(err) {
            return 0
        }
        // Unexpected error or panic
        panic(err)
    }
    
    // Verify task was created correctly
    if !env.VerifyLastTask() {
        panic("task validation failed")
    }
    
    return 1
}

func FuzzWebSocketProtocol(data []byte) int {
    env := security.GetFuzzEnvironment()
    conn := env.GetFuzzConnection()
    
    // Send fuzzed WebSocket frame
    err := conn.WriteMessage(websocket.BinaryMessage, data)
    if err != nil {
        return 0
    }
    
    // Read response
    _, response, err := conn.ReadMessage()
    if err != nil {
        // Connection should not be dropped
        panic("connection dropped on fuzzed input")
    }
    
    // Verify error response
    if !security.IsErrorResponse(response) {
        panic("non-error response to fuzzed input")
    }
    
    return 1
}
```

### 7. Contract Testing

```go
// File: test/contract/api_contract_test.go
package contract_test

import (
    "testing"
    
    "github.com/pact-foundation/pact-go/v2/consumer"
    "github.com/pact-foundation/pact-go/v2/matchers"
    "github.com/stretchr/testify/assert"
)

func TestTaskAPIContract(t *testing.T) {
    // Create Pact consumer
    pact, err := consumer.NewV3Pact(consumer.MockHTTPProviderConfig{
        Consumer: "WebSocketClient",
        Provider: "MCPServer",
        Host:     "127.0.0.1",
        Port:     8080,
    })
    require.NoError(t, err)
    
    t.Run("create task contract", func(t *testing.T) {
        pact.
            AddInteraction().
            Given("authenticated agent exists").
            UponReceiving("a request to create a task").
            WithRequest("POST", "/api/v1/tasks",
                matchers.MapMatcher{
                    "type":     matchers.String("analysis"),
                    "title":    matchers.String("Analyze codebase"),
                    "priority": matchers.Regex("low|normal|high|critical", "normal"),
                }).
            WillRespondWith(201,
                matchers.MapMatcher{
                    "id":         matchers.UUID(),
                    "type":       matchers.String("analysis"),
                    "title":      matchers.String("Analyze codebase"),
                    "status":     matchers.String("pending"),
                    "created_at": matchers.Timestamp(),
                })
        
        err := pact.Verify(t, func(tc consumer.MockServerConfig) error {
            client := NewAPIClient(tc.Host, tc.Port)
            
            task, err := client.CreateTask(TaskRequest{
                Type:     "analysis",
                Title:    "Analyze codebase",
                Priority: "normal",
            })
            
            assert.NoError(t, err)
            assert.NotEmpty(t, task.ID)
            assert.Equal(t, "pending", task.Status)
            
            return err
        })
        
        assert.NoError(t, err)
    })
    
    t.Run("websocket message contract", func(t *testing.T) {
        pact.
            AddInteraction().
            Given("WebSocket connection established").
            UponReceiving("task.create message").
            WithRequest("WS", "/ws",
                matchers.MapMatcher{
                    "id":     matchers.UUID(),
                    "type":   matchers.String("request"),
                    "method": matchers.String("task.create"),
                    "params": matchers.MapMatcher{
                        "type":  matchers.String("analysis"),
                        "title": matchers.String("Test task"),
                    },
                }).
            WillRespondWith(200,
                matchers.MapMatcher{
                    "id":     matchers.UUID(),
                    "type":   matchers.String("response"),
                    "result": matchers.MapMatcher{
                        "task_id": matchers.UUID(),
                        "status":  matchers.String("created"),
                    },
                })
        
        // Verify WebSocket contract
        err := pact.Verify(t, func(tc consumer.MockServerConfig) error {
            ws := ConnectWebSocket(tc.Host, tc.Port)
            defer ws.Close()
            
            response := ws.SendRequest("task.create", map[string]interface{}{
                "type":  "analysis",
                "title": "Test task",
            })
            
            assert.NotNil(t, response.Result)
            assert.NotEmpty(t, response.Result["task_id"])
            
            return nil
        })
        
        assert.NoError(t, err)
    })
}
```

### 8. Mutation Testing

```go
// File: test/mutation/mutation_test.go
package mutation_test

import (
    "testing"
    
    "github.com/zimmski/go-mutesting"
    "github.com/stretchr/testify/assert"
)

func TestMutationCoverage(t *testing.T) {
    // Configure mutation testing
    config := mutesting.Config{
        Pkg:      "github.com/S-Corkum/devops-mcp/pkg/services",
        TestPkg:  "github.com/S-Corkum/devops-mcp/pkg/services_test",
        Mutators: []mutesting.Mutator{
            &mutesting.ConditionalsBoundaryMutator{},
            &mutesting.MathMutator{},
            &mutesting.NegateConditionalsMutator{},
            &mutesting.RemoveStatementMutator{},
            &mutesting.ReturnValueMutator{},
        },
    }
    
    // Run mutation testing
    results, err := mutesting.Run(config)
    require.NoError(t, err)
    
    // Calculate mutation score
    killed := 0
    survived := 0
    
    for _, result := range results {
        if result.Killed {
            killed++
        } else {
            survived++
            t.Logf("Survived mutation: %s at %s:%d", 
                result.Mutation, result.File, result.Line)
        }
    }
    
    mutationScore := float64(killed) / float64(killed+survived) * 100
    t.Logf("Mutation Score: %.2f%% (%d killed, %d survived)", 
        mutationScore, killed, survived)
    
    // Require high mutation score
    assert.Greater(t, mutationScore, 80.0, 
        "Mutation score too low - tests may not be thorough enough")
}
```

### 9. Property-Based Testing

```go
// File: test/property/task_properties_test.go
package property_test

import (
    "testing"
    
    "github.com/leanovate/gopter"
    "github.com/leanovate/gopter/gen"
    "github.com/leanovate/gopter/prop"
)

func TestTaskDelegationProperties(t *testing.T) {
    properties := gopter.NewProperties(gopter.DefaultTestParameters())
    
    properties.Property("task delegation preserves task data", prop.ForAll(
        func(taskData TaskData, agents []string) bool {
            if len(agents) < 2 {
                return true // Skip if not enough agents
            }
            
            env := NewTestEnv()
            defer env.Cleanup()
            
            // Create task
            task := env.CreateTask(taskData)
            originalData := task.Clone()
            
            // Delegate through all agents
            currentAgent := agents[0]
            for _, nextAgent := range agents[1:] {
                err := env.DelegateTask(task.ID, currentAgent, nextAgent)
                if err != nil {
                    return false
                }
                currentAgent = nextAgent
            }
            
            // Verify task data unchanged
            finalTask := env.GetTask(task.ID)
            return taskDataEqual(originalData, finalTask)
        },
        genTaskData(),
        gen.SliceOf(gen.Identifier()),
    ))
    
    properties.Property("concurrent task creation is consistent", prop.ForAll(
        func(numAgents int, numTasks int) bool {
            if numAgents < 1 || numAgents > 100 {
                return true
            }
            if numTasks < 1 || numTasks > 1000 {
                return true
            }
            
            env := NewTestEnv()
            defer env.Cleanup()
            
            // Create agents concurrently creating tasks
            taskChan := make(chan uuid.UUID, numTasks*numAgents)
            errChan := make(chan error, numTasks*numAgents)
            
            var wg sync.WaitGroup
            for i := 0; i < numAgents; i++ {
                wg.Add(1)
                go func(agentID int) {
                    defer wg.Done()
                    
                    for j := 0; j < numTasks; j++ {
                        task, err := env.CreateTaskAs(
                            fmt.Sprintf("agent-%d", agentID),
                            TaskData{
                                Type:  "test",
                                Title: fmt.Sprintf("Task %d-%d", agentID, j),
                            },
                        )
                        
                        if err != nil {
                            errChan <- err
                        } else {
                            taskChan <- task.ID
                        }
                    }
                }(i)
            }
            
            wg.Wait()
            close(taskChan)
            close(errChan)
            
            // Verify all tasks created
            taskIDs := make(map[uuid.UUID]bool)
            for id := range taskChan {
                if taskIDs[id] {
                    return false // Duplicate ID
                }
                taskIDs[id] = true
            }
            
            // No errors allowed
            for err := range errChan {
                if err != nil {
                    return false
                }
            }
            
            return len(taskIDs) == numAgents*numTasks
        },
        gen.IntRange(1, 10),
        gen.IntRange(1, 100),
    ))
    
    properties.TestingRun(t)
}

func genTaskData() gopter.Gen {
    return gopter.CombineGens(
        gen.Identifier(),
        gen.Identifier(),
        gen.OneConstOf("low", "normal", "high", "critical"),
        gen.MapOf(gen.Identifier(), gen.AnyString()),
    ).Map(func(values []interface{}) TaskData {
        return TaskData{
            Type:       values[0].(string),
            Title:      values[1].(string),
            Priority:   values[2].(string),
            Parameters: values[3].(map[string]interface{}),
        }
    })
}
```

### 10. Performance Benchmarks

```go
// File: test/benchmark/collaboration_bench_test.go
package benchmark_test

import (
    "context"
    "sync"
    "testing"
    
    "github.com/S-Corkum/devops-mcp/pkg/collaboration/crdt"
)

func BenchmarkCRDTOperations(b *testing.B) {
    scenarios := []struct {
        name      string
        agents    int
        opsPerSec int
    }{
        {"2agents_10ops", 2, 10},
        {"5agents_50ops", 5, 50},
        {"10agents_100ops", 10, 100},
        {"50agents_500ops", 50, 500},
    }
    
    for _, scenario := range scenarios {
        b.Run(scenario.name, func(b *testing.B) {
            doc := crdt.NewRGA(uuid.New(), "bench-node", nil)
            
            b.ResetTimer()
            b.RunParallel(func(pb *testing.PB) {
                agentID := fmt.Sprintf("agent-%d", atomic.AddInt32(&agentCounter, 1))
                pos := 0
                
                for pb.Next() {
                    // Simulate mixed operations
                    switch pos % 3 {
                    case 0:
                        doc.Insert(pos, 'x')
                    case 1:
                        if pos > 0 {
                            doc.Delete(pos - 1)
                        }
                    case 2:
                        doc.GetContent()
                    }
                    pos++
                }
            })
            
            b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ops/sec")
        })
    }
}

func BenchmarkWorkflowExecution(b *testing.B) {
    env := setupBenchEnv(b)
    defer env.Cleanup()
    
    workflows := []struct {
        name  string
        steps int
        type  string
    }{
        {"sequential_5steps", 5, "sequential"},
        {"parallel_5steps", 5, "parallel"},
        {"sequential_20steps", 20, "sequential"},
        {"parallel_20steps", 20, "parallel"},
        {"complex_50steps", 50, "complex"},
    }
    
    for _, wf := range workflows {
        b.Run(wf.name, func(b *testing.B) {
            workflow := env.CreateWorkflow(wf.type, wf.steps)
            
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                execution := env.ExecuteWorkflow(workflow)
                env.WaitForCompletion(execution)
            }
            
            b.ReportMetric(float64(wf.steps), "steps")
            b.ReportMetric(float64(b.N*wf.steps)/b.Elapsed().Seconds(), "steps/sec")
        })
    }
}

func BenchmarkWebSocketThroughput(b *testing.B) {
    env := setupBenchEnv(b)
    defer env.Cleanup()
    
    messageSizes := []int{100, 1024, 10240, 102400} // 100B, 1KB, 10KB, 100KB
    
    for _, size := range messageSizes {
        b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
            conn := env.CreateConnection()
            defer conn.Close()
            
            message := generateMessage(size)
            
            b.SetBytes(int64(size))
            b.ResetTimer()
            
            for i := 0; i < b.N; i++ {
                err := conn.WriteMessage(websocket.BinaryMessage, message)
                if err != nil {
                    b.Fatal(err)
                }
                
                _, _, err = conn.ReadMessage()
                if err != nil {
                    b.Fatal(err)
                }
            }
            
            b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "msg/sec")
        })
    }
}
```

## Enhanced Monitoring Implementation

### 1. Metrics Collection

```go
// File: pkg/observability/collaboration_metrics.go
package observability

import (
    "context"
    "time"
    
    "github.com/prometheus/client_golang/prometheus"
)

// CollaborationMetrics tracks multi-agent collaboration metrics
type CollaborationMetrics struct {
    // Task metrics
    taskCreated         *prometheus.CounterVec
    taskCompleted       *prometheus.CounterVec
    taskDelegated       *prometheus.CounterVec
    taskFailed          *prometheus.CounterVec
    taskDuration        *prometheus.HistogramVec
    taskQueueDepth      *prometheus.GaugeVec
    
    // Workflow metrics
    workflowStarted     *prometheus.CounterVec
    workflowCompleted   *prometheus.CounterVec
    workflowFailed      *prometheus.CounterVec
    workflowDuration    *prometheus.HistogramVec
    workflowStepLatency *prometheus.HistogramVec
    
    // Agent metrics
    activeAgents        *prometheus.GaugeVec
    agentUtilization    *prometheus.GaugeVec
    agentPerformance    *prometheus.HistogramVec
    agentFailures       *prometheus.CounterVec
    
    // Collaboration metrics
    delegationSuccess   *prometheus.CounterVec
    conflictResolutions *prometheus.CounterVec
    stateSync           *prometheus.HistogramVec
    
    // WebSocket metrics
    wsConnections       prometheus.Gauge
    wsMessageRate       *prometheus.CounterVec
    wsMessageSize       *prometheus.HistogramVec
    wsErrors            *prometheus.CounterVec
}

// NewCollaborationMetrics creates new collaboration metrics
func NewCollaborationMetrics(reg prometheus.Registerer) *CollaborationMetrics {
    m := &CollaborationMetrics{
        taskCreated: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "devops_mcp_task_created_total",
                Help: "Total number of tasks created",
            },
            []string{"type", "priority", "tenant_id"},
        ),
        
        taskCompleted: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "devops_mcp_task_completed_total",
                Help: "Total number of tasks completed",
            },
            []string{"type", "priority", "tenant_id", "status"},
        ),
        
        taskDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "devops_mcp_task_duration_seconds",
                Help:    "Task execution duration in seconds",
                Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
            },
            []string{"type", "priority"},
        ),
        
        workflowDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "devops_mcp_workflow_duration_seconds",
                Help:    "Workflow execution duration in seconds",
                Buckets: prometheus.ExponentialBuckets(1, 2, 10),
            },
            []string{"type", "name"},
        ),
        
        activeAgents: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "devops_mcp_active_agents",
                Help: "Number of active agents",
            },
            []string{"capability", "status"},
        ),
        
        wsConnections: prometheus.NewGauge(
            prometheus.GaugeOpts{
                Name: "devops_mcp_websocket_connections",
                Help: "Current number of WebSocket connections",
            },
        ),
        
        conflictResolutions: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "devops_mcp_conflict_resolutions_total",
                Help: "Total number of conflict resolutions",
            },
            []string{"type", "strategy", "success"},
        ),
    }
    
    // Register all metrics
    reg.MustRegister(
        m.taskCreated,
        m.taskCompleted,
        m.taskDuration,
        m.workflowDuration,
        m.activeAgents,
        m.wsConnections,
        m.conflictResolutions,
    )
    
    return m
}

// RecordTaskCreated records task creation
func (m *CollaborationMetrics) RecordTaskCreated(taskType, priority, tenantID string) {
    m.taskCreated.WithLabelValues(taskType, priority, tenantID).Inc()
}

// RecordTaskCompleted records task completion
func (m *CollaborationMetrics) RecordTaskCompleted(taskType, priority, tenantID, status string, duration time.Duration) {
    m.taskCompleted.WithLabelValues(taskType, priority, tenantID, status).Inc()
    m.taskDuration.WithLabelValues(taskType, priority).Observe(duration.Seconds())
}

// RecordWorkflowExecution records workflow execution
func (m *CollaborationMetrics) RecordWorkflowExecution(workflowType, name string, duration time.Duration, success bool) {
    status := "success"
    if !success {
        status = "failure"
    }
    
    if success {
        m.workflowCompleted.WithLabelValues(workflowType, name).Inc()
    } else {
        m.workflowFailed.WithLabelValues(workflowType, name).Inc()
    }
    
    m.workflowDuration.WithLabelValues(workflowType, name).Observe(duration.Seconds())
}
```

### 2. Distributed Tracing

```go
// File: pkg/observability/tracing.go
package observability

import (
    "context"
    
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

// CollaborationTracer provides tracing for multi-agent operations
type CollaborationTracer struct {
    tracer trace.Tracer
}

// NewCollaborationTracer creates a new tracer
func NewCollaborationTracer() *CollaborationTracer {
    return &CollaborationTracer{
        tracer: otel.Tracer("devops-mcp-collaboration"),
    }
}

// TraceTaskDelegation traces task delegation
func (t *CollaborationTracer) TraceTaskDelegation(ctx context.Context, taskID, fromAgent, toAgent string) (context.Context, trace.Span) {
    return t.tracer.Start(ctx, "task.delegation",
        trace.WithAttributes(
            attribute.String("task.id", taskID),
            attribute.String("from.agent", fromAgent),
            attribute.String("to.agent", toAgent),
        ),
    )
}

// TraceWorkflowExecution traces workflow execution
func (t *CollaborationTracer) TraceWorkflowExecution(ctx context.Context, workflowID, executionID string) (context.Context, trace.Span) {
    return t.tracer.Start(ctx, "workflow.execution",
        trace.WithAttributes(
            attribute.String("workflow.id", workflowID),
            attribute.String("execution.id", executionID),
        ),
    )
}
```

### 3. Alerting Rules

```yaml
# File: monitoring/prometheus/alerts/collaboration_alerts.yml
groups:
  - name: collaboration
    interval: 30s
    rules:
      # Task alerts
      - alert: HighTaskFailureRate
        expr: |
          rate(devops_mcp_task_failed_total[5m]) 
          / rate(devops_mcp_task_created_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High task failure rate detected"
          description: "{{ $value | humanizePercentage }} of tasks are failing"
      
      # Workflow alerts
      - alert: WorkflowExecutionSlow
        expr: |
          histogram_quantile(0.95, rate(devops_mcp_workflow_duration_seconds_bucket[5m])) > 300
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Workflow execution is slow"
          description: "95th percentile workflow duration is {{ $value }}s"
      
      # Agent alerts
      - alert: LowAgentAvailability
        expr: |
          sum(devops_mcp_active_agents{status="available"}) 
          / sum(devops_mcp_active_agents) < 0.5
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Low agent availability"
          description: "Only {{ $value | humanizePercentage }} of agents are available"
      
      # WebSocket alerts
      - alert: WebSocketConnectionSpike
        expr: |
          rate(devops_mcp_websocket_connections[1m]) > 100
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "WebSocket connection spike detected"
          description: "Connection rate is {{ $value }} per minute"
      
      # Conflict alerts
      - alert: HighConflictRate
        expr: |
          rate(devops_mcp_conflict_resolutions_total[5m]) > 10
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High conflict resolution rate"
          description: "{{ $value }} conflicts per minute"
```

### 4. Grafana Dashboards

```json
{
  "dashboard": {
    "title": "Multi-Agent Collaboration Dashboard",
    "panels": [
      {
        "title": "Task Metrics",
        "targets": [
          {
            "expr": "rate(devops_mcp_task_created_total[5m])",
            "legendFormat": "Created - {{type}}"
          },
          {
            "expr": "rate(devops_mcp_task_completed_total[5m])",
            "legendFormat": "Completed - {{type}}"
          }
        ]
      },
      {
        "title": "Task Duration (p95)",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(devops_mcp_task_duration_seconds_bucket[5m]))",
            "legendFormat": "{{type}} - {{priority}}"
          }
        ]
      },
      {
        "title": "Active Agents by Capability",
        "targets": [
          {
            "expr": "devops_mcp_active_agents",
            "legendFormat": "{{capability}} - {{status}}"
          }
        ]
      },
      {
        "title": "Workflow Success Rate",
        "targets": [
          {
            "expr": "rate(devops_mcp_workflow_completed_total[5m]) / (rate(devops_mcp_workflow_completed_total[5m]) + rate(devops_mcp_workflow_failed_total[5m]))",
            "legendFormat": "{{type}}"
          }
        ]
      }
    ]
  }
}
```

### 5. SLO/SLI Definitions

```yaml
# File: monitoring/slo/collaboration_slos.yaml
slos:
  - name: task_completion_rate
    description: "Tasks should complete successfully"
    sli:
      type: ratio
      good_events: "devops_mcp_task_completed_total{status='completed'}"
      total_events: "devops_mcp_task_created_total"
    objectives:
      - target: 99.9
        window: 30d
      - target: 99.5
        window: 7d
    error_budget_policy:
      - action: "page_oncall"
        threshold: 0.1  # 10% of error budget consumed
      - action: "freeze_deployments"
        threshold: 0.5  # 50% of error budget consumed
  
  - name: workflow_latency
    description: "Workflows should complete within SLA"
    sli:
      type: latency
      metric: "devops_mcp_workflow_duration_seconds"
      threshold: 300  # 5 minutes
    objectives:
      - target: 95
        window: 30d
      - target: 90
        window: 7d
  
  - name: agent_availability
    description: "Sufficient agents should be available"
    sli:
      type: availability
      metric: "devops_mcp_active_agents{status='available'} / devops_mcp_active_agents"
    objectives:
      - target: 90
        window: 30d
  
  - name: websocket_reliability
    description: "WebSocket connections should be stable"
    sli:
      type: ratio
      good_events: "devops_mcp_websocket_messages_total - devops_mcp_websocket_errors_total"
      total_events: "devops_mcp_websocket_messages_total"
    objectives:
      - target: 99.99
        window: 30d
```

### 6. Comprehensive Observability Stack

```go
// File: pkg/observability/unified_observability.go
package observability

import (
    "context"
    "time"
    
    "github.com/prometheus/client_golang/prometheus"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace"
    "go.opentelemetry.io/otel/exporters/prometheus"
    "go.opentelemetry.io/otel/sdk/metric"
    "go.opentelemetry.io/otel/sdk/trace"
    "go.uber.org/zap"
    "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// UnifiedObservability provides complete observability
type UnifiedObservability struct {
    metrics      *MetricsCollector
    tracer       trace.TracerProvider
    logger       *zap.Logger
    profiler     *Profiler
    errorTracker *ErrorTracker
    sloMonitor   *SLOMonitor
}

// MetricsCollector collects all metrics
type MetricsCollector struct {
    // Business metrics
    taskThroughput    *prometheus.HistogramVec
    workflowDuration  *prometheus.HistogramVec
    agentUtilization  *prometheus.GaugeVec
    conflictRate      *prometheus.CounterVec
    
    // Technical metrics
    apiLatency        *prometheus.HistogramVec
    dbQueryDuration   *prometheus.HistogramVec
    cacheHitRate      *prometheus.GaugeVec
    memoryUsage       *prometheus.GaugeVec
    
    // Custom metrics
    customMetrics     map[string]prometheus.Collector
}

// Profiler handles continuous profiling
type Profiler struct {
    cpuProfiler    *CPUProfiler
    memProfiler    *MemoryProfiler
    blockProfiler  *BlockingProfiler
    mutexProfiler  *MutexProfiler
}

// ErrorTracker tracks and analyzes errors
type ErrorTracker struct {
    errorCounts    *prometheus.CounterVec
    errorPatterns  map[string]*ErrorPattern
    anomalyDetector *AnomalyDetector
}

// SLOMonitor monitors SLO compliance
type SLOMonitor struct {
    slos           map[string]*SLO
    errorBudgets   map[string]*ErrorBudget
    burnRates      map[string]float64
    alertManager   *AlertManager
}

// Initialize complete observability
func InitializeObservability(config ObservabilityConfig) (*UnifiedObservability, error) {
    // Initialize structured logging
    logger, err := initLogger(config.Logging)
    if err != nil {
        return nil, err
    }
    
    // Initialize metrics
    metricsCollector := initMetrics(config.Metrics)
    
    // Initialize tracing
    tracerProvider, err := initTracing(config.Tracing)
    if err != nil {
        return nil, err
    }
    
    // Initialize profiling
    profiler := initProfiling(config.Profiling)
    
    // Initialize error tracking
    errorTracker := initErrorTracking(config.ErrorTracking)
    
    // Initialize SLO monitoring
    sloMonitor := initSLOMonitoring(config.SLOs)
    
    return &UnifiedObservability{
        metrics:      metricsCollector,
        tracer:       tracerProvider,
        logger:       logger,
        profiler:     profiler,
        errorTracker: errorTracker,
        sloMonitor:   sloMonitor,
    }, nil
}

// RecordTaskOperation records comprehensive task metrics
func (o *UnifiedObservability) RecordTaskOperation(ctx context.Context, op TaskOperation) {
    // Start span
    ctx, span := o.tracer.Tracer("task").Start(ctx, "operation")
    defer span.End()
    
    start := time.Now()
    
    // Record metrics
    defer func() {
        duration := time.Since(start)
        
        o.metrics.taskThroughput.WithLabelValues(
            op.Type, op.Priority, op.Status,
        ).Observe(duration.Seconds())
        
        // Log structured data
        o.logger.Info("task_operation",
            zap.String("task_id", op.TaskID),
            zap.String("type", op.Type),
            zap.Duration("duration", duration),
            zap.String("status", op.Status),
        )
        
        // Update SLO tracking
        o.sloMonitor.RecordEvent("task_completion", op.Status == "completed")
        
        // Profile if slow
        if duration > 5*time.Second {
            o.profiler.CaptureProfile(ctx, "slow_task_operation")
        }
    }()
}
```

### 7. Test Data Management

```go
// File: test/testdata/data_manager.go
package testdata

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    
    "github.com/google/uuid"
    "github.com/S-Corkum/devops-mcp/pkg/models"
)

// TestDataManager manages test data lifecycle
type TestDataManager struct {
    mu          sync.RWMutex
    scenarios   map[string]*TestScenario
    fixtures    map[string]interface{}
    generators  map[string]DataGenerator
    snapshots   map[string]*DataSnapshot
}

// TestScenario represents a complete test scenario
type TestScenario struct {
    ID          string
    Name        string
    Description string
    Agents      []AgentFixture
    Tasks       []TaskFixture
    Workflows   []WorkflowFixture
    Workspaces  []WorkspaceFixture
    Setup       []SetupStep
    Teardown    []TeardownStep
}

// DataGenerator generates test data
type DataGenerator interface {
    Generate(count int) []interface{}
    GenerateWithConstraints(count int, constraints map[string]interface{}) []interface{}
}

// LoadScenario loads a test scenario
func (m *TestDataManager) LoadScenario(name string) (*TestScenario, error) {
    m.mu.RLock()
    scenario, exists := m.scenarios[name]
    m.mu.RUnlock()
    
    if !exists {
        // Load from file
        scenario, err := m.loadScenarioFromFile(name)
        if err != nil {
            return nil, err
        }
        
        m.mu.Lock()
        m.scenarios[name] = scenario
        m.mu.Unlock()
    }
    
    return scenario, nil
}

// SetupScenario sets up all test data for a scenario
func (m *TestDataManager) SetupScenario(ctx context.Context, scenario *TestScenario) error {
    // Take snapshot for rollback
    snapshot := m.takeSnapshot()
    
    // Execute setup steps
    for _, step := range scenario.Setup {
        if err := step.Execute(ctx); err != nil {
            // Rollback on failure
            m.restoreSnapshot(snapshot)
            return fmt.Errorf("setup step %s failed: %w", step.Name, err)
        }
    }
    
    // Create agents
    for _, agent := range scenario.Agents {
        if err := m.createAgent(ctx, agent); err != nil {
            m.restoreSnapshot(snapshot)
            return err
        }
    }
    
    // Create tasks
    for _, task := range scenario.Tasks {
        if err := m.createTask(ctx, task); err != nil {
            m.restoreSnapshot(snapshot)
            return err
        }
    }
    
    return nil
}

// GenerateLoadTestData generates data for load testing
func (m *TestDataManager) GenerateLoadTestData(config LoadTestConfig) (*LoadTestData, error) {
    data := &LoadTestData{
        Agents:     make([]*models.Agent, 0, config.NumAgents),
        Tasks:      make([]*models.Task, 0, config.NumTasks),
        Workflows:  make([]*models.Workflow, 0, config.NumWorkflows),
    }
    
    // Generate agents with varied capabilities
    agentGen := m.generators["agent"].(*AgentGenerator)
    agents := agentGen.GenerateWithConstraints(config.NumAgents, map[string]interface{}{
        "capability_distribution": config.CapabilityDistribution,
        "availability_range":      config.AvailabilityRange,
    })
    
    for _, agent := range agents {
        data.Agents = append(data.Agents, agent.(*models.Agent))
    }
    
    // Generate tasks with realistic distribution
    taskGen := m.generators["task"].(*TaskGenerator)
    tasks := taskGen.GenerateWithConstraints(config.NumTasks, map[string]interface{}{
        "type_distribution":     config.TaskTypeDistribution,
        "priority_distribution": config.PriorityDistribution,
        "complexity_range":      config.ComplexityRange,
    })
    
    for _, task := range tasks {
        data.Tasks = append(data.Tasks, task.(*models.Task))
    }
    
    return data, nil
}
```

### 8. CI/CD Integration

```yaml
# File: .github/workflows/comprehensive-testing.yml
name: Comprehensive Testing Pipeline

on:
  pull_request:
    branches: [main]
  push:
    branches: [main]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go-version: [1.21, 1.22, 1.23]
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: ${{ matrix.go-version }}
      
      - name: Run unit tests with coverage
        run: |
          make test-unit-coverage
          
      - name: Check coverage threshold
        run: |
          coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          if (( $(echo "$coverage < 90" | bc -l) )); then
            echo "Coverage $coverage% is below 90% threshold"
            exit 1
          fi
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out
  
  integration-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: postgres
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
      
      redis:
        image: redis:7-alpine
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Run integration tests
        run: make test-integration
        env:
          DATABASE_URL: postgres://postgres:postgres@localhost:5432/test
          REDIS_URL: redis://localhost:6379
  
  security-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Run security tests
        run: make test-security
      
      - name: Run vulnerability scan
        uses: aquasecurity/trivy-action@master
        with:
          scan-type: 'fs'
          scan-ref: '.'
          format: 'sarif'
          output: 'trivy-results.sarif'
      
      - name: Upload Trivy scan results
        uses: github/codeql-action/upload-sarif@v2
        with:
          sarif_file: 'trivy-results.sarif'
  
  load-tests:
    runs-on: ubuntu-latest
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v4
      
      - name: Run load tests
        run: make test-load
        env:
          LOAD_TEST_DURATION: 5m
          LOAD_TEST_USERS: 100
      
      - name: Analyze performance results
        run: |
          make analyze-performance
          
      - name: Comment PR with results
        if: github.event_name == 'pull_request'
        uses: actions/github-script@v6
        with:
          script: |
            const fs = require('fs');
            const results = fs.readFileSync('performance-results.md', 'utf8');
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: results
            });
  
  mutation-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Run mutation tests
        run: make test-mutation
        timeout-minutes: 30
      
      - name: Check mutation score
        run: |
          score=$(cat mutation-score.txt)
          if (( $(echo "$score < 80" | bc -l) )); then
            echo "Mutation score $score% is below 80% threshold"
            exit 1
          fi
  
  contract-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Run contract tests
        run: make test-contract
      
      - name: Publish contracts
        if: github.ref == 'refs/heads/main'
        run: |
          make publish-contracts
        env:
          PACT_BROKER_URL: ${{ secrets.PACT_BROKER_URL }}
          PACT_BROKER_TOKEN: ${{ secrets.PACT_BROKER_TOKEN }}
```

### 9. Incident Response Runbooks

```markdown
# File: docs/runbooks/multi-agent-incidents.md

## High Task Failure Rate

### Alert
`HighTaskFailureRate` - Task failure rate > 10%

### Impact
- Degraded service performance
- Delayed task completion
- Potential data inconsistency

### Investigation Steps
1. Check task failure metrics by type:
   ```promql
   rate(devops_mcp_task_failed_total[5m]) by (type, reason)
   ```

2. Identify failing agents:
   ```promql
   rate(devops_mcp_agent_failures[5m]) by (agent_id, error_type)
   ```

3. Check system resources:
   ```bash
   kubectl top nodes
   kubectl top pods -n mcp
   ```

4. Review recent deployments:
   ```bash
   kubectl rollout history deployment/mcp-server -n mcp
   ```

### Mitigation Steps
1. **Immediate**:
   - Enable circuit breakers for failing services
   - Increase task retry limits temporarily
   - Scale up healthy agents

2. **Short-term**:
   - Rollback recent deployments if identified as cause
   - Redistribute load from failing agents
   - Clear any backlogs in task queues

3. **Long-term**:
   - Implement additional monitoring
   - Add automated recovery mechanisms
   - Review and update retry policies

### Escalation
- After 15 minutes: Page on-call engineer
- After 30 minutes: Escalate to team lead
- After 1 hour: Involve platform team

## WebSocket Connection Storm

### Alert
`WebSocketConnectionSpike` - Connection rate > 100/minute

### Impact
- Resource exhaustion
- Degraded performance for existing connections
- Potential service outage

### Investigation Steps
1. Identify connection sources:
   ```promql
   rate(devops_mcp_websocket_connections[1m]) by (source_ip, agent_type)
   ```

2. Check authentication failures:
   ```promql
   rate(devops_mcp_auth_failures[5m]) by (reason)
   ```

3. Analyze connection patterns:
   ```bash
   kubectl logs -n mcp deployment/mcp-server --since=10m | grep "connection"
   ```

### Mitigation Steps
1. **Immediate**:
   - Enable rate limiting at ingress
   - Increase connection pool limits
   - Block suspicious IPs

2. **Short-term**:
   - Scale WebSocket servers horizontally
   - Implement connection queueing
   - Add CAPTCHA for new connections

3. **Long-term**:
   - Implement progressive rate limiting
   - Add anomaly detection
   - Review authentication mechanisms
```

## Opus 4 Implementation Notes

When implementing with Claude Code Opus 4:

1. **Complete Test Suites**: Request generation of all test categories at once
2. **Edge Cases**: Include tests for all failure scenarios
3. **Performance Tests**: Generate comprehensive benchmarks
4. **Monitoring Setup**: Create complete observability configuration
5. **Documentation**: Generate test documentation and runbooks

## Enhanced Test Execution Strategy

### Local Development Testing

```bash
# Quick validation before commit
make pre-commit  # Runs fmt, lint, unit tests

# Comprehensive local testing
make test-all-local  # All tests except load/chaos

# Specific test suites with filtering
make test-unit FOCUS="TaskService"
make test-integration FOCUS="WebSocket"
make test-functional FOCUS="MultiAgent"

# Security testing
make test-security
make test-fuzzing FUZZ_TIME=60s

# Performance testing
make bench PACKAGE=./pkg/collaboration/crdt
make test-load USERS=50 DURATION=2m

# Contract testing
make test-contract
make verify-contracts

# Mutation testing
make test-mutation PACKAGE=./pkg/services

# Generate comprehensive report
make test-report FORMAT=html
```

### CI/CD Pipeline Testing

```yaml
# Staging deployment criteria
staging_deployment:
  requires:
    - unit_test_coverage: ">= 90%"
    - integration_tests: "PASS"
    - security_scan: "NO_HIGH_VULNS"
    - contract_tests: "VERIFIED"
    - performance_regression: "< 5%"
    - mutation_score: ">= 80%"

# Production deployment criteria  
production_deployment:
  requires:
    - staging_soak_test: "72_HOURS"
    - load_test: "PASS_SLA"
    - chaos_test: "RECOVERED"
    - security_audit: "APPROVED"
    - runbook_validation: "COMPLETE"
    - rollback_tested: "VERIFIED"
```

### Test Environment Management

```bash
# Environment setup
make test-env-up      # Start all test dependencies
make test-env-verify  # Verify environment health
make test-env-reset   # Clean and reset data
make test-env-down    # Teardown environment

# Data management
make test-data-generate SCENARIO=high_load
make test-data-import FILE=production_sample.json
make test-data-snapshot NAME=pre_chaos_test
make test-data-restore NAME=pre_chaos_test

# Performance profiling
make profile-tests PACKAGE=./test/load
make analyze-test-performance
make compare-benchmarks BASE=main COMPARE=feature
```

### Testing Best Practices

1. **Test Pyramid**:
   - 70% Unit tests (milliseconds)
   - 20% Integration tests (seconds)
   - 10% E2E tests (minutes)

2. **Test Data**:
   - Use factories for consistent data
   - Snapshot important states
   - Clean up after each test
   - Never use production data

3. **Test Isolation**:
   - Each test independent
   - No shared state
   - Parallel execution safe
   - Deterministic results

4. **Performance Testing**:
   - Baseline before changes
   - Test under realistic load
   - Monitor resource usage
   - Set clear SLA targets

5. **Security Testing**:
   - Test all auth paths
   - Verify tenant isolation
   - Check input validation
   - Scan for vulnerabilities

### Monitoring in Production

```bash
# Real-time monitoring
make monitor-dashboard      # Open Grafana
make monitor-alerts        # View active alerts
make monitor-slo           # Check SLO compliance
make monitor-traces        # View distributed traces

# Incident response
make incident-create SEVERITY=high
make incident-timeline ID=INC-123
make incident-postmortem ID=INC-123

# Performance analysis
make analyze-latency SERVICE=mcp-server PERIOD=1h
make analyze-errors SERVICE=all PERIOD=24h
make analyze-capacity FORECAST=7d
```

## Production Readiness Checklist

### Testing Requirements
- [ ] Unit test coverage > 90%
- [ ] All integration tests passing
- [ ] Functional tests cover all user journeys
- [ ] Load tests meet performance SLAs
- [ ] Chaos tests demonstrate resilience
- [ ] Security tests show no vulnerabilities
- [ ] Contract tests verified with consumers
- [ ] Mutation score > 80%

### Monitoring Requirements
- [ ] All SLOs defined and measured
- [ ] Comprehensive dashboards created
- [ ] Alerts configured with runbooks
- [ ] Distributed tracing enabled
- [ ] Error tracking integrated
- [ ] Performance baselines established
- [ ] Capacity planning completed

### Operational Requirements
- [ ] Incident response procedures documented
- [ ] On-call rotation established
- [ ] Runbooks for common issues
- [ ] Deployment rollback tested
- [ ] Disaster recovery plan verified
- [ ] Security audit completed
- [ ] Compliance requirements met

## Next Steps

After completing Phase 6:
1. **Staging Deployment**:
   - Deploy complete system to staging
   - Run 72-hour soak test
   - Conduct security penetration testing
   - Perform user acceptance testing

2. **Production Preparation**:
   - Finalize operational runbooks
   - Train support team
   - Set up monitoring alerts
   - Prepare rollback plan

3. **Production Rollout**:
   - Canary deployment (5% traffic)
   - Progressive rollout (25%, 50%, 100%)
   - Monitor SLOs closely
   - Be ready for quick rollback

4. **Post-Deployment**:
   - Monitor for 2 weeks
   - Gather performance metrics
   - Conduct retrospective
   - Plan optimization phase