package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/antopolskiy/kanban-md/internal/board"
)

// Layout of the detail-view hierarchy tree.
const (
	hierarchyIndentPerLevel = 3
	hierarchyBranchWidth    = 3
	// minHierarchyTextWidth is the text width below which the tree stops
	// indenting deeper levels. minUsableColumnWidth (14) does not transfer: it
	// bounds a card column that shows a title alone, while a tree row spends
	// about 19 cells on "#123 [in-progress] " before the first letter of a title.
	minHierarchyTextWidth = 20

	hierarchyHeading  = "Hierarchy"
	hierarchyEllipsis = "…"

	// Branch glyphs and the continuation columns behind them, three cells each.
	branchTee   = "├─ "
	branchEnd   = "└─ "
	branchPipe  = "│  "
	branchBlank = "   "

	// hierarchyDimColor is the foreground of dimStyle. The tree needs the value
	// itself because it composes one style per segment instead of nesting
	// renders. hierarchyCompleteColor is the green that already carries the
	// "done and valid" meaning in dropTargetColumnHeaderStyle.
	hierarchyDimColor      = "241"
	hierarchyCompleteColor = "42"
)

// relationLine is one rendered line of a tree row, split into the segments the
// decoration rules apply to. prefix is indentation plus branch glyph and is
// never decorated; text starts at the '#'; count is the child counter, which is
// only set on the line that carries it.
type relationLine struct {
	prefix string
	text   string
	count  string
}

// plain returns the undecorated full text of the line.
func (l relationLine) plain() string { return l.prefix + l.text + l.count }

// relationLineState carries the decorations that apply to one segment.
type relationLineState struct {
	dim       bool
	bold      bool
	underline bool
	complete  bool
}

// relationSegmentStyle composes one style out of the attributes that apply,
// rather than nesting renders: a nested render ends the outer style at the inner
// reset, which is what made the reach of an underline depend on the row state.
func relationSegmentStyle(state relationLineState) lipgloss.Style {
	style := lipgloss.NewStyle()
	switch {
	case state.dim:
		style = style.Foreground(lipgloss.Color(hierarchyDimColor))
	case state.complete:
		style = style.Foreground(lipgloss.Color(hierarchyCompleteColor))
	}
	if state.bold {
		style = style.Bold(true)
	}
	if state.underline {
		style = style.Underline(true)
	}
	return style
}

// hierarchyMaxIndentDepth returns the deepest level that still earns its own
// indentation step at a given view width. Deeper levels render on the column of
// the last level that fit.
func hierarchyMaxIndentDepth(width int) int {
	free := width - relationGutterWidth - hierarchyBranchWidth - minHierarchyTextWidth
	if free < 0 {
		return 0
	}
	return free / hierarchyIndentPerLevel
}

// hierarchyPrefixWidth returns the cells a row of the given depth spends on
// indentation and branch glyph.
func hierarchyPrefixWidth(depth, width int) int {
	return min(depth, hierarchyMaxIndentDepth(width))*hierarchyIndentPerLevel + hierarchyBranchWidth
}

// hierarchyTextWidth returns the cells left for a row's text. It never drops
// below one: wrapTitle turns a non-positive width into empty lines.
func hierarchyTextWidth(depth, width int) int {
	return max(1, width-relationGutterWidth-hierarchyPrefixWidth(depth, width))
}

// hierarchyBranch builds the structure prefix of a row. lastAtDepth carries, per
// ancestor level, whether the row on that level was the last shown sibling; a
// level with siblings still to come draws the continuation pipe. The indentation
// is capped where hierarchyPrefixWidth caps it.
func hierarchyBranch(lastAtDepth []bool, depth, width int, last bool) string {
	var out strings.Builder
	for i := range min(depth, hierarchyMaxIndentDepth(width)) {
		if i < len(lastAtDepth) && lastAtDepth[i] {
			out.WriteString(branchBlank)
		} else {
			out.WriteString(branchPipe)
		}
	}
	if last {
		out.WriteString(branchEnd)
	} else {
		out.WriteString(branchTee)
	}
	return out.String()
}

// hierarchyIndent returns the indentation of a depth without a branch glyph,
// which is the column the glyphs of that level stand in.
func hierarchyIndent(depth, width int) string {
	return strings.Repeat(" ", min(depth, hierarchyMaxIndentDepth(width))*hierarchyIndentPerLevel)
}

// hierarchyRowLines lays out one tree row: the structure prefix, the wrapped
// text and the child counter. The counter never wraps — it either fits at the
// end of the last text line or moves onto a continuation line of its own.
func hierarchyRowLines(row board.HierarchyRow, prefix string, width int) []relationLine {
	textWidth := hierarchyTextWidth(row.Depth, width)
	wrapped := wrapTitle(fmt.Sprintf("#%d [%s] %s", row.ID, row.Status, row.Title), textWidth, noLineLimit)
	continuation := strings.Repeat(" ", lipgloss.Width(prefix))

	lines := make([]relationLine, 0, len(wrapped)+1)
	for i, text := range wrapped {
		linePrefix := continuation
		if i == 0 {
			linePrefix = prefix
		}
		lines = append(lines, relationLine{prefix: linePrefix, text: text})
	}
	if row.Total == 0 {
		return lines
	}

	count := fmt.Sprintf(" (%d/%d done)", row.Done, row.Total)
	last := &lines[len(lines)-1]
	if lipgloss.Width(last.text)+lipgloss.Width(count) <= textWidth {
		last.count = count
		return lines
	}
	return append(lines, relationLine{prefix: continuation, count: count})
}

// appendHierarchy renders the hierarchy tree: the ancestor path of the open
// task, the task itself at its place and its descendants, in one reading order
// that cursor, hit test and hover all follow.
//
// A one-row tree is left out entirely — a "Hierarchy" section holding only the
// task whose title is the header of the same view says nothing.
func (b *Board) appendHierarchy(c *detailContent, tree board.HierarchyTree, width int) {
	if len(tree.Rows) == 0 || (len(tree.Rows) == 1 && !tree.CutAbove && !tree.CutBelow) {
		return
	}
	c.lines = append(c.lines, "")
	c.lines = append(c.lines, lipgloss.NewStyle().Bold(true).Render(hierarchyHeading))
	if tree.CutAbove {
		c.lines = append(c.lines, hierarchyEllipsisLine(0, width))
	}

	var lastAtDepth []bool
	deepest := 0
	for _, row := range tree.Rows {
		lastAtDepth = trackLastAtDepth(lastAtDepth, row.Depth, row.Last)
		ref := b.hierarchyRef(row, hierarchyBranch(lastAtDepth, row.Depth, width, row.Last), width)
		ref.startLine = len(c.lines)
		ref.lineCount = len(ref.lines)
		c.relations = append(c.relations, ref)
		for _, line := range ref.lines {
			c.lines = append(c.lines, line.plain())
		}
		deepest = max(deepest, row.Depth)
	}

	if tree.CutBelow {
		c.lines = append(c.lines, hierarchyEllipsisLine(deepest, width))
	}
}

// hierarchyRef turns a tree row into a relation block. The open task is never
// navigable: it is already on screen, so it is no cursor stop and no click
// target.
func (b *Board) hierarchyRef(row board.HierarchyRow, prefix string, width int) relationRef {
	return relationRef{
		taskID:    row.ID,
		navigable: !row.Current && b.relationVisible(row.ID),
		current:   row.Current,
		complete:  row.Total > 0 && row.Done == row.Total,
		lines:     hierarchyRowLines(row, prefix, width),
	}
}

// hierarchyEllipsisLine builds the cut-off marker for one side of the tree. It
// is no cursor stop, no click target and carries no hover, so it is dimmed here
// instead of at render time.
func hierarchyEllipsisLine(depth, width int) string {
	return relationGutter + hierarchyIndent(depth, width) + dimStyle.Render(hierarchyEllipsis)
}

// trackLastAtDepth records, per level, whether the row just placed there was the
// last shown sibling on it. Preorder guarantees a row's ancestors were recorded
// before the row itself.
func trackLastAtDepth(flags []bool, depth int, last bool) []bool {
	for len(flags) <= depth {
		flags = append(flags, false)
	}
	flags[depth] = last
	return flags
}
