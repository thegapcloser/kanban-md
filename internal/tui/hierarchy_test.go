package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/antopolskiy/kanban-md/internal/config"
	"github.com/antopolskiy/kanban-md/internal/task"
)

const hierarchyStatusInProgress = "in-progress"

// setupHierarchyBoard creates a board with a three-level tree:
// #1 milestone (depth 0) → #2, #3 epics (depth 1) → #4, #5 stories (depth 2).
func setupHierarchyBoard(t *testing.T) *Board {
	t.Helper()

	kanbanDir := filepath.Join(t.TempDir(), "kanban")
	tasksDir := filepath.Join(kanbanDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0o750); err != nil {
		t.Fatalf("creating dirs: %v", err)
	}

	cfg := config.NewDefault("Hierarchy Board")
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
	b := setupHierarchyBoard(t)

	if got := len(b.tasks); got != 5 {
		t.Fatalf("unfiltered board shows %d tasks, want 5", got)
	}

	// L → only depth 0 (the milestone).
	b = press(t, b, "L")
	if got := len(b.tasks); got != 1 {
		t.Errorf("level 0 shows %d tasks, want 1", got)
	}

	// L → only depth 1 (the two epics).
	b = press(t, b, "L")
	if got := len(b.tasks); got != 2 {
		t.Errorf("level 1 shows %d tasks, want 2", got)
	}

	// L → only depth 2 (the two stories).
	b = press(t, b, "L")
	if got := len(b.tasks); got != 2 {
		t.Errorf("level 2 shows %d tasks, want 2", got)
	}

	// L → back to all levels; the deepest level is 2, so the cycle wraps here.
	b = press(t, b, "L")
	if got := len(b.tasks); got != 5 {
		t.Errorf("after wrapping, board shows %d tasks, want 5", got)
	}
}

func TestLevelFilterIsAListedStatusBarAction(t *testing.T) {
	b := setupHierarchyBoard(t)

	// The action is always listed so the shortcut is discoverable, and it
	// doubles as the indicator of the active level.
	if got := b.renderStatusBar(); !strings.Contains(got, "level[all]") {
		t.Errorf("status bar %q does not list level[all] while unfiltered", got)
	}

	b = press(t, b, "L")
	if got := b.renderStatusBar(); !strings.Contains(got, "level[0]") {
		t.Errorf("status bar %q does not show level[0] after filtering to depth 0", got)
	}

	b = press(t, b, "L")
	if got := b.renderStatusBar(); !strings.Contains(got, "level[1]") {
		t.Errorf("status bar %q does not show level[1] after filtering to depth 1", got)
	}
}

func TestLevelFilterKeepsSelectionInBounds(t *testing.T) {
	b := setupHierarchyBoard(t)

	// Move to the last card of the backlog column, then filter it away.
	b = press(t, b, "j")
	b = press(t, b, "j")
	b = press(t, b, "L")

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

func TestActiveCardUsesThickBorderSoLevelColorSurvives(t *testing.T) {
	b := setupHierarchyBoard(t)

	view := b.View()
	if !strings.Contains(view, "┏") {
		t.Error("selected card does not use a thick border")
	}
	if !strings.Contains(view, "╭") {
		t.Error("unselected cards no longer use the rounded border")
	}
}
