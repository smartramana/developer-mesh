package crdt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGCounter(t *testing.T) {
	t.Run("New counter starts at zero", func(t *testing.T) {
		counter := NewGCounter()
		assert.Equal(t, uint64(0), counter.Value())
	})

	t.Run("Increment increases value", func(t *testing.T) {
		counter := NewGCounter()
		counter.Increment("node1", 5)
		assert.Equal(t, uint64(5), counter.Value())

		counter.Increment("node1", 3)
		assert.Equal(t, uint64(8), counter.Value())
	})

	t.Run("Merge takes maximum values", func(t *testing.T) {
		counter1 := NewGCounter()
		counter1.Increment("node1", 5)

		counter2 := NewGCounter()
		counter2.Increment("node2", 3)

		// Merge counter2 into counter1
		err := counter1.Merge(counter2)
		require.NoError(t, err)

		// Value should be sum of both
		assert.Equal(t, uint64(8), counter1.Value())

		// counter2 should be unchanged
		assert.Equal(t, uint64(3), counter2.Value())
	})

	t.Run("Merge with overlapping increments", func(t *testing.T) {
		// Simulate two nodes incrementing independently
		counter1 := NewGCounter()
		counter1.Increment("node1", 5)

		counter2 := NewGCounter()
		counter2.Increment("node1", 10) // Same node ID

		// Merge should take the maximum
		err := counter1.Merge(counter2)
		require.NoError(t, err)

		assert.Equal(t, uint64(10), counter1.Value())
	})

	t.Run("Merge is commutative", func(t *testing.T) {
		counter1 := NewGCounter()
		counter1.Increment("node1", 5)

		counter2 := NewGCounter()
		counter2.Increment("node2", 3)

		// Create copies for reverse merge
		counter1Copy := &GCounter{
			counters: make(map[NodeID]uint64),
		}
		for k, v := range counter1.counters {
			counter1Copy.counters[k] = v
		}

		counter2Copy := &GCounter{
			counters: make(map[NodeID]uint64),
		}
		for k, v := range counter2.counters {
			counter2Copy.counters[k] = v
		}

		// Merge in both directions
		if err := counter1.Merge(counter2); err != nil {
			t.Errorf("counter1.Merge failed: %v", err)
		}
		if err := counter2Copy.Merge(counter1Copy); err != nil {
			t.Errorf("counter2Copy.Merge failed: %v", err)
		}

		// Results should be the same
		assert.Equal(t, counter1.Value(), counter2Copy.Value())
	})

	t.Run("Merge with invalid type returns error", func(t *testing.T) {
		counter := NewGCounter()
		pnCounter := NewPNCounter()

		err := counter.Merge(pnCounter)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot merge GCounter")
	})

	t.Run("Concurrent increments", func(t *testing.T) {
		counter := NewGCounter()

		// Simulate concurrent increments
		done := make(chan bool, 3)

		go func() {
			for i := 0; i < 100; i++ {
				counter.Increment("node1", 1)
			}
			done <- true
		}()

		go func() {
			for i := 0; i < 100; i++ {
				counter.Increment("node2", 1)
			}
			done <- true
		}()

		go func() {
			for i := 0; i < 100; i++ {
				_ = counter.Value()
			}
			done <- true
		}()

		// Wait for all goroutines
		for i := 0; i < 3; i++ {
			<-done
		}

		// Should have counted all increments
		assert.Equal(t, uint64(200), counter.Value())
	})
}

func BenchmarkGCounterIncrement(b *testing.B) {
	counter := NewGCounter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.Increment("bench-node", 1)
	}
}

func BenchmarkGCounterMerge(b *testing.B) {
	counter1 := NewGCounter()
	counter1.Increment("node1", 1000)

	counter2 := NewGCounter()
	counter2.Increment("node2", 2000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = counter1.Merge(counter2)
	}
}
