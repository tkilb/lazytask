package config

import (
	"os"
	"path/filepath"
	"strings"
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

// withTempAppConfigDir redirects appConfigPath() to a fresh per-test
// directory by pointing $HOME at it, so tests never touch a real config
// file. It returns the resulting ~/.config directory so tests can write
// there directly.
func withTempAppConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return filepath.Join(dir, ".config")
}

func writeAppConfig(t *testing.T, configDir, contents string) {
	t.Helper()
	dir := filepath.Join(configDir, appConfigDirName)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, appConfigFileName), []byte(contents), 0o644))
}

func TestLoadGitSyncConfig_Missing(t *testing.T) {
	withTempAppConfigDir(t)

	got, err := LoadGitSyncConfig()
	require.NoError(t, err)
	assert.Equal(t, GitSyncConfig{}, got)
}

func TestLoadGitSyncConfig_Populated(t *testing.T) {
	configDir := withTempAppConfigDir(t)
	writeAppConfig(t, configDir, `
sync:
  git:
    local_path: /tmp/tasks-sync
    branch: main
    remote: git@github.com:me/tasks-sync.git
    encryption_secret_file: ~/.secrets/lazytask-sync-secret
`)

	got, err := LoadGitSyncConfig()
	require.NoError(t, err)
	assert.Equal(t, GitSyncConfig{
		LocalPath:            "/tmp/tasks-sync",
		Branch:               "main",
		Remote:               "git@github.com:me/tasks-sync.git",
		EncryptionSecretFile: "~/.secrets/lazytask-sync-secret",
	}, got)
}

func TestLoadGitSyncConfig_LocalPathTildeExpansion(t *testing.T) {
	// Locks in that lazytask expands "~/" itself for sync.git.local_path
	// (relying on os.UserHomeDir(), which reads $HOME identically on
	// Linux and macOS) rather than handing the literal "~/..." string to
	// `task config`. Taskwarrior's own tilde expansion of this setting has
	// been observed to misbehave on macOS (producing a bogus /home/...
	// path and a cryptic "Operation not supported" sync failure), so this
	// must not regress on any platform.
	configDir := withTempAppConfigDir(t)
	writeAppConfig(t, configDir, `
sync:
  git:
    local_path: ~/tasks-sync
`)

	got, err := LoadGitSyncConfig()
	require.NoError(t, err)
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, "tasks-sync"), got.LocalPath)
}

func TestLoadGitSyncConfig_PartiallyPopulated(t *testing.T) {
	configDir := withTempAppConfigDir(t)
	writeAppConfig(t, configDir, `
sync:
  git:
    remote: git@github.com:me/tasks-sync.git
`)

	got, err := LoadGitSyncConfig()
	require.NoError(t, err)
	assert.Equal(t, GitSyncConfig{Remote: "git@github.com:me/tasks-sync.git"}, got)
}

func TestLoadGitSyncConfig_Malformed(t *testing.T) {
	configDir := withTempAppConfigDir(t)
	writeAppConfig(t, configDir, "sync: [this is not a mapping")

	_, err := LoadGitSyncConfig()
	assert.Error(t, err)
}

func TestEnsureSyncSecret_GeneratesAndPersists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	secret, err := EnsureSyncSecret("")
	require.NoError(t, err)
	assert.NotEmpty(t, secret)

	path := filepath.Join(home, defaultSyncSecretRelPath)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	// A second call returns the same, already-persisted secret rather
	// than generating a new one.
	again, err := EnsureSyncSecret("")
	require.NoError(t, err)
	assert.Equal(t, secret, again)
}

func TestEnsureSyncSecret_CustomPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom-secret")

	secret, err := EnsureSyncSecret(path)
	require.NoError(t, err)
	assert.NotEmpty(t, secret)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, secret, strings.TrimSpace(string(data)))
}

func TestEnsureSyncSecret_TildeExpansion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	secret, err := EnsureSyncSecret("~/custom/secret-file")
	require.NoError(t, err)
	assert.NotEmpty(t, secret)

	_, err = os.Stat(filepath.Join(home, "custom", "secret-file"))
	require.NoError(t, err)
}

func TestEnsureSyncSecret_ExistingFileIsReturnedVerbatim(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	require.NoError(t, os.WriteFile(path, []byte("preexisting-secret\n"), 0o600))

	secret, err := EnsureSyncSecret(path)
	require.NoError(t, err)
	assert.Equal(t, "preexisting-secret", secret)
}

func TestRotateSyncSecret_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	require.NoError(t, os.WriteFile(path, []byte("preexisting-secret\n"), 0o600))

	rotated, err := RotateSyncSecret(path)
	require.NoError(t, err)
	assert.NotEmpty(t, rotated)
	assert.NotEqual(t, "preexisting-secret", rotated)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, rotated, strings.TrimSpace(string(data)))
}

func TestRotateSyncSecret_CreatesWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "secret")

	rotated, err := RotateSyncSecret(path)
	require.NoError(t, err)
	assert.NotEmpty(t, rotated)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestSyncSecretPath_DefaultAndOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	defaultPath, err := SyncSecretPath("")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, defaultSyncSecretRelPath), defaultPath)

	overridePath, err := SyncSecretPath("~/custom/secret-file")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, "custom", "secret-file"), overridePath)
}
