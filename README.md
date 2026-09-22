# lazytask

A TUI frontend for the [Taskwarrior](https://taskwarrior.org/) CLI, written in Go using [Bubble Tea](https://github.com/charmbracelet/bubbletea), with a [lazygit](https://github.com/jesseduffield/lazygit)-inspired panel layout.

lazytask shells out to the `task` binary (`export`/`import`/filtered commands) rather than reading taskwarrior's data files directly, so it stays compatible with your existing taskwarrior setup and sync config.

## Requirements

- Go 1.22+
- [Taskwarrior](https://taskwarrior.org/) CLI (`task`)

## Running

```bash
go run ./cmd/lazytask
```

## Panels

The screen is split into a two-column grid, cycled with number keys `0`-`4` or `Tab`/`Shift+Tab`:

| Key | Panel | Description |
|-----|-------|-------------|
| `1` | **Status** | Summary line (`#id [status] P:project T:tags`) for the task selected in Tasks. |
| `2` | **Tasks** | The task list, with `Todo`/`Done`/`Deleted` tabs (cycle with `[`/`]`). |
| `3` | **Projects** | Distinct project names, with `(N)` pending-task counts and `(all)`/`(none)` entries; moving the cursor filters Tasks immediately. |
| `4` | **Tags** | *(planned — not yet implemented)* |
| `0` | **Details** | Full read-only detail view (ID, UUID, description, status, project, tags, priority, due, urgency, entry/modified/end) for the selected task. |

## Keybindings

**Global** (any panel focused):

| Key | Action |
|-----|--------|
| `q` / `ctrl+c` | Quit |
| `0`-`4` | Focus a panel |
| `Tab` / `Shift+Tab` | Cycle panel focus |
| `a` | Open the add-task form (new task is auto-assigned to the active project filter, if any) |

**Tasks panel** (when focused):

| Key | Action |
|-----|--------|
| `↑/k` `↓/j` | Move selection |
| `[` / `]` | Switch Todo / Done / Deleted tab |
| `d` | Mark task done (confirm) |
| `r` | Reopen/restore a Done or Deleted task back to Todo (confirm) |
| `x` | Delete a task (Todo/Done) or permanently purge it (Deleted) (confirm) |
| `e` | Edit task in `$EDITOR` |

**Projects panel** (when focused):

| Key | Action |
|-----|--------|
| `↑/k` `↓/j` | Select a project, auto-applies as the Tasks filter |
| `Shift+R` | Rename the selected project (renames it on every task, any status; warns before merging into an existing project) |

All destructive/mutating actions (mark done, delete, purge, reopen, rename) show a confirmation popup — `y`/`enter` to confirm, `n`/`esc` to cancel.

## Testing

```bash
go test ./...
```

## Project status & roadmap

See `PROGRESS.md` for current status and `requirements.md` for the full chunked delivery process and upcoming work.
