package crdt

import (
	"testing"
	
	"github.com/stretchr/testify/assert"
)

func TestVectorClock(t *testing.T) {
	t.Run("New vector clock is empty", func(t *testing.T) {
		vc := NewVectorClock()
		assert.NotNil(t, vc)
		assert.Equal(t, 0, len(vc))
	})
	
	t.Run("Increment updates clock", func(t *testing.T) {
		vc := NewVectorClock()
		vc.Increment("node1")
		
		assert.Equal(t, uint64(1), vc["node1"])
		
		vc.Increment("node1")
		assert.Equal(t, uint64(2), vc["node1"])
		
		vc.Increment("node2")
		assert.Equal(t, uint64(1), vc["node2"])
	})
	
	t.Run("Update takes maximum values", func(t *testing.T) {
		vc1 := NewVectorClock()
		vc1["node1"] = 5
		vc1["node2"] = 3
		
		vc2 := NewVectorClock()
		vc2["node1"] = 3
		vc2["node2"] = 5
		vc2["node3"] = 1
		
		vc1.Update(vc2)
		
		assert.Equal(t, uint64(5), vc1["node1"])
		assert.Equal(t, uint64(5), vc1["node2"])
		assert.Equal(t, uint64(1), vc1["node3"])
	})
	
	t.Run("HappensBefore detects causality", func(t *testing.T) {
		// vc1 happens before vc2
		vc1 := VectorClock{"node1": 1, "node2": 2}
		vc2 := VectorClock{"node1": 2, "node2": 3}
		
		assert.True(t, vc1.HappensBefore(vc2))
		assert.False(t, vc2.HappensBefore(vc1))
		
		// Concurrent clocks (neither happens before the other)
		vc3 := VectorClock{"node1": 2, "node2": 1}
		vc4 := VectorClock{"node1": 1, "node2": 2}
		
		assert.False(t, vc3.HappensBefore(vc4))
		assert.False(t, vc4.HappensBefore(vc3))
		
		// Equal clocks
		vc5 := VectorClock{"node1": 1, "node2": 2}
		vc6 := VectorClock{"node1": 1, "node2": 2}
		
		assert.False(t, vc5.HappensBefore(vc6))
		assert.False(t, vc6.HappensBefore(vc5))
	})
	
	t.Run("Concurrent detects concurrent clocks", func(t *testing.T) {
		// Concurrent clocks
		vc1 := VectorClock{"node1": 2, "node2": 1}
		vc2 := VectorClock{"node1": 1, "node2": 2}
		
		assert.True(t, vc1.Concurrent(vc2))
		assert.True(t, vc2.Concurrent(vc1))
		
		// Not concurrent (vc3 happens before vc4)
		vc3 := VectorClock{"node1": 1, "node2": 2}
		vc4 := VectorClock{"node1": 2, "node2": 3}
		
		assert.False(t, vc3.Concurrent(vc4))
		assert.False(t, vc4.Concurrent(vc3))
		
		// Equal clocks are concurrent (neither happens before the other)
		vc5 := VectorClock{"node1": 1, "node2": 2}
		vc6 := VectorClock{"node1": 1, "node2": 2}
		assert.True(t, vc5.Concurrent(vc6))
	})
	
	t.Run("Clone creates independent copy", func(t *testing.T) {
		vc1 := VectorClock{"node1": 1, "node2": 2}
		vc2 := vc1.Clone()
		
		// Should be equal
		assert.Equal(t, vc1["node1"], vc2["node1"])
		assert.Equal(t, vc1["node2"], vc2["node2"])
		
		// But independent
		vc2.Increment("node1")
		assert.Equal(t, uint64(1), vc1["node1"])
		assert.Equal(t, uint64(2), vc2["node1"])
	})
}

func BenchmarkVectorClockIncrement(b *testing.B) {
	vc := NewVectorClock()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vc.Increment("node1")
	}
}

func BenchmarkVectorClockUpdate(b *testing.B) {
	vc1 := VectorClock{
		"node1": 100,
		"node2": 200,
		"node3": 300,
	}
	
	vc2 := VectorClock{
		"node1": 150,
		"node2": 150,
		"node4": 100,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vc3 := vc1.Clone()
		vc3.Update(vc2)
	}
}