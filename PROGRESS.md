# lazytask — Progress & Delivery Notes

## Status Overview

| #   | Chunk                            | Status                    | Objective                                                                                                                                                               |
| --- | -------------------------------- | ------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | `chore/scaffold`                 | **Committed** (`88c7686`) | Base project scaffolding: go.mod, minimal Bubble Tea model, README, .gitignore.                                                                                         |
| 2   | `internal/taskwarrior/client`    | **Committed** (`3015c2b`) | Thin wrapper around `task export` returning parsed `[]Task` (JSON decode only). Unit + integration tests. No UI.                                                        |
| 3   | `internal/taskwarrior/mutations` | **Committed** (`aa950b5`) | `Add`, `Done`, `Delete` wrapper functions over `task` CLI. Unit + integration tests. No UI.                                                                             |
| 4   | `internal/ui/tasklist`           | **Committed**             | Bubble Tea model rendering task list panel (fixture-driven), lazygit-style bordered pane, selection navigation. No live taskwarrior wiring.                             |
| 5   | `wire: list panel to real data`  | **Committed** (`fc35204`) | Connect chunk 2 client to chunk 4 panel on startup; `r` key refresh.                                                                                                    |
| 6   | `internal/ui/addform`            | **Committed** (`9c76e74`) | Input panel for new task description, wired to chunk 3 `Add`.                                                                                                           |
| 7   | `feature: complete/delete`       | **Committed**             | Keybindings (`d` done, `x` delete + confirm) wired to chunk 3.                                                                                                          |
| 8   | `internal/editor`                | **Committed** (`5933879`) | Helper to write task to temp file, launch `$EDITOR`, read back changes; unit tests.                                                                                     |
| 9   | `feature: edit task`             | **Committed** (`a0690ff`) | Wire chunk 8 into task list (`e` key), re-import edited fields.                                                                                                         |
| 10  | `feature: markdown edit buffer`  | **Committed**             | Replace chunk 9's JSON edit buffer with a structured key/value plain-text buffer (editable fields + `---` divider + read-only reference fields) for the `$EDITOR` flow. |
| 11  | `polish: status bar & help`      | Planned                   | Lazygit-style bottom bar showing active keybindings for current panel.                                                                                                  |

---

## Notes

Detailed per-chunk objective/files/verification/manual-QA notes for chunks 1-9
have been trimmed from this file to save space; see each chunk's commit
message/diff in `git log` for full history. Key carried-forward decisions:

- `taskwarrior.Task` (chunk 2) is reused as the row type across `tasklist`,
  `addform`, and edit — no duplicate UI-local struct.
- Mutations (`Add`/`Done`/`Delete`/`Import`) match on UUID when available,
  falling back to numeric ID, since IDs renumber after completion/deletion.
- Successful add/done/delete/edit mutations always trigger an automatic
  list refresh (`fetchTasks`), regardless of UI mode state at message-arrival
  time (a source of a real bug in chunk 6, fixed by moving result-message
  handling to the top-level `Update`, unconditional on mode flags).
- Chunk 9's edit flow serializes the whole `Task` as indented JSON for the
  edit buffer and re-imports via `task import`; `tea.WithAltScreen()` is
  required in `tea.NewProgram` so resuming from the external editor doesn't
  leave stacked/duplicated panel artifacts on screen.
- Chunk 10 replaces the JSON edit buffer with `internal/editbuffer`
  (`Serialize`/`Parse`/`Apply`): a plain-text key/value format with editable
  fields (`Description`, `Project`, `Priority`, `Due`, `Tags`) above a `---`
  divider and read-only reference fields below it. `editTaskCallback` now
  takes the original `Task` as a closure argument and always reconstructs
  read-only fields (UUID, ID, Status, Entry, Modified, End, Urgency) from it
  rather than the parsed buffer, so edits to the read-only section are
  structurally impossible to apply, not merely validated away.

## Up Next

### Chunk 11: `polish: status bar & help`

- **Objective**: Lazygit-style bottom bar showing active keybindings for the current panel.

## Deferred Feature Requests (not scheduled as a chunk yet)

- **Focus newly-added task in list**: after add-task submission auto-refreshes
  the list (chunk 6), select/focus the just-created task instead of leaving
  the cursor wherever it was. Logged in requirements.md §8. Needs its own
  chunk (e.g. matching the created task's ID/UUID from `Add`'s return value
  against the refreshed `[]taskwarrior.Task` and setting the list cursor to
  it) — not implemented as part of chunk 6.
