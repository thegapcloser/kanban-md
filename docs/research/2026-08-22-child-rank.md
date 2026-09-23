# Explicit ordering for a parent's direct children

## Question

`kanban-md show <parent>` lists direct children in task-ID order, which is
creation order. When the intended reading order of an epic's children differs
from the order they happened to be created in, there is no way to express it, so
the order lives in prose inside the parent's body and drifts from the board.
What is the smallest first-class mechanism that fixes this, and what must it
deliberately not touch?

## Method

- Read the current ordering implementation and the tests that pin it.
- Traced every consumer of the child list to find what an ordering change would
  reach: CLI table, compact and JSON output, and the TUI detail view.
- Checked the frontmatter compatibility rules in `AGENTS.md` against a new
  optional field.
- Surveyed open issues and merged PRs for existing ordering or rank work.

## Findings

- `internal/board/children.go::SummarizeChildren` sorts direct children strictly
  by ascending `Task.ID`, and `children_test.go` fixes that behavior.
- All three CLI formats and the TUI detail view go through that one function
  (`cmd/show.go:56`, `internal/tui/board.go:2249`), so ordering has a single
  seam. No separate sort exists downstream.
- `task.Task` has `Parent *int` but no ordering field, and neither `create` nor
  `edit` has a flag for one.
- No open issue covers child ordering. Issue #16 / PR #18 (preserving unknown
  frontmatter fields) is adjacent but solves a different problem: it keeps
  foreign keys alive, it does not give the tool an ordering concept.
- Per `AGENTS.md`, a new optional frontmatter field with `omitempty` is
  backward compatible on its own; the requirement is a fixture under
  `internal/task/testdata/compat/v1/tasks/` plus a compat test.
- Priority is not a substitute. Priority is a scheduling signal that feeds
  `list`, `pick` and class-of-service handling; overloading it to mean "third
  epic" would corrupt all of those.

## Conclusion

Add an optional `child_rank` field that orders a parent's direct children and
nothing else.

- `child_rank` is optional, positive, and requires `parent` — a rank orders a
  task among siblings, so without a sibling group it has no meaning and is
  rejected rather than stored silently.
- Ranked children sort ascending by rank; equal ranks and the unranked group
  both fall back to ascending task ID. Unranked children follow the ranked ones,
  so adopting the feature on an existing board is incremental: rank the few
  children whose position matters and leave the rest alone.
- Sparse ranks (10, 20, 30) are the documented convention so an insert never
  forces a renumbering pass.
- Clearing the parent clears the rank atomically, because the sibling group the
  rank refers to is gone. Reparenting keeps the rank; the same `edit` call can
  set a new one.
- The rank is confined to the ordering of a parent's direct children in
  detail views. It does not affect priority, `pick`, WIP limits, or board and
  list sorting.

`list --sort child-rank` and TUI editing of the rank are deliberately out of
scope: the CLI plus correct detail-view ordering delivers the whole use case,
and both additions can land later without changing the field contract.

## Naming

`child_rank` over `position` or `order`. The value is a property of the
child-to-parent edge, and the name says which relationship it sorts. `position`
reads like an absolute board coordinate, and `order` collides with the sort
options of `list`.

## File anchors

- `internal/board/children.go` — the single ordering seam.
- `internal/task/task.go` — frontmatter and JSON schema.
- `cmd/create.go`, `cmd/edit.go` — flag surface.
- `internal/board/mutate.go` — `validateDeps` is reached by both create and
  edit, so relation validation has one carrier.
