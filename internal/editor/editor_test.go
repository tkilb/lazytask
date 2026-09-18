package editor

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFakeEditor writes an executable shell script to a temp dir that acts
// as a stand-in editor, and returns its path. Using a real (non-interactive)
// script avoids needing an actual interactive editor in tests.
func writeFakeEditor(t *testing.T, script string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake editor scripts require a POSIX shell")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "fake-editor.sh")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755))
	return path
}

func TestEdit_RoundTripsUnmodifiedContent(t *testing.T) {
	fake := writeFakeEditor(t, `exit 0
`)

	got, err := Edit("hello\nworld\n", WithEditor(fake))
	require.NoError(t, err)
	assert.Equal(t, "hello\nworld\n", got)
}

func TestEdit_ReturnsEditorModifications(t *testing.T) {
	fake := writeFakeEditor(t, `echo "edited" >> "$1"
`)

	got, err := Edit("original\n", WithEditor(fake))
	require.NoError(t, err)
	assert.Equal(t, "original\nedited\n", got)
}

func TestEdit_ReceivesTempFileAsSoleArg(t *testing.T) {
	fake := writeFakeEditor(t, `if [ "$#" -ne 1 ]; then
  echo "expected exactly 1 arg, got $#" >&2
  exit 1
fi
`)

	_, err := Edit("content\n", WithEditor(fake))
	require.NoError(t, err)
}

func TestEdit_EditorFailureReturnsError(t *testing.T) {
	fake := writeFakeEditor(t, `exit 1
`)

	_, err := Edit("content\n", WithEditor(fake))
	require.Error(t, err)
}

func TestEdit_EditorWithArguments(t *testing.T) {
	fake := writeFakeEditor(t, `echo "arg=$1" >> "$2"
`)

	got, err := Edit("start\n", WithEditor(fake+" myarg"))
	require.NoError(t, err)
	assert.Equal(t, "start\narg=myarg\n", got)
}

func TestEdit_UsesEnvironEditor(t *testing.T) {
	fake := writeFakeEditor(t, `echo "from-env" >> "$1"
`)

	got, err := Edit("base\n", WithEnviron([]string{"EDITOR=" + fake}))
	require.NoError(t, err)
	assert.Equal(t, "base\nfrom-env\n", got)
}

func TestEdit_TempFileRemovedAfterward(t *testing.T) {
	fake := writeFakeEditor(t, `exit 0
`)
	out, err := Edit("x\n", WithEditor(fake))
	require.NoError(t, err)
	assert.Equal(t, "x\n", out)

	entries, err := os.ReadDir(os.TempDir())
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), "lazytask-edit-", "temp edit file should have been removed")
	}
}

func TestPrepare_ReturnsRunnableCmdAndReadableSession(t *testing.T) {
	fake := writeFakeEditor(t, `echo "edited" >> "$1"
`)

	cmd, session, err := Prepare("original\n", WithEditor(fake))
	require.NoError(t, err)
	defer session.Close()

	require.NoError(t, cmd.Run())

	got, err := session.Read()
	require.NoError(t, err)
	assert.Equal(t, "original\nedited\n", got)
}

func TestSession_CloseRemovesTempFile(t *testing.T) {
	_, session, err := Prepare("content\n")
	require.NoError(t, err)

	require.NoError(t, session.Close())
	_, err = session.Read()
	assert.Error(t, err)

	// Closing an already-removed file must not error.
	assert.NoError(t, session.Close())
}

func TestResolveEditor_PrefersWithEditorOption(t *testing.T) {
	cmd, err := resolveEditor(config{editor: "myeditor --flag", env: []string{"EDITOR=other"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"myeditor", "--flag"}, cmd)
}

func TestResolveEditor_FallsBackToEnvironEditor(t *testing.T) {
	cmd, err := resolveEditor(config{env: []string{"EDITOR=nano"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"nano"}, cmd)
}

func TestResolveEditor_FallsBackToDefault(t *testing.T) {
	cmd, err := resolveEditor(config{env: []string{"PATH=/usr/bin"}})
	require.NoError(t, err)
	assert.Equal(t, []string{DefaultEditor}, cmd)
}

func TestResolveEditor_IgnoresEmptyEnvironEditor(t *testing.T) {
	cmd, err := resolveEditor(config{env: []string{"EDITOR="}})
	require.NoError(t, err)
	assert.Equal(t, []string{DefaultEditor}, cmd)
}

func TestEnvValue(t *testing.T) {
	env := []string{"FOO=bar", "EDITOR=vim", "EMPTY="}
	assert.Equal(t, "bar", envValue(env, "FOO"))
	assert.Equal(t, "vim", envValue(env, "EDITOR"))
	assert.Equal(t, "", envValue(env, "EMPTY"))
	assert.Equal(t, "", envValue(env, "MISSING"))
}
