# lazytask — Chunked Delivery Requirements

Status: v1 (MVP) is complete. This doc now covers ongoing process (Sections
1-6) and upcoming work (Section 7) for a driving agent ("driver") that
breaks work into small, independently-committable worker tasks, delivered
**one chunk at a time with a mandatory human checkpoint between chunks.**

This is explicitly **not** meant to run unattended. There is no
"orchestrator" that dispatches chunk after chunk on its own — a human must
review and approve after every single chunk before the next one starts.

## 1. Project Summary

A TUI frontend for the [taskwarrior](https://taskwarrior.org/) CLI, written in
Go using [Bubble Tea](https://github.com/charmbracelet/bubbletea), with a
[lazygit](https://github.com/jesseduffield/lazygit)-inspired panel layout
(multiple bordered panes, keyboard-driven navigation, contextual
action bar/status line).

## 2. Tech Stack

- Language: Go 1.22
- Module path: `github.com/tkilb/lazytask`
- TUI framework: Bubble Tea (+ Bubbles components, Lip Gloss for styling)
- Taskwarrior integration: **shell out** to the `task` binary, using
  `task export` / `task import` and `task <filter> <command>` invocations.
  No direct reads of taskwarrior's data files.
- Config format: YAML (`gopkg.in/yaml.v3`)
- Testing: standard `testing` package + `stretchr/testify` (assertions only,
  no mocking framework unless a worker task demonstrates a clear need)

## 3. MVP (v1) — COMPLETE

Work now moves to the **Future Phases** in Section 7. Nothing from Section 7
may be started without explicit human sign-off, per Section 5.

## 4. Testing Bar (per chunk)

Every chunk must ship with:

- **Unit tests** for any pure logic (parsing, model/state transitions,
  config loading) — table-driven, per Go convention.
- **Integration tests** that exercise the real `task` CLI against an
  isolated taskwarrior data directory (set `TASKDATA`/`TASKRC` to a temp dir
  per test, never touch the developer's real task data). Skip gracefully
  (`t.Skip`) if `task` is not installed in the environment.
- A short **manual QA note** in the chunk's PR/commit description telling
  the user exactly what to run and look for (e.g., "run `go run ./cmd/lazytask`,
  press `a`, type a description, confirm it appears in the list").

## 5. Human-in-the-Loop Checkpoints (mandatory, non-negotiable)

The driving agent must treat every chunk in Section 6 as a **hard stop
boundary**. The process for each chunk is:

1. **Announce before starting** — state which single chunk (by name/number)
   is about to be worked on and its stated objective, matching Section 6
   exactly. Do not start work on a chunk that wasn't just approved.
2. **Do the work for that one chunk only** — no peeking ahead, no
   pre-building later chunks "while we're at it."
3. **Stop after the chunk is done.** Do not commit, and do not start the
   next chunk. Report back:
   - what was built (files touched, rough diff size)
   - `go build ./...` and `go test ./...` results
   - the manual QA note (exact steps for the human to verify by hand)
   - any assumptions/open questions hit along the way
4. **Wait for explicit human sign-off** before doing anything further. Valid
   human responses are: approve & continue to next chunk, request changes to
   the current chunk, or stop entirely. The driver must not infer approval
   from silence, and must not treat "looks fine" as a request to also start
   the next chunk unless the human says so.
5. **The human commits.** Per standing instructions, the agent never commits
   or pushes on the user's behalf — the human reviews the working tree and
   commits it themselves once satisfied. This doubles as the natural
   checkpoint gate.
6. **One chunk in flight at any time.** Never queue up or silently begin
   chunk N+1 while chunk N is awaiting review, even if idle.

If at any point the driver is unsure whether it has approval to proceed, it
must stop and ask rather than assume.

## 6. Chunking Rules (credit-control guardrails)

"Small" here is measured in **diff size and objective count**, not human
clock-time, since AI agents consume tokens/credits at a very different rate
than a human would spend minutes:

- **One objective per chunk.** A chunk implements exactly one capability
  from Section 3 (or one clearly-named sub-piece of it, e.g. "list panel
  rendering" vs. "list panel keybindings" if the full feature is too large).
- **Bounded diff size.** Target ~150–250 changed lines (excluding
  generated/vendor/test-fixture files) and no more than ~4–6 touched files
  per chunk. If a worker agent estimates a chunk will exceed this, it must
  stop and report back for the chunk to be split, rather than proceeding.
- **One commit per chunk**, buildable and passing tests on its own
  (`go build ./...` and `go test ./...` must both succeed before the chunk
  is considered done). Never leave the tree in a broken intermediate state.
- **No speculative work.** Agents must not implement future-phase features
  (Section 7), refactor unrelated code, add dependencies not listed in
  Section 2, or fetch external repos/docs "for inspiration" mid-task.
- **Hard stop conditions** — a worker agent must halt and report rather than
  continue if it: needs a new third-party dependency, needs to modify more
  than one chunk's worth of prior work, encounters ambiguity in this spec,
  or is about to exceed the diff-size bound above.
- **Explicit interfaces first.** Where a chunk depends on another
  not-yet-built chunk, define the minimal Go interface/struct it needs and
  stub it, rather than blocking — this lets chunks be parallelized safely.

## 7. Future Phases (post-MVP, not to be started without explicit sign-off)

Same rules as Section 5 (one chunk at a time, human sign-off between each)
and Section 6 (bounded diff size, no speculative work) apply to every phase
and chunk below. Each phase's chunk list is a **starting proposal** — it
should be re-confirmed with the human before the first chunk of that phase
begins, since scope/design may shift by the time we get there.

### Phase 2 — Navigation & Discovery

- ~~**Focus newly-added task in list**~~ — **DONE.** After submitting the
  add-task form, the list panel now selects/focuses the task that was just
  created once the refreshed data comes back, via `tasklist.Model.SelectID`
  matching `Add`'s returned numeric ID against the refreshed
  `[]taskwarrior.Task`.
- ~~**Popups for warnings and errors**~~ — **DONE.** Added
  `internal/ui/popup` (bordered, colored overlay boxes rendered on top of
  the preserved background via `popup.Overlay`): `Box` for
  Info/Warning/Error severities (dismiss-any-key), and `ConfirmBox` for
  yes/no confirmations, both sharing a `renderBox` layout helper. Follow-up
  ad-hoc work (direct instruction, not a separate sign-off) converted two
  other UI surfaces to the same popup styling for consistency: the add-task
  form (previously a full-screen panel replacing the background; now a
  centered overlay box with the grid still visible behind it) and the
  delete-confirmation prompt (previously plain status-bar text; now an
  orange `ConfirmBox` overlay, same visual language as the severity
  popups).
- **Tasks panel scrolling** — deferred follow-up from the Chunk 1 layout
  refinement above: when the task list is taller than the panel's assigned
  height, it should scroll instead of overflowing unclipped (current
  behavior). Not yet scoped/chunked.
- ~~**Global vs. local keybinding architecture + Done/Deleted task
  reopen/restore/purge**~~ — **DONE.** Ad-hoc, direct-instruction feature
  (not from the Section 7 backlog), delivered in 3 signed-off chunks plus
  follow-up refinements:
  1. Refactored key dispatch in `cmd/lazytask/main.go` into a lazygit-style
     global tier (`q`/`ctrl+c`, `0`-`4`, `Tab`/`Shift+Tab` — fire from any
     panel focus) and a Tasks-panel-local tier (nav, `[`/`]`, `a`/`d`/`x`/`e`,
     `r`), replacing the old fully-global dispatch. Dropped the old `r` =
     refresh binding (redundant since every mutation/tab-switch already
     auto-refreshes).
  2. Added `Restore` (`modify status:pending`) to reopen a Done or Deleted
     task back to Todo, bound to `r` on the Done/Deleted tabs; user-facing
     status-bar label is "reopen" (internal identifiers keep "restore"
     naming to match the underlying taskwarrior operation).
  3. Added `Purge` (`purge`) to permanently delete a task from the Deleted
     tab, bound to `x` there (soft-delete `x` semantics unchanged on
     Todo/Done).
  4. Follow-up: fixed a bug where `d` (mark done) errored on the Done tab
     (taskwarrior's `done` command rejects already-completed tasks); `d` is
     now a no-op on the Done tab (hint hidden), and on the Deleted tab `d`
     restores-then-dones the task in one action. Added confirmation popups
     (`y`/`enter` to confirm, `n`/`esc` to cancel) for all four mutating
     actions that weren't already confirmed: mark-done (Todo and Deleted
     tabs) and reopen/restore (Done and Deleted tabs), matching the
     existing delete/purge confirm pattern.
  5. Follow-up: popup wording refined so Done/Deleted-tab prompts never
     reference the numeric task ID (taskwarrior always reports `ID: 0` for
     non-pending tasks, so it's meaningless there) — only Todo-tab prompts
     keep the ID. Final wording: `Mark task %d %q as done?` (Todo),
     `Mark %q as done?` (Deleted, via restore+done), `Reopen %q?` (Done),
     `Restore %q as a todo?` (Deleted).
  6. Follow-up: all four confirm popups (delete/purge/done/reopen) now
     accept `enter` as well as `y` to confirm; `popup.ConfirmBox`'s hint
     text updated to `"(y/enter) confirm   (n/esc) cancel"`.
- ~~**Project rename + Projects-panel task counts**~~ — **DONE.** Ad-hoc,
  direct-instruction feature (not from the Section 7 backlog), delivered in
  2 signed-off chunks:
  1. Projects panel entries now show a fixed-width `(N)` task-count suffix
     (right-aligned so all counts line up), counting only pending
     ("todo lane") tasks. `(all)` shows the total pending count across all
     projects; `(none)` shows the pending count of tasks with no project.
  2. `Shift+R` on a real project entry (not `(all)`/`(none)`) opens a
     rename text-input box (reusing/generalizing the `addform` bordered-box
     component via a new `NewNamed`/`SetValue` API), pre-filled with the
     current project name and auto-trimming leading/trailing whitespace on
     submit. Confirming renames the project on **every** task regardless of
     status (pending/completed/deleted), via the existing `Export`/`Import`
     round-trip filtered to exact project-name matches in Go (not a
     taskwarrior CLI filter, to avoid prefix-matching subprojects). A
     normal orange `ConfirmBox` confirms the rename; if the trimmed target
     name matches an *existing different* project, a second, red
     `DangerConfirmBox` (new `popup.DangerConfirmBox`) warns that this will
     merge tasks into that project and may be hard to undo. The active
     project filter follows the rename if it was pointed at the renamed
     project.
  3. Follow-up fix (direct instruction, same feature): projects with zero
     pending tasks were vanishing from the panel entirely (deleting a
     project's last task) or lingering forever after a restart (a project
     with only completed tasks) because `fetchProjects` re-derived the
     entire list from scratch on every refresh. It now queries
     `status:pending` only, and `model` keeps a session-lifetime
     `knownProjects` set that's unioned with each fetch — a project stays
     visible (showing `(0)`) once seen this session, even after its last
     pending task is completed/deleted, but a project with no pending
     tasks left simply won't reappear after an app restart. Also fixed a
     separate, longer-standing bug where marking a task done, deleting,
     restoring, or purging only refreshed the Tasks list, never the
     Projects panel — so counts went stale after any of those actions
     (most visibly as a `(0)`-count project still showing `(1)`). All four
     now refresh both the Tasks list and Projects panel.
- **General panel layout** — UX discussed and scoped below into 6 chunks,
  based on a since-removed `layout.md` design note. This covers the general
  layout, the filter/search panel, and the project & tag side panels as a
  single cohesive design (not three separate features). One chunk at a time,
  per Section 5; only Chunk 1 is approved to start — the rest need a
  checkpoint after the prior chunk lands, per usual.

  1. ~~**Panel grid scaffold + focus navigation**~~ — **DONE.** Two-column
     50/50 grid computed from `tea.WindowSizeMsg`; left column split into 4
     stacked rows (Status, Tasks, Projects, Tags); right column holds one
     large panel (Details, key `0`). Focus switches via number keys `0`-`4`
     and via `Tab`/`Shift+Tab` cycling, with lazygit-style border-color
     highlight on the focused panel. Existing `tasklist` slots into panel
     `2`; panels `1`/`3`/`4`/`0` are empty placeholders for now. Shared
     "focused panel" styling lives in `internal/ui/panel`
     (`panel.Render`/`panel.Frame`) for later chunks to reuse. Follow-up
     visual refinement folded into this chunk: panel titles are embedded
     inline in the top border (lazygit-style, e.g. `╭──[1]-Status────╮`)
     instead of a separate content row, reclaiming a row of body height in
     every panel, and the Tasks panel shows its `[2]-Tasks` number-key hint
     like the other four. A second follow-up pass (ad-hoc, direct
     instruction rather than a new formal chunk) reworked the left column's
     internal layout: Projects and Tags now sit side-by-side as two
     half-width panels in a single row at the bottom of the left column
     (instead of two separate stacked rows), and row heights are static
     rather than an even split — Status is a fixed single content line,
     Tasks takes the largest remaining share, and Projects/Tags share the
     rest, tuned to roughly Tasks 67% / Projects+Tags 33% of the
     non-Status height. This required lowering the shared
     `internal/ui/panel` `minHeight` clamp from 3 to 1 (it was silently
     forcing the 1-line Status panel to 3 content lines) and making the
     Tasks panel (`internal/ui/tasklist`) stretch-fill its assigned height
     instead of sizing purely from its task count. Tasks panel scrolling
     (for when task count exceeds the visible height) was explicitly
     deferred by the user as a follow-up feature — not yet scoped or
     implemented; Tasks currently overflows unclipped if there are more
     tasks than fit.
  2. ~~**Status panel (key 1)**~~ — **DONE.** Shows the ID, status,
     project, and tags of whatever task is currently selected in Tasks
     (`#<id> [<status>] P:<project> T:<tags>`, with `(none)` placeholders
     when a task has no project/tags); updates live as the Tasks cursor
     moves, since it reads the Tasks list's current selection on every
     render.
  3. ~~**Tasks panel status tabs (Todo/Done/Deleted)**~~ — **DONE.**
     Extends `tasklist` with 3 tabs (Todo/Done/Deleted), cycled via `[`/`]`
     (not the panel-focus number keys/Tab), each re-querying `task export`
     with the matching `status:pending`/`status:completed`/`status:deleted`
     filter; title renders as `[2]-Todo - Done - Deleted` via a new
     `panel.FrameTabs` helper. Two follow-up ad-hoc styling passes (direct
     instruction, same in-flight chunk): the active tab first switched from
     an inverted/reverse-video highlight to a solid focus-colored label,
     then (to fix a contrast bug where the whole title — border, `[2]-`
     prefix, and active tab — rendered uniformly magenta whenever the Tasks
     panel itself was focused, making the active tab indistinguishable) the
     active/inactive tab colors were split out into their own
     focus-independent constants (`activeTabColor` orange, `inactiveTabColor`
     dim gray) separate from the panel's own `FocusedColor`/`UnfocusedColor`
     border styling. Raised, but explicitly deferred, a related tangent:
     theming/configurable colors — logged below under Phase 3.
  4. ~~**Shared filter state + Projects panel (key 3)**~~ — **DONE.**
     Added a `filterState` struct (project as `*string`; nil = no filter) in
     `cmd/lazytask/filter.go`, plus a new `internal/ui/projects` panel
     listing distinct project names sourced from an unfiltered `task
     export` query (so the list doesn't shrink as the filter is applied).
     The list's special entries are `(all)` (clears the project filter) and
     `(none)` (filters to tasks with no project, `project:`), always sorted
     first — `(all)` then `(none)` — ahead of the real project names
     (several label spellings were tried, e.g. `*all*`/`*none*`; `(all)`/
     `(none)` was the final pick). Moving the cursor in the Projects panel
     (`↑/k`/`↓/j`) auto-applies the corresponding filter and refetches Tasks
     immediately — no `enter` press needed, based on follow-up feedback
     that navigating should be enough. A `filterState.equal` method was
     added since `filterState`'s `*string` project field can't be compared
     with `==`/`!=` by value, which is needed to avoid redundant refetches
     when the cursor moves without actually changing the selected filter.
  5. **Tags panel (key 4)** — same pattern as Projects, built on the
     filter-state infra from Chunk 4: lists distinct tags, with an `*any*`
     entry first (wrapped in `*`), selecting a tag sets a `+tag` filter.
     Also sourced from the same unfiltered query as Chunk 4, for the same
     reason.
  6. **Details panel (key 0, right column)** — full-detail read-only view
     of the Tasks-selected task, occupying the whole right column; visible
     in the border like lazygit's focused-panel treatment.

### Phase 3 — Customization

- **Theming (colors configurable via YAML)** — currently all panel
  border/title colors live as hardcoded `lipgloss.Color` constants in
  `internal/ui/panel` (`FocusedColor`, `UnfocusedColor`, plus the Tasks
  panel's tab-highlight `activeTabColor`/`inactiveTabColor` added
  alongside the status-tabs chunk). Raised as a tangent while fixing a
  related contrast bug (the active status tab was indistinguishable from
  the border when the Tasks panel itself was focused, since both used the
  same focus color) — not yet scoped/chunked; needs a decision on the YAML
  schema (named palette vs. per-role color keys) before chunking.
- **Configurable keymaps via YAML** — user-overridable key bindings for
  existing actions (list nav, add/done/delete/edit), loaded via the existing
  YAML config plumbing. Needs a decision on config file location/precedence
  before chunking.
- **Custom user-defined tasks/actions via YAML** — let users define their
  own quick-actions (e.g. canned `task` command templates) in config. Needs
  a decision on the templating/placeholder syntax before chunking.

### Phase 4 — Data Safety & Sync

- **Undo stack** — reverse the last mutation (add/done/delete/edit). Needs a
  decision on scope (single-level vs. multi-level undo, in-memory vs.
  persisted across restarts) before chunking.
- **Taskwarrior sync support** — wrap `task sync` so multi-machine sync
  configured outside lazytask can be triggered/monitored from the TUI. Needs
  a decision on how much sync-config setup (if any) lazytask should own vs.
  assume is already configured via taskwarrior's own `sync` settings.

## 8. Open Questions / Assumptions Log

- Assuming taskwarrior (`task` binary) is already installed on the target
  dev machine and CI runners used for integration tests; if not,
  integration tests should skip rather than fail.
- Assuming single-user, local-only taskwarrior data (no multi-context /
  multi-profile support in v1).
- Assuming terminal true-color support is not required; Lip Gloss should
  degrade gracefully on 16/256-color terminals.
