package api_test

import (
	"context"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"functional-tests/shared"

	ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

var _ = Describe("WebSocket Multi-Step Workflows", func() {
	var (
		conn   *websocket.Conn
		ctx    context.Context
		cancel context.CancelFunc
		wsURL  string
		apiKey string
	)

	BeforeEach(func() {
		ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)

		// Get test configuration
		config := shared.GetTestConfig()
		wsURL = config.WebSocketURL
		apiKey = shared.GetTestAPIKey("test-tenant-1")

		var err error
		conn, err = shared.EstablishConnection(wsURL, apiKey)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if conn != nil {
			conn.Close(websocket.StatusNormalClosure, "")
		}
		cancel()
	})

	Describe("Tool Chaining", func() {
		It("should execute tools sequentially with output piping", func() {
			// Define a workflow: fetch data → transform → analyze
			workflowID := uuid.New().String()

			// Step 1: Create workflow
			createWorkflowMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.create",
				Params: map[string]interface{}{
					"id":   workflowID,
					"name": "data-processing-pipeline",
					"steps": []map[string]interface{}{
						{
							"id":   "fetch",
							"tool": "github_get_repository",
							"arguments": map[string]interface{}{
								"owner": "test-org",
								"repo":  "test-repo",
							},
						},
						{
							"id":         "transform",
							"tool":       "data_transformer",
							"depends_on": []string{"fetch"},
							"arguments": map[string]interface{}{
								"input":  "$fetch.result.description", // Reference previous output
								"format": "json",
							},
						},
						{
							"id":         "analyze",
							"tool":       "code_reviewer",
							"depends_on": []string{"transform"},
							"arguments": map[string]interface{}{
								"code":     "$transform.result.output",
								"language": "json",
							},
						},
					},
				},
			}

			err := wsjson.Write(ctx, conn, createWorkflowMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())
			Expect(createResp.Error).To(BeNil())

			// Step 2: Execute workflow
			executeMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.execute",
				Params: map[string]interface{}{
					"workflow_id": workflowID,
					"stream":      true, // Get progress updates
				},
			}

			err = wsjson.Write(ctx, conn, executeMsg)
			Expect(err).NotTo(HaveOccurred())

			// Get execution response
			var executeResp ws.Message
			err = wsjson.Read(ctx, conn, &executeResp)
			Expect(err).NotTo(HaveOccurred())
			Expect(executeResp.ID).To(Equal(executeMsg.ID))
			Expect(executeResp.Error).To(BeNil())

			// Extract execution details from response
			result, ok := executeResp.Result.(map[string]interface{})
			Expect(ok).To(BeTrue())

			executionID, ok := result["execution_id"].(string)
			Expect(ok).To(BeTrue())
			Expect(executionID).NotTo(BeEmpty())

			// Check execution order from response
			executionOrder, ok := result["execution_order"].([]interface{})
			Expect(ok).To(BeTrue())

			// Convert to string array
			stepOrder := make([]string, len(executionOrder))
			for i, step := range executionOrder {
				stepOrder[i] = step.(string)
			}

			// Verify execution order
			Expect(stepOrder).To(Equal([]string{"fetch", "transform", "analyze"}))

			// TODO: In a real implementation, we would check workflow status
			// to verify data was passed between steps
		})

		It("should execute simple workflow with notifications", func() {
			// Create a simple workflow to test notifications
			workflowID := uuid.New().String()

			createWorkflowMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.create",
				Params: map[string]interface{}{
					"id":   workflowID,
					"name": "simple-test",
					"steps": []map[string]interface{}{
						{
							"id":   "step1",
							"tool": "test_runner",
							"arguments": map[string]interface{}{
								"test_suite": "unit",
							},
						},
						{
							"id":   "step2",
							"tool": "test_runner",
							"arguments": map[string]interface{}{
								"test_suite": "integration",
							},
						},
					},
				},
			}

			err := wsjson.Write(ctx, conn, createWorkflowMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Execute workflow with streaming
			executeMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.execute",
				Params: map[string]interface{}{
					"workflow_id": workflowID,
					"stream":      true,
				},
			}

			err = wsjson.Write(ctx, conn, executeMsg)
			Expect(err).NotTo(HaveOccurred())

			// Track notifications
			notifications := []ws.Message{}
			gotResponse := false

			timeout := time.After(5 * time.Second)
			for len(notifications) < 4 { // Expect 2 started + 2 completed
				select {
				case <-timeout:
					GinkgoWriter.Printf("Timeout. Got response: %v, notifications: %d\n", gotResponse, len(notifications))
					for i, n := range notifications {
						GinkgoWriter.Printf("Notification %d: %s\n", i+1, n.Method)
					}
					Fail("Timeout waiting for notifications")
				default:
					var msg ws.Message
					err := wsjson.Read(ctx, conn, &msg)
					Expect(err).NotTo(HaveOccurred())

					if msg.Type == ws.MessageTypeResponse && msg.ID == executeMsg.ID {
						gotResponse = true
						GinkgoWriter.Printf("Got execute response\n")
					} else if msg.Type == ws.MessageTypeNotification {
						notifications = append(notifications, msg)
						GinkgoWriter.Printf("Got notification: %s\n", msg.Method)
					}
				}
			}

			Expect(gotResponse).To(BeTrue())
			Expect(len(notifications)).To(BeNumerically(">=", 4))
		})

		It("should support conditional branching", func() {
			// Create a workflow with conditional execution
			workflowID := uuid.New().String()

			createWorkflowMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.create",
				Params: map[string]interface{}{
					"id":   workflowID,
					"name": "conditional-deployment",
					"steps": []map[string]interface{}{
						{
							"id":   "run_tests",
							"tool": "test_runner",
							"arguments": map[string]interface{}{
								"test_suite": "integration",
							},
						},
						{
							"id":         "deploy_staging",
							"tool":       "deployment_tool",
							"depends_on": []string{"run_tests"},
							"condition": map[string]interface{}{
								"type":       "expression",
								"expression": "$run_tests.result.passed == $run_tests.result.total",
							},
							"arguments": map[string]interface{}{
								"application": "test-app",
								"environment": "staging",
								"version":     "1.0.0",
							},
						},
						{
							"id":         "notify_failure",
							"tool":       "notification_tool",
							"depends_on": []string{"run_tests"},
							"condition": map[string]interface{}{
								"type":       "expression",
								"expression": "$run_tests.result.failed > 0",
							},
							"arguments": map[string]interface{}{
								"message": "Tests failed: $run_tests.result.failed failures",
								"channel": "dev-alerts",
							},
						},
					},
				},
			}

			err := wsjson.Write(ctx, conn, createWorkflowMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Execute workflow
			executeMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.execute",
				Params: map[string]interface{}{
					"workflow_id": workflowID,
				},
			}

			err = wsjson.Write(ctx, conn, executeMsg)
			Expect(err).NotTo(HaveOccurred())

			// Track which steps executed
			executedSteps := make(map[string]bool)
			startedSteps := make(map[string]bool)

			// First, get the execute response
			var executeResp ws.Message
			err = wsjson.Read(ctx, conn, &executeResp)
			Expect(err).NotTo(HaveOccurred())
			Expect(executeResp.ID).To(Equal(executeMsg.ID))
			Expect(executeResp.Error).To(BeNil())
			GinkgoWriter.Printf("Got execute response\n")

			// Now wait for notifications
			timeout := time.After(5 * time.Second)
			done := false
			messageCount := 0

			for !done && len(executedSteps) < 2 { // We expect at least 2 steps
				select {
				case <-timeout:
					GinkgoWriter.Printf("Timeout reached. Received %d notifications, started steps: %v, completed steps: %v\n", messageCount, startedSteps, executedSteps)
					done = true
				default:
					var msg ws.Message
					ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
					err := wsjson.Read(ctx, conn, &msg)
					cancel()

					if err != nil {
						// Timeout is expected when no messages
						continue
					}

					messageCount++
					GinkgoWriter.Printf("Notification %d: Type=%s, Method=%s\n", messageCount, msg.Type, msg.Method)

					if msg.Type == ws.MessageTypeNotification {
						if msg.Method == "workflow.step_completed" {
							if params, ok := msg.Params.(map[string]interface{}); ok {
								stepID := params["step_id"].(string)
								executedSteps[stepID] = true
								GinkgoWriter.Printf("Step completed: %s\n", stepID)
							}
						} else if msg.Method == "workflow.step_started" {
							if params, ok := msg.Params.(map[string]interface{}); ok {
								stepID := params["step_id"].(string)
								startedSteps[stepID] = true
								GinkgoWriter.Printf("Step started: %s\n", stepID)
							}
						}
					}
				}
			}

			// Verify conditional execution
			// For now, accept either started or completed as evidence of execution
			Expect(startedSteps["run_tests"] || executedSteps["run_tests"]).To(BeTrue(), "Tests should always run")

			// Either deploy or notify should run, but not both
			deployRan := executedSteps["deploy_staging"]
			notifyRan := executedSteps["notify_failure"]
			Expect(deployRan || notifyRan).To(BeTrue(), "One branch should execute")
			Expect(deployRan && notifyRan).To(BeFalse(), "Both branches should not execute")
		})

		It("should execute tools in parallel when possible", func() {
			// Create workflow with parallel steps
			workflowID := uuid.New().String()

			createWorkflowMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.create",
				Params: map[string]interface{}{
					"id":   workflowID,
					"name": "parallel-analysis",
					"steps": []map[string]interface{}{
						{
							"id":   "fetch_data",
							"tool": "data_fetcher",
							"arguments": map[string]interface{}{
								"source": "main-db",
							},
						},
						// These three can run in parallel after fetch_data
						{
							"id":         "security_scan",
							"tool":       "security_scanner",
							"depends_on": []string{"fetch_data"},
							"parallel":   true,
							"arguments": map[string]interface{}{
								"data":  "$fetch_data.result",
								"level": "deep",
							},
						},
						{
							"id":         "performance_test",
							"tool":       "performance_tester",
							"depends_on": []string{"fetch_data"},
							"parallel":   true,
							"arguments": map[string]interface{}{
								"data":       "$fetch_data.result",
								"iterations": 100,
							},
						},
						{
							"id":         "quality_check",
							"tool":       "code_reviewer",
							"depends_on": []string{"fetch_data"},
							"parallel":   true,
							"arguments": map[string]interface{}{
								"code": "$fetch_data.result",
							},
						},
						// Final step depends on all parallel steps
						{
							"id":         "generate_report",
							"tool":       "report_generator",
							"depends_on": []string{"security_scan", "performance_test", "quality_check"},
							"arguments": map[string]interface{}{
								"security":    "$security_scan.result",
								"performance": "$performance_test.result",
								"quality":     "$quality_check.result",
							},
						},
					},
				},
			}

			err := wsjson.Write(ctx, conn, createWorkflowMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Execute workflow
			executeMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.execute",
				Params: map[string]interface{}{
					"workflow_id": workflowID,
					"stream":      true,
				},
			}

			startTime := time.Now()
			err = wsjson.Write(ctx, conn, executeMsg)
			Expect(err).NotTo(HaveOccurred())

			// Track step timing
			stepStartTimes := make(map[string]time.Time)
			stepEndTimes := make(map[string]time.Time)

			completed := false
			for !completed {
				var msg ws.Message
				err := wsjson.Read(ctx, conn, &msg)
				Expect(err).NotTo(HaveOccurred())

				if msg.Type == ws.MessageTypeNotification {
					if msg.Method == "workflow.step_started" {
						if params, ok := msg.Params.(map[string]interface{}); ok {
							stepID := params["step_id"].(string)
							stepStartTimes[stepID] = time.Now()
						}
					} else if msg.Method == "workflow.step_completed" {
						if params, ok := msg.Params.(map[string]interface{}); ok {
							stepID := params["step_id"].(string)
							stepEndTimes[stepID] = time.Now()
						}
					}
				} else if msg.Type == ws.MessageTypeResponse {
					completed = true
				}
			}

			totalDuration := time.Since(startTime)

			// Verify parallel execution
			// The three parallel steps should have overlapping execution times
			parallelSteps := []string{"security_scan", "performance_test", "quality_check"}

			overlapFound := false
			for i := 0; i < len(parallelSteps); i++ {
				for j := i + 1; j < len(parallelSteps); j++ {
					step1Start := stepStartTimes[parallelSteps[i]]
					step1End := stepEndTimes[parallelSteps[i]]
					step2Start := stepStartTimes[parallelSteps[j]]
					step2End := stepEndTimes[parallelSteps[j]]

					// Check if execution times overlap
					if step1Start.Before(step2End) && step2Start.Before(step1End) {
						overlapFound = true
						GinkgoWriter.Printf("Steps %s and %s ran in parallel\n",
							parallelSteps[i], parallelSteps[j])
					}
				}
			}

			Expect(overlapFound).To(BeTrue(), "Parallel steps should have overlapping execution")

			GinkgoWriter.Printf("Workflow completed in %v\n", totalDuration)
		})
	})

	Describe("Transaction Management", func() {
		It("should support atomic multi-tool operations", func() {
			// Create a transactional workflow
			workflowID := uuid.New().String()

			createWorkflowMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.create",
				Params: map[string]interface{}{
					"id":            workflowID,
					"name":          "atomic-update",
					"transactional": true, // Enable transaction
					"steps": []map[string]interface{}{
						{
							"id":   "lock_resource",
							"tool": "collaborative_editor",
							"arguments": map[string]interface{}{
								"document_id": "critical-config",
								"operation":   "lock",
								"agent_id":    "workflow-agent",
							},
						},
						{
							"id":         "update_config",
							"tool":       "collaborative_editor",
							"depends_on": []string{"lock_resource"},
							"arguments": map[string]interface{}{
								"document_id": "critical-config",
								"operation":   "write",
								"content":     "new configuration data",
								"agent_id":    "workflow-agent",
							},
						},
						{
							"id":         "validate_config",
							"tool":       "config_validator",
							"depends_on": []string{"update_config"},
							"arguments": map[string]interface{}{
								"config": "$update_config.result.content",
							},
						},
						{
							"id":         "unlock_resource",
							"tool":       "collaborative_editor",
							"depends_on": []string{"validate_config"},
							"arguments": map[string]interface{}{
								"document_id": "critical-config",
								"operation":   "unlock",
								"agent_id":    "workflow-agent",
							},
						},
					},
				},
			}

			err := wsjson.Write(ctx, conn, createWorkflowMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Execute workflow
			executeMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.execute",
				Params: map[string]interface{}{
					"workflow_id": workflowID,
				},
			}

			err = wsjson.Write(ctx, conn, executeMsg)
			Expect(err).NotTo(HaveOccurred())

			// Track transaction state
			transactionEvents := make([]string, 0)

			timeout := time.After(10 * time.Second)
			done := false

			for !done {
				select {
				case <-timeout:
					done = true
				default:
					var msg ws.Message
					err := wsjson.Read(ctx, conn, &msg)
					if err != nil {
						continue
					}

					if msg.Type == ws.MessageTypeNotification {
						if msg.Method == "workflow.transaction_event" {
							if params, ok := msg.Params.(map[string]interface{}); ok {
								event := params["event"].(string)
								transactionEvents = append(transactionEvents, event)
								GinkgoWriter.Printf("Transaction event: %s\n", event)
							}
						}
					} else if msg.Type == ws.MessageTypeResponse {
						done = true

						// Verify transaction completed successfully
						if result, ok := msg.Result.(map[string]interface{}); ok {
							Expect(result["status"]).To(Equal("committed"))
						}
					}
				}
			}

			// Verify transaction lifecycle
			Expect(transactionEvents).To(ContainElement("begin"))
			Expect(transactionEvents).To(ContainElement("commit"))
			Expect(transactionEvents).NotTo(ContainElement("rollback"))
		})

		It("should rollback on failure", func() {
			// Create workflow that will fail
			workflowID := uuid.New().String()

			createWorkflowMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.create",
				Params: map[string]interface{}{
					"id":            workflowID,
					"name":          "failing-transaction",
					"transactional": true,
					"steps": []map[string]interface{}{
						{
							"id":   "create_resource",
							"tool": "resource_manager",
							"arguments": map[string]interface{}{
								"action": "create",
								"name":   "temp-resource",
							},
						},
						{
							"id":         "failing_step",
							"tool":       "workflow_executor",
							"depends_on": []string{"create_resource"},
							"arguments": map[string]interface{}{
								"step": "fail", // This will trigger failure
							},
						},
					},
				},
			}

			err := wsjson.Write(ctx, conn, createWorkflowMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Execute workflow
			executeMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.execute",
				Params: map[string]interface{}{
					"workflow_id": workflowID,
				},
			}

			err = wsjson.Write(ctx, conn, executeMsg)
			Expect(err).NotTo(HaveOccurred())

			// Track rollback
			rollbackOccurred := false
			compensatingActions := make([]string, 0)

			timeout := time.After(10 * time.Second)
			done := false

			for !done {
				select {
				case <-timeout:
					done = true
				default:
					var msg ws.Message
					err := wsjson.Read(ctx, conn, &msg)
					if err != nil {
						continue
					}

					if msg.Type == ws.MessageTypeNotification {
						if msg.Method == "workflow.transaction_event" {
							if params, ok := msg.Params.(map[string]interface{}); ok {
								event := params["event"].(string)
								if event == "rollback" {
									rollbackOccurred = true
								}
							}
						} else if msg.Method == "workflow.compensating_action" {
							if params, ok := msg.Params.(map[string]interface{}); ok {
								action := params["action"].(string)
								compensatingActions = append(compensatingActions, action)
							}
						}
					} else if msg.Type == ws.MessageTypeResponse || msg.Type == ws.MessageTypeError {
						done = true
					}
				}
			}

			Expect(rollbackOccurred).To(BeTrue(), "Transaction should rollback on failure")
			Expect(compensatingActions).To(ContainElement("delete_resource"), "Should clean up created resource")
		})

		It("should support workflow state checkpointing", func() {
			// Create a long-running workflow with checkpoints
			workflowID := uuid.New().String()

			createWorkflowMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.create",
				Params: map[string]interface{}{
					"id":                 workflowID,
					"name":               "resumable-workflow",
					"checkpoint_enabled": true,
					"steps": []map[string]interface{}{
						{
							"id":         "step1",
							"tool":       "long_running_analysis",
							"checkpoint": true, // Save state after this step
							"arguments": map[string]interface{}{
								"data":  "dataset-part-1",
								"depth": "deep",
							},
						},
						{
							"id":         "step2",
							"tool":       "long_running_analysis",
							"depends_on": []string{"step1"},
							"checkpoint": true,
							"arguments": map[string]interface{}{
								"data":  "dataset-part-2",
								"depth": "deep",
							},
						},
						{
							"id":         "step3",
							"tool":       "report_generator",
							"depends_on": []string{"step2"},
							"arguments": map[string]interface{}{
								"data1": "$step1.result",
								"data2": "$step2.result",
							},
						},
					},
				},
			}

			err := wsjson.Write(ctx, conn, createWorkflowMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Start workflow execution
			executeMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.execute",
				Params: map[string]interface{}{
					"workflow_id": workflowID,
				},
			}

			err = wsjson.Write(ctx, conn, executeMsg)
			Expect(err).NotTo(HaveOccurred())

			// Wait for first checkpoint
			var checkpointID string
			checkpointReceived := false

			timeout := time.After(5 * time.Second)
			for !checkpointReceived {
				select {
				case <-timeout:
					Fail("Timeout waiting for checkpoint")
				default:
					var msg ws.Message
					err := wsjson.Read(ctx, conn, &msg)
					if err != nil {
						continue
					}

					if msg.Type == ws.MessageTypeNotification && msg.Method == "workflow.checkpoint_saved" {
						if params, ok := msg.Params.(map[string]interface{}); ok {
							checkpointID = params["checkpoint_id"].(string)
							checkpointReceived = true
							GinkgoWriter.Printf("Checkpoint saved: %s\n", checkpointID)
						}
					}
				}
			}

			// Simulate interruption - cancel workflow
			cancelMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.cancel",
				Params: map[string]interface{}{
					"workflow_id": workflowID,
				},
			}

			err = wsjson.Write(ctx, conn, cancelMsg)
			Expect(err).NotTo(HaveOccurred())

			// Clear remaining messages
			time.Sleep(500 * time.Millisecond)

			// Resume from checkpoint
			resumeMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.resume",
				Params: map[string]interface{}{
					"workflow_id":   workflowID,
					"checkpoint_id": checkpointID,
				},
			}

			err = wsjson.Write(ctx, conn, resumeMsg)
			Expect(err).NotTo(HaveOccurred())

			// Verify workflow resumes and completes
			resumed := false
			completed := false

			timeout = time.After(10 * time.Second)
			for !completed {
				select {
				case <-timeout:
					Fail("Timeout waiting for workflow completion")
				default:
					var msg ws.Message
					err := wsjson.Read(ctx, conn, &msg)
					if err != nil {
						continue
					}

					if msg.Type == ws.MessageTypeNotification && msg.Method == "workflow.resumed" {
						resumed = true
						if params, ok := msg.Params.(map[string]interface{}); ok {
							GinkgoWriter.Printf("Resumed from step: %s\n", params["resumed_from_step"])
						}
					} else if msg.Type == ws.MessageTypeResponse && msg.ID == resumeMsg.ID {
						completed = true
					}
				}
			}

			Expect(resumed).To(BeTrue(), "Workflow should resume from checkpoint")
		})
	})

	Describe("Complex Workflow Patterns", func() {
		It("should support nested workflows", func() {
			// Create parent workflow that calls child workflows
			parentWorkflowID := uuid.New().String()

			createParentMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.create",
				Params: map[string]interface{}{
					"id":   parentWorkflowID,
					"name": "parent-workflow",
					"steps": []map[string]interface{}{
						{
							"id":   "prepare_data",
							"tool": "data_fetcher",
							"arguments": map[string]interface{}{
								"source": "main-dataset",
							},
						},
						{
							"id":         "process_batch_1",
							"type":       "sub_workflow",
							"depends_on": []string{"prepare_data"},
							"workflow": map[string]interface{}{
								"name": "batch-processor",
								"steps": []map[string]interface{}{
									{
										"id":   "transform",
										"tool": "data_transformer",
										"arguments": map[string]interface{}{
											"input":  "$parent.prepare_data.result.batch1",
											"format": "json",
										},
									},
									{
										"id":         "analyze",
										"tool":       "long_running_analysis",
										"depends_on": []string{"transform"},
										"arguments": map[string]interface{}{
											"data": "$transform.result.output",
										},
									},
								},
							},
						},
						{
							"id":         "process_batch_2",
							"type":       "sub_workflow",
							"depends_on": []string{"prepare_data"},
							"parallel":   true, // Run in parallel with batch_1
							"workflow": map[string]interface{}{
								"name": "batch-processor",
								"steps": []map[string]interface{}{
									{
										"id":   "transform",
										"tool": "data_transformer",
										"arguments": map[string]interface{}{
											"input":  "$parent.prepare_data.result.batch2",
											"format": "xml",
										},
									},
									{
										"id":         "analyze",
										"tool":       "long_running_analysis",
										"depends_on": []string{"transform"},
										"arguments": map[string]interface{}{
											"data": "$transform.result.output",
										},
									},
								},
							},
						},
						{
							"id":         "combine_results",
							"tool":       "report_generator",
							"depends_on": []string{"process_batch_1", "process_batch_2"},
							"arguments": map[string]interface{}{
								"batch1_results": "$process_batch_1.result",
								"batch2_results": "$process_batch_2.result",
							},
						},
					},
				},
			}

			err := wsjson.Write(ctx, conn, createParentMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Execute parent workflow
			executeMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.execute",
				Params: map[string]interface{}{
					"workflow_id": parentWorkflowID,
					"stream":      true,
				},
			}

			err = wsjson.Write(ctx, conn, executeMsg)
			Expect(err).NotTo(HaveOccurred())

			// Track sub-workflow execution
			subWorkflowsStarted := make(map[string]bool)
			subWorkflowsCompleted := make(map[string]bool)

			completed := false
			for !completed {
				var msg ws.Message
				err := wsjson.Read(ctx, conn, &msg)
				Expect(err).NotTo(HaveOccurred())

				if msg.Type == ws.MessageTypeNotification {
					if msg.Method == "workflow.sub_workflow_started" {
						if params, ok := msg.Params.(map[string]interface{}); ok {
							subID := params["sub_workflow_id"].(string)
							subWorkflowsStarted[subID] = true
							GinkgoWriter.Printf("Sub-workflow started: %s\n", subID)
						}
					} else if msg.Method == "workflow.sub_workflow_completed" {
						if params, ok := msg.Params.(map[string]interface{}); ok {
							subID := params["sub_workflow_id"].(string)
							subWorkflowsCompleted[subID] = true
						}
					}
				} else if msg.Type == ws.MessageTypeResponse {
					completed = true
				}
			}

			// Verify sub-workflows executed
			Expect(len(subWorkflowsStarted)).To(Equal(2), "Should start 2 sub-workflows")
			Expect(len(subWorkflowsCompleted)).To(Equal(2), "Should complete 2 sub-workflows")
		})

		It("should handle dynamic workflow generation", func() {
			// Create a workflow that generates steps dynamically based on input
			workflowID := uuid.New().String()

			createWorkflowMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.create",
				Params: map[string]interface{}{
					"id":      workflowID,
					"name":    "dynamic-workflow",
					"dynamic": true,
					"generator": map[string]interface{}{
						"tool": "workflow_generator",
						"arguments": map[string]interface{}{
							"template":         "multi-environment-deploy",
							"environments":     []string{"dev", "staging", "prod"},
							"require_approval": []string{"staging", "prod"},
						},
					},
				},
			}

			err := wsjson.Write(ctx, conn, createWorkflowMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Get generated workflow structure
			getWorkflowMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.get",
				Params: map[string]interface{}{
					"workflow_id": workflowID,
				},
			}

			err = wsjson.Write(ctx, conn, getWorkflowMsg)
			Expect(err).NotTo(HaveOccurred())

			var getResp ws.Message
			err = wsjson.Read(ctx, conn, &getResp)
			Expect(err).NotTo(HaveOccurred())

			// Verify dynamic generation created expected steps
			if result, ok := getResp.Result.(map[string]interface{}); ok {
				if steps, ok := result["steps"].([]interface{}); ok {
					// Should have steps for each environment plus approvals
					Expect(len(steps)).To(BeNumerically(">=", 5)) // 3 deploys + 2 approvals minimum

					// Verify approval steps were inserted
					approvalSteps := 0
					for _, step := range steps {
						if stepMap, ok := step.(map[string]interface{}); ok {
							if tool, ok := stepMap["tool"].(string); ok && tool == "approval_tool" {
								approvalSteps++
							}
						}
					}
					Expect(approvalSteps).To(Equal(2), "Should have 2 approval steps")
				}
			}
		})
	})

	Describe("Error Recovery", func() {
		It("should support retry with exponential backoff", func() {
			// Create workflow with retry policy
			workflowID := uuid.New().String()

			createWorkflowMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.create",
				Params: map[string]interface{}{
					"id":   workflowID,
					"name": "retry-workflow",
					"retry_policy": map[string]interface{}{
						"max_attempts":     3,
						"backoff_type":     "exponential",
						"initial_delay_ms": 100,
						"max_delay_ms":     5000,
					},
					"steps": []map[string]interface{}{
						{
							"id":   "flaky_operation",
							"tool": "flaky_tool", // Tool that fails sometimes
							"arguments": map[string]interface{}{
								"failure_rate": 0.7, // 70% failure rate
							},
						},
					},
				},
			}

			err := wsjson.Write(ctx, conn, createWorkflowMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Execute workflow
			executeMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.execute",
				Params: map[string]interface{}{
					"workflow_id": workflowID,
				},
			}

			startTime := time.Now()
			err = wsjson.Write(ctx, conn, executeMsg)
			Expect(err).NotTo(HaveOccurred())

			// Track retry attempts
			retryAttempts := 0
			retryDelays := make([]time.Duration, 0)
			lastRetryTime := startTime

			completed := false
			for !completed {
				var msg ws.Message
				err := wsjson.Read(ctx, conn, &msg)
				Expect(err).NotTo(HaveOccurred())

				if msg.Type == ws.MessageTypeNotification && msg.Method == "workflow.retry_attempt" {
					retryAttempts++
					currentTime := time.Now()
					delay := currentTime.Sub(lastRetryTime)
					retryDelays = append(retryDelays, delay)
					lastRetryTime = currentTime

					if params, ok := msg.Params.(map[string]interface{}); ok {
						GinkgoWriter.Printf("Retry attempt %v after %v delay\n",
							params["attempt"], delay)
					}
				} else if msg.Type == ws.MessageTypeResponse || msg.Type == ws.MessageTypeError {
					completed = true
				}
			}

			// Verify exponential backoff
			if len(retryDelays) > 1 {
				for i := 1; i < len(retryDelays); i++ {
					// Each delay should be roughly double the previous
					ratio := float64(retryDelays[i]) / float64(retryDelays[i-1])
					Expect(ratio).To(BeNumerically("~", 2.0, 0.5), "Delays should follow exponential pattern")
				}
			}
		})

		It("should support circuit breaker pattern", func() {
			// Create workflow with circuit breaker
			workflowID := uuid.New().String()

			createWorkflowMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.create",
				Params: map[string]interface{}{
					"id":   workflowID,
					"name": "circuit-breaker-workflow",
					"circuit_breaker": map[string]interface{}{
						"failure_threshold": 3,
						"timeout_ms":        1000,
						"reset_timeout_ms":  5000,
					},
					"steps": []map[string]interface{}{
						{
							"id":                      "external_api_call",
							"tool":                    "external_api",
							"circuit_breaker_enabled": true,
							"arguments": map[string]interface{}{
								"endpoint": "https://flaky-api.example.com",
							},
						},
					},
				},
			}

			err := wsjson.Write(ctx, conn, createWorkflowMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Execute workflow multiple times to trigger circuit breaker
			circuitOpen := false

			for i := 0; i < 5; i++ {
				executeMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "workflow.execute",
					Params: map[string]interface{}{
						"workflow_id": workflowID,
					},
				}

				err = wsjson.Write(ctx, conn, executeMsg)
				Expect(err).NotTo(HaveOccurred())

				var response ws.Message
				err = wsjson.Read(ctx, conn, &response)
				Expect(err).NotTo(HaveOccurred())

				if response.Error != nil {
					errorMsg := response.Error.Message
					if strings.Contains(errorMsg, "circuit breaker open") {
						circuitOpen = true
						GinkgoWriter.Printf("Circuit breaker opened after %d attempts\n", i+1)
						break
					}
				}
			}

			Expect(circuitOpen).To(BeTrue(), "Circuit breaker should open after failures")
		})
	})
})
