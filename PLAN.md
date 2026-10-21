# lazytask — Chunked Delivery Requirements

Status: v1 and all phases through Phase 7 are complete and have been removed
from this document (see `git log` for history). This doc now holds the
ongoing process rules (Sections 1-5) and the work that is still outstanding
(Section 6), for a driving agent ("driver") that breaks work into small,
independently-committable worker tasks, delivered **one chunk at a time with
a mandatory human checkpoint between chunks.**

This is explicitly **not** meant to run unattended. A human must review and
approve after every chunk before the next one starts.

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

Gotchas that are easy to rediscover the hard way:

- Filters passed to `task` must be **separate argv elements**
  (`"project:work", "+urgent"`), never one space-joined string — taskwarrior
  treats a single argument containing a space as one unparsed word and
  matches nothing.
- The saved project/tag filter lives at `os.TempDir()/lazytask/state.json`.
  Tests must never write the real one: `cmd/lazytask`'s `TestMain` points
  `$TMPDIR` at a throwaway dir, and any other package whose tests save state
  must do the same.
- Sync concurrency pitfalls are documented in `SYNC_GOTCHAS.md` (the `Client`
  mutex and the preemptible `SyncBackground`); read it before touching sync.

## 3. Testing Bar (per chunk)

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

## 4. Human-in-the-Loop Checkpoints (mandatory, non-negotiable)

The driving agent must treat every chunk as a **hard stop boundary**. The
process for each chunk is:

1. **Announce before starting** — state which single chunk (by name/number)
   is about to be worked on and its stated objective. Do not start work on a
   chunk that wasn't just approved.
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
   the current chunk, or stop entirely. Never infer approval from silence,
   and never treat "looks fine" as a request to also start the next chunk
   unless the human says so.
5. **Commit only when asked.** The agent does not commit or push on its own;
   the human reviews the working tree and either commits or asks the agent to.
6. **One chunk in flight at any time.**

If at any point the driver is unsure whether it has approval to proceed, it
must stop and ask rather than assume.

## 5. Chunking Rules (credit-control guardrails)

"Small" here is measured in **diff size and objective count**, not human
clock-time:

- **One objective per chunk.** A chunk implements exactly one capability (or
  one clearly-named sub-piece of it, if the full feature is too large).
- **Bounded diff size.** Target ~150–250 changed lines (excluding
  generated/vendor/test-fixture files) and no more than ~4–6 touched files
  per chunk. If a worker agent estimates a chunk will exceed this, it must
  stop and report back for the chunk to be split.
- **One commit per chunk**, buildable and passing tests on its own
  (`go build ./...` and `go test ./...` must both succeed before the chunk
  is considered done). Never leave the tree in a broken intermediate state.
- **No speculative work.** Agents must not implement later-phase features,
  refactor unrelated code, add dependencies not listed in Section 2, or fetch
  external repos/docs "for inspiration" mid-task.
- **Keep this document current.** Before reporting a chunk as done, and
  right after any planning discussion that resolves an open question, update
  this file with (a) the decision just made, so it isn't re-litigated, (b)
  the chunk's status, and (c) any new sub-tasks. **When an item is finished,
  delete it** rather than marking it done — this file is read by every new
  agent, so finished work is wasted context (history lives in `git log`).
- **Hard stop conditions** — a worker agent must halt and report rather than
  continue if it: needs a new third-party dependency, needs to modify more
  than one chunk's worth of prior work, encounters ambiguity in this spec,
  or is about to exceed the diff-size bound above.
- **Explicit interfaces first.** Where a chunk depends on another
  not-yet-built chunk, define the minimal Go interface/struct it needs and
  stub it, rather than blocking.

## 6. Outstanding Work

Nothing below may be started without explicit human sign-off, per Section 4.

### Sync history purge routine (Phase 5 remainder)

Git-sync (config, manual `S`, auto-sync, secret management, `sync-reset`) is
done. What remains is purging old data from the sync repo's git history.
Nothing is implemented yet: no retention config and no purge code exist.

**Goal.** TaskChampion's built-in version-file cleanup only prunes
already-snapshotted files, uses a hardcoded 180-day retention (not
configurable via `task config`), and does **not** rewrite git history — old
encrypted blobs stay recoverable from git objects until history is rewritten
and garbage-collected. To actually purge data older than **one week**,
lazytask needs its own maintenance routine.

**Decisions (human, 2026-09-29):**

- The routine is **lazytask-internal**: it runs on a schedule inside
  lazytask's own process, not from an external cron script.
- The retention window is **YAML-configurable**, default 7 days, under
  `sync.git.*` (exact key name TBD at chunking time, following the existing
  `sync.git.*` convention in `internal/config.GitSyncConfig`).
- No multi-device coordination safeguard is needed (see "Multi-device" below).

**How the git-sync backend stores data** (from TaskChampion's
`src/server/gitsync/mod.rs`, `task` 3.5.0 / PR #4111):

- Each sync writes one immutable file `v-{parent_uuid}-{child_uuid}`
  containing an encrypted history segment. `snapshot` and `meta` files are
  overwritten in place as newer snapshots/version pointers are produced.
- Its own `cleanup()` only `git rm`s version files that are covered by a
  snapshot and whose last-touching commit is older than the hardcoded
  180-day constant. That stages a `git rm` + commit; it does **not** rewrite
  history, so deleted blobs remain in older commit objects.
- TaskChampion never reads commit _history_ for sync correctness — the only
  historical operation is `git log -1 --format=%ct -- <file>` to compute a
  file's age. Otherwise it only cares about the working tree at `HEAD`, so a
  full history rewrite is safe as long as the final `HEAD` tree (meta,
  snapshot, surviving version files) is preserved byte-for-byte.

**Proposed purge routine:**

1. Take a lock on the sync repo directory so this never races an in-flight
   `task sync` (reuse the `Client` mutex / preemption approach in
   `SYNC_GOTCHAS.md`).
2. `git fetch` and fast-forward to the true remote tip first — never rewrite
   from a stale local view.
3. Squash instead of selectively rewriting: `git checkout --orphan purge-tmp
   && git add -A && git commit` creates one parentless commit holding exactly
   the current tree.
4. Rename `purge-tmp` over the configured `sync.git.branch`, then `git push
   --force` to `sync.git.remote`.
5. Locally: `git reflog expire --expire=now --all && git gc --prune=now
   --aggressive` to drop the now-unreachable objects.
6. If the push is rejected (a `task sync` landed mid-purge), abort and retry
   from step 2. Do not reuse TaskChampion's pull-and-reset retry logic; it is
   designed for appending versions, not rewriting history.

**Notes for whoever chunks this:**

- Squashing resets TaskChampion's per-file "age" tracking (derived from `git
  log`), restarting its 180-day cleanup timer. Harmless, since the 7-day
  purge supersedes it, but document it so it isn't mistaken for a bug.
- **Multi-device:** not a risk. Every TaskChampion sync calls
  `reset_to_remote()` (`git fetch` into `FETCH_HEAD`, then an unconditional
  `git reset --hard FETCH_HEAD`), which has no dependency on shared ancestry.
  Other devices self-heal on their next `task sync` after a force-push; no
  re-clone or "rewritten root" detection is needed.
- **Mitigation, not a guarantee.** Hosts such as GitHub commonly keep
  unreachable objects for a grace window (historically up to ~90 days), and
  any existing fork/mirror/backup keeps full history. Tell the user this is
  defense-in-depth on top of keeping the repo private.

**Chunking plan:**

1. **Purge config plumbing** — add the retention-window field (default 7
   days) to `GitSyncConfig`/`LoadGitSyncConfig`, mirroring the existing
   pattern. Config surface and tests only, no purge logic.
2. **Purge git operations** — the squash/force-push/gc sequence above as a
   testable function (e.g. `taskwarrior.Client.PurgeSyncHistory`), invokable
   manually but not yet auto-triggered. The force-push touches a real remote:
   require explicit human confirmation before running it against one.
3. **Internal scheduling** — a background ticker in lazytask's lifecycle that
   compares last-purge time to the retention window, locked against a
   concurrent `task sync`. Reuse the quiet-failure pattern auto-sync uses
   (`autoSyncErrMsg`, status line, no popup).

## 7. Future Ideas

- **Configurable keymaps via YAML** — user-overridable key bindings for
  existing actions (list nav, add/done/delete/edit), loaded via the existing
  YAML config plumbing. Needs a decision on config file location/precedence
  before chunking.
- **Custom user-defined tasks/actions via YAML** — let users define their
  own quick-actions (e.g. canned `task` command templates) in config. Needs
  a decision on the templating/placeholder syntax before chunking.

## 8. Assumptions

- Taskwarrior (`task`) is installed on dev machines and CI runners; if not,
  integration tests skip rather than fail.
- Single-user, local data (no multi-context / multi-profile support).
- True-color terminal support is not required; Lip Gloss should degrade
  gracefully on 16/256-color terminals.
