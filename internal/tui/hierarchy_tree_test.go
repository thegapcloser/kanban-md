package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/antopolskiy/kanban-md/internal/board"
	"github.com/antopolskiy/kanban-md/internal/config"
	"github.com/antopolskiy/kanban-md/internal/task"
)

// SGR parameters the three row states are proven by. Asserting on the sequence
// and not on the style variable keeps the test independent of the value it pins.
const (
	sgrUnderline = "4"
	sgrBold      = "1"
	sgrDim       = "38;5;241"
	sgrTerminal  = "38;5;42"
)

// e2eChildRow is the screen row the child of the open task lands on in the
// E2E fixture shape. e2e/tui_mouse_navigation_test.go clicks exactly there, and
// this fast unit test is what reports a new number when the geometry moves.
const e2eChildRow = 11

// Status and priority names used by the tree fixtures.
const (
	treeStatusDone      = "done"
	treePriorityHigh    = "critical"
	treePriorityDefault = "medium"
)

func idPtr(i int) *int { return &i }

// plainLine strips the escape sequences from a rendered line. The external test
// package has its own helper; this one keeps hierarchy assertions inside the
// package that owns the renderer.
func plainLine(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		if strings.HasPrefix(s[i:], "\x1b[") {
			end := strings.IndexByte(s[i:], 'm')
			if end < 0 {
				break
			}
			i += end + 1
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

// hierarchyTestBoard builds a mouse-enabled board sitting in the detail view of
// one task, with a fixed level budget. A struct literal skips loadTasks, so the
// index is built by hand.
func hierarchyTestBoard(tasks []*task.Task, detailID, levels, width, height int) *Board {
	cfg := config.NewDefault("Hierarchy Test")
	cfg.TUI.HierarchyLevels = &levels
	var active []*task.Task
	for _, tk := range tasks {
		if !cfg.IsArchivedStatus(tk.Status) {
			active = append(active, tk)
		}
	}
	b := &Board{
		cfg:             cfg,
		allTasks:        tasks,
		tasks:           active,
		unfilteredTasks: active,
		columns:         columnsForTasks(cfg.BoardStatuses(), active),
		width:           width,
		height:          height,
		view:            viewDetail,
		now:             func() time.Time { return mouseTestTime.Add(time.Hour) },
		mouseNow:        func() time.Time { return mouseTestTime },
		mouseEnabled:    true,
		sortField:       sortFields[0],
		sortReverse:     true,
	}
	b.rebuildHierarchyIndex()
	b.detailTask = b.hierarchyIndex.Task(detailID)
	_ = b.View()
	return b
}

// hierarchyFixtureTasks is the three-level shape the tree tests share: #1 with
// two children of which one is done, #2 with two done children, #3 with none.
func hierarchyFixtureTasks() []*task.Task {
	return []*task.Task{
		{ID: 1, Title: "Root One", Status: dragStatusBacklog, Priority: treePriorityHigh, Updated: mouseTestTime},
		{ID: 2, Title: "Mid Two", Status: dragStatusTodo, Priority: "high", Parent: idPtr(1), Updated: mouseTestTime},
		{ID: 3, Title: "Mid Three", Status: treeStatusDone, Priority: treePriorityDefault, Parent: idPtr(1), Updated: mouseTestTime},
		{ID: 4, Title: "Leaf Four", Status: treeStatusDone, Priority: "low", Parent: idPtr(2), Updated: mouseTestTime},
		{ID: 5, Title: "Leaf Five", Status: treeStatusDone, Priority: "low", Parent: idPtr(2), Updated: mouseTestTime},
	}
}

// archivedAncestorTasks puts the open task under an archived parent, which the
// tree shows dimmed as the last row of the chain.
func archivedAncestorTasks() []*task.Task {
	return []*task.Task{
		{ID: 1, Title: "Archived Root", Status: config.ArchivedStatus, Priority: treePriorityDefault, Updated: mouseTestTime},
		{ID: 2, Title: "Open Ticket", Status: dragStatusTodo, Priority: "high", Parent: idPtr(1), Updated: mouseTestTime},
	}
}

// statusPaletteTasks hangs one child per status under the open task, so one
// board renders a terminal status next to three non-terminal ones.
func statusPaletteTasks() []*task.Task {
	return []*task.Task{
		{ID: 1, Title: "Palette Root", Status: dragStatusTodo, Priority: treePriorityHigh, Updated: mouseTestTime},
		{ID: 2, Title: "Working Two", Status: hierarchyStatusInProgress, Priority: "high", Parent: idPtr(1), Updated: mouseTestTime},
		{ID: 3, Title: "Finished Three", Status: treeStatusDone, Priority: treePriorityDefault, Parent: idPtr(1), Updated: mouseTestTime},
		{ID: 4, Title: "Reviewed Four", Status: "review", Priority: "low", Parent: idPtr(1), Updated: mouseTestTime},
	}
}

// terminalStatusWithHiddenChildTasks puts a terminal status, a counter and a
// hover on one row: #2 is done, and its own child stays outside a one-level
// tree, so the row keeps its counter.
func terminalStatusWithHiddenChildTasks() []*task.Task {
	return []*task.Task{
		{ID: 1, Title: "Counted Root", Status: dragStatusTodo, Priority: treePriorityHigh, Updated: mouseTestTime},
		{ID: 2, Title: "Done Mid", Status: treeStatusDone, Priority: "high", Parent: idPtr(1), Updated: mouseTestTime},
		{ID: 3, Title: "Hidden Leaf", Status: dragStatusTodo, Priority: "low", Parent: idPtr(2), Updated: mouseTestTime},
	}
}

// treeLines returns the plain content lines of the tree block, without the
// heading and without the gutter the renderer adds.
func treeLines(b *Board) []string {
	c := b.detailContent(b.detailTask)
	var out []string
	for _, ref := range c.relations {
		for i := range ref.lineCount {
			out = append(out, c.lines[ref.startLine+i])
		}
	}
	return out
}

// rowLineFor returns the rendered screen line of a relation row's first line.
func rowLineFor(t *testing.T, b *Board, id int) string {
	t.Helper()
	c := b.detailContent(b.detailTask)
	for _, ref := range c.relations {
		if ref.taskID != id {
			continue
		}
		lines := strings.Split(b.View(), "\n")
		if ref.startLine >= len(lines) {
			t.Fatalf("row of #%d is at line %d, beyond the %d rendered lines", id, ref.startLine, len(lines))
		}
		return lines[ref.startLine]
	}
	t.Fatalf("no relation row for #%d", id)
	return ""
}

// textWithSGR walks a rendered line and returns the plain text that was emitted
// while the given SGR parameter was active. It tolerates styles emitted per
// rune, so the assertion holds regardless of how the renderer batches escapes.
func textWithSGR(line, param string) string {
	var out strings.Builder
	active := map[string]bool{}
	for i := 0; i < len(line); {
		if strings.HasPrefix(line[i:], "\x1b[") {
			end := strings.IndexByte(line[i:], 'm')
			if end < 0 {
				break
			}
			applySGR(active, line[i+2:i+end])
			i += end + 1
			continue
		}
		if active[param] {
			out.WriteByte(line[i])
		}
		i++
	}
	return out.String()
}

// applySGR folds one escape sequence body into the set of active parameters.
// Extended color parameters span several tokens and stay one entry, so "4" from
// "38;5;42;4" is still recognized as the underline it is.
func applySGR(active map[string]bool, body string) {
	tokens := strings.Split(body, ";")
	for i := 0; i < len(tokens); i++ {
		switch {
		case tokens[i] == "0" || tokens[i] == "":
			for k := range active {
				delete(active, k)
			}
		case (tokens[i] == "38" || tokens[i] == "48") && i+2 < len(tokens) && tokens[i+1] == "5":
			active[strings.Join(tokens[i:i+3], ";")] = true
			i += 2
		case (tokens[i] == "38" || tokens[i] == "48") && i+4 < len(tokens) && tokens[i+1] == "2":
			active[strings.Join(tokens[i:i+5], ";")] = true
			i += 4
		default:
			active[tokens[i]] = true
		}
	}
}

// --- T6: structure, wrapping, capping, counter position, ellipsis ---

func TestHierarchyBranchPrefixes(t *testing.T) {
	const wide = 120
	tests := []struct {
		name        string
		lastAtDepth []bool
		depth       int
		width       int
		last        bool
		want        string
	}{
		{name: "root, last", depth: 0, width: wide, last: true, want: "└─ "},
		{name: "root, more siblings", depth: 0, width: wide, want: "├─ "},
		{
			name: "under a last parent", lastAtDepth: []bool{true}, depth: 1, width: wide,
			want: "   ├─ ",
		},
		{
			name: "under a parent with siblings", lastAtDepth: []bool{false}, depth: 1, width: wide,
			want: "│  ├─ ",
		},
		{
			name: "two continuations", lastAtDepth: []bool{false, false}, depth: 2, width: wide,
			last: true, want: "│  │  └─ ",
		},
		{
			name: "mixed continuations", lastAtDepth: []bool{true, false}, depth: 2, width: wide,
			last: true, want: "   │  └─ ",
		},
		{
			name:  "indentation stops growing at width 30",
			depth: 3, width: 30, last: true, lastAtDepth: []bool{false, false, false},
			want: "│  └─ ",
		},
		{
			name:  "no indentation left at width 10",
			depth: 5, width: 10, last: true, lastAtDepth: []bool{false, false, false, false, false},
			want: "└─ ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := hierarchyBranch(tc.lastAtDepth, tc.depth, tc.width, tc.last)
			if got != tc.want {
				t.Errorf("hierarchyBranch() = %q, want %q", got, tc.want)
			}
			if want := hierarchyPrefixWidth(tc.depth, tc.width); lipgloss.Width(got) != want {
				t.Errorf("prefix width = %d, want %d from hierarchyPrefixWidth", lipgloss.Width(got), want)
			}
		})
	}
}

func TestHierarchyPrefixWidthStopsGrowing(t *testing.T) {
	for _, width := range []int{120, 40, 30, 10, 3} {
		for depth := range 7 {
			prefix := hierarchyPrefixWidth(depth, width)
			text := hierarchyTextWidth(depth, width)

			if text < 1 {
				t.Errorf("width %d depth %d: text width = %d, want at least 1", width, depth, text)
			}
			if prefix > hierarchyBranchWidth && text < minHierarchyTextWidth {
				t.Errorf("width %d depth %d: indented to %d cells but left only %d for text, want at least %d",
					width, depth, prefix, text, minHierarchyTextWidth)
			}
			if depth > 0 && prefix < hierarchyPrefixWidth(depth-1, width) {
				t.Errorf("width %d: prefix shrank from depth %d to %d", width, depth-1, depth)
			}
		}
	}
}

func TestHierarchyRowsWrapWithCountOnLastLine(t *testing.T) {
	row := board.HierarchyRow{
		ID: 1, Status: dragStatusBacklog, Done: 1, Total: 2,
		Title: "Alpha Beta Gamma Delta Epsilon Zeta Eta",
	}
	prefix := hierarchyBranch(nil, 0, 40, true)

	lines := hierarchyRowLines(row, prefix, 40)

	if len(lines) != 2 {
		t.Fatalf("row wrapped into %d lines, want 2: %+v", len(lines), lines)
	}
	if lines[1].text() == "" || lines[1].count == "" {
		t.Errorf("counter does not sit at the end of the last text line: %+v", lines)
	}
	if lines[0].count != "" {
		t.Errorf("counter appears on the first line as well: %+v", lines)
	}
	if lines[1].prefix != "   " {
		t.Errorf("continuation prefix = %q, want three cells of indentation", lines[1].prefix)
	}

	narrow := board.HierarchyRow{
		ID: 1, Status: dragStatusBacklog, Done: 1, Total: 2,
		Title: "Alpha Beta Gamma Delta Epsilonzz",
	}

	lines = hierarchyRowLines(narrow, prefix, 30)

	last := lines[len(lines)-1]
	if last.count == "" || last.text() != "" {
		t.Errorf("counter was not moved onto a continuation line of its own: %+v", lines)
	}
	if last.prefix != "   " {
		t.Errorf("counter line prefix = %q, want continuation indentation", last.prefix)
	}
	for i, l := range lines {
		if l.text() == "" && l.count == "" {
			t.Errorf("line %d of %+v is empty", i, lines)
		}
	}
}

func TestHierarchyDegenerateWidthsDoNotPanicOrEmpty(t *testing.T) {
	for _, width := range []int{10, 3} {
		b := hierarchyTestBoard(hierarchyFixtureTasks(), 1, 3, width, 40)

		view := b.View()
		if view == "" {
			t.Fatalf("width %d rendered nothing", width)
		}
		lines := treeLines(b)
		if len(lines) == 0 {
			t.Fatalf("width %d rendered no tree rows", width)
		}
		if len(lines) > 100 {
			t.Fatalf("width %d rendered %d tree rows, want a finite tree", width, len(lines))
		}
		for i, line := range lines {
			if strings.TrimSpace(plainLine(line)) == "" {
				t.Errorf("width %d: tree line %d is blank: %q", width, i, line)
			}
		}
	}
}

// --- T7: decoration as a segment rule ---

func TestHierarchyUnderlineStartsAtTheHash(t *testing.T) {
	withANSIProfile(t)
	b := hierarchyTestBoard(hierarchyFixtureTasks(), 1, 1, 120, 40)
	target := relationTargetFor(t, b, 2)

	hoverAt(b, 4, target.rect.y0)
	line := rowLineFor(t, b, 2)

	prefix := relationGutter + hierarchyBranch([]bool{true}, 1, 120, false)
	if got := strings.Index(line, "\x1b["); got != len(prefix) {
		t.Errorf("first escape sequence at byte %d, want %d — the end of %q. Line: %q",
			got, len(prefix), prefix, line)
	}
	if got := underlinedRows(b.View()); len(got) != 1 || got[0] != target.rect.y0 {
		t.Errorf("underlined rows = %v, want exactly row %d", got, target.rect.y0)
	}
}

func TestHierarchyUnderlineCoversWholeTextIncludingCount(t *testing.T) {
	withANSIProfile(t)
	b := hierarchyTestBoard(hierarchyFixtureTasks(), 1, 1, 120, 40)
	target := relationTargetFor(t, b, 2)

	hoverAt(b, 4, target.rect.y0)
	line := rowLineFor(t, b, 2)

	want := "#2 [todo] Mid Two (2/2 done)"
	if got := textWithSGR(line, sgrUnderline); got != want {
		t.Errorf("underlined text = %q, want %q", got, want)
	}
	if got := textWithSGR(line, sgrTerminal); got != "" {
		t.Errorf("green text on a row whose status is not terminal = %q, want none: %q", got, line)
	}
}

func TestHierarchyUnderlineAndGreenBracketCoexist(t *testing.T) {
	// The bracket keeps its color while the underline stays active over it, and
	// the counter after it keeps neither a color of its own nor loses the
	// underline. Nesting would end one of the two at the other's reset.
	withANSIProfile(t)
	b := hierarchyTestBoard(terminalStatusWithHiddenChildTasks(), 1, 1, 120, 40)
	target := relationTargetFor(t, b, 2)

	hoverAt(b, 4, target.rect.y0)
	line := rowLineFor(t, b, 2)

	want := "#2 [done] Done Mid (0/1 done)"
	if got := textWithSGR(line, sgrUnderline); got != want {
		t.Errorf("underlined text = %q, want %q", got, want)
	}
	if got := textWithSGR(line, sgrTerminal); got != "[done]" {
		t.Errorf("green text inside the underlined row = %q, want the status bracket alone: %q", got, line)
	}
}

func TestHierarchyTerminalStatusBracketIsGreen(t *testing.T) {
	withANSIProfile(t)
	b := hierarchyTestBoard(statusPaletteTasks(), 1, 1, 120, 40)

	terminal := rowLineFor(t, b, 3) // [done]
	if got := textWithSGR(terminal, sgrTerminal); got != "["+treeStatusDone+"]" {
		t.Errorf("green text of the terminal row = %q, want the status bracket alone: %q", got, terminal)
	}

	// Every other status on the board is not terminal and gets no color at all.
	for _, id := range []int{1, 2, 4} {
		line := rowLineFor(t, b, id)
		if strings.Contains(line, "\x1b["+sgrTerminal+"m") {
			t.Errorf("row of #%d has a non-terminal status and is rendered green: %q", id, line)
		}
	}

	// On a row that carries a counter too, the color stops at the bracket: the
	// counter inherits dim and bold from its row and nothing else.
	counted := rowLineFor(t, hierarchyTestBoard(terminalStatusWithHiddenChildTasks(), 1, 1, 120, 40), 2)
	if !strings.Contains(plainLine(counted), "(0/1 done)") {
		t.Fatalf("row of #2 = %q, want it to carry a counter", plainLine(counted))
	}
	if got := textWithSGR(counted, sgrTerminal); got != "["+treeStatusDone+"]" {
		t.Errorf("green text of a counted terminal row = %q, want the bracket alone: %q", got, counted)
	}
}

func TestHierarchyOpenTicketIsBoldAndGreenAtOnce(t *testing.T) {
	// bold and green apply to the same row: the open ticket in a terminal
	// status is both, and the bracket is the segment that carries the color.
	withANSIProfile(t)
	b := hierarchyTestBoard(terminalStatusWithHiddenChildTasks(), 2, 1, 120, 40)

	open := rowLineFor(t, b, 2)

	if got := textWithSGR(open, sgrBold); got != "#2 [done] Done Mid" {
		t.Errorf("bold text of the open ticket = %q, want its whole row text: %q", got, open)
	}
	if got := textWithSGR(open, sgrTerminal); got != "["+treeStatusDone+"]" {
		t.Errorf("green text of the open ticket = %q, want the status bracket: %q", got, open)
	}
}

func TestHierarchyOpenTicketIsBoldAndNotDimmed(t *testing.T) {
	withANSIProfile(t)
	b := hierarchyTestBoard(hierarchyFixtureTasks(), 1, 1, 120, 40)

	open := rowLineFor(t, b, 1)
	if got := textWithSGR(open, sgrBold); !strings.HasPrefix(got, "#1 ") {
		t.Errorf("bold text of the open ticket = %q, want its row text: %q", got, open)
	}
	if textWithSGR(open, sgrDim) != "" {
		t.Errorf("the open ticket is dimmed as well as bold: %q", open)
	}

	archived := hierarchyTestBoard(archivedAncestorTasks(), 2, 1, 120, 40)
	ancestor := rowLineFor(t, archived, 1)
	if got := textWithSGR(ancestor, sgrDim); !strings.HasPrefix(got, "#1 ") {
		t.Errorf("dimmed text of the archived ancestor = %q, want its row text: %q", got, ancestor)
	}
	if textWithSGR(ancestor, sgrBold) != "" {
		t.Errorf("the archived ancestor is rendered bold: %q", ancestor)
	}

	// The ellipsis row is the one dimmed row without an ID.
	cut := hierarchyTestBoard(hierarchyFixtureTasks(), 1, 1, 120, 40)
	ellipsis := ellipsisRow(t, cut.View())
	if got := textWithSGR(ellipsis, sgrDim); got != hierarchyEllipsis {
		t.Errorf("dimmed text of the ellipsis row = %q, want %q", got, hierarchyEllipsis)
	}
	if textWithSGR(ellipsis, sgrBold) != "" || textWithSGR(ellipsis, sgrUnderline) != "" {
		t.Errorf("the ellipsis row carries more than dim: %q", ellipsis)
	}
}

// ellipsisRow returns the rendered row holding the cut-off marker.
func ellipsisRow(t *testing.T, view string) string {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, hierarchyEllipsis) {
			return line
		}
	}
	t.Fatalf("no rendered row holds the ellipsis marker:\n%q", view)
	return ""
}

func TestHierarchyOpenTicketIsNoCursorStopAndNoClickTarget(t *testing.T) {
	withANSIProfile(t)
	b := hierarchyTestBoard(hierarchyFixtureTasks(), 1, 1, 120, 40)

	for _, target := range b.layout.relations {
		if target.taskID == 1 {
			t.Errorf("the open ticket has a click target: %+v", target)
		}
	}

	c := b.detailContent(b.detailTask)
	nav := navigableRelations(c)
	if len(nav) == 0 {
		t.Fatal("no navigable rows in the tree")
	}
	for range len(nav) + 1 {
		b.moveDetailCursor(1)
		ref, ok := b.detailCursorRef(b.detailContent(b.detailTask))
		if ok && ref.taskID == 1 {
			t.Fatalf("tab put the cursor on the open ticket")
		}
	}

	openRow := refFor(t, c, 1)
	hoverAt(b, 4, openRow.startLine)
	if rows := underlinedRows(b.View()); len(rows) != 0 {
		t.Errorf("hovering the open ticket underlined rows %v", rows)
	}
}

// refFor returns the relation block of a task ID.
func refFor(t *testing.T, c detailContent, id int) relationRef {
	t.Helper()
	for _, ref := range c.relations {
		if ref.taskID == id {
			return ref
		}
	}
	t.Fatalf("no relation block for #%d", id)
	return relationRef{}
}

// --- T8: cursor, hit test and scrolling on a tall tree ---

// newDeepTreeTestBoard builds a board whose tree is far taller than the window:
// one root, six children, five grandchildren each, two levels shown, 20 rows of
// terminal height.
func newDeepTreeTestBoard() *Board {
	tasks := []*task.Task{
		{ID: 1, Title: "Deep Root", Status: dragStatusBacklog, Priority: treePriorityHigh, Updated: mouseTestTime},
	}
	id := 2
	for epic := range 6 {
		epicID := id
		tasks = append(tasks, &task.Task{
			ID: epicID, Title: fmt.Sprintf("Epic %d", epic), Status: dragStatusTodo,
			Priority: treePriorityDefault, Parent: idPtr(1), Updated: mouseTestTime,
		})
		id++
		for story := range 5 {
			tasks = append(tasks, &task.Task{
				ID: id, Title: fmt.Sprintf("Story %d-%d", epic, story), Status: dragStatusTodo,
				Priority: "low", Parent: idPtr(epicID), Updated: mouseTestTime,
			})
			id++
		}
	}
	return hierarchyTestBoard(tasks, 1, 2, 120, 20)
}

func TestDeepTreeTabScrollsThroughEveryStopInReadingOrder(t *testing.T) {
	b := newDeepTreeTestBoard()
	nav := navigableRelations(b.detailContent(b.detailTask))
	if len(nav) != 36 {
		t.Fatalf("navigable rows = %d, want 36 (6 epics plus 30 stories)", len(nav))
	}

	for i := range nav {
		b.moveDetailCursor(1)
		c := b.detailContent(b.detailTask)
		ref, ok := b.detailCursorRef(c)
		if !ok {
			t.Fatalf("cursor inactive after %d steps", i+1)
		}
		if ref.taskID != nav[i].taskID {
			t.Fatalf("step %d put the cursor on #%d, want #%d in reading order",
				i+1, ref.taskID, nav[i].taskID)
		}
		// Every block of this fixture fits the window; the block that cannot is
		// TestHierarchyBlockTallerThanTheWindowIsTopAligned.
		off, viewHeight := b.detailViewport(len(c.lines))
		if ref.startLine < off || ref.startLine+ref.lineCount > off+viewHeight {
			t.Fatalf("step %d left #%d (lines %d..%d) outside the window %d..%d",
				i+1, ref.taskID, ref.startLine, ref.startLine+ref.lineCount, off, off+viewHeight)
		}
	}
}

func TestDeepTreeClickOnLastRowAfterScrolling(t *testing.T) {
	b := newDeepTreeTestBoard()
	nav := navigableRelations(b.detailContent(b.detailTask))
	lastID := nav[len(nav)-1].taskID

	for range len(nav) {
		b.moveDetailCursor(1)
	}
	_ = b.View()

	target := relationTargetFor(t, b, lastID)
	clickAt(b, 4, target.rect.y0, tea.MouseButtonLeft)

	if b.detailTask == nil || b.detailTask.ID != lastID {
		t.Fatalf("click opened %v, want #%d", b.detailTask, lastID)
	}
	if len(b.detailStack) != 1 {
		t.Errorf("detail stack has %d frames, want 1", len(b.detailStack))
	}
}

func TestDeepTreeHalfScrolledBlockKeepsClippedHitZone(t *testing.T) {
	b := newDeepTreeTestBoard()
	// A title long enough to wrap the block over exactly two lines.
	b.allTasks[1].Title = strings.Repeat("Wrapping epic title ", 7)
	b.rebuildHierarchyIndex()
	b.invalidatePointerState()

	ref := refFor(t, b.detailContent(b.detailTask), 2)
	if ref.lineCount != 2 {
		t.Fatalf("row of #2 spans %d lines, want exactly 2", ref.lineCount)
	}
	b.detailScrollOff = ref.startLine + 1
	b.invalidatePointerState()
	_ = b.View()

	target := relationTargetFor(t, b, 2)
	if target.rect.y0 != 0 || target.rect.y1 != 1 {
		t.Fatalf("clipped hit zone = %d..%d, want 0..1", target.rect.y0, target.rect.y1)
	}
	withANSIProfile(t)
	hoverAt(b, 4, 0)
	if rows := underlinedRows(b.View()); len(rows) != 1 || rows[0] != 0 {
		t.Errorf("underlined rows = %v, want exactly row 0", rows)
	}
	clickAt(b, 4, 0, tea.MouseButtonLeft)
	if b.detailTask == nil || b.detailTask.ID != 2 {
		t.Errorf("click on the clipped row opened %v, want #2", b.detailTask)
	}
}

func TestDeepTreeWheelClampReachesLastRow(t *testing.T) {
	b := newDeepTreeTestBoard()
	nav := navigableRelations(b.detailContent(b.detailTask))
	lastID := nav[len(nav)-1].taskID

	for range 100 {
		_, _ = b.Update(tea.MouseMsg{
			X: 10, Y: 5, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress,
		})
	}

	want := fmt.Sprintf("#%d ", lastID)
	if got := b.View(); !strings.Contains(plainLine(got), want) {
		t.Errorf("the last tree row %q is not visible at the clamped offset:\n%s", want, got)
	}
}

// --- T9: the E2E click coordinate, derived instead of counted ---

func TestE2EChildRowGeometryIsPinned(t *testing.T) {
	// Same shape as the E2E fixture: a critical backlog parent with the default
	// class set, one backlog child, mouse on, 120x40.
	tasks := []*task.Task{
		{
			ID: 1, Title: "Parent task", Status: dragStatusBacklog, Priority: treePriorityHigh,
			Class: config.DefaultClass, Created: mouseTestTime, Updated: mouseTestTime,
		},
		{
			ID: 2, Title: "Child task", Status: dragStatusBacklog, Priority: treePriorityDefault,
			Class: config.DefaultClass, Parent: idPtr(1), Created: mouseTestTime, Updated: mouseTestTime,
		},
	}
	b := hierarchyTestBoard(tasks, 1, 1, 120, 40)

	got := relationTargetFor(t, b, 2).rect.y0

	if got != e2eChildRow {
		t.Errorf("the child row sits on screen row %d, not %d. Update childRow in "+
			"e2e/tui_mouse_navigation_test.go and e2eChildRow here to %d.", got, e2eChildRow, got)
	}
}

// --- Proofs the first round of tests left open ---

// archivedAncestorWithDoneChildrenTasks makes the archived ancestor's
// counter complete, so dimming and the "all done" green compete on one row.
func archivedAncestorWithDoneChildrenTasks() []*task.Task {
	tasks := archivedAncestorTasks()
	tasks[1].Status = treeStatusDone
	return append(tasks, &task.Task{
		ID: 3, Title: "Sibling Three", Status: treeStatusDone,
		Priority: treePriorityDefault, Parent: idPtr(1), Updated: mouseTestTime,
	})
}

func TestHierarchyDimmedRowKeepsItsDimCounter(t *testing.T) {
	withANSIProfile(t)
	b := hierarchyTestBoard(archivedAncestorWithDoneChildrenTasks(), 2, 1, 120, 40)

	ancestor := rowLineFor(t, b, 1)

	want := "#1 [archived] Archived Root (2/2 done)"
	if got := textWithSGR(ancestor, sgrDim); got != want {
		t.Errorf("dimmed text of the archived ancestor = %q, want %q — the counter has to be dimmed too: %q",
			got, want, ancestor)
	}
	// Archived is terminal for the board but never carries the color, so this
	// row proves the absence twice: through that rule and through dim.
	if got := textWithSGR(ancestor, sgrTerminal); got != "" {
		t.Errorf("green text on a dimmed row = %q, want none: green never wins against dim: %q",
			got, ancestor)
	}
}

func TestHierarchyOpenTicketCounterIsBold(t *testing.T) {
	withANSIProfile(t)
	// Zero levels keeps the children of the open ticket off the screen, which
	// is what keeps its counter on it.
	b := hierarchyTestBoard(hierarchyFixtureTasks(), 1, 0, 120, 40)

	open := rowLineFor(t, b, 1)

	want := "#1 [backlog] Root One (1/2 done)"
	if got := textWithSGR(open, sgrBold); got != want {
		t.Errorf("bold text of the open ticket = %q, want %q — its counter inherits the row decoration: %q",
			got, want, open)
	}
}

func TestHierarchyHeadingIsBold(t *testing.T) {
	withANSIProfile(t)
	b := hierarchyTestBoard(hierarchyFixtureTasks(), 1, 1, 120, 40)

	var heading string
	for _, line := range strings.Split(b.View(), "\n") {
		if strings.TrimSpace(plainLine(line)) == hierarchyHeading {
			heading = line
			break
		}
	}
	if heading == "" {
		t.Fatalf("no rendered line holds the %q heading:\n%s", hierarchyHeading, b.View())
	}

	if got := textWithSGR(heading, sgrBold); got != hierarchyHeading {
		t.Errorf("bold text of the heading row = %q, want %q: %q", got, hierarchyHeading, heading)
	}
}

func TestHierarchyIndentedRowsFitTheTerminalWidth(t *testing.T) {
	// The available text width is the view width minus the cursor gutter minus
	// the structure prefix. Dropping the prefix term only shows on an indented
	// row whose title wraps, which is what this fixture renders.
	const width = 40
	tasks := hierarchyFixtureTasks()
	for _, tk := range tasks {
		tk.Title = strings.Repeat("Wide title word ", 4)
	}
	b := hierarchyTestBoard(tasks, 1, 3, width, 60)

	c := b.detailContent(b.detailTask)
	view := strings.Split(b.View(), "\n")
	wrapped := 0
	for _, ref := range c.relations {
		if ref.lineCount > 1 {
			wrapped++
		}
		for i := range ref.lineCount {
			line := plainLine(view[ref.startLine+i])
			if got := lipgloss.Width(line); got > width {
				t.Errorf("row of #%d, line %d is %d cells wide at width %d: %q",
					ref.taskID, i, got, width, line)
			}
		}
	}

	deepest := 0
	for _, row := range b.hierarchyIndex.Tree(b.detailTask.ID, b.cfg, b.cfg.HierarchyLevels()).Rows {
		deepest = max(deepest, row.Depth)
	}
	if deepest < 2 || wrapped == 0 {
		t.Fatalf("fixture reached depth %d with %d wrapped rows, want an indented row that wraps", deepest, wrapped)
	}
}

func TestHierarchyCounterFitBoundaryIsExact(t *testing.T) {
	const width = 40
	prefix := hierarchyBranch(nil, 0, width, true)
	textWidth := hierarchyTextWidth(0, width)
	count := " (1/2 done)"
	head := fmt.Sprintf("#1 [%s] ", dragStatusBacklog)

	fitting := board.HierarchyRow{
		ID: 1, Status: dragStatusBacklog, Done: 1, Total: 2,
		Title: strings.Repeat("a", textWidth-lipgloss.Width(count)-lipgloss.Width(head)),
	}
	lines := hierarchyRowLines(fitting, prefix, width)
	if len(lines) != 1 || lines[0].count != count {
		t.Errorf("a row filling the last cell the counter fits in wrapped anyway: %+v", lines)
	}

	over := fitting
	over.Title += "a"
	lines = hierarchyRowLines(over, prefix, width)
	if len(lines) != 2 || lines[1].text() != "" || lines[1].count != count {
		t.Errorf("one cell past the fit, the counter did not move onto its own line: %+v", lines)
	}
}

func TestHierarchyCursorGutterOnlyOnTheFirstLine(t *testing.T) {
	tasks := hierarchyFixtureTasks()
	tasks[1].Title = strings.Repeat("Wrapping child title ", 4)
	b := hierarchyTestBoard(tasks, 1, 1, 40, 60)

	b.moveDetailCursor(1)
	c := b.detailContent(b.detailTask)
	ref := refFor(t, c, 2)
	if ref.lineCount < 2 {
		t.Fatalf("row of #2 spans %d lines, want at least 2", ref.lineCount)
	}
	view := strings.Split(b.View(), "\n")

	first := plainLine(view[ref.startLine])
	if !strings.HasPrefix(first, relationCursorGutter) {
		t.Errorf("first line of the cursor row = %q, want the cursor gutter %q", first, relationCursorGutter)
	}
	for i := 1; i < ref.lineCount; i++ {
		line := plainLine(view[ref.startLine+i])
		if !strings.HasPrefix(line, relationGutter) || strings.HasPrefix(line, relationCursorGutter) {
			t.Errorf("continuation line %d of the cursor row = %q, want the plain gutter %q", i, line, relationGutter)
		}
	}
}

func TestHierarchyBlockTallerThanTheWindowIsTopAligned(t *testing.T) {
	// A block that cannot fit is aligned to its top; aligning it to its bottom
	// would scroll its first line — the one carrying the ID — out of the window.
	tasks := hierarchyFixtureTasks()
	tasks[1].Title = strings.Repeat("Very long child title ", 12)
	tasks[2].Title = strings.Repeat("Very long sibling title ", 12)
	b := hierarchyTestBoard(tasks, 1, 1, 40, detailChrome+3)

	_, viewHeight := b.detailViewport(len(b.detailLines(b.detailTask)))
	ref := refFor(t, b.detailContent(b.detailTask), 2)
	if ref.lineCount <= viewHeight {
		t.Fatalf("row of #2 spans %d lines in a window of %d, want a taller block", ref.lineCount, viewHeight)
	}

	b.moveDetailCursor(1)

	if b.detailScrollOff != ref.startLine {
		t.Errorf("scroll offset = %d, want the first line of the block, %d", b.detailScrollOff, ref.startLine)
	}
}

// archivedOpenTicketTasks opens an archived task itself. The board layer allows
// this shape; the TUI has no path into it today, which is why the color rule
// needs a test rather than a reachable case.
func archivedOpenTicketTasks() []*task.Task {
	return []*task.Task{
		{ID: 1, Title: "Archived Open", Status: config.ArchivedStatus, Priority: treePriorityHigh, Updated: mouseTestTime},
		{ID: 2, Title: "Active Child", Status: dragStatusTodo, Priority: "high", Parent: idPtr(1), Updated: mouseTestTime},
	}
}

func TestHierarchyArchivedOpenTicketIsNeverGreen(t *testing.T) {
	withANSIProfile(t)
	b := hierarchyTestBoard(archivedOpenTicketTasks(), 1, 1, 120, 40)

	open := rowLineFor(t, b, 1)

	if got := textWithSGR(open, sgrTerminal); got != "" {
		t.Errorf("green text on the archived open ticket = %q, want none: archived is not finished, it is gone: %q",
			got, open)
	}
	if got := textWithSGR(open, sgrBold); got == "" {
		t.Errorf("the open ticket lost its bold text: %q", open)
	}
}

// statusTokenInTitleTasks gives a task a title that repeats its own status
// token, so only the real bracket may take the color.
func statusTokenInTitleTasks() []*task.Task {
	return []*task.Task{
		{ID: 1, Title: "Token Root", Status: dragStatusTodo, Priority: treePriorityHigh, Updated: mouseTestTime},
		{ID: 2, Title: "Rename [done] label", Status: treeStatusDone, Priority: "high", Parent: idPtr(1), Updated: mouseTestTime},
	}
}

func TestHierarchyGreenTakesTheStatusBracketNotTheTitle(t *testing.T) {
	withANSIProfile(t)
	b := hierarchyTestBoard(statusTokenInTitleTasks(), 1, 1, 120, 40)

	row := rowLineFor(t, b, 2)

	// Both tokens are the same string, so the value alone cannot tell them
	// apart — the position can. The status bracket is the first one.
	plain := plainLine(row)
	if strings.Index(plain, "[done]") == strings.LastIndex(plain, "[done]") {
		t.Fatalf("fixture no longer repeats the status token, the test proves nothing: %q", plain)
	}
	if green := textWithSGR(row, sgrTerminal); green != "[done]" {
		t.Fatalf("green text = %q, want %q: %q", green, "[done]", row)
	}
	// Everything the plain line carries before the green span. With the wrong
	// bracket colored this holds the status token as well.
	at := strings.Index(row, sgrTerminal)
	if at < 0 {
		t.Fatalf("no green span in the row, nothing to place: %q", row)
	}
	before := plainLine(row[:at])
	if strings.Contains(before, "[done]") {
		t.Errorf("text before the green span = %q: the color took the title, not the status bracket: %q",
			before, row)
	}
}

func TestHierarchyNarrowWidthLeavesNothingGreen(t *testing.T) {
	withANSIProfile(t)
	// Width 8 and 12 break the wrap before the status token, so
	// splitStatusSegment finds no bracket and takes its fallback. This pins the
	// resulting behavior — nothing green on those rows — but no mutation of the
	// fallback was found that turns it red, so read it as a guard against future
	// change, not as a proof of the current branch.
	for _, width := range []int{8, 12} {
		b := hierarchyTestBoard(statusPaletteTasks(), 1, 1, width, 40)
		for _, line := range treeLines(b) {
			if got := textWithSGR(line, sgrTerminal); got != "" {
				t.Errorf("width %d: green text %q on a line too narrow to hold a status bracket: %q",
					width, got, line)
			}
		}
	}
}
