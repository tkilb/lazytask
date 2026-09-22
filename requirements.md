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

DONE (see `git log` for detail): focus newly-added task in list; popups for
warnings/errors/confirmations (`internal/ui/popup`); global vs. local
keybinding architecture + Done/Deleted task reopen/restore/purge; project
rename + Projects-panel task counts; global add key with project-filter
auto-assign on new tasks; general panel layout (grid + focus nav; Status
panel key `1`; Tasks panel status tabs Todo/Done/Deleted; shared filter
state + Projects panel key `3`; Details panel key `0`).

Remaining:

- **Tasks panel scrolling** — when the task list is taller than the panel's
  assigned height, it should scroll instead of overflowing unclipped
  (current behavior). Not yet scoped/chunked.

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

### Phase 4 — Data Safety & Sync

- **Undo stack** — reverse the last mutation (add/done/delete/edit). Needs a
  decision on scope (single-level vs. multi-level undo, in-memory vs.
  persisted across restarts) before chunking.
- **Taskwarrior sync support** — wrap `task sync` so multi-machine sync
  configured outside lazytask can be triggered/monitored from the TUI. Needs
  a decision on how much sync-config setup (if any) lazytask should own vs.
  assume is already configured via taskwarrior's own `sync` settings.

### Phase 5 — Priority logic

- **Priority-based coloring** — color-code each row in the Tasks panel by
  its `priority` field (e.g. H=red, M=yellow, L=default/unstyled, none=
  default), consistent with the existing hardcoded-color pattern in
  `internal/ui/panel` (see Phase 3 theming note — should stay overridable
  later, not hardcoded in a way that fights that future work).
- **Priority sort mode** — add a selectable sort mode for the Tasks panel
  that orders by `priority` (H > M > L > none), alongside whatever sort
  mode(s) already exist. Needs a decision on the keybinding to cycle/select
  sort mode before chunking.
- **Quick set-priority keys** — from the Tasks panel (task focused), `h`/
  `m`/`l` set that task's priority directly to H/M/L respectively (via
  `task <id> modify priority:H|M|L`), no popup/confirmation needed. A
  fourth key/action to clear priority back to none is still open (not
  specified — needs a decision, e.g. reusing one of h/m/l as a toggle-off
  if already at that value, vs. a separate key).
- **Urgency-based manual reordering (`Ctrl+j` / `Ctrl+k`)** — within a
  task's current priority lane (H, M, or L), let the user "nudge" a task
  up/down relative to its neighbors to establish a custom order, without
  changing its priority band.
  - **Open design problem, not yet solved:** taskwarrior's `urgency` is a
    **computed, read-only value** (derived from due date, age, tags,
    project, etc. via coefficients) — it is not a stored field and cannot
    be directly set via `task modify urgency:...`. So "mutate the urgency
    score" as stated isn't literally achievable against taskwarrior as-is.
    Options to actually deliver the described "custom order within a
    priority lane" behavior, to be decided before chunking:
    1. Introduce a lazytask-owned UDA (e.g. `priority_ord` or similar,
       registered via taskwarrior's UDA config) as a manual sort-key
       nudged by `Ctrl+j`/`Ctrl+k`, used as a secondary sort key after
       priority — real `urgency` is left alone, purely computed as normal.
    2. Nudge one of taskwarrior's real urgency inputs that's cheap to
       toggle per-task (e.g. a lazytask-managed tag like `+ord1`/`+ord2`
       with configured urgency coefficients) so it does actually move the
       real urgency score, at the cost of needing coefficient config setup.
    3. Purely a lazytask-side, in-memory/local sort override (not synced
       to taskwarrior at all), simplest but lost if sorting is recomputed
       from a fresh `task export`.
    - Leaning toward option 1 (dedicated UDA) as least surprising and most
      durable, but needs explicit sign-off since it means lazytask starts
      writing/depending on a UDA it defines, which is new territory.
  - Also needs a decision on whether `Ctrl+j`/`Ctrl+k` swap with the
    adjacent task or nudge by a fixed increment (matters once ties/gaps in
    the sort key accumulate).
- **Urgency visibility** — show taskwarrior's real computed `urgency` score
  (e.g. as a column/detail-panel field) so the user can see it alongside
  priority, and optionally allow sorting by it as a separate sort mode from
  the H/M/L priority sort above. Needs a decision on where it's displayed
  (list column vs. Details panel only).

### Phase 6 - Advanced Wizards

- Make an new component that will allow the user to quickly pick a due date.
  The idea is we want a cord like '2d' for two days from now and '1w' for on week from now.
  '2b' will be two business days from now, assume Sat and Sun are not business days.
- 'p' from the edit panel will open a popup for a quick project picker for filtering
- 't' from the edit panel will open a popup for a quick tag picker for filtering
- 'P' from the edit panel will open a popup for project re-assign, allow for existing project to be selected from a list
  or a new one to be keyed in
  or a new one to be keyed in
- 'D' will make use of the date component for the selected task. cord shortcuts may be used for a quick date or have the option for a custom date key in
- Add will now have additional fields for project, tags, due date an priority.

### Phase 7 — Tags

- **Tags panel (key 4)** — same pattern as the Projects panel, built on the
  existing filter-state infra: lists distinct tags, with an `*any*` entry
  first (wrapped in `*`), selecting a tag sets a `+tag` filter, sourced from
  the same unfiltered query as Projects. Pending sign-off/implementation
- 'T' from the edit panel will open a popup for tagging re-assign, allow for existing tags to be selected from a list. Tags will be appended if selected.
  If keyed in, there will be a comma delimited list and the mode will be replacement instead.

### Phase 8 — Low Priority

- **Remote task data** - Allow for a remote mode an local mode for tasks state.
  Approach: use taskwarrior/TaskChampion's native **git sync backend**
  (`sync.git.local_path` / `sync.git.branch` / `sync.git.remote` /
  `sync.encryption_secret`, confirmed supported as of `task` 3.5.0, PR #4111)
  rather than raw file sync or a new backend. Task payloads are client-side
  encrypted before being committed, so the remote git host never sees
  plaintext task data. The intended remote is a **private** git repo, but
  encryption + a history purge (below) are extra defense-in-depth layers in
  case that repo is ever exposed.
  - **History retention/purge** — TaskChampion's built-in version-file
    cleanup only prunes already-snapshotted files and defaults to a
    hardcoded 180-day retention (not configurable via `task config`); it
    also does **not** rewrite git history, so old encrypted blobs remain
    recoverable from git history/objects until history is rewritten and
    garbage-collected. To actually purge data older than **one week**,
    lazytask will need its own maintenance routine (e.g. periodic
    history-squash/rewrite + force-push + `git gc --prune=now` on the sync
    repo) run on a schedule, since this isn't achievable via existing
    taskwarrior config alone. Needs a decision on how/when this maintenance
    runs (triggered from lazytask vs. an external cron) before chunking.

#### Phase 8 design notes — git history purge routine

Investigated directly against TaskChampion's `src/server/gitsync/mod.rs` (as
of the commit that added git-sync support, `task` 3.5.0 / PR #4111), for the
benefit of whichever agent chunks this later. Not implemented — design only.

**How the git-sync backend actually stores data:**

- Each sync writes one immutable file `v-{parent_uuid}-{child_uuid}`
  containing an encrypted history segment. `snapshot` and `meta` files get
  overwritten in place as newer snapshots/version pointers are produced.
- TaskChampion's own cleanup (`cleanup()`) only `git rm`s version files that
  are (a) already covered by a snapshot and (b) whose last-touching commit
  is older than a **hardcoded 180-day `version_retention` constant** — this
  is not exposed via any `task config sync.git.*` key.
- Critically, that cleanup only stages a `git rm` + commit; it does **not**
  rewrite git history. The deleted blob content is still present in older
  commit objects until history is rewritten and garbage-collected.
- TaskChampion never reads git commit _history_ for correctness of syncing
  — the only historical git operation is `git log -1 --format=%ct -- <file>`
  to compute a single file's age for that cleanup check. Otherwise it only
  cares about working-tree state at `HEAD`. This means a full history
  rewrite is safe from TaskChampion's point of view as long as the final
  `HEAD` tree (meta, snapshot, surviving version files) is preserved
  byte-for-byte.

**Proposed purge routine** (target retention: ~7 days of git history, run on
a schedule):

1. Take a lock/mutex on the sync repo directory so this never races with an
   in-flight `task sync`.
2. `git fetch` + fast-forward to the true remote tip first — never rewrite
   from a stale local view of the repo.
3. Squash rather than selectively rewrite: `git checkout --orphan
purge-tmp && git add -A && git commit` to create one fresh, parentless
   commit holding exactly the current `meta`/`snapshot`/surviving version
   files. (A full squash is simpler and more robust than trying to
   selectively drop only commits older than the cutoff, and is safe per the
   point above.)
4. Rename `purge-tmp` over the configured `sync.git.branch`, then `git push
--force` to `sync.git.remote`.
5. Locally: `git reflog expire --expire=now --all && git gc --prune=now
--aggressive` to actually drop the now-unreachable objects on disk.
6. On push rejection (a `task sync` landed mid-purge), abort and retry from
   step 2 — do not reuse TaskChampion's own pull-and-reset retry logic,
   which is designed for appending versions, not rewriting history.

**Known side effects / open questions for the chunking agent (not decided
here):**

- Squashing resets TaskChampion's own internal per-file "age" tracking
  (since it's derived from `git log`), effectively restarting its 180-day
  cleanup timer. Harmless — our external 7-day purge supersedes it — but
  should be documented so it isn't mistaken for a bug later.
- **Every other replica/device syncing against this repo must also discard
  its old local clone/history after a purge**, or it can silently
  resurrect "deleted" history on its next push. Needs a coordination story
  (e.g. detect a rewritten root commit and force re-clone) before this is
  safe to ship for multi-device use.
- **Force-push + local `git gc` is mitigation, not a guarantee of remote
  deletion.** Hosts such as GitHub commonly retain unreachable objects for
  a grace window (historically up to ~90 days) before their own background
  GC reclaims them, and any existing fork/mirror/backup of the repo keeps
  full history regardless of what this routine does. This must be
  documented to the user as defense-in-depth on top of "keep the repo
  private," not an absolute guarantee.
- Trigger mechanism still needs a decision: lazytask-internal
  timer/hook vs. a separate external cron script the user manages.
  Leaning toward starting with an external script, since a
  history-rewriting operation is riskier to run inside the TUI process's
  own lifecycle.
- Retention window (7 days) should probably be YAML-configurable rather
  than hardcoded, consistent with the rest of Phase 8's config-driven
  items.

### Phase 9 — Low Priority

- **Configurable keymaps via YAML** — user-overridable key bindings for
  existing actions (list nav, add/done/delete/edit), loaded via the existing
  YAML config plumbing. Needs a decision on config file location/precedence
  before chunking.
- **Custom user-defined tasks/actions via YAML** — let users define their
  own quick-actions (e.g. canned `task` command templates) in config. Needs
  a decision on the templating/placeholder syntax before chunking.

## 8. Open Questions / Assumptions Log

- Assuming taskwarrior (`task` binary) is already installed on the target
  dev machine and CI runners used for integration tests; if not,
  integration tests should skip rather than fail.
- Assuming single-user, local-only taskwarrior data (no multi-context /
  multi-profile support in v1).
- Assuming terminal true-color support is not required; Lip Gloss should
  degrade gracefully on 16/256-color terminals.
