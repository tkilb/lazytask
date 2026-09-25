# lazytask

[![Build](https://github.com/tkilb/lazytask/actions/workflows/build.yml/badge.svg)](https://github.com/tkilb/lazytask/actions/workflows/build.yml)

A TUI frontend for the [Taskwarrior](https://taskwarrior.org/) CLI, written in Go using [Bubble Tea](https://github.com/charmbracelet/bubbletea), with a [lazygit](https://github.com/jesseduffield/lazygit)-inspired panel layout.

> **Windows is not supported.** Not in CI, not in releases, not in the install script, not in `lazytask update`. This is intentional — see [Install](#install).

lazytask shells out to the `task` binary (`export`/`import`/filtered commands) rather than reading taskwarrior's data files directly, so it stays compatible with your existing taskwarrior setup and sync config.

## Requirements

- Go 1.22+
- [Taskwarrior](https://taskwarrior.org/) CLI (`task`)

## Install

Linux and macOS only — sorry, not sorry, Windoze isn't supported.

```bash
curl -fsSL https://raw.githubusercontent.com/tkilb/lazytask/main/scripts/install.sh | sh
```

This downloads the latest release binary for your OS/arch and installs it to `~/bin` (override with `INSTALL_DIR=/some/path`). Make sure that directory is on your `PATH`.

To upgrade later, just run:

```bash
lazytask update
```

This checks GitHub Releases for a newer version, downloads the matching binary for your OS/arch, and replaces the running executable in place. If you're already on the latest version, it tells you and exits without doing anything.

### Versioning

Releases currently use `0.0.X` (patch-only) versioning while the project stabilizes — see `make release` / `make release minor` / `make release major` in the [Makefile](Makefile) for how new versions get cut. This will move to a `v1.x` scheme once the feature set settles.

## Running

```bash
go run ./cmd/lazytask
```

## Panels

The screen is split into a two-column grid, cycled with number keys `0`-`4` or `Tab`/`Shift+Tab`:

| Key | Panel        | Description                                                                                                                                                                                   |
| --- | ------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `1` | **Status**   | Summary line (`#id [status] P:project T:tags`) for the task selected in Tasks.                                                                                                                |
| `2` | **Tasks**    | The task list, with `Todo`/`Done`/`Deleted` tabs (cycle with `[`/`]`). Sorted by taskwarrior's computed urgency, highest first. The Priority cell is colored (H = red, M = yellow, L = blue). |
| `3` | **Projects** | Distinct project names, with `(N)` pending-task counts and `(all)`/`(none)` entries; moving the cursor filters Tasks immediately.                                                             |
| `4` | **Tags**     | _(planned — not yet implemented)_                                                                                                                                                             |
| `0` | **Details**  | Full read-only detail view (ID, UUID, description, status, project, tags, priority, due, urgency, entry/modified/end) for the selected task.                                                  |

## Keybindings

**Global** (any panel focused):

| Key                 | Action                                                                                  |
| ------------------- | --------------------------------------------------------------------------------------- |
| `q` / `ctrl+c`      | Quit                                                                                    |
| `0`-`4`             | Focus a panel                                                                           |
| `Tab` / `Shift+Tab` | Cycle panel focus                                                                       |
| `a`                 | Open the add-task form (new task is auto-assigned to the active project filter, if any) |
| `u`                 | Undo the last undoable action (done/delete/restore/purge/priority change)               |
| `ctrl+r`            | Redo the last undone action                                                             |

**Tasks panel** (when focused):

| Key             | Action                                                                |
| --------------- | --------------------------------------------------------------------- |
| `↑/k` `↓/j`     | Move selection                                                        |
| `[` / `]`       | Switch Todo / Done / Deleted tab                                      |
| `d`             | Mark task done (confirm)                                              |
| `r`             | Reopen/restore a Done or Deleted task back to Todo (confirm)          |
| `x`             | Delete a task (Todo/Done) or permanently purge it (Deleted) (confirm) |
| `e`             | Edit task in `$EDITOR`                                                |
| `h` / `m` / `l` | Set the selected task's priority to High / Medium / Low               |

**Projects panel** (when focused):

| Key         | Action                                                                                                            |
| ----------- | ----------------------------------------------------------------------------------------------------------------- |
| `↑/k` `↓/j` | Select a project, auto-applies as the Tasks filter                                                                |
| `Shift+R`   | Rename the selected project (renames it on every task, any status; warns before merging into an existing project) |

All destructive/mutating actions (mark done, delete, purge, reopen, rename) show a confirmation popup — `y`/`enter` to confirm, `n`/`esc` to cancel.

## Testing

```bash
go test ./...
```

## Project status & roadmap

See `PROGRESS.md` for current status and `requirements.md` for the full chunked delivery process and upcoming work.
