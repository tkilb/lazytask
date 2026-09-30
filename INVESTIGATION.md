# Investigation: intermittent "flaky and laggy" UI behavior (git-sync related)

**Status:** open, unassigned. Written 2026-09-30 for a follow-up agent to pick
up — no fix has been attempted yet beyond the hypothesis/evidence below.

## Symptom (as reported by the human)

> "I am noticing some flaky and laggy behavior, I think this is related to
> the git remote work."

No exact repro steps were given yet (just-noticed, intermittent). This doc
exists to hand off a strong, evidence-backed starting hypothesis rather than
starting from zero.

## Important context: uncommitted changes in the working tree right now

Before starting, know that `git status` currently shows these **uncommitted**
changes, all from this same session, immediately prior to the human's report:

```
 M PLAN.md
 M cmd/lazytask/main.go
 M internal/taskwarrior/client.go
 M internal/taskwarrior/client_test.go
 M internal/taskwarrior/mutations.go
 M internal/ui/tasklist/model.go
?? cmd/lazytask/bulkpurge_test.go
```

Two unrelated pieces of work are mixed into this diff:

1. **"Purge all" bulk-purge feature** (`cmd/lazytask/main.go`'s `X` key,
   `purgeAllTasks`, `internal/ui/tasklist.Model.Tasks()`) — new feature,
   almost certainly **not** related to this symptom (it doesn't touch
   sync/concurrency at all beyond calling the now-shared `Purge`/`Import`
   methods sequentially, same as the pre-existing single-task purge).
2. **A fix for a *different*, already-diagnosed bug**
   (`internal/taskwarrior/client.go` / `mutations.go`): intermittent
   `"database is locked: Error code 5: database is locked"` errors, caused
   by `Client` having zero serialization across its `task` subprocess
   invocations. The fix added a single `sync.Mutex` (`Client.mu`) and wrapped
   **every** subprocess call (`run()`, `Export`, `Import`, `Sync`) in
   `Lock()`/`Unlock()` around `cmd.Run()`. This was verified to fix the
   locking bug (see `internal/taskwarrior/client_test.go`'s
   `TestClient_ConcurrentInvocations_NoDatabaseLocked`, and the manual
   before/after repro described in its doc comment: 30+ failures per run
   without the mutex, 0 with it).

**This mutex is the prime suspect for the new "laggy" complaint** — see
Hypothesis 1 below. It was added in direct response to a different bug
report in this same session, so the timing lines up: the human's very next
observation after that fix landed (uncommitted) was "flaky and laggy."

Separately, `git log --oneline` shows auto-sync (periodic + on-mutation) was
already committed *before* this session (`38bd9ff feat: adds auto sync to
lazytask`), so it's also possible the laggy/flaky behavior **predates** the
mutex change and is a standalone auto-sync issue — see Hypothesis 2.
**The first thing the follow-up agent should do is establish which of these
is actually in play** (see "How to narrow it down" below) before changing
anything.

## Hypothesis 1 (leading theory): the new global mutex serializes everything
behind an unbounded-duration `task sync`, including real network I/O

### The mechanism

- `Client.mu` (added this session) is a single, coarse `sync.Mutex` shared by
  *every* method on the one `*Client` instance the whole app uses
  (`cmd/lazytask/main.go:653`, `client := taskwarrior.NewClient()`). That
  includes `Export` (used for every tasks/projects/tags list refresh),
  `Add`/`Done`/`Delete`/`Restore`/`Purge`/`SetPriority`/`SetDue`/`SetProject`/
  `SetUrgencyOffset`/`Annotate`/`Import` (via the shared `run()` helper), and
  **`Sync`** (`task sync`, which performs real git network I/O — fetch +
  push to whatever remote `sync.git.remote` points at).
- `Sync` is called from three places, **none of which set a context
  deadline** — every single call site uses bare `context.Background()`:
  - Manual `S` key → `runSync` (`cmd/lazytask/main.go:~1390`).
  - Periodic auto-sync tick, default every 5 minutes
    (`autoSyncTickMsg` handling, `cmd/lazytask/main.go:~1995`, via
    `runAutoSync`).
  - On-mutation debounced auto-sync, 2s after *any* local mutation
    (`mutationSyncDebounceMsg` handling, `cmd/lazytask/main.go:~2010`, via
    the same `runAutoSync`).
- Bubbletea runs each `tea.Cmd` in its own goroutine, so the `Update` loop
  itself isn't blocked by a slow `Sync` — but the **result** the user is
  waiting on (e.g. the task list refreshing after pressing `d` to mark
  something done) *is* blocked, because `fetchTasks`/`fetchProjects`/
  `fetchTags` all call `Export`, which now must wait for `Client.mu`, which
  a concurrently-running `Sync` may be holding for as long as its network
  I/O takes.
- If the configured git remote is slow, temporarily unreachable (VPN
  drop, flaky wifi, DNS hiccup, SSH auth prompt, etc.), `git fetch`/`git
  push` underneath `task sync` can hang for a long time — with no
  `context.WithTimeout` anywhere in the call chain, there's nothing in
  lazytask bounding that wait; worst case is whatever the OS/git/SSH
  default connect-timeout is (can be 30s–2min+).
- **Net effect:** every ~5 minutes (periodic tick) and ~2s after *every*
  keystroke that mutates a task (debounced auto-sync), a `Sync` call can
  acquire the global mutex and hold it — potentially for a long,
  network-dependent, unbounded duration — during which **every other**
  `task` invocation in the app (every list refresh, every new mutation)
  queues up behind it. This matches both halves of the reported symptom:
  - **"laggy"** — normal actions (done/delete/edit/etc.) stall waiting for
    the mutex.
  - **"flaky" / "from time to time"** — it only manifests when a sync
    happens to be slow (network-dependent), not on every action, and lines
    up with the ~5-minute periodic cadence or shortly after any edit.

### Why this is a regression specifically introduced by the mutex fix

Before the mutex existed, an in-flight `Sync` did **not** block other
`task` subprocesses — they ran fully concurrently (which is *why* the
"database is locked" bug existed in the first place). So previously, a slow
`Sync` could corrupt/collide with a concurrent `Export`/`Add` (the original
bug), but it did **not** make the rest of the UI feel laggy, because nothing
was waiting on it. The mutex traded "occasional data-layer error" for
"occasional full-app stall," without adding any bound on how long that stall
can last.

## Hypothesis 2: auto-sync itself was already janky before the mutex, independent of locking

Even ignoring the mutex, `38bd9ff` (already committed, pre-dating this
session) introduced two unconditional, automatic triggers for a
potentially-slow network operation:

- A periodic ticker, default every 5 minutes
  (`sync.git.auto_sync_interval_minutes`, `autoSyncTick`).
- A 2-second-debounced on-mutation trigger firing after *any* of 10 mutation
  types (`scheduleMutationAutoSync`).

Even without the new mutex, if `task sync` itself is slow (e.g. the git
remote is far away, the repo has grown large — see the still-unbuilt
history-purge routine in `PLAN.md`'s Phase 5 notes, which exists specifically
because this repo's history is expected to grow unbounded without it — or
squash/gc hasn't run and packfiles are large), TaskChampion's own sync
process might itself intermittently contend with a concurrent `Export`/
mutation at the SQLite/version-file level (this is plausible but not
confirmed — unlike Hypothesis 1, no direct repro has been attempted for this
angle yet). Worth ruling in/out independently of Hypothesis 1.

## How to narrow it down (recommended first steps)

1. **Isolate the mutex's contribution.** Temporarily stash/revert just the
   `Client.mu` changes (`internal/taskwarrior/client.go`,
   `internal/taskwarrior/mutations.go`) while keeping everything else, and
   see if the laggy feeling goes away. Conversely, test with the mutex in
   place but `sync.git.*` **unconfigured** (so `Sync` is never actually
   called) — if lag disappears, that isolates it to `Sync` specifically
   rather than the mutex's effect on `Export`/`Add`/etc. in general.
2. **Reproduce with a deliberately slow/unreachable remote.** Point
   `sync.git.remote` at an unroutable address (e.g. `10.255.255.1`, a
   blackhole IP) or a real but firewalled host, then perform a mutation and
   observe whether every subsequent keypress stalls for a long, consistent
   duration (this would strongly confirm Hypothesis 1). The existing
   integration test harness in `internal/taskwarrior/client_test.go`
   (`TestClient_ConcurrentInvocations_NoDatabaseLocked`) already shows the
   pattern for spinning up an isolated `TASKDATA`/`TASKRC`/bare-git-remote
   sandbox — a similar test could measure *latency* of a concurrent
   `Export` while a slow `Sync` is in flight, not just correctness.
3. **Check for any lag even with git-sync fully unconfigured.** If the human
   also sees lag/flakiness with no `sync.git.*` set up at all (so `Sync` is
   never invoked), that would point away from both hypotheses above and
   toward something else entirely (e.g. the serialized `Export ×3`
   refresh pattern below, or something unrelated to sync at all).
4. **Note the mutex also fully serializes `fetchTasks`/`fetchProjects`/
   `fetchTags`**, which previously ran as three concurrent `task export`
   subprocesses (dispatched together via `tea.Batch` after every mutation,
   see the repeated pattern at `cmd/lazytask/main.go` lines ~1915–1973).
   With the mutex, these now run strictly sequentially, roughly tripling
   the wall-clock cost of every post-mutation refresh. This alone is a
   smaller, constant-factor slowdown (not "flaky," always-on) — worth
   ruling in/out separately from the sync-related freeze, since it could
   compound with it.

## Suggested fix directions (not yet attempted — evaluate after confirming the hypothesis)

- **Add a context timeout to every `Sync` call site** (`runSync`,
  `runAutoSync`'s two call sites) — e.g. a configurable
  `sync.git.timeout_seconds` (suggest defaulting to something like 15–30s).
  This bounds the worst case even if the mutex stays coarse-grained, turning
  an indefinite freeze into a bounded, user-visible failure (already has a
  reporting path via `autoSyncErrMsg`/`taskSyncErrMsg`).
- **Consider narrowing the mutex's scope.** The original bug this mutex
  fixed needs confirming: does a concurrent `Export` actually collide with
  an in-flight `Sync` at the SQLite level, or only concurrent *writes*
  (`Add`/`Done`/`Delete`/etc.) do? If reads are actually safe to run
  alongside `Sync`, a `sync.RWMutex` (reads take `RLock`, writes/`Sync` take
  `Lock`) could restore most of the lost concurrency while keeping the
  locking-bug fix. **This needs to be verified empirically** (extend the
  stress-test methodology in `client_test.go`) before assuming it's safe —
  `task sync` itself does read-modify-write against the local replica
  (pull + merge before push), so it may need to be treated as a writer
  either way.
  - Trade note: if regressing to an RWMutex, the pre-existing lock bug was
    reproduced specifically via **concurrent `Add` + `Sync`** in the test
    that validated the current fix — so at minimum, `Sync` must still
    exclude all other writers. It's specifically *read* concurrency
    (`Export` while `Sync` is in flight) that's unconfirmed either way and
    worth testing before loosening it.
- **Consider whether auto-sync should hold any lock at all**, or whether it
  should instead skip/reschedule itself if it can't acquire the lock
  promptly (non-blocking `TryLock`), rather than ever blocking user-facing
  operations behind a background sync the user didn't explicitly ask for
  (the manual `S` key blocking is more acceptable — the user explicitly
  asked for it and gets a popup either way).
- Independently of the above, if Hypothesis 2 is confirmed to matter, revisit
  the auto-sync intervals/debounce decided in `PLAN.md`'s Phase 5 section
  (5 min periodic / 2s on-mutation) — these were human decisions recorded
  2026-09-29 and would need explicit human sign-off to change, per this
  repo's `PLAN.md` Section 5 (human-in-the-loop checkpoints) — don't treat
  this doc as pre-approval to change them.

## Relevant files

- `internal/taskwarrior/client.go` — `Client.mu`, `Sync`, `Export`.
- `internal/taskwarrior/mutations.go` — shared `run()` helper (also holds
  `Client.mu`), `Import`.
- `internal/taskwarrior/client_test.go` —
  `TestClient_ConcurrentInvocations_NoDatabaseLocked`, the existing
  stress-test sandbox pattern to extend for latency measurement.
- `cmd/lazytask/main.go` — `runSync`, `runAutoSync`, `autoSyncTick`,
  `scheduleMutationAutoSync`, `mutationSyncDebounce`, and every mutation's
  `fetchTasks`/`fetchProjects`/`fetchTags`/`scheduleMutationAutoSync`
  `tea.Batch` call sites (search for `scheduleMutationAutoSync` to find all
  ~10).
- `PLAN.md`, Phase 5 section — full design history of the git-sync backend,
  auto-sync decisions (dates/rationale), and the *not yet built*
  history-purge routine (relevant background: repo history is expected to
  grow unbounded without it, which could make `Sync` progressively slower
  over time on a long-lived install).

## Open questions for the human (don't assume answers)

- Does the lag/flakiness happen with `sync.git.*` **unconfigured**? (Rules
  Hypotheses 1 and 2 in/out entirely.)
- Roughly how long does a "laggy" pause last — sub-second, a few seconds, or
  tens of seconds+? (Distinguishes "just slower because serialized" from
  "stalled behind a real network wait.")
- Does it correlate with network conditions (e.g. worse on wifi/VPN, fine on
  wired/no-VPN)?
- Is `sync.git.remote` a local path, LAN host, or real internet remote
  (e.g. GitHub)? Latency/reachability characteristics differ a lot.
