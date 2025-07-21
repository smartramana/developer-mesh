package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"

	ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

// Binary protocol constants (must match server implementation)
const (
	BinaryProtocolVersion = 1
	HeaderSize            = 12 // version(1) + flags(1) + messageType(2) + payloadSize(4) + reserved(4)
	FlagCompressed        = 1 << 0
	FlagEncrypted         = 1 << 1
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

	// Binary protocol
	binaryMode           bool
	compressionThreshold int
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
	ta.mu.Lock()
	// If stopCh was closed from a previous connection, create a new one
	select {
	case <-ta.stopCh:
		ta.stopCh = make(chan struct{})
	default:
		// stopCh is still open, good to use
	}
	ta.mu.Unlock()

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
		// Try both session_id (snake_case) and sessionId (camelCase) for compatibility
		if sessionID, ok := result["session_id"].(string); ok {
			ta.sessionID = sessionID
		} else if sessionID, ok := result["sessionId"].(string); ok {
			ta.sessionID = sessionID
		}
	}

	return nil
}

// readMessages continuously reads messages from the WebSocket
func (ta *TestAgent) readMessages() {
	// Don't close channels here - they may be reused for reconnection
	// Channels will be garbage collected when TestAgent is no longer referenced

	for {
		select {
		case <-ta.stopCh:
			return
		default:
			var msg ws.Message
			var err error

			// Check if we're in binary mode
			ta.mu.RLock()
			binaryMode := ta.binaryMode
			ta.mu.RUnlock()

			if binaryMode {
				// Read binary message
				msgType, data, readErr := ta.conn.Read(context.Background())
				if readErr != nil {
					err = readErr
				} else if msgType == websocket.MessageBinary {
					// Decode binary message
					decodedMsg, decodeErr := ta.decodeBinaryMessage(data)
					if decodeErr != nil {
						err = decodeErr
					} else {
						msg = *decodedMsg
					}
				} else {
					// In binary mode but received non-binary message
					err = fmt.Errorf("expected binary message, got type %v", msgType)
				}
			} else {
				// Standard JSON mode
				err = wsjson.Read(context.Background(), ta.conn, &msg)
			}

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
			return nil, ctx.Err() // Return the actual context error (DeadlineExceeded or Canceled)
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
	binaryMode := ta.binaryMode
	compressionThreshold := ta.compressionThreshold
	ta.mu.RUnlock()

	// If binary mode is enabled, encode as binary
	if binaryMode {
		data, err := ta.encodeBinaryMessage(msg, compressionThreshold)
		if err != nil {
			return fmt.Errorf("failed to encode binary message: %w", err)
		}
		if err := ta.conn.Write(ctx, websocket.MessageBinary, data); err != nil {
			return fmt.Errorf("failed to send binary message: %w", err)
		}
	} else {
		// Standard JSON mode
		if err := wsjson.Write(ctx, ta.conn, msg); err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}
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
		"name":         ta.name,
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

	// Signal the readMessages goroutine to stop
	// Don't close stopCh here to allow for reconnection
	select {
	case <-ta.stopCh:
		// Already closed
	default:
		close(ta.stopCh)
	}

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

		err := ta.conn.Close(websocket.StatusNormalClosure, "Test completed")
		ta.conn = nil // Clear the connection reference
		return err
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

// SetBinaryMode enables or disables binary protocol
func (ta *TestAgent) SetBinaryMode(enabled bool, compressionThreshold int) {
	ta.mu.Lock()
	defer ta.mu.Unlock()
	ta.binaryMode = enabled
	ta.compressionThreshold = compressionThreshold
}

// encodeBinaryMessage encodes a message to binary format
func (ta *TestAgent) encodeBinaryMessage(msg *ws.Message, compressionThreshold int) ([]byte, error) {
	// Marshal message to JSON
	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Check if compression is needed
	var flags byte
	if len(payload) > compressionThreshold && compressionThreshold > 0 {
		compressed, err := compressPayload(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to compress payload: %w", err)
		}
		payload = compressed
		flags |= FlagCompressed
	}

	// Create header
	header := make([]byte, HeaderSize)
	header[0] = BinaryProtocolVersion // version
	header[1] = flags                 // flags

	// Message type (2 bytes)
	msgType := getMessageTypeCode(msg.Type)
	binary.BigEndian.PutUint16(header[2:4], msgType)

	// Payload size (4 bytes)
	binary.BigEndian.PutUint32(header[4:8], uint32(len(payload)))

	// Reserved (4 bytes) - leave as zeros

	// Combine header and payload
	return append(header, payload...), nil
}

// decodeBinaryMessage decodes a binary message
func (ta *TestAgent) decodeBinaryMessage(data []byte) (*ws.Message, error) {
	if len(data) < HeaderSize {
		return nil, fmt.Errorf("message too short: %d bytes", len(data))
	}

	// Parse header
	version := data[0]
	if version != BinaryProtocolVersion {
		return nil, fmt.Errorf("unsupported protocol version: %d", version)
	}

	flags := data[1]
	// msgType := binary.BigEndian.Uint16(data[2:4]) // Not used in decoding
	payloadSize := binary.BigEndian.Uint32(data[4:8])

	// Validate payload size
	if len(data) < HeaderSize+int(payloadSize) {
		return nil, fmt.Errorf("incomplete payload: expected %d bytes, got %d", payloadSize, len(data)-HeaderSize)
	}

	payload := data[HeaderSize : HeaderSize+payloadSize]

	// Decompress if needed
	if flags&FlagCompressed != 0 {
		decompressed, err := decompressPayload(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress payload: %w", err)
		}
		payload = decompressed
	}

	// Unmarshal JSON
	var msg ws.Message
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return &msg, nil
}

// Helper functions for binary protocol

func compressPayload(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decompressPayload(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	return io.ReadAll(gz)
}

func getMessageTypeCode(msgType ws.MessageType) uint16 {
	switch msgType {
	case ws.MessageTypeRequest:
		return 1
	case ws.MessageTypeResponse:
		return 2
	case ws.MessageTypeNotification:
		return 3
	case ws.MessageTypeError:
		return 4
	default:
		return 0
	}
}
