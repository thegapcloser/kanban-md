# All-motion mouse reporting for detail-view relation navigation

## Question

Relation rows in the TUI detail view were to become clickable links with a hover
affordance. Hover requires the terminal to report pointer movement while no
button is held, which the TUI did not request. The question was what enabling
that reporting mode changes elsewhere in the program, and what the extra event
volume costs per render.

## Method

- Inspected the pinned Bubble Tea v1.3.10 source for how the two mouse reporting
  modes are activated and how a buttonless motion event is parsed.
- Traced every place in `internal/tui` that reacts to `tea.MouseMsg`, and every
  test that asserts on the activation sequence or sends motion events.
- Measured the cost of one `View()` pass over a detail view with a 60-line
  Markdown body, with and without a memo on the rendered body.

## Findings

### Cell motion (1002) cannot express hover

`tea.WithMouseCellMotion()` emitted `\x1b[?1002h`, which asks the terminal for
button-press, button-release, and motion *while a button is held*. A pointer
move with no button down was never delivered, so hover feedback was not
expressible in that mode. `tea.WithMouseAllMotion()` emits `\x1b[?1003h`, which
adds exactly those buttonless moves. The two sequences are pinned in
`bubbletea@v1.3.10/screen_test.go:32` (1002) and `:37` (1003).

Bubble Tea reported a buttonless move as `Action == tea.MouseActionMotion` with
`Button == tea.MouseButtonNone`: `parseMouseButton` sets `MouseButtonNone` when
the button bits equal 3, and the motion bit then turns the action into
`MouseActionMotion`. That pair became the hover predicate `isHoverMotion`.

### Five side effects of the switch, and how each was closed

1. **The e2e activation assertion broke.** `e2e/tui_mouse_navigation_test.go`
   waited for the raw sequence `\x1b[?1002h` before driving the session. Against
   an all-motion program that wait ran into its timeout; the recorded output
   showed `\x1b[?1002h` was simply never sent. The assertion was moved to
   `\x1b[?1003h`, and a new e2e test asserts the same sequence before clicking a
   relation row.

2. **The error toast lost its lifetime.** `Update` cleared `b.err` on every
   `tea.MouseMsg`. Under all-motion reporting a plain pointer move would
   therefore have dismissed an error toast, where previously only real input
   did. The clear was made conditional on `!isHoverMotion(...)`, which restored
   the original semantics: the toast survives until the next actual key press,
   click, wheel event, or drag.

3. **Glamour rebuilt the renderer on every pointer move.** Bubble Tea calls
   `model.View()` once per message (`bubbletea@v1.3.10/tea.go:502`), and the
   detail view built a fresh `glamour.NewTermRenderer` on every render. A
   one-entry memo keyed on the complete input of the render function — body
   text, width, and background brightness — removed the rebuild. Because the key
   *is* the input of a pure function, the memo has no invalidation path and
   cannot go stale; a reload that changes the body on disk changes the key.

   Measured on an Apple M3 Max over a detail view with a 60-line Markdown body,
   200 renders each:

   | Render path | Time per `View()` |
   |---|---|
   | memoized | 31 µs |
   | rebuilt per render | 1.06 ms |

   Without the memo, every pointer move cost about a millisecond of renderer
   setup, which is why the memo was treated as part of the feature rather than
   as a later optimization.

4. **A held modifier froze the hover.** `handleMouse` discards any gesture while
   Shift, Alt, or Ctrl is held, and under all-motion reporting that guard also
   catches plain pointer moves: the underline stayed on the row the pointer had
   already left. The guard was extended to drop the remembered hover position as
   well, so the underline disappears instead of freezing. The same guard still
   discards a pending double-click candidate. That was left alone: it did the
   same for real clicks under 1002 reporting, and discarding every
   modifier-held gesture is the purpose of the guard.

5. **A buttonless move could drag the drop marker.** Drag handling itself is
   unchanged while a button is held: the terminal then reports motion *with*
   button bits, so `handleBoardMouse` and `updateDragTarget` keep seeing
   `tea.MouseButtonLeft`, and `drag_test.go` drives motion that way. Buttonless
   motion was a different matter. The first pass over this branch recorded it as
   already a no-op in both motion branches, which held only for the detail
   branch, where it returns early for hover. In the board branch it was a no-op
   only as long as no press was armed: with a press armed on a card, a
   buttonless move reached `updateDragTarget` and moved `dragStarted`,
   `destination`, and `destinationStatus` — a drop marker following a pointer
   that holds no button, reachable under 1003 reporting whenever a button
   release is lost. The board branch was given the same hover early return as the
   detail branch, so a buttonless move now touches the gesture fields in neither
   view.

### Navigability follows the `unfilteredTasks` invariant

Which relation rows could be navigable at all was decided by an existing
asymmetry in the two task collections the detail view reads. The parent row is
resolved against `b.allTasks`, which includes archived tasks, so an archived
parent renders with its title. Children are summarized from
`b.unfilteredTasks`, which excludes archived tasks but ignores the board search
and the hierarchy level filter.

`refreshDetailTask` looked up the open task in `b.unfilteredTasks` alone and
left the detail view when it was not there. Opening a task outside that set
would therefore have produced a view that closes itself on the next reload. The
same branch changed that exit into a step back through the relation history,
which falls back to closing only when no history is left; the lookup set stayed
the same.

The navigability rule was aligned with that invariant: a child row is navigable
exactly when its task is in `b.unfilteredTasks`, and a parent row additionally
requires that `board.FindParent` resolved it. The resolver discards a dangling
reference and a self-reference alike, and an unresolved row has no task to open,
so judging the parent ID stored on the task would have made a task pointing at
itself look like a link to itself. That makes every direct child navigable —
including one filtered off the board — and leaves all three non-openable parent
cases (a dangling reference, a self-reference, and an archived parent) as dimmed,
inert rows.
