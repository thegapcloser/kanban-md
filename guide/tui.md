# Interactive TUI

`kanban-md tui` opens a full interactive terminal board with keyboard navigation. It auto-refreshes when task files change on disk.
If no board exists in the current directory, `kanban-md tui` can initialize one and then offers to add that board directory to `.gitignore`.

```bash
kanban-md tui             # launch from any directory with a kanban/ board
kanban-md tui --dir PATH  # point to a specific kanban directory
kanban-md tui --hide-empty-columns  # override config and hide empty columns
kanban-md tui --show-empty-columns  # override config and show empty columns
kanban-md tui --mouse      # opt in to mouse navigation
kanban-md tui --narrow     # force the single-column layout at any width
```

Set `tui.hide_empty_columns` in `config.yml` to control the default behavior.

> **Note:** Older releases shipped a standalone `kanban-md-tui` binary. It has been retired — use `kanban-md tui` instead.

In create/edit dialogs, text fields support cursor-based editing (`←/→`, `Home/End`, `Backspace`, `Delete`).

Opening a task shows a `Hierarchy` block: the open task with its ancestor path
above it and its descendants below it, as one tree. A task the tree has nothing
to show around — no reachable parent, no children, nothing cut off — has no
block, because its own title is the header right above. Archived descendants
remain hidden in the TUI. A board search controls which cards are visible, but
does not hide anything from the tree of the selected task.

Task bodies are rendered as Markdown using the terminal's default foreground
for the main text, so they remain readable when a terminal switches between
light and dark themes while the TUI is running.

## The hierarchy tree

The detail view shows where the open task sits in the board, as one tree. At the
default of one level it reaches one step in each direction, and the `…` says that
the board goes on below:

```
Hierarchy
  └─ #1 [todo] Milestone One
     ├─ #2 [done] Epic Two (2/2 done)
     └─ #3 [backlog] Epic Three (0/1 done)
     …
```

The same board with `tui.hierarchy_levels: 2` reaches the stories, and the marker
is gone because nothing is cut off any more:

```
Hierarchy
  └─ #1 [todo] Milestone One
     ├─ #2 [done] Epic Two
     │  ├─ #4 [done] Story Four
     │  └─ #5 [done] Story Five
     └─ #3 [backlog] Epic Three
        └─ #6 [backlog] Story Six
```

`(x/y done)` counts direct, non-archived children in a terminal status over
direct, non-archived children — the same number `show` reports. It stands on a
row that does not show **all** of its counted children — one missing child is
enough. In the first example above, one level reaches the epics but not their
stories, so each epic carries the counter and the milestone does not, because
both its children are right below it. At two levels nothing is left to summarize
and no row carries a counter. An ancestor is the case in between: it shows the
one child that continues the path to your task and hides its siblings, so it
keeps the counter. On a board whose `parent` links form a cycle a row can keep
its counter although the child it counts is on screen as one of its own
ancestors; the rule looks below a row, not above it.

Three states say whether a row is a link. Plain text is a row you can open. **Bold**
text is the task you are looking at; it sits at its place in the tree and is not
a link to itself. Dimmed text is present but not openable: an archived ancestor,
which is shown and ends the chain there, and the `…` marker. A parent reference
that cannot be resolved — a dangling ID or a task pointing at itself — produces
no row at all; `show` still reports it.

The `[status]` of a finished task is green: the last column of your board,
whatever you call it. This is the same notion of "finished" that `(x/y done)`
counts by, but not the same rule — the counter drops archived children before it
looks at their status, so `[archived]` never reaches its numbers. The bracket stands on every row, so the signal does not depend on
having children. A dimmed row stays fully dimmed, `[archived]` included: an
archived ancestor is not finished, it is gone.

A `…` above the tree means the ancestor path continues past the level budget, a
`…` below it means a task on the deepest shown level still has children. Neither
is a link. On a board whose `parent` links form a cycle the lower marker can
appear although the cycle leaves nothing further to show.

## Relation navigation

Every tree row except the open task is a link. `Tab` and `Shift+Tab` walk a
cursor through them in reading order — ancestors from the outside in, then the
descendants — and wrap around at both ends. The cursor starts inactive, so a
freshly opened task still reads as plain text until the first `Tab`. `Enter`
opens the task under the cursor.

`Esc` and `Backspace` go one step back and restore the screen you left: same
task, same scroll position, same cursor row. With no history left they close the
detail view as before. `q` always closes the whole chain at once.

A row is only navigable when its task is active (not archived). Descendants are
always navigable, even when a search or a level filter hides them from the board.

Opening a relation leaves the board alone: search query, level filter and card
selection are all unchanged when the detail view closes.

## How deep the tree reaches

`tui.hierarchy_levels` is the one knob for both directions:

```bash
kanban-md config set tui.hierarchy_levels 2   # two levels up and two down
kanban-md config set tui.hierarchy_levels 0   # only the open task
```

Unset means 1 — one level up, one level down. `N` reaches N levels up **and** N
levels down, cut off wherever the tree ends. Depth comes from the parent chain
alone, so the setting works the same on a two-level board and on a six-level one;
there is no maximum and no notion of milestone, epic or story behind it.

`tui.hierarchy_levels` sets the depth of this tree and nothing else. The board
itself has its own use for the same depth — the `v` level filter and
`tui.level_colors`, see [Hierarchy levels](#hierarchy-levels) — and the two
settings are independent.

The `show` command keeps its parent line and children list unchanged. That
divergence is deliberate: the tree is a navigation aid for the TUI, and the CLI
output format is a contract for agents that read it.

## Narrow mode (small terminals)

On terminals too narrow to show every column side by side — a phone over SSH, a
split pane — the board automatically switches to a single-column layout. It shows
one column at a time, full width, under a two-line header: a tab strip of all
columns (the active one highlighted) on top, and the active column's own full
name, count, and WIP limit below. Card titles stay readable instead of being
crushed to a few characters per column.

Switch columns with `←`/`→`, `h`/`l`, or `Tab`/`Shift+Tab`. With `--mouse`, tap a
tab to jump straight to that column; tapping a card selects it as usual.

Narrow mode activates automatically once columns can no longer get a usable
width. To force or tune it:

```bash
kanban-md tui --narrow                         # force narrow mode for this run
kanban-md config set tui.narrow_threshold 80  # persist a custom trigger width
```

Set `tui.narrow_threshold` with `kanban-md config set` (or directly in
`config.yml`) to override the automatic trigger — the board goes narrow below
that terminal width. Use `0` for automatic behavior or `1` to effectively
disable narrow mode.

## Mouse mode

Mouse controls are opt-in, so the normal keyboard-only TUI remains unchanged.
Start mouse mode with:

```bash
kanban-md tui --mouse
```

| Mouse action | Result |
|--------------|--------|
| Click a card | Select the card and synchronize keyboard navigation |
| Double-click the same card within 500 ms | Open its detail view |
| Click `Back` | Go one step back in the relation history, or return to the board |
| Wheel over a column | Activate that column and move its selection one card |
| Wheel in a detail view | Scroll the task body three lines |
| Click a hierarchy row in a detail view | Open that task (single click) |
| Move the pointer over a hierarchy row | Underline it as a click target |
| Hold a card, drag to another visible column, and release | Move the task to that status |

The entire rendered destination column is a drop target, including its header,
cards, and visible empty area. Releasing over the source column or outside a
valid column cancels the drag. Keyboard shortcuts continue to work while mouse
mode is active, so both input styles can be mixed freely.

The board status line begins with the card count and `? help`, followed by the
optional mouse indicator and the remaining actions. Shortcut characters are
highlighted inside their action labels so the essential hints survive narrow
terminal widths.

Status moves made in the TUI preserve an existing task claim. If an unclaimed
task enters a `require_claim` status, the TUI automatically claims it using the
local hostname; that claim remains attached if the task later moves elsewhere.

Hover needs to see the pointer move with no button held, so `--mouse` enables
all-motion reporting (`1003`) rather than cell-motion reporting (`1002`). The
terminal then reports every pointer move inside the TUI, which makes the note on
native text selection below more relevant, not less.

Terminals commonly reserve a modifier such as Shift or Option/Alt to bypass
application mouse reporting for native text selection. The exact modifier is
terminal-dependent; use the terminal's normal selection shortcut or omit
`--mouse` when native selection is preferred.

## Hierarchy levels

A board that mixes milestones, epics and stories can be filtered to one level of
the tree at a time. The level comes from the `parent` chain alone — a task with
no parent is level 0, its children are level 1, and so on. With the common
milestone → epic → story layout that makes level 0 the milestones, level 1 the
epics, and level 2 the stories, without any extra field on the task.

This filter and `tui.level_colors` below act on the board's cards. How deep the
detail view's tree reaches is a separate setting,
[`tui.hierarchy_levels`](#how-deep-the-tree-reaches).

Press `v` to cycle the filter: all levels → level 0 only → level 1 only → … →
back to all levels. The cycle stops at the deepest level actually present on the
board. The status line lists the shortcut next to `sort` and doubles as the
indicator: `level[all]` while unfiltered, `level[1]` while showing level 1.

Card borders can also be colored by level, which keeps the tree readable while
no filter is active. This is off by default:

```bash
kanban-md config set tui.level_colors true
```

With it on, each level gets its own border color and the selected card is drawn
with a thick border instead of changing color, so its level color stays visible.
A blocked card always keeps its red warning color; selection adds the thick border.
Blocked cards keep their red border either way.

## Keyboard shortcuts

| Key | Action |
|-----|--------|
| `h` / `l` | Move between columns |
| `j` / `k` | Move between tasks within a column |
| `Enter` | View task details; in a detail view, open the relation under the cursor |
| `Tab` / `Shift+Tab` | In a detail view, move the relation cursor forward / backward |
| `Esc` / `Backspace` | In a detail view, go one step back in the relation history, or close it |
| `c` | Create task in current column |
| `e` | Edit selected task (same 4-step flow as create) |
| `E` | Open the selected task's Markdown file in `$VISUAL`, then `$EDITOR`, then `vi` when available |
| `m` | Move task to a different status (picker dialog) |
| `n` / `p` | Move task to next / previous status |
| `d` | Delete task (with confirmation) |
| `s` | Cycle the sort field (priority → created → updated → title) |
| `S` | Reverse the sort direction |
| `/` | Search/filter tasks live. By default matches a case-insensitive substring of the title. Start the query with `#` to search ticket IDs instead: `#12` matches every ID beginning with `12` (e.g. #12, #121), and a trailing space (`#12 `) requires an exact match (only #12). `Enter` keeps the filter, `Esc` clears it |
| `v` | Cycle the hierarchy level filter (all levels → level 0 → level 1 → … → all levels) |
| `r` | Refresh board |
| `?` | Show help |
| `q` / `Ctrl+C` | Quit; in a detail view, `q` closes the whole relation history |
