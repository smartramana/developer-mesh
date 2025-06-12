package websocket

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/S-Corkum/devops-mcp/pkg/models"
    ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

// MessageHandler processes a specific message type
type MessageHandler func(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error)

// RegisterHandlers sets up all message handlers
func (s *Server) RegisterHandlers() {
    s.handlers = map[string]MessageHandler{
        "initialize":      s.handleInitialize,
        "tool.list":       s.handleToolList,
        "tool.execute":    s.handleToolExecute,
        "context.create":  s.handleContextCreate,
        "context.get":     s.handleContextGet,
        "context.update":  s.handleContextUpdate,
        "event.subscribe": s.handleEventSubscribe,
    }
}

// Add handlers field to Server struct (this would be added to server.go)
// TODO: Uncomment when implementing message handlers
// type ServerWithHandlers struct {
//     *Server
//     handlers map[string]MessageHandler
//     
//     // Dependencies (these will be set in Task 7)
//     toolRegistry   ToolRegistry
//     contextManager ContextManager
//     eventBus       EventBus
// }

// processMessage handles incoming WebSocket messages
func (s *Server) processMessage(ctx context.Context, conn *Connection, msg *ws.Message) ([]byte, error) {
    // Validate message
    if msg.Type != ws.MessageTypeRequest {
        return nil, fmt.Errorf("invalid message type: %d", msg.Type)
    }
    
    // Get handler
    handler, ok := s.handlers[msg.Method]
    if !ok {
        return s.createErrorResponse(msg.ID, ws.ErrCodeMethodNotFound, "Method not found")
    }
    
    // Convert params to json.RawMessage if needed
    var params json.RawMessage
    if msg.Params != nil {
        paramBytes, err := json.Marshal(msg.Params)
        if err != nil {
            return s.createErrorResponse(msg.ID, ws.ErrCodeInvalidParams, "Invalid parameters")
        }
        params = paramBytes
    }
    
    // Execute handler
    result, err := handler(ctx, conn, params)
    if err != nil {
        return s.createErrorResponse(msg.ID, ws.ErrCodeServerError, err.Error())
    }
    
    // Create response using pooled object
    response := GetMessage()
    defer PutMessage(response)
    
    response.ID = msg.ID
    response.Type = ws.MessageTypeResponse
    response.Result = result
    
    return json.Marshal(response)
}

// createErrorResponse creates an error response message
func (s *Server) createErrorResponse(id string, code int, message string) ([]byte, error) {
    response := GetMessage()
    defer PutMessage(response)
    
    response.ID = id
    response.Type = ws.MessageTypeError
    response.Error = &ws.Error{
        Code:    code,
        Message: message,
    }
    
    return json.Marshal(response)
}

// handleInitialize handles the initialize method
func (s *Server) handleInitialize(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
    var initParams struct {
        Version string `json:"version"`
        Name    string `json:"name"`
    }
    
    if err := json.Unmarshal(params, &initParams); err != nil {
        return nil, err
    }
    
    // Return server capabilities
    return map[string]interface{}{
        "version": "1.0.0",
        "capabilities": map[string]interface{}{
            "tools":   true,
            "context": true,
            "events":  true,
            "binary":  false, // Will be true in Task 5
        },
    }, nil
}

// handleToolList handles the tool.list method
func (s *Server) handleToolList(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
    // This will be implemented properly in Task 7
    // For now, return a mock response
    return map[string]interface{}{
        "tools": []map[string]interface{}{
            {
                "id":          "github.list_repos",
                "name":        "List GitHub Repositories",
                "description": "Lists repositories for a GitHub organization",
                "parameters": map[string]interface{}{
                    "org": map[string]string{
                        "type":        "string",
                        "description": "GitHub organization name",
                        "required":    "true",
                    },
                },
            },
        },
    }, nil
}

// handleToolExecute handles the tool.execute method
func (s *Server) handleToolExecute(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
    var execParams struct {
        Tool string                 `json:"tool"`
        Args map[string]interface{} `json:"args"`
    }
    
    if err := json.Unmarshal(params, &execParams); err != nil {
        return nil, err
    }
    
    // This will be implemented properly in Task 7
    // For now, return a mock response
    return map[string]interface{}{
        "tool":   execParams.Tool,
        "status": "completed",
        "result": map[string]interface{}{
            "message": "Tool executed successfully",
        },
    }, nil
}

// handleContextCreate handles the context.create method
func (s *Server) handleContextCreate(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
    var createParams struct {
        Name    string `json:"name"`
        Content string `json:"content"`
    }
    
    if err := json.Unmarshal(params, &createParams); err != nil {
        return nil, err
    }
    
    // This will be implemented properly in Task 7
    // For now, return a mock response with a generated ID
    contextID := fmt.Sprintf("ctx_%d", time.Now().UnixNano())
    
    return map[string]interface{}{
        "id":         contextID,
        "name":       createParams.Name,
        "content":    createParams.Content,
        "agent_id":   conn.AgentID,
        "tenant_id":  conn.TenantID,
        "created_at": time.Now().Format(time.RFC3339),
        "updated_at": time.Now().Format(time.RFC3339),
    }, nil
}

// handleContextGet handles the context.get method
func (s *Server) handleContextGet(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
    var getParams struct {
        ContextID string `json:"context_id"`
    }
    
    if err := json.Unmarshal(params, &getParams); err != nil {
        return nil, err
    }
    
    // This will be implemented properly in Task 7
    // For now, return a mock response
    return map[string]interface{}{
        "id":         getParams.ContextID,
        "agent_id":   conn.AgentID,
        "content":    "Mock context content",
        "created_at": "2024-01-01T00:00:00Z",
        "updated_at": "2024-01-01T00:00:00Z",
    }, nil
}

// handleContextUpdate handles the context.update method
func (s *Server) handleContextUpdate(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
    var updateParams struct {
        ContextID string `json:"context_id"`
        Content   string `json:"content"`
    }
    
    if err := json.Unmarshal(params, &updateParams); err != nil {
        return nil, err
    }
    
    // This will be implemented properly in Task 7
    // For now, return a mock response
    return map[string]interface{}{
        "id":         updateParams.ContextID,
        "updated_at": "2024-01-01T00:00:00Z",
    }, nil
}

// handleEventSubscribe handles the event.subscribe method
func (s *Server) handleEventSubscribe(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
    var subParams struct {
        Events []string `json:"events"`
    }
    
    if err := json.Unmarshal(params, &subParams); err != nil {
        return nil, err
    }
    
    // This will be implemented properly in Task 7
    // For now, just acknowledge the subscription
    return map[string]interface{}{
        "subscribed": subParams.Events,
        "status":     "active",
    }, nil
}

// Tool represents a tool definition (will be replaced in Task 7)
type Tool struct {
    ID          string                 `json:"id"`
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Parameters  map[string]interface{} `json:"parameters"`
}

// Interfaces that will be implemented in Task 7
type ToolRegistry interface {
    GetToolsForAgent(agentID string) ([]Tool, error)
    ExecuteTool(ctx context.Context, agentID, toolID string, args map[string]interface{}) (interface{}, error)
}

type ContextManager interface {
    GetContext(ctx context.Context, contextID string) (*models.Context, error)
    UpdateContext(ctx context.Context, contextID string, content string) (*models.Context, error)
}

type EventBus interface {
    Subscribe(connectionID string, events []string) error
    Unsubscribe(connectionID string) error
    Publish(event string, data interface{}) error
}