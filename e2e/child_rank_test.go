package e2e_test

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Child rank tests
// ---------------------------------------------------------------------------

type childRankChildJSON struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	ChildRank *int   `json:"child_rank,omitempty"`
}

type childRankDetailJSON struct {
	ID       int                  `json:"id"`
	Children []childRankChildJSON `json:"children"`
}

// setupRankedFamily creates a parent whose child ranks deliberately contradict
// the technical task IDs, which is the case the feature exists for.
func setupRankedFamily(t *testing.T) string {
	t.Helper()

	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Parent")
	mustCreateTask(t, kanbanDir, "Third", "--parent", "1", "--child-rank", "30")
	mustCreateTask(t, kanbanDir, "First", "--parent", "1", "--child-rank", "10")
	mustCreateTask(t, kanbanDir, "Second", "--parent", "1", "--child-rank", "20")
	return kanbanDir
}

func TestCreateWithChildRankWritesTheField(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Parent")
	child := mustCreateTask(t, kanbanDir, "Child", "--parent", "1", "--child-rank", "20")

	if child.ChildRank == nil {
		t.Fatal("child_rank is nil, want 20")
	}
	if *child.ChildRank != 20 {
		t.Errorf("child_rank = %d, want 20", *child.ChildRank)
	}
}

func TestCreateWithoutChildRankOmitsTheField(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Parent")
	child := mustCreateTask(t, kanbanDir, "Child", "--parent", "1")

	if child.ChildRank != nil {
		t.Errorf("child_rank = %v, want nil", child.ChildRank)
	}
}

func TestCreateRejectsChildRankWithoutParent(t *testing.T) {
	kanbanDir := initBoard(t)

	errResp := runKanbanJSONError(t, kanbanDir, "create", "Orphan rank", "--child-rank", "10")
	if errResp.Code != codeInvalidInput {
		t.Errorf("code = %q, want %s", errResp.Code, codeInvalidInput)
	}
}

func TestCreateRejectsNonPositiveChildRank(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Parent")

	for _, rank := range []string{"0", "-1"} {
		errResp := runKanbanJSONError(t, kanbanDir,
			"create", "Bad rank", "--parent", "1", "--child-rank", rank)
		if errResp.Code != codeInvalidInput {
			t.Errorf("rank %s: code = %q, want %s", rank, errResp.Code, codeInvalidInput)
		}
	}
}

func TestEditSetsAndClearsChildRank(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Parent")
	mustCreateTask(t, kanbanDir, "Child", "--parent", "1")

	var edited taskJSON
	if r := runKanbanJSON(t, kanbanDir, &edited, "edit", "2", "--child-rank", "40"); r.exitCode != 0 {
		t.Fatalf("edit --child-rank failed: %s", r.stderr)
	}
	if edited.ChildRank == nil || *edited.ChildRank != 40 {
		t.Fatalf("child_rank = %v, want 40", edited.ChildRank)
	}

	var cleared taskJSON
	if r := runKanbanJSON(t, kanbanDir, &cleared, "edit", "2", "--clear-child-rank"); r.exitCode != 0 {
		t.Fatalf("edit --clear-child-rank failed: %s", r.stderr)
	}
	if cleared.ChildRank != nil {
		t.Errorf("child_rank = %v after clear, want nil", cleared.ChildRank)
	}
}

func TestEditClearParentAlsoClearsChildRank(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Parent")
	mustCreateTask(t, kanbanDir, "Child", "--parent", "1", "--child-rank", "10")

	var cleared taskJSON
	if r := runKanbanJSON(t, kanbanDir, &cleared, "edit", "2", "--clear-parent"); r.exitCode != 0 {
		t.Fatalf("edit --clear-parent failed: %s", r.stderr)
	}
	if cleared.Parent != nil {
		t.Errorf("parent = %v, want nil", cleared.Parent)
	}
	if cleared.ChildRank != nil {
		t.Errorf("child_rank = %v, want nil after --clear-parent", cleared.ChildRank)
	}
}

func TestEditKeepsChildRankOnReparent(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Parent A")
	mustCreateTask(t, kanbanDir, "Parent B")
	mustCreateTask(t, kanbanDir, "Child", "--parent", "1", "--child-rank", "10")

	var moved taskJSON
	if r := runKanbanJSON(t, kanbanDir, &moved, "edit", "3", "--parent", "2"); r.exitCode != 0 {
		t.Fatalf("edit --parent failed: %s", r.stderr)
	}
	if moved.Parent == nil || *moved.Parent != 2 {
		t.Fatalf("parent = %v, want 2", moved.Parent)
	}
	if moved.ChildRank == nil || *moved.ChildRank != 10 {
		t.Errorf("child_rank = %v, want 10 to survive reparenting", moved.ChildRank)
	}
}

func TestEditRejectsChildRankWithoutParent(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Standalone")

	errResp := runKanbanJSONError(t, kanbanDir, "edit", "1", "--child-rank", "10")
	if errResp.Code != codeInvalidInput {
		t.Errorf("code = %q, want %s", errResp.Code, codeInvalidInput)
	}
}

func TestEditRejectsConflictingChildRankFlags(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Parent")
	mustCreateTask(t, kanbanDir, "Child", "--parent", "1")

	cases := [][]string{
		{"edit", "2", "--child-rank", "10", "--clear-child-rank"},
		{"edit", "2", "--child-rank", "10", "--clear-parent"},
	}
	for _, args := range cases {
		errResp := runKanbanJSONError(t, kanbanDir, args...)
		if errResp.Code != codeStatusConflict {
			t.Errorf("%v: code = %q, want %s", args, errResp.Code, codeStatusConflict)
		}
	}
}

func TestShowOrdersChildrenByChildRankInJSON(t *testing.T) {
	kanbanDir := setupRankedFamily(t)

	var detail childRankDetailJSON
	if r := runKanbanJSON(t, kanbanDir, &detail, "show", "1"); r.exitCode != 0 {
		t.Fatalf("show failed: %s", r.stderr)
	}

	want := []int{3, 4, 2}
	if len(detail.Children) != len(want) {
		t.Fatalf("children len = %d, want %d", len(detail.Children), len(want))
	}
	for i, id := range want {
		if detail.Children[i].ID != id {
			t.Fatalf("children IDs = %v, want %v", childRankIDs(detail.Children), want)
		}
	}
	if detail.Children[0].ChildRank == nil || *detail.Children[0].ChildRank != 10 {
		t.Errorf("first child rank = %v, want 10", detail.Children[0].ChildRank)
	}
}

func TestShowOrdersChildrenByChildRankInTable(t *testing.T) {
	kanbanDir := setupRankedFamily(t)

	r := runKanban(t, kanbanDir, "--table", "show", "1")
	if r.exitCode != 0 {
		t.Fatalf("show failed: %s", r.stderr)
	}
	assertOrder(t, r.stdout, "#3", "#4", "#2")
	if !strings.Contains(r.stdout, "rank 10") {
		t.Errorf("show output missing 'rank 10', got: %s", r.stdout)
	}
}

func TestShowDisplaysOwnChildRank(t *testing.T) {
	kanbanDir := setupRankedFamily(t)

	r := runKanban(t, kanbanDir, "--table", "show", "3")
	if r.exitCode != 0 {
		t.Fatalf("show failed: %s", r.stderr)
	}
	if !strings.Contains(r.stdout, "rank 10") {
		t.Errorf("show output missing own 'rank 10', got: %s", r.stdout)
	}

	c := runKanban(t, kanbanDir, "--compact", "show", "3")
	if c.exitCode != 0 {
		t.Fatalf("show --compact failed: %s", c.stderr)
	}
	if !strings.Contains(c.stdout, "child-rank:10") {
		t.Errorf("compact output missing 'child-rank:10', got: %s", c.stdout)
	}
}

func TestShowPutsUnrankedChildrenLast(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Parent")
	mustCreateTask(t, kanbanDir, "Unranked low ID", "--parent", "1")
	mustCreateTask(t, kanbanDir, "Ranked high ID", "--parent", "1", "--child-rank", "50")

	r := runKanban(t, kanbanDir, "--table", "show", "1")
	if r.exitCode != 0 {
		t.Fatalf("show failed: %s", r.stderr)
	}
	assertOrder(t, r.stdout, "#3", "#2")
}

func childRankIDs(children []childRankChildJSON) []int {
	ids := make([]int, 0, len(children))
	for _, child := range children {
		ids = append(ids, child.ID)
	}
	return ids
}

// assertOrder checks that the needles appear in the given order in out.
func assertOrder(t *testing.T, out string, needles ...string) {
	t.Helper()

	pos := -1
	for _, needle := range needles {
		at := strings.Index(out, needle)
		if at < 0 {
			t.Fatalf("output missing %q, got: %s", needle, out)
		}
		if at <= pos {
			t.Fatalf("%q appears out of order (want %v), got: %s", needle, needles, out)
		}
		pos = at
	}
}
