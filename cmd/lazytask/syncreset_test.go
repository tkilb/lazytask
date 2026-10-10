package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tkilb/lazytask/internal/taskwarrior"
)

func TestIsSyncResetArg(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"no args", nil, false},
		{"empty slice", []string{}, false},
		{"sync-reset", []string{"sync-reset"}, true},
		{"sync-reset flag", []string{"sync-reset", "-y"}, true},
		{"unrelated arg", []string{"version"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isSyncResetArg(tc.args); got != tc.want {
				t.Errorf("isSyncResetArg(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestRunSyncReset_AbortOnNo(t *testing.T) {
	in := strings.NewReader("n\n")
	var out bytes.Buffer
	client := taskwarrior.NewClient()

	err := runSyncReset(in, &out, client, nil)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Aborted.")
}

func TestRunSyncReset_UnknownFlag(t *testing.T) {
	in := strings.NewReader("")
	var out bytes.Buffer
	client := taskwarrior.NewClient()

	err := runSyncReset(in, &out, client, []string{"--invalid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown sync-reset flag")
}

func TestRunSyncReset_ResetsAndSyncs(t *testing.T) {
	home := withIsolatedHome(t)
	taskData := filepath.Join(home, ".task")
	require.NoError(t, os.MkdirAll(taskData, 0755))

	dbFile := filepath.Join(taskData, "taskchampion.sqlite3")
	require.NoError(t, os.WriteFile(dbFile, []byte("dummy data"), 0644))

	// Write mock task binary that exits 0 on sync
	fakeBin := filepath.Join(t.TempDir(), "task")
	script := "#!/bin/sh\nexit 0\n"
	require.NoError(t, os.WriteFile(fakeBin, []byte(script), 0755))

	client := taskwarrior.NewClient(
		taskwarrior.WithBinary(fakeBin),
		taskwarrior.WithTaskData(taskData),
	)

	var out bytes.Buffer
	err := runSyncReset(strings.NewReader(""), &out, client, []string{"-y"})
	require.NoError(t, err)

	// Local db file should be removed
	_, err = os.Stat(dbFile)
	assert.True(t, os.IsNotExist(err))

	assert.Contains(t, out.String(), "Resetting local Taskwarrior database and sync clone...")
	assert.Contains(t, out.String(), "Successfully reset local state and synced from remote.")
}
