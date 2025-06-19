package crdt

import (
	"testing"
	
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPNCounter(t *testing.T) {
	t.Run("New counter starts at zero", func(t *testing.T) {
		counter := NewPNCounter()
		assert.Equal(t, int64(0), counter.Value())
	})
	
	t.Run("Increment and decrement", func(t *testing.T) {
		counter := NewPNCounter()
		
		counter.Increment("node1", 10)
		assert.Equal(t, int64(10), counter.Value())
		
		counter.Decrement("node1", 3)
		assert.Equal(t, int64(7), counter.Value())
		
		counter.Decrement("node1", 10)
		assert.Equal(t, int64(-3), counter.Value())
	})
	
	t.Run("Merge combines increments and decrements", func(t *testing.T) {
		counter1 := NewPNCounter()
		counter1.Increment("node1", 10)
		counter1.Decrement("node1", 3)
		
		counter2 := NewPNCounter()
		counter2.Increment("node2", 5)
		counter2.Decrement("node2", 2)
		
		err := counter1.Merge(counter2)
		require.NoError(t, err)
		
		// Should be (10 + 5) - (3 + 2) = 10
		assert.Equal(t, int64(10), counter1.Value())
	})
	
	t.Run("Merge is idempotent", func(t *testing.T) {
		counter1 := NewPNCounter()
		counter1.Increment("node1", 10)
		
		counter2 := NewPNCounter()
		counter2.Increment("node2", 5)
		
		// Merge multiple times
		err := counter1.Merge(counter2)
		require.NoError(t, err)
		firstValue := counter1.Value()
		
		err = counter1.Merge(counter2)
		require.NoError(t, err)
		secondValue := counter1.Value()
		
		assert.Equal(t, firstValue, secondValue)
	})
	
	t.Run("Concurrent operations", func(t *testing.T) {
		counter := NewPNCounter()
		
		done := make(chan bool, 4)
		
		// Incrementers
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
		
		// Decrementers
		go func() {
			for i := 0; i < 50; i++ {
				counter.Decrement("node3", 1)
			}
			done <- true
		}()
		
		// Reader
		go func() {
			for i := 0; i < 100; i++ {
				_ = counter.Value()
			}
			done <- true
		}()
		
		// Wait for all
		for i := 0; i < 4; i++ {
			<-done
		}
		
		// Should be 200 - 50 = 150
		assert.Equal(t, int64(150), counter.Value())
	})
	
	t.Run("Merge with wrong type returns error", func(t *testing.T) {
		pnCounter := NewPNCounter()
		gCounter := NewGCounter()
		
		err := pnCounter.Merge(gCounter)
		assert.Error(t, err)
	})
}