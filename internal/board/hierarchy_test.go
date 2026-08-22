package board_test

import (
	"testing"

	"github.com/antopolskiy/kanban-md/internal/board"
	"github.com/antopolskiy/kanban-md/internal/task"
)

func ptr(i int) *int { return &i }

func TestDepthsNestedChain(t *testing.T) {
	tasks := []*task.Task{
		{ID: 1},                 // milestone
		{ID: 2, Parent: ptr(1)}, // epic
		{ID: 3, Parent: ptr(2)}, // story
		{ID: 4, Parent: ptr(3)}, // sub-story
		{ID: 5},                 // second root
	}

	got := board.Depths(tasks)

	want := map[int]int{1: 0, 2: 1, 3: 2, 4: 3, 5: 0}
	for id, depth := range want {
		if got[id] != depth {
			t.Errorf("Depths()[%d] = %d, want %d", id, got[id], depth)
		}
	}
}

func TestDepthsMissingParentIsRoot(t *testing.T) {
	// Parent 99 does not exist (e.g. deleted); the task must not vanish from
	// the depth map, it counts as a root.
	tasks := []*task.Task{{ID: 1, Parent: ptr(99)}}

	if got := board.Depths(tasks)[1]; got != 0 {
		t.Errorf("Depths()[1] = %d, want 0 for a dangling parent", got)
	}
}

func TestDepthsCycleTerminates(t *testing.T) {
	// A parent cycle must not hang or overflow the stack.
	tasks := []*task.Task{
		{ID: 1, Parent: ptr(2)},
		{ID: 2, Parent: ptr(1)},
		{ID: 3, Parent: ptr(3)}, // self-reference
	}

	got := board.Depths(tasks)

	if len(got) != 3 {
		t.Fatalf("Depths() has %d entries, want 3", len(got))
	}
	for id, depth := range got {
		if depth < 0 {
			t.Errorf("Depths()[%d] = %d, want a non-negative depth", id, depth)
		}
	}
}

func TestMaxDepth(t *testing.T) {
	tasks := []*task.Task{
		{ID: 1},
		{ID: 2, Parent: ptr(1)},
		{ID: 3, Parent: ptr(2)},
	}

	if got := board.MaxDepth(board.Depths(tasks)); got != 2 {
		t.Errorf("MaxDepth() = %d, want 2", got)
	}
	if got := board.MaxDepth(map[int]int{}); got != 0 {
		t.Errorf("MaxDepth(empty) = %d, want 0", got)
	}
}
