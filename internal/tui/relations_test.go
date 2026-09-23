package tui_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thegapcloser/kanban-md/internal/config"
	"github.com/thegapcloser/kanban-md/internal/task"
	"github.com/thegapcloser/kanban-md/internal/tui"
)

// titleGrandchild is the indirect descendant that must never show up as a
// direct child of the epic.
const titleGrandchild = "Grandchild"

// relationCursorMarker is the cursor gutter as it appears in a rendered row.
const relationCursorMarker = "> "

// newFileBoard writes a task set to a fresh board directory and opens a TUI on
// it at 120x40, the size every relation and snapshot assertion assumes.
func newFileBoard(t *testing.T, name string, tasks []*task.Task) (*tui.Board, *config.Config) {
	t.Helper()

	dir := t.TempDir()
	kanbanDir := filepath.Join(dir, "kanban")
	tasksDir := filepath.Join(kanbanDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0o750); err != nil {
		t.Fatalf("creating dirs: %v", err)
	}

	cfg := config.NewDefault(name)
	cfg.SetDir(kanbanDir)
	if err := cfg.Save(); err != nil {
		t.Fatalf("saving config: %v", err)
	}
	for _, tk := range tasks {
		path := filepath.Join(tasksDir, task.GenerateFilename(tk.ID, tk.Title))
		if err := task.Write(path, tk); err != nil {
			t.Fatalf("writing task %d: %v", tk.ID, err)
		}
	}

	b := tui.NewBoard(cfg)
	b.SetNow(testNow)
	b.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return b, cfg
}

// setupRelationNavBoard builds a board whose tree covers every navigability
// case of the detail view: resolvable parent and children (#1..#3, #7), an
// archived child of #1 (#4), a dangling parent reference (#5), and a child of
// that archived parent (#6).
func setupRelationNavBoard(t *testing.T) (*tui.Board, *config.Config) {
	t.Helper()

	epicID := 1
	childID := 2
	archivedID := 4
	danglingID := 999
	tasks := []*task.Task{
		{ID: 1, Title: "Epic Alpha", Status: config.DefaultStatus, Priority: priorityCritical, Updated: testRefTime},
		{ID: 2, Title: "Child One", Status: statusTodo, Priority: "high", Parent: &epicID, Updated: testRefTime},
		{ID: 3, Title: "Child Two", Status: statusDone, Priority: "medium", Parent: &epicID, Updated: testRefTime},
		{ID: 4, Title: "Archived Thing", Status: "archived", Priority: "medium", Parent: &epicID, Updated: testRefTime},
		{ID: 5, Title: "Orphan Child", Status: statusTodo, Priority: "low", Parent: &danglingID, Updated: testRefTime},
		{ID: 6, Title: "Archived Parent Child", Status: statusTodo, Priority: "low", Parent: &archivedID, Updated: testRefTime},
		{ID: 7, Title: titleGrandchild, Status: statusTodo, Priority: "medium", Parent: &childID, Updated: testRefTime},
	}
	return newFileBoard(t, "Relation Nav Board", tasks)
}

// setupHierarchyTreeBoard writes a three-level board — one milestone, two epics,
// three stories — and opens the detail view of one task with a fixed level
// budget.
func setupHierarchyTreeBoard(t *testing.T, detailTitle string, levels int) *tui.Board {
	t.Helper()

	milestoneID := 1
	firstEpicID := 2
	secondEpicID := 3
	tasks := []*task.Task{
		{ID: 1, Title: "Milestone One", Status: statusTodo, Priority: "high", Updated: testRefTime},
		{
			ID: 2, Title: "Epic Two", Status: statusDone, Priority: "medium",
			Parent: &milestoneID, Updated: testRefTime,
		},
		{
			ID: 3, Title: "Epic Three", Status: config.DefaultStatus, Priority: "medium",
			Parent: &milestoneID, Updated: testRefTime,
		},
		{
			ID: 4, Title: "Story Four", Status: statusDone, Priority: "low",
			Parent: &firstEpicID, Updated: testRefTime,
		},
		{
			ID: 5, Title: "Story Five", Status: statusDone, Priority: "low",
			Parent: &firstEpicID, Updated: testRefTime,
		},
		{
			ID: 6, Title: "Story Six", Status: config.DefaultStatus, Priority: "low",
			Parent: &secondEpicID, Updated: testRefTime,
		},
	}
	b, cfg := newFileBoard(t, "Hierarchy Tree Board", tasks)
	cfg.TUI.HierarchyLevels = &levels
	return openTaskDetail(t, b, detailTitle)
}

// openTaskDetail searches for a task by title and opens its detail view. The
// search leaves the cursor in a column that may now be empty, so the helper
// walks right until Enter actually opens a detail view.
func openTaskDetail(t *testing.T, b *tui.Board, title string) *tui.Board {
	t.Helper()
	b = sendKey(b, "/")
	for _, r := range title {
		b = sendKey(b, string(r))
	}
	b = sendSpecialKey(b, tea.KeyEnter)
	for range 6 {
		b = sendSpecialKey(b, tea.KeyEnter)
		if containsStr(b.View(), "Task #") {
			return b
		}
		b = sendKey(b, "l")
	}
	t.Fatalf("could not open the detail view of %q:\n%s", title, b.View())
	return b
}

func TestBoard_DetailRelationLinesUnchangedWithoutCursor(t *testing.T) {
	b, _ := setupRelationNavBoard(t)
	b = sendSpecialKey(b, tea.KeyEnter)
	v := b.View()

	for _, want := range []string{
		"Task #1: Epic Alpha",
		// #2 and #3 are the counted children of #1 and both stand below it, so
		// #1 carries no rollup; the hidden child of #2 keeps its counter.
		"└─ #1 [backlog] Epic Alpha",
		"├─ #2 [todo] Child One (0/1 done)",
		"└─ #3 [done] Child Two",
	} {
		if !containsStr(v, want) {
			t.Errorf("detail view of #1 missing %q:\n%s", want, v)
		}
	}
	// #4 is an archived child of #1 and #7 an indirect descendant: one level of
	// tree must hold neither, and both are present in the fixture so the
	// assertion can actually fail.
	for _, unwanted := range []string{"Archived Thing", titleGrandchild, "Epic Alpha (1/2 done)"} {
		if containsStr(v, unwanted) {
			t.Errorf("detail view of #1 should not list %q:\n%s", unwanted, v)
		}
	}
}

// cursorRow returns the rendered row carrying the cursor gutter, ANSI stripped,
// or the empty string when no row has it. Asserting on the row instead of on a
// literal "> " + prefix keeps the assertion about the cursor and not about how
// deep the row happens to be indented.
func cursorRow(v string) string {
	for _, line := range strings.Split(stripANSI(v), "\n") {
		if strings.HasPrefix(line, relationCursorMarker) {
			return line
		}
	}
	return ""
}

// lastRenderedLine returns the bottom line of a view with ANSI codes stripped.
func lastRenderedLine(v string) string {
	lines := strings.Split(stripANSI(v), "\n")
	return lines[len(lines)-1]
}

func TestBoard_DetailCursorInactiveOnOpen(t *testing.T) {
	b, _ := setupRelationNavBoard(t)
	b = sendSpecialKey(b, tea.KeyEnter)
	v := b.View()

	if !containsStr(v, "  ├─ #2 [todo] Child One") {
		t.Errorf("tree row is missing the two-cell gutter:\n%s", v)
	}
	if row := cursorRow(v); row != "" {
		t.Errorf("detail view marks row %q before the cursor was activated:\n%s", row, v)
	}
}

func TestBoard_DetailHintOmitsRelationKeysWithoutRelations(t *testing.T) {
	b, _ := setupTestBoard(t)
	b = sendSpecialKey(b, tea.KeyEnter)

	if got := lastRenderedLine(b.View()); got != "q/esc:back" {
		t.Errorf("hint on a relationless task = %q, want %q", got, "q/esc:back")
	}
}

func TestBoard_DetailHintShowsRelationKeys(t *testing.T) {
	b, _ := setupRelationNavBoard(t)
	b = sendSpecialKey(b, tea.KeyEnter)

	if !containsStr(b.View(), "tab:relations  enter:open") {
		t.Errorf("hint does not advertise the relation keys:\n%s", b.View())
	}
}

func TestBoard_DetailHintFitsNarrowWidth(t *testing.T) {
	b, _ := setupRelationNavBoard(t)
	b.Update(tea.WindowSizeMsg{Width: 24, Height: 12})
	b = sendSpecialKey(b, tea.KeyEnter)

	if got := lastRenderedLine(b.View()); lipgloss.Width(got) > 24 {
		t.Errorf("hint %q is %d cells wide, want at most 24", got, lipgloss.Width(got))
	}
}

func TestBoard_DetailTabActivatesAndWrapsCursor(t *testing.T) {
	b, _ := setupRelationNavBoard(t)
	b = sendSpecialKey(b, tea.KeyEnter)

	for i, want := range []string{
		"├─ #2 [todo] Child One",
		"└─ #3 [done] Child Two",
		"├─ #2 [todo] Child One",
	} {
		b = sendSpecialKey(b, tea.KeyTab)
		if got := cursorRow(b.View()); !strings.Contains(got, want) {
			t.Fatalf("tab press %d: cursor row is %q, want it on %q:\n%s", i+1, got, want, b.View())
		}
	}

	b = sendSpecialKey(b, tea.KeyShiftTab)
	if got := cursorRow(b.View()); !strings.Contains(got, "└─ #3 [done] Child Two") {
		t.Errorf("shift+tab did not wrap backwards, cursor row is %q:\n%s", got, b.View())
	}
}

func TestBoard_DetailCursorVisitsAncestorFirst(t *testing.T) {
	// Reading order: the ancestor stands above the open task, so tab reaches it
	// before the descendant below.
	b, _ := setupRelationNavBoard(t)
	b = openTaskDetail(t, b, "Child One")

	b = sendSpecialKey(b, tea.KeyTab)
	if got := cursorRow(b.View()); !strings.Contains(got, "└─ #1 [backlog] Epic Alpha") {
		t.Fatalf("first tab selected %q, want the ancestor row:\n%s", got, b.View())
	}
	b = sendSpecialKey(b, tea.KeyTab)
	if got := cursorRow(b.View()); !strings.Contains(got, "└─ #7 [todo] "+titleGrandchild) {
		t.Errorf("second tab selected %q, want the descendant row:\n%s", got, b.View())
	}
}

func TestBoard_DetailNonNavigableRelationIsNoCursorStop(t *testing.T) {
	// A broken parent reference is no longer a row of its own: the chain simply
	// ends, and a one-row tree is not shown at all. An archived ancestor is
	// shown, ends the chain and stays out of reach of the cursor.
	for _, tt := range []struct {
		name     string
		title    string
		wantLine string
		wantTree bool
	}{
		{name: "dangling parent", title: "Orphan Child"},
		{
			name:     "archived parent",
			title:    "Archived Parent Child",
			wantLine: "  └─ #4 [archived] Archived Thing",
			wantTree: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := setupRelationNavBoard(t)
			b = openTaskDetail(t, b, tt.title)
			b = sendSpecialKey(b, tea.KeyTab)
			v := b.View()

			if tt.wantTree != containsStr(v, "Hierarchy") {
				t.Errorf("hierarchy block present = %t, want %t:\n%s", !tt.wantTree, tt.wantTree, v)
			}
			if tt.wantLine != "" && !containsStr(v, tt.wantLine) {
				t.Errorf("tree row is not rendered as %q:\n%s", tt.wantLine, v)
			}
			if row := cursorRow(v); row != "" {
				t.Errorf("tab put the cursor on %q, a non-navigable row:\n%s", row, v)
			}
		})
	}
}

func TestBoard_DetailTabScrollsCursorIntoView(t *testing.T) {
	b, cfg := setupRelationNavBoard(t)
	addLongBodyToTask(t, cfg, 1, 60)
	b = sendKey(b, "r")
	b = sendSpecialKey(b, tea.KeyEnter)

	for range 40 {
		b = sendKey(b, "j")
	}
	if containsStr(b.View(), "├─ #2") {
		t.Fatalf("tree rows are still visible after scrolling to the bottom:\n%s", b.View())
	}

	b = sendSpecialKey(b, tea.KeyTab)
	if got := cursorRow(b.View()); !strings.Contains(got, "├─ #2") {
		t.Errorf("tab did not scroll the cursor row back into view, cursor row is %q:\n%s",
			got, b.View())
	}
}

func TestBoard_DetailEnterOpensCursorRelation(t *testing.T) {
	b, _ := setupRelationNavBoard(t)
	b = sendSpecialKey(b, tea.KeyEnter)
	b = sendSpecialKey(b, tea.KeyTab)
	b = sendSpecialKey(b, tea.KeyEnter)

	if !containsStr(b.View(), "Task #2: Child One") {
		t.Errorf("enter did not open the relation under the cursor:\n%s", b.View())
	}
}

func TestBoard_DetailEnterWithoutCursorIsNoop(t *testing.T) {
	b, _ := setupRelationNavBoard(t)
	b = sendSpecialKey(b, tea.KeyEnter)
	b = sendSpecialKey(b, tea.KeyEnter)

	if !containsStr(b.View(), "Task #1: Epic Alpha") {
		t.Errorf("enter without an active cursor left the detail view:\n%s", b.View())
	}
}

func TestBoard_DetailOpenRelationKeepsBoardFilterAndSelection(t *testing.T) {
	b, _ := setupRelationNavBoard(t)
	b = sendKey(b, "v")
	if !containsStr(b.View(), "level[0]") {
		t.Fatalf("level filter is not active:\n%s", b.View())
	}

	b = sendSpecialKey(b, tea.KeyEnter)
	b = sendSpecialKey(b, tea.KeyTab)
	b = sendSpecialKey(b, tea.KeyEnter)
	if !containsStr(b.View(), "Task #2: Child One") {
		t.Fatalf("a child filtered off the board did not open:\n%s", b.View())
	}

	b = sendKey(b, "q")
	if !containsStr(b.View(), "level[0]") {
		t.Errorf("opening a relation changed the board level filter:\n%s", b.View())
	}
	if !containsStr(b.View(), "#1 Epic Alpha") {
		t.Errorf("opening a relation changed the board selection:\n%s", b.View())
	}
}

func TestBoard_DetailOpenRelationFollowsCycle(t *testing.T) {
	b, _ := setupRelationNavBoard(t)
	b = sendSpecialKey(b, tea.KeyEnter)
	b = sendSpecialKey(b, tea.KeyTab)
	b = sendSpecialKey(b, tea.KeyEnter) // #1 -> #2
	b = sendSpecialKey(b, tea.KeyTab)
	b = sendSpecialKey(b, tea.KeyEnter) // #2 -> parent #1

	v := b.View()
	if !containsStr(v, "Task #1: Epic Alpha") {
		t.Fatalf("following the cycle back to the parent failed:\n%s", v)
	}
	if !containsStr(v, "esc:back(2)") {
		t.Errorf("navigation stack does not hold two frames:\n%s", v)
	}
}

// sendReload makes the board re-read its tasks from disk.
func sendReload(b *tui.Board) *tui.Board {
	m, _ := b.Update(tui.ReloadMsg{})
	return m.(*tui.Board)
}

// deleteTaskFile removes a task file from disk so a reload loses the task.
func deleteTaskFile(t *testing.T, cfg *config.Config, taskID int) {
	t.Helper()
	path, err := task.FindByID(cfg.TasksPath(), taskID)
	if err != nil {
		t.Fatalf("finding task %d: %v", taskID, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("removing task %d: %v", taskID, err)
	}
}

func TestBoard_DetailEscPopsOneLevelAndRestoresView(t *testing.T) {
	for _, tt := range []struct {
		name string
		back tea.KeyType
	}{
		{name: "esc", back: tea.KeyEsc},
		{name: "backspace", back: tea.KeyBackspace},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b, cfg := setupRelationNavBoard(t)
			addLongBodyToTask(t, cfg, 1, 60)
			b = sendKey(b, "r")
			b = sendSpecialKey(b, tea.KeyEnter)
			for range 10 {
				b = sendKey(b, "j")
			}
			b = sendSpecialKey(b, tea.KeyTab)
			want := b.View()

			b = sendSpecialKey(b, tea.KeyEnter)
			if !containsStr(b.View(), "Task #2: Child One") {
				t.Fatalf("relation did not open:\n%s", b.View())
			}
			b = sendSpecialKey(b, tt.back)

			if got := b.View(); got != want {
				t.Errorf("going back did not restore the abandoned view:\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestBoard_DetailEscClosesWithEmptyStack(t *testing.T) {
	b, _ := setupTestBoard(t)
	b = sendSpecialKey(b, tea.KeyEnter)
	b = sendSpecialKey(b, tea.KeyEsc)

	if !containsStr(b.View(), "? help") {
		t.Errorf("esc on an empty stack did not close the detail view:\n%s", b.View())
	}
}

func TestBoard_DetailQClosesWholeStack(t *testing.T) {
	b, _ := setupRelationNavBoard(t)
	b = sendSpecialKey(b, tea.KeyEnter)
	b = sendSpecialKey(b, tea.KeyTab)
	b = sendSpecialKey(b, tea.KeyEnter) // #1 -> #2
	b = sendSpecialKey(b, tea.KeyTab)
	b = sendSpecialKey(b, tea.KeyTab)
	b = sendSpecialKey(b, tea.KeyEnter) // #2 -> #7
	if !containsStr(b.View(), "Task #7: Grandchild") {
		t.Fatalf("second hop did not open the grandchild:\n%s", b.View())
	}

	b = sendKey(b, "q")
	if !containsStr(b.View(), "? help") {
		t.Fatalf("q did not close the detail view:\n%s", b.View())
	}

	// Reopening from the board must start a fresh history. Pressing esc in the
	// board view proves nothing — there it is Quit and leaves the view alone.
	b = sendSpecialKey(b, tea.KeyEnter)
	if !containsStr(b.View(), "Task #1: Epic Alpha") {
		t.Fatalf("reopening the detail view failed:\n%s", b.View())
	}
	if containsStr(b.View(), "esc:back(") {
		t.Fatalf("the reopened detail view still advertises a relation history:\n%s", b.View())
	}

	b = sendSpecialKey(b, tea.KeyEsc)
	v := b.View()
	if !containsStr(v, "? help") {
		t.Errorf("esc walked back into the history q was supposed to forget:\n%s", v)
	}
	if containsStr(v, "Task #") {
		t.Errorf("esc reopened a task from the history q was supposed to forget:\n%s", v)
	}
}

func TestBoard_DetailHintCountsOnlyReachableFrames(t *testing.T) {
	b, cfg := setupRelationNavBoard(t)
	b = sendSpecialKey(b, tea.KeyEnter)
	b = sendSpecialKey(b, tea.KeyTab)
	b = sendSpecialKey(b, tea.KeyEnter) // #1 -> #2
	b = sendSpecialKey(b, tea.KeyTab)
	b = sendSpecialKey(b, tea.KeyTab)
	b = sendSpecialKey(b, tea.KeyEnter) // #2 -> #7
	if !containsStr(b.View(), "esc:back(2)") {
		t.Fatalf("two hops did not produce two frames:\n%s", b.View())
	}

	// #2 vanishes from the middle of the chain. esc skips its frame, so the
	// hint must promise one step, not two.
	deleteTaskFile(t, cfg, 2)
	b = sendReload(b)

	v := b.View()
	if !containsStr(v, "Task #7: "+titleGrandchild) {
		t.Fatalf("the reload left the detail view of #7:\n%s", v)
	}
	if !containsStr(v, "esc:back(1)") {
		t.Errorf("hint does not count only the frames esc can reach:\n%s", v)
	}

	b = sendSpecialKey(b, tea.KeyEsc)
	if !containsStr(b.View(), "Task #1: Epic Alpha") {
		t.Errorf("one esc did not land on the single reachable frame:\n%s", b.View())
	}
}

func TestBoard_DetailPopsWhenCurrentTaskVanishes(t *testing.T) {
	b, cfg := setupRelationNavBoard(t)
	b = sendSpecialKey(b, tea.KeyEnter)
	b = sendSpecialKey(b, tea.KeyTab)
	b = sendSpecialKey(b, tea.KeyEnter)

	deleteTaskFile(t, cfg, 2)
	b = sendReload(b)

	if !containsStr(b.View(), "Task #1: Epic Alpha") {
		t.Errorf("a vanished task closed the detail view instead of going back:\n%s", b.View())
	}
}

func TestBoard_DetailStaleCursorIsClamped(t *testing.T) {
	b, cfg := setupRelationNavBoard(t)
	b = sendSpecialKey(b, tea.KeyEnter)
	b = sendSpecialKey(b, tea.KeyTab)
	b = sendSpecialKey(b, tea.KeyTab)

	deleteTaskFile(t, cfg, 3)
	b = sendReload(b)

	v := b.View()
	if !containsStr(v, "Task #1: Epic Alpha") {
		t.Fatalf("reload left the detail view of #1:\n%s", v)
	}
	if got := strings.Count(stripANSI(v), "> "); got > 1 {
		t.Errorf("view holds %d cursor markers after the cursor row vanished:\n%s", got, v)
	}
}

// setTaskBody rewrites the body of a task on disk.
func setTaskBody(t *testing.T, cfg *config.Config, taskID int, body string) {
	t.Helper()
	path, err := task.FindByID(cfg.TasksPath(), taskID)
	if err != nil {
		t.Fatalf("finding task %d: %v", taskID, err)
	}
	tk, err := task.Read(path)
	if err != nil {
		t.Fatalf("reading task %d: %v", taskID, err)
	}
	tk.Body = body
	if err := task.Write(path, tk); err != nil {
		t.Fatalf("writing task %d: %v", taskID, err)
	}
}

func TestBoard_DetailShowsBodyChangedOnDisk(t *testing.T) {
	b, cfg := setupRelationNavBoard(t)
	setTaskBody(t, cfg, 1, "BODY BEFORE RELOAD")
	b = sendKey(b, "r")
	b = sendSpecialKey(b, tea.KeyEnter)
	if !containsStr(b.View(), "BODY BEFORE RELOAD") {
		t.Fatalf("initial body is not rendered:\n%s", b.View())
	}

	setTaskBody(t, cfg, 1, "BODY AFTER RELOAD")
	b = sendReload(b)

	v := b.View()
	if !containsStr(v, "BODY AFTER RELOAD") {
		t.Errorf("reload did not surface the changed body:\n%s", v)
	}
	if containsStr(v, "BODY BEFORE RELOAD") {
		t.Errorf("stale body survived the reload:\n%s", v)
	}
}

// setupDeepChainBoard writes a seven-task parent chain and opens the detail view
// of its deepest task with a fixed level budget.
func setupDeepChainBoard(t *testing.T, levels int) *tui.Board {
	t.Helper()

	names := []string{
		"Chain Alpha", "Chain Bravo", "Chain Charlie", "Chain Delta",
		"Chain Echo", "Chain Foxtrot", "Chain Golf",
	}
	tasks := make([]*task.Task, 0, len(names))
	for i, name := range names {
		tk := &task.Task{
			ID: i + 1, Title: name, Status: statusTodo,
			Priority: "medium", Updated: testRefTime,
		}
		if i > 0 {
			parent := i
			tk.Parent = &parent
		}
		tasks = append(tasks, tk)
	}
	b, cfg := newFileBoard(t, "Deep Chain Board", tasks)
	cfg.TUI.HierarchyLevels = &levels
	return openTaskDetail(t, b, names[len(names)-1])
}

// hierarchyBlockLines returns the rendered tree block: the lines from the
// `Hierarchy` heading up to the following blank line, trailing padding removed.
func hierarchyBlockLines(t *testing.T, v string) []string {
	t.Helper()
	lines := strings.Split(stripANSI(v), "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "Hierarchy" {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("no Hierarchy block in the view:\n%s", v)
	}
	var out []string
	for _, line := range lines[start+1:] {
		if strings.TrimSpace(line) == "" {
			break
		}
		out = append(out, strings.TrimRight(line, " "))
	}
	return out
}

func TestBoard_DetailZeroLevelsShowsOnlyTheOpenTicketAndEllipsis(t *testing.T) {
	// Zero levels leaves a one-row tree, which is only shown because it is cut
	// off — below on the milestone, above on the story. Each side alone has to
	// keep the block on screen.
	milestone := setupHierarchyTreeBoard(t, "Milestone One", 0)
	wantMilestone := []string{
		"  └─ #1 [todo] Milestone One (1/2 done)",
		"  …",
	}
	if got := hierarchyBlockLines(t, milestone.View()); !reflect.DeepEqual(got, wantMilestone) {
		t.Errorf("tree of #1 at zero levels =\n%s\nwant\n%s",
			strings.Join(got, "\n"), strings.Join(wantMilestone, "\n"))
	}

	story := setupHierarchyTreeBoard(t, "Story Four", 0)
	wantStory := []string{
		"  …",
		"  └─ #4 [done] Story Four",
	}
	if got := hierarchyBlockLines(t, story.View()); !reflect.DeepEqual(got, wantStory) {
		t.Errorf("tree of #4 at zero levels =\n%s\nwant\n%s",
			strings.Join(got, "\n"), strings.Join(wantStory, "\n"))
	}
}

func TestBoard_DetailCounterOnlyWhereChildrenAreHidden(t *testing.T) {
	// One level down from the milestone: its own children stand below it, the
	// children of the two epics do not. Two levels down nothing is left to
	// summarize, and every counter is gone.
	oneLevel := setupHierarchyTreeBoard(t, "Milestone One", 1)
	wantOne := []string{
		"  └─ #1 [todo] Milestone One",
		"     ├─ #2 [done] Epic Two (2/2 done)",
		"     └─ #3 [backlog] Epic Three (0/1 done)",
		"     …",
	}
	if got := hierarchyBlockLines(t, oneLevel.View()); !reflect.DeepEqual(got, wantOne) {
		t.Errorf("tree of #1 at one level =\n%s\nwant\n%s",
			strings.Join(got, "\n"), strings.Join(wantOne, "\n"))
	}

	twoLevels := setupHierarchyTreeBoard(t, "Milestone One", 2)
	wantTwo := []string{
		"  └─ #1 [todo] Milestone One",
		"     ├─ #2 [done] Epic Two",
		"     │  ├─ #4 [done] Story Four",
		"     │  └─ #5 [done] Story Five",
		"     └─ #3 [backlog] Epic Three",
		"        └─ #6 [backlog] Story Six",
	}
	if got := hierarchyBlockLines(t, twoLevels.View()); !reflect.DeepEqual(got, wantTwo) {
		t.Errorf("tree of #1 at two levels =\n%s\nwant\n%s",
			strings.Join(got, "\n"), strings.Join(wantTwo, "\n"))
	}
}

func TestBoard_DetailSixLevelsWalksDeepBoard(t *testing.T) {
	// Depth comes from the parent chain alone, so a six-level budget walks a
	// seven-task chain to both ends and cuts nothing off.
	b := setupDeepChainBoard(t, 6)

	got := hierarchyBlockLines(t, b.View())

	// Every task of the chain has exactly one child and the tree shows it, so
	// no row of it carries a counter.
	want := []string{
		"  └─ #1 [todo] Chain Alpha",
		"     └─ #2 [todo] Chain Bravo",
		"        └─ #3 [todo] Chain Charlie",
		"           └─ #4 [todo] Chain Delta",
		"              └─ #5 [todo] Chain Echo",
		"                 └─ #6 [todo] Chain Foxtrot",
		"                    └─ #7 [todo] Chain Golf",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tree of #7 at six levels =\n%s\nwant\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
