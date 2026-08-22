package board

import "github.com/antopolskiy/kanban-md/internal/task"

// Depths returns the hierarchy depth of every task, keyed by task ID.
//
// Depth is derived from the parent chain alone: a task without a resolvable
// parent has depth 0, its children depth 1, and so on. With the common
// milestone/epic/story layout that makes depth 0 the milestones, depth 1 the
// epics and depth 2 the stories, without the board needing a task-type field.
//
// A parent that no longer exists is treated as absent, so an orphaned task
// stays visible as a root rather than dropping out of the map. Parent cycles
// (including self-references) terminate at the first repeated ID.
func Depths(tasks []*task.Task) map[int]int {
	byID := make(map[int]*task.Task, len(tasks))
	for _, t := range tasks {
		byID[t.ID] = t
	}

	depths := make(map[int]int, len(tasks))
	for _, t := range tasks {
		depths[t.ID] = depthOf(t, byID, depths)
	}
	return depths
}

// depthOf walks the parent chain of t, memoizing resolved depths in depths.
// seen guards against cycles: a repeated ID ends the walk at the current depth.
func depthOf(t *task.Task, byID map[int]*task.Task, depths map[int]int) int {
	seen := map[int]bool{}
	depth := 0
	for current := t; ; depth++ {
		if seen[current.ID] {
			return depth
		}
		seen[current.ID] = true

		if cached, ok := depths[current.ID]; ok && current != t {
			return depth + cached
		}
		if current.Parent == nil {
			return depth
		}
		parent, ok := byID[*current.Parent]
		if !ok {
			return depth
		}
		current = parent
	}
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
