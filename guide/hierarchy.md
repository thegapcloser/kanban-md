# Parents, children and dependencies

Show full details of a task. When the task has direct children, the detail view
includes their IDs, statuses, titles, and a terminal/total roll-up. Parent
status remains independently managed; the roll-up is informational only.

```bash
kanban-md show ID
kanban-md show ID --archived  # include archived children in the roll-up
```

| Flag | Description |
|------|-------------|
| `--archived` | Include archived direct children (hidden by default) |

Children are ordered by child rank first and task ID second — see
[Child rank](#child-rank). Without any ranks that is plain task-ID order,
matching the default `list --parent` order.
Human-readable CLI and TUI detail views prefix them with `├─` and `└─` tree
guides so the parent-child relationship remains visually clear.
Tasks with a direct parent show an upward relation such as
`↑ Parent  #1 [in-progress] Parent title`; if the parent file is unavailable,
the relationship falls back to its stored task ID.
JSON output always contains a `children` array; compact output adds a
`children:DONE/TOTAL done` annotation only when children are present.

**Parent links stay acyclic.** `create` and `edit` reject a parent that would
close a ring, naming the chain it would form:

```
$ kanban-md edit 2 --parent 1
Error: parent would create a cycle (#2 → #1 → #2)
```

`depends_on` is checked the same way, since two tasks that depend on each other
can never become unblocked. Both checks run wherever the link is written, so
`create --parent`, `edit --parent` and `edit --add-dep` are all covered.

## Child rank

Task IDs follow the order tasks were created, which is rarely the order the work
should be read in. `child_rank` gives a parent's direct children an explicit
order without renumbering anything:

```bash
kanban-md create "Second epic" --parent 1 --child-rank 20
kanban-md edit 5 --child-rank 10          # move #5 to the front
kanban-md edit 5 --clear-child-rank       # back to task-ID order
kanban-md show 1                          # children render in rank order
```

Rules:

- `child_rank` is optional and positive, and requires a `parent`.
- Ranked children come first, ascending by rank; equal ranks fall back to task ID.
- Children without a rank follow the ranked ones, also by task ID.
- Leave gaps (`10, 20, 30`) so a later insert needs no renumbering.
- Clearing the parent clears the rank with it, since the sibling group is gone.
  Reparenting keeps it — pass a new `--child-rank` in the same `edit` to change it.
- The rank only affects how a parent's direct children are ordered in detail
  views. It does not touch priority, `pick`, WIP limits, or board sorting.

A representative board for trying the CLI and TUI behavior is available in
[`examples/issue-11-demo`](../examples/issue-11-demo/README.md).
