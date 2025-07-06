package agent

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"

	ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

// TestAgent represents a test AI agent for E2E testing
type TestAgent struct {
	conn         *websocket.Conn
	agentID      string
	name         string
	capabilities []string
	apiKey       string
	baseURL      string
	sessionID    string

	// Connection state
	connected bool
	mu        sync.RWMutex

	// Message handling
	responses chan *ws.Message
	errors    chan error
	stopCh    chan struct{}

	// Metrics
	messagesSent     int64
	messagesReceived int64
	lastActivity     time.Time
}

// NewTestAgent creates a new test agent instance
func NewTestAgent(name string, capabilities []string, apiKey, baseURL string) *TestAgent {
	return &TestAgent{
		agentID:      uuid.New().String(),
		name:         name,
		capabilities: capabilities,
		apiKey:       apiKey,
		baseURL:      baseURL,
		responses:    make(chan *ws.Message, 100),
		errors:       make(chan error, 10),
		stopCh:       make(chan struct{}),
	}
}

// Connect establishes WebSocket connection to the MCP server
func (ta *TestAgent) Connect(ctx context.Context) error {
	// Parse base URL and construct WebSocket URL
	baseURL := ta.baseURL
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}
	// Convert https to wss, http to ws
	wsURL := strings.Replace(baseURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	wsURL = strings.TrimRight(wsURL, "/") + "/ws"

	opts := &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": []string{"Bearer " + ta.apiKey},
			"X-Agent-ID":    []string{ta.agentID},
			"X-Agent-Name":  []string{ta.name},
		},
		Subprotocols: []string{"mcp.v1"},
	}

	conn, _, err := websocket.Dial(ctx, wsURL, opts)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	ta.mu.Lock()
	ta.conn = conn
	ta.connected = true
	ta.lastActivity = time.Now()
	ta.mu.Unlock()

	// Start message reader
	go ta.readMessages()

	// Initialize connection
	if err := ta.initialize(ctx); err != nil {
		_ = ta.Close()
		return fmt.Errorf("initialization failed: %w", err)
	}

	return nil
}

// initialize performs MCP protocol initialization
func (ta *TestAgent) initialize(ctx context.Context) error {
	initMsg := ws.Message{
		ID:     uuid.New().String(),
		Type:   ws.MessageTypeRequest,
		Method: "initialize",
		Params: map[string]interface{}{
			"name":         ta.name,
			"agentId":      ta.agentID,
			"version":      "1.0.0",
			"capabilities": ta.capabilities,
			"metadata": map[string]interface{}{
				"type":        "test-agent",
				"environment": "e2e-test",
			},
		},
	}

	response, err := ta.sendAndWait(ctx, &initMsg, 10*time.Second)
	if err != nil {
		return err
	}

	if response.Error != nil {
		return fmt.Errorf("initialization error: %s", response.Error.Message)
	}

	// Extract session ID if provided
	if result, ok := response.Result.(map[string]interface{}); ok {
		if sessionID, ok := result["sessionId"].(string); ok {
			ta.sessionID = sessionID
		}
	}

	return nil
}

// readMessages continuously reads messages from the WebSocket
func (ta *TestAgent) readMessages() {
	defer close(ta.responses)
	defer close(ta.errors)

	for {
		select {
		case <-ta.stopCh:
			return
		default:
			var msg ws.Message
			err := wsjson.Read(context.Background(), ta.conn, &msg)
			if err != nil {
				if websocket.CloseStatus(err) != -1 {
					ta.mu.Lock()
					ta.connected = false
					ta.mu.Unlock()
					return
				}
				select {
				case ta.errors <- err:
				case <-ta.stopCh:
					return
				}
				continue
			}

			ta.mu.Lock()
			ta.messagesReceived++
			ta.lastActivity = time.Now()
			ta.mu.Unlock()

			select {
			case ta.responses <- &msg:
			case <-ta.stopCh:
				return
			}
		}
	}
}

// sendAndWait sends a message and waits for response
func (ta *TestAgent) sendAndWait(ctx context.Context, msg *ws.Message, timeout time.Duration) (*ws.Message, error) {
	if err := ta.SendMessage(ctx, msg); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case response := <-ta.responses:
			if response.ID == msg.ID {
				return response, nil
			}
			// Put it back if not our response
			select {
			case ta.responses <- response:
			default:
			}
		case err := <-ta.errors:
			return nil, err
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for response to message %s", msg.ID)
		}
	}
}

// SendMessage sends a message through the WebSocket
func (ta *TestAgent) SendMessage(ctx context.Context, msg *ws.Message) error {
	ta.mu.RLock()
	if !ta.connected {
		ta.mu.RUnlock()
		return fmt.Errorf("not connected")
	}
	ta.mu.RUnlock()

	if err := wsjson.Write(ctx, ta.conn, msg); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	ta.mu.Lock()
	ta.messagesSent++
	ta.lastActivity = time.Now()
	ta.mu.Unlock()

	return nil
}

// ExecuteMethod executes a method and returns the response
func (ta *TestAgent) ExecuteMethod(ctx context.Context, method string, params interface{}) (*ws.Message, error) {
	msg := ws.Message{
		ID:     uuid.New().String(),
		Type:   ws.MessageTypeRequest,
		Method: method,
		Params: params,
	}

	return ta.sendAndWait(ctx, &msg, 30*time.Second)
}

// RegisterCapabilities registers agent capabilities with the server
func (ta *TestAgent) RegisterCapabilities(ctx context.Context) error {
	resp, err := ta.ExecuteMethod(ctx, "agent.register", map[string]interface{}{
		"capabilities": ta.capabilities,
		"status":       "available",
		"metadata": map[string]interface{}{
			"maxConcurrentTasks": 5,
			"preferredTaskTypes": ta.capabilities,
		},
	})

	if err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("registration error: %s", resp.Error.Message)
	}

	return nil
}

// Heartbeat sends a heartbeat/ping message
func (ta *TestAgent) Heartbeat(ctx context.Context) error {
	resp, err := ta.ExecuteMethod(ctx, "ping", nil)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("heartbeat error: %s", resp.Error.Message)
	}

	return nil
}

// AcceptTask accepts an assigned task
func (ta *TestAgent) AcceptTask(ctx context.Context, taskID string) error {
	resp, err := ta.ExecuteMethod(ctx, "task.accept", map[string]interface{}{
		"taskId": taskID,
	})

	if err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("accept task error: %s", resp.Error.Message)
	}

	return nil
}

// CompleteTask marks a task as completed
func (ta *TestAgent) CompleteTask(ctx context.Context, taskID string, result interface{}) error {
	resp, err := ta.ExecuteMethod(ctx, "task.complete", map[string]interface{}{
		"taskId": taskID,
		"result": result,
		"status": "completed",
	})

	if err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("complete task error: %s", resp.Error.Message)
	}

	return nil
}

// WaitForTask waits for a task assignment
func (ta *TestAgent) WaitForTask(ctx context.Context, timeout time.Duration) (*ws.Message, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case msg := <-ta.responses:
			if msg.Type == ws.MessageTypeNotification && msg.Method == "task.assigned" {
				return msg, nil
			}
			// Put back non-task messages
			select {
			case ta.responses <- msg:
			default:
			}
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for task assignment")
		}
	}
}

// Close gracefully closes the connection
func (ta *TestAgent) Close() error {
	ta.mu.Lock()
	defer ta.mu.Unlock()

	if !ta.connected {
		return nil
	}

	close(ta.stopCh)
	ta.connected = false

	if ta.conn != nil {
		// Send disconnect message
		disconnectMsg := ws.Message{
			ID:     uuid.New().String(),
			Type:   ws.MessageTypeRequest,
			Method: "disconnect",
			Params: map[string]interface{}{
				"reason": "test completed",
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = wsjson.Write(ctx, ta.conn, disconnectMsg)

		return ta.conn.Close(websocket.StatusNormalClosure, "Test completed")
	}

	return nil
}

// IsConnected returns true if the agent is connected
func (ta *TestAgent) IsConnected() bool {
	ta.mu.RLock()
	defer ta.mu.RUnlock()
	return ta.connected
}

// GetMetrics returns agent metrics
func (ta *TestAgent) GetMetrics() (sent int64, received int64, lastActivity time.Time) {
	ta.mu.RLock()
	defer ta.mu.RUnlock()
	return ta.messagesSent, ta.messagesReceived, ta.lastActivity
}

// GetID returns the agent ID
func (ta *TestAgent) GetID() string {
	return ta.agentID
}

// GetSessionID returns the session ID
func (ta *TestAgent) GetSessionID() string {
	return ta.sessionID
}
