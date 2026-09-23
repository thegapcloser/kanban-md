---
name: kanban-md
description: >
  Use when the user mentions tasks, a kanban board, backlog, work items,
  priorities, blockers, standup, triage, sprint planning or flow metrics, or
  wants to create, list, move, edit, claim or delete tasks in a kanban-md
  board. Not for the multi-agent development loop with worktrees and merges
  (kanban-based-development).
contract: skill-authoring-v3
allowed-tools:
  - Bash(kanban-md *)
---

# kanban-md

## Purpose

Reads and changes a kanban-md board through its CLI. Each task is a Markdown
file with YAML frontmatter under `kanban/tasks/`. The skill decides which
command to run and how to call it safely; it never decides what the work is.

## Anchors

- **Command-Query Separation** — read with `board`, `list` and `show` first,
  then change state with exactly one targeted mutation per intent.
- **Lease** — a claim is a lock with a timeout. Take it atomically with `pick`
  or `--claim`, renew it while you work, and release it when you stop.

## Contract

- **Input:** the user's request and a board, found from the working directory
  or given with `--dir PATH`.
- **Output:** the requested board change or answer, made only through the CLI.
- **Consumer:** the user and every other agent that reads the same board.
- **Escalation:** a CLI error names its cause. Read it, then read
  `kanban-md <command> --help` for the exact flag contract. Without a board,
  run `kanban-md init --name NAME` only when the user asks for a new board.

## Steps

### Step 1 · Orient

The board at skill load:

!`kanban-md board --compact 2>/dev/null || echo 'No board found. Create one with: kanban-md init --name PROJECT_NAME'`

Read the board before you use a status or priority. Statuses and priorities
are board-specific; the defaults are `backlog, todo, in-progress, review, done`
and `low, medium, high, critical`.

| Question | Command |
|---|---|
| What is active, blocked, overdue? | `kanban-md board --compact` |
| Which tasks match? | `kanban-md list --compact [--status S] [--priority P] [--tag T] [--assignee A] [--parent ID] [--search TEXT] [--blocked \| --not-blocked \| --unblocked] [--sort FIELD] [-r] [-n N]` |
| What does one task say? | `kanban-md show ID` |
| What happened recently? | `kanban-md log --compact [--task ID] [--limit N]` |
| How does work flow? | `kanban-md metrics --compact` |
| Summary for `AGENTS.md`? | `kanban-md context [--write-to FILE]` |

`--not-blocked` hides tasks marked blocked. `--unblocked` shows tasks whose
dependencies are all done.

### Step 2 · Change the board

Pick the one command that matches the intent. `move`, `edit` and `delete`
take comma-separated IDs for bulk changes.

| Intent | Command |
|---|---|
| Create | `kanban-md create "TITLE" [--priority P] [--tags T1,T2] [--parent ID] [--depends-on ID] [--body "TEXT"] [--claim AGENT]` |
| Move | `kanban-md move ID STATUS` or `kanban-md move ID --next \| --prev` |
| Change fields | `kanban-md edit ID [--title T] [--priority P] [--assignee A] [--add-tag T] [--remove-tag T] [--due YYYY-MM-DD]` |
| Block or unblock | `kanban-md edit ID --block "REASON"` or `kanban-md edit ID --unblock` |
| Link | `kanban-md edit ID --parent ID [--child-rank N]` or `kanban-md edit ID --add-dep ID` |
| Archive | `kanban-md delete ID --yes` |

`move` sets `started` on the first move out of the initial status and
`completed` on a move to a terminal status. A given STATUS wins over
`--next`/`--prev`. `--next` fails at the last status and `--prev` at the first.
`--child-rank` orders siblings, lower first; use gaps like 10, 20, 30.

**Body edits use one mode per call:**

| Goal | Command |
|---|---|
| Replace one exact passage | `kanban-md edit ID --body-replace "OLD" --body-with "NEW"` |
| Replace several passages at once | repeat `--body-replace` and `--body-with` in pairs |
| Add a note | `kanban-md edit ID -a "NOTE" [-t]` |
| Rewrite the whole body | `kanban-md edit ID --body "TEXT"` |

`--body`, `--append-body` and `--body-replace` cannot be combined. To replace
and append, run `edit` twice. Each search passage must occur exactly once;
otherwise nothing is written.

### Step 3 · Coordinate on a shared board

Use claims whenever more than one agent can touch the board. A claim blocks
changes by others until the board's `claim_timeout` expires.

| Moment | Command |
|---|---|
| Session start | `kanban-md agent-name` once, then use the name as `AGENT` |
| Take the next task | `kanban-md pick --claim AGENT --status todo --move in-progress` |
| Progress note, renews the claim | `kanban-md edit ID -a "NOTE" -t --claim AGENT` |
| Park for review or user input | `kanban-md handoff ID --claim AGENT --note "NEXT STEP" [--block "REASON"] -t --release` |
| Resume a parked task | `kanban-md edit ID --unblock --claim AGENT` if it was blocked, then `kanban-md move ID in-progress --claim AGENT` |
| Finish | `kanban-md edit ID --release`, then `kanban-md move ID done` |

### Step 4 · Verify

Read the task again with `kanban-md show ID` after a change the user relies
on. A non-zero exit code means nothing was written for that task.

**Completion Criterion:** `show` or `list` returns the requested state.

## Guardrails

Role boundary: **this skill operates the board; it never decides scope,
priority or completion on its own.**

- Never edit files under `kanban/tasks/` by hand; the CLI keeps IDs, timestamps
  and the activity log consistent.
- Never hide a CLI exit code behind a pipe such as `| tail`; a failed edit then
  looks like a success.
- Never put backticks or `$(...)` inside double-quoted `--body`, `--append-body`
  or `--note` values; the shell executes them. Use single quotes or escape them.
- Never hardcode statuses or priorities; read them from `kanban-md board`.
- Use `--json` only to parse output in a program. For JSON fields, see
  [references/json-schemas.md](references/json-schemas.md).
