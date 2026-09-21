# lazytask — Progress & Delivery Notes

## Status Overview

**v1 (MVP) — COMPLETE.** See `git log` for the full per-chunk history.

**Phase 2 progress:**
- ✅ Focus newly-added task in list — after add-task submit, the list now
  selects the newly created task once the refresh completes.
- 🔜 General panel layout (grid + focus nav, status/tasks-tabs/projects/tags
  panels, details panel) — UX discussed and scoped into 6 chunks in
  requirements.md §7. This also covers what was previously listed as the
  separate "filter/search panel" and "project & tag side panels" items —
  they turned out to be one cohesive design, not three.
  - ✅ **Chunk 1 (panel grid scaffold + focus navigation) — DONE.**
    Two-column 50/50 grid (Status/Tasks/Projects/Tags stacked left,
    Details full-height right), number-key (`0`-`4`) and `Tab`/`Shift+Tab`
    focus cycling with lazygit-style border highlighting, shared
    `internal/ui/panel` styling helpers. Includes a follow-up visual pass:
    panel titles now embed inline in the top border (e.g.
    `╭──[1]-Status────╮`) instead of a separate content row, reclaiming a
    row of height per panel, and the Tasks panel shows its `[2]-Tasks`
    number-key hint consistently with the other panels.
  - 🔜 Chunk 2 (Status panel, key 1) is next, pending sign-off.

## Up Next

Next work comes from requirements.md §7 "Future Phases":

- **Phase 2 — Navigation & Discovery**: Chunk 1 of general panel layout is
  done (see above); Chunk 2 (Status panel, key 1) needs sign-off to start;
  popups for warnings/errors still needs a UX discussion before it can be
  chunked.
- **Phase 3 — Customization**: configurable keymaps via YAML, custom
  user-defined tasks/actions via YAML.
- **Phase 4 — Data Safety & Sync**: undo stack, taskwarrior sync support.

Each phase's chunk list is a proposal only and needs fresh scoping/sign-off
before its first chunk starts, per requirements.md §5/§7 — nothing here is
to be assumed automatically.

