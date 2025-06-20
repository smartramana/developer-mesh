package api_test

import (
	"context"
	"fmt"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"functional-tests/shared"

	ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

var _ = Describe("WebSocket Session Management", func() {
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

	Describe("Session Creation and Persistence", func() {
		It("should create and maintain session state", func() {
			// Create a session
			createMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.create",
				Params: map[string]interface{}{
					"agent_profile": map[string]interface{}{
						"name":         "test-agent",
						"capabilities": []string{"code_review", "testing", "documentation"},
						"preferences": map[string]interface{}{
							"response_style": "concise",
							"language":       "go",
						},
					},
				},
			}

			err := wsjson.Write(ctx, conn, createMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())
			Expect(createResp.Error).To(BeNil())

			sessionID := ""
			if result, ok := createResp.Result.(map[string]interface{}); ok {
				sessionID = result["session_id"].(string)
				Expect(sessionID).NotTo(BeEmpty())
				Expect(result).To(HaveKey("created_at"))
			}

			// Update session state
			updateMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.update_state",
				Params: map[string]interface{}{
					"session_id": sessionID,
					"state": map[string]interface{}{
						"current_project": "devops-mcp",
						"active_files": []string{
							"main.go",
							"pkg/server/server.go",
						},
						"context_tokens": 1500,
					},
				},
			}

			err = wsjson.Write(ctx, conn, updateMsg)
			Expect(err).NotTo(HaveOccurred())

			var updateResp ws.Message
			err = wsjson.Read(ctx, conn, &updateResp)
			Expect(err).NotTo(HaveOccurred())
			Expect(updateResp.Error).To(BeNil())

			// Retrieve session
			getMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.get",
				Params: map[string]interface{}{
					"session_id": sessionID,
				},
			}

			err = wsjson.Write(ctx, conn, getMsg)
			Expect(err).NotTo(HaveOccurred())

			var getResp ws.Message
			err = wsjson.Read(ctx, conn, &getResp)
			Expect(err).NotTo(HaveOccurred())

			// Verify session data
			if result, ok := getResp.Result.(map[string]interface{}); ok {
				Expect(result["session_id"]).To(Equal(sessionID))

				if state, ok := result["state"].(map[string]interface{}); ok {
					Expect(state["current_project"]).To(Equal("devops-mcp"))
					Expect(state["context_tokens"]).To(Equal(float64(1500)))

					if files, ok := state["active_files"].([]interface{}); ok {
						Expect(len(files)).To(Equal(2))
					}
				}

				if profile, ok := result["agent_profile"].(map[string]interface{}); ok {
					Expect(profile["name"]).To(Equal("test-agent"))
				}
			}
		})

		It("should maintain conversation history", func() {
			// Create session
			sessionID := uuid.New().String()
			createMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.create",
				Params: map[string]interface{}{
					"session_id": sessionID,
				},
			}

			err := wsjson.Write(ctx, conn, createMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Add messages to conversation
			messages := []map[string]interface{}{
				{
					"role":    "user",
					"content": "Can you review this code?",
				},
				{
					"role":    "assistant",
					"content": "I'd be happy to review your code. Please share it.",
				},
				{
					"role":    "user",
					"content": "func main() { fmt.Println(\"Hello\") }",
				},
				{
					"role":    "assistant",
					"content": "The code looks good. Consider adding error handling.",
				},
			}

			for _, msg := range messages {
				addMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "session.add_message",
					Params: map[string]interface{}{
						"session_id": sessionID,
						"message":    msg,
					},
				}

				err = wsjson.Write(ctx, conn, addMsg)
				Expect(err).NotTo(HaveOccurred())

				var addResp ws.Message
				err = wsjson.Read(ctx, conn, &addResp)
				Expect(err).NotTo(HaveOccurred())
			}

			// Get conversation history
			historyMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.get_history",
				Params: map[string]interface{}{
					"session_id": sessionID,
					"limit":      10,
				},
			}

			err = wsjson.Write(ctx, conn, historyMsg)
			Expect(err).NotTo(HaveOccurred())

			var historyResp ws.Message
			err = wsjson.Read(ctx, conn, &historyResp)
			Expect(err).NotTo(HaveOccurred())

			// Verify history
			if result, ok := historyResp.Result.(map[string]interface{}); ok {
				if history, ok := result["messages"].([]interface{}); ok {
					Expect(len(history)).To(Equal(len(messages)))

					// Verify message order and content
					for i, msg := range history {
						if m, ok := msg.(map[string]interface{}); ok {
							expectedRole := messages[i]["role"].(string)
							Expect(m["role"]).To(Equal(expectedRole))
							Expect(m).To(HaveKey("content"))
							Expect(m).To(HaveKey("timestamp"))
						}
					}
				}
			}
		})

		It("should support session branching", func() {
			// Create parent session
			parentID := uuid.New().String()
			createMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.create",
				Params: map[string]interface{}{
					"session_id": parentID,
					"initial_context": map[string]interface{}{
						"project": "test-project",
						"task":    "implement feature X",
					},
				},
			}

			err := wsjson.Write(ctx, conn, createMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Add some conversation
			for i := 0; i < 3; i++ {
				addMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "session.add_message",
					Params: map[string]interface{}{
						"session_id": parentID,
						"message": map[string]interface{}{
							"role":    "user",
							"content": fmt.Sprintf("Step %d of implementation", i+1),
						},
					},
				}

				err = wsjson.Write(ctx, conn, addMsg)
				Expect(err).NotTo(HaveOccurred())

				var addResp ws.Message
				err = wsjson.Read(ctx, conn, &addResp)
				Expect(err).NotTo(HaveOccurred())
			}

			// Create branch from parent
			branchMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.branch",
				Params: map[string]interface{}{
					"parent_session_id": parentID,
					"branch_point":      2, // Branch after second message
					"branch_name":       "alternative-approach",
				},
			}

			err = wsjson.Write(ctx, conn, branchMsg)
			Expect(err).NotTo(HaveOccurred())

			var branchResp ws.Message
			err = wsjson.Read(ctx, conn, &branchResp)
			Expect(err).NotTo(HaveOccurred())

			branchID := ""
			if result, ok := branchResp.Result.(map[string]interface{}); ok {
				branchID = result["branch_session_id"].(string)
				Expect(branchID).NotTo(BeEmpty())
				Expect(branchID).NotTo(Equal(parentID))
			}

			// Verify branch has parent's history up to branch point
			historyMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.get_history",
				Params: map[string]interface{}{
					"session_id": branchID,
				},
			}

			err = wsjson.Write(ctx, conn, historyMsg)
			Expect(err).NotTo(HaveOccurred())

			var historyResp ws.Message
			err = wsjson.Read(ctx, conn, &historyResp)
			Expect(err).NotTo(HaveOccurred())

			if result, ok := historyResp.Result.(map[string]interface{}); ok {
				if history, ok := result["messages"].([]interface{}); ok {
					Expect(len(history)).To(Equal(2)) // Only first 2 messages
				}
				Expect(result).To(HaveKey("parent_session_id"))
				Expect(result["parent_session_id"]).To(Equal(parentID))
			}

			// Add different content to branch
			addBranchMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.add_message",
				Params: map[string]interface{}{
					"session_id": branchID,
					"message": map[string]interface{}{
						"role":    "user",
						"content": "Let's try a different approach",
					},
				},
			}

			err = wsjson.Write(ctx, conn, addBranchMsg)
			Expect(err).NotTo(HaveOccurred())

			var addBranchResp ws.Message
			err = wsjson.Read(ctx, conn, &addBranchResp)
			Expect(err).NotTo(HaveOccurred())

			// Verify parent and branch have diverged
			parentHistoryMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.get_history",
				Params: map[string]interface{}{
					"session_id": parentID,
				},
			}

			err = wsjson.Write(ctx, conn, parentHistoryMsg)
			Expect(err).NotTo(HaveOccurred())

			var parentHistoryResp ws.Message
			err = wsjson.Read(ctx, conn, &parentHistoryResp)
			Expect(err).NotTo(HaveOccurred())

			if result, ok := parentHistoryResp.Result.(map[string]interface{}); ok {
				if history, ok := result["messages"].([]interface{}); ok {
					Expect(len(history)).To(Equal(3)) // Original 3 messages
				}
			}
		})
	})

	Describe("Session Recovery", func() {
		It("should recover session after disconnection", func() {
			// Create session with state
			sessionID := uuid.New().String()
			createMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.create",
				Params: map[string]interface{}{
					"session_id": sessionID,
					"persistent": true,
					"state": map[string]interface{}{
						"important_data": "must not lose this",
						"progress":       50,
					},
				},
			}

			err := wsjson.Write(ctx, conn, createMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Add some messages
			for i := 0; i < 3; i++ {
				addMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "session.add_message",
					Params: map[string]interface{}{
						"session_id": sessionID,
						"message": map[string]interface{}{
							"role":    "user",
							"content": fmt.Sprintf("Message %d", i+1),
						},
					},
				}

				err = wsjson.Write(ctx, conn, addMsg)
				Expect(err).NotTo(HaveOccurred())

				var addResp ws.Message
				err = wsjson.Read(ctx, conn, &addResp)
				Expect(err).NotTo(HaveOccurred())
			}

			// Simulate disconnection
			conn.Close(websocket.StatusGoingAway, "simulating disconnect")

			// Wait a bit
			time.Sleep(500 * time.Millisecond)

			// Reconnect
			newConn, err := shared.EstablishConnection(wsURL, apiKey)
			Expect(err).NotTo(HaveOccurred())
			defer newConn.Close(websocket.StatusNormalClosure, "")

			// Recover session
			recoverMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.recover",
				Params: map[string]interface{}{
					"session_id": sessionID,
				},
			}

			err = wsjson.Write(ctx, newConn, recoverMsg)
			Expect(err).NotTo(HaveOccurred())

			var recoverResp ws.Message
			err = wsjson.Read(ctx, newConn, &recoverResp)
			Expect(err).NotTo(HaveOccurred())

			// Verify session recovered successfully
			if result, ok := recoverResp.Result.(map[string]interface{}); ok {
				Expect(result["recovered"]).To(BeTrue())

				if state, ok := result["state"].(map[string]interface{}); ok {
					Expect(state["important_data"]).To(Equal("must not lose this"))
					Expect(state["progress"]).To(Equal(float64(50)))
				}

				if history, ok := result["message_count"].(float64); ok {
					Expect(int(history)).To(Equal(3))
				}
			}
		})

		It("should handle session expiration", func() {
			// Create session with short TTL
			sessionID := uuid.New().String()
			createMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.create",
				Params: map[string]interface{}{
					"session_id":  sessionID,
					"ttl_seconds": 2, // 2 second TTL
				},
			}

			err := wsjson.Write(ctx, conn, createMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Wait for session to expire
			time.Sleep(3 * time.Second)

			// Try to use expired session
			getMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.get",
				Params: map[string]interface{}{
					"session_id": sessionID,
				},
			}

			err = wsjson.Write(ctx, conn, getMsg)
			Expect(err).NotTo(HaveOccurred())

			var getResp ws.Message
			err = wsjson.Read(ctx, conn, &getResp)
			Expect(err).NotTo(HaveOccurred())

			// Should get error or expired status
			if getResp.Error != nil {
				Expect(getResp.Error.Message).To(ContainSubstring("expired"))
			} else if result, ok := getResp.Result.(map[string]interface{}); ok {
				Expect(result["status"]).To(Equal("expired"))
			}
		})
	})

	Describe("Session Analytics", func() {
		It("should track session metrics", func() {
			sessionID := uuid.New().String()

			// Create session
			createMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.create",
				Params: map[string]interface{}{
					"session_id":    sessionID,
					"track_metrics": true,
				},
			}

			err := wsjson.Write(ctx, conn, createMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Perform various operations
			operations := []map[string]interface{}{
				{
					"method": "tool.execute",
					"tool":   "code_reviewer",
				},
				{
					"method": "context.create",
					"size":   1000,
				},
				{
					"method": "tool.execute",
					"tool":   "test_runner",
				},
			}

			for _, op := range operations {
				opMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: op["method"].(string),
					Params: map[string]interface{}{
						"session_id": sessionID,
						"tool":       op["tool"],
						"size":       op["size"],
					},
				}

				err = wsjson.Write(ctx, conn, opMsg)
				Expect(err).NotTo(HaveOccurred())

				var opResp ws.Message
				err = wsjson.Read(ctx, conn, &opResp)
				Expect(err).NotTo(HaveOccurred())

				time.Sleep(100 * time.Millisecond)
			}

			// Get session metrics
			metricsMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.get_metrics",
				Params: map[string]interface{}{
					"session_id": sessionID,
				},
			}

			err = wsjson.Write(ctx, conn, metricsMsg)
			Expect(err).NotTo(HaveOccurred())

			var metricsResp ws.Message
			err = wsjson.Read(ctx, conn, &metricsResp)
			Expect(err).NotTo(HaveOccurred())

			// Verify metrics
			if result, ok := metricsResp.Result.(map[string]interface{}); ok {
				Expect(result).To(HaveKey("duration_seconds"))
				Expect(result).To(HaveKey("operation_count"))
				Expect(result).To(HaveKey("token_usage"))

				if opCount, ok := result["operation_count"].(float64); ok {
					Expect(int(opCount)).To(BeNumerically(">=", len(operations)))
				}

				if toolUsage, ok := result["tool_usage"].(map[string]interface{}); ok {
					Expect(toolUsage).To(HaveKey("code_reviewer"))
					Expect(toolUsage).To(HaveKey("test_runner"))
				}
			}
		})

		It("should export session data", func() {
			sessionID := uuid.New().String()

			// Create session with data
			createMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.create",
				Params: map[string]interface{}{
					"session_id": sessionID,
				},
			}

			err := wsjson.Write(ctx, conn, createMsg)
			Expect(err).NotTo(HaveOccurred())

			var createResp ws.Message
			err = wsjson.Read(ctx, conn, &createResp)
			Expect(err).NotTo(HaveOccurred())

			// Add some conversation
			messages := []string{
				"Can you help me optimize this function?",
				"Here's my analysis of the function...",
				"What about performance considerations?",
				"Let me benchmark the different approaches...",
			}

			for i, content := range messages {
				role := "user"
				if i%2 == 1 {
					role = "assistant"
				}

				addMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "session.add_message",
					Params: map[string]interface{}{
						"session_id": sessionID,
						"message": map[string]interface{}{
							"role":    role,
							"content": content,
						},
					},
				}

				err = wsjson.Write(ctx, conn, addMsg)
				Expect(err).NotTo(HaveOccurred())

				var addResp ws.Message
				err = wsjson.Read(ctx, conn, &addResp)
				Expect(err).NotTo(HaveOccurred())
			}

			// Export session data
			exportMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.export",
				Params: map[string]interface{}{
					"session_id": sessionID,
					"format":     "json",
					"include": []string{
						"messages",
						"state",
						"metadata",
						"metrics",
					},
				},
			}

			err = wsjson.Write(ctx, conn, exportMsg)
			Expect(err).NotTo(HaveOccurred())

			var exportResp ws.Message
			err = wsjson.Read(ctx, conn, &exportResp)
			Expect(err).NotTo(HaveOccurred())

			// Verify export contains all requested data
			if result, ok := exportResp.Result.(map[string]interface{}); ok {
				if export, ok := result["export"].(map[string]interface{}); ok {
					Expect(export).To(HaveKey("session_id"))
					Expect(export).To(HaveKey("messages"))
					Expect(export).To(HaveKey("metadata"))

					if msgs, ok := export["messages"].([]interface{}); ok {
						Expect(len(msgs)).To(Equal(len(messages)))
					}

					if metadata, ok := export["metadata"].(map[string]interface{}); ok {
						Expect(metadata).To(HaveKey("created_at"))
						Expect(metadata).To(HaveKey("last_activity"))
						Expect(metadata).To(HaveKey("message_count"))
					}
				}

				// Should also provide download info
				Expect(result).To(HaveKey("download_url"))
				Expect(result).To(HaveKey("expires_at"))
			}
		})
	})

	Describe("Multi-Session Management", func() {
		It("should list active sessions for an agent", func() {
			// Create multiple sessions
			sessionIDs := make([]string, 3)
			for i := 0; i < 3; i++ {
				sessionID := uuid.New().String()
				sessionIDs[i] = sessionID

				createMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "session.create",
					Params: map[string]interface{}{
						"session_id": sessionID,
						"name":       fmt.Sprintf("Session %d", i+1),
						"tags":       []string{"test", fmt.Sprintf("priority-%d", i+1)},
					},
				}

				err := wsjson.Write(ctx, conn, createMsg)
				Expect(err).NotTo(HaveOccurred())

				var createResp ws.Message
				err = wsjson.Read(ctx, conn, &createResp)
				Expect(err).NotTo(HaveOccurred())
			}

			// List sessions
			listMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.list",
				Params: map[string]interface{}{
					"filter": map[string]interface{}{
						"tags": []string{"test"},
					},
					"sort_by": "created_at",
					"limit":   10,
				},
			}

			err := wsjson.Write(ctx, conn, listMsg)
			Expect(err).NotTo(HaveOccurred())

			var listResp ws.Message
			err = wsjson.Read(ctx, conn, &listResp)
			Expect(err).NotTo(HaveOccurred())

			// Verify list results
			if result, ok := listResp.Result.(map[string]interface{}); ok {
				if sessions, ok := result["sessions"].([]interface{}); ok {
					Expect(len(sessions)).To(Equal(3))

					for i, session := range sessions {
						if s, ok := session.(map[string]interface{}); ok {
							Expect(s).To(HaveKey("session_id"))
							Expect(s).To(HaveKey("name"))
							Expect(s["name"]).To(Equal(fmt.Sprintf("Session %d", i+1)))

							if tags, ok := s["tags"].([]interface{}); ok {
								Expect(tags).To(ContainElement("test"))
							}
						}
					}
				}
			}
		})

		It("should switch between sessions", func() {
			// Create two sessions
			session1ID := uuid.New().String()
			session2ID := uuid.New().String()

			for i, sessionID := range []string{session1ID, session2ID} {
				createMsg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "session.create",
					Params: map[string]interface{}{
						"session_id": sessionID,
						"state": map[string]interface{}{
							"context": fmt.Sprintf("Session %d context", i+1),
						},
					},
				}

				err := wsjson.Write(ctx, conn, createMsg)
				Expect(err).NotTo(HaveOccurred())

				var createResp ws.Message
				err = wsjson.Read(ctx, conn, &createResp)
				Expect(err).NotTo(HaveOccurred())
			}

			// Set active session to session1
			switchMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.set_active",
				Params: map[string]interface{}{
					"session_id": session1ID,
				},
			}

			err := wsjson.Write(ctx, conn, switchMsg)
			Expect(err).NotTo(HaveOccurred())

			var switchResp ws.Message
			err = wsjson.Read(ctx, conn, &switchResp)
			Expect(err).NotTo(HaveOccurred())

			// Execute a tool (should use session1 context)
			toolMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "tool.execute",
				Params: map[string]interface{}{
					"name":      "echo_context",
					"arguments": map[string]interface{}{},
				},
			}

			err = wsjson.Write(ctx, conn, toolMsg)
			Expect(err).NotTo(HaveOccurred())

			var toolResp ws.Message
			err = wsjson.Read(ctx, conn, &toolResp)
			Expect(err).NotTo(HaveOccurred())

			// Should have session1 context
			if result, ok := toolResp.Result.(map[string]interface{}); ok {
				Expect(result["context"]).To(ContainSubstring("Session 1"))
			}

			// Switch to session2
			switch2Msg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "session.set_active",
				Params: map[string]interface{}{
					"session_id": session2ID,
				},
			}

			err = wsjson.Write(ctx, conn, switch2Msg)
			Expect(err).NotTo(HaveOccurred())

			var switch2Resp ws.Message
			err = wsjson.Read(ctx, conn, &switch2Resp)
			Expect(err).NotTo(HaveOccurred())

			// Execute tool again
			tool2Msg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "tool.execute",
				Params: map[string]interface{}{
					"name":      "echo_context",
					"arguments": map[string]interface{}{},
				},
			}

			err = wsjson.Write(ctx, conn, tool2Msg)
			Expect(err).NotTo(HaveOccurred())

			var tool2Resp ws.Message
			err = wsjson.Read(ctx, conn, &tool2Resp)
			Expect(err).NotTo(HaveOccurred())

			// Should have session2 context
			if result, ok := tool2Resp.Result.(map[string]interface{}); ok {
				Expect(result["context"]).To(ContainSubstring("Session 2"))
			}
		})
	})
})
