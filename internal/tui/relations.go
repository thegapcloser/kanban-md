package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/antopolskiy/kanban-md/internal/board"
	"github.com/antopolskiy/kanban-md/internal/task"
)

func (b *Board) viewDetail() string {
	t := b.detailTask
	if t == nil {
		return "No task selected."
	}

	lines := b.detailLines(t)

	// Reserve space for the blank separator line and the fixed status hint.
	viewHeight := b.height - 2 //nolint:mnd // 2 = blank line + hint line
	if viewHeight < 1 {
		viewHeight = len(lines)
	}

	// Build the status hint (always visible at bottom).
	hint := "q/esc:back"
	if len(lines) > viewHeight {
		hint += "  j/k:scroll  g/G:top/bottom"
	}

	// Apply viewport scrolling and clamp stored offset so subsequent key
	// presses start from the correct position (prevents overshoot past end).
	off := b.detailScrollOff
	maxOff := len(lines) - viewHeight
	if maxOff < 0 {
		maxOff = 0
	}
	if off > maxOff {
		off = maxOff
		b.detailScrollOff = off
	}

	end := off + viewHeight
	if end > len(lines) {
		end = len(lines)
	}

	visible := strings.Join(lines[off:end], "\n")
	if !b.mouseEnabled {
		return visible + "\n\n" + dimStyle.Render(hint)
	}

	hint = "← Back  " + hint
	renderedLines := end - off
	if renderedLines < viewHeight {
		visible += strings.Repeat("\n", viewHeight-renderedLines)
	}
	backWidth := min(lipgloss.Width("← Back"), b.width)
	if backWidth > 0 && b.height > 0 {
		b.layout.back = &backTarget{
			rect: rect{x0: 0, y0: b.height - 1, x1: backWidth, y1: b.height},
		}
	}
	return visible + "\n\n" + dimStyle.Render(truncate(hint, b.width))
}

func (b *Board) detailLines(t *task.Task) []string {
	parent := board.FindParent(b.allTasks, t)
	children := board.SummarizeChildren(b.unfilteredTasks, t.ID, b.cfg, false)
	return buildDetailContent(t, parent, children, b.width)
}

func buildDetailContent(
	t *task.Task,
	parent *board.ParentTask,
	children board.ChildSummary,
	width int,
) []string {
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
	if t.Parent != nil {
		lines = append(lines, "")
		lines = append(lines, wrapTitle(parentRelationLine(*t.Parent, parent), width, noLineLimit)...)
	}
	if children.Total() > 0 {
		lines = append(lines, "")
		heading := fmt.Sprintf("Children (%d/%d done)", children.Done, children.Total())
		lines = append(lines, lipgloss.NewStyle().Bold(true).Render(heading))
		for i, child := range children.Children {
			branch := "├─"
			if i == len(children.Children)-1 {
				branch = "└─"
			}
			line := fmt.Sprintf("%s #%d [%s] %s", branch, child.ID, child.Status, child.Title)
			lines = append(lines, wrapTitle(line, width, noLineLimit)...)
		}
	}
	if t.Body != "" {
		lines = append(lines, "")
		body := unescapeBody(t.Body)
		rendered := renderMarkdown(body, width)
		lines = append(lines, strings.Split(rendered, "\n")...)
	}
	return lines
}

func parentRelationLine(parentID int, parent *board.ParentTask) string {
	if parent == nil {
		return fmt.Sprintf("↑ Parent  #%d", parentID)
	}
	return fmt.Sprintf("↑ Parent  #%d [%s] %s", parent.ID, parent.Status, parent.Title)
}
