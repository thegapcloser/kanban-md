package board

import (
	"sort"

	"github.com/antopolskiy/kanban-md/internal/config"
	"github.com/antopolskiy/kanban-md/internal/task"
)

// ChildTask is the stable, read-only child representation used by task-detail views.
type ChildTask struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	ChildRank *int   `json:"child_rank,omitempty"`
}

// ParentTask is the resolved, read-only parent representation used by task-detail views.
type ParentTask struct {
	ID     int
	Title  string
	Status string
}

// ChildSummary contains direct children and their terminal-status roll-up.
type ChildSummary struct {
	Children []ChildTask
	Done     int
}

// Total returns the number of direct children in the summary.
func (s ChildSummary) Total() int {
	return len(s.Children)
}

// FindParent resolves a task's direct parent, including an archived parent.
// Missing and defensive self-references return nil so renderers can fall back
// to the parent ID stored on the task.
func FindParent(tasks []*task.Task, current *task.Task) *ParentTask {
	if current.Parent == nil || *current.Parent == current.ID {
		return nil
	}
	for _, candidate := range tasks {
		if candidate.ID == *current.Parent {
			return &ParentTask{ID: candidate.ID, Title: candidate.Title, Status: candidate.Status}
		}
	}
	return nil
}

// SummarizeChildren returns direct children in child-rank order: ranked
// children ascending by rank first, then unranked children, each group broken
// by ascending task ID. Archived children are omitted unless includeArchived is
// true. Self-references are ignored defensively; normal CLI mutations reject
// them before writing.
func SummarizeChildren(
	tasks []*task.Task,
	parentID int,
	cfg *config.Config,
	includeArchived bool,
) ChildSummary {
	summary := ChildSummary{Children: make([]ChildTask, 0)}
	for _, candidate := range tasks {
		if candidate.ID == parentID || candidate.Parent == nil || *candidate.Parent != parentID {
			continue
		}
		if !includeArchived && cfg.IsArchivedStatus(candidate.Status) {
			continue
		}

		summary.Children = append(summary.Children, ChildTask{
			ID:        candidate.ID,
			Title:     candidate.Title,
			Status:    candidate.Status,
			ChildRank: candidate.ChildRank,
		})
		if cfg.IsTerminalStatus(candidate.Status) {
			summary.Done++
		}
	}

	sort.Slice(summary.Children, func(i, j int) bool {
		return lessByChildRank(summary.Children[i], summary.Children[j])
	})
	return summary
}

// lessByChildRank orders ranked children ahead of unranked ones and breaks
// equal ranks — and the unranked group — by ascending task ID.
func lessByChildRank(a, b ChildTask) bool {
	switch {
	case a.ChildRank != nil && b.ChildRank == nil:
		return true
	case a.ChildRank == nil && b.ChildRank != nil:
		return false
	case a.ChildRank != nil && *a.ChildRank != *b.ChildRank:
		return *a.ChildRank < *b.ChildRank
	default:
		return a.ID < b.ID
	}
}
