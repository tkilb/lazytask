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

func doneTabList(tasks []taskwarrior.Task) tasklist.Model {
	return tasklist.New(tasks).NextStatus()
}

func TestDeleteAllDoneTasks_EmptyIsNoOp(t *testing.T) {
	assert.Nil(t, deleteAllDoneTasks(&stubDeleter{}, &stubRestorer{}, &stubDoner{}, nil))
}

func TestDeleteAllDoneTasks_DeletesAllAndBuildsUndo(t *testing.T) {
	deleter, restorer, doner := &stubDeleter{}, &stubRestorer{}, &stubDoner{}
	tasks := []taskwarrior.Task{{ID: 1, UUID: "u1"}, {ID: 2, UUID: "u2"}}

	msg := deleteAllDoneTasks(deleter, restorer, doner, tasks)()
	deleted, ok := msg.(tasksDeletedMsg)
	require.True(t, ok, "got %T", msg)
	assert.Equal(t, []string{"u1", "u2"}, deleter.ids)
	assert.Equal(t, "delete 2 done tasks", deleted.action.Description)

	require.NoError(t, deleted.action.Undo())
	assert.Equal(t, []string{"u1", "u2"}, restorer.ids)
	assert.Equal(t, []string{"u1", "u2"}, doner.ids)

	require.NoError(t, deleted.action.Redo())
	assert.Equal(t, []string{"u1", "u2", "u1", "u2"}, deleter.ids)
}

func TestDeleteAllDoneTasks_ErrorReturnsErrMsg(t *testing.T) {
	wantErr := errors.New("boom")
	tasks := []taskwarrior.Task{{ID: 1, UUID: "u1"}}

	msg := deleteAllDoneTasks(&stubDeleter{err: wantErr}, &stubRestorer{}, &stubDoner{}, tasks)()
	errMsg, ok := msg.(tasksDeleteErrMsg)
	require.True(t, ok)
	assert.Equal(t, wantErr, errMsg.err)
}

func TestModelUpdate_ShiftXArmsDeleteAllOnDoneTabOnly(t *testing.T) {
	tasks := []taskwarrior.Task{{ID: 1, Description: "a"}}
	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")}

	nm, _ := model{list: doneTabList(tasks)}.Update(key)
	assert.True(t, nm.(model).deletingAll)

	nm, _ = model{list: tasklist.New(tasks)}.Update(key)
	assert.False(t, nm.(model).deletingAll)

	nm, _ = model{list: doneTabList(nil)}.Update(key)
	assert.False(t, nm.(model).deletingAll)
}

func TestModelUpdate_DeletingAllConfirm(t *testing.T) {
	tasks := []taskwarrior.Task{{ID: 1, UUID: "u1"}, {ID: 2, UUID: "u2"}}
	deleter := &stubDeleter{}
	m := model{reader: &stubReader{tasks: tasks}, deleter: deleter, restorer: &stubRestorer{}, doner: &stubDoner{}, list: doneTabList(tasks), deletingAll: true}

	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	assert.False(t, nm.(model).deletingAll)
	require.NotNil(t, cmd)
	_, ok := cmd().(tasksDeletedMsg)
	require.True(t, ok)
	assert.Equal(t, []string{"u1", "u2"}, deleter.ids)

	m.deletingAll = true
	nm, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, nm.(model).deletingAll)
	assert.Nil(t, cmd)
	assert.Len(t, deleter.ids, 2)
}

func TestModelUpdate_TasksDeletedMsgTriggersRefresh(t *testing.T) {
	m := model{reader: &stubReader{}, list: tasklist.New(nil)}
	_, cmd := m.Update(tasksDeletedMsg{})
	require.NotNil(t, cmd)
	_, ok := findTasksLoaded(runBatch(cmd))
	assert.True(t, ok)
}
