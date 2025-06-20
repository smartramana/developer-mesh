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

var _ = Describe("WebSocket Real-Time Subscriptions", func() {
	var (
		conn   *websocket.Conn
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

	Describe("Event Subscriptions", func() {
		It("should subscribe to tool execution events", func() {
			// Subscribe to tool events
			subMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "subscribe",
				Params: map[string]interface{}{
					"resource": "tool.events",
					"filter": map[string]interface{}{
						"tool_name": "test_runner",
					},
				},
			}

			err := wsjson.Write(ctx, conn, subMsg)
			Expect(err).NotTo(HaveOccurred())

			var subResp ws.Message
			err = wsjson.Read(ctx, conn, &subResp)
			Expect(err).NotTo(HaveOccurred())
			Expect(subResp.Error).To(BeNil())

			subscriptionID := ""
			if result, ok := subResp.Result.(map[string]interface{}); ok {
				subscriptionID = result["subscription_id"].(string)
			}
			Expect(subscriptionID).NotTo(BeEmpty())

			// Execute tool to generate events
			toolMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "tool.execute",
				Params: map[string]interface{}{
					"name": "test_runner",
					"arguments": map[string]interface{}{
						"test_suite": "unit",
					},
				},
			}

			err = wsjson.Write(ctx, conn, toolMsg)
			Expect(err).NotTo(HaveOccurred())

			// Collect events
			events := make([]map[string]interface{}, 0)
			toolCompleted := false

			timeout := time.After(5 * time.Second)
			for !toolCompleted {
				select {
				case <-timeout:
					toolCompleted = true
				default:
					var msg ws.Message
					err := wsjson.Read(ctx, conn, &msg)
					if err != nil {
						continue
					}

					if msg.Type == ws.MessageTypeNotification && msg.Method == "subscription.event" {
						if params, ok := msg.Params.(map[string]interface{}); ok {
							if params["subscription_id"] == subscriptionID {
								if event, ok := params["event"].(map[string]interface{}); ok {
									events = append(events, event)
								}
							}
						}
					} else if msg.Type == ws.MessageTypeResponse && msg.ID == toolMsg.ID {
						toolCompleted = true
					}
				}
			}

			// Verify events
			Expect(len(events)).To(BeNumerically(">", 0), "Should receive subscription events")

			// Check event types
			eventTypes := make(map[string]int)
			for _, event := range events {
				if eventType, ok := event["type"].(string); ok {
					eventTypes[eventType]++
				}
			}

			Expect(eventTypes).To(HaveKey("tool.started"))
			Expect(eventTypes).To(HaveKey("tool.completed"))

			// Unsubscribe
			unsubMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "unsubscribe",
				Params: map[string]interface{}{
					"subscription_id": subscriptionID,
				},
			}

			err = wsjson.Write(ctx, conn, unsubMsg)
			Expect(err).NotTo(HaveOccurred())

			var unsubResp ws.Message
			err = wsjson.Read(ctx, conn, &unsubResp)
			Expect(err).NotTo(HaveOccurred())
			Expect(unsubResp.Error).To(BeNil())
		})

		It("should support wildcard subscriptions", func() {
			// Subscribe to all tool events
			subMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "subscribe",
				Params: map[string]interface{}{
					"resource": "tool.events",
					"filter": map[string]interface{}{
						"tool_name": "*", // Wildcard
					},
				},
			}

			err := wsjson.Write(ctx, conn, subMsg)
			Expect(err).NotTo(HaveOccurred())

			var subResp ws.Message
			err = wsjson.Read(ctx, conn, &subResp)
			Expect(err).NotTo(HaveOccurred())

			subscriptionID := ""
			if result, ok := subResp.Result.(map[string]interface{}); ok {
				subscriptionID = result["subscription_id"].(string)
			}

			// Execute different tools
			tools := []string{"test_runner", "code_reviewer", "data_transformer"}
			eventsPerTool := make(map[string]int)

			for _, tool := range tools {
				toolMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "tool.execute",
					Params: map[string]interface{}{
						"name": tool,
						"arguments": map[string]interface{}{
							"input": "test data",
						},
					},
				}

				err = wsjson.Write(ctx, conn, toolMsg)
				Expect(err).NotTo(HaveOccurred())
			}

			// Collect events
			timeout := time.After(3 * time.Second)
			for {
				select {
				case <-timeout:
					goto done
				default:
					var msg ws.Message
					err := wsjson.Read(ctx, conn, &msg)
					if err != nil {
						continue
					}

					if msg.Type == ws.MessageTypeNotification && msg.Method == "subscription.event" {
						if params, ok := msg.Params.(map[string]interface{}); ok {
							if params["subscription_id"] == subscriptionID {
								if event, ok := params["event"].(map[string]interface{}); ok {
									if toolName, ok := event["tool_name"].(string); ok {
										eventsPerTool[toolName]++
									}
								}
							}
						}
					}
				}
			}
		done:

			// Verify received events from all tools
			for _, tool := range tools {
				Expect(eventsPerTool[tool]).To(BeNumerically(">", 0),
					fmt.Sprintf("Should receive events for tool %s", tool))
			}
		})

		It("should handle multiple concurrent subscriptions", func() {
			// Create multiple subscriptions
			subscriptions := []struct {
				resource string
				filter   map[string]interface{}
				id       string
			}{
				{
					resource: "tool.events",
					filter:   map[string]interface{}{"tool_name": "test_runner"},
				},
				{
					resource: "context.events",
					filter:   map[string]interface{}{"event_type": "truncation"},
				},
				{
					resource: "workflow.events",
					filter:   map[string]interface{}{"workflow_id": "*"},
				},
			}

			// Subscribe to all
			for i := range subscriptions {
				subMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "subscribe",
					Params: map[string]interface{}{
						"resource": subscriptions[i].resource,
						"filter":   subscriptions[i].filter,
					},
				}

				err := wsjson.Write(ctx, conn, subMsg)
				Expect(err).NotTo(HaveOccurred())

				var subResp ws.Message
				err = wsjson.Read(ctx, conn, &subResp)
				Expect(err).NotTo(HaveOccurred())
				Expect(subResp.Error).To(BeNil())

				if result, ok := subResp.Result.(map[string]interface{}); ok {
					subscriptions[i].id = result["subscription_id"].(string)
				}
			}

			// Verify all subscriptions are active
			listMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "subscription.list",
			}

			err := wsjson.Write(ctx, conn, listMsg)
			Expect(err).NotTo(HaveOccurred())

			var listResp ws.Message
			err = wsjson.Read(ctx, conn, &listResp)
			Expect(err).NotTo(HaveOccurred())

			if result, ok := listResp.Result.(map[string]interface{}); ok {
				if active, ok := result["active_subscriptions"].([]interface{}); ok {
					Expect(len(active)).To(Equal(len(subscriptions)))
				}
			}

			// Clean up
			for _, sub := range subscriptions {
				unsubMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "unsubscribe",
					Params: map[string]interface{}{
						"subscription_id": sub.id,
					},
				}

				err = wsjson.Write(ctx, conn, unsubMsg)
				Expect(err).NotTo(HaveOccurred())

				var unsubResp ws.Message
				err = wsjson.Read(ctx, conn, &unsubResp)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})

	Describe("Real-Time Monitoring", func() {
		It("should stream metrics in real-time", func() {
			// Subscribe to metrics
			subMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "subscribe",
				Params: map[string]interface{}{
					"resource": "metrics",
					"filter": map[string]interface{}{
						"metric_names": []string{"cpu_usage", "memory_usage", "active_connections"},
						"interval_ms":  1000, // 1 second
					},
				},
			}

			err := wsjson.Write(ctx, conn, subMsg)
			Expect(err).NotTo(HaveOccurred())

			var subResp ws.Message
			err = wsjson.Read(ctx, conn, &subResp)
			Expect(err).NotTo(HaveOccurred())

			subscriptionID := ""
			if result, ok := subResp.Result.(map[string]interface{}); ok {
				subscriptionID = result["subscription_id"].(string)
			}

			// Collect metrics for 3 seconds
			metrics := make([]map[string]interface{}, 0)
			timeout := time.After(3500 * time.Millisecond)

			for {
				select {
				case <-timeout:
					goto done
				default:
					var msg ws.Message
					err := wsjson.Read(ctx, conn, &msg)
					if err != nil {
						continue
					}

					if msg.Type == ws.MessageTypeNotification && msg.Method == "metrics.update" {
						if params, ok := msg.Params.(map[string]interface{}); ok {
							if params["subscription_id"] == subscriptionID {
								if data, ok := params["data"].(map[string]interface{}); ok {
									metrics = append(metrics, data)
								}
							}
						}
					}
				}
			}
		done:

			// Should receive ~3 metric updates (one per second)
			Expect(len(metrics)).To(BeNumerically("~", 3, 1))

			// Verify metric structure
			for _, metric := range metrics {
				Expect(metric).To(HaveKey("timestamp"))
				Expect(metric).To(HaveKey("cpu_usage"))
				Expect(metric).To(HaveKey("memory_usage"))
				Expect(metric).To(HaveKey("active_connections"))
			}

			// Verify metrics change over time
			if len(metrics) > 1 {
				firstTime := metrics[0]["timestamp"].(float64)
				lastTime := metrics[len(metrics)-1]["timestamp"].(float64)
				Expect(lastTime - firstTime).To(BeNumerically("~", float64(len(metrics)-1)*1000, 500))
			}
		})

		It("should support rate limiting for high-frequency events", func() {
			// Subscribe with rate limiting
			subMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "subscribe",
				Params: map[string]interface{}{
					"resource": "log.stream",
					"filter": map[string]interface{}{
						"source": "high-volume-app",
					},
					"rate_limit": map[string]interface{}{
						"max_per_second": 10,
						"burst":          20,
					},
				},
			}

			err := wsjson.Write(ctx, conn, subMsg)
			Expect(err).NotTo(HaveOccurred())

			var subResp ws.Message
			err = wsjson.Read(ctx, conn, &subResp)
			Expect(err).NotTo(HaveOccurred())

			// Trigger high-frequency events
			triggerMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "test.generate_events",
				Params: map[string]interface{}{
					"event_type":       "log.stream",
					"source":           "high-volume-app",
					"rate_per_second":  100, // 100 events/sec
					"duration_seconds": 2,
				},
			}

			err = wsjson.Write(ctx, conn, triggerMsg)
			Expect(err).NotTo(HaveOccurred())

			// Count received events
			eventsReceived := 0
			start := time.Now()
			timeout := time.After(3 * time.Second)

			for {
				select {
				case <-timeout:
					goto done
				default:
					var msg ws.Message
					err := wsjson.Read(ctx, conn, &msg)
					if err != nil {
						continue
					}

					if msg.Type == ws.MessageTypeNotification && msg.Method == "subscription.event" {
						eventsReceived++
					}
				}
			}
		done:

			duration := time.Since(start).Seconds()
			eventsPerSecond := float64(eventsReceived) / duration

			GinkgoWriter.Printf("Received %d events in %.1fs (%.1f/sec)\n",
				eventsReceived, duration, eventsPerSecond)

			// Should be rate limited to ~10/sec (with some burst)
			Expect(eventsPerSecond).To(BeNumerically("<=", 15))
			Expect(eventsPerSecond).To(BeNumerically(">=", 8))
		})
	})

	Describe("Collaborative Features", func() {
		It("should broadcast agent status changes", func() {
			// Create two connections (two agents)
			conn2, err := shared.EstablishConnection(wsURL, apiKey)
			Expect(err).NotTo(HaveOccurred())
			defer conn2.Close(websocket.StatusNormalClosure, "")

			// Agent 1 subscribes to agent status
			subMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "subscribe",
				Params: map[string]interface{}{
					"resource": "agent.status",
					"filter": map[string]interface{}{
						"include_self": false,
					},
				},
			}

			err = wsjson.Write(ctx, conn, subMsg)
			Expect(err).NotTo(HaveOccurred())

			var subResp ws.Message
			err = wsjson.Read(ctx, conn, &subResp)
			Expect(err).NotTo(HaveOccurred())

			// Agent 2 updates its status
			statusMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "agent.update_status",
				Params: map[string]interface{}{
					"status": "busy",
					"activity": map[string]interface{}{
						"type":       "tool_execution",
						"tool":       "code_reviewer",
						"started_at": time.Now().Format(time.RFC3339),
					},
				},
			}

			err = wsjson.Write(ctx, conn2, statusMsg)
			Expect(err).NotTo(HaveOccurred())

			// Agent 1 should receive the status update
			statusReceived := false
			timeout := time.After(2 * time.Second)

			for !statusReceived {
				select {
				case <-timeout:
					Fail("Timeout waiting for status update")
				default:
					var msg ws.Message
					err := wsjson.Read(ctx, conn, &msg)
					if err != nil {
						continue
					}

					if msg.Type == ws.MessageTypeNotification && msg.Method == "agent.status_changed" {
						if params, ok := msg.Params.(map[string]interface{}); ok {
							Expect(params["status"]).To(Equal("busy"))
							Expect(params).To(HaveKey("activity"))
							statusReceived = true
						}
					}
				}
			}
		})

		It("should support shared document subscriptions", func() {
			documentID := uuid.New().String()

			// Subscribe to document changes
			subMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "subscribe",
				Params: map[string]interface{}{
					"resource": "document.changes",
					"filter": map[string]interface{}{
						"document_id": documentID,
					},
				},
			}

			err := wsjson.Write(ctx, conn, subMsg)
			Expect(err).NotTo(HaveOccurred())

			var subResp ws.Message
			err = wsjson.Read(ctx, conn, &subResp)
			Expect(err).NotTo(HaveOccurred())

			// Make document changes
			changes := []map[string]interface{}{
				{
					"operation": "insert",
					"position":  0,
					"text":      "Hello, ",
				},
				{
					"operation": "append",
					"text":      "world!",
				},
				{
					"operation": "replace",
					"start":     7,
					"end":       12,
					"text":      "AI agents",
				},
			}

			receivedChanges := make([]map[string]interface{}, 0)
			var wg sync.WaitGroup

			// Start change listener
			wg.Add(1)
			go func() {
				defer wg.Done()
				timeout := time.After(3 * time.Second)
				for {
					select {
					case <-timeout:
						return
					default:
						var msg ws.Message
						err := wsjson.Read(ctx, conn, &msg)
						if err != nil {
							continue
						}

						if msg.Type == ws.MessageTypeNotification && msg.Method == "document.changed" {
							if params, ok := msg.Params.(map[string]interface{}); ok {
								if change, ok := params["change"].(map[string]interface{}); ok {
									receivedChanges = append(receivedChanges, change)
								}
							}
						}
					}
				}
			}()

			// Apply changes
			for _, change := range changes {
				changeMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "document.apply_change",
					Params: map[string]interface{}{
						"document_id": documentID,
						"change":      change,
					},
				}

				err = wsjson.Write(ctx, conn, changeMsg)
				Expect(err).NotTo(HaveOccurred())

				var changeResp ws.Message
				err = wsjson.Read(ctx, conn, &changeResp)
				Expect(err).NotTo(HaveOccurred())

				time.Sleep(100 * time.Millisecond) // Allow time for propagation
			}

			wg.Wait()

			// Verify all changes were received
			Expect(len(receivedChanges)).To(Equal(len(changes)))
			for i, change := range receivedChanges {
				Expect(change["operation"]).To(Equal(changes[i]["operation"]))
			}
		})
	})

	Describe("Event Filtering and Aggregation", func() {
		It("should support complex event filters", func() {
			// Subscribe with complex filter
			subMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "subscribe",
				Params: map[string]interface{}{
					"resource": "tool.events",
					"filter": map[string]interface{}{
						"$or": []map[string]interface{}{
							{
								"$and": []map[string]interface{}{
									{"tool_name": "test_runner"},
									{"event_type": "completed"},
									{"result.failed": map[string]interface{}{"$gt": 0}},
								},
							},
							{
								"$and": []map[string]interface{}{
									{"tool_name": "deployment_tool"},
									{"environment": "prod"},
								},
							},
						},
					},
				},
			}

			err := wsjson.Write(ctx, conn, subMsg)
			Expect(err).NotTo(HaveOccurred())

			var subResp ws.Message
			err = wsjson.Read(ctx, conn, &subResp)
			Expect(err).NotTo(HaveOccurred())
			Expect(subResp.Error).To(BeNil())

			// The subscription should only receive events matching the complex filter
		})

		It("should aggregate events over time windows", func() {
			// Subscribe with aggregation
			subMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "subscribe",
				Params: map[string]interface{}{
					"resource": "metrics.aggregated",
					"filter": map[string]interface{}{
						"metrics": []string{"request_count", "error_count", "response_time"},
					},
					"aggregation": map[string]interface{}{
						"window_ms": 5000, // 5 second windows
						"functions": []string{"sum", "avg", "max", "min", "count"},
						"group_by":  []string{"endpoint", "method"},
					},
				},
			}

			err := wsjson.Write(ctx, conn, subMsg)
			Expect(err).NotTo(HaveOccurred())

			var subResp ws.Message
			err = wsjson.Read(ctx, conn, &subResp)
			Expect(err).NotTo(HaveOccurred())

			// Generate some metrics
			for i := 0; i < 10; i++ {
				metricMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "metrics.record",
					Params: map[string]interface{}{
						"metric":    "request_count",
						"value":     1,
						"endpoint":  "/api/tools",
						"method":    "POST",
						"timestamp": time.Now().UnixMilli(),
					},
				}

				err = wsjson.Write(ctx, conn, metricMsg)
				Expect(err).NotTo(HaveOccurred())

				var metricResp ws.Message
				err = wsjson.Read(ctx, conn, &metricResp)
				Expect(err).NotTo(HaveOccurred())

				time.Sleep(500 * time.Millisecond)
			}

			// Wait for aggregated results
			aggregatedReceived := false
			timeout := time.After(6 * time.Second)

			for !aggregatedReceived {
				select {
				case <-timeout:
					Fail("Timeout waiting for aggregated metrics")
				default:
					var msg ws.Message
					err := wsjson.Read(ctx, conn, &msg)
					if err != nil {
						continue
					}

					if msg.Type == ws.MessageTypeNotification && msg.Method == "metrics.aggregated" {
						if params, ok := msg.Params.(map[string]interface{}); ok {
							if agg, ok := params["aggregation"].(map[string]interface{}); ok {
								Expect(agg).To(HaveKey("window_start"))
								Expect(agg).To(HaveKey("window_end"))
								Expect(agg).To(HaveKey("groups"))

								if groups, ok := agg["groups"].([]interface{}); ok {
									Expect(len(groups)).To(BeNumerically(">", 0))
									for _, group := range groups {
										if g, ok := group.(map[string]interface{}); ok {
											Expect(g).To(HaveKey("endpoint"))
											Expect(g).To(HaveKey("method"))
											Expect(g).To(HaveKey("sum"))
											Expect(g).To(HaveKey("count"))
										}
									}
								}

								aggregatedReceived = true
							}
						}
					}
				}
			}
		})
	})

	Describe("Subscription Lifecycle", func() {
		It("should handle connection loss gracefully", func() {
			// Subscribe to events
			subMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "subscribe",
				Params: map[string]interface{}{
					"resource":   "tool.events",
					"filter":     map[string]interface{}{},
					"persistent": true, // Survive reconnection
				},
			}

			err := wsjson.Write(ctx, conn, subMsg)
			Expect(err).NotTo(HaveOccurred())

			var subResp ws.Message
			err = wsjson.Read(ctx, conn, &subResp)
			Expect(err).NotTo(HaveOccurred())

			subscriptionID := ""
			if result, ok := subResp.Result.(map[string]interface{}); ok {
				subscriptionID = result["subscription_id"].(string)
			}

			// Simulate connection interruption
			conn.Close(websocket.StatusGoingAway, "simulating disconnect")

			// Reconnect
			time.Sleep(500 * time.Millisecond)
			newConn, err := shared.EstablishConnection(wsURL, apiKey)
			Expect(err).NotTo(HaveOccurred())
			defer newConn.Close(websocket.StatusNormalClosure, "")

			// Restore subscription
			restoreMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "subscription.restore",
				Params: map[string]interface{}{
					"subscription_id": subscriptionID,
				},
			}

			err = wsjson.Write(ctx, newConn, restoreMsg)
			Expect(err).NotTo(HaveOccurred())

			var restoreResp ws.Message
			err = wsjson.Read(ctx, newConn, &restoreResp)
			Expect(err).NotTo(HaveOccurred())

			if result, ok := restoreResp.Result.(map[string]interface{}); ok {
				Expect(result["restored"]).To(BeTrue())
				// Check if missed_events exists and is numeric
				if missedEvents, exists := result["missed_events"]; exists {
					Expect(missedEvents).To(BeAssignableToTypeOf(float64(0)))
				}
			}
		})

		It("should clean up subscriptions on timeout", func() {
			// Subscribe with short TTL
			subMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "subscribe",
				Params: map[string]interface{}{
					"resource": "test.events",
					"filter":   map[string]interface{}{},
					"ttl_ms":   2000, // 2 second TTL
				},
			}

			err := wsjson.Write(ctx, conn, subMsg)
			Expect(err).NotTo(HaveOccurred())

			var subResp ws.Message
			err = wsjson.Read(ctx, conn, &subResp)
			Expect(err).NotTo(HaveOccurred())

			subscriptionID := ""
			if result, ok := subResp.Result.(map[string]interface{}); ok {
				subscriptionID = result["subscription_id"].(string)
			}

			// Wait for TTL to expire
			time.Sleep(2500 * time.Millisecond)

			// Try to use expired subscription
			statusMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "subscription.status",
				Params: map[string]interface{}{
					"subscription_id": subscriptionID,
				},
			}

			err = wsjson.Write(ctx, conn, statusMsg)
			Expect(err).NotTo(HaveOccurred())

			var statusResp ws.Message
			err = wsjson.Read(ctx, conn, &statusResp)
			Expect(err).NotTo(HaveOccurred())

			if result, ok := statusResp.Result.(map[string]interface{}); ok {
				Expect(result["active"]).To(BeFalse())
				Expect(result["reason"]).To(ContainSubstring("expired"))
			}
		})
	})
})
