package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	doubleClickWindow = 500 * time.Millisecond
	detailWheelStep   = 3
	detailChrome      = 2 // blank separator + fixed hint
)

// rect is a zero-based, half-open terminal-cell rectangle.
type rect struct {
	x0 int
	y0 int
	x1 int
	y1 int
}

func (r rect) contains(x, y int) bool {
	return x >= r.x0 && x < r.x1 && y >= r.y0 && y < r.y1
}

func (r rect) empty() bool {
	return r.x0 >= r.x1 || r.y0 >= r.y1
}

func (r rect) intersect(other rect) rect {
	return rect{
		x0: max(r.x0, other.x0),
		y0: max(r.y0, other.y0),
		x1: min(r.x1, other.x1),
		y1: min(r.y1, other.y1),
	}
}

func (r rect) translate(x, y int) rect {
	return rect{x0: r.x0 + x, y0: r.y0 + y, x1: r.x1 + x, y1: r.y1 + y}
}

type cardTarget struct {
	taskID int
	col    int
	row    int
	rect   rect
}

type columnTarget struct {
	col    int
	status string
	rect   rect
}

type backTarget struct {
	rect rect
}

// relationTarget is the click target of one navigable relation row in the
// detail view.
type relationTarget struct {
	taskID int
	rect   rect
}

type layoutSnapshot struct {
	generation uint64
	width      int
	height     int
	view       view
	cards      []cardTarget
	columns    []columnTarget
	// tabs are the narrow-mode tab-strip hit targets; tapping one switches
	// the active column.
	tabs []columnTarget
	back *backTarget
	// relations are the detail-view relation rows that can be opened with a
	// single click, in render order.
	relations []relationTarget
}

type pointerTargetKind int

const (
	pointerTargetNone pointerTargetKind = iota
	pointerTargetCard
	pointerTargetBack
	pointerTargetRelation
)

type pointerState struct {
	pressed           bool
	kind              pointerTargetKind
	taskID            int
	rect              rect
	generation        uint64
	sourceCol         int
	destination       int
	destinationStatus string
	dragStarted       bool
	lastClickID       int
	lastClickTime     time.Time
	// Hover position of the last buttonless motion event. Cleared only by
	// invalidatePointerState, never by clearGesture, so a hover survives a
	// stream of motion events but dies on any key press or resize.
	hoverActive bool
	hoverX      int
	hoverY      int
}

func (b *Board) invalidatePointerState() {
	b.layoutGeneration++
	b.layout = layoutSnapshot{}
	b.pointer = pointerState{}
}

func (b *Board) clearGesture() {
	b.pointer.pressed = false
	b.pointer.kind = pointerTargetNone
	b.pointer.taskID = 0
	b.pointer.rect = rect{}
	b.pointer.generation = 0
	b.pointer.sourceCol = 0
	b.pointer.destination = -1
	b.pointer.destinationStatus = ""
	b.pointer.dragStarted = false
}

func (b *Board) clearPendingClick() {
	b.pointer.lastClickID = 0
	b.pointer.lastClickTime = time.Time{}
}

// clearHover forgets the remembered pointer position, so nothing stays
// underlined once the hover is no longer valid.
func (b *Board) clearHover() {
	b.pointer.hoverActive = false
	b.pointer.hoverX = 0
	b.pointer.hoverY = 0
}

func (b *Board) captureBoardLayout(colWidth, targetHeight int, targets [][]cardTarget) {
	if !b.mouseEnabled || b.view != viewBoard || b.width <= 0 || targetHeight <= 0 {
		return
	}

	screen := rect{x0: 0, y0: 0, x1: b.width, y1: min(targetHeight, b.height)}
	if screen.empty() {
		return
	}

	for colIdx, col := range b.columns {
		x := colIdx * colWidth
		colRect := rect{x0: x, y0: 0, x1: x + colWidth, y1: targetHeight}.intersect(screen)
		if colRect.empty() {
			continue
		}
		b.layout.columns = append(b.layout.columns, columnTarget{
			col:    colIdx,
			status: col.status,
			rect:   colRect,
		})
		for _, target := range targets[colIdx] {
			target.rect = target.rect.translate(x, 0).intersect(screen)
			if !target.rect.empty() {
				b.layout.cards = append(b.layout.cards, target)
			}
		}
	}
}

// captureNarrowBoardLayout records hit-test rects for narrow (single-column)
// mode: tab-strip targets on row 0, the active column below it, and its card
// targets translated past the tab bar. With one column in the layout,
// drag-to-move degrades to click semantics; switching happens via the tabs.
func (b *Board) captureNarrowBoardLayout(targetHeight int, targets []cardTarget, tabs []columnTarget) {
	if !b.mouseEnabled || b.view != viewBoard || b.width <= 0 || targetHeight <= 0 {
		return
	}

	screen := rect{x0: 0, y0: 0, x1: b.width, y1: min(targetHeight, b.height)}
	if screen.empty() {
		return
	}

	const tabBarHeight = 1
	for _, tab := range tabs {
		tab.rect = tab.rect.intersect(screen)
		if !tab.rect.empty() {
			b.layout.tabs = append(b.layout.tabs, tab)
		}
	}

	colRect := rect{x0: 0, y0: tabBarHeight, x1: b.width, y1: targetHeight}.intersect(screen)
	if colRect.empty() {
		return
	}
	b.layout.columns = append(b.layout.columns, columnTarget{
		col:    b.activeCol,
		status: b.columns[b.activeCol].status,
		rect:   colRect,
	})
	for _, target := range targets {
		target.rect = target.rect.translate(0, tabBarHeight).intersect(screen)
		if !target.rect.empty() {
			b.layout.cards = append(b.layout.cards, target)
		}
	}
}

// tabAt returns the narrow-mode tab target at the given position, if any.
func (b *Board) tabAt(x, y int) *columnTarget {
	if b.layout.generation != b.layoutGeneration || b.layout.view != viewBoard {
		return nil
	}
	for i := range b.layout.tabs {
		if b.layout.tabs[i].rect.contains(x, y) {
			return &b.layout.tabs[i]
		}
	}
	return nil
}

func (b *Board) handleMouse(msg tea.MouseEvent) (tea.Model, tea.Cmd) {
	if !b.mouseEnabled || msg.Shift || msg.Alt || msg.Ctrl {
		b.clearGesture()
		if msg.Shift || msg.Alt || msg.Ctrl {
			b.clearPendingClick()
			// All-motion reporting routes plain pointer moves through here too,
			// and a hover is a gesture like any other: dropping it keeps the
			// underline from freezing on the row the pointer has left.
			b.clearHover()
		}
		return b, nil
	}

	switch b.view {
	case viewBoard:
		return b.handleBoardMouse(msg)
	case viewDetail:
		return b.handleDetailMouse(msg)
	default:
		b.clearGesture()
		return b, nil
	}
}

func (b *Board) handleBoardMouse(msg tea.MouseEvent) (tea.Model, tea.Cmd) {
	if msg.IsWheel() {
		b.clearGesture()
		b.clearPendingClick()
		if isVerticalWheel(msg.Button) {
			b.handleBoardWheel(msg)
		}
		return b, nil
	}

	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft {
			b.clearGesture()
			b.clearPendingClick()
			return b, nil
		}
		if tab := b.tabAt(msg.X, msg.Y); tab != nil {
			b.clearGesture()
			b.clearPendingClick()
			b.activeCol = tab.col
			b.clampRow()
			b.ensureVisible()
			return b, nil
		}
		if target := b.cardAt(msg.X, msg.Y); target != nil {
			b.pointer.pressed = true
			b.pointer.kind = pointerTargetCard
			b.pointer.taskID = target.taskID
			b.pointer.rect = target.rect
			b.pointer.generation = b.layout.generation
			b.pointer.sourceCol = target.col
			b.pointer.destination = -1
		} else {
			b.clearGesture()
			b.clearPendingClick()
		}
	case tea.MouseActionMotion:
		// Symmetric to the detail branch: a buttonless move must never touch
		// the gesture state. With an armed press it would otherwise drag the
		// destination marker along with a pointer that holds no button.
		if isHoverMotion(msg) {
			return b, nil
		}
		if b.pointer.pressed && b.pointer.kind == pointerTargetCard {
			b.updateDragTarget(msg.X, msg.Y)
		}
	case tea.MouseActionRelease:
		if msg.Button != tea.MouseButtonLeft && msg.Button != tea.MouseButtonNone {
			b.clearGesture()
			b.clearPendingClick()
			return b, nil
		}
		return b.releaseBoardPointer(msg.X, msg.Y)
	}

	return b, nil
}

func (b *Board) updateDragTarget(x, y int) {
	if b.pointer.generation != b.layout.generation ||
		b.layout.generation != b.layoutGeneration {
		b.pointer.dragStarted = true
		b.pointer.destination = -1
		b.pointer.destinationStatus = ""
		return
	}

	if !b.pointer.rect.contains(x, y) {
		b.pointer.dragStarted = true
	}

	target := b.columnAt(x, y)
	if target == nil || target.col == b.pointer.sourceCol {
		b.pointer.destination = -1
		b.pointer.destinationStatus = ""
		return
	}

	b.pointer.destination = target.col
	b.pointer.destinationStatus = target.status
}

func (b *Board) releaseBoardPointer(x, y int) (tea.Model, tea.Cmd) {
	if !b.pointer.pressed ||
		b.pointer.kind != pointerTargetCard ||
		b.pointer.generation != b.layout.generation ||
		b.layout.generation != b.layoutGeneration {
		b.clearGesture()
		b.clearPendingClick()
		return b, nil
	}

	targetColumn := b.columnAt(x, y)
	if targetColumn == nil {
		b.clearGesture()
		b.clearPendingClick()
		return b, nil
	}

	taskID := b.pointer.taskID
	if targetColumn.col != b.pointer.sourceCol {
		targetStatus := targetColumn.status
		b.clearGesture()
		b.clearPendingClick()
		return b.executeMoveTask(taskID, targetStatus, true)
	}

	if b.pointer.dragStarted || !b.pointer.rect.contains(x, y) {
		b.clearGesture()
		b.clearPendingClick()
		return b, nil
	}

	b.clearGesture()
	if !b.selectTask(taskID) {
		b.clearPendingClick()
		return b, nil
	}

	now := b.mouseNow()
	if b.pointer.lastClickID == taskID &&
		!b.pointer.lastClickTime.IsZero() &&
		now.Sub(b.pointer.lastClickTime) >= 0 &&
		now.Sub(b.pointer.lastClickTime) <= doubleClickWindow {
		b.clearPendingClick()
		b.handleEnter()
		return b, nil
	}

	b.pointer.lastClickID = taskID
	b.pointer.lastClickTime = now
	return b, nil
}

func (b *Board) dragDestination() (int, string, bool) {
	if !b.pointer.pressed ||
		b.pointer.kind != pointerTargetCard ||
		b.pointer.destination < 0 ||
		b.pointer.destinationStatus == "" ||
		b.pointer.generation != b.layoutGeneration {
		return 0, "", false
	}
	return b.pointer.destination, b.pointer.destinationStatus, true
}

func (b *Board) handleDetailMouse(msg tea.MouseEvent) (tea.Model, tea.Cmd) {
	if msg.IsWheel() {
		b.clearGesture()
		b.clearPendingClick()
		if isVerticalWheel(msg.Button) {
			b.handleDetailWheel(msg.Button)
		}
		return b, nil
	}

	switch msg.Action {
	case tea.MouseActionPress:
		b.handleDetailPress(msg)
	case tea.MouseActionMotion:
		// Hover first: a buttonless move must never touch the gesture state.
		if isHoverMotion(msg) {
			b.pointer.hoverActive = true
			b.pointer.hoverX = msg.X
			b.pointer.hoverY = msg.Y
			return b, nil
		}
		if b.pointer.pressed && !b.pointer.rect.contains(msg.X, msg.Y) {
			b.clearGesture()
		}
	case tea.MouseActionRelease:
		b.handleDetailRelease(msg)
	}

	return b, nil
}

func (b *Board) handleDetailWheel(button tea.MouseButton) {
	switch button {
	case tea.MouseButtonWheelUp:
		b.detailScrollOff -= detailWheelStep
		if b.detailScrollOff < 0 {
			b.detailScrollOff = 0
		}
	case tea.MouseButtonWheelDown:
		b.detailScrollOff += detailWheelStep
		b.clampDetailScroll()
	}
}

func (b *Board) handleDetailPress(msg tea.MouseEvent) {
	if msg.Button != tea.MouseButtonLeft {
		b.clearGesture()
		b.clearPendingClick()
		return
	}
	if b.layout.back != nil && b.layout.back.rect.contains(msg.X, msg.Y) {
		b.beginDetailGesture(pointerTargetBack, 0, b.layout.back.rect)
		return
	}
	if target := b.relationAt(msg.X, msg.Y); target != nil {
		b.beginDetailGesture(pointerTargetRelation, target.taskID, target.rect)
		return
	}
	b.clearGesture()
	b.clearPendingClick()
}

// beginDetailGesture arms a press on a detail-view target.
func (b *Board) beginDetailGesture(kind pointerTargetKind, taskID int, target rect) {
	b.pointer.pressed = true
	b.pointer.kind = kind
	b.pointer.taskID = taskID
	b.pointer.rect = target
	b.pointer.generation = b.layout.generation
}

// detailGestureLandsOn reports whether the armed press is still backed by the
// rendered layout and the release happened inside its target.
func (b *Board) detailGestureLandsOn(x, y int) bool {
	return b.pointer.pressed &&
		b.pointer.generation == b.layout.generation &&
		b.layout.generation == b.layoutGeneration &&
		b.pointer.rect.contains(x, y)
}

func (b *Board) handleDetailRelease(msg tea.MouseEvent) {
	if msg.Button != tea.MouseButtonLeft && msg.Button != tea.MouseButtonNone {
		b.clearGesture()
		return
	}
	if !b.detailGestureLandsOn(msg.X, msg.Y) {
		b.clearGesture()
		return
	}
	switch b.pointer.kind {
	case pointerTargetBack:
		b.backOrCloseDetail()
	case pointerTargetRelation:
		taskID := b.pointer.taskID
		b.clearGesture()
		b.openRelation(taskID)
	default:
		b.clearGesture()
	}
}

// relationAt returns the detail-view relation target at the given position, if
// the rendered layout is still current.
func (b *Board) relationAt(x, y int) *relationTarget {
	if b.layout.generation != b.layoutGeneration || b.layout.view != viewDetail {
		return nil
	}
	for i := range b.layout.relations {
		if b.layout.relations[i].rect.contains(x, y) {
			return &b.layout.relations[i]
		}
	}
	return nil
}

// isHoverMotion reports whether an event is a plain pointer move with no button
// held. Bubble Tea reports those as a motion action without a button, and only
// all-motion reporting (1003) delivers them at all.
func isHoverMotion(msg tea.MouseEvent) bool {
	return msg.Action == tea.MouseActionMotion && msg.Button == tea.MouseButtonNone
}

func isVerticalWheel(button tea.MouseButton) bool {
	return button == tea.MouseButtonWheelUp || button == tea.MouseButtonWheelDown
}

func (b *Board) handleBoardWheel(msg tea.MouseEvent) {
	target := b.columnAt(msg.X, msg.Y)
	if target == nil {
		return
	}

	b.activeCol = target.col
	b.clampRow()
	col := b.currentColumn()
	if col == nil || len(col.tasks) == 0 {
		return
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if b.activeRow > 0 {
			b.activeRow--
		}
	case tea.MouseButtonWheelDown:
		if b.activeRow < len(col.tasks)-1 {
			b.activeRow++
		}
	default:
		return
	}
	b.ensureVisible()
}

func (b *Board) cardAt(x, y int) *cardTarget {
	if b.layout.generation != b.layoutGeneration || b.layout.view != viewBoard {
		return nil
	}
	for i := range b.layout.cards {
		if b.layout.cards[i].rect.contains(x, y) {
			return &b.layout.cards[i]
		}
	}
	return nil
}

func (b *Board) columnAt(x, y int) *columnTarget {
	if b.layout.generation != b.layoutGeneration || b.layout.view != viewBoard {
		return nil
	}
	for i := range b.layout.columns {
		if b.layout.columns[i].rect.contains(x, y) {
			return &b.layout.columns[i]
		}
	}
	return nil
}

func (b *Board) selectTask(id int) bool {
	for colIdx := range b.columns {
		for rowIdx, t := range b.columns[colIdx].tasks {
			if t.ID == id {
				b.activeCol = colIdx
				b.activeRow = rowIdx
				b.ensureVisible()
				return true
			}
		}
	}
	b.clampRow()
	return false
}

func (b *Board) clampDetailScroll() {
	if b.detailTask == nil {
		b.detailScrollOff = 0
		return
	}
	viewHeight := b.height - detailChrome
	if viewHeight < 1 {
		viewHeight = len(b.detailLines(b.detailTask))
	}
	maxOff := len(b.detailLines(b.detailTask)) - viewHeight
	if maxOff < 0 {
		maxOff = 0
	}
	if b.detailScrollOff > maxOff {
		b.detailScrollOff = maxOff
	}
}
