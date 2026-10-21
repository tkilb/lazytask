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

func TestClient_ModifyTags_InvalidArgs(t *testing.T) {
	c := NewClient()
	ctx := context.Background()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{"empty id", c.ModifyTags(ctx, " ", []string{"a"}, nil), "must not be empty"},
		{"nothing to do", c.ModifyTags(ctx, "1", nil, nil), "no tag changes"},
		{"empty add tag", c.ModifyTags(ctx, "1", []string{" "}, nil), "invalid tag"},
		{"add with space", c.ModifyTags(ctx, "1", []string{"a b"}, nil), "invalid tag"},
		{"add leading plus", c.ModifyTags(ctx, "1", []string{"+a"}, nil), "invalid tag"},
		{"remove leading minus", c.ModifyTags(ctx, "1", nil, []string{"-a"}), "invalid tag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, tt.err)
			assert.Contains(t, tt.err.Error(), tt.want)
		})
	}
}

func TestClient_ModifyTags_Integration(t *testing.T) {
	if _, err := exec.LookPath("task"); err != nil {
		t.Skip("task CLI not found in PATH; skipping integration test")
	}

	tempDir := t.TempDir()
	taskRC := filepath.Join(tempDir, ".taskrc")
	require.NoError(t, os.WriteFile(taskRC, []byte("confirmation=off\n"), 0600))
	client := NewClient(WithTaskData(filepath.Join(tempDir, "data")), WithTaskRC(taskRC))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.Add(ctx, "Write report", "+old", "+keep")
	require.NoError(t, err)
	tasks, err := client.Export(ctx)
	require.NoError(t, err)
	uuid := tasks[0].UUID

	// Adds and removes apply together.
	require.NoError(t, client.ModifyTags(ctx, uuid, []string{"new1", "new2"}, []string{"old"}))
	tasks, err = client.Export(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"keep", "new1", "new2"}, tasks[0].Tags)

	// Removing everything leaves the task untagged.
	require.NoError(t, client.ModifyTags(ctx, uuid, nil, []string{"keep", "new1", "new2"}))
	tasks, err = client.Export(ctx)
	require.NoError(t, err)
	assert.Empty(t, tasks[0].Tags)
}
