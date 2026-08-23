package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/antopolskiy/kanban-md/internal/task"
)

// Layout constants for the relation gutter in front of every detail-view
// relation row. The gutter is always rendered so that switching the cursor on
// and off never reflows the surrounding text.
const (
	relationGutterWidth  = 2
	relationGutter       = "  "
	relationCursorGutter = "> "
)

// relationRef addresses one row of the hierarchy tree inside
// detailContent.lines. The row can span several lines because wrapTitle breaks
// long titles, so the block is described as a half-open range
// [startLine, startLine+lineCount). lines carries the same content split into
// the segments the decoration rules apply to.
type relationRef struct {
	taskID    int
	startLine int
	lineCount int
	navigable bool
	current   bool
	terminal  bool
	lines     []relationLine
}

// detailContent is the fully built detail-view body plus the tree rows inside
// it, in reading order: the ancestors of the open task from the outside in, the
// open task, then its descendants in preorder.
type detailContent struct {
	lines     []string
	relations []relationRef
}

// detailFrame is one entry of the detail-view navigation stack. It carries the
// complete state viewDetail reads while rendering, so popping a frame restores
// the abandoned screen exactly.
type detailFrame struct {
	taskID    int
	scrollOff int
	cursorOn  bool
	cursor    int
}

func (b *Board) viewDetail() string {
	t := b.detailTask
	if t == nil {
		return "No task selected."
	}

	c := b.detailContent(t)
	off, viewHeight := b.detailViewport(len(c.lines))
	body := b.renderDetailBody(c, off, viewHeight)
	hint := b.detailHint(c, len(c.lines) > viewHeight)

	if !b.mouseEnabled {
		return body + "\n\n" + dimStyle.Render(truncate(hint, b.width))
	}

	hint = "← Back  " + hint
	renderedLines := min(off+viewHeight, len(c.lines)) - off
	if renderedLines < viewHeight {
		body += strings.Repeat("\n", viewHeight-renderedLines)
	}
	backWidth := min(lipgloss.Width("← Back"), b.width)
	if backWidth > 0 && b.height > 0 {
		b.layout.back = &backTarget{
			rect: rect{x0: 0, y0: b.height - 1, x1: backWidth, y1: b.height},
		}
	}
	return body + "\n\n" + dimStyle.Render(truncate(hint, b.width))
}

// detailViewport returns the first visible content line and the height of the
// body window, clamping the stored scroll offset so the next key press starts
// from the position actually rendered.
func (b *Board) detailViewport(total int) (int, int) {
	viewHeight := b.height - detailChrome
	if viewHeight < 1 {
		viewHeight = total
	}
	off := b.detailScrollOff
	maxOff := total - viewHeight
	if maxOff < 0 {
		maxOff = 0
	}
	if off > maxOff {
		off = maxOff
		b.detailScrollOff = off
	}
	return off, viewHeight
}

// detailCursorRef returns the relation under the keyboard cursor. It is the only
// read path for the cursor and clamps it on the way, so a reload that shrinks
// the relation list can never leave the cursor pointing at nothing.
func (b *Board) detailCursorRef(c detailContent) (relationRef, bool) {
	nav := navigableRelations(c)
	if len(nav) == 0 {
		b.detailCursorOn = false
		b.detailCursor = 0
		return relationRef{}, false
	}
	if b.detailCursor >= len(nav) {
		b.detailCursor = len(nav) - 1
	}
	if b.detailCursor < 0 {
		b.detailCursor = 0
	}
	if !b.detailCursorOn {
		return relationRef{}, false
	}
	return nav[b.detailCursor], true
}

// renderDetailBody cuts the visible window out of the content and decorates the
// relation rows inside it.
func (b *Board) renderDetailBody(c detailContent, off, viewHeight int) string {
	end := min(off+viewHeight, len(c.lines))
	if off > end {
		off = end
	}
	b.captureDetailLayout(c, off, viewHeight)
	cursorRef, hasCursor := b.detailCursorRef(c)
	hoverRef, hasHover := b.detailHoverRef(c, off, viewHeight)

	out := make([]string, 0, end-off)
	for i := off; i < end; i++ {
		ref, isRelation := relationAtLine(c, i)
		if !isRelation {
			out = append(out, c.lines[i])
			continue
		}
		out = append(out, decorateRelationLine(ref, ref.lines[i-ref.startLine], relationDecor{
			firstLine: i == ref.startLine,
			onCursor:  hasCursor && ref.startLine == cursorRef.startLine,
			onHover:   hasHover && ref.startLine == hoverRef.startLine,
		}))
	}
	return strings.Join(out, "\n")
}

// detailHoverRef returns the navigable relation block under the mouse pointer.
// The pointer position is remembered from the last buttonless motion event, so
// a position that no longer covers a relation row simply matches nothing — no
// leave event is needed.
func (b *Board) detailHoverRef(c detailContent, off, viewHeight int) (relationRef, bool) {
	if !b.mouseEnabled || !b.pointer.hoverActive {
		return relationRef{}, false
	}
	screen := rect{x0: 0, y0: 0, x1: b.width, y1: min(viewHeight, b.height)}
	if !screen.contains(b.pointer.hoverX, b.pointer.hoverY) {
		return relationRef{}, false
	}
	ref, ok := relationAtLine(c, b.pointer.hoverY+off)
	if !ok || !ref.navigable {
		return relationRef{}, false
	}
	return ref, true
}

// captureDetailLayout records a click target for every navigable relation row
// currently on screen. It runs inside the renderer, so target and decoration
// can never disagree about the scroll offset.
func (b *Board) captureDetailLayout(c detailContent, off, viewHeight int) {
	if !b.mouseEnabled || b.view != viewDetail || b.width <= 0 || viewHeight <= 0 {
		return
	}
	screen := rect{x0: 0, y0: 0, x1: b.width, y1: min(viewHeight, b.height)}
	if screen.empty() {
		return
	}
	for _, ref := range c.relations {
		if !ref.navigable {
			continue
		}
		hit := rect{
			x0: 0,
			y0: ref.startLine - off,
			x1: b.width,
			y1: ref.startLine + ref.lineCount - off,
		}.intersect(screen)
		if hit.empty() {
			continue
		}
		b.layout.relations = append(b.layout.relations, relationTarget{taskID: ref.taskID, rect: hit})
	}
}

// relationAtLine returns the relation block that owns a content line.
func relationAtLine(c detailContent, line int) (relationRef, bool) {
	for _, ref := range c.relations {
		if line >= ref.startLine && line < ref.startLine+ref.lineCount {
			return ref, true
		}
	}
	return relationRef{}, false
}

// relationDecor tells decorateRelationLine which decorations apply to one line
// of a relation block.
type relationDecor struct {
	firstLine bool
	onCursor  bool
	onHover   bool
}

// decorateRelationLine prefixes a tree line with its gutter and decorates its
// text: bold for the open task, dim for a row that cannot be opened, underline
// for the hovered row and green for a status bracket that reports the task is
// finished. Gutter, indentation and branch glyph stay undecorated — the click
// target is the node, not the frame around it.
func decorateRelationLine(ref relationRef, line relationLine, decor relationDecor) string {
	gutter := relationGutter
	if decor.onCursor && decor.firstLine {
		gutter = relationCursorGutter
	}
	state := relationLineState{
		dim:       !ref.navigable && !ref.current,
		bold:      ref.current,
		underline: decor.onHover,
	}
	statusState := state
	statusState.terminal = ref.terminal

	return gutter + line.prefix +
		renderRelationSegment(line.head, state) +
		renderRelationSegment(line.status, statusState) +
		renderRelationSegment(line.tail, state) +
		renderRelationSegment(line.count, state)
}

// renderRelationSegment styles one segment of a tree line. An empty segment
// renders to nothing at all, so a row never carries escape codes for a part it
// does not have.
func renderRelationSegment(text string, state relationLineState) string {
	if text == "" {
		return ""
	}
	return relationSegmentStyle(state).Render(text)
}

// detailHint builds the fixed bottom line of the detail view. Only keys that do
// something in this concrete view are advertised.
func (b *Board) detailHint(c detailContent, overflow bool) string {
	hint := "q/esc:back"
	if steps := b.detailBackSteps(); steps > 0 {
		hint = fmt.Sprintf("q:close  esc:back(%d)", steps)
	}
	if overflow {
		hint += "  j/k:scroll  g/G:top/bottom"
	}
	if len(navigableRelations(c)) > 0 {
		hint += "  tab:relations  enter:open"
	}
	return hint
}

// detailBackSteps counts the times esc can still go back. popDetailFrame skips
// frames whose task has vanished, so counting raw frames would advertise steps
// the key does not take.
func (b *Board) detailBackSteps() int {
	steps := 0
	for _, frame := range b.detailStack {
		if b.relationVisible(frame.taskID) {
			steps++
		}
	}
	return steps
}

// relationVisible reports whether a task is reachable at all: it mirrors
// refreshDetailTask, where only a task in unfilteredTasks survives the next
// reload. It is a precondition for a navigable row, not the whole rule — the
// open task is reachable and is still no link to itself.
//
// unfilteredTasks is allTasks minus the archived ones by construction, so an
// indexed, non-archived task is exactly a member of it.
func (b *Board) relationVisible(taskID int) bool {
	t := b.hierarchyIndex.Task(taskID)
	return t != nil && !b.cfg.IsArchivedStatus(t.Status)
}

// detailContent builds a task's detail-view content around its hierarchy tree.
// The tree comes from the index over allTasks, so an archived ancestor still
// resolves, while archived descendants are filtered out on the way down — the
// same two sources the view read before.
func (b *Board) detailContent(t *task.Task) detailContent {
	tree := b.hierarchyIndex.Tree(t.ID, b.cfg, b.cfg.HierarchyLevels())
	c := detailContent{lines: detailHeadLines(t, b.width)}
	b.appendHierarchy(&c, tree, b.width)
	b.appendDetailBody(&c, t, b.width)
	return c
}

// detailLines returns just the rendered lines of a task's detail content.
func (b *Board) detailLines(t *task.Task) []string {
	return b.detailContent(t).lines
}

// navigableRelations returns the relations that can take the cursor, in render
// order.
func navigableRelations(c detailContent) []relationRef {
	var nav []relationRef
	for _, ref := range c.relations {
		if ref.navigable {
			nav = append(nav, ref)
		}
	}
	return nav
}

// detailHeadLines renders everything above the relation rows: header,
// separator, metadata, timestamps and the blocked banner.
func detailHeadLines(t *task.Task, width int) []string {
	var lines []string
	header := fmt.Sprintf("Task #%d: %s", t.ID, t.Title)
	// Word-wrap the header so long titles fit within the available terminal width.
	boldStyle := lipgloss.NewStyle().Bold(true)
	for _, l := range wrapTitle(header, width, noLineLimit) {
		lines = append(lines, boldStyle.Render(l))
	}
	// Separator: as wide as the header, capped at terminal width.
	sepWidth := lipgloss.Width(header)
	if sepWidth > width {
		sepWidth = width
	}
	lines = append(lines, strings.Repeat("─", sepWidth))
	lines = append(lines, "")
	lines = append(lines, detailLabelStyle.Render("Status:")+"  "+t.Status)
	lines = append(lines, detailLabelStyle.Render("Priority:")+"  "+t.Priority)
	lines = append(lines, detailMetadataLines(t)...)
	lines = append(lines, detailTimestampLines(t)...)
	if t.Blocked {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render("BLOCKED: "+t.BlockReason))
	}
	return lines
}

// appendDetailBody renders the markdown body below the relations.
func (b *Board) appendDetailBody(c *detailContent, t *task.Task, width int) {
	if t.Body == "" {
		return
	}
	c.lines = append(c.lines, "")
	rendered := b.renderTaskBody(unescapeBody(t.Body), width)
	c.lines = append(c.lines, strings.Split(rendered, "\n")...)
}

// moveDetailCursor advances the relation cursor. The first press activates it —
// forwards on the first relation, backwards on the last — and every further
// press wraps in both directions.
func (b *Board) moveDetailCursor(delta int) {
	if b.detailTask == nil {
		return
	}
	c := b.detailContent(b.detailTask)
	nav := navigableRelations(c)
	if len(nav) == 0 {
		b.detailCursorOn = false
		b.detailCursor = 0
		return
	}
	if b.detailCursorOn {
		b.detailCursor = ((b.detailCursor+delta)%len(nav) + len(nav)) % len(nav)
	} else {
		b.detailCursorOn = true
		b.detailCursor = 0
		if delta < 0 {
			b.detailCursor = len(nav) - 1
		}
	}
	b.ensureRelationVisible(len(c.lines), nav[b.detailCursor])
}

// ensureRelationVisible scrolls the body just far enough to show the cursor row.
// A block taller than the window is aligned to its top instead of its bottom.
func (b *Board) ensureRelationVisible(total int, ref relationRef) {
	viewHeight := b.height - detailChrome
	if viewHeight < 1 {
		// The whole content fits, so there is nothing to scroll.
		return
	}
	maxOff := total - viewHeight
	if maxOff < 0 {
		maxOff = 0
	}
	off := b.detailScrollOff
	switch {
	case ref.startLine < off:
		off = ref.startLine
	case ref.startLine+ref.lineCount > off+viewHeight:
		off = min(ref.startLine+ref.lineCount-viewHeight, ref.startLine)
	}
	b.detailScrollOff = max(0, min(off, maxOff))
}

// openRelation opens a related task in the detail view and remembers the
// abandoned screen on the navigation stack. A target that is not navigable
// leaves the view untouched.
func (b *Board) openRelation(taskID int) {
	if b.detailTask == nil {
		return
	}
	var target *task.Task
	for _, candidate := range b.unfilteredTasks {
		if candidate.ID == taskID {
			target = candidate
			break
		}
	}
	if target == nil {
		return
	}
	b.detailStack = append(b.detailStack, detailFrame{
		taskID:    b.detailTask.ID,
		scrollOff: b.detailScrollOff,
		cursorOn:  b.detailCursorOn,
		cursor:    b.detailCursor,
	})
	b.detailTask = target
	b.detailScrollOff = 0
	b.detailCursorOn = false
	b.detailCursor = 0
	b.invalidatePointerState()
}

// openCursorRelation opens whatever the keyboard cursor points at.
func (b *Board) openCursorRelation() {
	if b.detailTask == nil {
		return
	}
	if ref, ok := b.detailCursorRef(b.detailContent(b.detailTask)); ok {
		b.openRelation(ref.taskID)
	}
}

// popDetailFrame returns to the previously visited task, restoring its scroll
// offset and cursor. Frames whose task has vanished are skipped. It reports
// whether a frame could be restored.
func (b *Board) popDetailFrame() bool {
	for len(b.detailStack) > 0 {
		frame := b.detailStack[len(b.detailStack)-1]
		b.detailStack = b.detailStack[:len(b.detailStack)-1]
		for _, candidate := range b.unfilteredTasks {
			if candidate.ID != frame.taskID {
				continue
			}
			b.detailTask = candidate
			b.detailScrollOff = frame.scrollOff
			b.detailCursorOn = frame.cursorOn
			b.detailCursor = frame.cursor
			b.invalidatePointerState()
			return true
		}
	}
	return false
}

// closeDetail leaves the detail view and forgets the relation history.
func (b *Board) closeDetail() {
	b.view = viewBoard
	b.detailTask = nil
	b.detailScrollOff = 0
	b.detailCursorOn = false
	b.detailCursor = 0
	b.detailStack = nil
	b.invalidatePointerState()
}

// backOrCloseDetail goes one step back in the relation history, or closes the
// detail view when there is no history left.
func (b *Board) backOrCloseDetail() {
	if !b.popDetailFrame() {
		b.closeDetail()
	}
}
