package api_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"functional-tests/shared"

	ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

var _ = Describe("WebSocket Multi-Agent Collaboration", func() {
	var (
		ctx    context.Context
		cancel context.CancelFunc
		wsURL  string
		apiKey string
	)

	BeforeEach(func() {
		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)

		// Get test configuration
		config := shared.GetTestConfig()
		wsURL = config.WebSocketURL
		apiKey = shared.GetTestAPIKey("test-tenant-1")
	})

	AfterEach(func() {
		cancel()
	})

	Describe("Agent Registration and Discovery", func() {
		It("should register multiple agents with different capabilities", func() {
			// Create connections for different agents
			agents := []struct {
				name         string
				capabilities []string
				conn         *websocket.Conn
				id           string
			}{
				{
					name:         "code-expert",
					capabilities: []string{"code_review", "refactoring", "optimization"},
				},
				{
					name:         "test-specialist",
					capabilities: []string{"test_generation", "test_execution", "coverage_analysis"},
				},
				{
					name:         "devops-engineer",
					capabilities: []string{"deployment", "monitoring", "infrastructure"},
				},
			}

			// Connect and register each agent
			for i := range agents {
				conn, err := shared.EstablishConnection(wsURL, apiKey)
				Expect(err).NotTo(HaveOccurred())
				agents[i].conn = conn
				defer conn.Close(websocket.StatusNormalClosure, "")

				// Register agent
				registerMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "agent.register",
					Params: map[string]interface{}{
						"name":         agents[i].name,
						"capabilities": agents[i].capabilities,
						"metadata": map[string]interface{}{
							"version":  "1.0.0",
							"language": "go",
						},
					},
				}

				err = wsjson.Write(ctx, conn, registerMsg)
				Expect(err).NotTo(HaveOccurred())

				var registerResp ws.Message
				err = wsjson.Read(ctx, conn, &registerResp)
				Expect(err).NotTo(HaveOccurred())
				Expect(registerResp.Error).To(BeNil())

				if result, ok := registerResp.Result.(map[string]interface{}); ok {
					agents[i].id = result["agent_id"].(string)
				}
			}

			// Discover agents with specific capabilities
			discoverMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "agent.discover",
				Params: map[string]interface{}{
					"capabilities": []string{"code_review"},
				},
			}

			err := wsjson.Write(ctx, agents[0].conn, discoverMsg)
			Expect(err).NotTo(HaveOccurred())

			var discoverResp ws.Message
			err = wsjson.Read(ctx, agents[0].conn, &discoverResp)
			Expect(err).NotTo(HaveOccurred())

			// Should find code-expert agent
			if result, ok := discoverResp.Result.(map[string]interface{}); ok {
				if foundAgents, ok := result["agents"].([]interface{}); ok {
					found := false
					for _, agent := range foundAgents {
						if a, ok := agent.(map[string]interface{}); ok {
							if a["name"] == "code-expert" {
								found = true
								break
							}
						}
					}
					Expect(found).To(BeTrue(), "Should find code-expert agent")
				}
			}
		})

		It("should track agent availability", func() {
			// Create two agent connections with different API keys (same tenant, different users)
			apiKey1 := shared.GetTestAPIKey("agent-1")
			agent1, err := shared.EstablishConnection(wsURL, apiKey1)
			Expect(err).NotTo(HaveOccurred())
			defer agent1.Close(websocket.StatusNormalClosure, "")

			apiKey2 := shared.GetTestAPIKey("agent-2")
			agent2, err := shared.EstablishConnection(wsURL, apiKey2)
			Expect(err).NotTo(HaveOccurred())
			defer agent2.Close(websocket.StatusNormalClosure, "")

			// Register both agents
			for i, conn := range []*websocket.Conn{agent1, agent2} {
				registerMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "agent.register",
					Params: map[string]interface{}{
						"name":         fmt.Sprintf("agent-%d", i+1),
						"capabilities": []string{"general"},
					},
				}

				err = wsjson.Write(ctx, conn, registerMsg)
				Expect(err).NotTo(HaveOccurred())

				var resp ws.Message
				err = wsjson.Read(ctx, conn, &resp)
				Expect(err).NotTo(HaveOccurred())
			}

			// Agent 2 subscribes to agent status changes
			subMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "subscribe",
				Params: map[string]interface{}{
					"resource": "agent.status",
				},
			}

			err = wsjson.Write(ctx, agent2, subMsg)
			Expect(err).NotTo(HaveOccurred())

			var subResp ws.Message
			err = wsjson.Read(ctx, agent2, &subResp)
			Expect(err).NotTo(HaveOccurred())

			// Agent 1 updates status to busy
			statusMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "agent.update_status",
				Params: map[string]interface{}{
					"status": "busy",
					"current_task": map[string]interface{}{
						"type":     "code_review",
						"duration": "5m",
					},
				},
			}

			err = wsjson.Write(ctx, agent1, statusMsg)
			Expect(err).NotTo(HaveOccurred())

			// Agent 2 should receive notification
			statusReceived := false
			timeout := time.After(2 * time.Second)

			for !statusReceived {
				select {
				case <-timeout:
					Fail("Timeout waiting for status update")
				default:
					var msg ws.Message
					err := wsjson.Read(ctx, agent2, &msg)
					if err != nil {
						continue
					}

					if msg.Type == ws.MessageTypeNotification && msg.Method == "agent.status_changed" {
						if params, ok := msg.Params.(map[string]interface{}); ok {
							if params["name"] == "agent-1" && params["status"] == "busy" {
								statusReceived = true
							}
						}
					}
				}
			}
		})
	})

	Describe("Task Delegation", func() {
		It("should delegate tasks between agents", func() {
			// Create coordinator and worker agents with different API keys (same tenant)
			apiKey1 := shared.GetTestAPIKey("agent-1")
			coordinator, err := shared.EstablishConnection(wsURL, apiKey1)
			Expect(err).NotTo(HaveOccurred())
			defer coordinator.Close(websocket.StatusNormalClosure, "")

			apiKey2 := shared.GetTestAPIKey("agent-2")
			worker, err := shared.EstablishConnection(wsURL, apiKey2)
			Expect(err).NotTo(HaveOccurred())
			defer worker.Close(websocket.StatusNormalClosure, "")

			// Register coordinator
			registerCoordMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "agent.register",
				Params: map[string]interface{}{
					"name":         "coordinator",
					"capabilities": []string{"orchestration", "delegation"},
					"role":         "coordinator",
				},
			}

			err = wsjson.Write(ctx, coordinator, registerCoordMsg)
			Expect(err).NotTo(HaveOccurred())

			var coordResp ws.Message
			err = wsjson.Read(ctx, coordinator, &coordResp)
			Expect(err).NotTo(HaveOccurred())

			// coordinatorID would be used in production for tracking
			// coordinatorID := ""
			if result, ok := coordResp.Result.(map[string]interface{}); ok {
				_ = result["agent_id"].(string)
			}

			// Register worker
			registerWorkerMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "agent.register",
				Params: map[string]interface{}{
					"name":         "worker-1",
					"capabilities": []string{"code_review", "testing"},
					"role":         "worker",
				},
			}

			err = wsjson.Write(ctx, worker, registerWorkerMsg)
			Expect(err).NotTo(HaveOccurred())

			var workerResp ws.Message
			err = wsjson.Read(ctx, worker, &workerResp)
			Expect(err).NotTo(HaveOccurred())

			workerID := ""
			if result, ok := workerResp.Result.(map[string]interface{}); ok {
				workerID = result["agent_id"].(string)
			}

			// Worker subscribes to task assignments
			taskSubMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "subscribe",
				Params: map[string]interface{}{
					"resource": "task.assignments",
					"filter": map[string]interface{}{
						"agent_id": workerID,
					},
				},
			}

			err = wsjson.Write(ctx, worker, taskSubMsg)
			Expect(err).NotTo(HaveOccurred())

			var taskSubResp ws.Message
			err = wsjson.Read(ctx, worker, &taskSubResp)
			Expect(err).NotTo(HaveOccurred())

			// Coordinator delegates task
			taskID := uuid.New().String()
			delegateMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "task.delegate",
				Params: map[string]interface{}{
					"task_id":     taskID,
					"to_agent_id": workerID,
					"task": map[string]interface{}{
						"type":        "code_review",
						"priority":    "high",
						"description": "Review PR #123",
						"payload": map[string]interface{}{
							"pr_url":  "https://github.com/org/repo/pull/123",
							"files":   5,
							"changes": 150,
						},
					},
				},
			}

			err = wsjson.Write(ctx, coordinator, delegateMsg)
			Expect(err).NotTo(HaveOccurred())

			// Worker should receive task assignment
			taskReceived := false
			timeout := time.After(2 * time.Second)

			for !taskReceived {
				select {
				case <-timeout:
					Fail("Timeout waiting for task assignment")
				default:
					var msg ws.Message
					err := wsjson.Read(ctx, worker, &msg)
					if err != nil {
						continue
					}

					if msg.Type == ws.MessageTypeNotification && msg.Method == "task.assigned" {
						if params, ok := msg.Params.(map[string]interface{}); ok {
							if params["task_id"] == taskID {
								taskReceived = true

								// Worker accepts task
								acceptMsg := ws.Message{
									ID:     uuid.New().String(),
									Type:   ws.MessageTypeRequest,
									Method: "task.accept",
									Params: map[string]interface{}{
										"task_id":            taskID,
										"estimated_duration": "10m",
									},
								}

								err = wsjson.Write(ctx, worker, acceptMsg)
								Expect(err).NotTo(HaveOccurred())
							}
						}
					}
				}
			}

			// Coordinator should receive acceptance notification
			acceptanceReceived := false
			timeout2 := time.After(2 * time.Second)

			for !acceptanceReceived {
				select {
				case <-timeout2:
					Fail("Timeout waiting for task acceptance")
				default:
					var msg ws.Message
					err := wsjson.Read(ctx, coordinator, &msg)
					if err != nil {
						continue
					}

					if msg.Type == ws.MessageTypeNotification && msg.Method == "task.accepted" {
						if params, ok := msg.Params.(map[string]interface{}); ok {
							if params["task_id"] == taskID && params["agent_id"] == workerID {
								acceptanceReceived = true
							}
						}
					}
				}
			}
		})

		It("should support task result aggregation", func() {
			// Create coordinator and multiple workers
			coordinator, err := shared.EstablishConnection(wsURL, apiKey)
			Expect(err).NotTo(HaveOccurred())
			defer coordinator.Close(websocket.StatusNormalClosure, "")

			numWorkers := 3
			workers := make([]*websocket.Conn, numWorkers)
			workerIDs := make([]string, numWorkers)

			for i := 0; i < numWorkers; i++ {
				worker, err := shared.EstablishConnection(wsURL, apiKey)
				Expect(err).NotTo(HaveOccurred())
				workers[i] = worker
				defer worker.Close(websocket.StatusNormalClosure, "")

				// Register worker
				registerMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "agent.register",
					Params: map[string]interface{}{
						"name":         fmt.Sprintf("worker-%d", i+1),
						"capabilities": []string{"analysis"},
					},
				}

				err = wsjson.Write(ctx, worker, registerMsg)
				Expect(err).NotTo(HaveOccurred())

				var resp ws.Message
				err = wsjson.Read(ctx, worker, &resp)
				Expect(err).NotTo(HaveOccurred())

				if result, ok := resp.Result.(map[string]interface{}); ok {
					workerIDs[i] = result["agent_id"].(string)
				}
			}

			// Create distributed task
			parentTaskID := uuid.New().String()
			createTaskMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "task.create_distributed",
				Params: map[string]interface{}{
					"task_id":     parentTaskID,
					"type":        "parallel_analysis",
					"description": "Analyze large codebase",
					"subtasks": []map[string]interface{}{
						{
							"id":          "subtask-1",
							"agent_id":    workerIDs[0],
							"description": "Analyze module A",
						},
						{
							"id":          "subtask-2",
							"agent_id":    workerIDs[1],
							"description": "Analyze module B",
						},
						{
							"id":          "subtask-3",
							"agent_id":    workerIDs[2],
							"description": "Analyze module C",
						},
					},
					"aggregation": map[string]interface{}{
						"method":       "combine_results",
						"wait_for_all": true,
					},
				},
			}

			err = wsjson.Write(ctx, coordinator, createTaskMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, coordinator, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Workers complete their subtasks
			var wg sync.WaitGroup
			// results would be collected in production
			// results := make([]map[string]interface{}, numWorkers)

			for i := 0; i < numWorkers; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()

					// Simulate work
					time.Sleep(time.Duration(100+idx*50) * time.Millisecond)

					// Submit result
					resultMsg := ws.Message{
						ID:     uuid.New().String(),
						Type:   ws.MessageTypeRequest,
						Method: "task.submit_result",
						Params: map[string]interface{}{
							"task_id":    parentTaskID,
							"subtask_id": fmt.Sprintf("subtask-%d", idx+1),
							"result": map[string]interface{}{
								"module":     string(rune('A' + idx)),
								"issues":     idx + 1,
								"coverage":   90 - idx*5,
								"complexity": 10 + idx*2,
							},
						},
					}

					err := wsjson.Write(ctx, workers[idx], resultMsg)
					Expect(err).NotTo(HaveOccurred())

					var resp ws.Message
					err = wsjson.Read(ctx, workers[idx], &resp)
					Expect(err).NotTo(HaveOccurred())
				}(i)
			}

			// Coordinator waits for aggregated result
			aggregatedReceived := false
			timeout := time.After(5 * time.Second)

			for !aggregatedReceived {
				select {
				case <-timeout:
					Fail("Timeout waiting for aggregated result")
				default:
					var msg ws.Message
					err := wsjson.Read(ctx, coordinator, &msg)
					if err != nil {
						continue
					}

					if msg.Type == ws.MessageTypeNotification && msg.Method == "task.completed" {
						if params, ok := msg.Params.(map[string]interface{}); ok {
							if params["task_id"] == parentTaskID {
								aggregatedReceived = true

								// Verify aggregated results
								if result, ok := params["aggregated_result"].(map[string]interface{}); ok {
									Expect(result).To(HaveKey("total_issues"))
									Expect(result).To(HaveKey("average_coverage"))
									Expect(result).To(HaveKey("max_complexity"))
									Expect(result).To(HaveKey("modules_analyzed"))

									if modules, ok := result["modules_analyzed"].(float64); ok {
										Expect(int(modules)).To(Equal(numWorkers))
									}
								}
							}
						}
					}
				}
			}

			wg.Wait()
		})
	})

	Describe("Collaborative Workflows", func() {
		It("should coordinate agents for complex workflows", func() {
			// Create specialized agents
			agents := make(map[string]*websocket.Conn)
			agentIDs := make(map[string]string)

			specializations := []string{"analyzer", "optimizer", "validator"}

			for _, spec := range specializations {
				conn, err := shared.EstablishConnection(wsURL, apiKey)
				Expect(err).NotTo(HaveOccurred())
				agents[spec] = conn
				defer conn.Close(websocket.StatusNormalClosure, "")

				// Register with specialization
				registerMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "agent.register",
					Params: map[string]interface{}{
						"name":         spec + "-agent",
						"capabilities": []string{spec},
						"metadata": map[string]interface{}{
							"specialization": spec,
						},
					},
				}

				err = wsjson.Write(ctx, conn, registerMsg)
				Expect(err).NotTo(HaveOccurred())

				var resp ws.Message
				err = wsjson.Read(ctx, conn, &resp)
				Expect(err).NotTo(HaveOccurred())

				if result, ok := resp.Result.(map[string]interface{}); ok {
					agentIDs[spec] = result["agent_id"].(string)
				}
			}

			// Create collaborative workflow
			workflowID := uuid.New().String()
			workflowMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.create_collaborative",
				Params: map[string]interface{}{
					"workflow_id": workflowID,
					"name":        "code-optimization-pipeline",
					"agents": map[string]interface{}{
						"analyzer":  agentIDs["analyzer"],
						"optimizer": agentIDs["optimizer"],
						"validator": agentIDs["validator"],
					},
					"steps": []map[string]interface{}{
						{
							"id":     "analyze",
							"agent":  "analyzer",
							"action": "analyze_code",
							"input":  "source_code",
						},
						{
							"id":         "optimize",
							"agent":      "optimizer",
							"action":     "optimize_based_on_analysis",
							"depends_on": []string{"analyze"},
							"input":      "$analyze.output",
						},
						{
							"id":         "validate",
							"agent":      "validator",
							"action":     "validate_optimization",
							"depends_on": []string{"optimize"},
							"input":      "$optimize.output",
						},
					},
				},
			}

			err := wsjson.Write(ctx, agents["analyzer"], workflowMsg)
			Expect(err).NotTo(HaveOccurred())

			var workflowResp ws.Message
			err = wsjson.Read(ctx, agents["analyzer"], &workflowResp)
			Expect(err).NotTo(HaveOccurred())

			// Execute workflow
			executeMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workflow.execute_collaborative",
				Params: map[string]interface{}{
					"workflow_id": workflowID,
					"input": map[string]interface{}{
						"source_code": "func example() { /* code */ }",
					},
				},
			}

			err = wsjson.Write(ctx, agents["analyzer"], executeMsg)
			Expect(err).NotTo(HaveOccurred())

			// Each agent should receive their task
			tasksReceived := make(map[string]bool)
			var wg sync.WaitGroup

			for spec, conn := range agents {
				wg.Add(1)
				go func(s string, c *websocket.Conn) {
					defer wg.Done()

					timeout := time.After(5 * time.Second)
					for {
						select {
						case <-timeout:
							return
						default:
							var msg ws.Message
							err := wsjson.Read(ctx, c, &msg)
							if err != nil {
								continue
							}

							if msg.Type == ws.MessageTypeNotification && msg.Method == "workflow.task_ready" {
								if params, ok := msg.Params.(map[string]interface{}); ok {
									tasksReceived[s] = true

									// Simulate task completion
									completeMsg := ws.Message{
										ID:     uuid.New().String(),
										Type:   ws.MessageTypeRequest,
										Method: "workflow.complete_task",
										Params: map[string]interface{}{
											"workflow_id": workflowID,
											"step_id":     params["step_id"],
											"output": map[string]interface{}{
												"status": "success",
												"data":   fmt.Sprintf("Processed by %s", s),
											},
										},
									}

									wsjson.Write(ctx, c, completeMsg)
								}
							}
						}
					}
				}(spec, conn)
			}

			// Wait for all agents to receive tasks
			time.Sleep(2 * time.Second)
			wg.Wait()

			// Verify all agents participated
			for spec := range agents {
				Expect(tasksReceived[spec]).To(BeTrue(),
					fmt.Sprintf("Agent %s should receive task", spec))
			}
		})

		It("should handle agent failures gracefully", func() {
			// Create primary and backup agents
			primary, err := shared.EstablishConnection(wsURL, apiKey)
			Expect(err).NotTo(HaveOccurred())
			defer primary.Close(websocket.StatusNormalClosure, "")

			backup, err := shared.EstablishConnection(wsURL, apiKey)
			Expect(err).NotTo(HaveOccurred())
			defer backup.Close(websocket.StatusNormalClosure, "")

			// Register both with same capabilities
			for i, conn := range []*websocket.Conn{primary, backup} {
				registerMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "agent.register",
					Params: map[string]interface{}{
						"name":         fmt.Sprintf("processor-%d", i+1),
						"capabilities": []string{"data_processing"},
						"role": func() string {
							if i == 0 {
								return "primary"
							} else {
								return "backup"
							}
						}(),
					},
				}

				err = wsjson.Write(ctx, conn, registerMsg)
				Expect(err).NotTo(HaveOccurred())

				var resp ws.Message
				err = wsjson.Read(ctx, conn, &resp)
				Expect(err).NotTo(HaveOccurred())
			}

			// Create task with failover
			taskID := uuid.New().String()
			createTaskMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "task.create",
				Params: map[string]interface{}{
					"task_id": taskID,
					"type":    "critical_processing",
					"failover": map[string]interface{}{
						"enabled":      true,
						"max_attempts": 2,
						"timeout_ms":   1000,
					},
					"requirements": map[string]interface{}{
						"capabilities": []string{"data_processing"},
					},
				},
			}

			err = wsjson.Write(ctx, primary, createTaskMsg)
			Expect(err).NotTo(HaveOccurred())

			// Primary agent fails the task
			go func() {
				for {
					var msg ws.Message
					err := wsjson.Read(ctx, primary, &msg)
					if err != nil {
						return
					}

					if msg.Type == ws.MessageTypeNotification && msg.Method == "task.assigned" {
						// Simulate failure
						time.Sleep(500 * time.Millisecond)

						failMsg := ws.Message{
							ID:     uuid.New().String(),
							Type:   ws.MessageTypeRequest,
							Method: "task.fail",
							Params: map[string]interface{}{
								"task_id": taskID,
								"reason":  "simulated failure",
							},
						}

						wsjson.Write(ctx, primary, failMsg)
						return
					}
				}
			}()

			// Backup should receive the task after primary fails
			backupReceived := false
			timeout := time.After(3 * time.Second)

			for !backupReceived {
				select {
				case <-timeout:
					Fail("Timeout waiting for failover")
				default:
					var msg ws.Message
					err := wsjson.Read(ctx, backup, &msg)
					if err != nil {
						continue
					}

					if msg.Type == ws.MessageTypeNotification && msg.Method == "task.assigned" {
						if params, ok := msg.Params.(map[string]interface{}); ok {
							if params["task_id"] == taskID {
								backupReceived = true

								// Backup completes the task
								completeMsg := ws.Message{
									ID:     uuid.New().String(),
									Type:   ws.MessageTypeRequest,
									Method: "task.complete",
									Params: map[string]interface{}{
										"task_id": taskID,
										"result": map[string]interface{}{
											"status":       "completed",
											"processed_by": "backup",
										},
									},
								}

								err = wsjson.Write(ctx, backup, completeMsg)
								Expect(err).NotTo(HaveOccurred())
							}
						}
					}
				}
			}
		})
	})

	Describe("Shared State Management", func() {
		It("should synchronize shared state between agents", func() {
			// Create two agents
			agent1, err := shared.EstablishConnection(wsURL, apiKey)
			Expect(err).NotTo(HaveOccurred())
			defer agent1.Close(websocket.StatusNormalClosure, "")

			agent2, err := shared.EstablishConnection(wsURL, apiKey)
			Expect(err).NotTo(HaveOccurred())
			defer agent2.Close(websocket.StatusNormalClosure, "")

			// Both agents join a shared workspace
			workspaceID := uuid.New().String()

			for i, conn := range []*websocket.Conn{agent1, agent2} {
				joinMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "workspace.join",
					Params: map[string]interface{}{
						"workspace_id": workspaceID,
						"agent_name":   fmt.Sprintf("agent-%d", i+1),
					},
				}

				err = wsjson.Write(ctx, conn, joinMsg)
				Expect(err).NotTo(HaveOccurred())

				var resp ws.Message
				err = wsjson.Read(ctx, conn, &resp)
				Expect(err).NotTo(HaveOccurred())
			}

			// Agent 1 updates shared state
			updateMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workspace.update_state",
				Params: map[string]interface{}{
					"workspace_id": workspaceID,
					"updates": map[string]interface{}{
						"current_task":      "implementing feature X",
						"progress":          25,
						"discovered_issues": []string{"issue1", "issue2"},
					},
				},
			}

			err = wsjson.Write(ctx, agent1, updateMsg)
			Expect(err).NotTo(HaveOccurred())

			// Agent 2 should receive state update
			stateReceived := false
			timeout := time.After(2 * time.Second)

			for !stateReceived {
				select {
				case <-timeout:
					Fail("Timeout waiting for state update")
				default:
					var msg ws.Message
					err := wsjson.Read(ctx, agent2, &msg)
					if err != nil {
						continue
					}

					if msg.Type == ws.MessageTypeNotification && msg.Method == "workspace.state_changed" {
						if params, ok := msg.Params.(map[string]interface{}); ok {
							if updates, ok := params["updates"].(map[string]interface{}); ok {
								Expect(updates["current_task"]).To(Equal("implementing feature X"))
								Expect(updates["progress"]).To(Equal(float64(25)))
								stateReceived = true
							}
						}
					}
				}
			}

			// Agent 2 adds to shared state
			addMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workspace.update_state",
				Params: map[string]interface{}{
					"workspace_id": workspaceID,
					"updates": map[string]interface{}{
						"progress":          50,
						"discovered_issues": []string{"issue3"}, // Should append
					},
					"merge_strategy": "append_arrays",
				},
			}

			err = wsjson.Write(ctx, agent2, addMsg)
			Expect(err).NotTo(HaveOccurred())

			// Get final state
			getStateMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "workspace.get_state",
				Params: map[string]interface{}{
					"workspace_id": workspaceID,
				},
			}

			err = wsjson.Write(ctx, agent1, getStateMsg)
			Expect(err).NotTo(HaveOccurred())

			var stateResp ws.Message
			err = wsjson.Read(ctx, agent1, &stateResp)
			Expect(err).NotTo(HaveOccurred())

			// Verify merged state
			if result, ok := stateResp.Result.(map[string]interface{}); ok {
				if state, ok := result["state"].(map[string]interface{}); ok {
					Expect(state["progress"]).To(Equal(float64(50)))

					if issues, ok := state["discovered_issues"].([]interface{}); ok {
						Expect(len(issues)).To(Equal(3)) // All issues combined
					}
				}
			}
		})

		It("should handle conflicting updates", func() {
			// Create two agents
			agent1, err := shared.EstablishConnection(wsURL, apiKey)
			Expect(err).NotTo(HaveOccurred())
			defer agent1.Close(websocket.StatusNormalClosure, "")

			agent2, err := shared.EstablishConnection(wsURL, apiKey)
			Expect(err).NotTo(HaveOccurred())
			defer agent2.Close(websocket.StatusNormalClosure, "")

			// Create shared document
			docID := uuid.New().String()
			createDocMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "document.create_shared",
				Params: map[string]interface{}{
					"document_id": docID,
					"content":     "Initial content",
					"version":     1,
				},
			}

			err = wsjson.Write(ctx, agent1, createDocMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, agent1, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Both agents try to update simultaneously
			var wg sync.WaitGroup
			conflicts := make(chan bool, 2)

			for i, conn := range []*websocket.Conn{agent1, agent2} {
				wg.Add(1)
				go func(idx int, c *websocket.Conn) {
					defer wg.Done()

					updateMsg := ws.Message{
						ID:     uuid.New().String(),
						Type:   ws.MessageTypeRequest,
						Method: "document.update",
						Params: map[string]interface{}{
							"document_id": docID,
							"content":     fmt.Sprintf("Updated by agent %d", idx+1),
							"version":     1, // Same version - will conflict
						},
					}

					err := wsjson.Write(ctx, c, updateMsg)
					Expect(err).NotTo(HaveOccurred())

					var resp ws.Message
					err = wsjson.Read(ctx, c, &resp)
					Expect(err).NotTo(HaveOccurred())

					if resp.Error != nil && resp.Error.Code == ws.ErrCodeConflict {
						conflicts <- true
					} else {
						conflicts <- false
					}
				}(i, conn)
			}

			wg.Wait()
			close(conflicts)

			// One should succeed, one should get conflict
			successCount := 0
			conflictCount := 0

			for result := range conflicts {
				if result {
					conflictCount++
				} else {
					successCount++
				}
			}

			Expect(successCount).To(Equal(1), "One update should succeed")
			Expect(conflictCount).To(Equal(1), "One update should conflict")
		})
	})
})
