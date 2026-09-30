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

- [ ] **Quick Duedate update ** — shift D on a existing task will summon a popup
      that uses the same duedate mechanism that add has.
- [ ] **Purge all** — bulk-purge action for tasks (needs scope/safety
      clarification before chunking — see Section 5's ask-before-assuming
      rule).
- [ ] **Keyboard hints popup via `?`** — a help overlay listing current key
      bindings.
      lets have a conversation of which keys show and which must be referenced via the help popup

Discussion for this:

- [ ] **Funnel for adhoc tasks** — a quick-capture flow for one-off/adhoc
      tasks (needs scope clarification before chunking).
- [ ] **Redo status bar, maybe rename to "suggested"** — revisit the status
      bar's current design/labeling.

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
