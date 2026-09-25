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

## 7. Outstanding Phases to be Completed

### Phase 2b — Anotations

**Progress note for the next agent (uncommitted working tree as of this
handoff — the human reviews/commits by hand per §5, so `git status`/`git
diff` reflect exactly what's landed so far but not yet committed):**

- [x] **Chunk 1 (data model)** — `taskwarrior.Task` now has an
      `Annotations []Annotation` field (`Annotation{Entry, Description}`,
      matching taskwarrior's export JSON shape). See
      `internal/taskwarrior/task.go` / `task_test.go`. Note: the phrase
      "edit popup" in this doc turned out to mean the **Add Task popup**
      (`internal/ui/taskform`), not the `e`-key `$EDITOR` buffer flow —
      confirmed with the human; both still need follow-up, see below.
- [x] **Chunk 2 (Add popup fields)** — `internal/ui/taskform`: Description
      is a single-line `textinput` again; a new multi-line `Annotations`
      textarea field was added right after it (own field, own tab stop).
      While Annotations is focused, `<enter>` inserts a newline (bubbles'
      textarea default) instead of submitting the form —
      `cmd/lazytask/main.go`'s `updateAdding` special-cases
      `FocusedField() == taskform.FieldAnnotations` to skip the submit
      branch. Tests: `internal/ui/taskform/model_test.go`.
      **Known gap:** the typed Annotations text is NOT yet persisted —
      `updateAdding`'s submit path still only reads
      Description/Project/Priority/Due. Do not consider the Add popup
      "done" until this is wired (see Chunk 3 below).
- [x] **Chunk 3 (persistence + $EDITOR edit-buffer)** — done, verified
      against a real `task` 3.5.0 binary (not just unit tests):
      - `Client.Annotate(ctx, id, text)` added to
        `internal/taskwarrior/mutations.go` (+ `TaskMutator` interface) —
        shells out to `task <id> annotate <text>`; embedded newlines in
        `text` are passed straight through (`task` stores them verbatim in
        the JSON `description` field), so no special quoting was needed.
      - **Add popup**: `TaskAnnotator` interface + `model.annotator` field
        added in `cmd/lazytask/main.go`; `addTask` now takes the form's
        `Annotations()` text and, once `Add` succeeds, calls `Annotate`
        once per non-blank line (`splitAnnotationLines`) against the new
        task's numeric ID. Wired at the `updateAdding` "enter" submit
        branch.
      - **`$EDITOR` edit-buffer**: `internal/editbuffer` gained an
        `Annotations:` section (one `- <text>` line per annotation, right
        after `Description:`; embedded newlines within a single
        annotation are escaped as literal `\n` on Serialize and unescaped
        on Parse, rather than using Description's multi-line-continuation
        style). `EditableFields.Annotations []string` holds the parsed
        texts; `Apply` reconciles them against `original.Annotations`,
        preserving each unchanged annotation's original `Entry` timestamp
        and leaving new/edited text with an empty `Entry` (which
        Taskwarrior fills in on import). **No separate
        Annotate/denotate-reconciliation plumbing was needed for this
        path** — confirmed directly against a real `task` binary that
        `task import` with a full `annotations` array in the JSON payload
        (which `editTaskCallback` already sends via the existing
        `Client.Import`/`TaskImporter`) correctly adds, edits, *and
        removes* annotations to match whatever's provided, the same way
        it already does for Description/Project/etc. Omitting the
        `annotations` field entirely from the JSON (not what this code
        does) would instead wipe existing annotations — noted here so a
        future change to the Import payload doesn't reintroduce that
        footgun.
      - Tests: `internal/taskwarrior/mutations_test.go` (unit +
        integration, the latter gated on `task` being on `PATH`),
        `cmd/lazytask/main_test.go`
        (`TestModelUpdate_AddingSubmitWithAnnotations`),
        `internal/editbuffer/editbuffer_test.go` (Serialize/Parse/Apply
        round-trip, escaping, entry-preservation, deletion-to-nil cases).
      - Diff came in at 6 files / +348/−11 — over the ~150–250 line
        target (flagged per §6 at the time), justified by this being
        shared plumbing both the Add popup and edit-buffer flows need;
        human reviewed and did not request a split.
- [ ] **Chunk 4 — Details panel annotations + tasklist single-line safety —
      NOT STARTED.** Clarified scope (confirmed with the human, supersedes
      the original "multi-line rendering/wrapping" framing below):
      - **Tasks list panel** must never display Annotations at all (it
        already doesn't — no code path there touches `Task.Annotations`)
        and must remain **strictly single-line per task row, always
        truncated, never wrapped** — this includes a `Description`
        containing embedded newlines (possible today via the `$EDITOR`
        multi-line Description flow), which currently leaks a raw `\n`
        into the row's rendered string and visually breaks the row across
        multiple terminal lines. Fix: collapse/strip embedded newlines
        (and any other control characters that'd force a line break)
        before truncating in `internal/ui/tasklist/model.go`'s
        `renderDataRow`/`truncate`/`renderDescriptionCell` path — no row
        height calculation or wrapping should be introduced; truncation
        stays exactly as-is otherwise.
      - **Details panel** (`detailsPanelContent` in `cmd/lazytask/main.go`,
        key `0`) is the only place Annotations should ever be shown: add
        an `Annotations:` section there (it already renders arbitrary
        multi-line, read-only content for the selected task, so no new
        wrapping logic is needed there — just append the annotation text,
        one per line, e.g. prefixed with their Entry timestamp).
      - Depends on Chunk 3 existing so there's actually annotation data to
        show, though the tasklist single-line-safety fix could in
        principle be tackled standalone.

Per §5, Chunk 4 still needs its own announce → build → stop →
human-sign-off cycle. **Chunk 3's code is built and in the (uncommitted)
working tree as of this handoff, but has not yet been explicitly signed off
by the human** (see §5 step 4 — do not infer approval from silence or from
this note alone). Before starting Chunk 4, the next agent must: (a)
re-read this note and run `git status`/`git diff` to confirm the working
tree still matches what's described above (nothing else landed or reverted
it), and (b) get the human's explicit go-ahead to proceed — do not treat
"Chunk 3 looks done in the diff" as equivalent to sign-off.

### Phase 3 — Tags (continuation)

Tags panel (key 4) has already shipped. Remaining scope:

- [ ] 'T' from the tasks panel will open a popup for tagging re-assign, allow for existing tags to be selected from a list. Tags will be appended if selected.
      If keyed in, there will be a comma delimited list and the mode will be replacement instead.
- [ ] 't' from the tasks panel will open a popup for a quick tag picker for filtering
- [ ] Add tags to the add form, should come before due date
- [ ] Add '/' search to tags just like as seen in projects

### Phase 4 — UX Enhancements

- [ ] **Redo status bar, maybe rename to "suggested"** — revisit the status
      bar's current design/labeling.
- [ ] **Better tasklist column headers and date display** — improve column
      header clarity and how due/other dates are formatted in the task list.
- [ ] **Purge all** — bulk-purge action for tasks (needs scope/safety
      clarification before chunking — see Section 5's ask-before-assuming
      rule).
- [ ] **Keyboard hints popup via `?`** — a help overlay listing current key
      bindings.
- [ ] **Funnel for adhoc tasks** — a quick-capture flow for one-off/adhoc
      tasks (needs scope clarification before chunking).

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
  than hardcoded, consistent with the rest of Phase 5's config-driven
  items.

### Phase 6 — Configurable Keymaps & Custom User Actions

Always last, per standing instruction.

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
