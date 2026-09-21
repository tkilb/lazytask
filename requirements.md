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
- **Popups for warnings and errors** — still needs discussion with user on
  UX; not yet scoped/chunked.
- **General panel layout** — UX discussed and scoped below into 6 chunks,
  based on a since-removed `layout.md` design note. This covers the general
  layout, the filter/search panel, and the project & tag side panels as a
  single cohesive design (not three separate features). One chunk at a time,
  per Section 5; only Chunk 1 is approved to start — the rest need a
  checkpoint after the prior chunk lands, per usual.

  1. **Panel grid scaffold + focus navigation** — two-column 50/50 grid
     computed from `tea.WindowSizeMsg`; left column split into 4 stacked
     rows (Status, Tasks, Projects, Tags); right column holds one large
     panel (Details, key `0`). Focus switches via number keys `0`-`4` and
     via `Tab`/`Shift+Tab` cycling, with lazygit-style border-color
     highlight on the focused panel. Existing `tasklist` slots into panel
     `2`; panels `1`/`3`/`4`/`0` are empty placeholders for now. Establishes
     the shared "focused panel" style helper later chunks reuse. **Approved
     to start.**
  2. **Status panel (key 1)** — shows the ID, status, and project of
     whatever task is currently selected in Tasks; updates live as the
     Tasks cursor moves.
  3. **Tasks panel status tabs (Todo/Done/Deleted)** — extends `tasklist`
     with 3 tabs, cycled via `[`/`]` (not the panel-focus number
     keys/Tab), each re-querying `task export` with the matching
     `status:pending`/`status:completed`/`status:deleted` filter.
  4. **Shared filter state + Projects panel (key 3)** — introduces a
     filter-state struct (project + tag) in the top-level model; new panel
     lists distinct projects, with `*none*` (no project) and `*all*` (no
     project filtering) entries at the bottom, wrapped in `*` to mark them
     as special. Selecting a project updates filter state and refetches
     Tasks with `project:X`. **Open design item to resolve when this chunk
     starts:** the Projects/Tags lists must be sourced from an unfiltered
     task query (e.g. `status:pending or status:completed`), not from
     whatever the Tasks panel's active filter currently returns — otherwise
     applying a filter would shrink the very lists used to change/clear
     that filter.
  5. **Tags panel (key 4)** — same pattern as Projects, built on the
     filter-state infra from Chunk 4: lists distinct tags, with an `*any*`
     entry first (wrapped in `*`), selecting a tag sets a `+tag` filter.
     Also sourced from the same unfiltered query as Chunk 4, for the same
     reason.
  6. **Details panel (key 0, right column)** — full-detail read-only view
     of the Tasks-selected task, occupying the whole right column; visible
     in the border like lazygit's focused-panel treatment.

### Phase 3 — Customization

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
