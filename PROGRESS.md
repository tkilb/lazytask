# lazytask — Progress & Delivery Notes

## Status Overview

**v1 (MVP) — COMPLETE.** See `git log` for full per-chunk history.

**Phase 2 (Navigation & Discovery)** — COMPLETE: panel grid + focus
nav, Status panel, Tasks panel status tabs (Todo/Done/Deleted), shared
filter state + Projects panel (with rename/counts), Details panel, popups
for warnings/errors/confirmations, global vs. local keybinding tiers with
Done/Deleted reopen/restore/purge, global add key with project auto-assign,
and Tasks/Projects panel scrolling with an "N of M" position footer.

**Phase 3 (Customization) — theming chunk DONE**: panel colors
(`internal/ui/panel`) now come from a small named `palette` block matching
lazygit's default theme (green focus border, blue tab highlight, default/
unstyled elsewhere) instead of raw color literals — no YAML config, kept
intentionally hardcoded. This also fixed the active-tab/focused-border
contrast bug. Tasks/Projects panel selected-row highlighting now uses the
same blue (`panel.SelectedRowBackground`) instead of a generic `Reverse`,
matching lazygit's `selectedLineBgColor`.

**Phase 4 (Data Safety & Sync) — undo stack chunk DONE**: multi-level,
in-memory undo/redo (`internal/undo.Stack`) wired into done/delete/
restore/purge on the Tasks panel (`u` = undo, `ctrl+r` = redo), with
purge's undo re-importing the pre-purge task snapshot. Not persisted
across restarts; add/edit mutations aren't covered yet.

**Phase 5 (Priority logic) — priority-based coloring chunk DONE**:
Tasks panel Priority cell is colored by priority (H = red, M = yellow,
L = blue, none/unrecognized = default) via `PriorityHighColor`/
`PriorityMediumColor`/`PriorityLowColor`/`PriorityLowDefaultColor` in
`internal/ui/panel/panel.go` and `priorityColor()`/`renderDataRow()` in
`internal/ui/tasklist/model.go`. Coloring is scoped to the Priority cell
only (not the whole row) to leave the Due column free for a possible
future overdue-date highlight.

**Phase 5 (Priority logic) — urgency-based default sort chunk DONE**:
Tasks panel is always sorted descending by taskwarrior's computed
`Task.Urgency` field via `sortByUrgency()` in
`internal/ui/tasklist/model.go`, applied in both `New()` and
`SetTasks()`. This replaces the originally-spec'd selectable/toggleable
"priority sort mode" (corrected in `requirements.md` as a mistaken
requirement) — no keybinding or toggle is needed since taskwarrior's
default urgency coefficients already weight priority heavily.

**Phase 5 (Priority logic) — quick set-priority keys chunk DONE**:
While the Tasks panel is focused, `h`/`m`/`l` set the selected task's
priority directly to H/M/L via `TaskPrioritizer.SetPriority()` (new
`internal/taskwarrior` client method, `task <id> modify priority:<P>`),
wired through `setPriorityTask()` in `cmd/lazytask/main.go`. There is no
"clear to none" key by design (per user decision, lazytask never treats
"no priority" as a target state). The resulting undo.Action restores the
task's prior priority (which could be empty, for tasks that had no
priority set outside lazytask). `pendingFocusID` keeps the cursor on the
same task after the refresh, since changing priority can move the task's
position in the urgency-sorted list. The remaining Phase 5 sub-feature
(urgency-based manual reordering) still has open design questions and is
not started. ("Urgency visibility" was dropped from scope — the Details
panel already displays taskwarrior's real `Urgency` field, and sorting by
it is already always-on, so there was nothing left to build.)

Noted as follow-ups from this chunk, not yet started or scoped:
- Default new tasks to Medium priority on creation (Add form).
- Normalize fuzzy priority input in add/edit forms (e.g.
  "medium"/"med"/"Med"/"m" → M, "l" → L, etc.).

## Up Next

From `requirements.md` §7 "Future Phases":

- **Phase 4 — Data Safety & Sync**: taskwarrior sync support remains open
  (undo stack chunk is done).
- **Phase 5 — Priority logic**: urgency-based manual reordering remains
  (priority-based coloring, urgency-based default sort, and quick
  set-priority keys chunks are done; "urgency visibility" was dropped —
  already satisfied by the existing Details panel field); still has open
  design questions needing sign-off before scoping its chunk.

Each phase's chunk list is a proposal only and needs fresh scoping/sign-off
before its first chunk starts, per requirements.md §5/§7 — nothing here is
to be assumed automatically.
