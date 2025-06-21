package api

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// WebSocketClient is a test client wrapper
type WebSocketClient struct {
	AgentID string
	Conn    *websocket.Conn
	t       *testing.T
}

// WebSocketMessage represents a message for testing
type WebSocketMessage struct {
	Type string                 `json:"type"`
	ID   string                 `json:"id"`
	Data map[string]interface{} `json:"data,omitempty"`
}

// NewWebSocketClient creates a new test WebSocket client
func NewWebSocketClient(t *testing.T, agentID string, capabilities []string) *WebSocketClient {
	// For now, return a stub implementation
	// TODO: Implement proper connection logic
	return &WebSocketClient{
		AgentID: agentID,
		t:       t,
	}
}

// SendMessage sends a message through the WebSocket
func (c *WebSocketClient) SendMessage(msg WebSocketMessage) error {
	if c.Conn == nil {
		return fmt.Errorf("connection not established")
	}
	ctx := context.Background()
	return wsjson.Write(ctx, c.Conn, msg)
}

// ReadMessage reads a message from the WebSocket
func (c *WebSocketClient) ReadMessage(ctx context.Context) (*WebSocketMessage, error) {
	if c.Conn == nil {
		return nil, fmt.Errorf("connection not established")
	}
	var msg WebSocketMessage
	err := wsjson.Read(ctx, c.Conn, &msg)
	return &msg, err
}

// Close closes the WebSocket connection
func (c *WebSocketClient) Close() error {
	if c.Conn != nil {
		return c.Conn.Close(websocket.StatusNormalClosure, "")
	}
	return nil
}

// Test task delegation between multiple agents
func TestWebSocketTaskDelegation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect two agents
	agent1 := NewWebSocketClient(t, "agent1", []string{"coding", "testing"})
	defer agent1.Close()

	agent2 := NewWebSocketClient(t, "agent2", []string{"documentation", "testing"})
	defer agent2.Close()

	// Agent 1 creates a task
	taskID := uuid.New()
	createTaskMsg := WebSocketMessage{
		Type: "task.create",
		ID:   uuid.New().String(),
		Data: map[string]interface{}{
			"task": map[string]interface{}{
				"id":          taskID.String(),
				"title":       "Write unit tests",
				"description": "Create comprehensive unit tests for the authentication module",
				"priority":    "high",
				"type":        "coding",
			},
		},
	}

	err := agent1.SendMessage(createTaskMsg)
	require.NoError(t, err)

	// Wait for confirmation
	msg, err := agent1.ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, "task.created", msg.Type)

	// Agent 1 delegates task to Agent 2
	delegateMsg := WebSocketMessage{
		Type: "task.delegate",
		ID:   uuid.New().String(),
		Data: map[string]interface{}{
			"task_id":         taskID.String(),
			"to_agent":        "agent2",
			"reason":          "Agent 2 has better testing expertise",
			"delegation_type": "manual",
		},
	}

	err = agent1.SendMessage(delegateMsg)
	require.NoError(t, err)

	// Agent 2 should receive delegation notification
	msg, err = agent2.ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, "task.delegated", msg.Type)

	assert.Equal(t, taskID.String(), msg.Data["task_id"])
	assert.Equal(t, "agent1", msg.Data["from_agent"])

	// Agent 2 accepts the task
	acceptMsg := WebSocketMessage{
		Type: "task.accept",
		ID:   uuid.New().String(),
		Data: map[string]interface{}{
			"task_id": taskID.String(),
		},
	}

	err = agent2.SendMessage(acceptMsg)
	require.NoError(t, err)

	// Verify acceptance
	msg, err = agent2.ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, "task.accepted", msg.Type)
}

// Test workflow coordination between multiple agents
func TestWebSocketWorkflowCoordination(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect three agents with different capabilities
	coder := NewWebSocketClient(t, "coder", []string{"coding", "debugging"})
	defer coder.Close()

	tester := NewWebSocketClient(t, "tester", []string{"testing", "qa"})
	defer tester.Close()

	reviewer := NewWebSocketClient(t, "reviewer", []string{"review", "documentation"})
	defer reviewer.Close()

	// Create a workflow
	workflowID := uuid.New()
	createWorkflowMsg := WebSocketMessage{
		Type: "workflow.create",
		ID:   uuid.New().String(),
		Data: map[string]interface{}{
			"workflow": map[string]interface{}{
				"id":          workflowID.String(),
				"name":        "Feature Development Pipeline",
				"description": "Complete feature development with code, test, and review",
				"steps": []map[string]interface{}{
					{
						"id":    uuid.New().String(),
						"name":  "Code Implementation",
						"type":  "task",
						"order": 1,
						"config": map[string]interface{}{
							"required_capability": "coding",
						},
					},
					{
						"id":    uuid.New().String(),
						"name":  "Write Tests",
						"type":  "task",
						"order": 2,
						"config": map[string]interface{}{
							"required_capability": "testing",
						},
					},
					{
						"id":    uuid.New().String(),
						"name":  "Code Review",
						"type":  "task",
						"order": 3,
						"config": map[string]interface{}{
							"required_capability": "review",
						},
					},
				},
			},
		},
	}

	err := coder.SendMessage(createWorkflowMsg)
	require.NoError(t, err)

	// Wait for confirmation
	msg, err := coder.ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, "workflow.created", msg.Type)

	// Start workflow execution
	startMsg := WebSocketMessage{
		Type: "workflow.start",
		ID:   uuid.New().String(),
		Data: map[string]interface{}{
			"workflow_id": workflowID.String(),
		},
	}

	err = coder.SendMessage(startMsg)
	require.NoError(t, err)

	// Coder should receive first task
	msg, err = coder.ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, "task.assigned", msg.Type)

	// Simulate step completion by each agent
	agents := []*WebSocketClient{coder, tester, reviewer}
	for i, agent := range agents {
		// Complete current step
		completeMsg := WebSocketMessage{
			Type: "workflow.step.complete",
			ID:   uuid.New().String(),
			Data: map[string]interface{}{
				"workflow_id": workflowID.String(),
				"step_index":  i,
				"result": map[string]interface{}{
					"status": "success",
					"output": fmt.Sprintf("Step %d completed by %s", i+1, agent.AgentID),
				},
			},
		}

		err = agent.SendMessage(completeMsg)
		require.NoError(t, err)

		// If not the last step, next agent should receive task
		if i < len(agents)-1 {
			msg, err = agents[i+1].ReadMessage(ctx)
			require.NoError(t, err)
			assert.Equal(t, "task.assigned", msg.Type)
		}
	}

	// Final agent should receive workflow completion
	msg, err = reviewer.ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, "workflow.completed", msg.Type)
}

// Test workspace collaboration with real-time updates
func TestWebSocketWorkspaceCollaboration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect multiple agents
	numAgents := 3
	agents := make([]*WebSocketClient, numAgents)
	for i := 0; i < numAgents; i++ {
		agents[i] = NewWebSocketClient(t, fmt.Sprintf("agent%d", i+1), []string{"collaboration"})
		defer agents[i].Close()
	}

	// First agent creates workspace
	workspaceID := uuid.New()
	createWorkspaceMsg := WebSocketMessage{
		Type: "workspace.create",
		ID:   uuid.New().String(),
		Data: map[string]interface{}{
			"workspace": map[string]interface{}{
				"id":          workspaceID.String(),
				"name":        "Test Collaboration Space",
				"description": "Space for testing multi-agent collaboration",
			},
		},
	}

	err := agents[0].SendMessage(createWorkspaceMsg)
	require.NoError(t, err)

	msg, err := agents[0].ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, "workspace.created", msg.Type)

	// Join other agents to workspace
	for i := 1; i < numAgents; i++ {
		joinMsg := WebSocketMessage{
			Type: "workspace.join",
			ID:   uuid.New().String(),
			Data: map[string]interface{}{
				"workspace_id": workspaceID.String(),
			},
		}

		err = agents[i].SendMessage(joinMsg)
		require.NoError(t, err)

		// All existing members should receive join notification
		for j := 0; j < i; j++ {
			msg, err = agents[j].ReadMessage(ctx)
			require.NoError(t, err)
			assert.Equal(t, "workspace.member.joined", msg.Type)
		}

		// Joining agent receives confirmation
		msg, err = agents[i].ReadMessage(ctx)
		require.NoError(t, err)
		assert.Equal(t, "workspace.joined", msg.Type)
	}

	// Test real-time document collaboration
	documentID := uuid.New()
	createDocMsg := WebSocketMessage{
		Type: "document.create",
		ID:   uuid.New().String(),
		Data: map[string]interface{}{
			"workspace_id": workspaceID.String(),
			"document": map[string]interface{}{
				"id":      documentID.String(),
				"title":   "Collaborative Design Doc",
				"content": "Initial content",
			},
		},
	}

	err = agents[0].SendMessage(createDocMsg)
	require.NoError(t, err)

	// All agents should receive document creation notification
	var wg sync.WaitGroup
	wg.Add(numAgents)

	for i := range agents {
		go func(agent *WebSocketClient) {
			defer wg.Done()
			msg, err := agent.ReadMessage(ctx)
			require.NoError(t, err)
			assert.Equal(t, "document.created", msg.Type)
		}(agents[i])
	}

	wg.Wait()

	// Test concurrent edits with CRDT
	edits := make(chan bool, numAgents-1)

	for i := 1; i < numAgents; i++ {
		go func(agentIndex int) {
			editMsg := WebSocketMessage{
				Type: "document.edit",
				ID:   uuid.New().String(),
				Data: map[string]interface{}{
					"document_id": documentID.String(),
					"operation": map[string]interface{}{
						"type":     "insert",
						"position": agentIndex * 10,
						"content":  fmt.Sprintf("\nEdit from agent%d", agentIndex+1),
					},
				},
			}

			err := agents[agentIndex].SendMessage(editMsg)
			assert.NoError(t, err)
			edits <- true
		}(i)
	}

	// Wait for all edits to complete
	for i := 1; i < numAgents; i++ {
		<-edits
	}

	// Request final document state
	getDocMsg := WebSocketMessage{
		Type: "document.get",
		ID:   uuid.New().String(),
		Data: map[string]interface{}{
			"document_id": documentID.String(),
		},
	}

	err = agents[0].SendMessage(getDocMsg)
	require.NoError(t, err)

	msg, err = agents[0].ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, "document.state", msg.Type)

	// Verify all edits were applied
	content := msg.Data["content"].(string)
	for i := 1; i < numAgents; i++ {
		assert.Contains(t, content, fmt.Sprintf("Edit from agent%d", i+1))
	}
}

// Test conflict resolution with vector clocks
func TestWebSocketConflictResolution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect two agents
	agent1 := NewWebSocketClient(t, "agent1", []string{"editing"})
	defer agent1.Close()

	agent2 := NewWebSocketClient(t, "agent2", []string{"editing"})
	defer agent2.Close()

	// Create shared state
	stateID := uuid.New()
	createStateMsg := WebSocketMessage{
		Type: "state.create",
		ID:   uuid.New().String(),
		Data: map[string]interface{}{
			"state": map[string]interface{}{
				"id":    stateID.String(),
				"type":  "counter",
				"value": 0,
			},
		},
	}

	err := agent1.SendMessage(createStateMsg)
	require.NoError(t, err)

	// Both agents receive confirmation
	msg1, err := agent1.ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, "state.created", msg1.Type)

	// Agent2 subscribes to state
	subscribeMsg := WebSocketMessage{
		Type: "state.subscribe",
		ID:   uuid.New().String(),
		Data: map[string]interface{}{
			"state_id": stateID.String(),
		},
	}

	err = agent2.SendMessage(subscribeMsg)
	require.NoError(t, err)

	msg2, err := agent2.ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, "state.subscribed", msg2.Type)

	// Both agents increment counter concurrently
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			incrementMsg := WebSocketMessage{
				Type: "state.increment",
				ID:   uuid.New().String(),
				Data: map[string]interface{}{
					"state_id": stateID.String(),
					"delta":    1,
				},
			}
			err := agent1.SendMessage(incrementMsg)
			assert.NoError(t, err)
			time.Sleep(10 * time.Millisecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			incrementMsg := WebSocketMessage{
				Type: "state.increment",
				ID:   uuid.New().String(),
				Data: map[string]interface{}{
					"state_id": stateID.String(),
					"delta":    2,
				},
			}
			err := agent2.SendMessage(incrementMsg)
			assert.NoError(t, err)
			time.Sleep(10 * time.Millisecond)
		}
	}()

	wg.Wait()

	// Allow time for CRDT synchronization
	time.Sleep(100 * time.Millisecond)

	// Query final state
	queryMsg := WebSocketMessage{
		Type: "state.get",
		ID:   uuid.New().String(),
		Data: map[string]interface{}{
			"state_id": stateID.String(),
		},
	}

	err = agent1.SendMessage(queryMsg)
	require.NoError(t, err)

	// Read messages until we get state response
	var finalValue int
	for {
		msg, err := agent1.ReadMessage(ctx)
		if err != nil {
			break
		}
		if msg.Type == "state.value" {
			finalValue = int(msg.Data["value"].(float64))
			break
		}
	}

	// Should have all increments: 5*1 + 5*2 = 15
	assert.Equal(t, 15, finalValue)
}

// Test agent capability matching for task assignment
func TestWebSocketCapabilityMatching(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect agents with different capabilities
	agents := []*WebSocketClient{
		NewWebSocketClient(t, "frontend-dev", []string{"javascript", "react", "css"}),
		NewWebSocketClient(t, "backend-dev", []string{"golang", "postgresql", "redis"}),
		NewWebSocketClient(t, "ml-engineer", []string{"python", "tensorflow", "data-analysis"}),
		NewWebSocketClient(t, "devops", []string{"kubernetes", "terraform", "monitoring"}),
	}

	for _, agent := range agents {
		defer agent.Close()
	}

	// Create tasks requiring specific capabilities
	tasks := []struct {
		id            string
		title         string
		capabilities  []string
		expectedAgent string
	}{
		{
			id:            uuid.New().String(),
			title:         "Implement React Component",
			capabilities:  []string{"react", "javascript"},
			expectedAgent: "frontend-dev",
		},
		{
			id:            uuid.New().String(),
			title:         "Optimize Database Queries",
			capabilities:  []string{"postgresql"},
			expectedAgent: "backend-dev",
		},
		{
			id:            uuid.New().String(),
			title:         "Deploy to Kubernetes",
			capabilities:  []string{"kubernetes"},
			expectedAgent: "devops",
		},
	}

	// Create and auto-assign tasks
	for _, task := range tasks {
		createMsg := WebSocketMessage{
			Type: "task.create.auto_assign",
			ID:   uuid.New().String(),
			Data: map[string]interface{}{
				"task": map[string]interface{}{
					"id":                    task.id,
					"title":                 task.title,
					"required_capabilities": task.capabilities,
				},
				"assignment_strategy": "capability_match",
			},
		}

		err := agents[0].SendMessage(createMsg)
		require.NoError(t, err)

		// Find which agent received the assignment
		assigned := make(chan string, 1)
		var assignWg sync.WaitGroup
		assignWg.Add(len(agents))

		for _, agent := range agents {
			go func(a *WebSocketClient) {
				defer assignWg.Done()
				msg, err := a.ReadMessage(ctx)
				if err == nil && msg.Type == "task.assigned" {
					if msg.Data["task_id"] == task.id {
						assigned <- a.AgentID
					}
				}
			}(agent)
		}

		// Wait with timeout
		go func() {
			assignWg.Wait()
			close(assigned)
		}()

		select {
		case agentID := <-assigned:
			assert.Equal(t, task.expectedAgent, agentID, "Task %s assigned to wrong agent", task.title)
		case <-time.After(5 * time.Second):
			t.Fatalf("Task %s was not assigned within timeout", task.title)
		}
	}
}

// Test performance with multiple concurrent agents
func TestWebSocketMultiAgentPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	numAgents := 10
	numTasksPerAgent := 20

	// Track metrics
	var totalTasks int32
	var completedTasks atomic.Int32
	startTime := time.Now()

	// Connect multiple agents
	agents := make([]*WebSocketClient, numAgents)
	for i := 0; i < numAgents; i++ {
		agents[i] = NewWebSocketClient(t, fmt.Sprintf("perf-agent-%d", i), []string{"processing"})
		defer agents[i].Close()
	}

	// Create shared workspace
	workspaceID := uuid.New()
	createWorkspaceMsg := WebSocketMessage{
		Type: "workspace.create",
		ID:   uuid.New().String(),
		Data: map[string]interface{}{
			"workspace": map[string]interface{}{
				"id":   workspaceID.String(),
				"name": "Performance Test Workspace",
			},
		},
	}

	err := agents[0].SendMessage(createWorkspaceMsg)
	require.NoError(t, err)

	// Wait for workspace creation
	msg, err := agents[0].ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, "workspace.created", msg.Type)

	// All agents join workspace
	var joinWg sync.WaitGroup
	joinWg.Add(numAgents - 1)

	for i := 1; i < numAgents; i++ {
		go func(agentIndex int) {
			defer joinWg.Done()
			joinMsg := WebSocketMessage{
				Type: "workspace.join",
				ID:   uuid.New().String(),
				Data: map[string]interface{}{
					"workspace_id": workspaceID.String(),
				},
			}
			err := agents[agentIndex].SendMessage(joinMsg)
			assert.NoError(t, err)
		}(i)
	}

	joinWg.Wait()

	// Each agent creates and processes tasks
	var taskWg sync.WaitGroup
	taskWg.Add(numAgents)

	for i := 0; i < numAgents; i++ {
		go func(agentIndex int) {
			defer taskWg.Done()

			for j := 0; j < numTasksPerAgent; j++ {
				// Create task
				taskID := uuid.New()
				createTaskMsg := WebSocketMessage{
					Type: "task.create",
					ID:   uuid.New().String(),
					Data: map[string]interface{}{
						"task": map[string]interface{}{
							"id":           taskID.String(),
							"title":        fmt.Sprintf("Task %d-%d", agentIndex, j),
							"workspace_id": workspaceID.String(),
						},
					},
				}

				err := agents[agentIndex].SendMessage(createTaskMsg)
				assert.NoError(t, err)

				// Simulate processing
				time.Sleep(time.Duration(10+j%20) * time.Millisecond)

				// Complete task
				completeMsg := WebSocketMessage{
					Type: "task.complete",
					ID:   uuid.New().String(),
					Data: map[string]interface{}{
						"task_id": taskID.String(),
						"result":  map[string]interface{}{"processed": true},
					},
				}

				err = agents[agentIndex].SendMessage(completeMsg)
				assert.NoError(t, err)

				completedTasks.Add(1)
			}
		}(i)
	}

	taskWg.Wait()
	duration := time.Since(startTime)

	// Calculate metrics
	totalTasks = int32(numAgents * numTasksPerAgent)
	tasksPerSecond := float64(totalTasks) / duration.Seconds()

	t.Logf("Performance Test Results:")
	t.Logf("- Total agents: %d", numAgents)
	t.Logf("- Total tasks: %d", totalTasks)
	t.Logf("- Completed tasks: %d", completedTasks.Load())
	t.Logf("- Duration: %v", duration)
	t.Logf("- Tasks/second: %.2f", tasksPerSecond)

	// Assert minimum performance threshold
	assert.Greater(t, tasksPerSecond, 50.0, "Performance below threshold")
}

// Helper function to drain messages
func drainMessages(client *WebSocketClient, duration time.Duration) {
	timeout := time.After(duration)
	for {
		select {
		case <-timeout:
			return
		default:
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			var msg WebSocketMessage
			err := wsjson.Read(ctx, client.Conn, &msg)
			cancel()
			if err != nil {
				// Connection closed or timeout, exit drain
				return
			}
		}
	}
}
