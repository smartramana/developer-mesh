package websocket

import (
	"bytes"
	"sync"

	ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
	"github.com/coder/websocket"
)

// Object pools for zero-allocation design
var (
	// Message pool for WebSocket messages
	messagePool = sync.Pool{
		New: func() interface{} {
			return &ws.Message{}
		},
	}

	// Buffer pool for binary encoding
	bufferPool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, 4096))
		},
	}

	// Connection pool for reusing connection objects
	// TODO: Uncomment when implementing connection pooling
	// connectionPool = sync.Pool{
	//     New: func() interface{} {
	//         return &Connection{
	//             send: make(chan []byte, 256),
	//         }
	//     },
	// }

	// Binary header pool
	binaryHeaderPool = sync.Pool{
		New: func() interface{} {
			return &ws.BinaryHeader{}
		},
	}

	// Byte slice pools for different sizes
	smallBytePool = sync.Pool{
		New: func() interface{} {
			b := make([]byte, 256)
			return &b
		},
	}

	mediumBytePool = sync.Pool{
		New: func() interface{} {
			b := make([]byte, 4096)
			return &b
		},
	}

	largeBytePool = sync.Pool{
		New: func() interface{} {
			b := make([]byte, 65536)
			return &b
		},
	}
)

// GetMessage retrieves a message from the pool
func GetMessage() *ws.Message {
	return messagePool.Get().(*ws.Message)
}

// PutMessage returns a message to the pool
func PutMessage(msg *ws.Message) {
	// Reset the message
	msg.ID = ""
	msg.Type = 0
	msg.Method = ""
	msg.Params = nil
	msg.Result = nil
	msg.Error = nil

	messagePool.Put(msg)
}

// GetBuffer retrieves a buffer from the pool
func GetBuffer() *bytes.Buffer {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// PutBuffer returns a buffer to the pool
func PutBuffer(buf *bytes.Buffer) {
	// Only put back reasonable sized buffers
	if buf.Cap() > 1024*1024 { // 1MB
		return
	}
	bufferPool.Put(buf)
}

// GetBinaryHeader retrieves a binary header from the pool
func GetBinaryHeader() *ws.BinaryHeader {
	return binaryHeaderPool.Get().(*ws.BinaryHeader)
}

// PutBinaryHeader returns a binary header to the pool
func PutBinaryHeader(header *ws.BinaryHeader) {
	// Reset the header
	*header = ws.BinaryHeader{}
	binaryHeaderPool.Put(header)
}

// GetByteSlice retrieves a byte slice from the appropriate pool
func GetByteSlice(size int) *[]byte {
	switch {
	case size <= 256:
		return smallBytePool.Get().(*[]byte)
	case size <= 4096:
		return mediumBytePool.Get().(*[]byte)
	case size <= 65536:
		return largeBytePool.Get().(*[]byte)
	default:
		// For very large sizes, allocate directly
		b := make([]byte, size)
		return &b
	}
}

// PutByteSlice returns a byte slice to the appropriate pool
func PutByteSlice(b *[]byte) {
	if b == nil {
		return
	}

	size := cap(*b)
	switch {
	case size <= 256:
		smallBytePool.Put(b)
	case size <= 4096:
		mediumBytePool.Put(b)
	case size <= 65536:
		largeBytePool.Put(b)
		// Don't pool very large slices
	}
}

// ConnectionPoolManager manages a pool of pre-allocated connections
type ConnectionPoolManager struct {
	pool chan *Connection
	size int
	// mu field reserved for future thread-safe operations
	// mu   sync.Mutex
}

// NewConnectionPoolManager creates a new connection pool manager
func NewConnectionPoolManager(size int) *ConnectionPoolManager {
	manager := &ConnectionPoolManager{
		pool: make(chan *Connection, size),
		size: size,
	}

	// Pre-allocate connections
	for i := 0; i < size/2; i++ {
		conn := &Connection{
			send: make(chan []byte, 256),
		}
		select {
		case manager.pool <- conn:
		default:
			// Pool is full, stop pre-filling
			return manager
		}
	}

	return manager
}

// Get retrieves a connection from the pool
func (m *ConnectionPoolManager) Get() *Connection {
	select {
	case conn := <-m.pool:
		return conn
	default:
		// Create new connection if pool is empty
		return &Connection{
			send: make(chan []byte, 256),
		}
	}
}

// Put returns a connection to the pool
func (m *ConnectionPoolManager) Put(conn *Connection) {
	// Close the underlying websocket connection if it exists
	if conn.conn != nil {
		// Ignore error as connection might already be closed
		_ = conn.conn.Close(websocket.StatusNormalClosure, "")
	}

	// Reset connection state
	conn.Connection = nil
	conn.conn = nil
	// Don't reset the send channel, reuse it

	select {
	case m.pool <- conn:
	default:
		// Pool is full, let GC handle it
	}
}

// Stats returns pool statistics
func (m *ConnectionPoolManager) Stats() (available, size int) {
	return len(m.pool), m.size
}

// Memory pool for reducing GC pressure
type MemoryPool struct {
	allocations uint64
	frees       uint64
	inUse       uint64
	mu          sync.Mutex
}

var globalMemoryPool = &MemoryPool{}

// TrackAllocation tracks memory allocation
func (mp *MemoryPool) TrackAllocation() {
	mp.mu.Lock()
	mp.allocations++
	mp.inUse++
	mp.mu.Unlock()
}

// TrackFree tracks memory free
func (mp *MemoryPool) TrackFree() {
	mp.mu.Lock()
	mp.frees++
	mp.inUse--
	mp.mu.Unlock()
}

// Stats returns memory pool statistics
func (mp *MemoryPool) Stats() (allocations, frees, inUse uint64) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	return mp.allocations, mp.frees, mp.inUse
}
