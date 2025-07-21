package websocket

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

func TestMessagePool(t *testing.T) {
	// Get a message from pool
	msg1 := GetMessage()
	assert.NotNil(t, msg1)

	// Set some values
	msg1.ID = "test-id"
	msg1.Type = ws.MessageTypeRequest
	msg1.Method = "test.method"

	// Put it back
	PutMessage(msg1)

	// Get another message - should be the same object
	msg2 := GetMessage()
	assert.NotNil(t, msg2)

	// Values should be reset
	assert.Empty(t, msg2.ID)
	assert.Equal(t, ws.MessageType(0), msg2.Type)
	assert.Empty(t, msg2.Method)
}

func TestBufferPool(t *testing.T) {
	// Get a buffer
	buf1 := GetBuffer()
	assert.NotNil(t, buf1)
	assert.Equal(t, 0, buf1.Len())

	// Write some data
	buf1.WriteString("test data")
	assert.Equal(t, 9, buf1.Len())

	// Put it back
	PutBuffer(buf1)

	// Get another buffer - should be reset
	buf2 := GetBuffer()
	assert.NotNil(t, buf2)
	assert.Equal(t, 0, buf2.Len())
}

func TestByteSlicePool(t *testing.T) {
	tests := []struct {
		name     string
		size     int
		poolType string
	}{
		{"small", 100, "small"},
		{"medium", 2000, "medium"},
		{"large", 50000, "large"},
		{"extra large", 100000, "direct"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get byte slice
			b := GetByteSlice(tt.size)
			assert.NotNil(t, b)
			assert.GreaterOrEqual(t, cap(*b), tt.size)

			// Put it back
			PutByteSlice(b)
		})
	}
}

func TestConnectionPoolManager(t *testing.T) {
	poolSize := 10
	manager := NewConnectionPoolManager(poolSize)

	// Check initial stats
	available, size := manager.Stats()
	assert.Equal(t, poolSize, size)
	assert.GreaterOrEqual(t, available, 0) // Pool may pre-allocate but won't be used

	// Get connections - always creates new ones
	conns := make([]*Connection, 0, poolSize)
	for i := 0; i < poolSize; i++ {
		conn := manager.Get()
		assert.NotNil(t, conn)
		assert.NotNil(t, conn.send)
		assert.NotNil(t, conn.closed)
		assert.NotNil(t, conn.Connection)
		conns = append(conns, conn)
	}

	// Check that each connection is unique
	for i := 0; i < len(conns); i++ {
		for j := i + 1; j < len(conns); j++ {
			assert.NotSame(t, conns[i], conns[j], "connections should be unique")
		}
	}

	// Put connections back - they should be destroyed, not pooled
	for _, conn := range conns {
		manager.Put(conn)
	}

	// Pool should remain empty since we don't reuse WebSocket connections
	available, _ = manager.Stats()
	assert.Equal(t, 0, available)
}

func TestMemoryPoolStats(t *testing.T) {
	mp := &MemoryPool{}

	// Initial stats
	allocs, frees, inUse := mp.Stats()
	assert.Equal(t, uint64(0), allocs)
	assert.Equal(t, uint64(0), frees)
	assert.Equal(t, uint64(0), inUse)

	// Track allocations
	mp.TrackAllocation()
	mp.TrackAllocation()

	allocs, frees, inUse = mp.Stats()
	assert.Equal(t, uint64(2), allocs)
	assert.Equal(t, uint64(0), frees)
	assert.Equal(t, uint64(2), inUse)

	// Track frees
	mp.TrackFree()

	allocs, frees, inUse = mp.Stats()
	assert.Equal(t, uint64(2), allocs)
	assert.Equal(t, uint64(1), frees)
	assert.Equal(t, uint64(1), inUse)
}

func BenchmarkMessagePool(b *testing.B) {
	b.Run("WithPool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			msg := GetMessage()
			msg.ID = "test"
			msg.Type = ws.MessageTypeRequest
			PutMessage(msg)
		}
	})

	b.Run("WithoutPool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			msg := &ws.Message{
				ID:   "test",
				Type: ws.MessageTypeRequest,
			}
			_ = msg
		}
	})
}

func BenchmarkBufferPool(b *testing.B) {
	data := []byte("test data for buffer pool benchmark")

	b.Run("WithPool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			buf := GetBuffer()
			buf.Write(data)
			PutBuffer(buf)
		}
	})

	b.Run("WithoutPool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			buf := bytes.NewBuffer(make([]byte, 0, 4096))
			buf.Write(data)
		}
	})
}
