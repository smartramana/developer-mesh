package websocket

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

// TestConnectionNilSafety tests that all connection methods handle nil embedded Connection gracefully
func TestConnectionNilSafety(t *testing.T) {
	t.Run("nil embedded connection", func(t *testing.T) {
		// Create a connection with nil embedded ws.Connection
		conn := &Connection{
			Connection: nil, // Explicitly nil
			send:       make(chan []byte, 256),
			closed:     make(chan struct{}),
		}

		// Test IsActive - should return false
		assert.False(t, conn.IsActive())

		// Test SetState - should not panic
		assert.NotPanics(t, func() {
			conn.SetState(ws.ConnectionStateConnected)
		})

		// Test GetState - should return closed state
		assert.Equal(t, ws.ConnectionStateClosed, conn.GetState())

		// Test Close - should not panic
		assert.NotPanics(t, func() {
			err := conn.Close()
			assert.NoError(t, err)
		})
	})

	t.Run("properly initialized connection", func(t *testing.T) {
		// Create a connection with initialized embedded ws.Connection
		wsConn := &ws.Connection{
			ID:        "test-id",
			AgentID:   "agent-id",
			TenantID:  "tenant-id",
			CreatedAt: time.Now(),
			LastPing:  time.Now(),
		}
		wsConn.State.Store(ws.ConnectionStateConnecting)

		conn := &Connection{
			Connection: wsConn,
			send:       make(chan []byte, 256),
			closed:     make(chan struct{}),
		}

		// Test IsActive - should work normally
		assert.True(t, conn.IsActive())

		// Test SetState - should work normally
		conn.SetState(ws.ConnectionStateConnected)
		assert.Equal(t, ws.ConnectionStateConnected, conn.GetState())

		// Test that state changes work
		conn.SetState(ws.ConnectionStateClosed)
		assert.Equal(t, ws.ConnectionStateClosed, conn.GetState())
		assert.False(t, conn.IsActive())
	})

	t.Run("connection from pool", func(t *testing.T) {
		// Test connection from pool manager
		poolManager := NewConnectionPoolManager(10)
		defer poolManager.Stop()

		conn := poolManager.Get()
		assert.NotNil(t, conn)
		assert.NotNil(t, conn.Connection, "Pool should initialize embedded Connection")
		assert.NotNil(t, conn.send)
		assert.NotNil(t, conn.closed)

		// Verify it's properly initialized and safe to use
		assert.NotPanics(t, func() {
			conn.SetState(ws.ConnectionStateConnected)
			assert.True(t, conn.IsActive())
			assert.Equal(t, ws.ConnectionStateConnected, conn.GetState())
		})

		// Return to pool
		poolManager.Put(conn)
	})

	t.Run("GetConnection from sync.Pool", func(t *testing.T) {
		// Test the sync.Pool directly
		conn := GetConnection()
		assert.NotNil(t, conn)
		assert.Nil(t, conn.Connection, "GetConnection resets embedded Connection to nil")
		assert.NotNil(t, conn.send)
		assert.NotNil(t, conn.closed)

		// Even with nil Connection, our safe methods should work
		assert.NotPanics(t, func() {
			assert.False(t, conn.IsActive())
			conn.SetState(ws.ConnectionStateConnected)
			assert.Equal(t, ws.ConnectionStateClosed, conn.GetState())
		})
		
		PutConnection(conn)
	})
}

// TestConnectionPoolInitialization verifies pool creates properly initialized connections
func TestConnectionPoolInitialization(t *testing.T) {
	// Get multiple connections from pool and verify they're all properly initialized
	conns := make([]*Connection, 20)
	
	for i := 0; i < 20; i++ {
		conn := connectionPool.Get().(*Connection)
		assert.NotNil(t, conn)
		// Note: sync.Pool may return connections with nil embedded Connection after GetConnection resets it
		// The important thing is that the pool creates valid connections when New is called
		assert.NotNil(t, conn.send)
		assert.NotNil(t, conn.closed)
		
		// If Connection is not nil, verify state is initialized
		if conn.Connection != nil {
			assert.Equal(t, ws.ConnectionStateClosed, conn.GetState())
		}
		conns[i] = conn
	}
	
	// Return all connections to pool
	for _, conn := range conns {
		connectionPool.Put(conn)
	}
	
	// Test the pool's New function directly
	newConn := connectionPool.New().(*Connection)
	assert.NotNil(t, newConn)
	assert.NotNil(t, newConn.Connection, "Pool's New function should create connections with initialized ws.Connection")
	assert.NotNil(t, newConn.send)
	assert.NotNil(t, newConn.closed)
	assert.Equal(t, ws.ConnectionStateClosed, newConn.GetState())
}