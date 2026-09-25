package taskwarrior

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestClient_Restore_EmptyID(t *testing.T) {
	c := NewClient()
	err := c.Restore(context.Background(), "  ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestClient_Purge_EmptyID(t *testing.T) {
	c := NewClient()
	err := c.Purge(context.Background(), "  ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestClient_SetPriority_EmptyID(t *testing.T) {
	c := NewClient()
	err := c.SetPriority(context.Background(), "  ", "H")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestClient_SetUrgencyOffset_EmptyID(t *testing.T) {
	c := NewClient()
	err := c.SetUrgencyOffset(context.Background(), "  ", 1.5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestClient_SetDue_EmptyID(t *testing.T) {
	c := NewClient()
	err := c.SetDue(context.Background(), "  ", "20240115T140000Z")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestClient_SetProject_EmptyID(t *testing.T) {
	c := NewClient()
	err := c.SetProject(context.Background(), "  ", "home")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestClient_Annotate_EmptyID(t *testing.T) {
	c := NewClient()
	err := c.Annotate(context.Background(), "  ", "a note")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestClient_Annotate_EmptyText(t *testing.T) {
	c := NewClient()
	err := c.Annotate(context.Background(), "1", "")
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

	// 3b. Restore the deleted task back to pending.
	err = client.Restore(ctx, deleted[0].UUID)
	require.NoError(t, err)

	restoredPending, err := client.Export(ctx, "status:pending")
	require.NoError(t, err)
	require.Len(t, restoredPending, 1)
	assert.Equal(t, "Write docs", restoredPending[0].Description)

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

	// 5. Delete the restored task again, then purge it permanently and
	// confirm it no longer shows up even in the deleted export.
	err = client.Delete(ctx, restoredPending[0].UUID)
	require.NoError(t, err)

	rePurgedDeleted, err := client.Export(ctx, "status:deleted")
	require.NoError(t, err)
	require.Len(t, rePurgedDeleted, 1)

	err = client.Purge(ctx, rePurgedDeleted[0].UUID)
	require.NoError(t, err)

	afterPurge, err := client.Export(ctx, "status:deleted")
	require.NoError(t, err)
	assert.Empty(t, afterPurge)

	// 6. Add one more task and confirm SetPriority updates its priority
	// field.
	prioID, err := client.Add(ctx, "Renew passport")
	require.NoError(t, err)

	err = client.SetPriority(ctx, fmt.Sprintf("%d", prioID), "H")
	require.NoError(t, err)

	prioritized, err := client.Export(ctx, "status:pending")
	require.NoError(t, err)
	require.Len(t, prioritized, 1)
	assert.Equal(t, "H", prioritized[0].Priority)

	// 7. SetDue updates the due field, in Taskwarrior's combined UTC
	// export format.
	err = client.SetDue(ctx, fmt.Sprintf("%d", prioID), "20240115T140000Z")
	require.NoError(t, err)

	dued, err := client.Export(ctx, "status:pending")
	require.NoError(t, err)
	require.Len(t, dued, 1)
	assert.Equal(t, "20240115T140000Z", dued[0].Due)

	// 8. SetProject updates the project field, and clears it back to
	// empty when passed "".
	err = client.SetProject(ctx, fmt.Sprintf("%d", prioID), "home")
	require.NoError(t, err)

	projected, err := client.Export(ctx, "status:pending")
	require.NoError(t, err)
	require.Len(t, projected, 1)
	assert.Equal(t, "home", projected[0].Project)

	err = client.SetProject(ctx, fmt.Sprintf("%d", prioID), "")
	require.NoError(t, err)

	unprojected, err := client.Export(ctx, "status:pending")
	require.NoError(t, err)
	require.Len(t, unprojected, 1)
	assert.Empty(t, unprojected[0].Project)

	// 9. Annotate adds a new, timestamped annotation to the task; calling
	// it again appends a second one rather than replacing the first.
	err = client.Annotate(ctx, fmt.Sprintf("%d", prioID), "first note")
	require.NoError(t, err)
	err = client.Annotate(ctx, fmt.Sprintf("%d", prioID), "second note")
	require.NoError(t, err)

	annotated, err := client.Export(ctx, "status:pending")
	require.NoError(t, err)
	require.Len(t, annotated, 1)
	require.Len(t, annotated[0].Annotations, 2)
	assert.Equal(t, "first note", annotated[0].Annotations[0].Description)
	assert.Equal(t, "second note", annotated[0].Annotations[1].Description)
}
