package main

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
)

// TestPurgeAllTasks_EmptyIsNoOp verifies purgeAllTasks returns a nil
// tea.Cmd (not a msg-returning func) when there is nothing to purge, so
// callers can skip dispatching it entirely.
func TestPurgeAllTasks_EmptyIsNoOp(t *testing.T) {
	purger := &stubPurger{}
	importer := &stubImporter{}

	cmd := purgeAllTasks(purger, importer, nil)
	assert.Nil(t, cmd)
	assert.Zero(t, purger.call)
}

// TestPurgeAllTasks_PurgesEveryTaskAndBuildsUndo verifies every task is
// purged (in order) and the returned undo.Action re-imports a combined
// snapshot covering all of them.
func TestPurgeAllTasks_PurgesEveryTaskAndBuildsUndo(t *testing.T) {
	purger := &stubPurger{}
	importer := &stubImporter{}
	tasks := []taskwarrior.Task{
		{ID: 1, UUID: "uuid-1", Description: "Buy milk", Project: "chores"},
		{ID: 2, UUID: "uuid-2", Description: "Walk dog", Project: "chores"},
	}

	cmd := purgeAllTasks(purger, importer, tasks)
	require.NotNil(t, cmd)

	msg := cmd()
	purged, ok := msg.(tasksPurgedMsg)
	require.True(t, ok, "expected tasksPurgedMsg, got %T", msg)
	assert.Equal(t, []string{"uuid-1", "uuid-2"}, purger.ids)
	assert.Equal(t, "purge 2 tasks", purged.action.Description)

	// Undo should re-import a single combined snapshot covering both
	// tasks, without needing any further Purge calls.
	require.NoError(t, purged.action.Undo())
	require.Len(t, importer.calls, 1)
	assert.Contains(t, string(importer.calls[0]), "uuid-1")
	assert.Contains(t, string(importer.calls[0]), "uuid-2")
	assert.Contains(t, string(importer.calls[0]), "chores")

	// Redo should re-purge every task again.
	require.NoError(t, purged.action.Redo())
	assert.Equal(t, []string{"uuid-1", "uuid-2", "uuid-1", "uuid-2"}, purger.ids)
}

// TestPurgeAllTasks_PurgeErrorReturnsErrMsg verifies a failing Purge call
// surfaces as tasksPurgeErrMsg rather than panicking or silently dropping
// the error.
func TestPurgeAllTasks_PurgeErrorReturnsErrMsg(t *testing.T) {
	wantErr := errors.New("purge failed")
	purger := &stubPurger{err: wantErr}
	importer := &stubImporter{}
	tasks := []taskwarrior.Task{{ID: 1, UUID: "uuid-1", Description: "Buy milk"}}

	cmd := purgeAllTasks(purger, importer, tasks)
	require.NotNil(t, cmd)

	msg := cmd()
	errMsg, ok := msg.(tasksPurgeErrMsg)
	require.True(t, ok, "expected tasksPurgeErrMsg, got %T", msg)
	assert.Equal(t, wantErr, errMsg.err)
	assert.Empty(t, importer.calls, "must not attempt to build/return an undo action on failure")
}

// TestPurgeAllTasks_FallsBackToNumericIDWhenUUIDMissing verifies taskID's
// UUID-preferred/numeric-ID-fallback behavior is used consistently for
// every task in the batch, same as the single-task purgeTask.
func TestPurgeAllTasks_FallsBackToNumericIDWhenUUIDMissing(t *testing.T) {
	purger := &stubPurger{}
	importer := &stubImporter{}
	tasks := []taskwarrior.Task{{ID: 7, Description: "No UUID yet"}}

	cmd := purgeAllTasks(purger, importer, tasks)
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(tasksPurgedMsg)
	require.True(t, ok)
	assert.Equal(t, []string{"7"}, purger.ids)
}

// deletedTabList builds a tasklist.Model already switched to the Deleted
// tab and populated with tasks, for the UI-wiring tests below.
func deletedTabList(tasks []taskwarrior.Task) tasklist.Model {
	return tasklist.New(tasks).NextStatus().NextStatus() // Todo -> Done -> Deleted
}

// TestModelUpdate_ShiftXKeyEntersPurgingAllModeOnDeletedTab verifies "X"
// only arms bulk-purge confirmation while on the Deleted tab.
func TestModelUpdate_ShiftXKeyEntersPurgingAllModeOnDeletedTab(t *testing.T) {
	tasks := []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}
	m := model{list: deletedTabList(tasks)}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	m = newModel.(model)
	assert.True(t, m.purgingAll)
	assert.Nil(t, cmd)
}

// TestModelUpdate_ShiftXKeyNoOpOnOtherTabs verifies "X" does nothing on the
// Todo/Done tabs, where bulk-purge was never offered.
func TestModelUpdate_ShiftXKeyNoOpOnOtherTabs(t *testing.T) {
	tasks := []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}

	m := model{list: tasklist.New(tasks)} // Todo
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	m = newModel.(model)
	assert.False(t, m.purgingAll)
	assert.Nil(t, cmd)

	m = model{list: tasklist.New(tasks).NextStatus()} // Done
	newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	m = newModel.(model)
	assert.False(t, m.purgingAll)
	assert.Nil(t, cmd)
}

// TestModelUpdate_ShiftXKeyNoOpWhenDeletedTabEmpty verifies "X" is a no-op
// when there is nothing to purge, mirroring "x"'s own no-selection no-op.
func TestModelUpdate_ShiftXKeyNoOpWhenDeletedTabEmpty(t *testing.T) {
	m := model{list: deletedTabList(nil)}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	m = newModel.(model)
	assert.False(t, m.purgingAll)
	assert.Nil(t, cmd)
}

// TestModelUpdate_PurgingAllConfirmYPurgesAll verifies confirming the bulk
// prompt dispatches purgeAllTasks over every task currently in the list.
func TestModelUpdate_PurgingAllConfirmYPurgesAll(t *testing.T) {
	purger := &stubPurger{}
	importer := &stubImporter{}
	tasks := []taskwarrior.Task{
		{ID: 1, UUID: "uuid-1", Description: "Buy milk"},
		{ID: 2, UUID: "uuid-2", Description: "Walk dog"},
	}
	reader := &stubReader{tasks: tasks}
	m := model{
		reader:     reader,
		purger:     purger,
		importer:   importer,
		list:       deletedTabList(tasks),
		purgingAll: true,
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = newModel.(model)
	assert.False(t, m.purgingAll)
	require.NotNil(t, cmd)

	msg := cmd()
	purged, ok := msg.(tasksPurgedMsg)
	require.True(t, ok)
	assert.Equal(t, []string{"uuid-1", "uuid-2"}, purger.ids)
	assert.Equal(t, "purge 2 tasks", purged.action.Description)

	// Regression: a successful bulk purge must trigger an automatic
	// refresh, same as the single-task purge flow.
	newModel, refreshCmd := m.Update(msg)
	m = newModel.(model)
	require.NotNil(t, refreshCmd)
	refreshMsgs := runBatch(refreshCmd)
	_, ok = findTasksLoaded(refreshMsgs)
	assert.True(t, ok)
}

// TestModelUpdate_PurgingAllConfirmNCancels verifies "n" cancels without
// purging anything.
func TestModelUpdate_PurgingAllConfirmNCancels(t *testing.T) {
	purger := &stubPurger{}
	tasks := []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}
	m := model{purger: purger, list: deletedTabList(tasks), purgingAll: true}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = newModel.(model)
	assert.False(t, m.purgingAll)
	assert.Nil(t, cmd)
	assert.Zero(t, purger.call)
}

// TestModelUpdate_PurgingAllConfirmEscCancels verifies "esc" cancels the
// same way "n" does.
func TestModelUpdate_PurgingAllConfirmEscCancels(t *testing.T) {
	purger := &stubPurger{}
	tasks := []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}
	m := model{purger: purger, list: deletedTabList(tasks), purgingAll: true}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(model)
	assert.False(t, m.purgingAll)
	assert.Nil(t, cmd)
	assert.Zero(t, purger.call)
}

// TestModelUpdate_TasksPurgedMsgTriggersRefresh mirrors
// TestModelUpdate_TaskPurgedMsgTriggersRefresh for the bulk variant.
func TestModelUpdate_TasksPurgedMsgTriggersRefresh(t *testing.T) {
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}}
	m := model{reader: reader, list: tasklist.New(nil)}

	newModel, cmd := m.Update(tasksPurgedMsg{})
	_ = newModel.(model)
	require.NotNil(t, cmd)
	msgs := runBatch(cmd)
	_, sawTasks := findTasksLoaded(msgs)
	assert.True(t, sawTasks, "expected a task refresh after bulk purge")
	var sawProjects bool
	for _, msg := range msgs {
		if _, ok := msg.(projectsLoadedMsg); ok {
			sawProjects = true
		}
	}
	assert.True(t, sawProjects, "expected a projects refresh after bulk purge, so counts stay in sync")
}

// TestModelUpdate_TasksPurgeErrMsgSetsErr mirrors
// TestModelUpdate_TaskPurgeErrMsgSetsErr for the bulk variant.
func TestModelUpdate_TasksPurgeErrMsgSetsErr(t *testing.T) {
	m := model{list: tasklist.New(nil)}
	wantErr := errors.New("bulk purge failed")

	newModel, cmd := m.Update(tasksPurgeErrMsg{err: wantErr})
	m = newModel.(model)
	assertErrPopup(t, m, wantErr)
	assert.Nil(t, cmd)
}

// TestView_PurgingAllShowsExactCount verifies the confirmation popup states
// the exact number of tasks about to be purged, per PLAN.md's decided
// confirmation design (count-and-confirm, not a bare y/n).
func TestView_PurgingAllShowsExactCount(t *testing.T) {
	tasks := []taskwarrior.Task{
		{ID: 1, Description: "Buy milk"},
		{ID: 2, Description: "Walk dog"},
		{ID: 3, Description: "Mow lawn"},
	}
	m := model{list: deletedTabList(tasks), purgingAll: true}

	view := m.View()
	assert.Contains(t, view, "all 3 deleted tasks")
	assert.Contains(t, view, "cannot be")
	assert.Contains(t, view, "undone.")
	assert.Contains(t, view, "Confirm")
	assert.Contains(t, view, "confirm")
	assert.Contains(t, view, "cancel")
}

// TestView_PurgingAllShowsSingularNoun verifies the count text uses
// "task" (not "tasks") when exactly one task would be purged.
func TestView_PurgingAllShowsSingularNoun(t *testing.T) {
	tasks := []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}
	m := model{list: deletedTabList(tasks), purgingAll: true}

	view := m.View()
	assert.Contains(t, view, "all 1 deleted task?")
}

// TestStatusBindings_ShiftXNotInStatusBar verifies "X" (purge all) is no
// longer part of the always-visible status bar on any tab, per the "?"
// help popup design (2026-09-30): Tasks-panel-local status bar hints were
// trimmed down to just navigation, with the rest (including "X") moved to
// the "?" popup only (see TestHelpSections_ContainsPurgeAllHint).
func TestStatusBindings_ShiftXNotInStatusBar(t *testing.T) {
	tasks := []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}

	for _, m := range []model{
		{list: tasklist.New(tasks), focus: focusTasks},   // Todo
		{list: deletedTabList(tasks), focus: focusTasks}, // Deleted
	} {
		for _, b := range m.statusBindings() {
			assert.NotEqual(t, "X", b.Key, "X hint must not show in the status bar on any tab")
		}
	}
}

// TestHelpSections_ContainsPurgeAllHint verifies the "?" help popup's
// Local section still documents "X"/"purge all" while focused on the Tasks
// panel's Deleted tab, even though it was trimmed from the status bar.
func TestHelpSections_ContainsPurgeAllHint(t *testing.T) {
	tasks := []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}
	m := model{list: deletedTabList(tasks), focus: focusTasks}

	var found bool
	for _, sec := range m.helpSections() {
		for _, b := range sec.Bindings {
			if b.Key == "X" {
				found = true
				assert.Contains(t, b.Label, "purge all")
			}
		}
	}
	assert.True(t, found, "expected helpSections to document the X/purge-all key on the Deleted tab")
}
