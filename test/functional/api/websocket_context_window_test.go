package api_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"functional-tests/shared"

	ws "github.com/developer-mesh/developer-mesh/pkg/models/websocket"
)

var _ = Describe("WebSocket Context Window Management", func() {
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
			_ = conn.Close(websocket.StatusNormalClosure, "")
		}
		cancel()
	})

	Describe("Token Counting and Limits", func() {
		It("should report token count for messages", func() {
			// Create a context with known content
			testContent := "This is a test message for token counting. It should have a predictable token count."

			createMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "context.create",
				Params: map[string]interface{}{
					"name":         "token-test-context",
					"content":      testContent,
					"model_id":     "gpt-4", // Default model for tests
					"return_stats": true,    // Request token statistics
				},
			}

			err := wsjson.Write(ctx, conn, createMsg)
			Expect(err).NotTo(HaveOccurred())

			var response ws.Message
			err = wsjson.Read(ctx, conn, &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response.Error).To(BeNil())

			// Verify token count is returned
			result, ok := response.Result.(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(result).To(HaveKey("token_count"))

			tokenCount := int(result["token_count"].(float64))
			expectedTokens := shared.SimulateTokenCount(testContent)

			// Token count should be within reasonable range
			Expect(tokenCount).To(BeNumerically("~", expectedTokens, int(float64(expectedTokens)*0.2)))

			GinkgoWriter.Printf("Content length: %d chars, Token count: %d\n",
				len(testContent), tokenCount)
		})

		It("should enforce context token limits", func() {
			// Get current token limit
			limitMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "context.get_limits",
			}

			err := wsjson.Write(ctx, conn, limitMsg)
			Expect(err).NotTo(HaveOccurred())

			var limitResp ws.Message
			err = wsjson.Read(ctx, conn, &limitResp)
			Expect(err).NotTo(HaveOccurred())

			maxTokens := 4096 // Default limit
			if limitResp.Result != nil {
				if result, ok := limitResp.Result.(map[string]interface{}); ok {
					if limit, ok := result["max_tokens"].(float64); ok {
						maxTokens = int(limit)
					}
				}
			}

			// Try to create context exceeding limit
			largeContent := shared.GenerateLargeContext(maxTokens + 1000)

			createMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "context.create",
				Params: map[string]interface{}{
					"name":     "oversized-context",
					"content":  largeContent,
					"model_id": "gpt-4", // Default model for tests
				},
			}

			err = wsjson.Write(ctx, conn, createMsg)
			Expect(err).NotTo(HaveOccurred())

			var response ws.Message
			err = wsjson.Read(ctx, conn, &response)
			Expect(err).NotTo(HaveOccurred())

			// Should either error or truncate
			if response.Error != nil {
				Expect(response.Error.Code).To(Equal(ws.ErrCodeContextTooLarge))
			} else {
				// Check if truncation occurred
				result, ok := response.Result.(map[string]interface{})
				Expect(ok).To(BeTrue())
				Expect(result).To(HaveKey("truncated"))
				Expect(result["truncated"].(bool)).To(BeTrue())
			}
		})

		It("should provide token count for conversation history", func() {
			contextID := uuid.New().String()

			// Create initial context
			createMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "context.create",
				Params: map[string]interface{}{
					"id":       contextID,
					"name":     "conversation-context",
					"content":  "System: You are a helpful assistant.",
					"model_id": "gpt-4", // Default model for tests
				},
			}

			err := wsjson.Write(ctx, conn, createMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Add multiple messages
			messages := []string{
				"User: What is the weather today?",
				"Assistant: I don't have access to current weather data.",
				"User: Can you help me write code?",
				"Assistant: Yes, I'd be happy to help you write code!",
			}

			for _, msg := range messages {
				appendMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "context.append",
					Params: map[string]interface{}{
						"context_id": contextID,
						"content":    msg,
					},
				}

				err = wsjson.Write(ctx, conn, appendMsg)
				Expect(err).NotTo(HaveOccurred())

				var appendResp ws.Message
				err = wsjson.Read(ctx, conn, &appendResp)
				Expect(err).NotTo(HaveOccurred())
			}

			// Get token statistics
			statsMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "context.get_stats",
				Params: map[string]interface{}{
					"context_id": contextID,
				},
			}

			err = wsjson.Write(ctx, conn, statsMsg)
			Expect(err).NotTo(HaveOccurred())

			var statsResp ws.Message
			err = wsjson.Read(ctx, conn, &statsResp)
			Expect(err).NotTo(HaveOccurred())

			// Verify statistics
			stats, ok := statsResp.Result.(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(stats).To(HaveKey("total_tokens"))
			Expect(stats).To(HaveKey("message_count"))
			Expect(stats).To(HaveKey("tokens_by_role"))

			GinkgoWriter.Printf("Context stats: %+v\n", stats)
		})
	})

	Describe("Context Truncation", func() {
		It("should send truncation signals when approaching limits", func() {
			contextID := uuid.New().String()
			maxTokens := 1000 // Small limit for testing

			// Create context with window monitoring
			createMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "context.create",
				Params: map[string]interface{}{
					"id":               contextID,
					"name":             "truncation-test",
					"content":          "System: Test assistant",
					"model_id":         "gpt-4", // Default model for tests
					"max_tokens":       maxTokens,
					"truncation_mode":  "sliding_window",
					"notify_threshold": 0.8, // Notify at 80% capacity
				},
			}

			err := wsjson.Write(ctx, conn, createMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Subscribe to context events
			subMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "subscribe",
				Params: map[string]interface{}{
					"resource": "context.events",
					"filter": map[string]interface{}{
						"context_id": contextID,
					},
				},
			}

			err = wsjson.Write(ctx, conn, subMsg)
			Expect(err).NotTo(HaveOccurred())

			var subResp ws.Message
			err = wsjson.Read(ctx, conn, &subResp)
			Expect(err).NotTo(HaveOccurred())

			// Add messages until we approach limit
			truncationWarningReceived := false
			currentTokens := 0

			for i := 0; currentTokens < maxTokens && !truncationWarningReceived; i++ {
				message := fmt.Sprintf("Message %d: %s", i, strings.Repeat("content ", 20))

				appendMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "context.append",
					Params: map[string]interface{}{
						"context_id": contextID,
						"content":    message,
					},
				}

				err = wsjson.Write(ctx, conn, appendMsg)
				Expect(err).NotTo(HaveOccurred())

				// Check for truncation warning
				timeout := time.After(100 * time.Millisecond)
				for {
					select {
					case <-timeout:
						goto next
					default:
						var msg ws.Message
						err := wsjson.Read(ctx, conn, &msg)
						if err != nil {
							goto next
						}

						switch msg.Type {
						case ws.MessageTypeNotification:
							if msg.Method == "context.truncation_warning" {
								truncationWarningReceived = true
								if params, ok := msg.Params.(map[string]interface{}); ok {
									currentTokens = int(params["current_tokens"].(float64))
									GinkgoWriter.Printf("Truncation warning at %d/%d tokens\n",
										currentTokens, maxTokens)
								}
							}
						case ws.MessageTypeResponse:
							// Append response
							if result, ok := msg.Result.(map[string]interface{}); ok {
								if tokens, ok := result["total_tokens"].(float64); ok {
									currentTokens = int(tokens)
								}
							}
						}
					}
				}
			next:
			}

			Expect(truncationWarningReceived).To(BeTrue(), "Should receive truncation warning")
			Expect(currentTokens).To(BeNumerically(">=", int(float64(maxTokens)*0.8)))
		})

		It("should preserve system messages during truncation", func() {
			contextID := uuid.New().String()

			// Create context with important system message
			systemPrompt := "System: You are a specialized code review assistant. Always maintain high standards."

			createMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "context.create",
				Params: map[string]interface{}{
					"id":         contextID,
					"name":       "system-preserve-test",
					"content":    systemPrompt,
					"model_id":   "gpt-4", // Default model for tests
					"max_tokens": 500,
					"messages": []map[string]interface{}{
						{
							"role":       "system",
							"content":    systemPrompt,
							"importance": 100, // Maximum importance
						},
					},
				},
			}

			err := wsjson.Write(ctx, conn, createMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Fill context with many user/assistant messages
			for i := 0; i < 20; i++ {
				userMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "context.append",
					Params: map[string]interface{}{
						"context_id": contextID,
						"message": map[string]interface{}{
							"role":       "user",
							"content":    fmt.Sprintf("User question %d with some content", i),
							"importance": 50,
						},
					},
				}

				err = wsjson.Write(ctx, conn, userMsg)
				Expect(err).NotTo(HaveOccurred())

				var userResp ws.Message
				err = wsjson.Read(ctx, conn, &userResp)
				Expect(err).NotTo(HaveOccurred())

				assistantMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "context.append",
					Params: map[string]interface{}{
						"context_id": contextID,
						"message": map[string]interface{}{
							"role":       "assistant",
							"content":    fmt.Sprintf("Assistant response %d with detailed answer", i),
							"importance": 40,
						},
					},
				}

				err = wsjson.Write(ctx, conn, assistantMsg)
				Expect(err).NotTo(HaveOccurred())

				var assistantResp ws.Message
				err = wsjson.Read(ctx, conn, &assistantResp)
				Expect(err).NotTo(HaveOccurred())
			}

			// Get current context
			getMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "context.get",
				Params: map[string]interface{}{
					"context_id":       contextID,
					"include_messages": true,
				},
			}

			err = wsjson.Write(ctx, conn, getMsg)
			Expect(err).NotTo(HaveOccurred())

			var getResp ws.Message
			err = wsjson.Read(ctx, conn, &getResp)
			Expect(err).NotTo(HaveOccurred())

			// Verify system message is preserved
			result, ok := getResp.Result.(map[string]interface{})
			Expect(ok).To(BeTrue())

			messages, ok := result["messages"].([]interface{})
			Expect(ok).To(BeTrue())
			Expect(len(messages)).To(BeNumerically(">", 0))

			// First message should be system message
			firstMsg, ok := messages[0].(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(firstMsg["role"]).To(Equal("system"))
			Expect(firstMsg["content"]).To(ContainSubstring("specialized code review assistant"))

			// Check truncation occurred
			Expect(result).To(HaveKey("truncated"))
			Expect(result["truncated"].(bool)).To(BeTrue())
			Expect(result).To(HaveKey("messages_removed"))

			GinkgoWriter.Printf("Truncation preserved system message, removed %v messages\n",
				result["messages_removed"])
		})
	})

	Describe("Sliding Window Management", func() {
		It("should implement sliding window with overlap", func() {
			contextID := uuid.New().String()

			// Create context with sliding window
			createMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "context.create",
				Params: map[string]interface{}{
					"id":              contextID,
					"name":            "sliding-window-test",
					"model_id":        "gpt-4", // Default model for tests
					"max_tokens":      1000,
					"window_overlap":  200, // Keep 200 tokens from previous window
					"truncation_mode": "sliding_window",
				},
			}

			err := wsjson.Write(ctx, conn, createMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Track window slides
			windowSlides := 0
			previousWindowEnd := ""

			// Subscribe to window events
			subMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "subscribe",
				Params: map[string]interface{}{
					"resource": "context.window_events",
					"filter": map[string]interface{}{
						"context_id": contextID,
					},
				},
			}

			err = wsjson.Write(ctx, conn, subMsg)
			Expect(err).NotTo(HaveOccurred())

			var subResp ws.Message
			err = wsjson.Read(ctx, conn, &subResp)
			Expect(err).NotTo(HaveOccurred())

			// Add messages to trigger window slides
			go func() {
				for i := 0; i < 50; i++ {
					message := fmt.Sprintf("Message %d: %s", i, strings.Repeat("data ", 30))

					appendMsg := ws.Message{
						ID:     uuid.New().String(),
						Type:   ws.MessageTypeRequest,
						Method: "context.append",
						Params: map[string]interface{}{
							"context_id": contextID,
							"content":    message,
						},
					}

					_ = wsjson.Write(ctx, conn, appendMsg)
					time.Sleep(50 * time.Millisecond)
				}
			}()

			// Monitor window slides
			timeout := time.After(5 * time.Second)
			for windowSlides < 2 {
				select {
				case <-timeout:
					goto done
				default:
					var msg ws.Message
					err := wsjson.Read(ctx, conn, &msg)
					if err != nil {
						continue
					}

					if msg.Type == ws.MessageTypeNotification && msg.Method == "context.window_slide" {
						windowSlides++
						if params, ok := msg.Params.(map[string]interface{}); ok {
							GinkgoWriter.Printf("Window slide %d: removed %v messages, kept %v overlap tokens\n",
								windowSlides, params["messages_removed"], params["overlap_tokens"])

							// Verify overlap
							if windowSlides > 1 && previousWindowEnd != "" {
								Expect(params).To(HaveKey("overlap_content"))
								overlap := params["overlap_content"].(string)
								Expect(overlap).To(ContainSubstring(previousWindowEnd))
							}

							if windowEnd, ok := params["window_end"].(string); ok {
								previousWindowEnd = windowEnd
							}
						}
					}
				}
			}
		done:

			Expect(windowSlides).To(BeNumerically(">=", 1), "Should have at least one window slide")
		})
	})

	Describe("Context Compression", func() {
		It("should trigger automatic summarization", func() {
			contextID := uuid.New().String()

			// Create context with compression enabled
			createMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "context.create",
				Params: map[string]interface{}{
					"id":                    contextID,
					"name":                  "compression-test",
					"model_id":              "gpt-4", // Default model for tests
					"max_tokens":            2000,
					"compression_enabled":   true,
					"compression_threshold": 0.7, // Compress at 70% capacity
					"compression_ratio":     0.5, // Target 50% reduction
				},
			}

			err := wsjson.Write(ctx, conn, createMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Add compressible conversation
			conversation := []map[string]interface{}{
				{"role": "user", "content": "Can you explain what a binary search tree is?"},
				{"role": "assistant", "content": "A binary search tree is a data structure where each node has at most two children, and the left child is always less than the parent, while the right child is greater."},
				{"role": "user", "content": "How do you insert a node?"},
				{"role": "assistant", "content": "To insert a node, you start at the root and compare values, going left if smaller or right if larger, until you find an empty position."},
				{"role": "user", "content": "What about deletion?"},
				{"role": "assistant", "content": "Deletion has three cases: leaf node (just remove), one child (replace with child), two children (replace with inorder successor)."},
			}

			// Add conversation multiple times to fill context
			compressionTriggered := false
			for round := 0; round < 10 && !compressionTriggered; round++ {
				for _, msg := range conversation {
					appendMsg := ws.Message{
						ID:     uuid.New().String(),
						Type:   ws.MessageTypeRequest,
						Method: "context.append",
						Params: map[string]interface{}{
							"context_id": contextID,
							"message":    msg,
						},
					}

					err = wsjson.Write(ctx, conn, appendMsg)
					Expect(err).NotTo(HaveOccurred())

					// Check for compression notification
					timeout := time.After(100 * time.Millisecond)
					select {
					case <-timeout:
						var resp ws.Message
						_ = wsjson.Read(ctx, conn, &resp) // Clear response
					default:
						var msg ws.Message
						err := wsjson.Read(ctx, conn, &msg)
						if err == nil && msg.Type == ws.MessageTypeNotification {
							if msg.Method == "context.compressed" {
								compressionTriggered = true
								if params, ok := msg.Params.(map[string]interface{}); ok {
									GinkgoWriter.Printf("Context compressed: before=%v tokens, after=%v tokens, ratio=%.2f\n",
										params["tokens_before"], params["tokens_after"],
										params["compression_ratio"])
								}
								break
							}
						}
					}
				}
			}

			Expect(compressionTriggered).To(BeTrue(), "Should trigger compression")

			// Verify compressed context maintains coherence
			getMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "context.get",
				Params: map[string]interface{}{
					"context_id":      contextID,
					"include_summary": true,
				},
			}

			err = wsjson.Write(ctx, conn, getMsg)
			Expect(err).NotTo(HaveOccurred())

			var getResp ws.Message
			err = wsjson.Read(ctx, conn, &getResp)
			Expect(err).NotTo(HaveOccurred())

			result, ok := getResp.Result.(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(result).To(HaveKey("summary"))
			Expect(result["summary"]).To(ContainSubstring("binary search tree"))
		})
	})

	Describe("Importance-Based Retention", func() {
		It("should retain high-importance messages during truncation", func() {
			contextID := uuid.New().String()

			createMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "context.create",
				Params: map[string]interface{}{
					"id":             contextID,
					"name":           "importance-test",
					"model_id":       "gpt-4", // Default model for tests
					"max_tokens":     500,
					"retention_mode": "importance_based",
				},
			}

			err := wsjson.Write(ctx, conn, createMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Add messages with different importance levels
			importantMessages := []string{
				"CRITICAL: Database connection string is postgres://user:pass@host:5432/db",
				"IMPORTANT: The API key for production is stored in environment variable PROD_API_KEY",
				"KEY INSIGHT: The performance issue is caused by N+1 queries in the user listing",
			}

			regularMessages := []string{
				"The weather today is sunny",
				"I had coffee this morning",
				"The meeting is at 3pm",
				"Don't forget to update the documentation",
			}

			// Add important messages
			for i, msg := range importantMessages {
				appendMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "context.append",
					Params: map[string]interface{}{
						"context_id": contextID,
						"message": map[string]interface{}{
							"role":       "assistant",
							"content":    msg,
							"importance": 90 + i, // 90, 91, 92
							"metadata": map[string]interface{}{
								"category": "critical_info",
							},
						},
					},
				}

				err = wsjson.Write(ctx, conn, appendMsg)
				Expect(err).NotTo(HaveOccurred())

				var resp ws.Message
				err = wsjson.Read(ctx, conn, &resp)
				Expect(err).NotTo(HaveOccurred())
			}

			// Add many regular messages to trigger truncation
			for i := 0; i < 30; i++ {
				msg := regularMessages[i%len(regularMessages)] + fmt.Sprintf(" (iteration %d)", i)

				appendMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "context.append",
					Params: map[string]interface{}{
						"context_id": contextID,
						"message": map[string]interface{}{
							"role":       "user",
							"content":    msg,
							"importance": 20 + (i % 30), // 20-49
						},
					},
				}

				err = wsjson.Write(ctx, conn, appendMsg)
				Expect(err).NotTo(HaveOccurred())

				var resp ws.Message
				err = wsjson.Read(ctx, conn, &resp)
				Expect(err).NotTo(HaveOccurred())
			}

			// Get final context
			getMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "context.get",
				Params: map[string]interface{}{
					"context_id":       contextID,
					"include_messages": true,
				},
			}

			err = wsjson.Write(ctx, conn, getMsg)
			Expect(err).NotTo(HaveOccurred())

			var getResp ws.Message
			err = wsjson.Read(ctx, conn, &getResp)
			Expect(err).NotTo(HaveOccurred())

			// Verify important messages are retained
			result, ok := getResp.Result.(map[string]interface{})
			Expect(ok).To(BeTrue())

			messages, ok := result["messages"].([]interface{})
			Expect(ok).To(BeTrue())

			// Check that critical messages are present
			criticalFound := 0
			regularFound := 0

			for _, msg := range messages {
				msgMap, ok := msg.(map[string]interface{})
				if !ok {
					continue
				}

				content := msgMap["content"].(string)
				for _, critical := range importantMessages {
					if strings.Contains(content, critical) {
						criticalFound++
						break
					}
				}

				for _, regular := range regularMessages {
					if strings.Contains(content, regular) {
						regularFound++
						break
					}
				}
			}

			GinkgoWriter.Printf("Retained %d critical messages and %d regular messages\n",
				criticalFound, regularFound)

			Expect(criticalFound).To(Equal(len(importantMessages)), "All critical messages should be retained")
			Expect(regularFound).To(BeNumerically("<", 10), "Most regular messages should be truncated")
		})
	})
})
