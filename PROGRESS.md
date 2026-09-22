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

## Up Next

From `requirements.md` §7 "Future Phases":

- **Phase 4 — Data Safety & Sync**: undo stack, taskwarrior sync support.
- **Phase 5 — Priority logic**: priority-based coloring, priority sort
  mode, quick set-priority keys, urgency-based reordering.

Each phase's chunk list is a proposal only and needs fresh scoping/sign-off
before its first chunk starts, per requirements.md §5/§7 — nothing here is
to be assumed automatically.
