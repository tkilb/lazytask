package main

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/popup"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
	"github.com/tkilb/lazytask/internal/undo"
)

// TestModelUpdate_UKeyNoOpWhenNothingToUndo verifies "u" shows an info
// popup rather than a no-op when the undo stack is empty.
func TestModelUpdate_UKeyNoOpWhenNothingToUndo(t *testing.T) {
	m := model{list: tasklist.New(nil)}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	m = newModel.(model)
	assert.Nil(t, cmd)
	msg, ok := m.popups.Current()
	require.True(t, ok)
	assert.Equal(t, popup.Info, msg.Severity)
	assert.Equal(t, "Nothing to undo", msg.Text)
}

// TestModelUpdate_CtrlRNoOpWhenNothingToRedo verifies "ctrl+r" shows an
// info popup rather than a no-op when the redo stack is empty.
func TestModelUpdate_CtrlRNoOpWhenNothingToRedo(t *testing.T) {
	m := model{list: tasklist.New(nil)}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = newModel.(model)
	assert.Nil(t, cmd)
	msg, ok := m.popups.Current()
	require.True(t, ok)
	assert.Equal(t, popup.Info, msg.Severity)
	assert.Equal(t, "Nothing to redo", msg.Text)
}

// TestModelUpdate_UndoDoneRestoresTask verifies marking a task done pushes
// an undo.Action that restores it to pending, and that "u" runs it.
func TestModelUpdate_UndoDoneRestoresTask(t *testing.T) {
	doner := &stubDoner{}
	restorer := &stubRestorer{}
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}}
	m := model{reader: reader, doner: doner, restorer: restorer, list: tasklist.New(reader.tasks), completing: true}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = newModel.(model)
	require.NotNil(t, cmd)
	msg := cmd()
	newModel, _ = m.Update(msg) // pushes the undo.Action from taskDoneMsg
	m = newModel.(model)
	require.True(t, m.undo.CanUndo())

	newModel, undoCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	m = newModel.(model)
	require.NotNil(t, undoCmd)
	undoMsg := undoCmd()
	applied, ok := undoMsg.(undoAppliedMsg)
	require.True(t, ok, "expected undoAppliedMsg, got %T", undoMsg)
	assert.Contains(t, applied.description, "abc-123")
	assert.Equal(t, []string{"abc-123"}, restorer.ids, "undo of done should restore the task to pending")

	newModel, _ = m.Update(undoMsg)
	m = newModel.(model)
	assert.True(t, m.undo.CanRedo())
}

// TestModelUpdate_UndoDeleteThenRedo verifies deleting a task pushes an
// undo.Action that restores it, and redo re-applies the delete.
func TestModelUpdate_UndoDeleteThenRedo(t *testing.T) {
	deleter := &stubDeleter{}
	restorer := &stubRestorer{}
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}}
	m := model{reader: reader, deleter: deleter, restorer: restorer, list: tasklist.New(reader.tasks), deleting: true}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = newModel.(model)
	require.NotNil(t, cmd)
	msg := cmd()
	newModel, _ = m.Update(msg)
	m = newModel.(model)
	assert.Equal(t, []string{"abc-123"}, deleter.ids)

	newModel, undoCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	m = newModel.(model)
	undoMsg := undoCmd()
	_, ok := undoMsg.(undoAppliedMsg)
	require.True(t, ok)
	assert.Equal(t, []string{"abc-123"}, restorer.ids)

	newModel, _ = m.Update(undoMsg)
	m = newModel.(model)
	m.popups = m.popups.Dismiss() // clear the "Undone: ..." popup so the next keypress isn't swallowed as a dismissal

	newModel, redoCmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = newModel.(model)
	require.NotNil(t, redoCmd)
	redoMsg := redoCmd()
	_, ok = redoMsg.(redoAppliedMsg)
	require.True(t, ok)
	assert.Equal(t, []string{"abc-123", "abc-123"}, deleter.ids, "redo of delete should delete again")
}

// TestModelUpdate_UndoDeleteFromDoneTabRestoresToDone is a regression test:
// deleting a task while viewing the Done tab must undo back to Done, not
// straight to pending (Taskwarrior's `done` refuses a non-pending task, so
// undoing this must restore-then-done, mirroring doneTask's own
// wasDeleted handling).
func TestModelUpdate_UndoDeleteFromDoneTabRestoresToDone(t *testing.T) {
	deleter := &stubDeleter{}
	restorer := &stubRestorer{}
	doner := &stubDoner{}
	tasks := []taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}
	list := tasklist.New(tasks).NextStatus() // Todo -> Done
	m := model{deleter: deleter, restorer: restorer, doner: doner, list: list, deleting: true}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = newModel.(model)
	require.NotNil(t, cmd)
	msg := cmd()
	newModel, _ = m.Update(msg)
	m = newModel.(model)
	assert.Equal(t, []string{"abc-123"}, deleter.ids)

	newModel, undoCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	m = newModel.(model)
	require.NotNil(t, undoCmd)
	undoMsg := undoCmd()
	_, ok := undoMsg.(undoAppliedMsg)
	require.True(t, ok)
	assert.Equal(t, []string{"abc-123"}, restorer.ids, "undo must restore to pending before re-marking done")
	assert.Equal(t, []string{"abc-123"}, doner.ids, "undo of a delete-from-Done must re-mark the task done, not leave it pending")
}

// TestModelUpdate_UndoPurgeReimportsSnapshot verifies purging a task
// captures its full field snapshot and undo re-imports it.
func TestModelUpdate_UndoPurgeReimportsSnapshot(t *testing.T) {
	purger := &stubPurger{}
	importer := &stubImporter{}
	task := taskwarrior.Task{ID: 1, UUID: "abc-123", Description: "Buy milk", Project: "chores"}
	reader := &stubReader{tasks: []taskwarrior.Task{task}}
	m := model{reader: reader, purger: purger, importer: importer, list: tasklist.New(reader.tasks), purging: true}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = newModel.(model)
	require.NotNil(t, cmd)
	msg := cmd()
	newModel, _ = m.Update(msg)
	m = newModel.(model)
	assert.Equal(t, []string{"abc-123"}, purger.ids)

	newModel, undoCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	m = newModel.(model)
	undoMsg := undoCmd()
	_, ok := undoMsg.(undoAppliedMsg)
	require.True(t, ok)
	require.Len(t, importer.calls, 1)
	assert.Contains(t, string(importer.calls[0]), "chores")
	assert.Contains(t, string(importer.calls[0]), "abc-123")
}

// TestModelUpdate_UndoErrMsgSetsErrPopup verifies a failed undo shows an
// error popup, same as other mutation failures.
func TestModelUpdate_UndoErrMsgSetsErrPopup(t *testing.T) {
	m := model{list: tasklist.New(nil)}
	wantErr := errors.New("undo failed")

	newModel, cmd := m.Update(undoErrMsg{err: wantErr})
	m = newModel.(model)
	assertErrPopup(t, m, wantErr)
	assert.Nil(t, cmd)
}

// TestModelUpdate_RedoErrMsgSetsErrPopup verifies a failed redo shows an
// error popup, same as other mutation failures.
func TestModelUpdate_RedoErrMsgSetsErrPopup(t *testing.T) {
	m := model{list: tasklist.New(nil)}
	wantErr := errors.New("redo failed")

	newModel, cmd := m.Update(redoErrMsg{err: wantErr})
	m = newModel.(model)
	assertErrPopup(t, m, wantErr)
	assert.Nil(t, cmd)
}

// TestModelUpdate_PushingActionAfterUndoDiscardsRedo is a light smoke test
// that the model's undo.Stack is a real multi-level stack (see
// internal/undo for the exhaustive unit tests of this behavior).
func TestModelUpdate_PushingActionAfterUndoDiscardsRedo(t *testing.T) {
	var m model
	m.undo.Push(undo.Action{Description: "a"})
	m.undo.Push(undo.Action{Description: "b"})
	m.undo.Undo()
	m.undo.Push(undo.Action{Description: "c"})
	assert.False(t, m.undo.CanRedo())
}
