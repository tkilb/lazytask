package taskwarrior

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
