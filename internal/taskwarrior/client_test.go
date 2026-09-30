package taskwarrior

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Options(t *testing.T) {
	c := NewClient(
		WithBinary("/usr/bin/custom-task"),
		WithTaskData("/tmp/data"),
		WithTaskRC("/tmp/.taskrc"),
		WithEnviron([]string{"FOO=bar"}),
	)

	assert.Equal(t, "/usr/bin/custom-task", c.binary)
	assert.Equal(t, "/tmp/data", c.taskData)
	assert.Equal(t, "/tmp/.taskrc", c.taskRC)
	assert.Equal(t, []string{"FOO=bar"}, c.environ)

	env := c.buildEnv()
	assert.Contains(t, env, "FOO=bar")
	assert.Contains(t, env, "TASKDATA=/tmp/data")
	assert.Contains(t, env, "TASKRC=/tmp/.taskrc")
}

func TestClient_Export_InvalidBinary(t *testing.T) {
	c := NewClient(WithBinary("non_existent_binary_12345"))
	ctx := context.Background()

	tasks, err := c.Export(ctx)
	require.Error(t, err)
	assert.Nil(t, tasks)
	assert.Contains(t, err.Error(), "task export failed")
}

func TestClient_Export_ContextCanceled(t *testing.T) {
	c := NewClient()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	tasks, err := c.Export(ctx)
	require.Error(t, err)
	assert.Nil(t, tasks)
}

func TestClient_Export_Integration(t *testing.T) {
	if _, err := exec.LookPath("task"); err != nil {
		t.Skip("task CLI not found in PATH; skipping integration test")
	}

	tempDir := t.TempDir()
	taskRC := filepath.Join(tempDir, ".taskrc")
	taskData := filepath.Join(tempDir, "data")

	// Create an empty .taskrc with confirmation disabled to prevent any interactive prompts.
	err := os.WriteFile(taskRC, []byte("confirmation=off\n"), 0600)
	require.NoError(t, err)

	env := append(os.Environ(), "TASKDATA="+taskData, "TASKRC="+taskRC)

	// Helper to run seed commands in the isolated test environment.
	runTaskCmd := func(args ...string) {
		t.Helper()
		cmd := exec.Command("task", args...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "task %v failed: %s", args, string(out))
	}

	// 1. Initially empty export
	client := NewClient(
		WithTaskData(taskData),
		WithTaskRC(taskRC),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tasks, err := client.Export(ctx)
	require.NoError(t, err)
	assert.Empty(t, tasks)

	// 2. Add two tasks: one pending, one to be completed
	runTaskCmd("rc.confirmation=off", "add", "Buy groceries", "project:Home", "priority:H")
	runTaskCmd("rc.confirmation=off", "add", "Write docs", "project:Work", "priority:M")
	runTaskCmd("rc.confirmation=off", "2", "done")

	// 3. Export all tasks
	allTasks, err := client.Export(ctx)
	require.NoError(t, err)
	require.Len(t, allTasks, 2)

	// 4. Export with filter (status:pending)
	pendingTasks, err := client.Export(ctx, "status:pending")
	require.NoError(t, err)
	require.Len(t, pendingTasks, 1)
	assert.Equal(t, "Buy groceries", pendingTasks[0].Description)
	assert.Equal(t, "Home", pendingTasks[0].Project)
	assert.Equal(t, "H", pendingTasks[0].Priority)
	assert.Equal(t, "pending", pendingTasks[0].Status)

	// 5. Export with non-matching filter
	emptyFilterTasks, err := client.Export(ctx, "project:NonExistent")
	require.NoError(t, err)
	assert.Empty(t, emptyFilterTasks)
}

func TestClient_EnsureUDA_Integration(t *testing.T) {
	if _, err := exec.LookPath("task"); err != nil {
		t.Skip("task CLI not found in PATH; skipping integration test")
	}

	tempDir := t.TempDir()
	taskRC := filepath.Join(tempDir, ".taskrc")
	taskData := filepath.Join(tempDir, "data")

	err := os.WriteFile(taskRC, []byte("confirmation=off\n"), 0600)
	require.NoError(t, err)

	client := NewClient(
		WithTaskData(taskData),
		WithTaskRC(taskRC),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First call registers the UDA from scratch.
	require.NoError(t, client.EnsureUDA(ctx))

	id, err := client.Add(ctx, "Buy groceries")
	require.NoError(t, err)
	require.NoError(t, client.SetUrgencyOffset(ctx, "1", 2.5))

	tasks, err := client.Export(ctx)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, id, tasks[0].ID)
	assert.Equal(t, 2.5, tasks[0].UrgencyOffset)

	// Second call is a no-op since the UDA is already configured correctly.
	require.NoError(t, client.EnsureUDA(ctx))
}

func TestClient_ApplyGitSyncConfig_Integration(t *testing.T) {
	if _, err := exec.LookPath("task"); err != nil {
		t.Skip("task CLI not found in PATH; skipping integration test")
	}

	tempDir := t.TempDir()
	taskRC := filepath.Join(tempDir, ".taskrc")
	taskData := filepath.Join(tempDir, "data")

	err := os.WriteFile(taskRC, []byte("confirmation=off\n"), 0600)
	require.NoError(t, err)

	client := NewClient(
		WithTaskData(taskData),
		WithTaskRC(taskRC),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Only two of the four keys are set; the others must be left untouched.
	cfg := GitSyncConfig{
		LocalPath: filepath.Join(tempDir, "sync-repo"),
		Remote:    "git@github.com:example/tasks-sync.git",
	}
	require.NoError(t, client.ApplyGitSyncConfig(ctx, cfg))

	got, err := client.run(ctx, "_get", "rc.sync.git.local_path")
	require.NoError(t, err)
	assert.Equal(t, cfg.LocalPath, strings.TrimSpace(got))

	got, err = client.run(ctx, "_get", "rc.sync.git.remote")
	require.NoError(t, err)
	assert.Equal(t, cfg.Remote, strings.TrimSpace(got))

	got, err = client.run(ctx, "_get", "rc.sync.git.branch")
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(got))

	// Second call is a no-op since both configured keys already match.
	require.NoError(t, client.ApplyGitSyncConfig(ctx, cfg))
}

// TestClient_Sync_Integration verifies Sync shells out to `task sync` and
// surfaces a clear, actionable error when no sync backend is configured
// (the common case, since this test never calls ApplyGitSyncConfig) —
// notably, without Taskwarrior's own noisy TASKRC/TASKDATA/confirmation
// override echoes (see Sync's doc comment). It doesn't assert success,
// since that would require a real, reachable git remote.
func TestClient_Sync_Integration(t *testing.T) {
	if _, err := exec.LookPath("task"); err != nil {
		t.Skip("task CLI not found in PATH; skipping integration test")
	}

	tempDir := t.TempDir()
	taskRC := filepath.Join(tempDir, ".taskrc")
	taskData := filepath.Join(tempDir, "data")

	err := os.WriteFile(taskRC, []byte("confirmation=off\n"), 0600)
	require.NoError(t, err)

	client := NewClient(
		WithTaskData(taskData),
		WithTaskRC(taskRC),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Sync(ctx, "")
	require.Error(t, err, "task sync should fail with no sync backend configured")
	assert.Equal(t, "no sync backend configured yet — set sync.git.* in lazytask's config.yaml (see `task-sync(5)`)", err.Error())
}

// TestClient_Sync_SecretNeverPersistedToTaskRC verifies the secret passed
// to Sync is applied only as a one-off rc. override for that invocation —
// never written into Taskwarrior's own config (unlike LocalPath/Branch/
// Remote via ApplyGitSyncConfig).
func TestClient_Sync_SecretNeverPersistedToTaskRC(t *testing.T) {
	if _, err := exec.LookPath("task"); err != nil {
		t.Skip("task CLI not found in PATH; skipping integration test")
	}

	tempDir := t.TempDir()
	taskRC := filepath.Join(tempDir, ".taskrc")
	taskData := filepath.Join(tempDir, "data")

	err := os.WriteFile(taskRC, []byte("confirmation=off\n"), 0600)
	require.NoError(t, err)

	client := NewClient(
		WithTaskData(taskData),
		WithTaskRC(taskRC),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Sync itself still fails here (no sync.git.* backend configured),
	// but that's fine: the point is checking .taskrc afterward.
	_ = client.Sync(ctx, "some-secret-value")

	got, err := client.run(ctx, "_get", "rc.sync.git.encryption_secret")
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(got), "secret must never be persisted via `task config`")
}

// TestClient_ConcurrentInvocations_NoDatabaseLocked is a regression test
// for a "database is locked: Error code 5: database is locked" failure
// users hit intermittently: lazytask fires several independent tea.Cmds
// (e.g. a mutation alongside the tasks/projects/tags refresh it triggers,
// an auto-sync tick landing mid-mutation, or two mutations racing) against
// the same shared *Client, and Taskwarrior's SQLite-backed replica storage
// does not tolerate concurrent processes touching one data directory.
// Client.mu (see its doc comment) serializes every subprocess this Client
// spawns.
//
// Confirmed directly while building this fix: with c.mu's Lock/Unlock
// temporarily stripped, this exact scenario (concurrent Add/Sync against a
// git-sync-configured replica) reliably produced 30+ failures per run,
// including literal "database is locked: Error code 5: database is
// locked" errors; with the mutex restored, 0 failures across repeated
// runs. git-sync is included deliberately (not just Export/Add) since
// that's what made the failure reproduce reliably locally — reads/writes
// alone rarely triggered it within a short-running test.
func TestClient_ConcurrentInvocations_NoDatabaseLocked(t *testing.T) {
	if _, err := exec.LookPath("task"); err != nil {
		t.Skip("task CLI not found in PATH; skipping integration test")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH; skipping integration test")
	}

	tempDir := t.TempDir()
	taskRC := filepath.Join(tempDir, ".taskrc")
	taskData := filepath.Join(tempDir, "data")
	require.NoError(t, os.WriteFile(taskRC, []byte("confirmation=off\n"), 0600))

	remoteDir := filepath.Join(tempDir, "remote.git")
	require.NoError(t, exec.Command("git", "init", "--bare", remoteDir).Run())

	client := NewClient(WithTaskData(taskData), WithTaskRC(taskRC))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	require.NoError(t, client.ApplyGitSyncConfig(ctx, GitSyncConfig{
		LocalPath: filepath.Join(tempDir, "gitclone"),
		Branch:    "main",
		Remote:    remoteDir,
	}))
	const secret = "test-only-encryption-secret-value"

	const workers = 10
	const itersPerWorker = 3
	var wg sync.WaitGroup
	errs := make(chan error, workers*itersPerWorker*2)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < itersPerWorker; j++ {
				if _, err := client.Add(ctx, fmt.Sprintf("concurrent task %d-%d", i, j)); err != nil {
					errs <- err
				}
				if err := client.Sync(ctx, secret); err != nil {
					errs <- err
				}
			}
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		assert.NoError(t, err)
	}
}

func TestCleanSyncStderr(t *testing.T) {
	raw := "TASKRC override: /tmp/.taskrc\nTASKDATA override: /tmp/data\nConfiguration override rc.confirmation=off\nSomething actually useful.\n"
	assert.Equal(t, "Something actually useful.", cleanSyncStderr(raw))
}

// fakeTaskBinary writes a tiny shell script standing in for the real
// `task` CLI: any invocation whose args include "sync" sleeps for delay
// before exiting 0 (simulating a slow/unreachable git remote); every
// other invocation just prints "[]" (a valid, empty `task export` result)
// and exits immediately. Returns the script's path.
func fakeTaskBinary(t *testing.T, delay time.Duration) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "task")
	script := fmt.Sprintf("#!/bin/sh\nfor arg in \"$@\"; do\n  if [ \"$arg\" = \"sync\" ]; then\n    sleep %f\n    exit 0\n  fi\ndone\necho '[]'\nexit 0\n", delay.Seconds())
	require.NoError(t, os.WriteFile(path, []byte(script), 0755))
	return path
}

// TestClient_SyncBackground_PreemptedByForegroundCall verifies the
// mechanism behind Hypothesis 1's fix: a foreground call (Export) issued
// while a SyncBackground is in flight against a slow "remote" doesn't
// wait for that sync's full duration — it preempts (cancels) the
// background sync and returns almost immediately instead.
func TestClient_SyncBackground_PreemptedByForegroundCall(t *testing.T) {
	const slowSyncDelay = 5 * time.Second
	client := NewClient(WithBinary(fakeTaskBinary(t, slowSyncDelay)))
	ctx := context.Background()

	bgDone := make(chan error, 1)
	go func() {
		bgDone <- client.SyncBackground(ctx, "")
	}()

	// Give SyncBackground a moment to acquire c.mu and start "sleeping".
	time.Sleep(200 * time.Millisecond)

	start := time.Now()
	_, err := client.Export(ctx)
	elapsed := time.Since(start)
	require.NoError(t, err)
	assert.Less(t, elapsed, 2*time.Second,
		"Export should be preempt the in-flight background sync rather than wait out its full %s delay", slowSyncDelay)

	select {
	case bgErr := <-bgDone:
		assert.ErrorIs(t, bgErr, context.Canceled, "preempted SyncBackground should report context.Canceled")
	case <-time.After(2 * time.Second):
		t.Fatal("SyncBackground did not return after being preempted")
	}
}

// TestClient_AwaitBackgroundSync_WaitsForInFlightSync verifies
// AwaitBackgroundSync lets an in-flight SyncBackground finish on its own
// (used on quit) rather than returning immediately, up to its timeout.
func TestClient_AwaitBackgroundSync_WaitsForInFlightSync(t *testing.T) {
	const syncDelay = 300 * time.Millisecond
	client := NewClient(WithBinary(fakeTaskBinary(t, syncDelay)))
	ctx := context.Background()

	bgDone := make(chan error, 1)
	go func() {
		bgDone <- client.SyncBackground(ctx, "")
	}()
	time.Sleep(50 * time.Millisecond)

	start := time.Now()
	client.AwaitBackgroundSync(5 * time.Second)
	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, elapsed, syncDelay/2, "AwaitBackgroundSync returned suspiciously fast for a sync still in flight")
	require.NoError(t, <-bgDone)
}

// TestClient_AwaitBackgroundSync_NoOpWhenIdle verifies AwaitBackgroundSync
// returns immediately when no background sync is running.
func TestClient_AwaitBackgroundSync_NoOpWhenIdle(t *testing.T) {
	client := NewClient()
	start := time.Now()
	client.AwaitBackgroundSync(5 * time.Second)
	assert.Less(t, time.Since(start), 500*time.Millisecond)
}
