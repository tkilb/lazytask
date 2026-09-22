package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withTempStateFile redirects os.TempDir()'s effective root for the
// duration of the test by pointing $TMPDIR at a fresh per-test directory,
// so tests never touch a real shared /tmp/lazytask/state.json.
func withTempStateFile(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
}

func TestSaveLoadRoundTrip(t *testing.T) {
	withTempStateFile(t)

	project := "work"
	require.NoError(t, Save(State{Project: &project}))

	got := Load()
	require.NotNil(t, got.Project)
	assert.Equal(t, "work", *got.Project)
}

func TestLoadMissingFile(t *testing.T) {
	withTempStateFile(t)

	got := Load()
	assert.Nil(t, got.Project)
}

func TestLoadCorruptFile(t *testing.T) {
	withTempStateFile(t)

	dir := filepath.Join(os.TempDir(), stateDirName)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, stateFileName), []byte("not json"), 0o644))

	got := Load()
	assert.Nil(t, got.Project)
}

func TestSaveLoadNoneProject(t *testing.T) {
	withTempStateFile(t)

	empty := ""
	require.NoError(t, Save(State{Project: &empty}))

	got := Load()
	require.NotNil(t, got.Project)
	assert.Equal(t, "", *got.Project)
}

func TestSaveLoadNoFilter(t *testing.T) {
	withTempStateFile(t)

	require.NoError(t, Save(State{}))

	got := Load()
	assert.Nil(t, got.Project)
}
