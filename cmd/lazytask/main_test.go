package main

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/addform"
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
	assert.Nil(t, mm.err)

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
	assert.Equal(t, wantErr, mm.err)
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

	t.Run("shows error when present", func(t *testing.T) {
		m := model{list: tasklist.New(nil), err: errors.New("task binary not found")}
		assert.Contains(t, m.View(), "error: task binary not found")
	})

	t.Run("shows add/refresh/quit hint", func(t *testing.T) {
		m := model{list: tasklist.New(nil)}
		assert.Contains(t, m.View(), "(a) add  (r) refresh  (q) quit")
	})

	t.Run("shows add form when adding", func(t *testing.T) {
		m := model{list: tasklist.New(nil), add: addform.New(), adding: true}
		view := m.View()
		assert.Contains(t, view, "Add Task")
		assert.Contains(t, view, "(enter) add  (esc) cancel")
	})
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
	_, ok := msg.(taskAddedMsg)
	assert.True(t, ok)
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

func TestModelUpdate_AddingSubmitEmptyDescriptionNoOp(t *testing.T) {
	adder := &stubAdder{}
	m := model{adder: adder, list: tasklist.New(nil), add: addform.New(), adding: true}
	m.add = m.add.Focus()

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(model)
	assert.True(t, m.adding)
	assert.Nil(t, cmd)
	assert.Empty(t, adder.descriptions)
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

func TestModelUpdate_TaskAddErrMsgSetsErr(t *testing.T) {
	m := model{list: tasklist.New(nil), add: addform.New()}
	wantErr := errors.New("add failed")

	newModel, cmd := m.Update(taskAddErrMsg{err: wantErr})
	m = newModel.(model)
	assert.Equal(t, wantErr, m.err)
	assert.Nil(t, cmd)
}
