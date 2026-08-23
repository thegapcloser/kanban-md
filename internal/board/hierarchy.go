package board

import (
	"sort"

	"github.com/antopolskiy/kanban-md/internal/config"
	"github.com/antopolskiy/kanban-md/internal/task"
)

// HierarchyIndex answers parent, child and depth questions about a task set in
// constant time per lookup. It is built once per load, so a view that walks the
// hierarchy per render does not scan the task list per row.
//
// Hierarchy is derived from the parent chain alone: a task without a resolvable
// parent has depth 0, its children depth 1, and so on. With the common
// milestone/epic/story layout that makes depth 0 the milestones, depth 1 the
// epics and depth 2 the stories, without the board needing a task-type field.
//
// Invalid parent links are tolerated so a broken board still renders, and one
// rule covers every walk in this file, upwards and downwards: a parent that no
// longer exists and a self-reference both count as no parent, matching
// FindParent, and a chain is cut at its first repeated task. Depths therefore do
// not depend on the order of tasks, and Tree renders no task twice.
type HierarchyIndex struct {
	tasks    []*task.Task
	byID     map[int]*task.Task
	children map[int][]*task.Task
}

// NewHierarchyIndex indexes a task set by ID and by parent ID. Archived tasks
// are included: an archived ancestor still has to resolve. Callers that must
// not show archived tasks filter them per walk.
func NewHierarchyIndex(tasks []*task.Task) *HierarchyIndex {
	ix := &HierarchyIndex{
		tasks:    tasks,
		byID:     make(map[int]*task.Task, len(tasks)),
		children: make(map[int][]*task.Task),
	}
	for _, t := range tasks {
		ix.byID[t.ID] = t
	}
	for _, t := range tasks {
		if t.Parent == nil || *t.Parent == t.ID {
			continue
		}
		ix.children[*t.Parent] = append(ix.children[*t.Parent], t)
	}
	for _, bucket := range ix.children {
		sort.Slice(bucket, func(i, j int) bool { return bucket[i].ID < bucket[j].ID })
	}
	return ix
}

// Task returns the indexed task with the given ID, or nil when it is unknown.
func (ix *HierarchyIndex) Task(id int) *task.Task {
	return ix.byID[id]
}

// Depths returns the hierarchy depth of every indexed task, keyed by task ID.
// See HierarchyIndex for how depth is derived and how broken links are handled.
func (ix *HierarchyIndex) Depths() map[int]int {
	depths := make(map[int]int, len(ix.tasks))
	for _, t := range ix.tasks {
		if _, resolved := depths[t.ID]; !resolved {
			resolveDepth(t, ix.byID, depths)
		}
	}
	return depths
}

// Depths returns the hierarchy depth of every task, keyed by task ID. It is the
// one-shot form of HierarchyIndex.Depths for callers that hold no index.
func Depths(tasks []*task.Task) map[int]int {
	return NewHierarchyIndex(tasks).Depths()
}

// HierarchyRow is one line of a HierarchyTree. Depth is relative to the topmost
// shown row, which is depth 0. Last reports whether the row is the last shown
// sibling on its level, Current marks the task the tree was built for. Done and
// Total count direct, non-archived children in a terminal status over direct,
// non-archived children, exactly as SummarizeChildren does.
type HierarchyRow struct {
	ID      int
	Title   string
	Status  string
	Depth   int
	Last    bool
	Current bool
	Done    int
	Total   int
}

// HierarchyTree is the ancestor path of a task, the task itself and its
// descendants, in reading order. CutAbove and CutBelow report that the tree
// continues beyond the topmost respectively the deepest shown row.
type HierarchyTree struct {
	Rows     []HierarchyRow
	CutAbove bool
	CutBelow bool
}

// Tree builds the hierarchy around a task: at most levels ancestors above it and
// at most levels of descendants below it, so one number governs both directions.
// Archived descendants are left out, an archived ancestor is shown as the last
// row and ends the chain. An unknown task yields an empty tree.
func (ix *HierarchyIndex) Tree(taskID int, cfg *config.Config, levels int) HierarchyTree {
	current := ix.byID[taskID]
	if current == nil {
		return HierarchyTree{}
	}
	if levels < 0 {
		levels = 0
	}

	ancestors, cutAbove := ix.ancestorRows(current, cfg, levels)
	onPath := make(map[int]bool, len(ancestors)+1)
	for _, row := range ancestors {
		onPath[row.ID] = true
	}
	onPath[current.ID] = true

	rows := make([]HierarchyRow, 0, len(ancestors)+1)
	rows = append(rows, ancestors...)
	rows = append(rows, ix.row(current, len(ancestors), true, true, cfg))
	rows = append(rows, ix.descendantRows(current, cfg, levels, len(ancestors)+1, onPath)...)

	return HierarchyTree{Rows: rows, CutAbove: cutAbove, CutBelow: ix.cutBelow(rows, cfg)}
}

// ancestorRows walks up from a task and returns its shown ancestors, outermost
// first. It also reports whether the level budget, not the top of the board, is
// what ended the chain: an ancestor the chain ran out of levels for is cut off
// and gets a marker, while a missing or repeated ancestor ends the chain without
// one, because there is nothing more to reach, and an archived one ends it as
// the last shown row, which is the end made visible.
func (ix *HierarchyIndex) ancestorRows(
	current *task.Task,
	cfg *config.Config,
	levels int,
) ([]HierarchyRow, bool) {
	onPath := map[int]bool{current.ID: true}
	chain := make([]*task.Task, 0, ancestorChainCap(levels, len(ix.byID))) // nearest ancestor first
	node := current

	for len(chain) < levels {
		parent, ok := resolveParent(node, ix.byID)
		if !ok || onPath[parent.ID] {
			return ancestorRowsFrom(ix, chain, cfg), false
		}
		onPath[parent.ID] = true
		chain = append(chain, parent)
		if cfg.IsArchivedStatus(parent.Status) {
			return ancestorRowsFrom(ix, chain, cfg), false
		}
		node = parent
	}

	parent, ok := resolveParent(node, ix.byID)
	cut := ok && !onPath[parent.ID]
	return ancestorRowsFrom(ix, chain, cfg), cut
}

// ancestorChainCap bounds the capacity the ancestor chain is allocated with.
// levels comes from the config and has no maximum on purpose, while a chain can
// never hold more tasks than the index does — sizing by levels alone turns a
// large setting into a large allocation on every render.
func ancestorChainCap(levels, indexed int) int {
	return min(levels, indexed)
}

// ancestorRowsFrom turns a nearest-first ancestor chain into rows in reading
// order. Every ancestor is the last shown row on its level, because the tree
// never shows an ancestor's siblings.
func ancestorRowsFrom(ix *HierarchyIndex, chain []*task.Task, cfg *config.Config) []HierarchyRow {
	rows := make([]HierarchyRow, 0, len(chain))
	for i := len(chain) - 1; i >= 0; i-- {
		rows = append(rows, ix.row(chain[i], len(chain)-1-i, true, false, cfg))
	}
	return rows
}

// descendantRows walks down from a task in preorder, siblings in ascending task
// ID. baseDepth is the depth of its direct children. onPath carries the ancestor
// rows and the open task, and skipping a child that sits on it is what ends a
// cycle: a task has at most one parent, so the descent reaches every task at
// most once and the only path it can run into is the one it started from.
func (ix *HierarchyIndex) descendantRows(
	root *task.Task,
	cfg *config.Config,
	levels, baseDepth int,
	onPath map[int]bool,
) []HierarchyRow {
	var rows []HierarchyRow

	var walk func(node *task.Task, rel int)
	walk = func(node *task.Task, rel int) {
		if rel >= levels {
			return // the level budget is spent; cutBelow reports what is left
		}
		shown := make([]*task.Task, 0, len(ix.children[node.ID]))
		for _, kid := range ix.activeChildren(node.ID, cfg) {
			if !onPath[kid.ID] {
				shown = append(shown, kid)
			}
		}
		for i, kid := range shown {
			rows = append(rows, ix.row(kid, baseDepth+rel, i == len(shown)-1, false, cfg))
			walk(kid, rel+1)
		}
	}
	walk(root, 0)

	return rows
}

// cutBelow reports whether the tree continues below its deepest shown level. One
// marker per side is enough: which node exactly has more children is what the
// per-row counter says.
func (ix *HierarchyIndex) cutBelow(rows []HierarchyRow, cfg *config.Config) bool {
	deepest := 0
	for _, row := range rows {
		if row.Depth > deepest {
			deepest = row.Depth
		}
	}
	for _, row := range rows {
		if row.Depth == deepest && ix.hasActiveChild(row.ID, cfg) {
			return true
		}
	}
	return false
}

// hasActiveChild reports whether a task has a direct, non-archived child. It
// answers the question cutBelow asks without building a slice per row.
func (ix *HierarchyIndex) hasActiveChild(id int, cfg *config.Config) bool {
	for _, kid := range ix.children[id] {
		if !cfg.IsArchivedStatus(kid.Status) {
			return true
		}
	}
	return false
}

// activeChildren returns the direct, non-archived children of a task in
// ascending task-ID order.
func (ix *HierarchyIndex) activeChildren(id int, cfg *config.Config) []*task.Task {
	bucket := ix.children[id]
	active := make([]*task.Task, 0, len(bucket))
	for _, kid := range bucket {
		if !cfg.IsArchivedStatus(kid.Status) {
			active = append(active, kid)
		}
	}
	return active
}

// row builds one tree row, including its child counts.
func (ix *HierarchyIndex) row(
	t *task.Task,
	depth int,
	last, current bool,
	cfg *config.Config,
) HierarchyRow {
	done, total := ix.childCounts(t.ID, cfg)
	return HierarchyRow{
		ID:      t.ID,
		Title:   t.Title,
		Status:  t.Status,
		Depth:   depth,
		Last:    last,
		Current: current,
		Done:    done,
		Total:   total,
	}
}

// childCounts counts direct, non-archived children and how many of them are in a
// terminal status, matching SummarizeChildren.
func (ix *HierarchyIndex) childCounts(id int, cfg *config.Config) (int, int) {
	done, total := 0, 0
	for _, kid := range ix.children[id] {
		if cfg.IsArchivedStatus(kid.Status) {
			continue
		}
		total++
		if cfg.IsTerminalStatus(kid.Status) {
			done++
		}
	}
	return done, total
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
