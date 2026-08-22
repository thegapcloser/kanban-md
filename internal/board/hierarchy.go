package board

import "github.com/antopolskiy/kanban-md/internal/task"

// Depths returns the hierarchy depth of every task, keyed by task ID.
//
// Depth is derived from the parent chain alone: a task without a resolvable
// parent has depth 0, its children depth 1, and so on. With the common
// milestone/epic/story layout that makes depth 0 the milestones, depth 1 the
// epics and depth 2 the stories, without the board needing a task-type field.
//
// Invalid parent links are tolerated so a broken board still renders. A parent
// that no longer exists and a self-reference both count as no parent, matching
// FindParent. A parent cycle is cut at its first repeated task, which then
// counts as a root; the result does not depend on the order of tasks.
func Depths(tasks []*task.Task) map[int]int {
	byID := make(map[int]*task.Task, len(tasks))
	for _, t := range tasks {
		byID[t.ID] = t
	}

	depths := make(map[int]int, len(tasks))
	for _, t := range tasks {
		if _, resolved := depths[t.ID]; !resolved {
			resolveDepth(t, byID, depths)
		}
	}
	return depths
}

// resolveDepth walks from t up to the top of its parent chain, then writes the
// depth of every task on that chain into depths. Collecting the chain first
// keeps the walk iterative (deep trees do not grow the stack) and keeps every
// task on it consistent with the top it was reached from.
func resolveDepth(t *task.Task, byID map[int]*task.Task, depths map[int]int) {
	var chain []*task.Task
	onChain := make(map[int]bool)
	base := 0

	for current := t; ; {
		if resolved, ok := depths[current.ID]; ok {
			// Reached an ancestor whose depth is already known; the last task
			// collected is its child.
			base = resolved + 1
			break
		}
		if onChain[current.ID] {
			break // cycle: the chain collected so far tops out at a root
		}
		onChain[current.ID] = true
		chain = append(chain, current)

		parent, ok := resolveParent(current, byID)
		if !ok {
			break
		}
		current = parent
	}

	// chain runs from t up to its top, so the last entry sits at base.
	for i, node := range chain {
		depths[node.ID] = base + len(chain) - 1 - i
	}
}

// resolveParent returns a task's parent, reporting false when the task is a
// root: no parent set, a self-reference, or a parent that no longer exists.
func resolveParent(t *task.Task, byID map[int]*task.Task) (*task.Task, bool) {
	if t.Parent == nil || *t.Parent == t.ID {
		return nil, false
	}
	parent, ok := byID[*t.Parent]
	return parent, ok
}

// MaxDepth returns the deepest level present in a depth map, or 0 when empty.
func MaxDepth(depths map[int]int) int {
	maxDepth := 0
	for _, d := range depths {
		if d > maxDepth {
			maxDepth = d
		}
	}
	return maxDepth
}
