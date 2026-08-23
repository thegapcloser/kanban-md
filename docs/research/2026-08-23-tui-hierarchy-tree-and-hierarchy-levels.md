# TUI hierarchy tree and `tui.hierarchy_levels`

Research behind replacing the detail view's parent line and children block with
one tree, and behind the single knob that governs how far it reaches.

## 1. Render cost: index at load time, no render cache

The detail view is rendered per Bubble Tea message, and mouse reporting runs in
all-motion mode since the relation-navigation change — so every pointer movement
rebuilds the view. A tree of N rows resolved naively costs one full scan of the
task list plus a sort and an allocation **per row**.

Counted on a real board (219 task files, 202 non-archived, maximum parent-chain
depth 2, 24 roots, widest fan-out 33 children):

| Variant | Candidate comparisons per render |
|---|---|
| One children list (the old detail view) | 202 |
| Naive tree, `levels: 1`, worst-case node | 6060 (30 rows) |
| Naive tree, `levels: 2` and deeper, worst-case node | 15150 (75 rows) |
| Tree through the load-path index | 75 map lookups |

### Measured, not extrapolated

`BenchmarkHierarchyTree` in `internal/board/hierarchy_test.go` runs both variants
against a generated board of the same shape (219 tasks, 24 roots, three levels,
fan-out 33, every 13th task archived), building the tree of the widest root with
`levels: 6` — 86 rows. `go test ./internal/board/ -run XXX -bench Hierarchy
-benchmem -benchtime=2s -count=3`, Apple M3 Max, Go 1.27:

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `HierarchyTree/index` | 9737 / 9796 / 9827 | 38784 | 144 |
| `HierarchyTree/naive` | 19181 / 19184 / 18973 | 19648 | 292 |
| `NewHierarchyIndex` | 9819 / 9963 / 9831 | 21792 | 349 |

**The operation count overstated the wall-clock advantage by two orders of
magnitude.** 200× fewer comparisons buy a factor of **1.96** in time, because a
linear scan over 219 pointers is cache-friendly while map lookups and the
per-row slices of the tree walk are not. The index also allocates more bytes per
tree than the naive walk does — 38784 B/op against 19648 B/op, because the
descent materializes a row and a child slice per node — and pays a further
~10 µs once per load.

The two arms are not the same work, and the difference is worth naming: the naive
reference only counts rows. It builds no `HierarchyRow`, walks no ancestor chain
and answers no `cutBelow`, all of which the indexed arm does. A naive
implementation of the same feature would therefore be slower than the arm
measured here, so the bias runs against the index and the factor of 1.96 is a
lower bound.

The index stays because it halves the per-frame cost of the thing that runs per
mouse movement, next to the memoized body render the previous round measured at
31 µs (`2026-08-23-mouse-all-motion-and-relation-navigation.md`). The decision
rule this run set itself before measuring was that a naive build under 10 µs
would make the index unnecessary; the naive build measured ~19 µs.

It does **not** save the load path a pass, as the change first claimed, and it
saves no map either. The base built one by-ID map and walked the tasks once each:
two passes, two maps counting the depth map. The index builds the by-ID map,
buckets the children, sorts every bucket, and `Depths` then walks the tasks a
third time — one pass more, one map more and the sorts. What the shared by-ID map
avoids is a *fourth* pass and map: an index bolted on top of the old `Depths`
would have built the by-ID map twice per load. Against the base, the load path
got more expensive, and the whole gain is on the render side.

A render memo over the built tree was rejected: it would be a second cache key
with its own staleness for a problem the index solves at the root. The index has
exactly one build site, `Board.rebuildHierarchyIndex`, which is what makes
"changed `allTasks` outside `loadTasks`" a single call and not a search.

## 2. One cycle rule for both directions

`internal/board/hierarchy.go` had one rule for walking up: a missing parent and a
self-reference both count as no parent, matching `FindParent`, and a chain is cut
at its first repeated task. The descent uses the same rule against the set of
tasks already on the rendered path, and `resolveParent` is reused unchanged —
it already takes the by-ID map the index holds and already folds "no parent",
"self-reference" and "parent gone" into one answer.

`resolveDepth` is not reusable: it collects a chain upwards in order to write
depths. Its `onChain` set is the model for the descent's path set, and the
walker's ascent is structurally the same loop.

"Cut at the first repeated task" means **no row**, symmetrical to the ancestor
side. The per-row counter still counts a cyclic child, because the counter means
"direct children" and nothing else. On a cyclic board that asymmetry is
deliberate and has a test.

## 3. Indentation is capped, not the tree

Available text width is `width − 2 (cursor gutter) − prefix`. Once fewer than
**20** cells are left for text, indentation stops growing and deeper levels
render on the column of the last level that fit.

The only existing minimum-width anchor is `minUsableColumnWidth = 14`
(`internal/tui/board.go`), and it does not transfer: it bounds a card column that
shows a title alone, while a tree row spends about 19 cells on `#123
[in-progress] ` before the first letter of a title appears. `NarrowThreshold` is
a terminal-width switch, not a text width. Hence `minHierarchyTextWidth = 20` as
its own constant.

Below the point where even level 0 fits, indentation is 0 and the text width is
clamped to at least 1 — `wrapTitle` with a non-positive width produces empty
lines. Widths 30, 10 and 3 are tested: no panic, no blank row, finite row count.

## 4. One `…` per side

A cut-off tree gets exactly one marker per side: above on the indentation of the
outermost shown ancestor, below on the indentation of the deepest shown level.
Neither is a cursor stop, a click target or a hover target.

The alternative — one `…` per truncated node — would have produced 33 identical
lines under the widest node of the real board, all saying what the `(x/y done)`
next to them already says. A marker per side is a statement about the boundary of
the tree; the node-level information is the counter.

The marker above is set when the level budget cut the chain, not when the walk
reached the end of it: a broken or repeated ancestor has nothing more behind it,
and an archived one is itself the last shown row, so the end is visible in the
tree either way. Whether the ancestor beyond the budget is archived makes no
difference — that row is exactly what got cut off. The marker below is set when a
node on the deepest shown level still has a non-archived child, which also covers
`levels: 0` on a task that has children.

## 5. Three states, one segment

Every decoration — dim, bold, green, underline — applies to the text span from
the `#` onwards. Structure (cursor gutter, indentation, `│`, `├─`/`└─`) is never
decorated, because the click target is the node and not the frame around it.

That makes three states directly comparable on the same range of cells:

| State | Meaning |
|---|---|
| plain | navigable |
| dim | present, not openable (archived ancestor, `…`) |
| bold | the open task |

Styles are **composed, not nested**. The previous renderer called
`relationHoverStyle.Render(dimStyle.Render(line))`, style inside style: the inner
reset ends the outer attribute, so the reach of an underline depended on the row
state, and the hover test could only prove that `\x1b[4m` appeared somewhere. The
segment rule builds one `lipgloss.Style` from the attributes that apply and
renders each segment once. `relationHoverStyle` lost its last user and is gone.

The counter is its own segment, which is what lets it carry a second color inside
an underlined row: green (`42`, the tone that already means "done and valid" in
`dropTargetColumnHeaderStyle`) when all direct children are done, and dim when
the row is dim — green never wins against dim.

Golden files run under `termenv.Ascii` and therefore cannot prove color or bold.
Every color, bold and underline assertion lives in ANSI256 tests against SGR
sequences, never against the renderer's own style variable. Three deliberate
mutations were run to confirm those assertions bite: decorating the structure as
well, nesting the counter style instead of composing it, and dropping the green.
Each was caught.

## 6. The CLI does not follow, on purpose

`show` keeps its parent line and its `Children (x/y done)` block. The tree is a
navigation aid, and nobody navigates a one-shot CLI output; on top of that
`show --json`/`--compact` is a contract for agents.

Five places carry the CLI form of the same statement and all five are unchanged:

- `internal/output/table.go` — the parent line and the children block including
  `├─`/`└─`
- `internal/output/table.go` — its own copy of the parent-line format
- `cmd/show.go` — the last remaining callers of `board.FindParent` and
  `board.SummarizeChildren`
- `internal/output/table_test.go` and `e2e/show_test.go` — the tests that pin the
  format
- `README.md`, `show` section — the documented output

## 7. The `↑ Parent  #999` line is gone from the TUI

A parent reference that cannot be resolved used to render as `↑ Parent  #999`,
dimmed and inert. In the tree an unresolvable reference ends the chain **without
a row**, which for a task with no descendants leaves a one-row tree, which is not
shown at all. So the broken reference is no longer visible in the TUI.

That is a deliberate consequence, not an oversight: the row said nothing a user
could act on, and `show` still reports the stored parent ID. The tests that
pinned the old line were rewritten to pin the new behavior — that a dangling or
self-referencing parent produces no hierarchy block, no click target and no
cursor stop.

## 8. Config

`tui.hierarchy_levels` is a `*int`. Only a pointer keeps "unset" (= 1 = today's
behavior) apart from an explicit 0 ("only the open task"); an `int` with 0 as
"unset" would have made "only the open task" inexpressible. With `omitempty` the
key stays out of every existing and newly written `config.yml`, and `NewDefault`
does not set it.

Schema version 12 → 13 with `migrateV12ToV13`, a v12 fixture and a compat test,
per the backward-compatibility rules in `AGENTS.md`. The migration leaves the
field unset.

Depth comes from the parent chain alone; the words milestone, epic and story do
not appear in the walker. There is no maximum, and a negative value is a
validation error.
