package shared

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
)

// StreamingTestClient handles streaming responses from tools
type StreamingTestClient struct {
	conn           *websocket.Conn
	progressChan   chan ProgressUpdate
	completionChan chan interface{}
	errorChan      chan error
}

// ProgressUpdate represents a progress notification
type ProgressUpdate struct {
	Percentage int
	Message    string
	Data       interface{}
}

// NewStreamingTestClient creates a client for testing streaming operations
func NewStreamingTestClient(wsURL, apiKey string) (*StreamingTestClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": []string{"Bearer " + apiKey},
		},
	}

	conn, _, err := websocket.Dial(ctx, wsURL, opts)
	if err != nil {
		return nil, err
	}

	client := &StreamingTestClient{
		conn:           conn,
		progressChan:   make(chan ProgressUpdate, 100),
		completionChan: make(chan interface{}, 1),
		errorChan:      make(chan error, 1),
	}

	// Start message reader
	go client.readMessages()

	return client, nil
}

func (c *StreamingTestClient) readMessages() {
	for {
		var msg ws.Message
		err := wsjson.Read(context.Background(), c.conn, &msg)
		if err != nil {
			c.errorChan <- err
			return
		}

		// Handle different message types
		if msg.Type == ws.MessageTypeProgress {
			if progress, ok := msg.Result.(map[string]interface{}); ok {
				update := ProgressUpdate{
					Percentage: int(progress["percentage"].(float64)),
					Message:    progress["message"].(string),
					Data:       progress["data"],
				}
				c.progressChan <- update
			}
		} else if msg.Type == ws.MessageTypeResponse {
			c.completionChan <- msg.Result
		} else if msg.Type == ws.MessageTypeError {
			c.errorChan <- fmt.Errorf("error response: %v", msg.Error)
		}
	}
}

func (c *StreamingTestClient) Close() error {
	return c.conn.Close(websocket.StatusNormalClosure, "")
}

// ContextWindowManager simulates token counting and window management
type ContextWindowManager struct {
	maxTokens           int
	currentTokens       int
	messages            []ContextMessage
	importanceThreshold int
	mu                  sync.RWMutex
}

// ContextMessage represents a message with token count and importance
type ContextMessage struct {
	ID         string
	Content    string
	Tokens     int
	Importance int // 0-100, higher is more important
	Timestamp  time.Time
	Role       string // system, user, assistant
}

// NewContextWindowManager creates a manager for testing context windows
func NewContextWindowManager(maxTokens int) *ContextWindowManager {
	return &ContextWindowManager{
		maxTokens:           maxTokens,
		importanceThreshold: 50,
		messages:            make([]ContextMessage, 0),
	}
}

func (m *ContextWindowManager) AddMessage(msg ContextMessage) (truncated bool, removedCount int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Simulate token counting (rough estimate: 1 token per 4 chars)
	msg.Tokens = len(msg.Content) / 4
	m.messages = append(m.messages, msg)
	m.currentTokens += msg.Tokens

	// Truncate if over limit
	if m.currentTokens > m.maxTokens {
		truncated = true
		removedCount = m.truncateMessages()
	}

	return truncated, removedCount
}

func (m *ContextWindowManager) truncateMessages() int {
	removed := 0

	// Always preserve system messages
	// Remove low-importance messages first
	for m.currentTokens > m.maxTokens && len(m.messages) > 1 {
		lowestIdx := -1
		lowestImportance := 101

		for i, msg := range m.messages {
			if msg.Role != "system" && msg.Importance < lowestImportance {
				lowestImportance = msg.Importance
				lowestIdx = i
			}
		}

		if lowestIdx >= 0 {
			m.currentTokens -= m.messages[lowestIdx].Tokens
			m.messages = append(m.messages[:lowestIdx], m.messages[lowestIdx+1:]...)
			removed++
		} else {
			break // No more messages to remove
		}
	}

	return removed
}

// WorkflowOrchestrator manages multi-step tool execution
type WorkflowOrchestrator struct {
	conn  *websocket.Conn
	steps []WorkflowStep
	state map[string]interface{}
	mu    sync.Mutex
}

// WorkflowStep represents a step in a workflow
type WorkflowStep struct {
	ID        string
	ToolName  string
	Arguments map[string]interface{}
	DependsOn []string // IDs of steps that must complete first
	Condition func(state map[string]interface{}) bool
	OnSuccess func(result interface{}, state map[string]interface{})
	OnError   func(err error, state map[string]interface{})
	Parallel  bool // Can run in parallel with other parallel steps
}

// NewWorkflowOrchestrator creates an orchestrator for testing workflows
func NewWorkflowOrchestrator(conn *websocket.Conn) *WorkflowOrchestrator {
	return &WorkflowOrchestrator{
		conn:  conn,
		steps: make([]WorkflowStep, 0),
		state: make(map[string]interface{}),
	}
}

func (o *WorkflowOrchestrator) AddStep(step WorkflowStep) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.steps = append(o.steps, step)
}

func (o *WorkflowOrchestrator) Execute(ctx context.Context) error {
	completed := make(map[string]bool)
	results := make(map[string]interface{})

	for len(completed) < len(o.steps) {
		// Find steps that can be executed
		readySteps := o.findReadySteps(completed)

		if len(readySteps) == 0 {
			return fmt.Errorf("no steps ready to execute, possible circular dependency")
		}

		// Execute ready steps
		var wg sync.WaitGroup
		errors := make(chan error, len(readySteps))

		for _, step := range readySteps {
			if step.Condition != nil && !step.Condition(o.state) {
				completed[step.ID] = true
				continue
			}

			wg.Add(1)
			go func(s WorkflowStep) {
				defer wg.Done()

				// Prepare arguments with state substitution
				args := o.substituteArguments(s.Arguments, results)

				// Execute tool
				msg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "tool.execute",
					Params: map[string]interface{}{
						"name":      s.ToolName,
						"arguments": args,
					},
				}

				if err := wsjson.Write(ctx, o.conn, msg); err != nil {
					errors <- err
					return
				}

				var response ws.Message
				if err := wsjson.Read(ctx, o.conn, &response); err != nil {
					errors <- err
					return
				}

				if response.Error != nil {
					if s.OnError != nil {
						s.OnError(fmt.Errorf("%s", response.Error.Message), o.state)
					}
					errors <- fmt.Errorf("step %s failed: %s", s.ID, response.Error.Message)
				} else {
					results[s.ID] = response.Result
					if s.OnSuccess != nil {
						s.OnSuccess(response.Result, o.state)
					}
				}

				completed[s.ID] = true
			}(step)

			// If not parallel, wait for completion
			if !step.Parallel {
				wg.Wait()
			}
		}

		wg.Wait()

		// Check for errors
		select {
		case err := <-errors:
			return err
		default:
		}
	}

	return nil
}

func (o *WorkflowOrchestrator) findReadySteps(completed map[string]bool) []WorkflowStep {
	ready := make([]WorkflowStep, 0)

	for _, step := range o.steps {
		if completed[step.ID] {
			continue
		}

		// Check dependencies
		allDepsComplete := true
		for _, dep := range step.DependsOn {
			if !completed[dep] {
				allDepsComplete = false
				break
			}
		}

		if allDepsComplete {
			ready = append(ready, step)
		}
	}

	return ready
}

func (o *WorkflowOrchestrator) substituteArguments(args map[string]interface{}, results map[string]interface{}) map[string]interface{} {
	substituted := make(map[string]interface{})

	for k, v := range args {
		if str, ok := v.(string); ok && len(str) > 2 && str[0] == '$' {
			// Variable reference like $step1.output
			varPath := str[1:]
			if val, found := o.resolveVariable(varPath, results); found {
				substituted[k] = val
			} else {
				substituted[k] = v
			}
		} else {
			substituted[k] = v
		}
	}

	return substituted
}

func (o *WorkflowOrchestrator) resolveVariable(path string, results map[string]interface{}) (interface{}, bool) {
	// Simple implementation - can be extended for nested paths
	if result, ok := results[path]; ok {
		return result, true
	}
	return nil, false
}

// SubscriptionManager handles real-time subscriptions
type SubscriptionManager struct {
	conn          *websocket.Conn
	subscriptions map[string]chan interface{}
	mu            sync.RWMutex
}

// NewSubscriptionManager creates a manager for testing subscriptions
func NewSubscriptionManager(conn *websocket.Conn) *SubscriptionManager {
	sm := &SubscriptionManager{
		conn:          conn,
		subscriptions: make(map[string]chan interface{}),
	}

	// Start event reader
	go sm.readEvents()

	return sm
}

func (sm *SubscriptionManager) Subscribe(ctx context.Context, resource string, filter map[string]interface{}) (chan interface{}, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	subID := uuid.New().String()
	eventChan := make(chan interface{}, 100)
	sm.subscriptions[subID] = eventChan

	// Send subscription request
	msg := ws.Message{
		ID:     uuid.New().String(),
		Type:   ws.MessageTypeRequest,
		Method: "subscribe",
		Params: map[string]interface{}{
			"subscription_id": subID,
			"resource":        resource,
			"filter":          filter,
		},
	}

	if err := wsjson.Write(ctx, sm.conn, msg); err != nil {
		delete(sm.subscriptions, subID)
		return nil, err
	}

	// Wait for confirmation
	var response ws.Message
	if err := wsjson.Read(ctx, sm.conn, &response); err != nil {
		delete(sm.subscriptions, subID)
		return nil, err
	}

	if response.Error != nil {
		delete(sm.subscriptions, subID)
		return nil, fmt.Errorf("subscription failed: %s", response.Error.Message)
	}

	return eventChan, nil
}

func (sm *SubscriptionManager) Unsubscribe(ctx context.Context, eventChan chan interface{}) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Find subscription ID
	var subID string
	for id, ch := range sm.subscriptions {
		if ch == eventChan {
			subID = id
			break
		}
	}

	if subID == "" {
		return fmt.Errorf("subscription not found")
	}

	// Send unsubscribe request
	msg := ws.Message{
		ID:     uuid.New().String(),
		Type:   ws.MessageTypeRequest,
		Method: "unsubscribe",
		Params: map[string]interface{}{
			"subscription_id": subID,
		},
	}

	if err := wsjson.Write(ctx, sm.conn, msg); err != nil {
		return err
	}

	delete(sm.subscriptions, subID)
	close(eventChan)

	return nil
}

func (sm *SubscriptionManager) readEvents() {
	for {
		var msg ws.Message
		err := wsjson.Read(context.Background(), sm.conn, &msg)
		if err != nil {
			return
		}

		if msg.Type == ws.MessageTypeNotification && msg.Method == "event" {
			if params, ok := msg.Params.(map[string]interface{}); ok {
				if subID, ok := params["subscription_id"].(string); ok {
					sm.mu.RLock()
					if eventChan, found := sm.subscriptions[subID]; found {
						select {
						case eventChan <- params["data"]:
						default:
							// Channel full, drop event
						}
					}
					sm.mu.RUnlock()
				}
			}
		}
	}
}

// SessionStore manages conversation state persistence
type SessionStore struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// Session represents a conversation session
type Session struct {
	ID           string
	AgentID      string
	Context      []ContextMessage
	State        map[string]interface{}
	LastActivity time.Time
}

// NewSessionStore creates a store for testing session management
func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*Session),
	}
}

func (s *SessionStore) CreateSession(agentID string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := &Session{
		ID:           uuid.New().String(),
		AgentID:      agentID,
		Context:      make([]ContextMessage, 0),
		State:        make(map[string]interface{}),
		LastActivity: time.Now(),
	}

	s.sessions[session.ID] = session
	return session
}

func (s *SessionStore) GetSession(id string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, found := s.sessions[id]
	return session, found
}

func (s *SessionStore) SaveSession(session *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session.LastActivity = time.Now()
	s.sessions[session.ID] = session
}

func (s *SessionStore) CleanupInactive(timeout time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0
	cutoff := time.Now().Add(-timeout)

	for id, session := range s.sessions {
		if session.LastActivity.Before(cutoff) {
			delete(s.sessions, id)
			removed++
		}
	}

	return removed
}

// BinaryProtocolCodec handles binary message encoding/decoding
type BinaryProtocolCodec struct {
	compressionThreshold int
}

// NewBinaryProtocolCodec creates a codec for binary protocol testing
func NewBinaryProtocolCodec() *BinaryProtocolCodec {
	return &BinaryProtocolCodec{
		compressionThreshold: 1024, // Compress messages over 1KB
	}
}

func (c *BinaryProtocolCodec) Encode(msg interface{}) ([]byte, error) {
	// Convert to JSON first
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	// TODO: Implement actual binary encoding
	// For now, return JSON (actual implementation would use protobuf or similar)
	return jsonData, nil
}

func (c *BinaryProtocolCodec) Decode(data []byte) (interface{}, error) {
	var msg interface{}
	err := json.Unmarshal(data, &msg)
	return msg, err
}

// MultiAgentCoordinator manages multiple agents
type MultiAgentCoordinator struct {
	agents map[string]*AgentConnection
	mu     sync.RWMutex
}

// AgentConnection represents a connected agent
type AgentConnection struct {
	ID           string
	Name         string
	Capabilities []string
	Conn         *websocket.Conn
	SharedState  map[string]interface{}
}

// NewMultiAgentCoordinator creates a coordinator for multi-agent testing
func NewMultiAgentCoordinator() *MultiAgentCoordinator {
	return &MultiAgentCoordinator{
		agents: make(map[string]*AgentConnection),
	}
}

func (c *MultiAgentCoordinator) RegisterAgent(agent *AgentConnection) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.agents[agent.ID] = agent
}

func (c *MultiAgentCoordinator) UnregisterAgent(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.agents, agentID)
}

func (c *MultiAgentCoordinator) SendToAgent(fromID, toID string, message interface{}) error {
	c.mu.RLock()
	toAgent, found := c.agents[toID]
	c.mu.RUnlock()

	if !found {
		return fmt.Errorf("agent %s not found", toID)
	}

	msg := ws.Message{
		ID:     uuid.New().String(),
		Type:   ws.MessageTypeNotification,
		Method: "agent.message",
		Params: map[string]interface{}{
			"from":    fromID,
			"message": message,
		},
	}

	return wsjson.Write(context.Background(), toAgent.Conn, msg)
}

func (c *MultiAgentCoordinator) BroadcastToAgents(fromID string, message interface{}, capability string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for id, agent := range c.agents {
		if id == fromID {
			continue
		}

		// Check capability filter
		if capability != "" {
			hasCapability := false
			for _, cap := range agent.Capabilities {
				if cap == capability {
					hasCapability = true
					break
				}
			}
			if !hasCapability {
				continue
			}
		}

		// Send message
		if err := c.SendToAgent(fromID, id, message); err != nil {
			return err
		}
	}

	return nil
}

// Helper functions for common test operations

// EstablishConnection creates an authenticated WebSocket connection
func EstablishConnection(wsURL, apiKey string) (*websocket.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": []string{"Bearer " + apiKey},
		},
	}

	conn, _, err := websocket.Dial(ctx, wsURL, opts)
	if err != nil {
		return nil, err
	}

	// Initialize connection
	initMsg := ws.Message{
		ID:     uuid.New().String(),
		Type:   ws.MessageTypeRequest,
		Method: "initialize",
		Params: map[string]interface{}{
			"name":    "test-agent",
			"version": "1.0.0",
		},
	}

	if err := wsjson.Write(ctx, conn, initMsg); err != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		return nil, err
	}

	var response ws.Message
	if err := wsjson.Read(ctx, conn, &response); err != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		return nil, err
	}

	if response.Error != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		return nil, fmt.Errorf("initialization failed: %s", response.Error.Message)
	}

	return conn, nil
}

// SimulateTokenCount estimates token count for a string
func SimulateTokenCount(text string) int {
	// Rough estimate: 1 token per 4 characters
	return len(text) / 4
}

// GenerateLargeContext creates a context of specified token size
func GenerateLargeContext(tokens int) string {
	// Generate approximately the right amount of text
	chars := tokens * 4
	result := ""
	template := "This is a sample message for testing context windows. "

	for len(result) < chars {
		result += template
	}

	return result[:chars]
}
