package crdt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLWWRegister(t *testing.T) {
	t.Run("New register has nil value", func(t *testing.T) {
		reg := NewLWWRegister()
		assert.Nil(t, reg.Get())
	})

	t.Run("Set updates value with timestamp", func(t *testing.T) {
		reg := NewLWWRegister()

		value1 := "first value"
		reg.Set(value1, time.Now(), "node1")
		assert.Equal(t, value1, reg.Get())

		// Later timestamp wins
		value2 := "second value"
		reg.Set(value2, time.Now().Add(time.Second), "node2")
		assert.Equal(t, value2, reg.Get())

		// Earlier timestamp is ignored
		value3 := "third value"
		reg.Set(value3, time.Now().Add(-time.Minute), "node3")
		assert.Equal(t, value2, reg.Get()) // Still second value
	})

	t.Run("Tie-breaking with node ID", func(t *testing.T) {
		reg := NewLWWRegister()
		timestamp := time.Now()

		// Same timestamp, different nodes
		reg.Set("value from node2", timestamp, "node2")
		reg.Set("value from node1", timestamp, "node1")

		// Should use node ID for tie-breaking (node2 > node1)
		assert.Equal(t, "value from node2", reg.Get())
	})

	t.Run("Merge combines registers", func(t *testing.T) {
		reg1 := NewLWWRegister()
		reg2 := NewLWWRegister()

		now := time.Now()
		reg1.Set("value1", now, "node1")
		reg2.Set("value2", now.Add(time.Second), "node2")

		err := reg1.Merge(reg2)
		require.NoError(t, err)

		// Should have the later value
		assert.Equal(t, "value2", reg1.Get())
	})

	t.Run("Merge is idempotent", func(t *testing.T) {
		reg1 := NewLWWRegister()
		reg2 := NewLWWRegister()

		reg1.Set("value1", time.Now(), "node1")
		reg2.Set("value2", time.Now().Add(time.Second), "node2")

		// Merge multiple times
		err := reg1.Merge(reg2)
		require.NoError(t, err)
		firstValue := reg1.Get()

		err = reg1.Merge(reg2)
		require.NoError(t, err)
		secondValue := reg1.Get()

		assert.Equal(t, firstValue, secondValue)
	})

	t.Run("Merge with wrong type returns error", func(t *testing.T) {
		reg := NewLWWRegister()
		counter := NewGCounter()

		err := reg.Merge(counter)
		assert.Error(t, err)
	})

	t.Run("Concurrent sets", func(t *testing.T) {
		reg := NewLWWRegister()
		done := make(chan bool, 3)

		go func() {
			for i := 0; i < 100; i++ {
				reg.Set(i, time.Now(), "node1")
			}
			done <- true
		}()

		go func() {
			for i := 0; i < 100; i++ {
				reg.Set(i+1000, time.Now(), "node2")
			}
			done <- true
		}()

		go func() {
			for i := 0; i < 100; i++ {
				_ = reg.Get()
			}
			done <- true
		}()

		// Wait for all
		for i := 0; i < 3; i++ {
			<-done
		}

		// Should have some value
		assert.NotNil(t, reg.Get())
	})
}

func BenchmarkLWWRegisterSet(b *testing.B) {
	reg := NewLWWRegister()
	now := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reg.Set(i, now.Add(time.Duration(i)), "node1")
	}
}

func BenchmarkLWWRegisterMerge(b *testing.B) {
	reg1 := NewLWWRegister()
	reg2 := NewLWWRegister()

	now := time.Now()
	reg1.Set("value1", now, "node1")
	reg2.Set("value2", now.Add(time.Second), "node2")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = reg1.Merge(reg2)
	}
}
