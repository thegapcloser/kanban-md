package tui

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/thegapcloser/kanban-md/internal/board"
	"github.com/thegapcloser/kanban-md/internal/config"
	"github.com/thegapcloser/kanban-md/internal/task"
)

// underlineSequence is the SGR code an underlined segment emits. Asserting on
// the sequence instead of on the renderer's own style keeps the test independent
// of the value it is supposed to pin down.
const underlineSequence = "\x1b[4m"

// newRelationTestBoard builds a mouse-enabled board sitting in the detail view
// of task #1. Unlike newMouseTestBoard it sets allTasks as well, so parent
// resolution — including the archived parent of #6 — actually works. Task #8
// points at itself, which no CLI path produces but a hand-written task file
// does.
func newRelationTestBoard() *Board {
	cfg := config.NewDefault("Relation Mouse Test")
	epicID := 1
	childID := 2
	archivedID := 4
	selfID := 8
	danglingID := 999
	all := []*task.Task{
		{ID: 1, Title: "Epic Alpha", Status: dragStatusBacklog, Priority: "critical", Updated: mouseTestTime},
		{ID: 2, Title: "Child One", Status: dragStatusTodo, Priority: "high", Parent: &epicID, Updated: mouseTestTime},
		{ID: 3, Title: "Child Two", Status: "done", Priority: "medium", Parent: &epicID, Updated: mouseTestTime},
		{ID: 4, Title: "Archived Thing", Status: config.ArchivedStatus, Priority: "medium", Updated: mouseTestTime},
		{ID: 5, Title: "Orphan Child", Status: dragStatusTodo, Priority: "low", Parent: &danglingID, Updated: mouseTestTime},
		{ID: 6, Title: "Archived Parent Child", Status: dragStatusTodo, Priority: "low", Parent: &archivedID, Updated: mouseTestTime},
		{ID: 7, Title: "Grandchild", Status: dragStatusTodo, Priority: "medium", Parent: &childID, Updated: mouseTestTime},
		{ID: 8, Title: "Self Parent", Status: dragStatusTodo, Priority: "low", Parent: &selfID, Updated: mouseTestTime},
	}
	var active []*task.Task
	for _, tk := range all {
		if !cfg.IsArchivedStatus(tk.Status) {
			active = append(active, tk)
		}
	}
	b := &Board{
		cfg:             cfg,
		allTasks:        all,
		tasks:           active,
		unfilteredTasks: active,
		columns:         columnsForTasks(cfg.BoardStatuses(), active),
		width:           120,
		height:          40,
		view:            viewDetail,
		detailTask:      all[0],
		now:             func() time.Time { return mouseTestTime.Add(time.Hour) },
		mouseNow:        func() time.Time { return mouseTestTime },
		mouseEnabled:    true,
		sortField:       sortFields[0],
		sortReverse:     true,
	}
	// A struct literal skips loadTasks, so the index has to be built by hand.
	b.rebuildHierarchyIndex()
	_ = b.View()
	return b
}

// taskByID returns a task from the board's active set.
func taskByID(t *testing.T, b *Board, id int) *task.Task {
	t.Helper()
	for _, tk := range b.unfilteredTasks {
		if tk.ID == id {
			return tk
		}
	}
	t.Fatalf("task #%d is not in the active set", id)
	return nil
}

// relationTargetFor returns the recorded hit target of a relation row.
func relationTargetFor(t *testing.T, b *Board, id int) relationTarget {
	t.Helper()
	for _, target := range b.layout.relations {
		if target.taskID == id {
			return target
		}
	}
	t.Fatalf("relation row for #%d has no hit target (targets=%v)", id, b.layout.relations)
	return relationTarget{}
}

// clickAt sends a press/release pair at one cell.
func clickAt(b *Board, x, y int, releaseButton tea.MouseButton) {
	_, _ = b.Update(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	_, _ = b.Update(tea.MouseMsg{X: x, Y: y, Button: releaseButton, Action: tea.MouseActionRelease})
}

func TestRelationLayoutCoversOnlyNavigableRows(t *testing.T) {
	b := newRelationTestBoard()

	var got []int
	for _, target := range b.layout.relations {
		got = append(got, target.taskID)
	}
	if len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("relation targets of #1 = %v, want [2 3]", got)
	}

	b.detailTask = taskByID(t, b, 5)
	b.invalidatePointerState()
	_ = b.View()
	if len(b.layout.relations) != 0 {
		t.Errorf("a dangling parent produced %d hit targets, want none", len(b.layout.relations))
	}
}

func TestRelationHitTestHonorsScrollOffset(t *testing.T) {
	b := newRelationTestBoard()
	b.detailTask.Body = strings.Repeat("body line\n", 60)
	b.invalidatePointerState()
	_ = b.View()
	before := relationTargetFor(t, b, 2).rect.y0

	b.detailScrollOff = 3
	b.invalidatePointerState()
	_ = b.View()
	after := relationTargetFor(t, b, 2).rect.y0

	if after != before-3 {
		t.Errorf("relation target y0 = %d at offset 3, want %d", after, before-3)
	}
}

func TestRelationClickOpensInOneClick(t *testing.T) {
	for _, tt := range []struct {
		name          string
		releaseButton tea.MouseButton
	}{
		{name: "SGR left-button release", releaseButton: tea.MouseButtonLeft},
		{name: "X10 buttonless release", releaseButton: tea.MouseButtonNone},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := newRelationTestBoard()
			target := relationTargetFor(t, b, 2)
			clickAt(b, target.rect.x0, target.rect.y0, tt.releaseButton)

			if b.detailTask == nil || b.detailTask.ID != 2 {
				t.Fatalf("detail task = %#v, want #2", b.detailTask)
			}
			if len(b.detailStack) != 1 {
				t.Errorf("navigation stack has %d frames, want 1", len(b.detailStack))
			}
		})
	}
}

func TestRelationClickOutsideDoesNothing(t *testing.T) {
	b := newRelationTestBoard()
	clickAt(b, 4, 3, tea.MouseButtonLeft) // the "Status:" row

	if b.detailTask.ID != 1 {
		t.Errorf("clicking a non-relation row opened #%d", b.detailTask.ID)
	}
	if len(b.detailStack) != 0 {
		t.Errorf("clicking a non-relation row pushed %d frames", len(b.detailStack))
	}
}

func TestRelationPressOutsideBlocksReleaseOverRelation(t *testing.T) {
	b := newRelationTestBoard()
	target := relationTargetFor(t, b, 2)

	// Press on the "Status:" row, release over a relation row. Only the
	// press-side hit test can refuse this: the release lands inside the rect the
	// press recorded, so the release-side check would let it through.
	_, _ = b.Update(tea.MouseMsg{
		X: 4, Y: 3,
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	})
	_, _ = b.Update(tea.MouseMsg{
		X: target.rect.x0, Y: target.rect.y0,
		Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease,
	})

	if b.detailTask.ID != 1 {
		t.Errorf("a press outside every relation opened #%d", b.detailTask.ID)
	}
	if len(b.detailStack) != 0 {
		t.Errorf("a press outside every relation pushed %d frames", len(b.detailStack))
	}
}

func TestRelationBackButtonPopsOneLevel(t *testing.T) {
	b := newRelationTestBoard()
	target := relationTargetFor(t, b, 2)
	clickAt(b, target.rect.x0, target.rect.y0, tea.MouseButtonLeft)
	_ = b.View()

	if b.layout.back == nil {
		t.Fatal("detail view has no Back target")
	}
	back := b.layout.back.rect
	clickAt(b, back.x0, back.y0, tea.MouseButtonNone)
	if b.detailTask == nil || b.detailTask.ID != 1 {
		t.Fatalf("Back did not return to #1: %#v", b.detailTask)
	}
	if len(b.detailStack) != 0 {
		t.Errorf("navigation stack still holds %d frames after Back", len(b.detailStack))
	}

	_ = b.View()
	back = b.layout.back.rect
	clickAt(b, back.x0, back.y0, tea.MouseButtonNone)
	if b.view != viewBoard {
		t.Errorf("Back on an empty stack left view = %v, want the board", b.view)
	}
}

// withANSIProfile switches lipgloss to a profile that actually emits style
// sequences, so styled output can be compared against style.Render.
func withANSIProfile(t *testing.T) {
	t.Helper()
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
}

// hoverAt sends a buttonless motion event, the way a terminal reports a plain
// pointer move under all-motion reporting.
func hoverAt(b *Board, x, y int) {
	_, _ = b.Update(tea.MouseMsg{
		X: x, Y: y, Action: tea.MouseActionMotion, Button: tea.MouseButtonNone,
	})
}

// underlinedRows returns the indices of the rendered rows carrying an underline
// sequence. Lip Gloss emits the sequence per rune, so the row index — not the
// row text — is what a hover assertion can hold on to.
func underlinedRows(v string) []int {
	var rows []int
	for i, line := range strings.Split(v, "\n") {
		if strings.Contains(line, underlineSequence) {
			rows = append(rows, i)
		}
	}
	return rows
}

// cursorLine returns the rendered row carrying the cursor gutter, escape
// sequences removed, or the empty string when no row has it.
func cursorLine(v string) string {
	for _, line := range strings.Split(plainLine(v), "\n") {
		if strings.HasPrefix(line, relationCursorGutter) {
			return line
		}
	}
	return ""
}

func TestRelationHoverUnderlinesRowUnderPointer(t *testing.T) {
	withANSIProfile(t)
	b := newRelationTestBoard()
	target := relationTargetFor(t, b, 2)

	if rows := underlinedRows(b.View()); len(rows) != 0 {
		t.Fatalf("rows %v are underlined before any pointer moved", rows)
	}

	hoverAt(b, 4, target.rect.y0)

	got := underlinedRows(b.View())
	if len(got) != 1 || got[0] != target.rect.y0 {
		t.Errorf("underlined rows = %v, want exactly row %d — the hit rect of #2",
			got, target.rect.y0)
	}
}

func TestRelationHoverIgnoresNonNavigableRow(t *testing.T) {
	withANSIProfile(t)
	b := newRelationTestBoard()
	b.detailTask = taskByID(t, b, 6)
	b.invalidatePointerState()
	_ = b.View()

	// The parent row of #6 sits directly below the blank line after the
	// timestamps; hover every body row to be certain none of them underlines.
	for y := range 12 {
		hoverAt(b, 4, y)
		if rows := underlinedRows(b.View()); len(rows) != 0 {
			t.Fatalf("hover at y=%d underlined rows %v of a non-navigable relation:\n%q",
				y, rows, b.View())
		}
	}
}

func TestRelationSelfParentIsInertInEveryPath(t *testing.T) {
	withANSIProfile(t)
	b := newRelationTestBoard()
	b.detailTask = taskByID(t, b, 8)
	b.invalidatePointerState()

	// A task pointing at itself resolves to no parent, so the chain ends without
	// a row — which leaves a one-row tree, and that is not shown at all. There
	// is nothing to click, nothing to tab to and nothing to hover.
	if strings.Contains(plainLine(b.View()), hierarchyHeading) {
		t.Fatalf("a self-referencing parent produced a hierarchy block:\n%q", b.View())
	}
	if len(b.layout.relations) != 0 {
		t.Errorf("a self-referencing parent produced %d click targets, want none",
			len(b.layout.relations))
	}

	b.moveDetailCursor(1)
	if strings.Contains(b.View(), relationCursorGutter) {
		t.Errorf("tab put the cursor on a row of a self-referencing task:\n%q", b.View())
	}

	for y := range 12 {
		hoverAt(b, 4, y)
		if rows := underlinedRows(b.View()); len(rows) != 0 {
			t.Fatalf("hover at y=%d underlined rows %v of a self-referencing parent:\n%q",
				y, rows, b.View())
		}
	}
}

func TestModifierMotionClearsHover(t *testing.T) {
	withANSIProfile(t)
	b := newRelationTestBoard()
	target := relationTargetFor(t, b, 2)

	hoverAt(b, 4, target.rect.y0)
	if !strings.Contains(b.View(), underlineSequence) {
		t.Fatalf("hover did not underline the row under the pointer:\n%q", b.View())
	}

	_, _ = b.Update(tea.MouseMsg{
		X: 4, Y: target.rect.y0 + 1,
		Action: tea.MouseActionMotion, Button: tea.MouseButtonNone, Shift: true,
	})

	if b.pointer.hoverActive {
		t.Error("a move with a held modifier left the hover armed")
	}
	if strings.Contains(b.View(), underlineSequence) {
		t.Errorf("the underline froze on the row the pointer left:\n%q", b.View())
	}
}

func TestBoardHoverMotionLeavesDragTargetAlone(t *testing.T) {
	b := newMouseTestBoard()
	source := targetForTask(t, b, 1)
	destination := columnTargetForStatus(t, b, dragStatusTodo)

	_, _ = b.Update(tea.MouseMsg{
		X: source.rect.x0 + 1, Y: source.rect.y0,
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	})
	if !b.pointer.pressed || b.pointer.kind != pointerTargetCard {
		t.Fatalf("press did not arm a card gesture: %+v", b.pointer)
	}

	_, _ = b.Update(tea.MouseMsg{
		X: destination.rect.x0 + 1, Y: destination.rect.y0,
		Action: tea.MouseActionMotion, Button: tea.MouseButtonNone,
	})

	if b.pointer.dragStarted {
		t.Error("a buttonless move started a drag")
	}
	if b.pointer.destination != -1 || b.pointer.destinationStatus != "" {
		t.Errorf("a buttonless move set the drop target to (%d, %q)",
			b.pointer.destination, b.pointer.destinationStatus)
	}
}

func TestRelationHoverClearedByKeyAndViewChange(t *testing.T) {
	b := newRelationTestBoard()
	target := relationTargetFor(t, b, 2)

	hoverAt(b, 4, target.rect.y0)
	if !b.pointer.hoverActive {
		t.Fatal("buttonless motion did not record a hover")
	}
	_, _ = b.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if b.pointer.hoverActive {
		t.Error("hover survived a key press")
	}

	_ = b.View()
	target = relationTargetFor(t, b, 2)
	hoverAt(b, 4, target.rect.y0)
	if !b.pointer.hoverActive {
		t.Fatal("buttonless motion did not record a hover")
	}
	_, _ = b.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if b.pointer.hoverActive {
		t.Error("hover survived a resize")
	}
}

func TestRelationHoverAndCursorCoexist(t *testing.T) {
	withANSIProfile(t)
	b := newRelationTestBoard()
	// The cursor is set through Update, the way a key press arrives. Only this
	// order can be asserted: the key path calls invalidatePointerState, so a
	// cursor key after a hover deletes the hover — pinned down by
	// TestRelationHoverClearedByKeyAndViewChange.
	_, _ = b.Update(tea.KeyMsg{Type: tea.KeyTab})
	_ = b.View()
	hoverTarget := relationTargetFor(t, b, 3)
	hoverAt(b, 4, hoverTarget.rect.y0)

	v := b.View()
	if got := cursorLine(v); !strings.Contains(got, "├─ #2 [todo] Child One") {
		t.Errorf("keyboard cursor row is %q, want it on #2 while another row is hovered:\n%q",
			got, v)
	}
	got := underlinedRows(v)
	if len(got) != 1 || got[0] != hoverTarget.rect.y0 {
		t.Errorf("underlined rows = %v, want exactly row %d — the hit rect of #3",
			got, hoverTarget.rect.y0)
	}
}

func TestHoverMotionDoesNotTouchGesture(t *testing.T) {
	b := newRelationTestBoard()
	target := relationTargetFor(t, b, 2)
	_, _ = b.Update(tea.MouseMsg{
		X: target.rect.x0, Y: target.rect.y0,
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	})
	if !b.pointer.pressed || b.pointer.kind != pointerTargetRelation {
		t.Fatalf("press did not arm a relation gesture: %+v", b.pointer)
	}

	hoverAt(b, 100, 30)

	if !b.pointer.pressed {
		t.Error("buttonless motion canceled an armed gesture")
	}
}

func TestHoverMotionKeepsErrorToast(t *testing.T) {
	b := newRelationTestBoard()
	b.err = errors.New("existing error")
	_ = b.View()

	hoverAt(b, 4, 5)
	if b.err == nil {
		t.Error("a buttonless motion event cleared the error toast")
	}

	// Counter-check: a motion event with a held button still clears it.
	b.err = errors.New("existing error")
	_, _ = b.Update(tea.MouseMsg{
		X: 4, Y: 5, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft,
	})
	if b.err != nil {
		t.Error("a button-held motion event no longer clears the error toast")
	}
}

func TestTaskBodyMemoReturnsSameOutputAsUncached(t *testing.T) {
	b := newRelationTestBoard()
	dark := lipgloss.HasDarkBackground()

	for _, body := range []string{"plain body", "# Heading\n\nwith a paragraph", "- one\n- two"} {
		for _, width := range []int{40, 80} {
			want := renderMarkdownForBackground(body, width, dark)
			for attempt := range 2 {
				if got := b.renderTaskBody(body, width); got != want {
					t.Fatalf("attempt %d: memo output for %q at width %d differs:\ngot:  %q\nwant: %q",
						attempt+1, body, width, got, want)
				}
			}
		}
	}
}

func TestTaskBodyMemoServesTheSecondCallFromCache(t *testing.T) {
	b := newRelationTestBoard()
	const body = "cached body"
	_ = b.renderTaskBody(body, 80)

	// Tamper with the stored value: only a cache hit can return it verbatim, so
	// this separates the memo from a second render that happens to agree.
	b.mdCache.rendered = "SENTINEL"

	if got := b.renderTaskBody(body, 80); got != "SENTINEL" {
		t.Errorf("second call for the same body re-rendered instead of hitting the memo: %q", got)
	}
}

func TestTaskBodyMemoPicksUpChangedBody(t *testing.T) {
	b := newRelationTestBoard()
	_ = b.renderTaskBody("first", 80)

	got := b.renderTaskBody("second", 80)
	if !strings.Contains(got, "second") {
		t.Errorf("memo returned %q for a changed body, want it to contain %q", got, "second")
	}
	if strings.Contains(got, "first") {
		t.Errorf("memo returned the previous body: %q", got)
	}
}

func TestRelationVisibleMatchesUnfilteredTaskSet(t *testing.T) {
	// relationVisible decides which rows can be opened. Resolving it through the
	// hierarchy index must answer for exactly the tasks unfilteredTasks holds —
	// no more (archived ones) and no less (filtered-out ones).
	b := newRelationTestBoard()

	inUnfiltered := make(map[int]bool, len(b.unfilteredTasks))
	for _, tk := range b.unfilteredTasks {
		inUnfiltered[tk.ID] = true
	}
	for _, tk := range b.allTasks {
		if got := b.relationVisible(tk.ID); got != inUnfiltered[tk.ID] {
			t.Errorf("relationVisible(%d) = %t, want %t (status %q)",
				tk.ID, got, inUnfiltered[tk.ID], tk.Status)
		}
	}
	for _, unknown := range []int{0, 999} {
		if b.relationVisible(unknown) {
			t.Errorf("relationVisible(%d) = true, want false for an unknown task", unknown)
		}
	}
}

func TestRebuildHierarchyIndexIsTheOnlyDepthSource(t *testing.T) {
	b := newRelationTestBoard()

	b.rebuildHierarchyIndex()

	if got, want := b.taskDepths, board.Depths(b.allTasks); !reflect.DeepEqual(got, want) {
		t.Errorf("taskDepths = %v, want %v", got, want)
	}
	if b.hierarchyIndex == nil {
		t.Fatal("hierarchyIndex = nil after rebuildHierarchyIndex")
	}
	if b.hierarchyIndex.Task(4) == nil {
		t.Error("index does not resolve the archived task #4, which parent rows need")
	}
}
