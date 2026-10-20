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

func TestClient_StartStopTag_InvalidArgs(t *testing.T) {
	c := NewClient()
	ctx := context.Background()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{"start empty id", c.Start(ctx, " "), "must not be empty"},
		{"stop empty id", c.Stop(ctx, " "), "must not be empty"},
		{"add tag empty id", c.AddTag(ctx, " ", "next"), "must not be empty"},
		{"remove tag empty id", c.RemoveTag(ctx, " ", "next"), "must not be empty"},
		{"add tag empty tag", c.AddTag(ctx, "1", " "), "invalid tag"},
		{"add tag leading plus", c.AddTag(ctx, "1", "+next"), "invalid tag"},
		{"remove tag leading minus", c.RemoveTag(ctx, "1", "-next"), "invalid tag"},
		{"add tag with space", c.AddTag(ctx, "1", "a b"), "invalid tag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, tt.err)
			assert.Contains(t, tt.err.Error(), tt.want)
		})
	}
}

func TestClient_StartStopTag_Integration(t *testing.T) {
	if _, err := exec.LookPath("task"); err != nil {
		t.Skip("task CLI not found in PATH; skipping integration test")
	}

	tempDir := t.TempDir()
	taskRC := filepath.Join(tempDir, ".taskrc")
	require.NoError(t, os.WriteFile(taskRC, []byte("confirmation=off\n"), 0600))

	client := NewClient(
		WithTaskData(filepath.Join(tempDir, "data")),
		WithTaskRC(taskRC),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.Add(ctx, "Write report")
	require.NoError(t, err)
	tasks, err := client.Export(ctx)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	uuid := tasks[0].UUID
	assert.Empty(t, tasks[0].Start)

	// Start sets the start attribute; Stop clears it.
	require.NoError(t, client.Start(ctx, uuid))
	tasks, err = client.Export(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, tasks[0].Start)

	require.NoError(t, client.Stop(ctx, uuid))
	tasks, err = client.Export(ctx)
	require.NoError(t, err)
	assert.Empty(t, tasks[0].Start)

	// AddTag/RemoveTag toggle the "next" tag.
	require.NoError(t, client.AddTag(ctx, uuid, "next"))
	tasks, err = client.Export(ctx)
	require.NoError(t, err)
	assert.Contains(t, tasks[0].Tags, "next")

	require.NoError(t, client.RemoveTag(ctx, uuid, "next"))
	tasks, err = client.Export(ctx)
	require.NoError(t, err)
	assert.NotContains(t, tasks[0].Tags, "next")
}
