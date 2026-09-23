package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/thegapcloser/kanban-md/internal/skill"
)

const skillNameKanbanMD = "kanban-md"

// ---------------------------------------------------------------------------
// selectModel.Init — trivial but uncovered
// ---------------------------------------------------------------------------

func TestSelectModel_Init_ReturnsNil(t *testing.T) {
	m := newTestModel()
	cmd := m.Init()
	if cmd != nil {
		t.Errorf("Init() = %v, want nil", cmd)
	}
}

// ---------------------------------------------------------------------------
// selectModel — arrow-key navigation (down/up)
// ---------------------------------------------------------------------------

func TestSelectModel_ArrowKeyNavigation(t *testing.T) {
	m := newTestModel()

	// Arrow down.
	m = sendTestSpecialKey(m, tea.KeyDown)
	if m.cursor != 1 {
		t.Errorf("cursor after down = %d, want 1", m.cursor)
	}

	// Arrow down again.
	m = sendTestSpecialKey(m, tea.KeyDown)
	if m.cursor != 2 {
		t.Errorf("cursor after down = %d, want 2", m.cursor)
	}

	// Arrow down at bottom — stays.
	m = sendTestSpecialKey(m, tea.KeyDown)
	if m.cursor != 2 {
		t.Errorf("cursor at bottom after down = %d, want 2", m.cursor)
	}

	// Arrow up.
	m = sendTestSpecialKey(m, tea.KeyUp)
	if m.cursor != 1 {
		t.Errorf("cursor after up = %d, want 1", m.cursor)
	}

	// Arrow up again.
	m = sendTestSpecialKey(m, tea.KeyUp)
	if m.cursor != 0 {
		t.Errorf("cursor after up = %d, want 0", m.cursor)
	}

	// Arrow up at top — stays.
	m = sendTestSpecialKey(m, tea.KeyUp)
	if m.cursor != 0 {
		t.Errorf("cursor at top after up = %d, want 0", m.cursor)
	}
}

// ---------------------------------------------------------------------------
// selectModel.Update — non-key messages are ignored
// ---------------------------------------------------------------------------

func TestSelectModel_NonKeyMessage(t *testing.T) {
	m := newTestModel()

	// WindowSizeMsg is not a KeyMsg — should be ignored.
	result, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	updated := result.(selectModel)

	if cmd != nil {
		t.Errorf("cmd = %v, want nil for non-key message", cmd)
	}
	if updated.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (unchanged)", updated.cursor)
	}
	if updated.done {
		t.Error("done should be false")
	}
}

// ---------------------------------------------------------------------------
// selectModel.View — cursor and checkbox rendering
// ---------------------------------------------------------------------------

func TestSelectModel_ViewCursorMarker(t *testing.T) {
	m := newTestModel()
	view := m.View()

	// The first item should have the cursor marker.
	lines := strings.Split(view, "\n")
	// Line 0 = prompt, line 1 = first item.
	if len(lines) < 2 {
		t.Fatalf("view has too few lines: %d", len(lines))
	}

	// Cursor on first item (index 0).
	if !strings.Contains(lines[1], "\u203a") { // › character
		t.Errorf("expected cursor marker on first item, got line: %q", lines[1])
	}
	// Second item should NOT have cursor marker.
	if len(lines) > 2 && strings.Contains(lines[2], "\u203a") {
		t.Errorf("cursor marker should not be on second item, got line: %q", lines[2])
	}
}

func TestSelectModel_ViewShowsPrompt(t *testing.T) {
	m := newTestModel()
	view := m.View()

	if !strings.Contains(view, "Test prompt:") {
		t.Errorf("view should contain prompt text, got:\n%s", view)
	}
}

func TestSelectModel_ViewShowsItemLabels(t *testing.T) {
	m := newTestModel()
	view := m.View()

	for _, item := range m.items {
		if !strings.Contains(view, item.label) {
			t.Errorf("view should contain item label %q, got:\n%s", item.label, view)
		}
	}
}

func TestSelectModel_ViewShowsDescriptions(t *testing.T) {
	m := newTestModel()
	view := m.View()

	for _, item := range m.items {
		if !strings.Contains(view, item.description) {
			t.Errorf("view should contain description %q, got:\n%s", item.description, view)
		}
	}
}

func TestSelectModel_ViewAfterCursorMove(t *testing.T) {
	m := newTestModel()
	m = sendTestKey(m, "j") // Move cursor to second item.
	view := m.View()

	lines := strings.Split(view, "\n")
	// Line 0 = prompt, line 1 = first item, line 2 = second item.
	if len(lines) < 3 {
		t.Fatalf("view has too few lines: %d", len(lines))
	}

	// First item should NOT have cursor.
	if strings.Contains(lines[1], "\u203a") {
		t.Errorf("cursor should not be on first item after move, got: %q", lines[1])
	}
	// Second item should have cursor.
	if !strings.Contains(lines[2], "\u203a") {
		t.Errorf("cursor should be on second item, got: %q", lines[2])
	}
}

// ---------------------------------------------------------------------------
// resolveAgents — non-interactive detection with multiple agent dirs
// ---------------------------------------------------------------------------

func TestResolveAgents_NonInteractiveDetectsMultipleAgents(t *testing.T) {
	projectRoot := t.TempDir()

	// Create directories for both claude and codex.
	for _, dir := range []string{".claude", ".agents"} {
		if err := os.MkdirAll(filepath.Join(projectRoot, dir), 0o750); err != nil {
			t.Fatal(err)
		}
	}

	agents := resolveAgents(nil, projectRoot, false)

	agentNames := make(map[string]bool)
	for _, a := range agents {
		agentNames[a.Name] = true
	}

	if !agentNames["claude"] {
		t.Error("expected claude in detected agents")
	}
	if !agentNames["codex"] {
		t.Error("expected codex in detected agents")
	}
}

func TestResolveAgents_NonInteractiveNoAgentsDetected(t *testing.T) {
	// Empty project root — no agent directories exist.
	projectRoot := t.TempDir()

	agents := resolveAgents(nil, projectRoot, false)

	if len(agents) != 0 {
		t.Errorf("expected empty agent list for empty project root, got %d agents", len(agents))
	}
}

func TestResolveAgents_FilterTakesPrecedenceOverDetection(t *testing.T) {
	// Even if no agent dirs exist, explicit filter should return the agent.
	projectRoot := t.TempDir()

	agents := resolveAgents([]string{"codex"}, projectRoot, false)

	if len(agents) != 1 {
		t.Fatalf("len = %d, want 1", len(agents))
	}
	if agents[0].Name != "codex" {
		t.Errorf("agent = %q, want codex", agents[0].Name)
	}
}

func TestResolveAgents_FilterWithUnknownAgent(t *testing.T) {
	agents := resolveAgents([]string{"nonexistent"}, t.TempDir(), false)

	if len(agents) != 0 {
		t.Errorf("expected empty list for unknown agent filter, got %d", len(agents))
	}
}

func TestResolveAgents_GlobalReturnsAllRegardlessOfDetection(t *testing.T) {
	// Even with an empty project root, global mode returns all agents.
	agents := resolveAgents(nil, t.TempDir(), true)

	if len(agents) != len(skill.Agents()) {
		t.Errorf("global mode: got %d agents, want %d", len(agents), len(skill.Agents()))
	}
}

// ---------------------------------------------------------------------------
// resolveSkills — edge cases
// ---------------------------------------------------------------------------

func TestResolveSkills_EmptyFilter(t *testing.T) {
	// Explicitly empty filter (not nil) should still return all skills non-interactively.
	skills, err := resolveSkills(newSkillCmd(), []string{})
	if err != nil {
		t.Fatalf("resolveSkills error: %v", err)
	}
	if len(skills) != len(skill.AvailableSkills) {
		t.Errorf("len = %d, want %d", len(skills), len(skill.AvailableSkills))
	}
}

func TestResolveSkills_UnknownSkillErrorMessage(t *testing.T) {
	_, err := resolveSkills(newSkillCmd(), []string{"bogus-skill"})
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "bogus-skill") {
		t.Errorf("error should mention the unknown skill name, got: %s", errMsg)
	}
	// Should list available skills.
	for _, name := range skill.Names() {
		if !strings.Contains(errMsg, name) {
			t.Errorf("error should list available skill %q, got: %s", name, errMsg)
		}
	}
}

func TestResolveSkills_DuplicateSkillsInFilter(t *testing.T) {
	// Passing the same skill twice should return it twice (no dedup in filter).
	skills, err := resolveSkills(newSkillCmd(), []string{skillNameKanbanMD, skillNameKanbanMD})
	if err != nil {
		t.Fatalf("resolveSkills error: %v", err)
	}
	if len(skills) != 2 {
		t.Errorf("len = %d, want 2 (duplicates preserved)", len(skills))
	}
}

func TestResolveSkills_VerifiesSkillInfo(t *testing.T) {
	// Verify that resolved skills carry the correct description.
	skills, err := resolveSkills(newSkillCmd(), []string{skillNameKanbanMD})
	if err != nil {
		t.Fatalf("resolveSkills error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("len = %d, want 1", len(skills))
	}
	if skills[0].Description == "" {
		t.Error("resolved skill should have a non-empty description")
	}
}

// ---------------------------------------------------------------------------
// resolveSkills — non-interactive returns all when no filter
// ---------------------------------------------------------------------------

func TestResolveSkills_NonInteractiveAllSkillsMatchRegistry(t *testing.T) {
	skills, err := resolveSkills(newSkillCmd(), nil)
	if err != nil {
		t.Fatalf("resolveSkills error: %v", err)
	}

	// Verify each returned skill matches the registry.
	for i, s := range skills {
		if s.Name != skill.AvailableSkills[i].Name {
			t.Errorf("skill[%d].Name = %q, want %q", i, s.Name, skill.AvailableSkills[i].Name)
		}
		if s.Description != skill.AvailableSkills[i].Description {
			t.Errorf("skill[%d].Description = %q, want %q", i, s.Description, skill.AvailableSkills[i].Description)
		}
	}
}

// ---------------------------------------------------------------------------
// relativePath — additional cases
// ---------------------------------------------------------------------------

func TestRelativePath_NestedSubdirectory(t *testing.T) {
	base := filepath.Join(string(filepath.Separator), "home", "user", "project")
	target := filepath.Join(base, "a", "b", "c", "file.txt")
	got := relativePath(base, target)
	want := filepath.Join("a", "b", "c", "file.txt")
	if got != want {
		t.Errorf("relativePath = %q, want %q", got, want)
	}
}

func TestRelativePath_ParentDirectory(t *testing.T) {
	base := filepath.Join(string(filepath.Separator), "home", "user", "project")
	target := filepath.Join(string(filepath.Separator), "home", "user", "other", "file.txt")
	got := relativePath(base, target)
	want := filepath.Join("..", "other", "file.txt")
	if got != want {
		t.Errorf("relativePath = %q, want %q", got, want)
	}
}

func TestRelativePath_WindowsDifferentDrives(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test for different drives")
	}
	// On Windows, filepath.Rel("C:\\foo", "D:\\bar") returns an error.
	got := relativePath("C:\\foo", "D:\\bar")
	// When Rel fails, the absolute path is returned.
	if got != "D:\\bar" {
		t.Errorf("relativePath = %q, want %q (absolute fallback)", got, "D:\\bar")
	}
}

func TestRelativePath_EmptyRoot(t *testing.T) {
	// Empty root with an absolute target — Rel should still work (relative to ".").
	got := relativePath("", filepath.Join("a", "b"))
	// filepath.Rel("", "a/b") returns "a/b" on Unix.
	if got == "" {
		t.Error("relativePath with empty root should not return empty string")
	}
}

// ---------------------------------------------------------------------------
// findProjectRoot — deeper subdirectory
// ---------------------------------------------------------------------------

func TestFindProjectRoot_DeeplyNested(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0o750); err != nil {
		t.Fatal(err)
	}

	// Create a deeply nested subdirectory.
	deep := filepath.Join(root, "a", "b", "c", "d", "e")
	if err := os.MkdirAll(deep, 0o750); err != nil {
		t.Fatal(err)
	}

	t.Chdir(deep)

	got, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot error: %v", err)
	}
	if got != root {
		t.Errorf("findProjectRoot = %q, want %q", got, root)
	}
}

// ---------------------------------------------------------------------------
// exitCodeError — additional coverage
// ---------------------------------------------------------------------------

func TestExitCodeError_HighCode(t *testing.T) {
	e := &exitCodeError{code: 127}
	got := e.Error()
	if got != "exit code 127" {
		t.Errorf("Error() = %q, want %q", got, "exit code 127")
	}
}

func TestExitCodeError_ImplementsError(t *testing.T) {
	var err error = &exitCodeError{code: 1}
	if err.Error() == "" {
		t.Error("Error() should return non-empty string")
	}
}

// ---------------------------------------------------------------------------
// runSkillInstall — non-interactive integration tests
// ---------------------------------------------------------------------------

func newSkillInstallCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "install"}
	cmd.Flags().StringSlice("agent", nil, "")
	cmd.Flags().StringSlice("skill", nil, "")
	cmd.Flags().Bool("global", false, "")
	cmd.Flags().Bool("force", false, "")
	cmd.Flags().String("path", "", "")
	return cmd
}

func TestRunSkillInstall_PathMode(t *testing.T) {
	dir := t.TempDir()

	r, w := captureStdout(t)
	cmd := newSkillInstallCmd()
	_ = cmd.Flags().Set("path", dir)
	_ = cmd.Flags().Set("skill", skillNameKanbanMD)

	err := runSkillInstall(cmd, nil)
	got := drainPipe(t, r, w)

	if err != nil {
		t.Fatalf("runSkillInstall error: %v", err)
	}

	// Verify the skill was installed.
	skillPath := filepath.Join(dir, skillNameKanbanMD, "SKILL.md")
	if _, statErr := os.Stat(skillPath); statErr != nil {
		t.Errorf("expected skill file at %s, got error: %v", skillPath, statErr)
	}
	_ = got
}

func TestRunSkillInstall_NoSkillsSelected(t *testing.T) {
	r, w := captureStdout(t)
	cmd := newSkillInstallCmd()
	_ = cmd.Flags().Set("skill", "nonexistent")

	err := runSkillInstall(cmd, nil)
	_ = drainPipe(t, r, w)

	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
}

func TestRunSkillInstall_NoAgentsDetected(t *testing.T) {
	// Non-interactive, no agent dirs, no filter → no agents selected.
	projectRoot := t.TempDir()
	gitDir := filepath.Join(projectRoot, ".git")
	if err := os.MkdirAll(gitDir, 0o750); err != nil {
		t.Fatal(err)
	}
	t.Chdir(projectRoot)

	r, w := captureStdout(t)
	cmd := newSkillInstallCmd()
	_ = cmd.Flags().Set("skill", skillNameKanbanMD)

	err := runSkillInstall(cmd, nil)
	got := drainPipe(t, r, w)

	if err != nil {
		t.Fatalf("runSkillInstall error: %v", err)
	}
	if !containsSubstring(got, "No agents selected") {
		t.Errorf("expected 'No agents selected', got: %s", got)
	}
}

func TestRunSkillInstall_WithAgentFlag(t *testing.T) {
	projectRoot := t.TempDir()
	gitDir := filepath.Join(projectRoot, ".git")
	if err := os.MkdirAll(gitDir, 0o750); err != nil {
		t.Fatal(err)
	}
	t.Chdir(projectRoot)

	r, w := captureStdout(t)
	cmd := newSkillInstallCmd()
	_ = cmd.Flags().Set("agent", "claude")
	_ = cmd.Flags().Set("skill", skillNameKanbanMD)

	err := runSkillInstall(cmd, nil)
	got := drainPipe(t, r, w)

	if err != nil {
		t.Fatalf("runSkillInstall error: %v", err)
	}
	if !containsSubstring(got, "Installed") {
		t.Errorf("expected 'Installed' in output, got: %s", got)
	}
}

func TestRunSkillInstall_SkipsUpToDate(t *testing.T) {
	savedVersion := version
	version = testVersion
	t.Cleanup(func() { version = savedVersion })

	projectRoot := t.TempDir()
	gitDir := filepath.Join(projectRoot, ".git")
	if err := os.MkdirAll(gitDir, 0o750); err != nil {
		t.Fatal(err)
	}
	t.Chdir(projectRoot)

	// First install.
	cmd1 := newSkillInstallCmd()
	_ = cmd1.Flags().Set("agent", "claude")
	_ = cmd1.Flags().Set("skill", skillNameKanbanMD)
	r1, w1 := captureStdout(t)
	if err := runSkillInstall(cmd1, nil); err != nil {
		_ = drainPipe(t, r1, w1)
		t.Fatalf("first install error: %v", err)
	}
	_ = drainPipe(t, r1, w1)

	// Second install without force — should skip.
	cmd2 := newSkillInstallCmd()
	_ = cmd2.Flags().Set("agent", "claude")
	_ = cmd2.Flags().Set("skill", skillNameKanbanMD)
	r2, w2 := captureStdout(t)
	err := runSkillInstall(cmd2, nil)
	got := drainPipe(t, r2, w2)

	if err != nil {
		t.Fatalf("second install error: %v", err)
	}
	if !containsSubstring(got, "skipped") || !containsSubstring(got, "up to date") {
		t.Errorf("expected 'skipped' or 'up to date' in output, got: %s", got)
	}
}

func TestRunSkillInstall_ForceOverwrites(t *testing.T) {
	savedVersion := version
	version = testVersion
	t.Cleanup(func() { version = savedVersion })

	projectRoot := t.TempDir()
	gitDir := filepath.Join(projectRoot, ".git")
	if err := os.MkdirAll(gitDir, 0o750); err != nil {
		t.Fatal(err)
	}
	t.Chdir(projectRoot)

	// First install.
	cmd1 := newSkillInstallCmd()
	_ = cmd1.Flags().Set("agent", "claude")
	_ = cmd1.Flags().Set("skill", skillNameKanbanMD)
	r1, w1 := captureStdout(t)
	if err := runSkillInstall(cmd1, nil); err != nil {
		_ = drainPipe(t, r1, w1)
		t.Fatalf("first install error: %v", err)
	}
	_ = drainPipe(t, r1, w1)

	// Second install with force — should reinstall.
	cmd2 := newSkillInstallCmd()
	_ = cmd2.Flags().Set("agent", "claude")
	_ = cmd2.Flags().Set("skill", skillNameKanbanMD)
	_ = cmd2.Flags().Set("force", "true")
	r2, w2 := captureStdout(t)
	err := runSkillInstall(cmd2, nil)
	got := drainPipe(t, r2, w2)

	if err != nil {
		t.Fatalf("force install error: %v", err)
	}
	if !containsSubstring(got, "Installed") {
		t.Errorf("expected 'Installed' in output (force mode), got: %s", got)
	}
}

// ---------------------------------------------------------------------------
// selectModel — edge case: single item
// ---------------------------------------------------------------------------

func TestSelectModel_SingleItem(t *testing.T) {
	m := selectModel{
		prompt: "Pick one:",
		items:  []menuItem{{label: "Only", description: "only item", selected: true}},
	}

	// Navigate down — should stay at 0.
	m = sendTestKey(m, "j")
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (single item)", m.cursor)
	}

	// Navigate up — should stay at 0.
	m = sendTestKey(m, "k")
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (single item)", m.cursor)
	}

	// View should show the item.
	view := m.View()
	if !strings.Contains(view, "Only") {
		t.Errorf("view should contain 'Only', got:\n%s", view)
	}
}

func TestSelectModel_EmptyItems(t *testing.T) {
	m := selectModel{
		prompt: "Empty:",
		items:  nil,
	}

	// Should not panic.
	view := m.View()
	if !strings.Contains(view, "Empty:") {
		t.Errorf("view should contain prompt, got:\n%s", view)
	}

	// Init should still return nil.
	if cmd := m.Init(); cmd != nil {
		t.Errorf("Init() = %v, want nil", cmd)
	}
}

// ---------------------------------------------------------------------------
// selectModel — toggle deselected item then confirm
// ---------------------------------------------------------------------------

func TestSelectModel_ToggleAndConfirm(t *testing.T) {
	m := selectModel{
		prompt: "Select:",
		items: []menuItem{
			{label: "A", selected: false},
			{label: "B", selected: false},
		},
	}

	// Toggle first item on.
	m = sendTestKey(m, " ")
	if !m.items[0].selected {
		t.Error("expected item A to be selected after toggle")
	}

	// Confirm.
	m = sendTestSpecialKey(m, tea.KeyEnter)
	if !m.done {
		t.Error("expected done after enter")
	}

	// Simulate what multiSelect does after confirmation.
	var selected []int
	for i, item := range m.items {
		if item.selected {
			selected = append(selected, i)
		}
	}
	if len(selected) != 1 || selected[0] != 0 {
		t.Errorf("selected = %v, want [0]", selected)
	}
}

// ---------------------------------------------------------------------------
// resolveAgentList — empty slice (not nil)
// ---------------------------------------------------------------------------

func TestResolveAgentList_EmptySlice(t *testing.T) {
	// Empty (non-nil) slice should return all agents — same as nil.
	agents := resolveAgentList([]string{})
	if len(agents) != len(skill.Agents()) {
		t.Errorf("len = %d, want %d", len(agents), len(skill.Agents()))
	}
}

// ---------------------------------------------------------------------------
// resolveAgents — detection with cursor dir
// ---------------------------------------------------------------------------

func TestResolveAgents_NonInteractiveDetectsCursor(t *testing.T) {
	projectRoot := t.TempDir()

	// Create .cursor directory so DetectAgents finds cursor.
	if err := os.MkdirAll(filepath.Join(projectRoot, ".cursor"), 0o750); err != nil {
		t.Fatal(err)
	}

	agents := resolveAgents(nil, projectRoot, false)

	found := false
	for _, a := range agents {
		if a.Name == "cursor" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected cursor in detected agents when .cursor exists")
	}
}

// ---------------------------------------------------------------------------
// runSkillInstall — no project root when not global
// ---------------------------------------------------------------------------

func TestRunSkillInstall_NoProjectRoot(t *testing.T) {
	// When in a directory with no .git and not global, findProjectRoot should
	// still return cwd. The install should still work with an explicit agent.
	dir := t.TempDir()
	t.Chdir(dir)

	r, w := captureStdout(t)
	cmd := newSkillInstallCmd()
	_ = cmd.Flags().Set("agent", "claude")
	_ = cmd.Flags().Set("skill", skillNameKanbanMD)

	err := runSkillInstall(cmd, nil)
	got := drainPipe(t, r, w)

	if err != nil {
		t.Fatalf("runSkillInstall error: %v", err)
	}
	if !containsSubstring(got, "Installed") {
		t.Errorf("expected 'Installed' in output, got: %s", got)
	}
}

// ---------------------------------------------------------------------------
// installToPath — force mode
// ---------------------------------------------------------------------------

func TestInstallToPath_ForceOverwrite(t *testing.T) {
	dir := t.TempDir()

	// First install.
	if err := installToPath(dir, skill.AvailableSkills[:1], false); err != nil {
		t.Fatalf("first install error: %v", err)
	}

	// Second install with force — should reinstall even at same version.
	r, w := captureStdout(t)
	err := installToPath(dir, skill.AvailableSkills[:1], true)
	got := drainPipe(t, r, w)

	if err != nil {
		t.Fatalf("force install error: %v", err)
	}
	// Force mode always installs, so should show the installed path.
	_ = got
}

// ---------------------------------------------------------------------------
// CheckSkillStaleness — multiple skills, only one warning
// ---------------------------------------------------------------------------

func TestCheckSkillStaleness_MultipleSkillsOneWarning(t *testing.T) {
	savedVersion := version
	version = testVersionUpgrade
	t.Cleanup(func() { version = savedVersion })

	projectRoot := t.TempDir()
	claude := skill.AgentByName(agentClaude)
	if claude == nil {
		t.Skip("claude agent not found")
	}
	baseDir := claude.ProjectPath(projectRoot)

	// Install two skills at the old version.
	for _, s := range skill.AvailableSkills {
		if err := skill.Install(s.Name, baseDir, testVersion); err != nil {
			t.Fatalf("Install error: %v", err)
		}
	}

	r, w := captureStderr(t)
	CheckSkillStaleness(projectRoot)
	got := drainPipe(t, r, w)

	// Should only print one warning (early return after first outdated).
	count := strings.Count(got, "outdated")
	if count != 1 {
		t.Errorf("expected exactly 1 'outdated' warning, got %d in: %s", count, got)
	}
}

// ---------------------------------------------------------------------------
// Helper: override isInteractiveFn and multiSelectFn for interactive path tests
// ---------------------------------------------------------------------------

func mockInteractive(t *testing.T, selectIndices []int) {
	t.Helper()
	savedInteractive := isInteractiveFn
	savedMultiSelect := multiSelectFn
	t.Cleanup(func() {
		isInteractiveFn = savedInteractive
		multiSelectFn = savedMultiSelect
	})

	isInteractiveFn = func() bool { return true }
	multiSelectFn = func(_ string, _ []menuItem) []int {
		return selectIndices
	}
}

// ---------------------------------------------------------------------------
// resolveAgents — interactive path: pre-selection logic
// ---------------------------------------------------------------------------

func TestResolveAgents_InteractiveSelectsAll(t *testing.T) {
	// Mock interactive mode, returning all indices.
	allAgents := skill.Agents()
	indices := make([]int, len(allAgents))
	for i := range indices {
		indices[i] = i
	}
	mockInteractive(t, indices)

	agents := resolveAgents(nil, t.TempDir(), false)

	if len(agents) != len(allAgents) {
		t.Errorf("got %d agents, want %d", len(agents), len(allAgents))
	}
}

func TestResolveAgents_InteractiveSelectsNone(t *testing.T) {
	// Mock interactive mode, returning empty selection.
	mockInteractive(t, nil)

	agents := resolveAgents(nil, t.TempDir(), false)

	if len(agents) != 0 {
		t.Errorf("got %d agents, want 0", len(agents))
	}
}

func TestResolveAgents_InteractiveSelectsSubset(t *testing.T) {
	// Mock interactive mode, selecting only the first agent.
	mockInteractive(t, []int{0})

	agents := resolveAgents(nil, t.TempDir(), false)

	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}
	allAgents := skill.Agents()
	if agents[0].Name != allAgents[0].Name {
		t.Errorf("agent = %q, want %q", agents[0].Name, allAgents[0].Name)
	}
}

func TestResolveAgents_InteractiveGlobalMode(t *testing.T) {
	// In global mode, all agents (including global-only) should be pre-selected.
	allAgents := skill.Agents()
	indices := make([]int, len(allAgents))
	for i := range indices {
		indices[i] = i
	}
	mockInteractive(t, indices)

	agents := resolveAgents(nil, t.TempDir(), true)

	if len(agents) != len(allAgents) {
		t.Errorf("global mode: got %d agents, want %d", len(agents), len(allAgents))
	}
}

func TestResolveAgents_InteractiveWithDetectedAgents(t *testing.T) {
	// Create a project with .claude dir so it's detected and pre-selected.
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".claude"), 0o750); err != nil {
		t.Fatal(err)
	}

	// Mock interactive mode — select first two agents.
	mockInteractive(t, []int{0, 1})

	agents := resolveAgents(nil, projectRoot, false)

	if len(agents) != 2 {
		t.Fatalf("got %d agents, want 2", len(agents))
	}
}

func TestResolveAgents_InteractiveFilterBypassesMenu(t *testing.T) {
	// Even in interactive mode, explicit filter bypasses the menu.
	mockInteractive(t, nil) // Would return nil if menu was called.

	agents := resolveAgents([]string{"claude"}, t.TempDir(), false)

	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1 (filter bypasses interactive)", len(agents))
	}
	if agents[0].Name != "claude" {
		t.Errorf("agent = %q, want claude", agents[0].Name)
	}
}

// ---------------------------------------------------------------------------
// resolveSkills — interactive path: menu construction and selection
// ---------------------------------------------------------------------------

func TestResolveSkills_InteractiveSelectsAll(t *testing.T) {
	allSkills := skill.AvailableSkills
	indices := make([]int, len(allSkills))
	for i := range indices {
		indices[i] = i
	}
	mockInteractive(t, indices)

	skills, err := resolveSkills(newSkillCmd(), nil)
	if err != nil {
		t.Fatalf("resolveSkills error: %v", err)
	}

	if len(skills) != len(allSkills) {
		t.Errorf("got %d skills, want %d", len(skills), len(allSkills))
	}
}

func TestResolveSkills_InteractiveSelectsNone(t *testing.T) {
	mockInteractive(t, nil)

	skills, err := resolveSkills(newSkillCmd(), nil)
	if err != nil {
		t.Fatalf("resolveSkills error: %v", err)
	}

	if len(skills) != 0 {
		t.Errorf("got %d skills, want 0", len(skills))
	}
}

func TestResolveSkills_InteractiveSelectsFirst(t *testing.T) {
	mockInteractive(t, []int{0})

	skills, err := resolveSkills(newSkillCmd(), nil)
	if err != nil {
		t.Fatalf("resolveSkills error: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].Name != skill.AvailableSkills[0].Name {
		t.Errorf("skill = %q, want %q", skills[0].Name, skill.AvailableSkills[0].Name)
	}
}

func TestResolveSkills_InteractiveFilterBypassesMenu(t *testing.T) {
	// Even in interactive mode, explicit filter bypasses the menu.
	mockInteractive(t, nil) // Would return nil if menu was called.

	skills, err := resolveSkills(newSkillCmd(), []string{skillNameKanbanMD})
	if err != nil {
		t.Fatalf("resolveSkills error: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1 (filter bypasses interactive)", len(skills))
	}
	if skills[0].Name != skillNameKanbanMD {
		t.Errorf("skill = %q, want kanban-md", skills[0].Name)
	}
}
