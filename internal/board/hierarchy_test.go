package board_test

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/antopolskiy/kanban-md/internal/board"
	"github.com/antopolskiy/kanban-md/internal/config"
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

// Status names and expected row summaries used by the hierarchy-tree fixtures.
const (
	statusBacklog = "backlog"
	statusTodo    = "todo"
	statusDone    = "done"

	rowRootAsAncestor = "d0/#1/todo/Root One/last=true/current=false/1-2"
	rowRootAsCurrent  = "d0/#1/todo/Root One/last=true/current=true/1-2"
	rowMidTwo         = "d1/#2/done/Mid Two/last=false/current=false/2-2"
	rowMidThree       = "d1/#3/backlog/Mid Three/last=true/current=false/0-1"
)

// hierarchyFixture is the three-level shape shared by the tree tests: one root
// with two children, the first of which has two children of its own.
func hierarchyFixture() []*task.Task {
	return []*task.Task{
		{ID: 1, Title: "Root One", Status: statusTodo},
		{ID: 2, Title: "Mid Two", Status: statusDone, Parent: ptr(1)},
		{ID: 3, Title: "Mid Three", Status: statusBacklog, Parent: ptr(1)},
		{ID: 4, Title: "Leaf Four", Status: statusDone, Parent: ptr(2)},
		{ID: 5, Title: "Leaf Five", Status: statusDone, Parent: ptr(2)},
		{ID: 6, Title: "Leaf Six", Status: statusBacklog, Parent: ptr(3)},
	}
}

// chainFixture builds a single parent chain of length tasks, #1 at the top.
func chainFixture(length int) []*task.Task {
	tasks := make([]*task.Task, 0, length)
	tasks = append(tasks, &task.Task{ID: 1, Title: "Level 1", Status: statusBacklog})
	for id := 2; id <= length; id++ {
		tasks = append(tasks, &task.Task{
			ID:     id,
			Title:  "Level " + strconv.Itoa(id),
			Status: statusBacklog,
			Parent: ptr(id - 1),
		})
	}
	return tasks
}

// rowSummary renders a tree row as one comparable string so a whole tree can be
// asserted in one line without reimplementing the walk in the test.
func rowSummary(r board.HierarchyRow) string {
	return fmt.Sprintf("d%d/#%d/%s/%s/last=%t/current=%t/%d-%d",
		r.Depth, r.ID, r.Status, r.Title, r.Last, r.Current, r.Done, r.Total)
}

func treeSummary(tree board.HierarchyTree) []string {
	out := make([]string, 0, len(tree.Rows))
	for _, r := range tree.Rows {
		out = append(out, rowSummary(r))
	}
	return out
}

// treeCase is one expectation of the symmetry table.
type treeCase struct {
	name     string
	taskID   int
	levels   int
	wantRows []string
	cutAbove bool
	cutBelow bool
}

// symmetryCases spells out the whole tree for every level budget, so the
// expectation is a literal picture and not a second implementation of the walk.
func symmetryCases() []treeCase {
	return []treeCase{
		{
			name:     "zero levels shows only the open ticket",
			taskID:   4,
			levels:   0,
			wantRows: []string{"d0/#4/done/Leaf Four/last=true/current=true/0-0"},
			cutAbove: true,
		},
		{
			name:   "one level up and down",
			taskID: 4,
			levels: 1,
			wantRows: []string{
				"d0/#2/done/Mid Two/last=true/current=false/2-2",
				"d1/#4/done/Leaf Four/last=true/current=true/0-0",
			},
			cutAbove: true,
		},
		{
			name:   "two levels reach the root",
			taskID: 4,
			levels: 2,
			wantRows: []string{
				rowRootAsAncestor,
				"d1/#2/done/Mid Two/last=true/current=false/2-2",
				"d2/#4/done/Leaf Four/last=true/current=true/0-0",
			},
		},
		{
			name:   "three levels cannot reach further than the root",
			taskID: 4,
			levels: 3,
			wantRows: []string{
				rowRootAsAncestor,
				"d1/#2/done/Mid Two/last=true/current=false/2-2",
				"d2/#4/done/Leaf Four/last=true/current=true/0-0",
			},
		},
		{
			name:   "root with two levels of descendants in preorder",
			taskID: 1,
			levels: 2,
			wantRows: []string{
				rowRootAsCurrent,
				rowMidTwo,
				"d2/#4/done/Leaf Four/last=false/current=false/0-0",
				"d2/#5/done/Leaf Five/last=true/current=false/0-0",
				rowMidThree,
				"d2/#6/backlog/Leaf Six/last=true/current=false/0-0",
			},
		},
		{
			name:   "root with one level cuts below",
			taskID: 1,
			levels: 1,
			wantRows: []string{
				rowRootAsCurrent,
				rowMidTwo,
				rowMidThree,
			},
			cutBelow: true,
		},
		{
			name:     "negative levels behave like zero",
			taskID:   1,
			levels:   -3,
			wantRows: []string{rowRootAsCurrent},
			cutBelow: true,
		},
		{
			name:   "middle node sees ancestor and descendants at once",
			taskID: 2,
			levels: 1,
			wantRows: []string{
				rowRootAsAncestor,
				"d1/#2/done/Mid Two/last=true/current=true/2-2",
				"d2/#4/done/Leaf Four/last=false/current=false/0-0",
				"d2/#5/done/Leaf Five/last=true/current=false/0-0",
			},
		},
	}
}

func TestHierarchyTreeAncestorsAndDescendantsAreSymmetric(t *testing.T) {
	ix := board.NewHierarchyIndex(hierarchyFixture())
	cfg := config.NewDefault("test")

	for _, tc := range symmetryCases() {
		t.Run(tc.name, func(t *testing.T) {
			tree := ix.Tree(tc.taskID, cfg, tc.levels)

			got := treeSummary(tree)
			if !reflect.DeepEqual(got, tc.wantRows) {
				t.Errorf("Tree(%d, %d) rows =\n%s\nwant\n%s",
					tc.taskID, tc.levels,
					strings.Join(got, "\n"), strings.Join(tc.wantRows, "\n"))
			}
			if tree.CutAbove != tc.cutAbove {
				t.Errorf("CutAbove = %t, want %t", tree.CutAbove, tc.cutAbove)
			}
			if tree.CutBelow != tc.cutBelow {
				t.Errorf("CutBelow = %t, want %t", tree.CutBelow, tc.cutBelow)
			}
		})
	}
}

func TestHierarchyTreeSixLevelsWithoutTypeNames(t *testing.T) {
	// Depth comes from the parent chain alone, so six levels work on a board
	// that has no milestone/epic/story notion at all.
	tasks := chainFixture(7)
	ix := board.NewHierarchyIndex(tasks)
	cfg := config.NewDefault("test")

	down := ix.Tree(1, cfg, 6)
	if len(down.Rows) != 7 {
		t.Fatalf("Tree(1, 6) has %d rows, want 7: %v", len(down.Rows), treeSummary(down))
	}
	for i, r := range down.Rows {
		if r.ID != i+1 || r.Depth != i {
			t.Errorf("row %d = %s, want #%d at depth %d", i, rowSummary(r), i+1, i)
		}
	}
	if down.CutAbove || down.CutBelow {
		t.Errorf("Tree(1, 6) cut flags = above %t, below %t, want both false", down.CutAbove, down.CutBelow)
	}

	up := ix.Tree(7, cfg, 6)
	if len(up.Rows) != 7 {
		t.Fatalf("Tree(7, 6) has %d rows, want 7: %v", len(up.Rows), treeSummary(up))
	}
	if !up.Rows[6].Current || up.Rows[6].ID != 7 {
		t.Errorf("last row = %s, want the open ticket #7", rowSummary(up.Rows[6]))
	}
	if up.CutAbove || up.CutBelow {
		t.Errorf("Tree(7, 6) cut flags = above %t, below %t, want both false", up.CutAbove, up.CutBelow)
	}
}

func TestHierarchyTreeCountsMatchSummarizeChildren(t *testing.T) {
	// The counter keeps the meaning it has in `show`: direct, non-archived
	// children in a terminal status over direct, non-archived children.
	tasks := append(hierarchyFixture(), &task.Task{
		ID: 7, Title: "Archived Leaf", Status: config.ArchivedStatus, Parent: ptr(3),
	})
	ix := board.NewHierarchyIndex(tasks)
	cfg := config.NewDefault("test")

	for _, tk := range tasks {
		want := board.SummarizeChildren(tasks, tk.ID, cfg, false)
		tree := ix.Tree(tk.ID, cfg, 0)
		if len(tree.Rows) != 1 {
			t.Fatalf("Tree(%d, 0) has %d rows, want 1", tk.ID, len(tree.Rows))
		}
		row := tree.Rows[0]
		if row.Done != want.Done || row.Total != want.Total() {
			t.Errorf("Tree(%d).Rows[0] = %d/%d done, SummarizeChildren = %d/%d done",
				tk.ID, row.Done, row.Total, want.Done, want.Total())
		}
	}
}

func TestHierarchyTreeArchivedAncestorEndsChain(t *testing.T) {
	tasks := []*task.Task{
		{ID: 1, Title: "Root", Status: statusBacklog},
		{ID: 2, Title: "Archived Mid", Status: config.ArchivedStatus, Parent: ptr(1)},
		{ID: 3, Title: "Leaf", Status: statusBacklog, Parent: ptr(2)},
	}
	ix := board.NewHierarchyIndex(tasks)
	cfg := config.NewDefault("test")

	tree := ix.Tree(3, cfg, 3)

	want := []string{
		"d0/#2/archived/Archived Mid/last=true/current=false/0-1",
		"d1/#3/backlog/Leaf/last=true/current=true/0-0",
	}
	if got := treeSummary(tree); !reflect.DeepEqual(got, want) {
		t.Errorf("rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if tree.CutAbove {
		t.Error("CutAbove = true, want false: an archived ancestor ends the chain without an ellipsis")
	}
}

func TestHierarchyTreeArchivedAncestorBeyondTheBudgetIsCutAbove(t *testing.T) {
	// The chain ends at an archived ancestor without a marker only when it
	// actually reached that ancestor. Ran the level budget out below it, the
	// archived row is itself cut off and the marker says so.
	tasks := []*task.Task{
		{ID: 1, Title: "Budget Root", Status: config.ArchivedStatus},
		{ID: 2, Title: "Budget Mid", Status: statusBacklog, Parent: ptr(1)},
		{ID: 3, Title: "Budget Leaf", Status: statusBacklog, Parent: ptr(2)},
	}
	ix := board.NewHierarchyIndex(tasks)
	cfg := config.NewDefault("test")

	cut := ix.Tree(3, cfg, 1)
	wantCut := []string{
		"d0/#2/backlog/Budget Mid/last=true/current=false/0-1",
		"d1/#3/backlog/Budget Leaf/last=true/current=true/0-0",
	}
	if got := treeSummary(cut); !reflect.DeepEqual(got, wantCut) {
		t.Errorf("Tree(3, 1) rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(wantCut, "\n"))
	}
	if !cut.CutAbove {
		t.Error("Tree(3, 1) CutAbove = false, want true: the archived root is cut off, not reached")
	}

	reached := ix.Tree(3, cfg, 2)
	wantReached := []string{
		"d0/#1/archived/Budget Root/last=true/current=false/0-1",
		"d1/#2/backlog/Budget Mid/last=true/current=false/0-1",
		"d2/#3/backlog/Budget Leaf/last=true/current=true/0-0",
	}
	if got := treeSummary(reached); !reflect.DeepEqual(got, wantReached) {
		t.Errorf("Tree(3, 2) rows =\n%s\nwant\n%s",
			strings.Join(got, "\n"), strings.Join(wantReached, "\n"))
	}
	if reached.CutAbove {
		t.Error("Tree(3, 2) CutAbove = true, want false: the chain ended at the archived row it shows")
	}
}

func TestHierarchyTreeUnboundedLevelBudgetWalksTheWholeChain(t *testing.T) {
	// hierarchy_levels has no maximum, and a budget far past the board still
	// walks the chain and nothing else.
	const length = 8
	ix := board.NewHierarchyIndex(chainFixture(length))
	cfg := config.NewDefault("test")

	tree := ix.Tree(length, cfg, 1<<20)

	if len(tree.Rows) != length || tree.CutAbove || tree.CutBelow {
		t.Errorf("Tree(%d, 1<<20) = %v, above %t, below %t, want the whole chain uncut",
			length, treeSummary(tree), tree.CutAbove, tree.CutBelow)
	}
}

func TestHierarchyTreeDanglingAndSelfParentProduceNoRow(t *testing.T) {
	tasks := []*task.Task{
		{ID: 1, Title: "Dangling", Status: statusBacklog, Parent: ptr(999)},
		{ID: 2, Title: "Self", Status: statusBacklog, Parent: ptr(2)},
	}
	ix := board.NewHierarchyIndex(tasks)
	cfg := config.NewDefault("test")

	for _, id := range []int{1, 2} {
		tree := ix.Tree(id, cfg, 3)
		if len(tree.Rows) != 1 || tree.Rows[0].ID != id || !tree.Rows[0].Current {
			t.Errorf("Tree(%d) rows = %v, want only the open ticket", id, treeSummary(tree))
		}
		if tree.CutAbove || tree.CutBelow {
			t.Errorf("Tree(%d) cut flags = above %t, below %t, want both false",
				id, tree.CutAbove, tree.CutBelow)
		}
	}
}

func TestHierarchyTreeUnknownTaskIsAnEmptyTree(t *testing.T) {
	ix := board.NewHierarchyIndex(hierarchyFixture())

	tree := ix.Tree(999, config.NewDefault("test"), 2)

	if len(tree.Rows) != 0 || tree.CutAbove || tree.CutBelow {
		t.Errorf("Tree(999) = %v, above %t, below %t, want an empty tree",
			treeSummary(tree), tree.CutAbove, tree.CutBelow)
	}
}

func TestHierarchyTreeDescentCycleTerminates(t *testing.T) {
	// A cycle must neither hang nor render a task twice.
	tasks := []*task.Task{
		{ID: 1, Title: "A", Status: statusBacklog, Parent: ptr(2)},
		{ID: 2, Title: "B", Status: statusBacklog, Parent: ptr(1)},
	}
	ix := board.NewHierarchyIndex(tasks)
	cfg := config.NewDefault("test")

	tree := ix.Tree(1, cfg, 6)

	want := []string{
		"d0/#2/backlog/B/last=true/current=false/0-1",
		"d1/#1/backlog/A/last=true/current=true/0-1",
	}
	if got := treeSummary(tree); !reflect.DeepEqual(got, want) {
		t.Errorf("rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if tree.CutAbove {
		t.Error("CutAbove = true, want false: a cycle ends the chain without an ellipsis")
	}
	// The counter still reports the direct child, because that is what it
	// means, even though the cycle keeps the row out of the tree.
	if !tree.CutBelow {
		t.Error("CutBelow = false, want true: #1 has a direct child that is not rendered")
	}
}

func TestHierarchyTreeCutFlags(t *testing.T) {
	tasks := hierarchyFixture()
	ix := board.NewHierarchyIndex(tasks)
	cfg := config.NewDefault("test")

	if tree := ix.Tree(4, cfg, 1); !tree.CutAbove || tree.CutBelow {
		t.Errorf("Tree(4, 1) = above %t, below %t, want above only", tree.CutAbove, tree.CutBelow)
	}
	if tree := ix.Tree(1, cfg, 1); tree.CutAbove || !tree.CutBelow {
		t.Errorf("Tree(1, 1) = above %t, below %t, want below only", tree.CutAbove, tree.CutBelow)
	}
	// #2 sits one level under the root, so one level of ancestors reaches the
	// top and nothing is cut off above it.
	if tree := ix.Tree(2, cfg, 1); tree.CutAbove || tree.CutBelow {
		t.Errorf("Tree(2, 1) = above %t, below %t, want neither", tree.CutAbove, tree.CutBelow)
	}
	if tree := ix.Tree(1, cfg, 2); tree.CutAbove || tree.CutBelow {
		t.Errorf("Tree(1, 2) = above %t, below %t, want neither", tree.CutAbove, tree.CutBelow)
	}
}

func TestHierarchyTreeArchivedChildrenAreNotDescendants(t *testing.T) {
	tasks := []*task.Task{
		{ID: 1, Title: "Root", Status: statusBacklog},
		{ID: 2, Title: "Archived", Status: config.ArchivedStatus, Parent: ptr(1)},
	}
	ix := board.NewHierarchyIndex(tasks)
	cfg := config.NewDefault("test")

	tree := ix.Tree(1, cfg, 2)

	if len(tree.Rows) != 1 || tree.Rows[0].ID != 1 {
		t.Errorf("rows = %v, want only the open ticket", treeSummary(tree))
	}
	if tree.Rows[0].Total != 0 {
		t.Errorf("Total = %d, want 0: archived children do not count", tree.Rows[0].Total)
	}
	if tree.CutBelow {
		t.Error("CutBelow = true, want false: an archived child is not something cut off")
	}
}

func TestHierarchyIndexDepthsMatchDepthsFunction(t *testing.T) {
	// The expectations are spelled out instead of taken from board.Depths: the
	// wrapper is the method, so holding one against the other could not fail.
	const chainLength = 20
	chainDepths := make(map[int]int, chainLength)
	for id := 1; id <= chainLength; id++ {
		chainDepths[id] = id - 1
	}
	fixtures := []struct {
		name  string
		tasks []*task.Task
		want  map[int]int
	}{
		{
			name:  "three levels",
			tasks: hierarchyFixture(),
			want:  map[int]int{1: 0, 2: 1, 3: 1, 4: 2, 5: 2, 6: 2},
		},
		{
			name:  "long chain",
			tasks: chainFixture(chainLength),
			want:  chainDepths,
		},
		{
			name: "broken links",
			tasks: []*task.Task{
				{ID: 1, Parent: ptr(999)},
				{ID: 2, Parent: ptr(2)},
				{ID: 3, Parent: ptr(1)},
			},
			want: map[int]int{1: 0, 2: 0, 3: 1},
		},
	}
	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			got := board.NewHierarchyIndex(tc.tasks).Depths()
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("HierarchyIndex.Depths() = %v, want %v", got, tc.want)
			}
			if wrapper := board.Depths(tc.tasks); !reflect.DeepEqual(wrapper, tc.want) {
				t.Errorf("Depths() = %v, want %v", wrapper, tc.want)
			}
		})
	}
}

func TestHierarchyIndexTaskLookup(t *testing.T) {
	ix := board.NewHierarchyIndex(hierarchyFixture())

	if got := ix.Task(4); got == nil || got.Title != "Leaf Four" {
		t.Errorf("Task(4) = %v, want Leaf Four", got)
	}
	if got := ix.Task(999); got != nil {
		t.Errorf("Task(999) = %v, want nil", got)
	}
}

// benchBoard generates a board in the shape of a real one: 219 tasks, 24 roots,
// three levels, a fan-out of 33 under the widest root and every 13th task
// archived.
func benchBoard() []*task.Task {
	const (
		total     = 219
		roots     = 24
		wideFanIn = 33
	)
	tasks := make([]*task.Task, 0, total)
	status := func(id int) string {
		switch {
		case id%13 == 0:
			return config.ArchivedStatus
		case id%3 == 0:
			return statusDone
		default:
			return statusBacklog
		}
	}
	for id := 1; id <= roots; id++ {
		tasks = append(tasks, &task.Task{ID: id, Title: "Bench Root", Status: status(id)})
	}
	level1 := make([]int, 0, 66)
	for id := roots + 1; id <= roots+66; id++ {
		parent := 1
		if len(level1) >= wideFanIn {
			parent = 2 + (len(level1)-wideFanIn)%(roots-1)
		}
		tasks = append(tasks, &task.Task{ID: id, Title: "Mid", Status: status(id), Parent: ptr(parent)})
		level1 = append(level1, id)
	}
	for i, id := 0, roots+67; id <= total; i, id = i+1, id+1 {
		parent := level1[i%len(level1)]
		tasks = append(tasks, &task.Task{ID: id, Title: "Leaf", Status: status(id), Parent: ptr(parent)})
	}
	return tasks
}

// naiveTreeRows walks the same tree without an index, resolving children with
// SummarizeChildren per node — the cost the load-path index removes.
func naiveTreeRows(tasks []*task.Task, cfg *config.Config, rootID, levels int) int {
	rows := 0
	var walk func(id, rel int)
	walk = func(id, rel int) {
		if rel >= levels {
			return
		}
		for _, kid := range board.SummarizeChildren(tasks, id, cfg, false).Children {
			rows++
			walk(kid.ID, rel+1)
		}
	}
	walk(rootID, 0)
	return rows
}

func BenchmarkHierarchyTree(b *testing.B) {
	tasks := benchBoard()
	cfg := config.NewDefault("bench")
	ix := board.NewHierarchyIndex(tasks)
	const widestRoot = 1

	b.Run("index", func(b *testing.B) {
		for range b.N {
			if got := len(ix.Tree(widestRoot, cfg, 6).Rows); got == 0 {
				b.Fatal("empty tree")
			}
		}
	})
	b.Run("naive", func(b *testing.B) {
		for range b.N {
			if got := naiveTreeRows(tasks, cfg, widestRoot, 6); got == 0 {
				b.Fatal("empty tree")
			}
		}
	})
}

func BenchmarkNewHierarchyIndex(b *testing.B) {
	tasks := benchBoard()
	for range b.N {
		if board.NewHierarchyIndex(tasks).Task(1) == nil {
			b.Fatal("index without task #1")
		}
	}
}
