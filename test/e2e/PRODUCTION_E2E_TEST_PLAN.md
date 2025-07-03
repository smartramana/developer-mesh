# DevOps MCP Production End-to-End Test Plan

## Overview

This document outlines a comprehensive E2E testing strategy for the DevOps MCP platform deployed at:
- MCP Server: `mcp.dev-mesh.io` (WebSocket)
- REST API: `api.dev-mesh.io` (HTTP/HTTPS)

The tests will simulate real AI agents connecting to the platform and executing various DevOps workflows.

## Test Architecture

### Test Agent Types

1. **Code Analysis Agent** - Analyzes codebases, identifies issues
2. **DevOps Automation Agent** - Executes CI/CD workflows
3. **Security Scanner Agent** - Performs security audits
4. **Infrastructure Agent** - Manages cloud resources
5. **Monitoring Agent** - Tracks system health and metrics

### Test Framework

- **Language**: Go (matching the codebase)
- **Framework**: Ginkgo + Gomega (BDD-style)
- **WebSocket Client**: github.com/coder/websocket (same as production)
- **JSON Handling**: github.com/coder/websocket/wsjson
- **HTTP Client**: Standard net/http with retry logic
- **Parallelization**: Run scenarios concurrently
- **Reporting**: JUnit XML + Custom HTML reports

## Test Implementation Examples

### Base Test Agent Client

```go
package e2e

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/coder/websocket"
    "github.com/coder/websocket/wsjson"
    "github.com/google/uuid"
    
    ws "github.com/your-org/devops-mcp/pkg/websocket"
)

type TestAgent struct {
    conn         *websocket.Conn
    agentID      string
    capabilities []string
    apiKey       string
    baseURL      string
}

func NewTestAgent(name string, capabilities []string, apiKey, baseURL string) *TestAgent {
    return &TestAgent{
        agentID:      uuid.New().String(),
        capabilities: capabilities,
        apiKey:       apiKey,
        baseURL:      baseURL,
    }
}

func (ta *TestAgent) Connect(ctx context.Context) error {
    wsURL := fmt.Sprintf("wss://%s/v1/ws", ta.baseURL)
    
    opts := &websocket.DialOptions{
        HTTPHeader: http.Header{
            "Authorization": []string{"Bearer " + ta.apiKey},
        },
    }
    
    conn, _, err := websocket.Dial(ctx, wsURL, opts)
    if err != nil {
        return fmt.Errorf("failed to connect: %w", err)
    }
    
    ta.conn = conn
    
    // Initialize connection
    initMsg := ws.Message{
        ID:     uuid.New().String(),
        Type:   ws.MessageTypeRequest,
        Method: "initialize",
        Params: map[string]interface{}{
            "name":         ta.agentID,
            "version":      "1.0.0",
            "capabilities": ta.capabilities,
        },
    }
    
    if err := wsjson.Write(ctx, conn, initMsg); err != nil {
        _ = conn.Close(websocket.StatusNormalClosure, "")
        return fmt.Errorf("failed to send init message: %w", err)
    }
    
    var response ws.Message
    if err := wsjson.Read(ctx, conn, &response); err != nil {
        _ = conn.Close(websocket.StatusNormalClosure, "")
        return fmt.Errorf("failed to read init response: %w", err)
    }
    
    if response.Error != nil {
        _ = conn.Close(websocket.StatusNormalClosure, "")
        return fmt.Errorf("initialization failed: %s", response.Error.Message)
    }
    
    return nil
}

func (ta *TestAgent) ExecuteMethod(ctx context.Context, method string, params interface{}) (*ws.Message, error) {
    msg := ws.Message{
        ID:     uuid.New().String(),
        Type:   ws.MessageTypeRequest,
        Method: method,
        Params: params,
    }
    
    if err := wsjson.Write(ctx, ta.conn, msg); err != nil {
        return nil, fmt.Errorf("failed to send message: %w", err)
    }
    
    var response ws.Message
    if err := wsjson.Read(ctx, ta.conn, &response); err != nil {
        return nil, fmt.Errorf("failed to read response: %w", err)
    }
    
    return &response, nil
}

func (ta *TestAgent) Close() error {
    if ta.conn != nil {
        return ta.conn.Close(websocket.StatusNormalClosure, "Test completed")
    }
    return nil
}
```

## E2E Test Scenarios

### 1. Single Agent Scenarios

#### 1.1 Basic Agent Lifecycle
```go
Describe("Agent Lifecycle", func() {
    It("should complete full lifecycle", func() {
        agent := NewTestAgent("test-agent", []string{"code_analysis"}, apiKey, "mcp.dev-mesh.io")
        
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        
        // Connect
        Expect(agent.Connect(ctx)).To(Succeed())
        defer agent.Close()
        
        // Register capabilities
        resp, err := agent.ExecuteMethod(ctx, "agent.register", map[string]interface{}{
            "capabilities": agent.capabilities,
            "status":      "available",
        })
        Expect(err).NotTo(HaveOccurred())
        Expect(resp.Error).To(BeNil())
        
        // Heartbeat
        resp, err = agent.ExecuteMethod(ctx, "ping", nil)
        Expect(err).NotTo(HaveOccurred())
        Expect(resp.Result).To(Equal("pong"))
        
        // Graceful disconnect handled by defer
    })
})
```

#### 1.2 Tool Discovery and Execution
```go
It("should discover and execute tools", func() {
    agent := NewTestAgent("tool-agent", []string{"devops"}, apiKey, "mcp.dev-mesh.io")
    
    ctx := context.Background()
    Expect(agent.Connect(ctx)).To(Succeed())
    defer agent.Close()
    
    // Discover tools
    resp, err := agent.ExecuteMethod(ctx, "tool.list", nil)
    Expect(err).NotTo(HaveOccurred())
    
    tools := resp.Result.([]interface{})
    Expect(tools).NotTo(BeEmpty())
    
    // Execute GitHub tool
    resp, err = agent.ExecuteMethod(ctx, "tool.execute", map[string]interface{}{
        "tool": "github",
        "operation": "list_repositories",
        "params": map[string]interface{}{
            "org": "test-org",
        },
    })
    Expect(err).NotTo(HaveOccurred())
})
```

### 2. Multi-Agent Collaboration Scenarios

#### 2.1 Code Review Workflow
```go
Describe("Code Review Workflow", func() {
    It("should coordinate code review between agents", func() {
        ctx := context.Background()
        
        // Create agents
        codeAgent := NewTestAgent("code-analyzer", []string{"code_analysis"}, apiKey, "mcp.dev-mesh.io")
        securityAgent := NewTestAgent("security-scanner", []string{"security_scanning"}, apiKey, "mcp.dev-mesh.io")
        
        // Connect both agents
        Expect(codeAgent.Connect(ctx)).To(Succeed())
        defer codeAgent.Close()
        
        Expect(securityAgent.Connect(ctx)).To(Succeed())
        defer securityAgent.Close()
        
        // Create collaborative workflow
        resp, err := codeAgent.ExecuteMethod(ctx, "workflow.create_collaborative", map[string]interface{}{
            "name": "code-review",
            "steps": []map[string]interface{}{
                {
                    "name": "analyze_code",
                    "agent_capability": "code_analysis",
                },
                {
                    "name": "security_scan",
                    "agent_capability": "security_scanning",
                },
            },
            "strategy": "sequential",
        })
        Expect(err).NotTo(HaveOccurred())
        
        workflowID := resp.Result.(map[string]interface{})["id"].(string)
        
        // Execute workflow
        resp, err = codeAgent.ExecuteMethod(ctx, "workflow.execute_collaborative", map[string]interface{}{
            "workflow_id": workflowID,
            "input": map[string]interface{}{
                "pr_url": "https://github.com/test/repo/pull/123",
            },
        })
        Expect(err).NotTo(HaveOccurred())
        
        // Monitor completion
        Eventually(func() string {
            resp, _ := codeAgent.ExecuteMethod(ctx, "workflow.status", map[string]interface{}{
                "workflow_id": workflowID,
            })
            return resp.Result.(map[string]interface{})["status"].(string)
        }, 30*time.Second).Should(Equal("completed"))
    })
})
```

### 3. Stress and Performance Scenarios

#### 3.1 Agent Swarm Test
```go
Describe("Agent Swarm", func() {
    It("should handle 50 concurrent agents", func() {
        ctx := context.Background()
        agents := make([]*TestAgent, 50)
        
        // Connect all agents concurrently
        var wg sync.WaitGroup
        errors := make(chan error, 50)
        
        for i := 0; i < 50; i++ {
            wg.Add(1)
            go func(idx int) {
                defer wg.Done()
                
                agent := NewTestAgent(
                    fmt.Sprintf("swarm-agent-%d", idx),
                    []string{"test_capability"},
                    apiKey,
                    "mcp.dev-mesh.io",
                )
                
                if err := agent.Connect(ctx); err != nil {
                    errors <- err
                    return
                }
                
                agents[idx] = agent
            }(i)
        }
        
        wg.Wait()
        close(errors)
        
        // Check for errors
        for err := range errors {
            Expect(err).NotTo(HaveOccurred())
        }
        
        // Execute tasks on all agents
        for _, agent := range agents {
            if agent != nil {
                defer agent.Close()
                
                resp, err := agent.ExecuteMethod(ctx, "echo", map[string]interface{}{
                    "message": "test",
                })
                Expect(err).NotTo(HaveOccurred())
                Expect(resp.Result).To(Equal("test"))
            }
        }
    })
})
```

### 4. Binary Protocol Testing

```go
Describe("Binary Protocol", func() {
    It("should handle binary messages with compression", func() {
        agent := NewTestAgent("binary-agent", []string{"data_processing"}, apiKey, "mcp.dev-mesh.io")
        
        ctx := context.Background()
        Expect(agent.Connect(ctx)).To(Succeed())
        defer agent.Close()
        
        // Send large message that triggers compression (>1KB)
        largeData := make([]byte, 2048)
        for i := range largeData {
            largeData[i] = byte(i % 256)
        }
        
        // Send as binary message
        err := agent.conn.Write(ctx, websocket.MessageBinary, largeData)
        Expect(err).NotTo(HaveOccurred())
        
        // Read response
        msgType, data, err := agent.conn.Read(ctx)
        Expect(err).NotTo(HaveOccurred())
        Expect(msgType).To(Equal(websocket.MessageBinary))
        Expect(len(data)).To(BeNumerically(">", 0))
    })
})
```

### 5. Session Recovery

```go
Describe("Session Recovery", func() {
    It("should recover session after disconnect", func() {
        agent := NewTestAgent("recovery-agent", []string{"persistent"}, apiKey, "mcp.dev-mesh.io")
        
        ctx := context.Background()
        Expect(agent.Connect(ctx)).To(Succeed())
        
        // Create session
        resp, err := agent.ExecuteMethod(ctx, "session.create", map[string]interface{}{
            "name": "test-session",
        })
        Expect(err).NotTo(HaveOccurred())
        
        sessionID := resp.Result.(map[string]interface{})["id"].(string)
        
        // Simulate disconnect
        agent.conn.Close(websocket.StatusNormalClosure, "")
        
        // Wait and reconnect
        time.Sleep(2 * time.Second)
        Expect(agent.Connect(ctx)).To(Succeed())
        defer agent.Close()
        
        // Recover session
        resp, err = agent.ExecuteMethod(ctx, "session.recover", map[string]interface{}{
            "session_id": sessionID,
        })
        Expect(err).NotTo(HaveOccurred())
        Expect(resp.Result.(map[string]interface{})["recovered"]).To(BeTrue())
    })
})
```

## Implementation Strategy

### Phase 1: Test Infrastructure (Week 1)
- [ ] Create test agent framework using coder/websocket
- [ ] Implement connection utilities matching production patterns
- [ ] Set up test data management
- [ ] Create test reporting system
- [ ] Implement test isolation mechanisms

### Phase 2: Basic E2E Tests (Week 2)
- [ ] Single agent scenarios
- [ ] Tool execution tests
- [ ] Context management tests
- [ ] Session management tests
- [ ] Basic error handling tests

### Phase 3: Collaboration Tests (Week 3)
- [ ] Multi-agent setup utilities
- [ ] Collaboration workflow tests
- [ ] Consensus mechanism tests
- [ ] Shared workspace tests
- [ ] Task delegation tests

### Phase 4: Advanced Scenarios (Week 4)
- [ ] Performance and stress tests
- [ ] Failure and recovery tests
- [ ] Security scenarios
- [ ] Real-world workflow tests
- [ ] Edge case handling

### Phase 5: CI/CD Integration (Week 5)
- [ ] GitHub Actions workflow
- [ ] Automated test execution
- [ ] Performance benchmarking
- [ ] Test result notifications
- [ ] Documentation generation

## Test Execution

### Local Development
```bash
# Run all E2E tests
make test-e2e

# Run specific scenario
make test-e2e SCENARIO=multi-agent-collaboration

# Run with verbose output
make test-e2e-verbose

# Run performance tests
make test-e2e-performance
```

### CI/CD Pipeline
```yaml
# Runs on:
- Every PR to main
- Nightly against production
- On-demand via workflow dispatch
- After production deployments
```

### Production Testing
```bash
# Safety measures:
- Read-only operations by default
- Dedicated test tenant
- Rate limiting awareness
- Automatic cleanup
- Non-disruptive testing
```

## Success Metrics

### Functional Coverage
- [ ] 100% of documented MCP methods tested
- [ ] All agent collaboration patterns verified
- [ ] Every error scenario handled
- [ ] All tool integrations validated

### Performance Targets
- [ ] WebSocket connection < 100ms
- [ ] Message round-trip < 50ms
- [ ] 1000+ messages/second throughput
- [ ] 100+ concurrent agents supported
- [ ] Zero message loss under load

### Reliability Goals
- [ ] 99.9% test suite stability
- [ ] Automatic recovery from failures
- [ ] Clear error diagnostics
- [ ] Reproducible test results

## Next Steps

1. Review and approve test plan
2. Set up test infrastructure
3. Implement core test utilities
4. Begin scenario implementation
5. Integrate with CI/CD pipeline
6. Deploy monitoring dashboards

---

**Note**: This plan uses the coder/websocket library consistently with the rest of the DevOps MCP codebase, ensuring compatibility and maintaining established patterns.