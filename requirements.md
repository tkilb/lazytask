# lazytask — Chunked Delivery Requirements

Status: v1 (MVP) planning doc for a driving agent ("driver") to break into
small, independently-committable worker tasks, delivered **one chunk at a
time with a mandatory human checkpoint between chunks.**

This is explicitly **not** meant to run unattended. There is no
"orchestrator" that dispatches chunk after chunk on its own — a human must
review and approve after every single chunk before the next one starts.

## 1. Project Summary

A TUI frontend for the [taskwarrior](https://taskwarrior.org/) CLI, written in
Go using [Bubble Tea](https://github.com/charmbracelet/bubbletea), with a
[lazygit](https://github.com/jesseduffield/lazygit)-inspired panel layout
(multiple bordered panes, keyboard-driven navigation, contextual
action bar/status line).

A prior project, `OsamaMahmood/lazytask` (Rust), covers similar ground.
Worker agents must **not** browse that repo or the internet to "borrow ideas" —
this doc already encodes the UI/UX decisions needed. Referencing external
repos mid-task burns credits and causes scope creep.

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

## 3. MVP Scope (v1) — build these, in this order

1. **List/browse tasks** — main panel showing pending tasks (id, description,
   project, priority, due date), scrollable/selectable list.
2. **Add task** — a prompt/input panel to create a new task via `task add`.
3. **Complete/delete task** — act on the currently selected task
   (`task <id> done`, `task <id> delete`) with a y/n confirmation for delete.
4. **Edit task fields** — open the task in the user's `$EDITOR` (fallback:
   `vi`) as a text buffer (similar to `task <id> edit`), then re-import on
   save.

Explicitly **out of scope for v1** (do not implement unless requirements are
updated): filter/search panel, project & tag side panels, configurable
keymaps, custom YAML-defined tasks, undo, sync. These are documented in
Section 6 as future phases so agents know not to attempt them early or
gold-plate v1 code in anticipation of them.

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
  (Section 6), refactor unrelated code, add dependencies not listed in
  Section 2, or fetch external repos/docs "for inspiration" mid-task.
- **Hard stop conditions** — a worker agent must halt and report rather than
  continue if it: needs a new third-party dependency, needs to modify more
  than one chunk's worth of prior work, encounters ambiguity in this spec,
  or is about to exceed the diff-size bound above.
- **Explicit interfaces first.** Where a chunk depends on another
  not-yet-built chunk, define the minimal Go interface/struct it needs and
  stub it, rather than blocking — this lets chunks be parallelized safely.

## 7. Suggested Chunk Breakdown (work through these one at a time)

1. `chore/scaffold` — go.mod, `cmd/lazytask/main.go` entrypoint, empty Bubble
   Tea `Model`/`Update`/`View`, README stub, `.gitignore`. No taskwarrior
   calls yet.
2. `internal/taskwarrior/client` — thin wrapper around `exec.Command("task",
"export", ...)` returning parsed `[]Task` structs (JSON decode only).
   Unit + integration tests. No UI.
3. `internal/taskwarrior/mutations` — `Add`, `Done`, `Delete` wrapper
   functions over the `task` CLI. Unit + integration tests. No UI.
4. `internal/ui/tasklist` — Bubble Tea model rendering the task list panel
   (static data / fixture-driven), lazygit-style bordered pane, up/down
   selection. No live taskwarrior wiring yet.
5. `wire: list panel to real data` — connect chunk 2's client to chunk 4's
   panel on startup; a `r` key refresh.
6. `internal/ui/addform` — input panel for new task description, wired to
   chunk 3's `Add`.
7. `feature: complete/delete` — keybindings (`d` done, `x` delete +
   confirm) wired to chunk 3.
8. `internal/editor` — helper to write a task to a temp file, launch
   `$EDITOR`, and read back changes; unit tests only (editor invocation
   itself is not integration-testable headlessly).
9. `feature: edit task` — wire chunk 8 into the task list (`e` key),
   re-import edited fields via taskwarrior client.
10. `polish: status bar & help` — lazygit-style bottom bar showing active
    keybindings for the current panel.

Each numbered item above is one chunk. Per Section 5, the driving agent must
stop and get explicit human sign-off after each one (build+tests pass,
manual QA note present) before starting the next.

## 8. Future Phases (post-MVP, not to be started without explicit sign-off)

- Filter/search panel
- Project & tag side panels (lazygit-style left sidebar)
- Configurable keymaps via YAML
- Custom user-defined tasks/actions via YAML
- Undo stack
- Taskwarrior sync support
- **Focus newly-added task in list** — after submitting the add-task form
  (chunk 6, `internal/ui/addform`), the list refresh currently leaves
  cursor/selection at whatever `tasklist.Model` defaults to. A follow-up
  chunk should make the list panel select/focus the task that was just
  created once the refreshed data comes back, instead of requiring the
  user to scroll to find it.

## 9. Open Questions / Assumptions Log

- Assuming taskwarrior (`task` binary) is already installed on the target
  dev machine and CI runners used for integration tests; if not,
  integration tests should skip rather than fail.
- Assuming single-user, local-only taskwarrior data (no multi-context /
  multi-profile support in v1).
- Assuming terminal true-color support is not required; Lip Gloss should
  degrade gracefully on 16/256-color terminals.
