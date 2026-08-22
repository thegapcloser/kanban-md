package tui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/antopolskiy/kanban-md/internal/config"
	"github.com/antopolskiy/kanban-md/internal/task"
	"github.com/antopolskiy/kanban-md/internal/tui"
)

// titleGrandchild is the indirect descendant that must never show up as a
// direct child of the epic.
const titleGrandchild = "Grandchild"

// setupRelationNavBoard builds a board whose tree covers every navigability
// case of the detail view: resolvable parent and children (#1..#3, #7), an
// archived child of #1 (#4), a dangling parent reference (#5), and a child of
// that archived parent (#6).
func setupRelationNavBoard(t *testing.T) (*tui.Board, *config.Config) {
	t.Helper()

	dir := t.TempDir()
	kanbanDir := filepath.Join(dir, "kanban")
	tasksDir := filepath.Join(kanbanDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0o750); err != nil {
		t.Fatalf("creating dirs: %v", err)
	}

	cfg := config.NewDefault("Relation Nav Board")
	cfg.SetDir(kanbanDir)
	if err := cfg.Save(); err != nil {
		t.Fatalf("saving config: %v", err)
	}

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
		"Children (1/2 done)",
		"├─ #2 [todo] Child One",
		"└─ #3 [done] Child Two",
	} {
		if !containsStr(v, want) {
			t.Errorf("detail view of #1 missing %q:\n%s", want, v)
		}
	}
	// #4 is an archived child of #1 and #7 an indirect descendant: the children
	// list must hold neither, and both are present in the fixture so the
	// assertion can actually fail.
	for _, unwanted := range []string{"Archived Thing", titleGrandchild} {
		if containsStr(v, unwanted) {
			t.Errorf("detail view of #1 should not list %q:\n%s", unwanted, v)
		}
	}
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
		t.Errorf("relation row is missing the two-cell gutter:\n%s", v)
	}
	for _, marker := range []string{"> ├─", "> └─", "> ↑ Parent"} {
		if containsStr(v, marker) {
			t.Errorf("detail view marks a relation with %q before the cursor was activated:\n%s", marker, v)
		}
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
		"> ├─ #2 [todo] Child One",
		"> └─ #3 [done] Child Two",
		"> ├─ #2 [todo] Child One",
	} {
		b = sendSpecialKey(b, tea.KeyTab)
		if !containsStr(b.View(), want) {
			t.Fatalf("tab press %d: cursor not on %q:\n%s", i+1, want, b.View())
		}
	}

	b = sendSpecialKey(b, tea.KeyShiftTab)
	if !containsStr(b.View(), "> └─ #3 [done] Child Two") {
		t.Errorf("shift+tab did not wrap backwards:\n%s", b.View())
	}
}

func TestBoard_DetailCursorVisitsParentFirst(t *testing.T) {
	b, _ := setupRelationNavBoard(t)
	b = openTaskDetail(t, b, "Child One")

	b = sendSpecialKey(b, tea.KeyTab)
	if !containsStr(b.View(), "> ↑ Parent  #1 [backlog] Epic Alpha") {
		t.Fatalf("first tab did not select the parent row:\n%s", b.View())
	}
	b = sendSpecialKey(b, tea.KeyTab)
	if !containsStr(b.View(), "> └─ #7 [todo] Grandchild") {
		t.Errorf("second tab did not select the child row:\n%s", b.View())
	}
}

func TestBoard_DetailNonNavigableRelationIsNoCursorStop(t *testing.T) {
	for _, tt := range []struct {
		name     string
		title    string
		wantLine string
	}{
		{name: "dangling parent", title: "Orphan Child", wantLine: "  ↑ Parent  #999"},
		{
			name:     "archived parent",
			title:    "Archived Parent Child",
			wantLine: "  ↑ Parent  #4 [archived] Archived Thing",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := setupRelationNavBoard(t)
			b = openTaskDetail(t, b, tt.title)
			b = sendSpecialKey(b, tea.KeyTab)
			v := b.View()

			if !containsStr(v, tt.wantLine) {
				t.Errorf("relation row is not rendered as %q:\n%s", tt.wantLine, v)
			}
			if containsStr(v, "> ↑ Parent") {
				t.Errorf("tab put the cursor on a non-navigable relation:\n%s", v)
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
		t.Fatalf("relation rows are still visible after scrolling to the bottom:\n%s", b.View())
	}

	b = sendSpecialKey(b, tea.KeyTab)
	if !containsStr(b.View(), "> ├─ #2") {
		t.Errorf("tab did not scroll the cursor row back into view:\n%s", b.View())
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
