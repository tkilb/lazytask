# lazytask — Progress & Delivery Notes

## Status Overview

**v1 (MVP) — COMPLETE.** See `git log` for the full per-chunk history.

**Phase 2 progress:**
- ✅ Focus newly-added task in list — after add-task submit, the list now
  selects the newly created task once the refresh completes.
- ✅ Popups for warnings and errors — new `internal/ui/popup` package
  renders bordered, colored overlay boxes on top of the preserved
  background (via `popup.Overlay`): `Box` for Info/Warning/Error severities
  (dismiss on any key), and `ConfirmBox` for yes/no confirmations, sharing a
  `renderBox` layout helper. Follow-up ad-hoc UI passes (direct
  instruction) applied the same popup styling elsewhere for consistency:
  - The add-task form is now a centered popup overlay instead of a
    full-screen panel — the grid stays visible behind it, matching the
    look/feel of the other popups.
  - The delete-confirmation prompt (previously inline status-bar text,
    `Delete task N "desc"? (y/n)`) is now an orange `ConfirmBox` overlay
    with a "Confirm" title and `(y) confirm  (n/esc) cancel` hint; the
    old `deleteBindings` status-bar entry was removed since it's no
    longer needed.
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
    - ✅ Follow-up layout refinement (ad-hoc, direct instruction) also
      done: Projects and Tags moved from two stacked rows into one row of
      two half-width panels at the bottom of the left column; row heights
      are now static instead of an even split (Status fixed at 1 content
      line, Tasks gets the largest share, Projects/Tags share the rest —
      tuned to ~67%/33% of the non-Status height per user feedback).
      Fixed two related bugs found along the way: `internal/ui/panel`'s
      `minHeight` clamp (3 → 1) was forcing Status to 3 lines instead of
      1, and `internal/ui/tasklist`'s `View()` now stretch-fills its
      assigned height instead of sizing purely from task count (it
      previously left a gap at the bottom of the left column). Tasks
      panel scrolling was explicitly deferred by the user as a follow-up
      feature (see "Up Next" below) — not implemented yet.
  - ✅ **Chunk 2 (Status panel, key 1) — DONE.** Status panel shows
    `#<id> [<status>] P:<project> T:<tags>` for whatever task is currently
    selected in Tasks (with `(none)` placeholders for empty project/tags),
    read live off the Tasks list's current selection on every render — no
    extra state to keep in sync.
  - 🔜 Chunk 3 (Tasks panel status tabs, Todo/Done/Deleted) is next,
    pending sign-off.

## Up Next

Next work comes from requirements.md §7 "Future Phases":

- **Phase 2 — Navigation & Discovery**: Chunks 1-2 of general panel layout
  are done (see above); Chunk 3 (Tasks panel status tabs) needs sign-off
  to start; popups for warnings/errors are done (see above); Tasks panel
  scrolling (deferred during the Chunk 1 layout refinement) still needs to
  be scoped/chunked.
- **Phase 3 — Customization**: configurable keymaps via YAML, custom
  user-defined tasks/actions via YAML.
- **Phase 4 — Data Safety & Sync**: undo stack, taskwarrior sync support.

Each phase's chunk list is a proposal only and needs fresh scoping/sign-off
before its first chunk starts, per requirements.md §5/§7 — nothing here is
to be assumed automatically.

