# Sync gotchas (git-sync backend)

**Status:** resolved for now, documented here in case it needs revisiting.
Written 2026-09-30, follow-up to `INVESTIGATION.md` (that doc's leading
hypothesis was confirmed and fixed; kept for full context/history).

## What happened

A `sync.Mutex` was added to `taskwarrior.Client` to fix a real bug
("database is locked: Error code 5") caused by concurrent `task`
subprocess invocations. That fix introduced a regression: `task sync`
(real git network I/O, no timeout) could hold the mutex for its entire
network round-trip, blocking *every other* `task` call — including plain
list refreshes — behind it. Reported by the human as "flaky and laggy"
behavior, up to ~1-8s pauses, with `sync.git.remote` pointed at a real
internet remote (GitHub-style).

## Fix implemented

Only lazytask's own *automatic* sync triggers (periodic timer + 2s
on-mutation debounce) were changed — the manual `S` key is untouched and
still blocks as before.

- `Client.SyncBackground()` (used only by auto-sync) is **preemptible**:
  any foreground `task` call (`run`/`Export`/`Import`/`Sync`) cancels an
  in-flight background sync's context first. `exec.CommandContext` kills
  the subprocess on cancellation, so the foreground call only waits
  however long the subprocess takes to die (typically milliseconds), not
  the sync's full duration.
- A preempted background sync reports `context.Canceled` and is treated
  as a silent skip, not a reportable failure — it just tries again on the
  *next* scheduled tick/debounce (no immediate idle-retry).
- On quit (`q`/`ctrl+c`), instead of preempting/killing an in-flight
  background sync, lazytask calls `Client.AwaitBackgroundSync` and waits
  up to **15s** for it to finish on its own (so pending local changes get
  a chance to reach the remote), then quits regardless.

Relevant code: `internal/taskwarrior/client.go` (`SyncBackground`,
`AwaitBackgroundSync`, `preemptBackgroundSync`, `doSync`),
`cmd/lazytask/main.go` (`runAutoSync`, `quitCmd`, `quitSyncGracePeriod`).

## What this does *not* fix — revisit if needed

- **The manual `S` key is still fully blocking with no timeout.** A slow
  or unreachable remote can still hang the UI indefinitely on a manual
  sync. This was left alone deliberately (INVESTIGATION.md: "the manual
  `S` key blocking is more acceptable — the user explicitly asked for it
  and gets a popup either way"), but if manual syncs start feeling
  unacceptably long/hangy too, add a `context.WithTimeout` to `runSync`'s
  call site (suggested 15-30s, configurable via a new
  `sync.git.timeout_seconds`-style setting).
- **Under heavy, continuous mutation activity, background auto-sync may
  rarely complete a full round-trip** — every preemption just waits for
  the next tick/debounce rather than retrying as soon as the UI goes
  idle. This was an explicit trade-off (chosen over "retry on idle" for
  simplicity) — the remote copy can lag further behind than before during
  a long burst of edits. If that turns out to matter in practice (e.g.
  you notice sync basically never succeeds during active use), consider
  adding idle-detection retry.
- **The underlying mutex is still fully coarse-grained** (one
  `sync.Mutex` serializing every `task` call, reads included). Whether a
  narrower `sync.RWMutex` (reads run concurrently with a sync) would be
  safe at the SQLite/replica level was flagged in `INVESTIGATION.md` as
  *unconfirmed* and still is — nothing here tested or changed that. If
  contention/perf ever becomes a problem again beyond what preemption
  fixes, that's the next thing to empirically verify before attempting.
- **Quit's 15s grace period is unconditional** — it always spins up a
  waiter goroutine and does a `select`/timeout even when no sync is
  running (cheap/no-op in that case, but worth knowing it's not skipped
  outright based on some "is a sync active" check beyond the internal
  `sync.WaitGroup`).

## Tests covering this

- `internal/taskwarrior/client_test.go`:
  `TestClient_SyncBackground_PreemptedByForegroundCall`,
  `TestClient_AwaitBackgroundSync_WaitsForInFlightSync`,
  `TestClient_AwaitBackgroundSync_NoOpWhenIdle`.
- `cmd/lazytask/main_test.go`:
  `TestModelUpdate_Quit_AwaitsInFlightBackgroundSync`, plus existing
  `sync_test.go` coverage of the manual/auto sync message-handling paths
  (updated to route auto-sync through `SyncBackground`).
