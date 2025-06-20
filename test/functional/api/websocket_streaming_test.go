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

var _ = Describe("WebSocket Streaming Operations", func() {
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

	Describe("Streaming Tool Responses", func() {
		It("should handle incremental output delivery from tools", func() {
			// Execute a long-running tool that streams results
			execMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "tool.execute",
				Params: map[string]interface{}{
					"name": "long_running_analysis",
					"arguments": map[string]interface{}{
						"data":  shared.GenerateLargeContext(1000), // 1000 tokens
						"depth": "deep",
					},
					"stream": true, // Enable streaming
				},
			}

			err := wsjson.Write(ctx, conn, execMsg)
			Expect(err).NotTo(HaveOccurred())

			// Collect progress updates
			progressUpdates := make([]int, 0)
			var finalResult interface{}
			completed := false

			// Read messages until completion
			for !completed {
				var msg ws.Message
				err := wsjson.Read(ctx, conn, &msg)
				Expect(err).NotTo(HaveOccurred())

				switch msg.Type {
				case ws.MessageTypeNotification:
					if msg.Method == "tool.progress" {
						if params, ok := msg.Params.(map[string]interface{}); ok {
							if progress, ok := params["percentage"].(float64); ok {
								progressUpdates = append(progressUpdates, int(progress))
								GinkgoWriter.Printf("Progress: %d%%\n", int(progress))
							}
						}
					}
				case ws.MessageTypeResponse:
					Expect(msg.ID).To(Equal(execMsg.ID))
					Expect(msg.Error).To(BeNil())
					finalResult = msg.Result
					completed = true
				case ws.MessageTypeError:
					Fail(fmt.Sprintf("Received error: %v", msg.Error))
				}
			}

			// Verify progress updates
			Expect(len(progressUpdates)).To(BeNumerically(">", 0), "Should receive progress updates")
			Expect(progressUpdates[len(progressUpdates)-1]).To(Equal(100), "Final progress should be 100%")

			// Verify progress is monotonic
			for i := 1; i < len(progressUpdates); i++ {
				Expect(progressUpdates[i]).To(BeNumerically(">=", progressUpdates[i-1]))
			}

			// Verify final result
			Expect(finalResult).NotTo(BeNil())
			if result, ok := finalResult.(map[string]interface{}); ok {
				Expect(result).To(HaveKey("findings"))
				Expect(result).To(HaveKey("risk_score"))
				Expect(result).To(HaveKey("duration_ms"))
			}
		})

		It("should support cancellation of in-progress operations", func() {
			// Start a long-running operation
			execMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "tool.execute",
				Params: map[string]interface{}{
					"name": "file_processor",
					"arguments": map[string]interface{}{
						"file_path": "/large/file.dat",
						"operation": "analyze",
					},
					"stream": true,
				},
			}

			err := wsjson.Write(ctx, conn, execMsg)
			Expect(err).NotTo(HaveOccurred())

			// Wait for first progress update
			progressReceived := false
			for !progressReceived {
				var msg ws.Message
				err := wsjson.Read(ctx, conn, &msg)
				Expect(err).NotTo(HaveOccurred())

				if msg.Type == ws.MessageTypeNotification && msg.Method == "tool.progress" {
					progressReceived = true

					// Send cancellation
					cancelMsg := ws.Message{
						ID:     uuid.New().String(),
						Type:   ws.MessageTypeRequest,
						Method: "tool.cancel",
						Params: map[string]interface{}{
							"operation_id": execMsg.ID,
						},
					}

					err = wsjson.Write(ctx, conn, cancelMsg)
					Expect(err).NotTo(HaveOccurred())
				}
			}

			// Verify cancellation response
			cancelled := false
			for !cancelled {
				var msg ws.Message
				err := wsjson.Read(ctx, conn, &msg)
				Expect(err).NotTo(HaveOccurred())

				if msg.Type == ws.MessageTypeResponse || msg.Type == ws.MessageTypeError {
					if msg.Error != nil && msg.Error.Code == ws.ErrCodeOperationCancelled {
						cancelled = true
					} else if result, ok := msg.Result.(map[string]interface{}); ok {
						if status, ok := result["status"].(string); ok && status == "cancelled" {
							cancelled = true
						}
					}
				}
			}

			Expect(cancelled).To(BeTrue(), "Operation should be cancelled")
		})

		It("should handle error during streaming gracefully", func() {
			// Execute a tool that will fail mid-stream
			execMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "tool.execute",
				Params: map[string]interface{}{
					"name": "workflow_executor",
					"arguments": map[string]interface{}{
						"step": "fail", // Special step that triggers failure
					},
					"stream": true,
				},
			}

			err := wsjson.Write(ctx, conn, execMsg)
			Expect(err).NotTo(HaveOccurred())

			// Read response
			var errorReceived bool
			for !errorReceived {
				var msg ws.Message
				err := wsjson.Read(ctx, conn, &msg)
				Expect(err).NotTo(HaveOccurred())

				if msg.Type == ws.MessageTypeError ||
					(msg.Type == ws.MessageTypeResponse && msg.Error != nil) {
					errorReceived = true
					Expect(msg.Error).NotTo(BeNil())
					Expect(msg.Error.Message).To(ContainSubstring("fail"))
				}
			}
		})
	})

	Describe("Large Context Streaming", func() {
		It("should stream large context chunks efficiently", func() {
			// Create a very large context
			largeContent := shared.GenerateLargeContext(10000) // 10K tokens

			createMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "context.create",
				Params: map[string]interface{}{
					"name":    "large-context",
					"content": largeContent,
					"stream":  true, // Stream the content
				},
			}

			startTime := time.Now()
			err := wsjson.Write(ctx, conn, createMsg)
			Expect(err).NotTo(HaveOccurred())

			// Track chunks received
			chunks := 0
			totalBytes := 0
			var contextID string

			for {
				var msg ws.Message
				err := wsjson.Read(ctx, conn, &msg)
				Expect(err).NotTo(HaveOccurred())

				if msg.Type == ws.MessageTypeNotification && msg.Method == "context.chunk" {
					chunks++
					if params, ok := msg.Params.(map[string]interface{}); ok {
						if chunkSize, ok := params["bytes"].(float64); ok {
							totalBytes += int(chunkSize)
						}
					}
				} else if msg.Type == ws.MessageTypeResponse {
					Expect(msg.ID).To(Equal(createMsg.ID))
					Expect(msg.Error).To(BeNil())

					if result, ok := msg.Result.(map[string]interface{}); ok {
						contextID = result["id"].(string)
					}
					break
				}
			}

			duration := time.Since(startTime)
			throughput := float64(totalBytes) / duration.Seconds() / 1024 / 1024 // MB/s

			GinkgoWriter.Printf("Streamed %d bytes in %d chunks over %v (%.2f MB/s)\n",
				totalBytes, chunks, duration, throughput)

			// Verify streaming was used
			Expect(chunks).To(BeNumerically(">", 1), "Should receive multiple chunks")
			Expect(totalBytes).To(BeNumerically(">=", len(largeContent)))
			Expect(contextID).NotTo(BeEmpty())

			// Verify throughput is reasonable
			Expect(throughput).To(BeNumerically(">", 1.0), "Should achieve >1 MB/s throughput")
		})

		It("should support binary protocol for efficient transfer", func() {
			Skip("Binary protocol implementation pending")

			// This test would verify binary encoding reduces payload size
			// and improves throughput compared to JSON
		})

		It("should apply compression for large payloads", func() {
			// Create highly compressible content
			repetitiveContent := ""
			for i := 0; i < 1000; i++ {
				repetitiveContent += "This is a repeating pattern that should compress well. "
			}

			createMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "context.create",
				Params: map[string]interface{}{
					"name":     "compressible-context",
					"content":  repetitiveContent,
					"compress": true,
				},
			}

			err := wsjson.Write(ctx, conn, createMsg)
			Expect(err).NotTo(HaveOccurred())

			var response ws.Message
			err = wsjson.Read(ctx, conn, &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response.Error).To(BeNil())

			// Check compression stats if available
			if result, ok := response.Result.(map[string]interface{}); ok {
				if stats, ok := result["compression_stats"].(map[string]interface{}); ok {
					originalSize := stats["original_size"].(float64)
					compressedSize := stats["compressed_size"].(float64)
					ratio := compressedSize / originalSize

					GinkgoWriter.Printf("Compression ratio: %.2f%% (%.0f -> %.0f bytes)\n",
						ratio*100, originalSize, compressedSize)

					Expect(ratio).To(BeNumerically("<", 0.5), "Should achieve >50% compression")
				}
			}
		})
	})

	Describe("Progress Notifications", func() {
		It("should provide detailed progress updates", func() {
			execMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "tool.execute",
				Params: map[string]interface{}{
					"name": "test_runner",
					"arguments": map[string]interface{}{
						"test_suite": "integration",
						"parallel":   true,
					},
					"stream":          true,
					"progress_detail": true, // Request detailed progress
				},
			}

			err := wsjson.Write(ctx, conn, execMsg)
			Expect(err).NotTo(HaveOccurred())

			// Collect all progress notifications
			progressEvents := make([]map[string]interface{}, 0)

			for {
				var msg ws.Message
				err := wsjson.Read(ctx, conn, &msg)
				Expect(err).NotTo(HaveOccurred())

				if msg.Type == ws.MessageTypeNotification && msg.Method == "tool.progress" {
					if params, ok := msg.Params.(map[string]interface{}); ok {
						progressEvents = append(progressEvents, params)
					}
				} else if msg.Type == ws.MessageTypeResponse {
					break
				}
			}

			// Verify detailed progress information
			Expect(len(progressEvents)).To(BeNumerically(">", 0))

			for _, event := range progressEvents {
				Expect(event).To(HaveKey("percentage"))
				Expect(event).To(HaveKey("message"))
				Expect(event).To(HaveKey("timestamp"))

				// Detailed progress should include additional fields
				if event["percentage"].(float64) > 0 {
					Expect(event).To(HaveKey("current_operation"))
					Expect(event).To(HaveKey("estimated_time_remaining"))
				}
			}
		})

		It("should support backpressure for slow consumers", func() {
			// Execute a tool that generates many updates quickly
			execMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "tool.execute",
				Params: map[string]interface{}{
					"name": "log_streamer",
					"arguments": map[string]interface{}{
						"source": "high-volume-app",
						"follow": true,
					},
					"stream":            true,
					"max_buffer_size":   10, // Small buffer to test backpressure
					"backpressure_mode": "drop_oldest",
				},
			}

			err := wsjson.Write(ctx, conn, execMsg)
			Expect(err).NotTo(HaveOccurred())

			// Simulate slow consumer by adding delays
			messagesReceived := 0
			droppedMessages := 0

			timeout := time.After(5 * time.Second)
			for {
				select {
				case <-timeout:
					goto done
				default:
					// Slow read
					time.Sleep(100 * time.Millisecond)

					var msg ws.Message
					err := wsjson.Read(ctx, conn, &msg)
					if err != nil {
						goto done
					}

					if msg.Type == ws.MessageTypeNotification {
						messagesReceived++

						if params, ok := msg.Params.(map[string]interface{}); ok {
							if dropped, ok := params["messages_dropped"].(float64); ok {
								droppedMessages = int(dropped)
							}
						}
					}
				}
			}
		done:

			GinkgoWriter.Printf("Received %d messages, %d dropped due to backpressure\n",
				messagesReceived, droppedMessages)

			// Verify backpressure handling
			Expect(messagesReceived).To(BeNumerically(">", 0))
			Expect(droppedMessages).To(BeNumerically(">", 0), "Should drop some messages due to slow consumer")
		})
	})

	Describe("Concurrent Streaming Operations", func() {
		It("should handle multiple concurrent streaming operations", func() {
			numOperations := 3
			var wg sync.WaitGroup
			errors := make(chan error, numOperations)

			for i := 0; i < numOperations; i++ {
				wg.Add(1)
				go func(index int) {
					defer wg.Done()

					// Create separate connection for each operation
					conn, err := shared.EstablishConnection(wsURL, apiKey)
					if err != nil {
						errors <- err
						return
					}
					defer conn.Close(websocket.StatusNormalClosure, "")

					// Execute streaming operation
					execMsg := ws.Message{
						ID:     fmt.Sprintf("op-%d-%s", index, uuid.New().String()),
						Type:   ws.MessageTypeRequest,
						Method: "tool.execute",
						Params: map[string]interface{}{
							"name": "long_running_analysis",
							"arguments": map[string]interface{}{
								"data":  fmt.Sprintf("Data set %d", index),
								"depth": "medium",
							},
							"stream": true,
						},
					}

					if err := wsjson.Write(context.Background(), conn, execMsg); err != nil {
						errors <- err
						return
					}

					// Read until completion
					progressCount := 0
					for {
						var msg ws.Message
						if err := wsjson.Read(context.Background(), conn, &msg); err != nil {
							errors <- err
							return
						}

						if msg.Type == ws.MessageTypeNotification {
							progressCount++
						} else if msg.Type == ws.MessageTypeResponse {
							if msg.Error != nil {
								errors <- fmt.Errorf("operation %d failed: %v", index, msg.Error)
							}
							break
						}
					}

					GinkgoWriter.Printf("Operation %d received %d progress updates\n", index, progressCount)
				}(i)
			}

			wg.Wait()
			close(errors)

			// Check for errors
			for err := range errors {
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})
})
