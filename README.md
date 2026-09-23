# kanban-md

[![CI](https://github.com/thegapcloser/kanban-md/actions/workflows/build.yml/badge.svg)](https://github.com/thegapcloser/kanban-md/actions/workflows/build.yml)
[![Release](https://github.com/thegapcloser/kanban-md/actions/workflows/release.yml/badge.svg)](https://github.com/thegapcloser/kanban-md/actions/workflows/release.yml)
[![Latest Release](https://img.shields.io/github/v/release/thegapcloser/kanban-md)](https://github.com/thegapcloser/kanban-md/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/thegapcloser/kanban-md.svg)](https://pkg.go.dev/github.com/thegapcloser/kanban-md)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A file-based Kanban board for AI agents and the humans who supervise them.
Several agents can work the same board in parallel without clashing. One
static binary, agent skills included, no database, no server: every task is a
Markdown file.

![Demo](assets/demo.gif)

## Why kanban-md?

- **Agents first.** `--compact` output saves tokens, `pick --claim` takes the
  next task atomically, and the bundled skills teach agents to use the board.
- **Safe with many agents.** A claim locks a task for one agent until it is
  released or times out.
- **Plain files.** Agents, humans, scripts and `grep` read the same Markdown.
  No API tokens, no rate limits.
- **Self-healing IDs.** Duplicate IDs, filename mismatches and `next_id` drift
  are repaired before a command runs.
- **A TUI to watch.** `kanban-md tui` shows the board live and refreshes when
  files change.

![Interactive TUI](assets/tui-demo.gif)

## Install

```bash
go install github.com/thegapcloser/kanban-md/cmd/kanban-md@latest
```

Pre-built binaries for macOS, Linux and Windows are on the
[Releases](https://github.com/thegapcloser/kanban-md/releases/latest) page.

## Quick start

```bash
kanban-md init --name "My Project"   # creates kanban/ and offers to add it to .gitignore
kanban-md skill install              # installs the agent skills for this project
kanban-md create "Fix login bug" --priority critical
kanban-md tui                        # watch the board
```

From here, agents usually run the commands. A typical agent loop:

```bash
AGENT=$(kanban-md agent-name)
kanban-md pick --claim "$AGENT" --status todo --move in-progress
kanban-md edit 1 -a "Tests pass, opening PR." -t --claim "$AGENT"
kanban-md handoff 1 --claim "$AGENT" --note "Ready for review" -t --release
```

Start the `kanban-based-development` skill in one agent to see the full loop
with claims, worktrees and merges. Then start it in several agents at once.

## How it works

`kanban-md init` creates a `kanban/` directory:

```
kanban/
  config.yml
  tasks/
    001-set-up-ci-pipeline.md
    002-write-api-docs.md
```

Each task is Markdown with YAML frontmatter:

```markdown
---
id: 1
title: Set up CI pipeline
status: backlog
priority: high
created: 2026-02-07T10:30:00Z
updated: 2026-02-07T10:30:00Z
tags:
  - devops
---

Optional body with more detail, context, or notes.
```

Commands find the board by walking up from the current directory, like `git`
finds `.git/`. Use `--dir PATH` to point at another board. The full
`config.yml`, every config key and custom statuses and priorities are in
[guide/configuration.md](guide/configuration.md).

## Commands

Every command documents its flags and flag rules in `kanban-md <command> --help`.

| Command | Purpose |
|---|---|
| `init` | Create a board |
| `create` (`add`) | Create a task |
| `list` (`ls`) | Filter, search, sort and group tasks |
| `show` | Show one task with its body, parent and children |
| `edit` | Change fields, body, links, blocked state or claim |
| `move` | Change status, directly or with `--next`/`--prev` |
| `pick` | Claim the next available task atomically |
| `handoff` | Move a task to review with a note, optionally block and release |
| `archive`, `delete` (`rm`) | Soft-delete a task into `archived` |
| `board` (`summary`) | Counts per status, WIP, blocked and overdue tasks |
| `metrics` | Throughput, lead and cycle time, flow efficiency, aging work |
| `log` | Activity log of board changes |
| `context` | Board summary for `AGENTS.md` or `CLAUDE.md` |
| `config` | Read or change board settings |
| `agent-name` | Random two-word name for claims |
| `skill` | Install, check and update the agent skills |
| `tui` | Interactive terminal board |
| `completion` | Shell completion for bash, zsh, fish and PowerShell |

Output is a table by default. `--compact` prints one line per record and is
the best format for agents. `--json` is for scripts. Set `KANBAN_OUTPUT` to
change the default.

Parents, children, child rank and dependencies, including the cycle check,
are described in [guide/hierarchy.md](guide/hierarchy.md).

## Working with several agents

### Claims

An agent claims a task before it works on it. Other agents cannot pick or
change a claimed task until the claim is released or `claim_timeout` expires
(1 hour by default). Statuses with `require_claim: true`, by default
`in-progress` and `review`, reject any `move` or `edit` without `--claim`.

```bash
kanban-md pick --claim agent-1 --move in-progress   # take the next task
kanban-md move 5 review --claim agent-1             # claim required here
kanban-md edit 5 --release && kanban-md move 5 done # finish
```

On Unix-like systems, a claimed task file is read-only. The claimant's
commands unlock it for the update and lock it again. This stops accidental
edits by another process running as the same user. It is not a security
boundary: that user can still change permissions, rename or delete the file.

### Classes of service and pick order

`pick` chooses from unclaimed, unblocked tasks whose dependencies are done. It
orders by class of service first, then by priority. Fixed-date tasks are also
sorted by due date.

| Class | Behavior |
|---|---|
| `expedite` | Picked first. Bypasses column WIP limits and has its own board-wide limit (default 1). |
| `fixed-date` | Picked by earliest due date within its priority. |
| `standard` | Default. Normal WIP and priority rules. |
| `intangible` | Picked last. For background work. |

`board --group-by` and `list --group-by` group tasks by assignee, tag, class,
priority or status.

## Agent skills

| Skill | Use |
|---|---|
| `kanban-md` | Which command fits which intent, claims on shared boards, safe body edits and shell pitfalls |
| `kanban-based-development` | Autonomous development loop with claims, git worktrees and a strict status lifecycle |

```bash
kanban-md skill install            # for all detected agents in this project
kanban-md skill install --global   # in your home directory
kanban-md skill check              # are the installed skills current?
kanban-md skill update             # bring them in line with this binary
```

Skills carry the CLI version. After an upgrade, `skill check` reports outdated
copies and `skill update` replaces them.

## Interactive TUI

`kanban-md tui` opens the board with keyboard navigation, create and edit
dialogs, search, a hierarchy tree in the detail view, optional mouse support
and a narrow layout for small terminals. Press `?` for the key bindings. Details
are in [guide/tui.md](guide/tui.md).

## Design principles

- **Agent first, human friendly.** Every feature works non-interactively and in
  pipes first. Humans get the TUI and tables.
- **Files are the format.** The CLI is a layer over plain files. Use the CLI to
  change tasks, because it keeps IDs, timestamps, claims and the activity log
  consistent.
- **No hidden state.** Everything lives in `config.yml` and the task files. Git
  handles collaboration.
- **Minimal.** The tool manages task files. It does not sync, notify or call
  external services.

## Development

```bash
make build      # build dist/kanban-md
make test       # unit and e2e tests
make lint       # golangci-lint
make all        # full pipeline
```

CI pins Go 1.25 and golangci-lint v2.10.1.

## Origin and license

This project continues [antopolskiy/kanban-md](https://github.com/antopolskiy/kanban-md)
as an independent project. [MIT](LICENSE).
