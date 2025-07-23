package websocket

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	ws "github.com/developer-mesh/developer-mesh/pkg/models/websocket"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

func TestBatchProcessor(t *testing.T) {
	// Create test configuration
	config := &BatchConfig{
		BatchSize:     5,
		MaxBatchSize:  10,
		FlushInterval: 50 * time.Millisecond,
		BinaryMode:    false, // Use JSON for easier testing
	}

	// Track sent batches
	var sentBatches [][]byte
	var mu sync.Mutex

	sendFunc := func(data []byte) error {
		mu.Lock()
		sentBatches = append(sentBatches, data)
		mu.Unlock()
		return nil
	}

	logger := observability.NewStandardLogger("test")

	processor := NewBatchProcessor(config, sendFunc, logger, nil)
	defer func() {
		if err := processor.Close(); err != nil {
			t.Errorf("Failed to close processor: %v", err)
		}
	}()

	// Add messages below batch size
	for i := 0; i < 3; i++ {
		msg := map[string]interface{}{
			"id":     i,
			"method": "test",
		}
		data, _ := json.Marshal(msg)
		err := processor.Add("conn1", data)
		assert.NoError(t, err)
	}

	// Should not have sent yet
	mu.Lock()
	assert.Equal(t, 0, len(sentBatches))
	mu.Unlock()

	// Add more to trigger batch
	for i := 3; i < 5; i++ {
		msg := map[string]interface{}{
			"id":     i,
			"method": "test",
		}
		data, _ := json.Marshal(msg)
		err := processor.Add("conn1", data)
		assert.NoError(t, err)
	}

	// Should have sent a batch
	time.Sleep(10 * time.Millisecond)
	mu.Lock()
	assert.Equal(t, 1, len(sentBatches))
	mu.Unlock()

	// Test flush timer
	msg := map[string]interface{}{
		"id":     99,
		"method": "test",
	}
	data, _ := json.Marshal(msg)
	if err := processor.Add("conn1", data); err != nil {
		t.Fatalf("Failed to add message: %v", err)
	}

	// Wait for flush interval
	time.Sleep(60 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, 2, len(sentBatches))
	mu.Unlock()
}

func TestBatchProcessorBinaryMode(t *testing.T) {
	config := &BatchConfig{
		BatchSize:     3,
		MaxBatchSize:  10,
		FlushInterval: 100 * time.Millisecond,
		BinaryMode:    true,
	}

	var sentData []byte
	sendFunc := func(data []byte) error {
		sentData = data
		return nil
	}

	logger := observability.NewStandardLogger("test")

	processor := NewBatchProcessor(config, sendFunc, logger, nil)
	defer func() {
		if err := processor.Close(); err != nil {
			t.Errorf("Failed to close processor: %v", err)
		}
	}()

	// Add messages
	for i := 0; i < 3; i++ {
		msg := &ws.Message{
			ID:     string(rune('0' + i)),
			Type:   ws.MessageTypeRequest,
			Method: "test",
		}
		data, _ := json.Marshal(msg)
		if err := processor.Add("conn1", data); err != nil {
			t.Fatalf("Failed to add message: %v", err)
		}
	}

	// Check binary format
	assert.NotNil(t, sentData)
	assert.Greater(t, len(sentData), 24) // At least header size

	// Verify magic number
	magic := uint32(sentData[0])<<24 | uint32(sentData[1])<<16 | uint32(sentData[2])<<8 | uint32(sentData[3])
	assert.Equal(t, ws.MagicNumber, magic)
}

func TestBatchStats(t *testing.T) {
	config := DefaultBatchConfig()
	config.BatchSize = 2

	sentCount := 0
	sendFunc := func(data []byte) error {
		sentCount++
		return nil
	}

	logger := observability.NewStandardLogger("test")

	processor := NewBatchProcessor(config, sendFunc, logger, nil)
	defer func() {
		if err := processor.Close(); err != nil {
			t.Errorf("Failed to close processor: %v", err)
		}
	}()

	// Send messages
	for i := 0; i < 10; i++ {
		data := []byte(`{"test": true}`)
		if err := processor.Add("conn1", data); err != nil {
			t.Fatalf("Failed to add message: %v", err)
		}
	}

	// Get stats
	stats := processor.Stats()
	assert.Equal(t, uint64(10), stats.TotalMessages)
	assert.Equal(t, uint64(5), stats.TotalBatches) // 10 messages / batch size 2
	assert.Equal(t, 0, stats.PendingCount)
	assert.Equal(t, float64(2), stats.AvgBatchSize)
}

func TestBatchManager(t *testing.T) {
	config := DefaultBatchConfig()
	logger := observability.NewStandardLogger("test")

	manager := NewBatchManager(config, logger, nil)
	defer func() {
		if err := manager.Close(); err != nil {
			t.Errorf("Failed to close manager: %v", err)
		}
	}()

	// Create processors for different connections
	sendFunc1 := func(data []byte) error { return nil }
	sendFunc2 := func(data []byte) error { return nil }

	proc1 := manager.GetProcessor("conn1", sendFunc1)
	proc2 := manager.GetProcessor("conn2", sendFunc2)

	assert.NotNil(t, proc1)
	assert.NotNil(t, proc2)
	assert.NotEqual(t, proc1, proc2)

	// Get same processor again
	proc1Again := manager.GetProcessor("conn1", sendFunc1)
	assert.Equal(t, proc1, proc1Again)

	// Remove processor
	err := manager.RemoveProcessor("conn1")
	assert.NoError(t, err)

	// Should create new processor
	proc1New := manager.GetProcessor("conn1", sendFunc1)
	assert.NotEqual(t, proc1, proc1New)
}

func BenchmarkBatchProcessor(b *testing.B) {
	config := DefaultBatchConfig()
	config.BatchSize = 100

	sendFunc := func(data []byte) error { return nil }
	logger := observability.NewStandardLogger("bench")

	processor := NewBatchProcessor(config, sendFunc, logger, nil)
	defer func() {
		if err := processor.Close(); err != nil {
			b.Errorf("Failed to close processor: %v", err)
		}
	}()

	msg := []byte(`{"id": "test", "method": "benchmark"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := processor.Add("conn1", msg); err != nil {
			b.Fatalf("Failed to add message: %v", err)
		}
	}
}

func BenchmarkBatchProcessorBinary(b *testing.B) {
	config := DefaultBatchConfig()
	config.BatchSize = 100
	config.BinaryMode = true

	sendFunc := func(data []byte) error { return nil }
	logger := observability.NewStandardLogger("bench")

	processor := NewBatchProcessor(config, sendFunc, logger, nil)
	defer func() {
		if err := processor.Close(); err != nil {
			b.Errorf("Failed to close processor: %v", err)
		}
	}()

	msg := []byte(`{"id": "test", "method": "benchmark"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := processor.Add("conn1", msg); err != nil {
			b.Fatalf("Failed to add message: %v", err)
		}
	}
}
