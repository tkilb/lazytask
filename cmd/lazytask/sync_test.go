package main

import (
	"errors"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/popup"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
)

// TestModelUpdate_SyncKeySuccess verifies "S" runs a sync via the syncer
// (passing through the resolved encryption secret), shows an info popup,
// and refreshes tasks/projects/tags afterward.
func TestModelUpdate_SyncKeySuccess(t *testing.T) {
	syncer := &stubSyncer{}
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}}
	secretFile := filepath.Join(t.TempDir(), "sync-secret")
	m := model{reader: reader, syncer: syncer, syncSecretFile: secretFile, list: tasklist.New(reader.tasks)}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	m = newModel.(model)
	require.NotNil(t, cmd)

	msgs := runBatch(cmd)
	assert.Equal(t, 1, syncer.call)
	require.Len(t, syncer.secrets, 1)
	assert.NotEmpty(t, syncer.secrets[0], "a secret should have been generated and passed through")
	require.NotEmpty(t, msgs)
	syncedMsg, ok := msgs[0].(taskSyncedMsg)
	require.True(t, ok, "expected first msg to be taskSyncedMsg, got %T", msgs[0])
	_ = syncedMsg

	newModel, refreshCmd := m.Update(msgs[0])
	m = newModel.(model)
	require.NotNil(t, refreshCmd)
	msg, ok := m.popups.Current()
	require.True(t, ok)
	assert.Equal(t, popup.Info, msg.Severity)
	assert.Equal(t, "Sync complete", msg.Text)
}

// TestModelUpdate_SyncKeyError verifies "S" shows an error popup when the
// underlying `task sync` call fails (e.g. no sync backend configured).
func TestModelUpdate_SyncKeyError(t *testing.T) {
	syncer := &stubSyncer{err: errors.New("no sync backend configured")}
	secretFile := filepath.Join(t.TempDir(), "sync-secret")
	m := model{syncer: syncer, syncSecretFile: secretFile, list: tasklist.New(nil)}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	m = newModel.(model)
	require.NotNil(t, cmd)

	errMsg := cmd()
	newModel, _ = m.Update(errMsg)
	m = newModel.(model)
	msg, ok := m.popups.Current()
	require.True(t, ok)
	assert.Equal(t, popup.Error, msg.Severity)
	assert.Equal(t, "no sync backend configured", msg.Text)
}

// TestModelUpdate_SyncKeyNoSyncer verifies "S" is a safe no-op when no
// syncer is wired (defensive: initialModel always sets one in practice).
func TestModelUpdate_SyncKeyNoSyncer(t *testing.T) {
	m := model{list: tasklist.New(nil)}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	_ = newModel.(model)
	assert.Nil(t, cmd)
}
