# lazytask — Progress & Delivery Notes

## Status Overview

| #   | Chunk                            | Status                    | Objective                                                                                                                                                                                                                                                                                                                                                                       |
| --- | -------------------------------- | ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | `chore/scaffold`                 | **Committed** (`88c7686`) | Base project scaffolding: go.mod, minimal Bubble Tea model, README, .gitignore.                                                                                                                                                                                                                                                                                                 |
| 2   | `internal/taskwarrior/client`    | **Committed** (`3015c2b`) | Thin wrapper around `task export` returning parsed `[]Task` (JSON decode only). Unit + integration tests. No UI.                                                                                                                                                                                                                                                                |
| 3   | `internal/taskwarrior/mutations` | **Committed** (`aa950b5`) | `Add`, `Done`, `Delete` wrapper functions over `task` CLI. Unit + integration tests. No UI.                                                                                                                                                                                                                                                                                     |
| 4   | `internal/ui/tasklist`           | **Committed**             | Bubble Tea model rendering task list panel (fixture-driven), lazygit-style bordered pane, selection navigation. No live taskwarrior wiring.                                                                                                                                                                                                                                     |
| 5   | `wire: list panel to real data`  | **Committed** (`fc35204`) | Connect chunk 2 client to chunk 4 panel on startup; `r` key refresh.                                                                                                                                                                                                                                                                                                            |
| 6   | `internal/ui/addform`            | **Committed** (`9c76e74`) | Input panel for new task description, wired to chunk 3 `Add`.                                                                                                                                                                                                                                                                                                                   |
| 7   | `feature: complete/delete`       | **Committed**             | Keybindings (`d` done, `x` delete + confirm) wired to chunk 3.                                                                                                                                                                                                                                                                                                                  |
| 8   | `internal/editor`                | **Committed** (`5933879`) | Helper to write task to temp file, launch `$EDITOR`, read back changes; unit tests.                                                                                                                                                                                                                                                                                             |
| 9   | `feature: edit task`             | **Committed** (`a0690ff`) | Wire chunk 8 into task list (`e` key), re-import edited fields.                                                                                                                                                                                                                                                                                                                 |
| 10  | `feature: markdown edit buffer`  | Planned                   | Replace chunk 9's JSON edit buffer with a structured key/value plain-text buffer (editable fields + `---` divider + read-only reference fields) for the `$EDITOR` flow.                                                                                                                                                                                                       |
| 11  | `polish: status bar & help`      | Planned                   | Lazygit-style bottom bar showing active keybindings for current panel.                                                                                                                                                                                                                                                                                                          |

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

## Up Next

### Chunk 10: `feature: markdown edit buffer`

- **Objective**: Replace the JSON edit buffer introduced in chunk 9 with a
  structured, human-friendly plain-text buffer for the `$EDITOR` edit flow
  (see requirements.md §7, chunk 10 for the full field layout).
- **Format**: key/value lines (`Field: value`), `Description` may span
  multiple lines. Editable fields (`Description`, `Project`, `Priority`,
  `Due`, `Tags`) appear first, followed by a `---` divider, followed by
  read-only reference fields (`ID`, `UUID`, `Status`, `Entry`, `Modified`,
  `End`, `Urgency`) shown for context but never re-imported.
- **Needs**: parser + serializer with unit tests (round-trip, embedded
  newlines, missing/blank fields, malformed input), swap-in for the
  existing JSON marshal/unmarshal calls added in chunk 9.

### Chunk 11: `polish: status bar & help`

- **Objective**: Lazygit-style bottom bar showing active keybindings for the current panel.

## Deferred Feature Requests (not scheduled as a chunk yet)

- **Focus newly-added task in list**: after add-task submission auto-refreshes
  the list (chunk 6), select/focus the just-created task instead of leaving
  the cursor wherever it was. Logged in requirements.md §8. Needs its own
  chunk (e.g. matching the created task's ID/UUID from `Add`'s return value
  against the refreshed `[]taskwarrior.Task` and setting the list cursor to
  it) — not implemented as part of chunk 6.
