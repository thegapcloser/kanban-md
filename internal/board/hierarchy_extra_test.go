package board

import (
	"math"
	"testing"
)

func TestAncestorChainCapIsBoundedByTheIndexedTasks(t *testing.T) {
	// hierarchy_levels has no maximum, so the level budget alone must never size
	// an allocation: math.MaxInt32 levels on a three-task board would ask for
	// 16 GiB per render.
	const indexed = 3

	if got := ancestorChainCap(math.MaxInt32, indexed); got != indexed {
		t.Errorf("ancestorChainCap(math.MaxInt32, %d) = %d, want %d — the budget sized the chain",
			indexed, got, indexed)
	}
	if got := ancestorChainCap(2, indexed); got != 2 {
		t.Errorf("ancestorChainCap(2, %d) = %d, want 2 — a budget below the task count still bounds it",
			indexed, got)
	}
}
