package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tkilb/lazytask/internal/editbuffer"
	"github.com/tkilb/lazytask/internal/editor"
	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/addform"
	"github.com/tkilb/lazytask/internal/ui/popup"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
)

// stubReader is a test double for TaskReader, avoiding any real `task`
// process invocation.
type stubReader struct {
	tasks []taskwarrior.Task
	err   error
	calls int
}

func (s *stubReader) Export(ctx context.Context, filters ...string) ([]taskwarrior.Task, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.tasks, nil
}

// stubAdder is a test double for TaskAdder, avoiding any real `task`
// process invocation.
type stubAdder struct {
	err          error
	descriptions []string
}

func (s *stubAdder) Add(ctx context.Context, description string, extraArgs ...string) (int, error) {
	s.descriptions = append(s.descriptions, description)
	if s.err != nil {
		return 0, s.err
	}
	return len(s.descriptions), nil
}

// stubDoner is a test double for TaskDoner, avoiding any real `task`
// process invocation.
type stubDoner struct {
	err  error
	ids  []string
	call int
}

func (s *stubDoner) Done(ctx context.Context, id string) error {
	s.call++
	s.ids = append(s.ids, id)
	return s.err
}

// stubDeleter is a test double for TaskDeleter, avoiding any real `task`
// process invocation.
type stubDeleter struct {
	err  error
	ids  []string
	call int
}

func (s *stubDeleter) Delete(ctx context.Context, id string) error {
	s.call++
	s.ids = append(s.ids, id)
	return s.err
}

// stubImporter is a test double for TaskImporter, avoiding any real `task`
// process invocation.
type stubImporter struct {
	err   error
	calls [][]byte
}

func (s *stubImporter) Import(ctx context.Context, data []byte) error {
	s.calls = append(s.calls, data)
	return s.err
}

func TestModelInit_FetchesTasks(t *testing.T) {
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}}
	m := model{reader: reader, list: tasklist.New(nil)}

	cmd := m.Init()
	assert.NotNil(t, cmd)

	msg := cmd()
	loaded, ok := msg.(tasksLoadedMsg)
	assert.True(t, ok)
	assert.Equal(t, reader.tasks, loaded.tasks)
}

func TestModelUpdate_TasksLoadedMsg(t *testing.T) {
	m := model{list: tasklist.New(nil)}
	tasks := []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}

	newModel, cmd := m.Update(tasksLoadedMsg{tasks: tasks})
	mm, ok := newModel.(model)
	assert.True(t, ok)
	assert.Nil(t, cmd)
	assert.False(t, mm.popups.Active())

	selected, ok := mm.list.Selected()
	assert.True(t, ok)
	assert.Equal(t, "Buy milk", selected.Description)
}

func TestModelUpdate_TasksErrMsg(t *testing.T) {
	m := model{list: tasklist.New(nil)}
	wantErr := errors.New("boom")

	newModel, cmd := m.Update(tasksErrMsg{err: wantErr})
	mm, ok := newModel.(model)
	assert.True(t, ok)
	assert.Nil(t, cmd)
	popupMsg, popupOk := mm.popups.Current()
	require.True(t, popupOk)
	assert.Equal(t, popup.Error, popupMsg.Severity)
	assert.Equal(t, wantErr.Error(), popupMsg.Text)
}

func TestModelUpdate_RefreshKeyTriggersFetch(t *testing.T) {
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}}
	m := model{reader: reader, list: tasklist.New(nil)}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	_, ok := newModel.(model)
	assert.True(t, ok)
	assert.NotNil(t, cmd)

	msg := cmd()
	_, ok = msg.(tasksLoadedMsg)
	assert.True(t, ok)
	assert.Equal(t, 1, reader.calls)
}

func TestModelUpdate_Quit(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.Msg
	}{
		{name: "quit on 'q'", msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}},
		{name: "quit on ctrl+c", msg: tea.KeyMsg{Type: tea.KeyCtrlC}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{list: tasklist.New(nil)}
			newModel, cmd := m.Update(tt.msg)
			mm, ok := newModel.(model)
			assert.True(t, ok)
			assert.True(t, mm.quitting)

			assert.NotNil(t, cmd)
			res := cmd()
			_, isQuit := res.(tea.QuitMsg)
			assert.True(t, isQuit)
		})
	}
}

func TestModelView(t *testing.T) {
	t.Run("quitting view", func(t *testing.T) {
		m := model{quitting: true}
		assert.Equal(t, "Exiting lazytask...\n", m.View())
	})

	t.Run("shows error popup when present", func(t *testing.T) {
		m := model{list: tasklist.New(nil), popups: popup.Model{}.Push(popup.Message{Severity: popup.Error, Text: "task binary not found"})}
		assert.Contains(t, m.View(), "task binary not found")
		assert.Contains(t, m.View(), "Error")
	})

	t.Run("shows add/refresh/quit hint", func(t *testing.T) {
		m := model{list: tasklist.New(nil)}
		view := m.View()
		for _, want := range []string{"a", "add", "d", "done", "x", "delete", "e", "edit", "r", "refresh", "q", "quit"} {
			assert.Contains(t, view, want)
		}
	})

	t.Run("shows delete confirmation prompt", func(t *testing.T) {
		m := model{
			list:     tasklist.New([]taskwarrior.Task{{ID: 1, Description: "Buy milk"}}),
			deleting: true,
		}
		view := m.View()
		assert.Contains(t, view, `Delete task 1 "Buy milk"?`)
		assert.Contains(t, view, "Confirm")
		assert.Contains(t, view, "confirm")
		assert.Contains(t, view, "cancel")
	})

	t.Run("shows add form when adding", func(t *testing.T) {
		m := model{list: tasklist.New(nil), add: addform.New(), adding: true}
		view := m.View()
		assert.Contains(t, view, "Add Task")
		assert.Contains(t, view, "enter")
		assert.Contains(t, view, "add")
		assert.Contains(t, view, "esc")
		assert.Contains(t, view, "cancel")
	})

	t.Run("status panel shows selected task's id/status/project", func(t *testing.T) {
		m := model{list: tasklist.New([]taskwarrior.Task{
			{ID: 3, Status: "pending", Project: "home", Description: "Mow lawn"},
		})}
		assert.Contains(t, m.View(), "#3  [pending]  P:home  T:(none)")
	})

	t.Run("status panel shows placeholder project when task has none", func(t *testing.T) {
		m := model{list: tasklist.New([]taskwarrior.Task{
			{ID: 5, Status: "pending", Description: "Buy milk"},
		})}
		assert.Contains(t, m.View(), "#5  [pending]  P:(none)  T:(none)")
	})

	t.Run("status panel shows tags when present", func(t *testing.T) {
		m := model{list: tasklist.New([]taskwarrior.Task{
			{ID: 7, Status: "pending", Project: "home", Tags: []string{"urgent", "chores"}, Description: "Mow lawn"},
		})}
		assert.Contains(t, m.View(), "#7  [pending]  P:home  T:urgent,chores")
	})

	t.Run("status panel shows placeholder when no task selected", func(t *testing.T) {
		m := model{list: tasklist.New(nil)}
		assert.Contains(t, m.View(), "(no task selected)")
	})
}

func TestStatusPanelContent_UpdatesWithSelection(t *testing.T) {
	m := model{list: tasklist.New([]taskwarrior.Task{
		{ID: 1, Status: "pending", Project: "work", Description: "A"},
		{ID: 2, Status: "pending", Project: "home", Description: "B"},
	})}
	assert.Equal(t, "#1  [pending]  P:work  T:(none)", m.statusPanelContent())

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	mm := newModel.(model)
	assert.Equal(t, "#2  [pending]  P:home  T:(none)", mm.statusPanelContent())
}

func TestModelUpdate_AKeyEntersAddingMode(t *testing.T) {
	m := model{list: tasklist.New(nil), add: addform.New()}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	mm, ok := newModel.(model)
	assert.True(t, ok)
	assert.True(t, mm.adding)
	assert.True(t, mm.add.Focused())
	assert.NotNil(t, cmd) // textinput.Blink from add.Init()
}

func TestModelUpdate_AddingTypeAndSubmit(t *testing.T) {
	adder := &stubAdder{}
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}}
	m := model{reader: reader, adder: adder, list: tasklist.New(nil), add: addform.New(), adding: true}
	m.add = m.add.Focus()

	for _, r := range "Buy milk" {
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = newModel.(model)
	}
	assert.Equal(t, "Buy milk", m.add.Value())

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(model)
	assert.False(t, m.adding)
	require.NotNil(t, cmd)

	msg := cmd()
	addedMsg, ok := msg.(taskAddedMsg)
	assert.True(t, ok)
	assert.Equal(t, 1, addedMsg.id)
	assert.Equal(t, []string{"Buy milk"}, adder.descriptions)

	// Regression: the resulting taskAddedMsg must trigger an automatic
	// refresh without requiring the user to press 'r' manually.
	newModel, refreshCmd := m.Update(msg)
	m = newModel.(model)
	require.NotNil(t, refreshCmd)

	refreshMsg := refreshCmd()
	loaded, ok := refreshMsg.(tasksLoadedMsg)
	assert.True(t, ok)
	assert.Equal(t, reader.tasks, loaded.tasks)
	assert.Equal(t, 1, reader.calls)
}

func TestModelUpdate_AddingSubmitEmptyDescriptionShowsWarningPopup(t *testing.T) {
	adder := &stubAdder{}
	m := model{adder: adder, list: tasklist.New(nil), add: addform.New(), adding: true}
	m.add = m.add.Focus()

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(model)
	assert.True(t, m.adding)
	assert.Nil(t, cmd)
	assert.Empty(t, adder.descriptions)
	msg, ok := m.popups.Current()
	require.True(t, ok)
	assert.Equal(t, popup.Warning, msg.Severity)
}

func TestModelUpdate_AddingEscCancels(t *testing.T) {
	m := model{list: tasklist.New(nil), add: addform.New(), adding: true}
	m.add = m.add.Focus()
	m.add, _ = m.add.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(model)
	assert.False(t, m.adding)
	assert.False(t, m.add.Focused())
	assert.Nil(t, cmd)
}

func TestModelUpdate_TaskAddedMsgTriggersRefresh(t *testing.T) {
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 2, Description: "Water plants"}}}
	m := model{reader: reader, list: tasklist.New(nil), add: addform.New()}

	newModel, cmd := m.Update(taskAddedMsg{})
	m = newModel.(model)
	require.NotNil(t, cmd)

	msg := cmd()
	loaded, ok := msg.(tasksLoadedMsg)
	assert.True(t, ok)
	assert.Equal(t, reader.tasks, loaded.tasks)
}

func TestModelUpdate_TaskAddedMsg_FocusesNewlyCreatedTask(t *testing.T) {
	reader := &stubReader{tasks: []taskwarrior.Task{
		{ID: 1, Description: "Buy milk"},
		{ID: 2, Description: "Water plants"},
		{ID: 3, Description: "Newly added task"},
	}}
	m := model{reader: reader, list: tasklist.New(nil), add: addform.New()}

	// Simulate Add() reporting the new task's numeric ID.
	newModel, cmd := m.Update(taskAddedMsg{id: 3})
	m = newModel.(model)
	require.NotNil(t, cmd)
	assert.Equal(t, 3, m.pendingFocusID)

	// The subsequent refresh should select task 3 and clear the pending
	// focus so later refreshes (from unrelated actions) don't re-apply it.
	msg := cmd()
	loaded := msg.(tasksLoadedMsg)
	newModel, _ = m.Update(loaded)
	m = newModel.(model)

	selected, ok := m.list.Selected()
	require.True(t, ok)
	assert.Equal(t, 3, selected.ID)
	assert.Zero(t, m.pendingFocusID)
}

func TestModelUpdate_TaskAddErrMsgSetsErr(t *testing.T) {
	m := model{list: tasklist.New(nil), add: addform.New()}
	wantErr := errors.New("add failed")

	newModel, cmd := m.Update(taskAddErrMsg{err: wantErr})
	m = newModel.(model)
	assertErrPopup(t, m, wantErr)
	assert.Nil(t, cmd)
}

func TestModelUpdate_DKeyMarksSelectedTaskDone(t *testing.T) {
	doner := &stubDoner{}
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}}
	m := model{reader: reader, doner: doner, list: tasklist.New(reader.tasks)}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = newModel.(model)
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(taskDoneMsg)
	assert.True(t, ok)
	assert.Equal(t, []string{"abc-123"}, doner.ids)

	// Regression: a successful done must trigger an automatic refresh.
	newModel, refreshCmd := m.Update(msg)
	m = newModel.(model)
	require.NotNil(t, refreshCmd)
	refreshMsg := refreshCmd()
	_, ok = refreshMsg.(tasksLoadedMsg)
	assert.True(t, ok)
	assert.Equal(t, 1, reader.calls)
}

func TestModelUpdate_DKeyNoSelectionNoOp(t *testing.T) {
	doner := &stubDoner{}
	m := model{doner: doner, list: tasklist.New(nil)}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = newModel.(model)
	assert.Nil(t, cmd)
	assert.Zero(t, doner.call)
}

func TestModelUpdate_TaskDoneErrMsgSetsErr(t *testing.T) {
	m := model{list: tasklist.New(nil)}
	wantErr := errors.New("done failed")

	newModel, cmd := m.Update(taskDoneErrMsg{err: wantErr})
	m = newModel.(model)
	assertErrPopup(t, m, wantErr)
	assert.Nil(t, cmd)
}

func TestModelUpdate_XKeyEntersDeletingMode(t *testing.T) {
	m := model{list: tasklist.New([]taskwarrior.Task{{ID: 1, Description: "Buy milk"}})}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = newModel.(model)
	assert.True(t, m.deleting)
	assert.Nil(t, cmd)
}

func TestModelUpdate_XKeyNoSelectionNoOp(t *testing.T) {
	m := model{list: tasklist.New(nil)}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = newModel.(model)
	assert.False(t, m.deleting)
	assert.Nil(t, cmd)
}

func TestModelUpdate_DeletingConfirmYDeletes(t *testing.T) {
	deleter := &stubDeleter{}
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}}
	m := model{
		reader:   reader,
		deleter:  deleter,
		list:     tasklist.New(reader.tasks),
		deleting: true,
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = newModel.(model)
	assert.False(t, m.deleting)
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(taskDeletedMsg)
	assert.True(t, ok)
	assert.Equal(t, []string{"abc-123"}, deleter.ids)

	// Regression: a successful delete must trigger an automatic refresh.
	newModel, refreshCmd := m.Update(msg)
	m = newModel.(model)
	require.NotNil(t, refreshCmd)
	refreshMsg := refreshCmd()
	_, ok = refreshMsg.(tasksLoadedMsg)
	assert.True(t, ok)
}

func TestModelUpdate_DeletingConfirmNCancels(t *testing.T) {
	deleter := &stubDeleter{}
	m := model{
		deleter:  deleter,
		list:     tasklist.New([]taskwarrior.Task{{ID: 1, Description: "Buy milk"}}),
		deleting: true,
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = newModel.(model)
	assert.False(t, m.deleting)
	assert.Nil(t, cmd)
	assert.Zero(t, deleter.call)
}

func TestModelUpdate_DeletingConfirmEscCancels(t *testing.T) {
	deleter := &stubDeleter{}
	m := model{
		deleter:  deleter,
		list:     tasklist.New([]taskwarrior.Task{{ID: 1, Description: "Buy milk"}}),
		deleting: true,
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(model)
	assert.False(t, m.deleting)
	assert.Nil(t, cmd)
	assert.Zero(t, deleter.call)
}

func TestModelUpdate_TaskDeleteErrMsgSetsErr(t *testing.T) {
	m := model{list: tasklist.New(nil)}
	wantErr := errors.New("delete failed")

	newModel, cmd := m.Update(taskDeleteErrMsg{err: wantErr})
	m = newModel.(model)
	assertErrPopup(t, m, wantErr)
	assert.Nil(t, cmd)
}

func TestModelUpdate_EKeyWithSelectionReturnsCmd(t *testing.T) {
	// editTask creates a real temp file synchronously (via editor.Prepare)
	// as soon as it's called, ahead of the returned tea.Cmd/tea.ExecProcess
	// actually running the editor. Since this test never drives that
	// process to completion (which is what normally cleans the file up),
	// redirect temp file creation into a scratch dir so nothing leaks into
	// the real OS temp dir.
	t.Setenv("TMPDIR", t.TempDir())

	m := model{
		importer: &stubImporter{},
		list:     tasklist.New([]taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}),
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m = newModel.(model)
	assert.False(t, m.popups.Active())
	assert.NotNil(t, cmd)
}

func TestModelUpdate_EKeyNoSelectionNoOp(t *testing.T) {
	m := model{importer: &stubImporter{}, list: tasklist.New(nil)}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m = newModel.(model)
	assert.Nil(t, cmd)
}

func TestModelUpdate_TaskEditedMsgTriggersRefresh(t *testing.T) {
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}}
	m := model{reader: reader, list: tasklist.New(nil)}

	newModel, cmd := m.Update(taskEditedMsg{})
	m = newModel.(model)
	assert.False(t, m.popups.Active())
	require.NotNil(t, cmd)

	msg := cmd()
	loaded, ok := msg.(tasksLoadedMsg)
	assert.True(t, ok)
	assert.Equal(t, reader.tasks, loaded.tasks)
}

func TestModelUpdate_TaskEditErrMsgSetsErr(t *testing.T) {
	m := model{list: tasklist.New(nil)}
	wantErr := errors.New("edit failed")

	newModel, cmd := m.Update(taskEditErrMsg{err: wantErr})
	m = newModel.(model)
	assertErrPopup(t, m, wantErr)
	assert.Nil(t, cmd)
}

func TestEditTaskCallback_ImportsEditedTask(t *testing.T) {
	original := taskwarrior.Task{UUID: "abc-123", Description: "Buy milk", Status: "pending"}
	_, session, err := editor.Prepare(editbuffer.Serialize(taskwarrior.Task{
		UUID: "abc-123", Description: "Buy milk and eggs", Status: "pending",
	}))
	require.NoError(t, err)
	defer session.Close()

	importer := &stubImporter{}
	msg := editTaskCallback(importer, session, original)(nil)

	_, ok := msg.(taskEditedMsg)
	assert.True(t, ok)
	require.Len(t, importer.calls, 1)

	var got taskwarrior.Task
	require.NoError(t, json.Unmarshal(importer.calls[0], &got))
	assert.Equal(t, "abc-123", got.UUID)
	assert.Equal(t, "Buy milk and eggs", got.Description)
}

func TestEditTaskCallback_EditorErrorSkipsImport(t *testing.T) {
	original := taskwarrior.Task{UUID: "abc-123"}
	_, session, err := editor.Prepare(editbuffer.Serialize(original))
	require.NoError(t, err)
	defer session.Close()

	importer := &stubImporter{}
	msg := editTaskCallback(importer, session, original)(errors.New("boom"))

	errMsg, ok := msg.(taskEditErrMsg)
	require.True(t, ok)
	assert.Error(t, errMsg.err)
	assert.Empty(t, importer.calls)
}

func TestEditTaskCallback_MissingUUIDSkipsImport(t *testing.T) {
	original := taskwarrior.Task{Description: "no uuid here"}
	_, session, err := editor.Prepare(editbuffer.Serialize(original))
	require.NoError(t, err)
	defer session.Close()

	importer := &stubImporter{}
	msg := editTaskCallback(importer, session, original)(nil)

	errMsg, ok := msg.(taskEditErrMsg)
	require.True(t, ok)
	assert.Contains(t, errMsg.err.Error(), "uuid")
	assert.Empty(t, importer.calls)
}

func TestEditTaskCallback_MalformedBufferSkipsImport(t *testing.T) {
	original := taskwarrior.Task{UUID: "abc-123", Description: "Buy milk"}
	_, session, err := editor.Prepare("Project: no description field at all\n---\n")
	require.NoError(t, err)
	defer session.Close()

	importer := &stubImporter{}
	msg := editTaskCallback(importer, session, original)(nil)

	errMsg, ok := msg.(taskEditErrMsg)
	require.True(t, ok)
	assert.Contains(t, errMsg.err.Error(), "Description")
	assert.Empty(t, importer.calls)
}

func TestModelUpdate_NumberKeysChangeFocus(t *testing.T) {
	tests := []struct {
		key       string
		wantFocus panelFocus
	}{
		{key: "1", wantFocus: focusStatus},
		{key: "2", wantFocus: focusTasks},
		{key: "3", wantFocus: focusProjects},
		{key: "4", wantFocus: focusTags},
		{key: "0", wantFocus: focusDetails},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			m := model{list: tasklist.New(nil)}
			newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})
			m = newModel.(model)
			assert.Equal(t, tt.wantFocus, m.focus)
			assert.Nil(t, cmd)
		})
	}
}

func TestModelUpdate_TabCyclesFocusForward(t *testing.T) {
	m := model{list: tasklist.New(nil)} // starts at focusTasks (index 1 in focusCycle)
	wantOrder := []panelFocus{focusProjects, focusTags, focusDetails, focusStatus, focusTasks}

	for _, want := range wantOrder {
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = newModel.(model)
		assert.Equal(t, want, m.focus)
	}
}

func TestModelUpdate_ShiftTabCyclesFocusBackward(t *testing.T) {
	m := model{list: tasklist.New(nil)} // starts at focusTasks (index 1 in focusCycle)

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = newModel.(model)
	assert.Equal(t, focusStatus, m.focus)
}

func TestModelUpdate_NavigationOnlyReachesListWhenTasksFocused(t *testing.T) {
	tasks := []taskwarrior.Task{
		{ID: 1, Description: "Buy milk"},
		{ID: 2, Description: "Water plants"},
	}

	t.Run("forwarded when Tasks panel focused", func(t *testing.T) {
		m := model{list: tasklist.New(tasks), focus: focusTasks}
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = newModel.(model)
		selected, ok := m.list.Selected()
		require.True(t, ok)
		assert.Equal(t, 2, selected.ID)
	})

	t.Run("not forwarded when another panel focused", func(t *testing.T) {
		m := model{list: tasklist.New(tasks), focus: focusStatus}
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = newModel.(model)
		selected, ok := m.list.Selected()
		require.True(t, ok)
		assert.Equal(t, 1, selected.ID)
	})
}

func TestModelUpdate_WindowSizeMsgResizesListPanel(t *testing.T) {
	m := model{list: tasklist.New(nil)}

	newModel, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = newModel.(model)
	assert.Equal(t, 120, m.width)
	assert.Equal(t, 40, m.height)
	assert.Nil(t, cmd)
}

func TestModelView_RendersGridWithAllPanelTitles(t *testing.T) {
	m := model{list: tasklist.New(nil), add: addform.New()}
	view := m.View()

	for _, want := range []string{"[2]-Todo - Done - Deleted", "[1]-Status", "[3]-Projects", "[4]-Tags", "[0]-Details"} {
		assert.Contains(t, view, want)
	}
}

func TestEditTaskCallback_ImportErrSetsErrMsg(t *testing.T) {
	original := taskwarrior.Task{UUID: "abc-123"}
	_, session, err := editor.Prepare(editbuffer.Serialize(original))
	require.NoError(t, err)
	defer session.Close()

	importer := &stubImporter{err: errors.New("import failed")}
	msg := editTaskCallback(importer, session, original)(nil)

	errMsg, ok := msg.(taskEditErrMsg)
	require.True(t, ok)
	assert.Contains(t, errMsg.err.Error(), "import failed")
}

// assertErrPopup asserts that m has a single queued error-severity popup
// matching wantErr.
func assertErrPopup(t *testing.T, m model, wantErr error) {
	t.Helper()
	msg, ok := m.popups.Current()
	require.True(t, ok, "expected a queued popup")
	assert.Equal(t, popup.Error, msg.Severity)
	assert.Equal(t, wantErr.Error(), msg.Text)
}
