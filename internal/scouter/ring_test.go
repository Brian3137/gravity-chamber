package scouter

import (
	"fmt"
	"testing"
)

func TestRingRemapsFewKeysOnNodeLoss(t *testing.T) {
	keys := make([]string, 100000)
	for i := range keys {
		keys[i] = fmt.Sprintf("fighter_%d", i)
	}

	ring := NewRing(150, "cache-a", "cache-b", "cache-c", "cache-d")
	before := make(map[string]string, len(keys))
	for _, k := range keys {
		before[k] = ring.Get(k)
	}

	ring.Remove("cache-c")

	moved := 0
	for _, k := range keys {
		if ring.Get(k) != before[k] {
			moved++
		}
	}
	pct := float64(moved) / float64(len(keys)) * 100
	t.Logf("consistent hashing: %.1f%% of keys moved when 1 of 4 nodes was removed", pct)

	// Losing 1 of 4 nodes should move ~25% — the keys that node owned.
	if pct > 35 {
		t.Errorf("moved %.1f%%, expected ~25%%", pct)
	}
}

func TestModuloRemapsAlmostEverything(t *testing.T) {
	// The comparison that makes the point.
	moved := 0
	const n = 100000
	for i := 0; i < n; i++ {
		h := xxhashString(fmt.Sprintf("fighter_%d", i))
		if h%4 != h%3 {
			moved++
		}
	}
	t.Logf("modulo hashing: %.1f%% of keys moved when going from 4 to 3 nodes",
		float64(moved)/n*100)
}