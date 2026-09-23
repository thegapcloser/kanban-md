package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/thegapcloser/kanban-md/internal/clierr"
	"github.com/thegapcloser/kanban-md/internal/output"
	"github.com/thegapcloser/kanban-md/internal/task"
)

// newBoardCmd creates a fresh cobra command with board flags for testing.
func newBoardCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("group-by", "", "")
	return cmd
}

func TestRunBoard_TableOutput(t *testing.T) {
	kanbanDir := setupBoard(t)
	createTaskFile(t, kanbanDir+"/tasks", 1, "task-one")
	createTaskFile(t, kanbanDir+"/tasks", 2, "task-two")

	oldFlagDir := flagDir
	flagDir = kanbanDir
	t.Cleanup(func() { flagDir = oldFlagDir })

	setFlags(t, false, true, false)
	r, w := captureStdout(t)

	cmd := newBoardCmd()
	err := runBoard(cmd, nil)

	got := drainPipe(t, r, w)

	if err != nil {
		t.Fatalf("runBoard error: %v", err)
	}
	if !containsSubstring(got, testBoardName) {
		t.Errorf("expected board name %q in output, got: %s", testBoardName, got)
	}
	if !containsSubstring(got, "STATUS") {
		t.Errorf("expected STATUS header in table output, got: %s", got)
	}
}

func TestRunBoard_JSONOutput(t *testing.T) {
	kanbanDir := setupBoard(t)
	createTaskFile(t, kanbanDir+"/tasks", 1, "json-task")

	oldFlagDir := flagDir
	flagDir = kanbanDir
	t.Cleanup(func() { flagDir = oldFlagDir })

	setFlags(t, true, false, false)
	r, w := captureStdout(t)

	cmd := newBoardCmd()
	err := runBoard(cmd, nil)

	got := drainPipe(t, r, w)

	if err != nil {
		t.Fatalf("runBoard error: %v", err)
	}
	if !containsSubstring(got, `"board_name"`) {
		t.Errorf("expected JSON board_name field, got: %s", got)
	}
}

func TestRunBoard_CompactOutput(t *testing.T) {
	kanbanDir := setupBoard(t)
	createTaskFile(t, kanbanDir+"/tasks", 1, "compact-task")

	oldFlagDir := flagDir
	flagDir = kanbanDir
	t.Cleanup(func() { flagDir = oldFlagDir })

	setFlags(t, false, false, true)
	r, w := captureStdout(t)

	cmd := newBoardCmd()
	err := runBoard(cmd, nil)

	got := drainPipe(t, r, w)

	if err != nil {
		t.Fatalf("runBoard error: %v", err)
	}
	if !containsSubstring(got, testBoardName) {
		t.Errorf("expected board name in compact output, got: %s", got)
	}
	// Compact output should include "tasks" count.
	if !containsSubstring(got, "tasks") {
		t.Errorf("expected 'tasks' in compact output, got: %s", got)
	}
}

func TestRunBoard_EmptyBoard(t *testing.T) {
	kanbanDir := setupBoard(t)

	oldFlagDir := flagDir
	flagDir = kanbanDir
	t.Cleanup(func() { flagDir = oldFlagDir })

	setFlags(t, false, true, false)
	r, w := captureStdout(t)

	cmd := newBoardCmd()
	err := runBoard(cmd, nil)

	got := drainPipe(t, r, w)

	if err != nil {
		t.Fatalf("runBoard error: %v", err)
	}
	if !containsSubstring(got, "Total: 0 tasks") {
		t.Errorf("expected 'Total: 0 tasks' for empty board, got: %s", got)
	}
}

func TestRunBoard_InvalidGroupBy(t *testing.T) {
	kanbanDir := setupBoard(t)

	oldFlagDir := flagDir
	flagDir = kanbanDir
	t.Cleanup(func() { flagDir = oldFlagDir })

	cmd := newBoardCmd()
	_ = cmd.Flags().Set("group-by", "nonexistent")

	err := runBoard(cmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid --group-by field")
	}
	var cliErr *clierr.Error
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected clierr.Error, got %T", err)
	}
	if cliErr.Code != clierr.InvalidGroupBy {
		t.Errorf("code = %q, want %q", cliErr.Code, clierr.InvalidGroupBy)
	}
}

func TestRunBoard_GroupByPriority_Table(t *testing.T) {
	kanbanDir := setupBoard(t)
	createTaskFile(t, kanbanDir+"/tasks", 1, "high-task")
	createTaskFile(t, kanbanDir+"/tasks", 2, "low-task")

	oldFlagDir := flagDir
	flagDir = kanbanDir
	t.Cleanup(func() { flagDir = oldFlagDir })

	setFlags(t, false, true, false)
	r, w := captureStdout(t)

	cmd := newBoardCmd()
	_ = cmd.Flags().Set("group-by", "priority")

	err := runBoard(cmd, nil)

	got := drainPipe(t, r, w)

	if err != nil {
		t.Fatalf("runBoard error: %v", err)
	}
	// Grouped output should show the priority group with task count.
	if !containsSubstring(got, "medium") {
		t.Errorf("expected 'medium' priority group in output, got: %s", got)
	}
}

func TestRunBoard_GroupByPriority_JSON(t *testing.T) {
	kanbanDir := setupBoard(t)
	createTaskFile(t, kanbanDir+"/tasks", 1, "json-group")

	oldFlagDir := flagDir
	flagDir = kanbanDir
	t.Cleanup(func() { flagDir = oldFlagDir })

	setFlags(t, true, false, false)
	r, w := captureStdout(t)

	cmd := newBoardCmd()
	_ = cmd.Flags().Set("group-by", "priority")

	err := runBoard(cmd, nil)

	got := drainPipe(t, r, w)

	if err != nil {
		t.Fatalf("runBoard error: %v", err)
	}
	if !containsSubstring(got, `"groups"`) {
		t.Errorf("expected JSON groups field, got: %s", got)
	}
}

func TestRunBoard_NoConfig(t *testing.T) {
	dir := t.TempDir()

	oldFlagDir := flagDir
	flagDir = dir
	t.Cleanup(func() { flagDir = oldFlagDir })

	cmd := newBoardCmd()
	err := runBoard(cmd, nil)
	if err == nil {
		t.Fatal("expected error when no board config exists")
	}
}

func TestRenderBoard_FiltersArchived(t *testing.T) {
	kanbanDir := setupBoard(t)

	// Create a regular task and an "archived" task.
	createTaskFile(t, kanbanDir+"/tasks", 1, "active-task")
	createTaskFileWithStatus(t, kanbanDir+"/tasks", 2, "old-task", "archived")

	oldFlagDir := flagDir
	flagDir = kanbanDir
	t.Cleanup(func() { flagDir = oldFlagDir })

	setFlags(t, false, true, false)
	r, w := captureStdout(t)

	cmd := newBoardCmd()
	err := runBoard(cmd, nil)

	got := drainPipe(t, r, w)

	if err != nil {
		t.Fatalf("runBoard error: %v", err)
	}
	// Board shows "Total: 1 tasks" because archived is excluded.
	if !containsSubstring(got, "Total: 1 tasks") {
		t.Errorf("expected 'Total: 1 tasks' (archived excluded), got: %s", got)
	}
}

func TestRenderBoard_OutputFormats(t *testing.T) {
	kanbanDir := setupBoard(t)
	createTaskFile(t, kanbanDir+"/tasks", 1, "format-task")

	oldFlagDir := flagDir
	flagDir = kanbanDir
	t.Cleanup(func() { flagDir = oldFlagDir })

	tests := []struct {
		name   string
		format output.Format
		json   bool
		table  bool
		compt  bool
		want   string
	}{
		{"table", output.FormatTable, false, true, false, "STATUS"},
		{"compact", output.FormatCompact, false, false, true, "tasks)"},
		{"json", output.FormatJSON, true, false, false, `"board_name"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setFlags(t, tt.json, tt.table, tt.compt)
			r, w := captureStdout(t)

			cmd := newBoardCmd()
			err := runBoard(cmd, nil)

			got := drainPipe(t, r, w)

			if err != nil {
				t.Fatalf("runBoard error: %v", err)
			}
			if !containsSubstring(got, tt.want) {
				t.Errorf("expected %q in %s output, got: %s", tt.want, tt.name, got)
			}
		})
	}
}

// createTaskFileWithStatus creates a task markdown file with a specific status.
func createTaskFileWithStatus(t *testing.T, tasksDir string, id int, title, status string) {
	t.Helper()
	slug := task.GenerateSlug(title)
	filename := task.GenerateFilename(id, slug)
	content := "---\nid: " + idStr(id) + "\ntitle: " + title + "\nstatus: " + status + "\npriority: medium\ncreated: 2025-01-01T00:00:00Z\nupdated: 2025-01-01T00:00:00Z\n---\n"
	path := filepath.Join(tasksDir, filename)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
