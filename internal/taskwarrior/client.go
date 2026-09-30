package taskwarrior

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// TaskReader provides read-only access to Taskwarrior tasks.
type TaskReader interface {
	Export(ctx context.Context, filters ...string) ([]Task, error)
}

// Client wraps the Taskwarrior CLI binary for executing operations.
//
// mu serializes every `task` subprocess invocation made through this
// Client (see run/Export/Import/Sync/SyncBackground), including reads.
// lazytask issues several independent tea.Cmds concurrently (e.g. a
// mutation's own command alongside the tasks/projects/tags refresh it
// triggers, or a background auto-sync tick landing mid-mutation), and
// Taskwarrior's newer SQLite-backed replica storage (task 3.x, used by
// the git-sync backend) does not tolerate concurrent processes touching
// the same data directory — concurrent invocations intermittently fail
// with "database is locked: Error code 5: database is locked". A single
// mutex per Client (there is exactly one shared Client instance for the
// whole app, see main.go) fully serializes lazytask's own `task` calls
// and eliminates that class of failure; it does not protect against a
// *different* process (e.g. a manual `task` invocation in another
// terminal) writing concurrently, which is an inherent limitation of
// Taskwarrior's own locking, not something lazytask can solve on its own.
//
// bgSyncMu/bgSyncCancel/bgSyncWG support SyncBackground's preemption: an
// in-flight automatic sync (periodic tick or on-mutation debounce) must
// never make a foreground, user-facing call wait for its full network
// round-trip. Every foreground method (run/Export/Import/Sync) calls
// preemptBackgroundSync first, which cancels SyncBackground's context —
// exec.CommandContext kills the underlying `task sync` subprocess as soon
// as that happens, freeing mu within milliseconds instead of however long
// the sync had left. See preemptBackgroundSync and SyncBackground.
type Client struct {
	binary   string
	taskData string
	taskRC   string
	environ  []string
	mu       sync.Mutex

	bgSyncMu     sync.Mutex
	bgSyncCancel context.CancelFunc
	bgSyncWG     sync.WaitGroup
}

// ClientOption configures a Client instance.
type ClientOption func(*Client)

// WithBinary overrides the Taskwarrior binary name or path.
func WithBinary(bin string) ClientOption {
	return func(c *Client) {
		c.binary = bin
	}
}

// WithTaskData sets the TASKDATA directory override.
func WithTaskData(dir string) ClientOption {
	return func(c *Client) {
		c.taskData = dir
	}
}

// WithTaskRC sets the TASKRC configuration file override.
func WithTaskRC(rcPath string) ClientOption {
	return func(c *Client) {
		c.taskRC = rcPath
	}
}

// WithEnviron sets an explicit environment for the CLI invocations.
func WithEnviron(env []string) ClientOption {
	return func(c *Client) {
		c.environ = env
	}
}

// NewClient returns a new Client with the default or configured options.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		binary: "task",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// buildEnv creates the process environment, applying TASKDATA and TASKRC overrides.
func (c *Client) buildEnv() []string {
	base := c.environ
	if base == nil {
		base = os.Environ()
	}

	env := make([]string, 0, len(base)+2)
	for _, kv := range base {
		if (c.taskData != "" && strings.HasPrefix(kv, "TASKDATA=")) ||
			(c.taskRC != "" && strings.HasPrefix(kv, "TASKRC=")) {
			continue
		}
		env = append(env, kv)
	}

	if c.taskData != "" {
		env = append(env, "TASKDATA="+c.taskData)
	}
	if c.taskRC != "" {
		env = append(env, "TASKRC="+c.taskRC)
	}

	return env
}

// EnsureUDA registers the "urgencyoffset" numeric UDA (see Task.UrgencyOffset) in
// Taskwarrior's config if it isn't already configured as such, so manual
// reordering (Ctrl+j/Ctrl+k in the Tasks panel) has somewhere to persist
// its per-task rank offset. Idempotent and safe to call on every startup:
// it first checks the current value via `task _get` and only writes when
// it differs, avoiding an unnecessary config file rewrite on every run.
func (c *Client) EnsureUDA(ctx context.Context) error {
	out, err := c.run(ctx, "_get", "rc.uda.urgencyoffset.type")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) == "numeric" {
		return nil
	}
	_, err = c.run(ctx, "rc.confirmation=off", "config", "uda.urgencyoffset.type", "numeric")
	return err
}

// GitSyncConfig holds Taskwarrior's git-sync backend settings (see `task`
// 3.5.0+, PR #4111). Any field left empty is skipped by ApplyGitSyncConfig
// rather than being written as an empty value, so a partially-filled config
// only touches the keys the user actually set.
//
// Deliberately excluded: the encryption secret. It's never persisted into
// Taskwarrior's own config via ApplyGitSyncConfig — see Sync, which
// instead accepts it as a one-off, per-invocation argument.
type GitSyncConfig struct {
	LocalPath string
	Branch    string
	Remote    string
}

// gitSyncConfigKeys pairs each GitSyncConfig field with its
// `task config sync.git.*` key, in application order.
func (g GitSyncConfig) gitSyncConfigKeys() []struct{ key, value string } {
	return []struct{ key, value string }{
		{"sync.git.local_path", g.LocalPath},
		{"sync.git.branch", g.Branch},
		{"sync.git.remote", g.Remote},
	}
}

// ApplyGitSyncConfig idempotently writes non-empty GitSyncConfig fields to
// Taskwarrior's config via `task config sync.git.*`, mirroring EnsureUDA's
// pattern: each key's current value is checked first via `task _get`, and
// only written when it differs, to avoid an unnecessary config file
// rewrite on every startup. Empty fields are left untouched entirely.
func (c *Client) ApplyGitSyncConfig(ctx context.Context, cfg GitSyncConfig) error {
	for _, kv := range cfg.gitSyncConfigKeys() {
		if kv.value == "" {
			continue
		}

		out, err := c.run(ctx, "_get", "rc."+kv.key)
		if err != nil {
			return err
		}
		if strings.TrimSpace(out) == kv.value {
			continue
		}

		if _, err := c.run(ctx, "rc.confirmation=off", "config", kv.key, kv.value); err != nil {
			return err
		}
	}
	return nil
}

// syncNotConfiguredMarker is the distinctive fragment of Taskwarrior's own
// "no sync backend configured" error (the single most common Sync failure
// by far, since it's what you get until sync.git.* is set up), used to
// substitute a clearer, actionable message in its place.
const syncNotConfiguredMarker = "No sync.* settings are configured"

// Sync runs `task sync`, triggering whatever sync backend Taskwarrior has
// configured (e.g. the git-sync backend set up via ApplyGitSyncConfig).
// This is a purely manual, user-initiated trigger — lazytask never calls
// this automatically on startup or on a schedule.
//
// secret, if non-empty, is passed as a one-off "rc.sync.encryption_secret="
// override for this invocation only — it is never written to
// Taskwarrior's own .taskrc (unlike the other sync.git.* keys set via
// ApplyGitSyncConfig). Pass "" if no secret is configured/needed.
//
// Unlike other Client methods, Sync doesn't reuse the shared run() helper:
// Taskwarrior's raw stderr on failure includes noisy debug echoes of the
// TASKRC/TASKDATA/rc.confirmation overrides lazytask itself just passed
// (e.g. "TASKRC override: ...", "Configuration override
// rc.confirmation=off"), which only restate flags/env vars this process
// set and are never useful to the end user — left in, they make an
// entirely expected, common state ("sync isn't configured yet") look like
// a multi-line crash dump. Sync strips those lines and gives the common
// "not configured" case its own plain-English message instead.
func (c *Client) Sync(ctx context.Context, secret string) error {
	c.preemptBackgroundSync()
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.doSync(ctx, secret)
}

// SyncBackground runs `task sync` the same way Sync does, but is intended
// for lazytask's own automatic triggers (the periodic timer and the
// on-mutation debounce) rather than the user-initiated "S" key. Unlike
// Sync, it is preemptible: every foreground Client call
// (run/Export/Import/Sync) calls preemptBackgroundSync before doing its
// own work, which cancels this call's context — exec.CommandContext kills
// the underlying `task sync` subprocess as soon as that happens, so a
// foreground call never waits for a slow or unreachable git remote's full
// round-trip, only however long the subprocess takes to die (typically
// milliseconds). Callers should treat the resulting context.Canceled
// error as "skipped this cycle" rather than a real sync failure worth
// surfacing to the user.
//
// See AwaitBackgroundSync for letting a call already in flight finish on
// its own instead of being preempted (used on quit).
func (c *Client) SyncBackground(ctx context.Context, secret string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c.bgSyncMu.Lock()
	c.bgSyncCancel = cancel
	c.bgSyncMu.Unlock()
	c.bgSyncWG.Add(1)
	defer func() {
		c.bgSyncMu.Lock()
		c.bgSyncCancel = nil
		c.bgSyncMu.Unlock()
		c.bgSyncWG.Done()
	}()

	c.mu.Lock()
	defer c.mu.Unlock()
	return c.doSync(ctx, secret)
}

// AwaitBackgroundSync blocks until a SyncBackground call currently in
// flight finishes on its own, or timeout elapses, whichever comes first.
// It returns immediately (a no-op) if no background sync is running.
// Intended for use only when the app is quitting (see main.go's quit
// handling), so a sync that's already under way gets a bounded chance to
// reach the remote instead of always being killed outright by exiting.
func (c *Client) AwaitBackgroundSync(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		c.bgSyncWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

// preemptBackgroundSync cancels a SyncBackground call currently in
// flight, if any, so the foreground call about to run doesn't have to
// wait behind it — see SyncBackground's doc comment. Safe to call even
// when no background sync is running.
func (c *Client) preemptBackgroundSync() {
	c.bgSyncMu.Lock()
	cancel := c.bgSyncCancel
	c.bgSyncMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// doSync runs `task sync` with ctx/secret and translates its result,
// assuming the caller already holds c.mu. Shared by Sync and
// SyncBackground.
func (c *Client) doSync(ctx context.Context, secret string) error {
	args := []string{"rc.confirmation=off"}
	if secret != "" {
		args = append(args, "rc.sync.encryption_secret="+secret)
	}
	args = append(args, "sync")

	cmd := exec.CommandContext(ctx, c.binary, args...)
	cmd.Env = c.buildEnv()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return nil
	}
	// A cancelled ctx (e.g. SyncBackground preempted by a foreground call,
	// see preemptBackgroundSync) kills the subprocess before it can
	// produce any meaningful stderr — surface ctx.Err() itself (typically
	// context.Canceled) rather than the generic message below, so callers
	// like runAutoSync can tell "preempted" apart from a real failure.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}

	msg := cleanSyncStderr(stderr.String())
	if strings.Contains(msg, syncNotConfiguredMarker) {
		return fmt.Errorf("no sync backend configured yet — set sync.git.* in lazytask's config.yaml (see `task-sync(5)`)")
	}
	if msg == "" {
		return fmt.Errorf("task sync failed")
	}
	return fmt.Errorf("task sync failed: %s", msg)
}

// cleanSyncStderr drops Taskwarrior's debug echoes of the overrides
// lazytask itself passed (see Sync's doc comment) and joins whatever
// remains into a single line, since a popup renders a wall of stray
// newlines poorly.
func cleanSyncStderr(raw string) string {
	lines := strings.Split(raw, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "TASKRC override:") ||
			strings.HasPrefix(trimmed, "TASKDATA override:") ||
			strings.HasPrefix(trimmed, "Configuration override") {
			continue
		}
		kept = append(kept, trimmed)
	}
	return strings.Join(kept, "; ")
}

// Export runs `task [filters...] export` and decodes the resulting JSON tasks.
func (c *Client) Export(ctx context.Context, filters ...string) ([]Task, error) {
	args := make([]string, 0, len(filters)+1)
	args = append(args, filters...)
	args = append(args, "export")

	cmd := exec.CommandContext(ctx, c.binary, args...)
	cmd.Env = c.buildEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	c.preemptBackgroundSync()
	c.mu.Lock()
	err := cmd.Run()
	c.mu.Unlock()
	if err != nil {
		stderrMsg := strings.TrimSpace(stderr.String())
		if stderrMsg != "" {
			return nil, fmt.Errorf("task export failed (%w): %s", err, stderrMsg)
		}
		return nil, fmt.Errorf("task export failed: %w", err)
	}

	tasks, err := ParseTasks(stdout.Bytes())
	if err != nil {
		return nil, fmt.Errorf("decoding export output: %w", err)
	}

	return tasks, nil
}
