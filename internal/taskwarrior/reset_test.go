package taskwarrior

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_DataLocation(t *testing.T) {
	tempDir := t.TempDir()
	c := NewClient(WithTaskData(tempDir))
	loc, err := c.DataLocation(context.Background())
	require.NoError(t, err)
	assert.Equal(t, tempDir, loc)
}

func TestClient_ResetLocalState(t *testing.T) {
	dataDir := t.TempDir()
	syncDir := t.TempDir()

	dbFile := filepath.Join(dataDir, "taskchampion.sqlite3")
	shmFile := filepath.Join(dataDir, "taskchampion.sqlite3-shm")
	walFile := filepath.Join(dataDir, "taskchampion.sqlite3-wal")
	otherFile := filepath.Join(dataDir, "other.txt")

	require.NoError(t, os.WriteFile(dbFile, []byte("data"), 0644))
	require.NoError(t, os.WriteFile(shmFile, []byte("shm"), 0644))
	require.NoError(t, os.WriteFile(walFile, []byte("wal"), 0644))
	require.NoError(t, os.WriteFile(otherFile, []byte("preserve"), 0644))

	syncCloneFile := filepath.Join(syncDir, "clone.git")
	require.NoError(t, os.WriteFile(syncCloneFile, []byte("git clone"), 0644))

	c := NewClient(WithTaskData(dataDir))
	err := c.ResetLocalState(context.Background(), syncDir)
	require.NoError(t, err)

	// SQLite files should be removed
	_, err = os.Stat(dbFile)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(shmFile)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(walFile)
	assert.True(t, os.IsNotExist(err))

	// Unrelated files preserved
	_, err = os.Stat(otherFile)
	assert.NoError(t, err)

	// Sync directory removed
	_, err = os.Stat(syncDir)
	assert.True(t, os.IsNotExist(err))
}
