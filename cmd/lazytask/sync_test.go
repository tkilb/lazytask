package main

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

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

// TestModelInit_SchedulesAutoSyncTick verifies Init() includes a periodic
// auto-sync tick command whenever a syncer and a positive interval are
// configured, alongside the usual startup fetches.
func TestModelInit_SchedulesAutoSyncTick(t *testing.T) {
	reader := &stubReader{}
	syncer := &stubSyncer{}
	m := model{reader: reader, syncer: syncer, autoSyncInterval: time.Millisecond, list: tasklist.New(nil)}

	msgs := runBatch(m.Init())
	var sawTick bool
	for _, msg := range msgs {
		if _, ok := msg.(autoSyncTickMsg); ok {
			sawTick = true
		}
	}
	assert.True(t, sawTick, "expected Init() to schedule an autoSyncTickMsg")
}

// TestModelInit_NoAutoSyncTickWithoutSyncer verifies Init() doesn't
// schedule the periodic timer at all when no syncer is wired (defensive,
// mirrors the "S" key's own nil-syncer guard).
func TestModelInit_NoAutoSyncTickWithoutSyncer(t *testing.T) {
	reader := &stubReader{}
	m := model{reader: reader, autoSyncInterval: time.Millisecond, list: tasklist.New(nil)}

	msgs := runBatch(m.Init())
	for _, msg := range msgs {
		_, ok := msg.(autoSyncTickMsg)
		assert.False(t, ok, "did not expect autoSyncTickMsg without a syncer")
	}
}

// TestModelUpdate_AutoSyncTick_Success verifies an autoSyncTickMsg runs a
// quiet background sync (no popup on success), clears any prior recorded
// failure, refreshes tasks/projects/tags, and reschedules the next tick.
func TestModelUpdate_AutoSyncTick_Success(t *testing.T) {
	syncer := &stubSyncer{}
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}}
	secretFile := filepath.Join(t.TempDir(), "sync-secret")
	m := model{
		reader:           reader,
		syncer:           syncer,
		syncSecretFile:   secretFile,
		autoSyncInterval: time.Millisecond,
		lastAutoSyncErr:  errors.New("stale prior failure"),
		list:             tasklist.New(reader.tasks),
	}

	newModel, cmd := m.Update(autoSyncTickMsg{})
	m = newModel.(model)
	require.NotNil(t, cmd)

	var sawNextTick, sawAutoSynced bool
	for _, msg := range runBatch(cmd) {
		switch msg.(type) {
		case autoSyncTickMsg:
			sawNextTick = true
		case autoSyncedMsg:
			sawAutoSynced = true
		}
	}
	assert.True(t, sawNextTick, "expected the next tick to be rescheduled")
	assert.True(t, sawAutoSynced, "expected runAutoSync to fire and report success")
	assert.Equal(t, 1, syncer.call)

	newModel, refreshCmd := m.Update(autoSyncedMsg{})
	m = newModel.(model)
	require.NotNil(t, refreshCmd)
	assert.Nil(t, m.lastAutoSyncErr, "success should clear any previously recorded failure")
	_, hasPopup := m.popups.Current()
	assert.False(t, hasPopup, "success must stay quiet: no popup")
}

// TestModelUpdate_AutoSyncTick_Error verifies a failed background sync
// records the error for the quiet status-line indicator instead of
// pushing an interrupting popup.
func TestModelUpdate_AutoSyncTick_Error(t *testing.T) {
	syncer := &stubSyncer{err: errors.New("no sync backend configured")}
	secretFile := filepath.Join(t.TempDir(), "sync-secret")
	m := model{syncer: syncer, syncSecretFile: secretFile, autoSyncInterval: time.Millisecond, list: tasklist.New(nil)}

	newModel, cmd := m.Update(autoSyncTickMsg{})
	m = newModel.(model)
	require.NotNil(t, cmd)

	var errMsg tea.Msg
	for _, msg := range runBatch(cmd) {
		if _, ok := msg.(autoSyncErrMsg); ok {
			errMsg = msg
		}
	}
	require.NotNil(t, errMsg, "expected an autoSyncErrMsg among the batched results")

	newModel, followUp := m.Update(errMsg)
	m = newModel.(model)
	assert.Nil(t, followUp)
	require.Error(t, m.lastAutoSyncErr)
	assert.Equal(t, "no sync backend configured", m.lastAutoSyncErr.Error())
	_, hasPopup := m.popups.Current()
	assert.False(t, hasPopup, "failure must stay quiet: no popup, status-line only")
}

// TestModelView_ShowsQuietAutoSyncFailureIndicator verifies a recorded
// background auto-sync failure surfaces in the bottom status line (not a
// popup), and that a healthy state shows no such indicator.
func TestModelView_ShowsQuietAutoSyncFailureIndicator(t *testing.T) {
	m := model{list: tasklist.New(nil), lastAutoSyncErr: errors.New("boom")}
	assert.Contains(t, m.View(), "auto-sync failed: boom")

	m.lastAutoSyncErr = nil
	assert.NotContains(t, m.View(), "auto-sync failed")
}

// TestScheduleMutationAutoSync_NoSyncer verifies scheduleMutationAutoSync
// is a safe no-op (nil cmd, unchanged generation) when no syncer is
// wired, mirroring the "S" key's own nil-syncer guard.
func TestScheduleMutationAutoSync_NoSyncer(t *testing.T) {
	m := &model{}
	cmd := m.scheduleMutationAutoSync()
	assert.Nil(t, cmd)
	assert.Equal(t, 0, m.mutationSyncGen)
}

// TestScheduleMutationAutoSync_WithSyncer verifies scheduleMutationAutoSync
// bumps the mutation-sync generation and returns a non-nil debounce cmd
// each time it's called, so a burst of mutations keeps advancing the
// generation (superseding any earlier, still-pending debounce timer).
func TestScheduleMutationAutoSync_WithSyncer(t *testing.T) {
	m := &model{syncer: &stubSyncer{}}

	cmd1 := m.scheduleMutationAutoSync()
	require.NotNil(t, cmd1)
	assert.Equal(t, 1, m.mutationSyncGen)

	cmd2 := m.scheduleMutationAutoSync()
	require.NotNil(t, cmd2)
	assert.Equal(t, 2, m.mutationSyncGen)
}

// TestModelUpdate_MutationSyncDebounce_CurrentGenTriggersSync verifies a
// mutationSyncDebounceMsg whose generation still matches the model's
// current mutationSyncGen (i.e. no newer mutation has superseded it)
// fires a quiet background sync via the same runAutoSync path as the
// periodic timer.
func TestModelUpdate_MutationSyncDebounce_CurrentGenTriggersSync(t *testing.T) {
	syncer := &stubSyncer{}
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}}
	secretFile := filepath.Join(t.TempDir(), "sync-secret")
	m := model{
		reader:          reader,
		syncer:          syncer,
		syncSecretFile:  secretFile,
		mutationSyncGen: 1,
		list:            tasklist.New(reader.tasks),
	}

	newModel, cmd := m.Update(mutationSyncDebounceMsg{gen: 1})
	m = newModel.(model)
	require.NotNil(t, cmd)

	var sawAutoSynced bool
	for _, msg := range runBatch(cmd) {
		if _, ok := msg.(autoSyncedMsg); ok {
			sawAutoSynced = true
		}
	}
	assert.True(t, sawAutoSynced, "expected runAutoSync to fire and report success")
	assert.Equal(t, 1, syncer.call)
}

// TestModelUpdate_MutationSyncDebounce_StaleGenIsNoop verifies a
// mutationSyncDebounceMsg carrying a generation older than the model's
// current mutationSyncGen (i.e. superseded by a later mutation in the
// same debounce burst) is a no-op — only the most recent mutation's timer
// should actually trigger a sync.
func TestModelUpdate_MutationSyncDebounce_StaleGenIsNoop(t *testing.T) {
	syncer := &stubSyncer{}
	m := model{syncer: syncer, mutationSyncGen: 2, list: tasklist.New(nil)}

	newModel, cmd := m.Update(mutationSyncDebounceMsg{gen: 1})
	_ = newModel.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, 0, syncer.call, "a stale debounce fire must not trigger a sync")
}

// TestModelUpdate_MutationSyncDebounce_NoSyncerIsNoop verifies a
// mutationSyncDebounceMsg is a safe no-op when no syncer is wired
// (defensive, mirrors the periodic timer's own nil-syncer guard).
func TestModelUpdate_MutationSyncDebounce_NoSyncerIsNoop(t *testing.T) {
	m := model{mutationSyncGen: 1, list: tasklist.New(nil)}

	newModel, cmd := m.Update(mutationSyncDebounceMsg{gen: 1})
	_ = newModel.(model)
	assert.Nil(t, cmd)
}

// TestModelUpdate_TaskDoneMsg_SchedulesMutationAutoSync verifies a
// mutation-success case (taskDoneMsg, standing in for the ~10 mutation
// message types that should trigger on-mutation auto-sync) bumps the
// mutation-sync generation as part of handling the message, scheduling a
// debounced auto-sync.
func TestModelUpdate_TaskDoneMsg_SchedulesMutationAutoSync(t *testing.T) {
	reader := &stubReader{}
	m := model{reader: reader, syncer: &stubSyncer{}, list: tasklist.New(nil)}

	newModel, cmd := m.Update(taskDoneMsg{})
	m = newModel.(model)
	require.NotNil(t, cmd)
	assert.Equal(t, 1, m.mutationSyncGen, "expected taskDoneMsg to schedule a mutation auto-sync")
}

// TestModelUpdate_UndoRedoMsg_DoesNotScheduleMutationAutoSync verifies
// undo/redo stay sync-exempt per requirements.md's auto-sync section: the
// periodic timer will eventually pick up whatever state the user leaves
// after an undo/redo, so these must not bump mutationSyncGen.
func TestModelUpdate_UndoRedoMsg_DoesNotScheduleMutationAutoSync(t *testing.T) {
	reader := &stubReader{}
	m := model{reader: reader, syncer: &stubSyncer{}, list: tasklist.New(nil)}

	newModel, _ := m.Update(undoAppliedMsg{description: "delete"})
	m = newModel.(model)
	assert.Equal(t, 0, m.mutationSyncGen, "undo must not schedule a mutation auto-sync")

	newModel, _ = m.Update(redoAppliedMsg{description: "delete"})
	m = newModel.(model)
	assert.Equal(t, 0, m.mutationSyncGen, "redo must not schedule a mutation auto-sync")
}
