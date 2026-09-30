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
- **Updating this document is MANDATORY, not optional.** Before reporting a
  chunk as done (Section 5 step 3), and immediately after any planning
  discussion that resolves an open question/decision (even before code is
  written), the agent MUST update this requirements.md to reflect: (a) any
  decision just made (recorded so it isn't re-litigated later), (b) status
  of the relevant chunk/checklist item, and (c) any new sub-tasks or
  chunking plan that resulted from the discussion. This applies even if the
  human hasn't explicitly asked for a doc update. This edit is part of the
  same chunk/turn and does not itself require a separate sign-off, but it
  must be included in what the human reviews before approving.
- **Hard stop conditions** — a worker agent must halt and report rather than
  continue if it: needs a new third-party dependency, needs to modify more
  than one chunk's worth of prior work, encounters ambiguity in this spec,
  or is about to exceed the diff-size bound above.
- **Explicit interfaces first.** Where a chunk depends on another
  not-yet-built chunk, define the minimal Go interface/struct it needs and
  stub it, rather than blocking — this lets chunks be parallelized safely.

## 7. Outstanding Phases to be Completed

### Phase 5 — Remote Task Data

- **Remote task data** - Allow for a remote mode an local mode for tasks state.
  Approach: use taskwarrior/TaskChampion's native **git sync backend**
  (`sync.git.local_path` / `sync.git.branch` / `sync.git.remote` /
  `sync.encryption_secret`, confirmed supported as of `task` 3.5.0, PR #4111)
  rather than raw file sync or a new backend. Task payloads are client-side
  encrypted before being committed, so the remote git host never sees
  plaintext task data. The intended remote is a **private** git repo, but
  encryption + a history purge (below) are extra defense-in-depth layers in
  case that repo is ever exposed.

  **Status: chunks 1–4 committed. History purge routine and auto-sync are
  both fully designed/decided (see below) but have no code written yet —
  awaiting sign-off to start the first chunk of either.**
  - 🐛 **Bug fix, 2026-09-30 (uncommitted) — intermittent "database is
    locked: Error code 5: database is locked" popup.** Root cause:
    `internal/taskwarrior.Client` had no serialization at all — every
    method (`Export`, `Add`, `Done`, `Delete`, `Purge`, `Import`,
    `SetPriority`/`SetDue`/`SetProject`/etc. via the shared `run()`
    helper, and `Sync`) spawned its own independent `task` subprocess, and
    lazytask routinely fires several of these concurrently as sibling
    `tea.Cmd`s (e.g. a mutation alongside the tasks/projects/tags refresh
    it triggers, an auto-sync tick landing mid-mutation, or two mutations
    racing). Taskwarrior 3.x's SQLite-backed replica storage does not
    tolerate concurrent processes touching one data directory. Fix: added
    a `sync.Mutex` field to `Client` (there is exactly one shared instance
    for the whole app) and serialized every subprocess invocation through
    it. Confirmed directly while building the fix: with the lock
    temporarily stripped, a concurrent Add+Sync stress test against a
    git-sync-configured replica reliably produced 30+ failures per run
    (including literal "database is locked" errors); restoring the lock
    dropped that to 0 across repeated runs. Covered by
    `internal/taskwarrior.TestClient_ConcurrentInvocations_NoDatabaseLocked`
    (skips if `task`/`git` aren't on `PATH`, like the package's other
    integration tests). Known limitation documented in `Client`'s comment:
    this only serializes lazytask's own `task` calls — it can't prevent a
    *different*, unrelated `task` process (e.g. a manual invocation in
    another terminal) from racing Taskwarrior's own locking.
  - ✅ **Chunk 1 — Sync config plumbing.** `internal/config.GitSyncConfig` +
    `LoadGitSyncConfig()` read `sync.git.*` from a fixed, hand-editable
    `~/.config/lazytask/config.yaml` (same path on every OS, incl. Arch
    Linux — deliberately not `os.UserConfigDir()`'s per-OS convention).
    `taskwarrior.Client.ApplyGitSyncConfig` idempotently writes those keys
    via `task config`, mirroring the existing `EnsureUDA` pattern. Wired
    into app startup in `cmd/lazytask/main.go`, best-effort with stderr
    warnings on failure.
  - ✅ **Chunk 2 — Manual sync trigger.** Global `S` keybinding runs
    `task sync` (`Client.Sync`). Taskwarrior's raw sync stderr is noisy
    (`TASKRC override:`, etc.) and the "no backend configured" case is
    common, so `cleanSyncStderr()` strips the noise and a plain-English
    message is substituted for that specific case, pointing at
    `sync.git.*` / `task-sync(5)`.
  - ✅ **Chunk 3 — Auto-managed, configurable encryption secret.** The
    secret is **never** written to Taskwarrior's own `.taskrc` — only
    passed as a one-off `rc.sync.encryption_secret=` override at `task
sync` time (verified via integration test). `config.EnsureSyncSecret`
    auto-generates one (32 random bytes, hex, `crypto/rand`) on first use
    and persists it at `~/.local/share/lazytask/sync-secret` (0600,
    deliberately outside `~/.config` so a dotfile manager tracking
    `config.yaml` won't sweep it up) unless overridden via `config.yaml`'s
    `sync.git.encryption_secret_file` (a _path_, safe to dotfile-manage,
    since it holds no secret material).
  - ✅ **Bugfix — `local_path` tilde expansion.** Taskwarrior's own `~`
    expansion of `sync.git.local_path` was observed to misresolve on
    macOS (`~/foo` → `/home//foo`, a bogus automount path), causing a
    cryptic "Operation not supported (os error 45)" sync failure. Fixed
    by having lazytask expand a leading `~/` itself (via
    `os.UserHomeDir()`, pure Go stdlib — platform-agnostic, including
    Linux/Arch) before ever handing the path to `task config`. Covered by
    `TestLoadGitSyncConfig_LocalPathTildeExpansion`.
  - ✅ **Chunk 4 — Sync secret CLI subcommand.** `lazytask sync-secret`
    ensures the encryption secret exists (creating it via the same
    `config.EnsureSyncSecret` path used by the first `S`-triggered sync)
    and prints its on-disk path (never the secret value itself).
    `lazytask sync-secret rotate` unconditionally regenerates it via the
    new `config.RotateSyncSecret`, after printing a warning that rotation
    permanently invalidates decryption of any history already pushed
    under the old secret and requires updating every other device sharing
    the sync repo.
  - **Not yet done:** the history purge routine below, and the
    auto-sync feature described further below. Neither has any code
    written yet as of this update — decisions/design only.
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

#### Phase 5 design notes — git history purge routine

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

**Known side effects / open questions for the chunking agent:**

- Squashing resets TaskChampion's own internal per-file "age" tracking
  (since it's derived from `git log`), effectively restarting its 180-day
  cleanup timer. Harmless — our external 7-day purge supersedes it — but
  should be documented so it isn't mistaken for a bug later.
- ~~Every other replica/device syncing against this repo must also
  discard its old local clone/history after a purge...~~ **RESOLVED,
  2026-09-29: not actually a risk.** Verified directly against
  TaskChampion's git-sync source (`src/server/gitsync/mod.rs`): every sync
  operation (not just ours) already calls `reset_to_remote()`, which does
  `git fetch <remote> <branch>` into `FETCH_HEAD` followed by an
  unconditional `git reset --hard FETCH_HEAD` — called before every
  read/write and specifically on push rejection. This is a hard reset, not
  a merge/rebase, so it has no dependency on shared ancestry: `git fetch`
  can't be "rejected" by a rewritten history (it only updates `FETCH_HEAD`,
  not a local branch ref), and `reset --hard` adopts the new tree
  unconditionally even with zero common ancestor. **Conclusion: no
  "lockout" risk.** Every other device self-heals automatically on its
  very next `task sync` after our purge force-pushes a squashed/orphan
  history — no manual re-clone or special "detect rewritten root"
  safeguard is needed. The multi-device-coordination chunk originally
  planned for this has been dropped from the plan as unnecessary.
- **Force-push + local `git gc` is mitigation, not a guarantee of remote
  deletion.** Hosts such as GitHub commonly retain unreachable objects for
  a grace window (historically up to ~90 days) before their own background
  GC reclaims them, and any existing fork/mirror/backup of the repo keeps
  full history regardless of what this routine does. This must be
  documented to the user as defense-in-depth on top of "keep the repo
  private," not an absolute guarantee.
- ~~Trigger mechanism still needs a decision...~~ **DECIDED, 2026-09-29:
  lazytask-internal** (runs on a schedule inside lazytask's own process
  lifecycle, not an external cron script the user manages — the doc had
  previously leaned toward external, but the human explicitly chose
  internal).
- ~~Retention window (7 days) should probably be YAML-configurable...~~
  **DECIDED, 2026-09-29: yes, YAML-configurable**, consistent with the
  rest of Phase 5's config-driven items (exact key name TBD at chunking
  time, following the `sync.git.*` convention).

**Revised chunking plan for the purge routine (decided 2026-09-29, no code
written yet):**

1. **Purge config plumbing** — add a `sync.git.*` retention-window field
   (default 7 days) to `GitSyncConfig`/`LoadGitSyncConfig`, mirroring the
   existing pattern. Config surface + tests only, no purge logic yet
   ("explicit interfaces first," per Section 6).
2. **Purge git operations (core routine)** — the squash/force-push/gc
   sequence above, exposed as a testable Go function (e.g.
   `taskwarrior.Client.PurgeSyncHistory`), invokable manually but not yet
   auto-triggered.
3. **Internal scheduling/trigger** — wire it into lazytask's own lifecycle
   (background ticker checking last-purge time vs. retention), with
   locking against a concurrent `task sync`.
   (A 4th "multi-device coordination safeguard" chunk was considered but
   dropped — see the resolved bullet above.)

**Auto-sync (new feature, decided 2026-09-29, not in the original Section 7
list — added here since it's a natural extension of Phase 5 and reuses the
same internal-scheduling mechanism as the purge routine above; no code
written yet):**

Currently `task sync` only runs when the user presses `S` (Chunk 2, already
shipped). Human requested this instead run automatically so devices that
"constantly sync" don't have to remember to press a key. Decided design:

- **Triggers, both of the following (not either/or):**
  - **Periodic**: a background timer while lazytask is open, interval
    YAML-configurable via `sync.git.auto_sync_interval_minutes` (default
    5).
  - **On-mutation**: after any local task mutation — add, done, delete,
    restore, purge, priority, due, project, reorder, edit — **except**
    undo/redo (human's call: undo/redo should stay sync-exempt; the
    periodic timer will eventually pick up whatever state they left).
    Debounced (~2s quiet period) so a burst of mutations (e.g. reordering
    several tasks in a row) coalesces into a single `task sync` instead of
    firing once per keypress.
- **Manual `S` key stays exactly as-is** (unchanged popup on both success
  and failure).
- **Auto-sync (periodic + on-mutation) UI is intentionally quiet**: no
  "Sync complete" popup on success; on failure, a status-line indicator
  only (no popup interruption). This needs new message types
  (`autoSyncedMsg`/`autoSyncErrMsg`) distinct from the existing manual
  `taskSyncedMsg`/`taskSyncErrMsg`, since the manual path's popups must be
  preserved unchanged.

**Chunking plan for auto-sync:**

1. ✅ **Periodic auto-sync (committed, `734ef18`)** —
   `sync.git.auto_sync_interval_minutes` config (default 5) +
   a `tea.Tick`-driven background timer that runs `task sync` silently,
   with quiet status-line-only failure reporting (`autoSyncTickMsg`,
   `autoSyncedMsg`, `autoSyncErrMsg`). No on-mutation hook yet.
2. ✅ **On-mutation auto-sync (complete, uncommitted, awaiting sign-off)** —
   `mutationSyncGen`/`mutationSyncDebounceMsg`/`scheduleMutationAutoSync`
   wire a debounced (2s quiet period) trigger into all 10
   mutation-success message cases (add, done, delete, restore, purge,
   priority, due, project, reorder, edit), reusing chunk 1's quiet
   `runAutoSync` path. Undo/redo are confirmed excluded. A generation
   counter ensures only the most recent mutation in a burst actually
   fires a sync (earlier, superseded timers become no-ops). Both
   chunks of the auto-sync plan are now done.

### Phase 6 — UX Enhancements

- [x] **Better tasklist column headers and date display** — Due column now
      shows a bare signed day-delta instead of the raw date (e.g. "2" = due
      in two days, "0" = due today, "-1" = overdue by a day; no due date
      renders as blank). Header label stays "Due" (decided 2026-09-30,
      human's call — no rename needed). Colors: overdue → red
      (`dueOverdueColor`), due today → bright orange 256-color
      (`dueTodayColor`, code 208 — chosen since every base-16 color was
      already spoken for by priority/search-match colors), due in 1 day →
      yellow (`dueTomorrowColor`), due in 2+ days → green
      (`dueLaterColor`). Implemented in `internal/ui/tasklist/model.go`
      (`dueDeltaDays`/`formatDueCell`/`dueColor`), with a `Model.now`
      field (default `time.Now`, overridable via `WithNow`, mirroring
      `internal/ui/datepick`'s same pattern) so tests get deterministic
      deltas. Covered by `TestDueDeltaDays`, `TestFormatDueCell`,
      `TestDueColor`, `TestRenderDataRow_DueCellColoredByDelta`,
      `TestModel_WithNow_UsedByView`.

- [x] **Quick Duedate update** — shift D on an existing task summons a popup
      (`internal/ui/datepick`) using the same cord/absolute-date parsing as
      the Add form's due-date field (`datepick.ResolveInput`). Found
      already fully implemented and committed (`'D'` keybinding in
      `cmd/lazytask/main.go`'s `updateDatePicking`, wired to
      `setDueTask`/`m.duer`) when checked 2026-09-30 — this checkbox was
      simply never ticked. No new code needed; verified `go build ./...`
      and `go test ./...` both still pass.
  - **Fine-tune, 2026-09-30 — "today" as a due date.** `dateparse.Parse`
    now accepts bare `"0"` (no unit letter, since a unit is meaningless
    for zero) and `N == 0` for any unit (`"0d"`/`"0w"`/`"0b"`), all
    resolving to the current instant (today) via `Cord.Resolve`. Old
    behavior rejected `N <= 0` outright. `internal/ui/datepick`'s
    placeholder text updated to mention `0`. Covered by new
    `dateparse.TestParse` cases (`"0"`, `"0d"`, `"0w"`, `"0b"`), a new
    `TestCordResolve` case, a `TestResolveString` case, and
    `datepick.TestResolvedZeroMeansToday`.
  - **Fine-tune, 2026-09-30 — popup visuals.** Two bugs fixed in
    `internal/ui/datepick`'s rendering, both pre-existing since the popup
    was first wired up: (1) the box used its own hardcoded blue-ish
    border color (`lipgloss.Color("62")`) instead of the app's standard
    focused-panel color; it now reuses `panel.FocusedColor` (green, same
    as every other focused panel/box). (2) the top border's corner/fill
    characters (`border.TopLeft`/dash-fill/`border.TopRight`) were
    rendered with **no** color style at all, while the title/hint text
    and the box's sides/bottom border *were* colored — this is what
    looked like "half blue": one edge uncolored, the rest colored. Fixed
    by wrapping the corner/fill segments in the same color style as the
    rest of the box. (3) Width: the popup previously rendered at the
    *full terminal width* (it read `m.width` from `tea.WindowSizeMsg`
    directly and used it as the box width verbatim) — now capped to a
    `maxWidth` of 64 cols (shrinking further, down to a `minWidth` floor
    of 58, only if the terminal itself is narrower), mirroring
    `internal/ui/popup`'s existing max/shrink/floor sizing pattern.
    Covered by new tests `TestView_WidthCappedRegardlessOfTerminalWidth`,
    `TestView_WidthShrinksForNarrowTerminal`, `TestTopBorder_FullyColored`.
  - **Fine-tune, 2026-09-30 — trimmed "Press " from popup hints.**
    Shortened every "Press <enter> to ..." hint to just "<enter> to ..."
    to save horizontal space: `datepick`'s hint, `addform`'s default
    hint, and the two inline hints main.go passes into
    `addform.NewNamed` for the Rename/Reassign Project popups.
  - **Fine-tune, 2026-09-30 — same border-color/width fixes applied to
    `internal/ui/addform`.** The Add Task / Rename Project / Reassign
    Project popups (all built on `addform.Model`, overlaid the same way
    as the due-date popup) had the identical pre-existing bugs: hardcoded
    blue-ish `lipgloss.Color("62")` border instead of
    `panel.FocusedColor`, an unstyled top-border corner/fill (the "half
    blue" look), and stretching to the full terminal width. Fixed
    identically — `panel.FocusedColor`, colored corners/fill in
    `topBorder`, and width capped to `maxWidth` 64 / floored at
    `minWidth` 60 (sized to fit the longest title+hint combo among this
    package's three callers, "Reassign Project" + its hint). Covered by
    new tests mirroring `datepick`'s:
    `TestView_WidthCappedRegardlessOfTerminalWidth`,
    `TestView_WidthShrinksForNarrowTerminal`, `TestTopBorder_FullyColored`.
  - **Fine-tune, 2026-09-30 — `datepick`'s maxWidth/minWidth are now
    content-derived, not guessed literals.** Human tried lowering
    `minWidth` to shrink the popup and saw no effect — root cause: on any
    terminal wider than `maxWidth` (virtually always), `View()` renders
    at exactly `maxWidth`; `minWidth` only ever applies as a floor on a
    terminal narrower than that, so it was the wrong constant to tune.
    Separately, `maxWidth` (64) had gone stale after the "Press " hint
    trim above shortened the actual content requirement. Replaced both
    hardcoded literals with `minWidth = 2 + len(" "+title+" ") +
    len(" "+hintText+" ") + 1` (the true minimum outer width that fits
    title+hint on one embedded border line without overlap — valid as a
    Go compile-time constant expression since title/hintText are
    constant strings) and `maxWidth = minWidth + 2`, so they can't drift
    out of sync with the actual title/hint text again. Net effect: popup
    shrank from 64 to 52 cols. `addform`'s equivalent constants were left
    untouched (not requested), so its 60/64 are still literals sized by
    hand-computed length in the earlier chunk above.
- [x] **Purge all** — bulk-purge action for tasks. **Scope/safety decided
      2026-09-30 (human sign-off), no code written yet:**
  - **Scope**: operates only on tasks currently visible on the Deleted tab
    under the active project/tag filters (same filtered set the list panel
    is already showing) — not every deleted task in taskwarrior regardless
    of filters.
  - **Keybinding**: `X` (Shift+x), active only while focused on the
    Deleted tab, mirroring how lowercase `x` is already relabeled "purge"
    there. Distinct from the existing per-task `x`/"purge" key, which
    stays unchanged.
  - **Confirmation**: a popup that states the exact count of tasks about
    to be purged and requires an explicit confirm keypress (not a bare
    single-key y/n, and not a typed "purge" phrase — count-and-confirm
    only).
  - **Undo**: one combined undo action restores every purged task at once
    (single entry on lazytask's undo stack), analogous to `purgeTask`'s
    per-task snapshot+re-import but batched into one `undo.Action`.
  - **Proposed chunking plan**:
    1. ✅ **Bulk purge core routine (done, uncommitted)** — `purgeAllTasks`
       (`cmd/lazytask/main.go`) takes the current filtered Deleted-tab task
       set, captures one combined JSON-array snapshot up front, calls
       `TaskPurger.Purge` per task (in order), and returns a single
       combined `tasksPurgedMsg`/`undo.Action` (or `tasksPurgeErrMsg` on
       failure; a nil `tea.Cmd` if the task set is empty). Undo re-imports
       the whole batch in one `Import` call; Redo re-purges every task.
       Documented limitation: a mid-batch Purge failure is not rolled
       back (no compensating un-purge of tasks already removed) — this
       core routine has no confirmation UI/keybinding wiring yet, so it's
       not reachable from the running app. Covered by
       `cmd/lazytask/bulkpurge_test.go`
       (`TestPurgeAllTasks_EmptyIsNoOp`,
       `TestPurgeAllTasks_PurgesEveryTaskAndBuildsUndo`,
       `TestPurgeAllTasks_PurgeErrorReturnsErrMsg`,
       `TestPurgeAllTasks_FallsBackToNumericIDWhenUUIDMissing`).
    2. ✅ **Confirmation popup + keybinding wiring (done, uncommitted)** —
       `X` (only on the Deleted tab, no-op elsewhere/when empty) arms a new
       `purgingAll` model flag; `y`/`enter` dispatches `purgeAllTasks` over
       every task the (filtered) list currently holds, `n`/`esc` cancels.
       Confirmation popup (`popup.DangerConfirmBox`) states the exact
       count ("Permanently delete all N deleted tasks? This cannot be
       undone.", singular "task" for N=1). `tasksPurgedMsg`/
       `tasksPurgeErrMsg` wired into `Update` identically to the
       single-task purge path (push undo action + refresh tasks/projects/
       tags/auto-sync, or show an error popup). Status bar shows an
       `X`/"purge all" hint only on the Deleted tab. Needed one small
       addition to `internal/ui/tasklist`: a new `Model.Tasks()` accessor
       returning the full current (already status/project/tag-filtered)
       task slice, since bulk actions need more than just the cursor's
       selection. Covered by 11 new tests in
       `cmd/lazytask/bulkpurge_test.go` (keybinding gating on tab/
       emptiness, confirm/cancel, refresh-on-success, error-on-failure,
       exact-count popup text incl. singular noun, status-bar hint
       gating).
    3. *(Not needed — chunks 1+2 together stayed within the bounded
       diff-size rule in Section 6; "Purge all" is now fully implemented.)*
- [ ] **Keyboard hints popup via `?`** — a help overlay listing current key
      bindings. **Scope decided 2026-09-30 (human sign-off):** status bar
      keeps a short always-visible "core" set (`0-4/tab`, `a`, `u`,
      `ctrl+r`, `S`, `?`, `q`, plus per-panel `↑/k`/`↓/j` nav only); every
      other real, working keybinding (tabs `[`/`]`, `d`, `x`/purge, `X`
      purge-all, `r` reopen/restore, `e`, `h/m/l` priority, `D` due date,
      `p` project filter, `ctrl+j/k` reorder, `/`/`n`/`N` search on both
      Tasks and Projects panels, `R` rename) is documented only in the `?`
      popup, grouped by panel section, as a static always-fully-populated
      reference (not filtered live by current tab, unlike the status
      bar — tab-specific behavior is called out in the label text
      instead, e.g. "delete (Todo/Done) / purge (Deleted)"). `?` only
      triggers the popup when no other modal/text-input is capturing
      keystrokes (so it still types as a literal `?` inside the Add Task
      form, search box, rename/reassign popups, etc.); any keypress
      dismisses the popup once open (pure reference, no confirm/cancel
      distinction).
      - ✅ **Implementation (done, uncommitted)** — new
        `internal/ui/helppopup` package (`Section`/`Box`, styled like the
        other popups with `panel.FocusedColor`-equivalent green border);
        `model.showingHelp` + `updateShowingHelp` wired the same way as
        every other modal short-circuit in `Update`; `helpSections()` in
        `cmd/lazytask/main.go` is the single source of truth for the full
        reference; `globalBindings`/`tasksLocalBindings`/
        `projectsLocalBindings` trimmed to the decided "core" set.
        **Diff-size note:** came out to ~450 changed/added lines across 5
        files (new package + tests, `main.go`, `main_test.go`,
        `bulkpurge_test.go`) — over Section 6's ~150–250 line target,
        though within the 4–6 file guideline. Flagging per Section 6's
        hard-stop rule rather than silently exceeding it; splitting
        further after the fact would mean re-doing completed, tested
        work, so reporting as one chunk for human review/split decision
        rather than proceeding to anything else. `go build ./...` and
        `go test ./...` both pass.
      - ✅ **Fine-tune, 2026-09-30 — scrolling + real palette color
        (uncommitted).** Two follow-up tweaks after initial review: (1)
        **Color bug fix** — the popup's border/title had actually used an
        arbitrary 256-color literal (`"42"`) rather than reusing
        `panel.FocusedColor` (lazygit's real default `activeBorderColor`,
        ANSI green `"2"`) the way `datepick`/`addform` already do; fixed
        to import and use `panel.FocusedColor` directly, restoring
        consistency with the rest of the app's lazygit-matched palette
        (per `panel.go`'s documented color roles) rather than the user
        having to eyeball colors against their own terminal theme. (2)
        **Scrolling** — the reference (35 lines across 4 sections) no
        longer fits in one screen; `helppopup.Box` now takes a
        `scrollOffset` and renders a fixed `maxVisibleRows` (14) viewport
        window plus a "N-M/total ↑/↓ scroll" footer hint, with
        `helppopup.MaxScroll(sections)` as the clamp bound.
        `model.helpScroll` + `updateShowingHelp` handle `up`/`k`
        (scroll back), `down`/`j` (scroll forward, clamped), and
        `esc`/`?` (close, resetting scroll to 0); other keys are now a
        no-op instead of "any key closes" (needed since arrows/j/k are
        reserved for scrolling). `go build ./...` and `go test ./...`
        both still pass.
      - ✅ **Fine-tune, 2026-09-30 — lazygit-matching redesign
        (uncommitted).** After the user asked me to research how lazygit
        itself implements this menu (cloned `jesseduffield/lazygit` and
        read `options_menu_action.go`/`menu_panel.go`/`menu_context.go`/
        `list_renderer.go`/`english.go`), the popup was reworked to
        actually match lazygit's real "Keybindings" menu conventions
        rather than a static all-panels dump:
        - **Context-sensitive Local/Global/Navigation**, computed live
          per current focus (and, for Tasks, the active tab) instead of
          a static reference — mirrors lazygit's own
          `getBindings(ctx)`. Local is per-panel (Tasks: tab-aware
          done/delete/purge/reopen/restore plus edit/priority/due
          date/project filter/reorder/search; Projects: rename/search;
          omitted entirely for Status/Details/Tags, which have nothing
          panel-specific to show). Global now excludes `0-4/tab` (moved
          to Navigation, matching lazygit's own split — panel-switching
          is movement, not a data action). Navigation always has
          `0-4`/`tab`/`shift+tab`; `↑/k`/`↓/j` only appear there while a
          list panel (Tasks/Projects/Tags) has focus.
        - **Title-in-border** — popup title changed from a body line
          reading "Keyboard Shortcuts" to lazygit's own menu title
          "Keybindings", now embedded in the top border via this repo's
          existing `panel.Frame` helper (the same title-in-border
          convention `tasklist`/`projects`/`tags` already use), instead
          of a separate bordered box built by hand.
        - **Green rule-style section headers** — `"─── Local"` etc.,
          bold + `panel.FocusedColor`, matching lazygit's
          `formatListSectionHeader` + `FgGreen.SetBold()`.
        - **Right-aligned key column** — key width now computed
          dynamically per-render from the longest key actually present
          (was previously left-aligned in a fixed-width column),
          matching lazygit's `AlignRight` key column. The scroll-
          position/`esc`/`?`-close hint moved into `panel.Frame`'s
          bottom-border footer parameter (lazygit-style, e.g. a
          `"3/10"`-style indicator baked into the border line) rather
          than a separate in-body hint line.
        - **Bug fix caught during manual visual QA**: the right-align
          padding math initially used `len()` (byte count) rather than
          display-column width, so keys containing multi-byte-but-
          single-column runes (`↑/k`, `↓/j`) were under-padded and
          didn't actually line up with same-width ASCII keys (`0-4`,
          `tab`). Fixed by switching `keyWidth`/`padLeft` to
          `ansi.StringWidth` (the same display-width-aware approach
          `popup.Overlay` already uses elsewhere in this codebase),
          verified via a manual demo render at both 80- and 44-column
          widths.
        - `helpSections()` in `main.go` became a `model` method (was a
          package-level static function) since it now needs `m.focus`/
          `m.list.Status()`; all tests that previously called it as a
          free function were rewritten to construct a `model` with the
          relevant focus/tab and call `m.helpSections()`.
        `go build ./...`, `go test ./...`, `gofmt -l`, and `go vet ./...`
        all pass/clean. Manually verified (throwaway demo renders, no
        committed test files) that each focus state yields the expected
        section set: Tasks/Todo → Local(10)+Global(6)+Nav(4); Projects →
        Local(3)+Global(6)+Nav(4); Tags → Global(6)+Nav(4, no Local);
        Status/Details → Global(6)+Nav(2, no Local, no up/down since
        those panels aren't list-navigable).
      - ✅ **Fine-tune, 2026-09-30 — corrections after side-by-side
        screenshot review against real lazygit (uncommitted).** The
        previous round's re-implementation still didn't match lazygit's
        actual rendered "Keybindings" menu once compared screenshot-to-
        screenshot; re-verified against `jesseduffield/lazygit`'s real
        source (`menu_context.go`'s `GetDisplayStrings`) rather than
        assumptions, and fixed:
        - **Key column color**: was blue (copied from this app's own
          status bar), should be (and now is) **cyan** — lazygit
          hardcodes `style.FgCyan` for the menu's key column regardless
          of theme, independent of the status bar's own
          `optionsTextColor` (which really is blue, so that assumption
          wasn't unreasonable, just the wrong source to copy from).
        - **Description column color**: was dimmed gray, should be (and
          now is) **plain/unstyled** — lazygit renders menu item
          descriptions in the terminal's default foreground, not dimmed.
        - **Section header format**: changed from a left-only
          `"─── Title"` rule to a symmetric **`"--- Title ---"`** rule
          (both sides), matching the actual rendered menu in the
          reference screenshot, and now **indented to start under the
          description column** (after the key column's width) rather
          than flush-left — lazygit's own header rows occupy the
          description column, leaving the key column blank for that row.
        - **Border/title color**: was reusing `panel.FocusedColor`
          (green, the "this panel is focused" convention used
          elsewhere in this app); lazygit's actual Keybindings menu uses
          a **neutral/default-colored** border+title instead (only the
          green is on the section headers) — `panel.Frame` is now called
          with `focused=false`.
        - **Size**: the popup was a small, fixed ~74×16 box; a full
          screenshot of lazygit's real menu (`lazygit-no-crop.png`,
          provided by the user for direct comparison against the running
          app) shows a much larger box, roughly 35-60% of the terminal's
          width/height and scaling with it. `helppopup.Box` gained a
          `screenHeight` parameter and a new `VisibleRows(screenHeight)`
          function that scales the popup's visible-row budget between
          `minVisibleRows` (8) and `maxVisibleRows` (40) based on the
          real terminal height (`reservedRows` (6) reserved for the
          border + a small edge margin), replacing the old hardcoded
          14-row constant. `MaxScroll` and `Box` both take the new
          `screenHeight` argument; `main.go`'s call sites now pass the
          model's real `height` instead of a bare scroll offset.
        All touched packages (`internal/ui/helppopup`, `cmd/lazytask`)
        rebuilt/retested: `go build ./...`, `go test ./...`, `gofmt -l`,
        `go vet ./...` all pass/clean. Manually re-rendered the full app
        view (120x45, help open, default/Tasks-Todo focus) to confirm all
        three sections (Local/Global/Navigation) now fit on one screen
        without scrolling at a realistic terminal size, with the
        corrected header format/indent and colors.

- [ ] **Redo status bar → "Suggested" bar (scope decided 2026-09-30, human
      sign-off).** Theme: "what's next for the user." The bottom bar stops
      showing keybinding hints entirely (the `?` popup is now the single
      source of truth for those) and instead renders one dynamic
      "Suggested" line pointing at the single most relevant task, in this
      priority order (highest to lowest):
      1. A **started** task (native taskwarrior `start`/`stop`
         attribute — "what I'm actively doing right now"). If more than
         one task is started, needs a tie-break rule (e.g. most-recently
         started, or highest urgency among started).
      2. A **`+next`-tagged** task (native taskwarrior tag convention —
         "queued up to do soon"), if nothing is started.
      3. An **orphan task** (no project assigned — see "Funnel for adhoc
         tasks" below), if nothing is started/next-tagged. This is
         intentionally ranked *below* started/next: an explicit "I'm
         doing this" or "I'm about to do this" signal always outranks
         "this needs a home."
      4. Otherwise, the **highest-urgency** task (existing taskwarrior
         `Urgency`/`UrgencyOffset` sort), as today's fallback.
      - New keybindings: `s` toggles start/stop on the selected task
        (writes/clears taskwarrior's `start` attribute — `S` is already
        taken by manual sync). `n` toggles the `next` tag on the
        selected task, reusing the existing `n`/`N` keys — no real
        conflict, since `n`/`N` currently only mean "next/prev search
        match" while a `/` search is active; outside of search mode
        they're free to mean "toggle next-tag" instead.
      - `f` focuses the Tasks panel, applies the existing `p` project
        filter to whichever project the Suggested task belongs to (no-op
        for orphan tasks, which have none), and moves the cursor to that
        task — so the user can jump straight to acting on it.
      - Display rule still needed for a task that is both started *and*
        `+next`-tagged (e.g. label reads "started", precedence note only,
        since rank 1 already covers it — no separate combined state
        needed given the ranking above).
      - Needs follow-up chunking: read/write plumbing for `start`/`stop`
        and the `next` tag in `internal/taskwarrior`, the ranking/
        selection logic, the new `Suggested` render replacing
        `statusbar.Render`, and the `s`/`n`/`f` keybinding wiring.
- [ ] **Funnel for adhoc tasks (scope decided 2026-09-30, human
      sign-off).** Reframed from "a quick-capture flow" to **orphan
      detection**: most adhoc/one-off tasks are expected to arrive via
      the native `task add` CLI (not lazytask's own Add Task form), so no
      new capture UI is needed. Instead, any task with no project
      assigned — regardless of whether it originated from the CLI or
      from lazytask's own `a` Add Task form (which already allows
      skipping project) — is "funneled" into visibility via rank 3 of
      the Suggested bar above, prompting the user to give it a project
      home. This item is effectively satisfied by the Suggested bar's
      orphan-ranking behavior; no separate implementation is expected
      beyond that.

### Phase 7 — Tags (continuation)

Tags panel (key 4) has already shipped. Remaining scope:

- [ ] 'T' from the tasks panel will open a popup for tagging re-assign, allow for existing tags to be selected from a list. Tags will be appended if selected.
      If keyed in, there will be a comma delimited list and the mode will be replacement instead.
- [ ] 't' from the tasks panel will open a popup for a quick tag picker for filtering
- [ ] Add tags to the add form, should come before due date
- [ ] Add '/' search to tags just like as seen in projects

## 8 Future Ideas

- **Configurable keymaps via YAML** — user-overridable key bindings for
  existing actions (list nav, add/done/delete/edit), loaded via the existing
  YAML config plumbing. Needs a decision on config file location/precedence
  before chunking.
- **Custom user-defined tasks/actions via YAML** — let users define their
  own quick-actions (e.g. canned `task` command templates) in config. Needs
  a decision on the templating/placeholder syntax before chunking.

## 9. Open Questions / Assumptions Log

- Assuming taskwarrior (`task` binary) is already installed on the target
  dev machine and CI runners used for integration tests; if not,
  integration tests should skip rather than fail.
- Assuming single-user, local-only taskwarrior data (no multi-context /
  multi-profile support in v1).
- Assuming terminal true-color support is not required; Lip Gloss should
  degrade gracefully on 16/256-color terminals.
