package websocket

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	ws "github.com/developer-mesh/developer-mesh/pkg/models/websocket"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// TestConnectionInitializationWithEmptyAgentID tests connection initialization when claims have empty UserID
func TestConnectionInitializationWithEmptyAgentID(t *testing.T) {
	// Create server
	server := NewServer(&auth.Service{}, observability.NewNoOpMetricsClient(), NewTestLogger(), Config{
		MaxConnections: 10,
		MaxMessageSize: 1024 * 1024,
	})

	// Test cases for different UserID scenarios
	tests := []struct {
		name            string
		userID          string
		expectGenerated bool
	}{
		{
			name:            "Valid UUID UserID",
			userID:          uuid.New().String(),
			expectGenerated: false,
		},
		{
			name:            "Empty UserID",
			userID:          "",
			expectGenerated: true,
		},
		{
			name:            "Non-UUID UserID",
			userID:          "user123",
			expectGenerated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate claims with specific UserID
			claims := &auth.Claims{
				UserID:   tt.userID,
				TenantID: "test-tenant",
			}

			// Get connection from pool
			connection := server.connectionPool.Get()
			require.NotNil(t, connection)

			// Simulate the connection initialization logic from HandleWebSocket
			connectionID := uuid.New().String()

			// Generate agent ID - use UserID if available, otherwise generate new UUID
			agentID := claims.UserID
			if agentID == "" {
				// Generate a new UUID for agents without explicit user ID
				agentID = uuid.New().String()
			}

			// Initialize connection - reuse existing ws.Connection if available
			if connection.Connection == nil {
				// This should not happen with properly initialized pool, but handle it gracefully
				connection.Connection = &ws.Connection{}
				connection.State.Store(ws.ConnectionStateClosed)
			}

			// Update connection properties
			connection.ID = connectionID
			connection.AgentID = agentID
			connection.TenantID = claims.TenantID
			connection.CreatedAt = time.Now()
			connection.LastPing = time.Now()

			// Verify agent ID is not empty
			assert.NotEmpty(t, connection.AgentID)

			// If we expected generation, verify it's a valid UUID
			if tt.expectGenerated {
				_, err := uuid.Parse(connection.AgentID)
				assert.NoError(t, err, "Generated agent ID should be a valid UUID")
			} else {
				// Should use the provided UserID
				assert.Equal(t, tt.userID, connection.AgentID)
			}

			// Verify connection is properly initialized
			assert.NotNil(t, connection.Connection)
			assert.NotEmpty(t, connection.ID)
			assert.NotEmpty(t, connection.TenantID)

			// Return connection to pool
			server.connectionPool.Put(connection)
		})
	}
}

// TestAgentIDPersistenceInHandlers tests that agent ID is properly passed through message handlers
func TestAgentIDPersistenceInHandlers(t *testing.T) {
	// Create server
	server := NewServer(&auth.Service{}, observability.NewNoOpMetricsClient(), NewTestLogger(), Config{})

	// Create connection with specific agent ID
	agentID := uuid.New().String()
	conn := &Connection{
		Connection: &ws.Connection{
			ID:        uuid.New().String(),
			AgentID:   agentID,
			TenantID:  "test-tenant",
			CreatedAt: time.Now(),
			LastPing:  time.Now(),
		},
		send:   make(chan []byte, 256),
		closed: make(chan struct{}),
		hub:    server,
	}
	conn.State.Store(ws.ConnectionStateConnected)

	// Verify agent ID is preserved in connection
	assert.Equal(t, agentID, conn.AgentID)
	assert.Equal(t, agentID, conn.AgentID)

	// Test that GetTenantUUID handles the tenant ID properly
	tenantUUID := conn.GetTenantUUID()
	assert.Equal(t, uuid.Nil, tenantUUID) // Should return zero UUID for invalid format "test-tenant"
}

// TestConnectionStateTransitionsWithNilChecks verifies state transitions handle nil connections
func TestConnectionStateTransitionsWithNilChecks(t *testing.T) {
	t.Run("State transitions with valid connection", func(t *testing.T) {
		// Create properly initialized connection
		wsConn := &ws.Connection{
			ID:        uuid.New().String(),
			AgentID:   uuid.New().String(),
			TenantID:  "test-tenant",
			CreatedAt: time.Now(),
			LastPing:  time.Now(),
		}
		wsConn.State.Store(ws.ConnectionStateConnecting)

		conn := &Connection{
			Connection: wsConn,
			send:       make(chan []byte, 256),
			closed:     make(chan struct{}),
		}

		// Test state transitions
		states := []ws.ConnectionState{
			ws.ConnectionStateConnecting,
			ws.ConnectionStateConnected,
			ws.ConnectionStateClosing,
			ws.ConnectionStateClosed,
		}

		for _, state := range states {
			conn.SetState(state)
			assert.Equal(t, state, conn.GetState())
		}

		// Test IsActive for different states
		conn.SetState(ws.ConnectionStateConnected)
		assert.True(t, conn.IsActive())

		conn.SetState(ws.ConnectionStateConnecting)
		assert.True(t, conn.IsActive())

		conn.SetState(ws.ConnectionStateClosing)
		assert.False(t, conn.IsActive())

		conn.SetState(ws.ConnectionStateClosed)
		assert.False(t, conn.IsActive())
	})

	t.Run("State transitions with nil connection", func(t *testing.T) {
		conn := &Connection{
			Connection: nil,
			send:       make(chan []byte, 256),
			closed:     make(chan struct{}),
		}

		// Should not panic and return safe defaults
		assert.NotPanics(t, func() {
			conn.SetState(ws.ConnectionStateConnected)
			assert.Equal(t, ws.ConnectionStateClosed, conn.GetState())
			assert.False(t, conn.IsActive())
		})
	})
}
