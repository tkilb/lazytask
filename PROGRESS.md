# lazytask — Progress & Delivery Notes

## Status Overview

**v1 (MVP) — COMPLETE.** See `git log` for full per-chunk history.

**Phase 2 (Navigation & Discovery)** — mostly complete: panel grid + focus
nav, Status panel, Tasks panel status tabs (Todo/Done/Deleted), shared
filter state + Projects panel (with rename/counts), Details panel, popups
for warnings/errors/confirmations, global vs. local keybinding tiers with
Done/Deleted reopen/restore/purge, global add key with project auto-assign.
See `requirements.md` §7 for what's still open.

## Up Next

From `requirements.md` §7 "Future Phases":

- **Phase 2 remainder**: Tags panel (key `4`, needs sign-off); Tasks panel
  scrolling (currently overflows unclipped when the list is taller than
  the panel).
- **Phase 3 — Customization**: theming/configurable colors (needs a YAML
  schema decision), configurable keymaps via YAML, custom user-defined
  tasks/actions via YAML.
- **Phase 4 — Data Safety & Sync**: undo stack, taskwarrior sync support.

Each phase's chunk list is a proposal only and needs fresh scoping/sign-off
before its first chunk starts, per requirements.md §5/§7 — nothing here is
to be assumed automatically.
