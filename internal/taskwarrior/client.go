package taskwarrior

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// TaskReader provides read-only access to Taskwarrior tasks.
type TaskReader interface {
	Export(ctx context.Context, filters ...string) ([]Task, error)
}

// Client wraps the Taskwarrior CLI binary for executing operations.
type Client struct {
	binary   string
	taskData string
	taskRC   string
	environ  []string
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
	args := []string{"rc.confirmation=off"}
	if secret != "" {
		args = append(args, "rc.sync.encryption_secret="+secret)
	}
	args = append(args, "sync")

	cmd := exec.CommandContext(ctx, c.binary, args...)
	cmd.Env = c.buildEnv()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err == nil {
		return nil
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

	if err := cmd.Run(); err != nil {
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
