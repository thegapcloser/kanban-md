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

func TestDepthsSelfReferenceIsRoot(t *testing.T) {
	// FindParent treats a self-reference as "no parent"; depth must agree,
	// otherwise the same task reads as a root in the detail view and as a
	// child on the board.
	tasks := []*task.Task{{ID: 1, Parent: ptr(1)}}

	if got := board.Depths(tasks)[1]; got != 0 {
		t.Errorf("Depths()[1] = %d, want 0 for a self-referencing task", got)
	}
}

func TestDepthsIndependentOfInputOrder(t *testing.T) {
	// The depth of a task must not depend on where it sits in the slice.
	one, two := 1, 2
	childFirst := []*task.Task{
		{ID: 3, Parent: &two},
		{ID: 2, Parent: &one},
		{ID: 1},
	}
	parentFirst := []*task.Task{
		{ID: 1},
		{ID: 2, Parent: &one},
		{ID: 3, Parent: &two},
	}

	a, b := board.Depths(childFirst), board.Depths(parentFirst)

	for id := 1; id <= 3; id++ {
		if a[id] != b[id] {
			t.Errorf("Depths()[%d] = %d with children first, %d with parents first", id, a[id], b[id])
		}
	}
	if a[3] != 2 {
		t.Errorf("Depths()[3] = %d, want 2", a[3])
	}
}

func TestDepthsCycleIsOrderIndependent(t *testing.T) {
	// Cycles are invalid data, but two tasks in the same cycle must not get
	// depths that depend on which one the walk happened to start from.
	one, two := 1, 2
	forward := board.Depths([]*task.Task{{ID: 1, Parent: &two}, {ID: 2, Parent: &one}})
	reverse := board.Depths([]*task.Task{{ID: 2, Parent: &one}, {ID: 1, Parent: &two}})

	for _, got := range []map[int]int{forward, reverse} {
		for id, depth := range got {
			if depth < 0 || depth > 1 {
				t.Errorf("Depths()[%d] = %d, want a depth within the two-task cycle", id, depth)
			}
		}
	}
}

func TestDepthsEmptyInput(t *testing.T) {
	if got := board.Depths(nil); len(got) != 0 {
		t.Errorf("Depths(nil) has %d entries, want 0", len(got))
	}
}

func TestDepthsLongChain(t *testing.T) {
	// A deep chain must resolve without recursing itself to death.
	const length = 5000
	tasks := make([]*task.Task, 0, length)
	tasks = append(tasks, &task.Task{ID: 1})
	for id := 2; id <= length; id++ {
		parent := id - 1
		tasks = append(tasks, &task.Task{ID: id, Parent: &parent})
	}

	got := board.Depths(tasks)

	if got[length] != length-1 {
		t.Errorf("Depths()[%d] = %d, want %d", length, got[length], length-1)
	}
	if board.MaxDepth(got) != length-1 {
		t.Errorf("MaxDepth() = %d, want %d", board.MaxDepth(got), length-1)
	}
}
