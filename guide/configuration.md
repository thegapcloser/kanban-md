# Configuration

kanban-md discovers its config by walking upward from the current directory, similar to how `git` finds `.git/`. This means you can run commands from any subdirectory in your project.

Use `--dir` to point to a specific board:

```bash
kanban-md --dir /path/to/kanban list
```

## config.yml

`kanban-md init --name "My Project"` writes this `config.yml`:

```yaml
version: 13
board:
    name: My Project
tasks_dir: tasks
statuses:
    - name: backlog
      show_duration: false
    - name: todo
    - name: in-progress
      require_claim: true
    - name: review
      require_claim: true
    - name: done
      show_duration: false
    - name: archived
      show_duration: false
priorities:
    - low
    - medium
    - high
    - critical
defaults:
    status: backlog
    priority: medium
    class: standard
claim_timeout: 1h
classes:
    - name: expedite
      wip_limit: 1
      bypass_column_wip: true
    - name: fixed-date
    - name: standard
    - name: intangible
tui:
    title_lines: 2
    age_thresholds:
        - after: 0s
          color: "242"
        - after: 1h
          color: "34"
        - after: 24h
          color: "226"
        - after: 72h
          color: "208"
        - after: 168h
          color: "196"
next_id: 1
```

`wip_limits` appears once you set limits, for example with `init --wip-limit in-progress:3`.

## Reading and changing values

```bash
kanban-md config                       # show all config values
kanban-md config get KEY               # get a single value
kanban-md config set KEY VALUE         # set a writable value
```

Available keys:

| Key | Writable | Description |
|-----|----------|-------------|
| `board.name` | yes | Board name |
| `board.description` | yes | Board description |
| `defaults.status` | yes | Default status for new tasks |
| `defaults.priority` | yes | Default priority for new tasks |
| `defaults.class` | yes | Default class of service for new tasks |
| `statuses` | no | List of statuses |
| `priorities` | no | List of priorities |
| `tasks_dir` | no | Tasks directory name |
| `wip_limits` | no | WIP limits per status |
| `claim_timeout` | yes | Claim expiration duration (e.g. `1h`, `30m`) |
| `classes` | no | Class of service definitions |
| `tui.title_lines` | yes | Number of title lines shown in TUI cards |
| `tui.hide_empty_columns` | yes | Hide columns with zero tasks in TUI |
| `tui.hierarchy_levels` | yes | Levels the TUI detail-view tree shows above and below the open task (unset = 1) |
| `tui.age_thresholds` | no | TUI age color thresholds |
| `next_id` | no | Next task ID |
| `version` | no | Config schema version |

## Custom statuses

Define your own workflow columns:

```bash
kanban-md init --statuses "open,in-progress,blocked,closed"
```

The order matters — it defines the progression for `move --next` and `move --prev`, and the sort order for `list --sort status`.

## Custom priorities

Edit `config.yml` directly to customize priorities:

```yaml
priorities:
  - trivial
  - normal
  - urgent
  - showstopper
defaults:
  priority: normal
```

Priority order runs from lowest to highest. `list --sort priority` shows the
highest configured priority first by default; use `--reverse` for lowest first.

## Output format

The default output format is **table** (human-readable). Use flags to switch:

```bash
# Default: table
kanban-md list --status todo

# Compact: one line per task, ideal for AI agents
kanban-md list --compact

# JSON: for scripting and piping
kanban-md list --json | jq '.[].title'
```

Set the `KANBAN_OUTPUT` environment variable to change the default: `json`, `table`, `compact`, or `oneline`.

Override priority: `--json`/`--table`/`--compact` flags > `KANBAN_OUTPUT` env var > table default.
