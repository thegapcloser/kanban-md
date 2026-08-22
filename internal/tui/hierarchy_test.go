package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/antopolskiy/kanban-md/internal/config"
	"github.com/antopolskiy/kanban-md/internal/task"
)

const hierarchyStatusInProgress = "in-progress"

// setupHierarchyBoard creates a board with a three-level tree:
// #1 milestone (depth 0) → #2, #3 epics (depth 1) → #4, #5 stories (depth 2).
func setupHierarchyBoard(t *testing.T, levelColors bool) *Board {
	t.Helper()

	kanbanDir := filepath.Join(t.TempDir(), "kanban")
	tasksDir := filepath.Join(kanbanDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0o750); err != nil {
		t.Fatalf("creating dirs: %v", err)
	}

	cfg := config.NewDefault("Hierarchy Board")
	cfg.TUI.LevelColors = levelColors
	cfg.SetDir(kanbanDir)
	if err := cfg.Save(); err != nil {
		t.Fatalf("saving config: %v", err)
	}

	one, two, three := 1, 2, 3
	fixtures := []*task.Task{
		{ID: 1, Title: "Milestone One", Status: dragStatusBacklog, Priority: "medium"},
		{ID: 2, Title: "Epic Alpha", Status: dragStatusBacklog, Priority: "medium", Parent: &one},
		{ID: 3, Title: "Epic Beta", Status: hierarchyStatusInProgress, Priority: "medium", Parent: &one},
		{ID: 4, Title: "Story One", Status: dragStatusBacklog, Priority: "medium", Parent: &two},
		{ID: 5, Title: "Story Two", Status: hierarchyStatusInProgress, Priority: "medium", Parent: &three},
	}
	for _, tk := range fixtures {
		path := filepath.Join(tasksDir, task.GenerateFilename(tk.ID, tk.Title))
		if err := task.Write(path, tk); err != nil {
			t.Fatalf("writing task: %v", err)
		}
	}

	b := NewBoard(cfg)
	b.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return b
}

func press(t *testing.T, b *Board, key string) *Board {
	t.Helper()
	m, _ := b.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	updated, ok := m.(*Board)
	if !ok {
		t.Fatalf("Update returned %T, want *Board", m)
	}
	return updated
}

func TestLevelFilterCyclesThroughDepths(t *testing.T) {
	b := setupHierarchyBoard(t, true)

	if got := len(b.tasks); got != 5 {
		t.Fatalf("unfiltered board shows %d tasks, want 5", got)
	}

	// L → only depth 0 (the milestone).
	b = press(t, b, "v")
	if got := len(b.tasks); got != 1 {
		t.Errorf("level 0 shows %d tasks, want 1", got)
	}

	// L → only depth 1 (the two epics).
	b = press(t, b, "v")
	if got := len(b.tasks); got != 2 {
		t.Errorf("level 1 shows %d tasks, want 2", got)
	}

	// L → only depth 2 (the two stories).
	b = press(t, b, "v")
	if got := len(b.tasks); got != 2 {
		t.Errorf("level 2 shows %d tasks, want 2", got)
	}

	// L → back to all levels; the deepest level is 2, so the cycle wraps here.
	b = press(t, b, "v")
	if got := len(b.tasks); got != 5 {
		t.Errorf("after wrapping, board shows %d tasks, want 5", got)
	}
}

func TestLevelFilterIsAListedStatusBarAction(t *testing.T) {
	b := setupHierarchyBoard(t, true)

	// The action is always listed so the shortcut is discoverable, and it
	// doubles as the indicator of the active level.
	if got := b.renderStatusBar(); !strings.Contains(got, "level[all]") {
		t.Errorf("status bar %q does not list level[all] while unfiltered", got)
	}

	b = press(t, b, "v")
	if got := b.renderStatusBar(); !strings.Contains(got, "level[0]") {
		t.Errorf("status bar %q does not show level[0] after filtering to depth 0", got)
	}

	b = press(t, b, "v")
	if got := b.renderStatusBar(); !strings.Contains(got, "level[1]") {
		t.Errorf("status bar %q does not show level[1] after filtering to depth 1", got)
	}
}

func TestLevelFilterKeepsSelectionInBounds(t *testing.T) {
	b := setupHierarchyBoard(t, true)

	// Move to the last card of the backlog column, then filter it away.
	b = press(t, b, "j")
	b = press(t, b, "j")
	b = press(t, b, "v")

	if b.activeRow >= len(b.columns[b.activeCol].tasks) && len(b.columns[b.activeCol].tasks) > 0 {
		t.Errorf("activeRow %d out of bounds after filtering", b.activeRow)
	}
	b.View() // must not panic
}

func TestLevelBorderColorsAreDistinct(t *testing.T) {
	seen := map[string]int{}
	for depth := range 3 {
		seen[string(levelBorderColor(depth))] = depth
	}
	if len(seen) != 3 {
		t.Errorf("levelBorderColor produces %d distinct colors for depths 0-2, want 3", len(seen))
	}
	if levelBorderColor(99) != levelBorderColor(len(levelBorderColors)-1) {
		t.Error("levelBorderColor does not clamp depths beyond the palette")
	}
}

func TestActiveCardUsesThickBorderWhenLevelColorsAreOn(t *testing.T) {
	b := setupHierarchyBoard(t, true)

	view := b.View()
	if !strings.Contains(view, "┏") {
		t.Error("selected card does not use a thick border")
	}
	if !strings.Contains(view, "╭") {
		t.Error("unselected cards no longer use the rounded border")
	}
}

func TestCardBordersUnchangedWhenLevelColorsAreOff(t *testing.T) {
	b := setupHierarchyBoard(t, false)

	if view := b.View(); strings.Contains(view, "┏") {
		t.Error("selected card uses a thick border although tui.level_colors is off")
	}

	// The filter itself stays available; only the colors are opt-in.
	b = press(t, b, "v")
	if got := len(b.tasks); got != 1 {
		t.Errorf("level 0 shows %d tasks with level colors off, want 1", got)
	}
}

func TestLevelFilterCombinesWithSearch(t *testing.T) {
	b := setupHierarchyBoard(t, true)

	// Level 1 alone holds both epics.
	b = press(t, b, "v")
	b = press(t, b, "v")
	if got := len(b.tasks); got != 2 {
		t.Fatalf("level 1 shows %d tasks, want 2", got)
	}

	// A search on top must narrow further, not replace the level filter.
	b = press(t, b, "/")
	b = press(t, b, "Alpha")
	if got := len(b.tasks); got != 1 {
		t.Errorf("level 1 plus search %q shows %d tasks, want 1", "Alpha", got)
	}
	if got := b.tasks[0].ID; got != 2 {
		t.Errorf("visible task is #%d, want #2 (Epic Alpha)", got)
	}
}

func TestLevelFilterKeyIsInertWhileTyping(t *testing.T) {
	b := setupHierarchyBoard(t, true)

	b = press(t, b, "/")
	b = press(t, b, "v")

	if b.levelFilterOn {
		t.Error("typing v into the search box cycled the level filter")
	}
	if got := b.searchInput.Value(); got != "v" {
		t.Errorf("search input = %q, want %q", got, "v")
	}
}

func TestLevelFilterWithHiddenEmptyColumns(t *testing.T) {
	// Filtering can empty every column but one. With hide_empty_columns on,
	// the columns disappear underneath the cursor.
	b := setupHierarchyBoard(t, true)
	b.hideEmptyColumns = true
	b.loadTasks()

	// Park the cursor on the in-progress column, which holds no milestone.
	b = press(t, b, "l")
	inProgress := b.columns[b.activeCol].status

	b = press(t, b, "v") // level 0: milestone #1 sits in backlog only

	if len(b.columns) != 1 {
		t.Fatalf("board kept %d columns, want only the one holding the milestone", len(b.columns))
	}
	if b.columns[0].status == inProgress {
		t.Errorf("surviving column is %q, want the one holding the milestone", inProgress)
	}
	if b.activeCol >= len(b.columns) {
		t.Fatalf("activeCol %d out of range for %d columns", b.activeCol, len(b.columns))
	}
	b.View() // must not panic
}

func TestBlockedCardKeepsRedBorderWithLevelColors(t *testing.T) {
	// Blocked is a warning, not a classification: it must win over the depth
	// color in both directions of the setting.
	for _, levelColors := range []bool{false, true} {
		b := setupHierarchyBoard(t, levelColors)
		blocked := &task.Task{ID: 1, Blocked: true}

		got := b.cardBorderStyle(blocked, false).GetBorderTopForeground()
		if want := blockedCardStyle.GetBorderTopForeground(); got != want {
			t.Errorf("level_colors=%v: blocked border = %v, want the blocked red %v",
				levelColors, got, want)
		}
	}
}

func TestLevelColorsOffIgnoresDepthEntirely(t *testing.T) {
	b := setupHierarchyBoard(t, false)

	milestone := &task.Task{ID: 1}
	story := &task.Task{ID: 4}
	if a, c := b.cardBorderStyle(milestone, false), b.cardBorderStyle(story, false); //
	a.GetBorderTopForeground() != c.GetBorderTopForeground() {
		t.Error("cards of different depths differ in color although tui.level_colors is off")
	}
}

func TestLevelBorderColorCoversEveryDepthOfTheBoard(t *testing.T) {
	// Every depth the palette names must be distinct; deeper levels share the
	// last entry rather than wrapping back to the top, which would make a
	// level-4 task look like a milestone.
	seen := make(map[lipgloss.Color]bool, len(levelBorderColors))
	for depth := range len(levelBorderColors) {
		seen[levelBorderColor(depth)] = true
	}
	if len(seen) != len(levelBorderColors) {
		t.Errorf("palette has %d distinct colors for %d levels", len(seen), len(levelBorderColors))
	}

	last := levelBorderColor(len(levelBorderColors) - 1)
	for _, depth := range []int{len(levelBorderColors), len(levelBorderColors) + 50, -1, -99} {
		if depth >= 0 && levelBorderColor(depth) != last {
			t.Errorf("levelBorderColor(%d) = %v, want the last palette entry %v",
				depth, levelBorderColor(depth), last)
		}
		if depth < 0 && levelBorderColor(depth) != levelBorderColors[0] {
			t.Errorf("levelBorderColor(%d) = %v, want the first palette entry",
				depth, levelBorderColor(depth))
		}
	}

	// The depth colors must not collide with the two colors that carry meaning
	// of their own.
	reserved := map[lipgloss.Color]string{
		blockedCardStyle.GetBorderTopForeground().(lipgloss.Color): "blocked",
		activeCardStyle.GetBorderTopForeground().(lipgloss.Color):  "selected",
	}
	for depth, c := range levelBorderColors {
		if name, clash := reserved[c]; clash {
			t.Errorf("level %d uses the %s color %v", depth, name, c)
		}
	}
}

func TestLevelDepthCountsArchivedParents(t *testing.T) {
	// An archived parent is filtered off the board but still determines how
	// deep its children sit, otherwise children jump a level when a parent is
	// archived.
	b := setupHierarchyBoard(t, true)
	moveTaskToStatus(t, b, 2, "archived") // Epic Alpha, parent of story #4
	b.loadTasks()

	if got := b.taskDepths[4]; got != 2 {
		t.Errorf("depth of #4 = %d, want 2 with its parent archived", got)
	}

	b = press(t, b, "v")
	b = press(t, b, "v") // level 1: only the still-active epic #3
	if got := len(b.tasks); got != 1 {
		t.Errorf("level 1 shows %d tasks, want 1 (the archived epic is off the board)", got)
	}
}

func TestLevelFilterOnEmptyBoardDoesNotPanic(t *testing.T) {
	kanbanDir := filepath.Join(t.TempDir(), "kanban")
	if err := os.MkdirAll(filepath.Join(kanbanDir, "tasks"), 0o750); err != nil {
		t.Fatalf("creating dirs: %v", err)
	}
	cfg := config.NewDefault("Empty Board")
	cfg.TUI.LevelColors = true
	cfg.SetDir(kanbanDir)
	if err := cfg.Save(); err != nil {
		t.Fatalf("saving config: %v", err)
	}

	b := NewBoard(cfg)
	b.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	for range 3 {
		b = press(t, b, "v")
		b.View()
	}
}

// moveTaskToStatus rewrites a task file's status directly, then leaves the
// caller to reload the board.
func moveTaskToStatus(t *testing.T, b *Board, id int, status string) {
	t.Helper()
	for _, tk := range b.allTasks {
		if tk.ID != id {
			continue
		}
		tk.Status = status
		if err := task.Write(tk.File, tk); err != nil {
			t.Fatalf("writing task #%d: %v", id, err)
		}
		return
	}
	t.Fatalf("task #%d not found", id)
}
