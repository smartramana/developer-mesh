package shared

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// MockTool represents a mock tool for testing
type MockTool struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
	Handler     func(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

// GetMockTools returns a set of mock tools for testing
func GetMockTools() []MockTool {
	return []MockTool{
		{
			Name:        "long_running_analysis",
			Description: "Analyzes data with progress updates",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"data": map[string]interface{}{
						"type":        "string",
						"description": "Data to analyze",
					},
					"depth": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"shallow", "medium", "deep"},
						"description": "Analysis depth",
					},
				},
				"required": []string{"data"},
			},
			Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
				data := args["data"].(string)
				depth := "medium"
				if d, ok := args["depth"].(string); ok {
					depth = d
				}

				// Simulate long-running operation with progress
				steps := map[string]int{
					"shallow": 3,
					"medium":  5,
					"deep":    10,
				}

				totalSteps := steps[depth]
				results := make([]string, 0)

				for i := 1; i <= totalSteps; i++ {
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(time.Duration(100+rand.Intn(400)) * time.Millisecond):
						// Progress update would be sent here
						results = append(results, fmt.Sprintf("Step %d: Analyzed %d bytes", i, len(data)/totalSteps))
					}
				}

				return map[string]interface{}{
					"summary":     fmt.Sprintf("Analyzed %d bytes at %s depth", len(data), depth),
					"findings":    results,
					"risk_score":  rand.Intn(100),
					"duration_ms": totalSteps * 250,
				}, nil
			},
		},
		{
			Name:        "data_transformer",
			Description: "Transforms data from one format to another",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"input": map[string]interface{}{
						"type":        "string",
						"description": "Input data",
					},
					"format": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"json", "xml", "csv", "yaml"},
						"description": "Output format",
					},
				},
				"required": []string{"input", "format"},
			},
			Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
				input := args["input"].(string)
				format := args["format"].(string)

				// Simulate transformation
				transformed := fmt.Sprintf("<%s>%s</%s>", format, input, format)

				return map[string]interface{}{
					"output":      transformed,
					"input_size":  len(input),
					"output_size": len(transformed),
					"format":      format,
				}, nil
			},
		},
		{
			Name:        "code_reviewer",
			Description: "Reviews code and provides suggestions",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"code": map[string]interface{}{
						"type":        "string",
						"description": "Code to review",
					},
					"language": map[string]interface{}{
						"type":        "string",
						"description": "Programming language",
					},
					"focus": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Areas to focus on",
					},
				},
				"required": []string{"code"},
			},
			Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
				code := args["code"].(string)
				language := "auto"
				if lang, ok := args["language"].(string); ok {
					language = lang
				}

				// Simulate code review
				lines := strings.Split(code, "\n")
				issues := make([]map[string]interface{}, 0)

				for i, line := range lines {
					if strings.Contains(line, "TODO") {
						issues = append(issues, map[string]interface{}{
							"line":     i + 1,
							"severity": "info",
							"message":  "TODO comment found",
						})
					}
					if strings.Contains(line, "FIXME") {
						issues = append(issues, map[string]interface{}{
							"line":     i + 1,
							"severity": "warning",
							"message":  "FIXME comment found",
						})
					}
				}

				return map[string]interface{}{
					"language":    language,
					"total_lines": len(lines),
					"issues":      issues,
					"score":       100 - len(issues)*5,
				}, nil
			},
		},
		{
			Name:        "resource_monitor",
			Description: "Monitors a resource and emits events",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"resource": map[string]interface{}{
						"type":        "string",
						"description": "Resource identifier",
					},
					"interval_ms": map[string]interface{}{
						"type":        "integer",
						"description": "Monitoring interval in milliseconds",
					},
					"metrics": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Metrics to monitor",
					},
				},
				"required": []string{"resource"},
			},
			Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
				resource := args["resource"].(string)
				interval := 1000
				if i, ok := args["interval_ms"].(float64); ok {
					interval = int(i)
				}

				// This would emit events in a real implementation
				return map[string]interface{}{
					"monitoring": true,
					"resource":   resource,
					"interval":   interval,
					"start_time": time.Now().Format(time.RFC3339),
				}, nil
			},
		},
		{
			Name:        "workflow_executor",
			Description: "Executes a workflow step",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"step": map[string]interface{}{
						"type":        "string",
						"description": "Step identifier",
					},
					"input": map[string]interface{}{
						"type":        "object",
						"description": "Step input data",
					},
					"dependencies": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Required dependencies",
					},
				},
				"required": []string{"step"},
			},
			Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
				step := args["step"].(string)
				input := args["input"]

				// Simulate workflow execution
				if step == "fail" {
					return nil, fmt.Errorf("step %s failed intentionally", step)
				}

				return map[string]interface{}{
					"step":   step,
					"status": "completed",
					"output": map[string]interface{}{
						"processed": input,
						"timestamp": time.Now().Format(time.RFC3339),
					},
				}, nil
			},
		},
		{
			Name:        "collaborative_editor",
			Description: "Collaborative document editing",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"document_id": map[string]interface{}{
						"type":        "string",
						"description": "Document identifier",
					},
					"operation": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"read", "write", "lock", "unlock"},
						"description": "Operation to perform",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Content for write operations",
					},
					"agent_id": map[string]interface{}{
						"type":        "string",
						"description": "Agent performing the operation",
					},
				},
				"required": []string{"document_id", "operation", "agent_id"},
			},
			Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
				docID := args["document_id"].(string)
				operation := args["operation"].(string)
				agentID := args["agent_id"].(string)

				switch operation {
				case "lock":
					return map[string]interface{}{
						"locked":    true,
						"locked_by": agentID,
						"expires":   time.Now().Add(5 * time.Minute).Format(time.RFC3339),
					}, nil
				case "unlock":
					return map[string]interface{}{
						"locked": false,
					}, nil
				case "write":
					content := ""
					if c, ok := args["content"].(string); ok {
						content = c
					}
					return map[string]interface{}{
						"written":     true,
						"bytes":       len(content),
						"modified_by": agentID,
						"version":     time.Now().Unix(),
					}, nil
				case "read":
					return map[string]interface{}{
						"content": fmt.Sprintf("Document %s content", docID),
						"version": time.Now().Unix() - 100,
						"read_by": agentID,
					}, nil
				default:
					return nil, fmt.Errorf("unknown operation: %s", operation)
				}
			},
		},
		{
			Name:        "test_runner",
			Description: "Runs tests and reports results",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"test_suite": map[string]interface{}{
						"type":        "string",
						"description": "Test suite to run",
					},
					"parallel": map[string]interface{}{
						"type":        "boolean",
						"description": "Run tests in parallel",
					},
					"timeout_ms": map[string]interface{}{
						"type":        "integer",
						"description": "Test timeout in milliseconds",
					},
				},
				"required": []string{"test_suite"},
			},
			Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
				suite := args["test_suite"].(string)
				parallel := false
				if p, ok := args["parallel"].(bool); ok {
					parallel = p
				}

				// Simulate test execution
				tests := rand.Intn(10) + 5
				passed := tests - rand.Intn(3)

				return map[string]interface{}{
					"suite":    suite,
					"total":    tests,
					"passed":   passed,
					"failed":   tests - passed,
					"duration": rand.Intn(5000) + 1000,
					"parallel": parallel,
					"coverage": 75 + rand.Intn(20),
				}, nil
			},
		},
		{
			Name:        "deployment_tool",
			Description: "Deploys applications",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"application": map[string]interface{}{
						"type":        "string",
						"description": "Application to deploy",
					},
					"environment": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"dev", "staging", "prod"},
						"description": "Target environment",
					},
					"version": map[string]interface{}{
						"type":        "string",
						"description": "Version to deploy",
					},
					"rollback": map[string]interface{}{
						"type":        "boolean",
						"description": "Rollback on failure",
					},
				},
				"required": []string{"application", "environment", "version"},
			},
			Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
				app := args["application"].(string)
				env := args["environment"].(string)
				version := args["version"].(string)

				// Simulate deployment
				if env == "prod" && rand.Float32() < 0.1 {
					return nil, fmt.Errorf("deployment to %s failed: health check failed", env)
				}

				return map[string]interface{}{
					"deployed":    true,
					"application": app,
					"environment": env,
					"version":     version,
					"url":         fmt.Sprintf("https://%s.%s.example.com", app, env),
					"healthy":     true,
					"timestamp":   time.Now().Format(time.RFC3339),
				}, nil
			},
		},
	}
}

// StreamingTool represents a tool that streams responses
type StreamingTool struct {
	MockTool
	StreamHandler func(ctx context.Context, args map[string]interface{}, stream chan<- StreamUpdate) error
}

// StreamUpdate represents a streaming update
type StreamUpdate struct {
	Type      string      // "progress", "data", "complete", "error"
	Progress  int         // 0-100 for progress updates
	Message   string      // Human-readable message
	Data      interface{} // Actual data chunk
	Error     error       // Error if any
	Timestamp time.Time
}

// GetStreamingTools returns tools that support streaming
func GetStreamingTools() []StreamingTool {
	return []StreamingTool{
		{
			MockTool: MockTool{
				Name:        "file_processor",
				Description: "Processes large files with streaming progress",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"file_path": map[string]interface{}{
							"type": "string",
						},
						"operation": map[string]interface{}{
							"type": "string",
							"enum": []string{"analyze", "transform", "compress"},
						},
					},
					"required": []string{"file_path", "operation"},
				},
			},
			StreamHandler: func(ctx context.Context, args map[string]interface{}, stream chan<- StreamUpdate) error {
				filePath := args["file_path"].(string)
				operation := args["operation"].(string)

				// Simulate file processing with progress
				totalChunks := 10
				for i := 0; i < totalChunks; i++ {
					select {
					case <-ctx.Done():
						stream <- StreamUpdate{
							Type:      "error",
							Error:     ctx.Err(),
							Timestamp: time.Now(),
						}
						return ctx.Err()
					case <-time.After(200 * time.Millisecond):
						progress := (i + 1) * 10
						stream <- StreamUpdate{
							Type:      "progress",
							Progress:  progress,
							Message:   fmt.Sprintf("Processing %s: chunk %d/%d", filePath, i+1, totalChunks),
							Data:      map[string]interface{}{"chunk": i + 1, "bytes_processed": (i + 1) * 1024},
							Timestamp: time.Now(),
						}
					}
				}

				// Send completion
				stream <- StreamUpdate{
					Type:    "complete",
					Message: fmt.Sprintf("Completed %s of %s", operation, filePath),
					Data: map[string]interface{}{
						"total_bytes": totalChunks * 1024,
						"operation":   operation,
						"duration_ms": totalChunks * 200,
					},
					Timestamp: time.Now(),
				}

				return nil
			},
		},
		{
			MockTool: MockTool{
				Name:        "log_streamer",
				Description: "Streams logs in real-time",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"source": map[string]interface{}{
							"type": "string",
						},
						"filter": map[string]interface{}{
							"type": "string",
						},
						"follow": map[string]interface{}{
							"type": "boolean",
						},
					},
					"required": []string{"source"},
				},
			},
			StreamHandler: func(ctx context.Context, args map[string]interface{}, stream chan<- StreamUpdate) error {
				source := args["source"].(string)
				follow := false
				if f, ok := args["follow"].(bool); ok {
					follow = f
				}

				// Simulate log streaming
				logLevels := []string{"INFO", "DEBUG", "WARN", "ERROR"}
				messages := []string{
					"Application started",
					"Processing request",
					"Database connected",
					"Cache hit",
					"Request completed",
				}

				count := 0
				for {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(time.Duration(rand.Intn(500)+100) * time.Millisecond):
						count++
						logEntry := map[string]interface{}{
							"timestamp": time.Now().Format(time.RFC3339),
							"level":     logLevels[rand.Intn(len(logLevels))],
							"source":    source,
							"message":   messages[rand.Intn(len(messages))],
							"count":     count,
						}

						stream <- StreamUpdate{
							Type:      "data",
							Message:   "New log entry",
							Data:      logEntry,
							Timestamp: time.Now(),
						}

						// Stop after some entries if not following
						if !follow && count >= 20 {
							stream <- StreamUpdate{
								Type:      "complete",
								Message:   "Log streaming completed",
								Timestamp: time.Now(),
							}
							return nil
						}
					}
				}
			},
		},
	}
}
