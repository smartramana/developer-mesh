package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
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
	Type   int                    `json:"type"`
	ID     string                 `json:"id"`
	Method string                 `json:"method,omitempty"`
	Params map[string]interface{} `json:"params,omitempty"`
	Result interface{}            `json:"result,omitempty"`
	Error  *WebSocketError        `json:"error,omitempty"`
}

// WebSocketError represents an error in a WebSocket message
type WebSocketError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Message type constants
const (
	MessageTypeRequest      = 0
	MessageTypeResponse     = 1
	MessageTypeNotification = 2
	MessageTypeError        = 3
)

// NewWebSocketClient creates a new test WebSocket client
func NewWebSocketClient(t *testing.T, agentID string, capabilities []string) *WebSocketClient {
	// Get test configuration
	config := getTestConfig()

	// Use the appropriate API key for the agent
	apiKey := getAPIKeyForAgent(agentID)

	// Establish WebSocket connection
	conn, err := establishWebSocketConnection(t, config.WebSocketURL, apiKey, agentID, capabilities)
	require.NoError(t, err, "Failed to establish WebSocket connection for agent %s", agentID)

	return &WebSocketClient{
		AgentID: agentID,
		Conn:    conn,
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

	// Setup test data
	_, cleanup := SetupTestData(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Register two agents
	agent1 := NewWebSocketClient(t, "agent1", []string{"coding", "testing"})
	defer func() {
		_ = agent1.Close()
	}()

	agent2 := NewWebSocketClient(t, "agent2", []string{"documentation", "testing"})
	defer func() {
		_ = agent2.Close()
	}()

	// Wait for agents to be fully registered
	time.Sleep(500 * time.Millisecond)

	// Agent 1 creates a task
	taskID := uuid.New().String()
	createTaskMsg := WebSocketMessage{
		Type:   MessageTypeRequest,
		ID:     uuid.New().String(),
		Method: "task.create",
		Params: map[string]interface{}{
			"id":          taskID,
			"title":       "Implement authentication module",
			"description": "Create JWT-based authentication",
			"priority":    "high",
			"type":        "coding",
		},
	}

	err := agent1.SendMessage(createTaskMsg)
	require.NoError(t, err)

	// Wait for task creation response
	msg, err := agent1.ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, MessageTypeResponse, msg.Type)
	assert.Nil(t, msg.Error)
	
	// Extract the actual task ID from the response
	if result, ok := msg.Result.(map[string]interface{}); ok {
		if id, ok := result["id"].(string); ok {
			taskID = id
			t.Logf("Created task with ID: %s", taskID)
		}
	}

	// Small delay to ensure task is persisted
	time.Sleep(100 * time.Millisecond)

	// Agent 1 delegates task to Agent 2
	delegateMsg := WebSocketMessage{
		Type:   MessageTypeRequest,
		ID:     uuid.New().String(),
		Method: "task.delegate",
		Params: map[string]interface{}{
			"task_id":  taskID,
			"to_agent": "agent2",
			"reason":   "Agent 2 has authentication expertise",
		},
	}

	err = agent1.SendMessage(delegateMsg)
	require.NoError(t, err)

	// Wait for delegation response
	msg, err = agent1.ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, MessageTypeResponse, msg.Type)
	assert.Nil(t, msg.Error)

	// Agent 2 accepts the task
	acceptMsg := WebSocketMessage{
		Type:   MessageTypeRequest,
		ID:     uuid.New().String(),
		Method: "task.accept",
		Params: map[string]interface{}{
			"task_id": taskID,
		},
	}

	err = agent2.SendMessage(acceptMsg)
	require.NoError(t, err)

	// Wait for acceptance response
	msg, err = agent2.ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, MessageTypeResponse, msg.Type)
	assert.Nil(t, msg.Error)
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
	defer func() {
		_ = coder.Close()
	}()

	tester := NewWebSocketClient(t, "tester", []string{"testing", "qa"})
	defer func() {
		_ = tester.Close()
	}()

	reviewer := NewWebSocketClient(t, "reviewer", []string{"review", "documentation"})
	defer func() {
		_ = reviewer.Close()
	}()

	// Create a collaborative workflow
	// Generate step IDs that we'll use later
	stepID1 := uuid.New().String()
	stepID2 := uuid.New().String()
	stepID3 := uuid.New().String()

	createWorkflowMsg := WebSocketMessage{
		Type:   MessageTypeRequest,
		ID:     uuid.New().String(),
		Method: "workflow.create_collaborative",
		Params: map[string]interface{}{
			"name":        "Feature Development Pipeline",
			"description": "Complete feature development with code, test, and review",
			"steps": []map[string]interface{}{
				{
					"id":    stepID1,
					"name":  "Code Implementation",
					"type":  "task",
					"order": 1,
					"config": map[string]interface{}{
						"required_capability": "coding",
					},
				},
				{
					"id":    stepID2,
					"name":  "Write Tests",
					"type":  "task",
					"order": 2,
					"config": map[string]interface{}{
						"required_capability": "testing",
					},
				},
				{
					"id":    stepID3,
					"name":  "Code Review",
					"type":  "task",
					"order": 3,
					"config": map[string]interface{}{
						"required_capability": "review",
					},
				},
			},
			"agents": []string{
				coder.AgentID,
				tester.AgentID,
				reviewer.AgentID,
			},
			"coordination_mode": "centralized",
			"decision_strategy": "majority",
			"timeout_seconds":   300,
			"max_retries":       3,
		},
	}

	err := coder.SendMessage(createWorkflowMsg)
	require.NoError(t, err)

	// Wait for workflow creation response
	msg, err := coder.ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, MessageTypeResponse, msg.Type)
	assert.Nil(t, msg.Error)

	// Extract workflow ID from response
	var workflowID string
	if result, ok := msg.Result.(map[string]interface{}); ok {
		if wfID, ok := result["workflow_id"].(string); ok {
			workflowID = wfID
		}
	}
	require.NotEmpty(t, workflowID, "Expected workflow_id in response")

	// Execute the collaborative workflow
	executeMsg := WebSocketMessage{
		Type:   MessageTypeRequest,
		ID:     uuid.New().String(),
		Method: "workflow.execute_collaborative",
		Params: map[string]interface{}{
			"workflow_id": workflowID,
			"agents": []string{
				coder.AgentID,
				tester.AgentID,
				reviewer.AgentID,
			},
		},
	}

	err = coder.SendMessage(executeMsg)
	require.NoError(t, err)

	// Wait for execution response
	msg, err = coder.ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, MessageTypeResponse, msg.Type)
	assert.Nil(t, msg.Error)

	// Extract execution ID from response
	var executionID string
	if result, ok := msg.Result.(map[string]interface{}); ok {
		t.Logf("Execute response: %+v", result)
		if execID, ok := result["execution_id"].(string); ok {
			executionID = execID
		}
	}
	require.NotEmpty(t, executionID, "Expected execution_id in response")
	t.Logf("Using execution ID: %s", executionID)

	// Complete tasks as they're assigned to each agent
	stepIDs := []string{stepID1, stepID2, stepID3}
	agents := []*WebSocketClient{coder, tester, reviewer}

	for i, stepID := range stepIDs {
		// Complete the workflow task
		completeMsg := WebSocketMessage{
			Type:   MessageTypeRequest,
			ID:     uuid.New().String(),
			Method: "workflow.complete_task",
			Params: map[string]interface{}{
				"execution_id": executionID,
				"step_id":      stepID,
				"result": map[string]interface{}{
					"status": "success",
					"output": fmt.Sprintf("Step %s completed by %s", stepID, agents[i].AgentID),
				},
			},
		}

		err = agents[i].SendMessage(completeMsg)
		require.NoError(t, err)

		// Wait for completion response
		msg, err = agents[i].ReadMessage(ctx)
		require.NoError(t, err)
		assert.Equal(t, MessageTypeResponse, msg.Type)
		if msg.Error != nil {
			t.Logf("Complete task error for step %s: %+v", stepID, msg.Error)
		}
		assert.Nil(t, msg.Error)
	}

	// Check final workflow status
	statusMsg := WebSocketMessage{
		Type:   MessageTypeRequest,
		ID:     uuid.New().String(),
		Method: "workflow.status",
		Params: map[string]interface{}{
			"execution_id": executionID,
		},
	}

	err = reviewer.SendMessage(statusMsg)
	require.NoError(t, err)

	// Wait for status response
	msg, err = reviewer.ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, MessageTypeResponse, msg.Type)
	assert.Nil(t, msg.Error)
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
		defer func(idx int) {
			_ = agents[idx].Close()
		}(i)
	}

	// First agent creates workspace (creator is automatically added as member)
	createWorkspaceMsg := WebSocketMessage{
		Type:   MessageTypeRequest,
		ID:     uuid.New().String(),
		Method: "workspace.create",
		Params: map[string]interface{}{
			"name":        "Test Collaboration Space",
			"description": "Space for testing multi-agent collaboration",
		},
	}

	err := agents[0].SendMessage(createWorkspaceMsg)
	require.NoError(t, err)

	msg, err := agents[0].ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, MessageTypeResponse, msg.Type)
	assert.Nil(t, msg.Error)

	// Extract workspace ID from response
	var workspaceID string
	if result, ok := msg.Result.(map[string]interface{}); ok {
		if wsID, ok := result["workspace_id"].(string); ok {
			workspaceID = wsID
		}
	}
	require.NotEmpty(t, workspaceID, "Expected workspace ID in response")

	// Join other agents to workspace (agent[0] is already a member as creator)
	for i := 1; i < numAgents; i++ {
		t.Logf("Agent %s joining workspace %s", agents[i].AgentID, workspaceID)
		joinMsg := WebSocketMessage{
			Type:   MessageTypeRequest,
			ID:     uuid.New().String(),
			Method: "workspace.join",
			Params: map[string]interface{}{
				"workspace_id": workspaceID,
			},
		}

		err = agents[i].SendMessage(joinMsg)
		require.NoError(t, err)

		// Wait for join response first
		msg, err = agents[i].ReadMessage(ctx)
		require.NoError(t, err)
		t.Logf("Agent %s received message type=%d method=%s params=%+v", agents[i].AgentID, msg.Type, msg.Method, msg.Params)
		assert.Equal(t, MessageTypeResponse, msg.Type)
		if msg.Error != nil {
			t.Logf("Join error: %+v", msg.Error)
		}
		assert.Nil(t, msg.Error)

		// Then all existing members should receive join notification
		for j := 0; j < i; j++ {
			notifyCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			msg, err = agents[j].ReadMessage(notifyCtx)
			cancel()
			require.NoError(t, err)
			assert.Equal(t, MessageTypeNotification, msg.Type)
			assert.Equal(t, "workspace.member.joined", msg.Method)
		}
	}

	// Test real-time document collaboration
	documentID := uuid.New().String()
	createDocMsg := WebSocketMessage{
		Type:   MessageTypeRequest,
		ID:     uuid.New().String(),
		Method: "document.create_shared",
		Params: map[string]interface{}{
			"workspace": workspaceID,
			"title":     "Collaborative Design Doc",
			"content":   "Initial content",
			"type":      "design",
		},
	}

	err = agents[0].SendMessage(createDocMsg)
	require.NoError(t, err)

	// First, wait for the response from document creation
	createResp, err := agents[0].ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, MessageTypeResponse, createResp.Type)
	assert.Nil(t, createResp.Error)

	// Extract document ID from response
	if result, ok := createResp.Result.(map[string]interface{}); ok {
		if docID, ok := result["document_id"].(string); ok {
			documentID = docID
		}
	}
	require.NotEmpty(t, documentID, "Expected document_id in response")

	// All other agents should receive document creation notification
	var wg sync.WaitGroup
	wg.Add(numAgents - 1) // Exclude the creating agent

	for i := 1; i < numAgents; i++ {
		go func(idx int) {
			defer wg.Done()
			msg, err := agents[idx].ReadMessage(ctx)
			require.NoError(t, err)
			assert.Equal(t, MessageTypeNotification, msg.Type)
			assert.Equal(t, "document.created", msg.Method)
		}(i)
	}

	wg.Wait()

	// Test concurrent edits with CRDT
	edits := make(chan bool, numAgents-1)

	for i := 1; i < numAgents; i++ {
		go func(agentIndex int) {
			editMsg := WebSocketMessage{
				Type:   MessageTypeRequest,
				ID:     uuid.New().String(),
				Method: "document.update",
				Params: map[string]interface{}{
					"document_id": documentID,
					"content":     fmt.Sprintf("Initial content\nEdit from agent%d", agentIndex+1),
					"metadata": map[string]interface{}{
						"edit_position": agentIndex * 10,
						"edit_type":     "insert",
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

	// Give time for all edits to be processed
	time.Sleep(100 * time.Millisecond)
}

// Test conflict resolution with vector clocks
func TestWebSocketConflictResolution(t *testing.T) {
	t.Skip("Skipping test - state management methods (state.create, state.subscribe, state.increment) not yet implemented")
	
	if testing.Short() {
		t.Skip("Skipping functional test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect two agents
	agent1 := NewWebSocketClient(t, "agent1", []string{"editing"})
	defer func() {
		_ = agent1.Close()
	}()

	agent2 := NewWebSocketClient(t, "agent2", []string{"editing"})
	defer func() {
		_ = agent2.Close()
	}()

	// Create shared state
	stateID := uuid.New().String()
	createStateMsg := WebSocketMessage{
		Type:   MessageTypeRequest,
		ID:     uuid.New().String(),
		Method: "state.create",
		Params: map[string]interface{}{
			"state": map[string]interface{}{
				"id":    stateID,
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
	if msg1.Error != nil {
		t.Logf("Agent1 error: %+v", msg1.Error)
	}
	assert.Equal(t, MessageTypeResponse, msg1.Type)

	// Agent2 subscribes to state
	subscribeMsg := WebSocketMessage{
		Type:   MessageTypeRequest,
		ID:     uuid.New().String(),
		Method: "state.subscribe",
		Params: map[string]interface{}{
			"state_id": stateID,
		},
	}

	err = agent2.SendMessage(subscribeMsg)
	require.NoError(t, err)

	msg2, err := agent2.ReadMessage(ctx)
	require.NoError(t, err)
	if msg2.Error != nil {
		t.Logf("Agent2 error: %+v", msg2.Error)
	}
	assert.Equal(t, MessageTypeResponse, msg2.Type)

	// Both agents increment counter concurrently
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			incrementMsg := WebSocketMessage{
				Type:   MessageTypeRequest,
				ID:     uuid.New().String(),
				Method: "state.increment",
				Params: map[string]interface{}{
					"state_id": stateID,
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
				Type:   MessageTypeRequest,
				ID:     uuid.New().String(),
				Method: "state.increment",
				Params: map[string]interface{}{
					"state_id": stateID,
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
		Type:   MessageTypeRequest,
		ID:     uuid.New().String(),
		Method: "state.get",
		Params: map[string]interface{}{
			"state_id": stateID,
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
		if msg.Type == MessageTypeResponse && msg.Method == "state.get" {
			if result, ok := msg.Result.(map[string]interface{}); ok {
				if val, ok := result["value"].(float64); ok {
					finalValue = int(val)
					break
				}
			}
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

	// Connect agents with different capabilities
	agents := []*WebSocketClient{
		NewWebSocketClient(t, "frontend-dev", []string{"javascript", "react", "css"}),
		NewWebSocketClient(t, "backend-dev", []string{"golang", "postgresql", "redis"}),
		NewWebSocketClient(t, "ml-engineer", []string{"python", "tensorflow", "data-analysis"}),
		NewWebSocketClient(t, "devops", []string{"kubernetes", "terraform", "monitoring"}),
	}

	for _, agent := range agents {
		defer func(a *WebSocketClient) {
			_ = a.Close()
		}(agent)
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

	// Wait a bit to ensure all agents are fully registered
	time.Sleep(100 * time.Millisecond)

	// Create and auto-assign tasks
	for _, task := range tasks {
		t.Logf("Creating task %s with capabilities %v", task.title, task.capabilities)
		
		createMsg := WebSocketMessage{
			Type:   MessageTypeRequest,
			ID:     uuid.New().String(),
			Method: "task.create_auto_assign",
			Params: map[string]interface{}{
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
		
		// Read messages until we get the response (not a notification)
		ctx := context.Background()
		var resp *WebSocketMessage
		for {
			msg, err := agents[0].ReadMessage(ctx)
			require.NoError(t, err)
			
			// Debug: Log all messages
			t.Logf("Message received: type=%d, id=%s, method=%s, result=%+v, error=%+v", 
				msg.Type, msg.ID, msg.Method, msg.Result, msg.Error)
			
			// Skip notifications, we want the response
			if msg.Type == MessageTypeResponse {
				resp = msg
				break
			}
		}
		
		require.Nil(t, resp.Error, "Task creation failed: %v", resp.Error)
		
		// Check the response for assignment information
		result, ok := resp.Result.(map[string]interface{})
		require.True(t, ok, "Response result should be a map, got %T", resp.Result)
		
		assignedTo, ok := result["assigned_to"].(string)
		require.True(t, ok, "Response should include assigned_to field")
		
		t.Logf("Task %s assigned to %s", task.title, assignedTo)
		assert.Equal(t, task.expectedAgent, assignedTo, "Task %s assigned to wrong agent", task.title)
		
		// Also verify the task was created with correct ID
		taskID, ok := result["task_id"].(string)
		require.True(t, ok, "Response should include task_id")
		assert.Equal(t, task.id, taskID, "Task ID mismatch")
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
		defer func(idx int) {
			_ = agents[idx].Close()
		}(i)
	}

	// Create shared workspace
	workspaceID := uuid.New().String()
	createWorkspaceMsg := WebSocketMessage{
		Type:   MessageTypeRequest,
		ID:     uuid.New().String(),
		Method: "workspace.create",
		Params: map[string]interface{}{
			"workspace": map[string]interface{}{
				"id":   workspaceID,
				"name": "Performance Test Workspace",
			},
		},
	}

	err := agents[0].SendMessage(createWorkspaceMsg)
	require.NoError(t, err)

	// Wait for workspace creation
	msg, err := agents[0].ReadMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, MessageTypeResponse, msg.Type)

	// All agents join workspace
	var joinWg sync.WaitGroup
	joinWg.Add(numAgents - 1)

	for i := 1; i < numAgents; i++ {
		go func(agentIndex int) {
			defer joinWg.Done()
			joinMsg := WebSocketMessage{
				Type:   MessageTypeRequest,
				ID:     uuid.New().String(),
				Method: "workspace.join",
				Params: map[string]interface{}{
					"workspace_id": workspaceID,
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
				taskID := uuid.New().String()
				createTaskMsg := WebSocketMessage{
					Type:   MessageTypeRequest,
					ID:     uuid.New().String(),
					Method: "task.create",
					Params: map[string]interface{}{
						"task": map[string]interface{}{
							"id":           taskID,
							"title":        fmt.Sprintf("Task %d-%d", agentIndex, j),
							"workspace_id": workspaceID,
						},
					},
				}

				err := agents[agentIndex].SendMessage(createTaskMsg)
				assert.NoError(t, err)

				// Simulate processing
				time.Sleep(time.Duration(10+j%20) * time.Millisecond)

				// Complete task
				completeMsg := WebSocketMessage{
					Type:   MessageTypeRequest,
					ID:     uuid.New().String(),
					Method: "task.complete",
					Params: map[string]interface{}{
						"task_id": taskID,
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

// Helper functions

// getTestConfig returns test configuration
func getTestConfig() struct {
	WebSocketURL string
} {
	wsURL := os.Getenv("MCP_WEBSOCKET_URL")
	if wsURL == "" {
		wsURL = "ws://localhost:8080/ws"
	}
	return struct {
		WebSocketURL string
	}{
		WebSocketURL: wsURL,
	}
}

// getAPIKeyForAgent returns the appropriate API key for an agent
func getAPIKeyForAgent(agentID string) string {
	// Use environment variable if set
	if apiKey := os.Getenv("MCP_API_KEY"); apiKey != "" {
		return apiKey
	}

	// Map agent IDs to their test API keys
	switch agentID {
	case "agent1":
		return "test-key-agent-1"
	case "agent2":
		return "test-key-agent-2"
	case "frontend-dev":
		return "test-key-frontend-dev"
	case "backend-dev":
		return "test-key-backend-dev"
	case "ml-engineer":
		return "test-key-ml-engineer"
	case "devops":
		return "test-key-devops"
	default:
		return "dev-admin-key-1234567890"
	}
}

// establishWebSocketConnection creates an authenticated WebSocket connection
func establishWebSocketConnection(t *testing.T, wsURL, apiKey, agentID string, capabilities []string) (*websocket.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": []string{"Bearer " + apiKey},
		},
	}

	conn, _, err := websocket.Dial(ctx, wsURL, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to dial WebSocket: %w", err)
	}

	// Send initialization message
	initMsg := WebSocketMessage{
		ID:     uuid.New().String(),
		Type:   MessageTypeRequest,
		Method: "initialize",
		Params: map[string]interface{}{
			"name":         agentID,
			"version":      "1.0.0",
			"capabilities": capabilities,
		},
	}
	
	t.Logf("Sending init message for agent %s with capabilities %v", agentID, capabilities)

	if err := wsjson.Write(ctx, conn, initMsg); err != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		return nil, fmt.Errorf("failed to send init message: %w", err)
	}

	// Read initialization response
	var response WebSocketMessage
	if err := wsjson.Read(ctx, conn, &response); err != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		return nil, fmt.Errorf("failed to read init response: %w", err)
	}

	// Check for error in response
	if response.Error != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		return nil, fmt.Errorf("initialization failed: %s", response.Error.Message)
	}

	return conn, nil
}
