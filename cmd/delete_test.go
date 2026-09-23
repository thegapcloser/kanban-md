package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thegapcloser/kanban-md/internal/clierr"
	"github.com/thegapcloser/kanban-md/internal/config"
	"github.com/thegapcloser/kanban-md/internal/task"
)

const testDeleteAgent = "agent-1"

// writeDeleteTask creates a task file in the given config's tasks directory.
func writeDeleteTask(t *testing.T, cfg *config.Config, tk *task.Task) {
	t.Helper()
	slug := task.GenerateSlug(tk.Title)
	filename := task.GenerateFilename(tk.ID, slug)
	path := filepath.Join(cfg.TasksPath(), filename)
	if err := task.Write(path, tk); err != nil {
		t.Fatal(err)
	}
}

// --- executeDelete tests ---

func TestExecuteDelete_BasicDelete(t *testing.T) {
	kanbanDir := setupBoard(t)
	cfg, err := config.Load(kanbanDir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	writeDeleteTask(t, cfg, &task.Task{
		ID:       1,
		Title:    "Delete me",
		Status:   "backlog",
		Priority: "medium",
		Created:  now,
		Updated:  now,
	})

	err = executeDelete(cfg, 1)
	if err != nil {
		t.Fatalf("executeDelete error: %v", err)
	}

	// Verify the task was soft-deleted (moved to archived).
	path, err := task.FindByID(cfg.TasksPath(), 1)
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if tk.Status != config.ArchivedStatus {
		t.Errorf("Status = %q, want %q", tk.Status, config.ArchivedStatus)
	}
}

func TestExecuteDelete_TaskNotFound(t *testing.T) {
	kanbanDir := setupBoard(t)
	cfg, err := config.Load(kanbanDir)
	if err != nil {
		t.Fatal(err)
	}

	err = executeDelete(cfg, 999)
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}
}

func TestExecuteDelete_ClaimedByOtherAgent(t *testing.T) {
	kanbanDir := setupBoard(t)
	cfg, err := config.Load(kanbanDir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	writeDeleteTask(t, cfg, &task.Task{
		ID:        1,
		Title:     "Claimed task",
		Status:    "in-progress",
		Priority:  "medium",
		ClaimedBy: testDeleteAgent,
		ClaimedAt: &now,
		Created:   now,
		Updated:   now,
	})

	// executeDelete passes empty claimant, so any active claim blocks delete.
	err = executeDelete(cfg, 1)
	if err == nil {
		t.Fatal("expected error for claimed task")
	}
	var cliErr *clierr.Error
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected clierr.Error, got %T", err)
	}
	if cliErr.Code != clierr.TaskClaimed {
		t.Errorf("code = %q, want %q", cliErr.Code, clierr.TaskClaimed)
	}
}

func TestExecuteDelete_ExpiredClaimAllows(t *testing.T) {
	kanbanDir := setupBoard(t)
	cfg, err := config.Load(kanbanDir)
	if err != nil {
		t.Fatal(err)
	}

	past := time.Now().Add(-48 * time.Hour)
	writeDeleteTask(t, cfg, &task.Task{
		ID:        1,
		Title:     "Expired claim task",
		Status:    "in-progress",
		Priority:  "medium",
		ClaimedBy: testDeleteAgent,
		ClaimedAt: &past,
		Created:   past,
		Updated:   past,
	})

	err = executeDelete(cfg, 1)
	if err != nil {
		t.Fatalf("expected expired claim to allow delete, got: %v", err)
	}
}

// --- deleteSingleTask tests ---

func TestDeleteSingleTask_WithYesTableOutput(t *testing.T) {
	kanbanDir := setupBoard(t)
	cfg, err := config.Load(kanbanDir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	writeDeleteTask(t, cfg, &task.Task{
		ID:       1,
		Title:    "Task to delete",
		Status:   "backlog",
		Priority: "medium",
		Created:  now,
		Updated:  now,
	})

	setFlags(t, false, true, false)
	r, w := captureStdout(t)

	err = deleteSingleTask(cfg, 1, true)
	got := drainPipe(t, r, w)

	if err != nil {
		t.Fatalf("deleteSingleTask error: %v", err)
	}
	if !containsSubstring(got, "Deleted task #1") {
		t.Errorf("expected 'Deleted task #1' in output, got: %s", got)
	}
}

func TestDeleteSingleTask_WithYesJSONOutput(t *testing.T) {
	kanbanDir := setupBoard(t)
	cfg, err := config.Load(kanbanDir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	writeDeleteTask(t, cfg, &task.Task{
		ID:       1,
		Title:    "JSON delete",
		Status:   "backlog",
		Priority: "medium",
		Created:  now,
		Updated:  now,
	})

	setFlags(t, true, false, false)
	r, w := captureStdout(t)

	err = deleteSingleTask(cfg, 1, true)
	got := drainPipe(t, r, w)

	if err != nil {
		t.Fatalf("deleteSingleTask error: %v", err)
	}
	if !containsSubstring(got, `"status"`) {
		t.Errorf("expected JSON with status field, got: %s", got)
	}
	if !containsSubstring(got, `"deleted"`) {
		t.Errorf("expected 'deleted' in JSON, got: %s", got)
	}
}

func TestDeleteSingleTask_TaskNotFound(t *testing.T) {
	kanbanDir := setupBoard(t)
	cfg, err := config.Load(kanbanDir)
	if err != nil {
		t.Fatal(err)
	}

	err = deleteSingleTask(cfg, 999, true)
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}
}

func TestDeleteSingleTask_ClaimedTaskBlocked(t *testing.T) {
	kanbanDir := setupBoard(t)
	cfg, err := config.Load(kanbanDir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	writeDeleteTask(t, cfg, &task.Task{
		ID:        1,
		Title:     "Claimed",
		Status:    "in-progress",
		Priority:  "medium",
		ClaimedBy: testDeleteAgent,
		ClaimedAt: &now,
		Created:   now,
		Updated:   now,
	})

	err = deleteSingleTask(cfg, 1, true)
	if err == nil {
		t.Fatal("expected error for claimed task")
	}
	var cliErr *clierr.Error
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected clierr.Error, got %T", err)
	}
	if cliErr.Code != clierr.TaskClaimed {
		t.Errorf("code = %q, want %q", cliErr.Code, clierr.TaskClaimed)
	}
}

func TestDeleteSingleTask_NonTTYRequiresYes(t *testing.T) {
	kanbanDir := setupBoard(t)
	cfg, err := config.Load(kanbanDir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	writeDeleteTask(t, cfg, &task.Task{
		ID:       1,
		Title:    "Non-TTY task",
		Status:   "backlog",
		Priority: "medium",
		Created:  now,
		Updated:  now,
	})

	// Without --yes and in non-TTY (test environment), should fail.
	err = deleteSingleTask(cfg, 1, false)
	if err == nil {
		t.Fatal("expected error for non-TTY without --yes")
	}
	var cliErr *clierr.Error
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected clierr.Error, got %T", err)
	}
	if cliErr.Code != clierr.ConfirmationReq {
		t.Errorf("code = %q, want %q", cliErr.Code, clierr.ConfirmationReq)
	}
}

// --- softDeleteAndLog tests ---

func TestSoftDeleteAndLog_BasicArchive(t *testing.T) {
	kanbanDir := setupBoard(t)
	cfg, err := config.Load(kanbanDir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	tk := &task.Task{
		ID:       1,
		Title:    "Archive me",
		Status:   "backlog",
		Priority: "medium",
		Created:  now,
		Updated:  now,
	}
	writeDeleteTask(t, cfg, tk)
	path, err := task.FindByID(cfg.TasksPath(), 1)
	if err != nil {
		t.Fatal(err)
	}

	err = softDeleteAndLog(cfg, path, tk)
	if err != nil {
		t.Fatalf("softDeleteAndLog error: %v", err)
	}
	if tk.Status != config.ArchivedStatus {
		t.Errorf("Status = %q, want %q", tk.Status, config.ArchivedStatus)
	}

	// Verify activity log was written.
	logPath := filepath.Join(kanbanDir, "activity.jsonl")
	data, err := os.ReadFile(logPath) //nolint:gosec // test path
	if err != nil {
		t.Fatal(err)
	}
	if !containsSubstring(string(data), "delete") {
		t.Errorf("expected 'delete' in activity log, got: %s", data)
	}
}

func TestSoftDeleteAndLog_AlreadyArchived(t *testing.T) {
	kanbanDir := setupBoard(t)
	cfg, err := config.Load(kanbanDir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	tk := &task.Task{
		ID:       1,
		Title:    "Already archived",
		Status:   config.ArchivedStatus,
		Priority: "medium",
		Created:  now,
		Updated:  now,
	}
	writeDeleteTask(t, cfg, tk)
	path, err := task.FindByID(cfg.TasksPath(), 1)
	if err != nil {
		t.Fatal(err)
	}

	err = softDeleteAndLog(cfg, path, tk)
	if err != nil {
		t.Fatalf("softDeleteAndLog error: %v", err)
	}
	// Status should remain archived (no-op).
	if tk.Status != config.ArchivedStatus {
		t.Errorf("Status = %q, want %q", tk.Status, config.ArchivedStatus)
	}
}

// --- warnDependents tests ---

func TestWarnDependents_NoDependents(t *testing.T) {
	kanbanDir := setupBoard(t)
	cfg, err := config.Load(kanbanDir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	writeDeleteTask(t, cfg, &task.Task{
		ID:       1,
		Title:    "Standalone",
		Status:   "backlog",
		Priority: "medium",
		Created:  now,
		Updated:  now,
	})

	rErr, wErr := captureStderr(t)
	warnDependents(cfg.TasksPath(), 1)
	got := drainPipe(t, rErr, wErr)

	if containsSubstring(got, "Warning") {
		t.Errorf("expected no warnings, got: %s", got)
	}
}

func TestWarnDependents_WithParentReference(t *testing.T) {
	kanbanDir := setupBoard(t)
	cfg, err := config.Load(kanbanDir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	writeDeleteTask(t, cfg, &task.Task{
		ID:       1,
		Title:    "Parent task",
		Status:   "backlog",
		Priority: "medium",
		Created:  now,
		Updated:  now,
	})
	parentID := 1
	writeDeleteTask(t, cfg, &task.Task{
		ID:       2,
		Title:    "Child task",
		Status:   "backlog",
		Priority: "medium",
		Parent:   &parentID,
		Created:  now,
		Updated:  now,
	})

	rErr, wErr := captureStderr(t)
	warnDependents(cfg.TasksPath(), 1)
	got := drainPipe(t, rErr, wErr)

	if !containsSubstring(got, "Warning") {
		t.Errorf("expected warning about dependent, got: %s", got)
	}
	if !containsSubstring(got, "#2") {
		t.Errorf("expected task #2 in warning, got: %s", got)
	}
}

func TestWarnDependents_WithDependsOnReference(t *testing.T) {
	kanbanDir := setupBoard(t)
	cfg, err := config.Load(kanbanDir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	writeDeleteTask(t, cfg, &task.Task{
		ID:       1,
		Title:    "Dependency task",
		Status:   "backlog",
		Priority: "medium",
		Created:  now,
		Updated:  now,
	})
	writeDeleteTask(t, cfg, &task.Task{
		ID:        2,
		Title:     "Dependent task",
		Status:    "backlog",
		Priority:  "medium",
		DependsOn: []int{1},
		Created:   now,
		Updated:   now,
	})

	rErr, wErr := captureStderr(t)
	warnDependents(cfg.TasksPath(), 1)
	got := drainPipe(t, rErr, wErr)

	if !containsSubstring(got, "Warning") {
		t.Errorf("expected warning about dependent, got: %s", got)
	}
	if !containsSubstring(got, "#2") {
		t.Errorf("expected task #2 in warning, got: %s", got)
	}
}
