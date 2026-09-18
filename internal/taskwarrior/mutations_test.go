package taskwarrior

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatedTaskRE(t *testing.T) {
	tests := []struct {
		name   string
		output string
		wantID string
		wantOk bool
	}{
		{name: "simple", output: "Created task 1.\n", wantID: "1", wantOk: true},
		{name: "multiline", output: "Some notice.\nCreated task 42.\n", wantID: "42", wantOk: true},
		{name: "no match", output: "Task not found.\n", wantOk: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := createdTaskRE.FindStringSubmatch(tt.output)
			if !tt.wantOk {
				assert.Nil(t, match)
				return
			}
			require.NotNil(t, match)
			assert.Equal(t, tt.wantID, match[1])
		})
	}
}

func TestClient_Add_InvalidBinary(t *testing.T) {
	c := NewClient(WithBinary("non_existent_binary_12345"))
	ctx := context.Background()

	id, err := c.Add(ctx, "Buy milk")
	require.Error(t, err)
	assert.Zero(t, id)
}

func TestClient_Done_EmptyID(t *testing.T) {
	c := NewClient()
	err := c.Done(context.Background(), "  ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestClient_Delete_EmptyID(t *testing.T) {
	c := NewClient()
	err := c.Delete(context.Background(), "  ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestClient_Import_EmptyData(t *testing.T) {
	c := NewClient()
	err := c.Import(context.Background(), []byte("   "))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestClient_Import_InvalidBinary(t *testing.T) {
	c := NewClient(WithBinary("non_existent_binary_12345"))
	err := c.Import(context.Background(), []byte(`{"description":"x"}`))
	require.Error(t, err)
}

func TestClient_Mutations_Integration(t *testing.T) {
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

	// 1. Add a task and confirm the returned ID resolves to it.
	id, err := client.Add(ctx, "Buy groceries", "project:Home", "priority:H")
	require.NoError(t, err)
	assert.Equal(t, 1, id)

	tasks, err := client.Export(ctx)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, "Buy groceries", tasks[0].Description)
	assert.Equal(t, "pending", tasks[0].Status)

	// 2. Add a second task, then mark the first done.
	_, err = client.Add(ctx, "Write docs", "project:Work")
	require.NoError(t, err)

	err = client.Done(ctx, "1")
	require.NoError(t, err)

	pending, err := client.Export(ctx, "status:pending")
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, "Write docs", pending[0].Description)

	completed, err := client.Export(ctx, "status:completed")
	require.NoError(t, err)
	require.Len(t, completed, 1)
	assert.Equal(t, "Buy groceries", completed[0].Description)

	// 3. Delete the remaining pending task, referencing it by UUID since
	// Taskwarrior renumbers pending IDs after a completion.
	err = client.Delete(ctx, pending[0].UUID)
	require.NoError(t, err)

	remainingPending, err := client.Export(ctx, "status:pending")
	require.NoError(t, err)
	assert.Empty(t, remainingPending)

	deleted, err := client.Export(ctx, "status:deleted")
	require.NoError(t, err)
	require.Len(t, deleted, 1)
	assert.Equal(t, "Write docs", deleted[0].Description)

	// 4. Import edited fields back onto the completed task, simulating a
	// round trip through an external editor.
	completed[0].Description = "Buy groceries and milk"
	completed[0].Project = "Home"
	data, err := json.Marshal(completed[0])
	require.NoError(t, err)

	err = client.Import(ctx, data)
	require.NoError(t, err)

	reimported, err := client.Export(ctx, "status:completed")
	require.NoError(t, err)
	require.Len(t, reimported, 1)
	assert.Equal(t, "Buy groceries and milk", reimported[0].Description)
	assert.Equal(t, "Home", reimported[0].Project)
	assert.Equal(t, completed[0].UUID, reimported[0].UUID)
}
