package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tkilb/lazytask/internal/config"
	"github.com/tkilb/lazytask/internal/editbuffer"
	"github.com/tkilb/lazytask/internal/editor"
	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/datepick"
	"github.com/tkilb/lazytask/internal/ui/popup"
	"github.com/tkilb/lazytask/internal/ui/projects"
	"github.com/tkilb/lazytask/internal/ui/taskform"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
)

// stubReader is a test double for TaskReader, avoiding any real `task`
// process invocation.
type stubReader struct {
	tasks       []taskwarrior.Task
	err         error
	calls       int
	lastFilters []string
}

func (s *stubReader) Export(ctx context.Context, filters ...string) ([]taskwarrior.Task, error) {
	s.calls++
	s.lastFilters = filters
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
	extraArgs    [][]string
}

func (s *stubAdder) Add(ctx context.Context, description string, extraArgs ...string) (int, error) {
	s.descriptions = append(s.descriptions, description)
	s.extraArgs = append(s.extraArgs, extraArgs)
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

// stubRestorer is a test double for TaskRestorer, avoiding any real `task`
// process invocation.
type stubRestorer struct {
	err  error
	ids  []string
	call int
}

func (s *stubRestorer) Restore(ctx context.Context, id string) error {
	s.call++
	s.ids = append(s.ids, id)
	return s.err
}

// stubPurger is a test double for TaskPurger, avoiding any real `task`
// process invocation.
type stubPurger struct {
	err  error
	ids  []string
	call int
}

func (s *stubPurger) Purge(ctx context.Context, id string) error {
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

// stubPrioritizer is a test double for TaskPrioritizer, avoiding any real
// `task` process invocation.
type stubPrioritizer struct {
	err        error
	ids        []string
	priorities []string
	call       int
}

func (s *stubPrioritizer) SetPriority(ctx context.Context, id, priority string) error {
	s.call++
	s.ids = append(s.ids, id)
	s.priorities = append(s.priorities, priority)
	return s.err
}

// stubDueSetter is a test double for TaskDueSetter, avoiding any real
// `task` process invocation.
type stubDueSetter struct {
	err  error
	ids  []string
	dues []string
	call int
}

func (s *stubDueSetter) SetDue(ctx context.Context, id, due string) error {
	s.call++
	s.ids = append(s.ids, id)
	s.dues = append(s.dues, due)
	return s.err
}

// stubReRanker is a test double for TaskReRanker, avoiding any real `task`
// process invocation.
type stubReRanker struct {
	err   error
	ids   []string
	ranks []float64
	call  int
}

func (s *stubReRanker) SetUrgencyOffset(ctx context.Context, id string, rank float64) error {
	s.call++
	s.ids = append(s.ids, id)
	s.ranks = append(s.ranks, rank)
	return s.err
}

// runBatch executes cmd, and if it returns a tea.BatchMsg (e.g. from
// refreshes that now fetch tasks and projects concurrently via
// tea.Batch), executes each of the batched sub-commands as well,
// returning all resulting messages in encounter order.
func runBatch(cmd tea.Cmd) []tea.Msg {
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var msgs []tea.Msg
	for _, c := range batch {
		if c == nil {
			continue
		}
		msgs = append(msgs, runBatch(c)...)
	}
	return msgs
}

// findTasksLoaded returns the first tasksLoadedMsg in msgs, if any.
func findTasksLoaded(msgs []tea.Msg) (tasksLoadedMsg, bool) {
	for _, m := range msgs {
		if tm, ok := m.(tasksLoadedMsg); ok {
			return tm, true
		}
	}
	return tasksLoadedMsg{}, false
}

func TestModelInit_FetchesTasks(t *testing.T) {
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}}
	m := model{reader: reader, list: tasklist.New(nil)}

	cmd := m.Init()
	assert.NotNil(t, cmd)

	loaded, ok := findTasksLoaded(runBatch(cmd))
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

// TestModelUpdate_RKeyIsNoOpForNow documents that 'r' no longer triggers a
// manual refresh (removed since every mutation/tab-switch already
// refetches automatically) and is not yet bound to anything else; it's
// reserved for the upcoming restore action (requirements.md Phase 2).
func TestModelUpdate_RKeyIsNoOpForNow(t *testing.T) {
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}}
	m := model{reader: reader, list: tasklist.New(nil)}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	_, ok := newModel.(model)
	assert.True(t, ok)
	assert.Nil(t, cmd)
	assert.Equal(t, 0, reader.calls)
}

// TestModelUpdate_TasksLocalKeysRequireTasksFocus verifies the new
// global/local keybinding split: Tasks-panel-only actions (done/delete/
// edit/tab-cycle) must not fire while some other panel has focus. "a" (add)
// is global (see TestModelUpdate_GlobalKeysWorkFromAnyFocus) and so is
// deliberately excluded here.
func TestModelUpdate_TasksLocalKeysRequireTasksFocus(t *testing.T) {
	for _, key := range []string{"d", "x", "e", "[", "]"} {
		t.Run(key, func(t *testing.T) {
			m := model{
				list:  tasklist.New([]taskwarrior.Task{{ID: 1, Description: "Buy milk"}}),
				focus: focusStatus,
			}
			newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
			mm, ok := newModel.(model)
			assert.True(t, ok)
			assert.Nil(t, cmd)
			assert.False(t, mm.adding)
			assert.False(t, mm.deleting)
		})
	}
}

// TestModelUpdate_GlobalKeysWorkFromAnyFocus verifies global bindings
// (quit, panel-focus keys, add) still fire regardless of which panel has
// focus.
func TestModelUpdate_GlobalKeysWorkFromAnyFocus(t *testing.T) {
	m := model{list: tasklist.New(nil), focus: focusStatus}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	mm, ok := newModel.(model)
	assert.True(t, ok)
	assert.True(t, mm.quitting)
	assert.NotNil(t, cmd)
}

// TestModelUpdate_AddIsGlobal verifies "a" opens the add-task form from any
// panel focus, not just Tasks.
func TestModelUpdate_AddIsGlobal(t *testing.T) {
	for _, focus := range []panelFocus{focusStatus, focusTasks, focusProjects, focusTags, focusDetails} {
		t.Run(panelTitle(focus), func(t *testing.T) {
			m := model{list: tasklist.New(nil), add: taskform.New(), focus: focus}
			newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
			mm, ok := newModel.(model)
			assert.True(t, ok)
			assert.True(t, mm.adding)
		})
	}
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

	t.Run("shows Tasks-local hints when Tasks panel is focused", func(t *testing.T) {
		m := model{list: tasklist.New(nil)} // zero-value focus == focusTasks
		view := m.View()
		for _, want := range []string{"a", "add", "d", "done", "x", "delete", "e", "edit", "q", "quit"} {
			assert.Contains(t, view, want)
		}
		assert.NotContains(t, view, "refresh")
	})

	t.Run("hides Tasks-local hints when another panel is focused", func(t *testing.T) {
		m := model{list: tasklist.New(nil), focus: focusStatus}
		view := m.View()
		assert.Contains(t, view, "quit")
		assert.Contains(t, view, "add")
		for _, notWant := range []string{"done", "delete", "edit", "refresh"} {
			assert.NotContains(t, view, notWant)
		}
	})

	t.Run("hides reopen hint on Todo tab", func(t *testing.T) {
		m := model{list: tasklist.New(nil)} // zero-value status == TabTodo
		assert.NotContains(t, m.View(), "reopen")
	})

	t.Run("shows reopen hint on Done tab", func(t *testing.T) {
		m := model{list: tasklist.New(nil).NextStatus()} // Todo -> Done
		assert.Contains(t, m.View(), "reopen")
	})

	t.Run("hides done hint on Done tab", func(t *testing.T) {
		m := model{list: tasklist.New(nil).NextStatus()} // Todo -> Done
		view := m.View()
		assert.NotContains(t, view, "done")
	})

	t.Run("shows restore hint (not reopen) and purge hint (not delete) on Deleted tab", func(t *testing.T) {
		m := model{list: tasklist.New(nil).NextStatus().NextStatus()} // Todo -> Done -> Deleted
		view := m.View()
		assert.Contains(t, view, "restore")
		assert.NotContains(t, view, "reopen")
		assert.Contains(t, view, "purge")
	})

	t.Run("shows delete confirmation prompt on Todo tab (with id)", func(t *testing.T) {
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

	t.Run("shows delete confirmation prompt on Done tab (no id)", func(t *testing.T) {
		m := model{
			list:     tasklist.New([]taskwarrior.Task{{ID: 0, Description: "Buy milk"}}).NextStatus(),
			deleting: true,
		}
		view := m.View()
		assert.Contains(t, view, `Delete "Buy milk"?`)
		assert.NotContains(t, view, "task 0")
	})

	t.Run("shows purge confirmation prompt (no id)", func(t *testing.T) {
		m := model{
			list:    tasklist.New([]taskwarrior.Task{{ID: 0, Description: "Buy milk"}}),
			purging: true,
		}
		view := m.View()
		assert.Contains(t, view, `Permanently delete "Buy milk"?`)
		assert.Contains(t, view, "cannot be")
		assert.Contains(t, view, "undone.")
		assert.Contains(t, view, "Confirm")
		assert.Contains(t, view, "confirm")
		assert.Contains(t, view, "cancel")
	})

	t.Run("shows done confirmation prompt on Todo tab (with id)", func(t *testing.T) {
		m := model{
			list:       tasklist.New([]taskwarrior.Task{{ID: 1, Description: "Buy milk"}}),
			completing: true,
		}
		view := m.View()
		assert.Contains(t, view, `Mark task 1 "Buy milk" as done?`)
		assert.Contains(t, view, "Confirm")
		assert.Contains(t, view, "confirm")
		assert.Contains(t, view, "cancel")
	})

	t.Run("shows done confirmation prompt on Deleted tab (no id)", func(t *testing.T) {
		m := model{
			list:       tasklist.New([]taskwarrior.Task{{ID: 0, Description: "Buy milk"}}).NextStatus().NextStatus(),
			completing: true,
		}
		view := m.View()
		assert.Contains(t, view, `Mark "Buy milk" as done?`)
	})

	t.Run("shows reopen confirmation prompt on Done tab (no id)", func(t *testing.T) {
		m := model{
			list:      tasklist.New([]taskwarrior.Task{{ID: 0, Description: "Buy milk"}}).NextStatus(),
			restoring: true,
		}
		view := m.View()
		assert.Contains(t, view, `Reopen "Buy milk"?`)
	})

	t.Run("shows restore confirmation prompt on Deleted tab (no id)", func(t *testing.T) {
		m := model{
			list:      tasklist.New([]taskwarrior.Task{{ID: 0, Description: "Buy milk"}}).NextStatus().NextStatus(),
			restoring: true,
		}
		view := m.View()
		assert.Contains(t, view, `Restore "Buy milk" as a todo?`)
	})

	t.Run("shows add form when adding", func(t *testing.T) {
		m := model{list: tasklist.New(nil), add: taskform.New(), adding: true}
		view := m.View()
		assert.Contains(t, view, "Description")
		assert.Contains(t, view, "Project")
		assert.Contains(t, view, "Priority")
		assert.Contains(t, view, "Due Date")
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

func TestDetailsPanelContent(t *testing.T) {
	t.Run("shows full task detail", func(t *testing.T) {
		m := model{list: tasklist.New([]taskwarrior.Task{
			{
				ID:            7,
				UUID:          "abc-123",
				Description:   "Mow lawn",
				Status:        "pending",
				Project:       "home",
				Priority:      "H",
				Due:           "20260101T000000Z",
				Entry:         "20250101T000000Z",
				Modified:      "20250102T000000Z",
				Tags:          []string{"urgent", "chores"},
				Urgency:       5.5,
				UrgencyOffset: 2.25,
			},
		})}
		content := m.detailsPanelContent()
		assert.Contains(t, content, "ID:          7")
		assert.Contains(t, content, "UUID:        abc-123")
		assert.Contains(t, content, "Description: Mow lawn")
		assert.Contains(t, content, "Status:      pending")
		assert.Contains(t, content, "Project:     home")
		assert.Contains(t, content, "Tags:        urgent,chores")
		assert.Contains(t, content, "Priority:    H")
		assert.Contains(t, content, "Due:         20260101T000000Z")
		assert.Contains(t, content, "Urgency:     5.50")
		assert.Contains(t, content, "Urg.Offset:  2.2500")
		assert.Contains(t, content, "Entry:       20250101T000000Z")
		assert.Contains(t, content, "Modified:    20250102T000000Z")
		assert.NotContains(t, content, "End:")
	})

	t.Run("shows placeholders for empty project, tags, priority, due", func(t *testing.T) {
		m := model{list: tasklist.New([]taskwarrior.Task{
			{ID: 5, Status: "pending", Description: "Buy milk"},
		})}
		content := m.detailsPanelContent()
		assert.Contains(t, content, "Project:     (none)")
		assert.Contains(t, content, "Tags:        (none)")
		assert.Contains(t, content, "Priority:    (none)")
		assert.Contains(t, content, "Due:         (none)")
	})

	t.Run("shows End when task is completed", func(t *testing.T) {
		m := model{list: tasklist.New([]taskwarrior.Task{
			{ID: 9, Status: "completed", Description: "Done task", End: "20250103T000000Z"},
		})}
		assert.Contains(t, m.detailsPanelContent(), "End:         20250103T000000Z")
	})

	t.Run("shows placeholder when no task selected", func(t *testing.T) {
		m := model{list: tasklist.New(nil)}
		assert.Equal(t, "(no task selected)", m.detailsPanelContent())
	})

	t.Run("updates live as selection moves", func(t *testing.T) {
		m := model{list: tasklist.New([]taskwarrior.Task{
			{ID: 1, Status: "pending", Description: "A"},
			{ID: 2, Status: "pending", Description: "B"},
		})}
		assert.Contains(t, m.detailsPanelContent(), "Description: A")

		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		mm := newModel.(model)
		assert.Contains(t, mm.detailsPanelContent(), "Description: B")
	})
}

func TestModelUpdate_AKeyEntersAddingMode(t *testing.T) {
	m := model{list: tasklist.New(nil), add: taskform.New()}

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
	m := model{reader: reader, adder: adder, list: tasklist.New(nil), add: taskform.New(), adding: true}
	m.add = m.add.Focus()

	for _, r := range "Buy milk" {
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = newModel.(model)
	}
	assert.Equal(t, "Buy milk", m.add.Description())

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

	refreshMsg := runBatch(refreshCmd)
	loaded, ok := findTasksLoaded(refreshMsg)
	assert.True(t, ok)
	assert.Equal(t, reader.tasks, loaded.tasks)
	assert.Equal(t, 2, reader.calls) // one for tasks, one for the projects panel
}

// TestModelUpdate_AddingAutoAssignsSelectedProjectFilter verifies that
// opening the add form (via "a") while a real project filter is selected
// (via the Projects panel) pre-fills the form's Project field with that
// project, so new tasks are automatically scoped to the currently-filtered
// project unless the user edits/clears the field before submitting.
func TestModelUpdate_AddingAutoAssignsSelectedProjectFilter(t *testing.T) {
	t.Run("real project filter is applied", func(t *testing.T) {
		adder := &stubAdder{}
		project := "chores"
		m := model{adder: adder, list: tasklist.New(nil), add: taskform.New(), filter: filterState{project: &project}}

		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
		m = newModel.(model)
		for _, r := range "Mow lawn" {
			newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			m = newModel.(model)
		}

		newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		_ = newModel.(model)
		require.NotNil(t, cmd)
		cmd()

		require.Len(t, adder.extraArgs, 1)
		assert.Equal(t, []string{"project:chores", "priority:M"}, adder.extraArgs[0])
	})

	t.Run("no filter (all) adds no project arg", func(t *testing.T) {
		adder := &stubAdder{}
		m := model{adder: adder, list: tasklist.New(nil), add: taskform.New()}

		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
		m = newModel.(model)
		for _, r := range "Buy milk" {
			newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			m = newModel.(model)
		}

		newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		_ = newModel.(model)
		require.NotNil(t, cmd)
		cmd()

		require.Len(t, adder.extraArgs, 1)
		assert.Equal(t, []string{"priority:M"}, adder.extraArgs[0])
	})

	t.Run("(none) filter adds no project arg", func(t *testing.T) {
		adder := &stubAdder{}
		none := ""
		m := model{adder: adder, list: tasklist.New(nil), add: taskform.New(), filter: filterState{project: &none}}

		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
		m = newModel.(model)
		for _, r := range "Buy milk" {
			newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			m = newModel.(model)
		}

		newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		_ = newModel.(model)
		require.NotNil(t, cmd)
		cmd()

		require.Len(t, adder.extraArgs, 1)
		assert.Equal(t, []string{"priority:M"}, adder.extraArgs[0])
	})
}

func TestModelUpdate_AddingSubmitEmptyDescriptionShowsWarningPopup(t *testing.T) {
	adder := &stubAdder{}
	m := model{adder: adder, list: tasklist.New(nil), add: taskform.New(), adding: true}
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
	m := model{list: tasklist.New(nil), add: taskform.New(), adding: true}
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
	m := model{reader: reader, list: tasklist.New(nil), add: taskform.New()}

	newModel, cmd := m.Update(taskAddedMsg{})
	m = newModel.(model)
	require.NotNil(t, cmd)

	loaded, ok := findTasksLoaded(runBatch(cmd))
	assert.True(t, ok)
	assert.Equal(t, reader.tasks, loaded.tasks)
}

func TestModelUpdate_TaskAddedMsg_FocusesNewlyCreatedTask(t *testing.T) {
	reader := &stubReader{tasks: []taskwarrior.Task{
		{ID: 1, Description: "Buy milk"},
		{ID: 2, Description: "Water plants"},
		{ID: 3, Description: "Newly added task"},
	}}
	m := model{reader: reader, list: tasklist.New(nil), add: taskform.New()}

	// Simulate Add() reporting the new task's numeric ID.
	newModel, cmd := m.Update(taskAddedMsg{id: 3})
	m = newModel.(model)
	require.NotNil(t, cmd)
	assert.Equal(t, 3, m.pendingFocusID)

	// The subsequent refresh should select task 3 and clear the pending
	// focus so later refreshes (from unrelated actions) don't re-apply it.
	loaded, ok := findTasksLoaded(runBatch(cmd))
	require.True(t, ok)
	newModel, _ = m.Update(loaded)
	m = newModel.(model)

	selected, ok := m.list.Selected()
	require.True(t, ok)
	assert.Equal(t, 3, selected.ID)
	assert.Zero(t, m.pendingFocusID)
}

func TestModelUpdate_TaskAddErrMsgSetsErr(t *testing.T) {
	m := model{list: tasklist.New(nil), add: taskform.New()}
	wantErr := errors.New("add failed")

	newModel, cmd := m.Update(taskAddErrMsg{err: wantErr})
	m = newModel.(model)
	assertErrPopup(t, m, wantErr)
	assert.Nil(t, cmd)
}

func TestModelUpdate_DKeyEntersCompletingMode(t *testing.T) {
	doner := &stubDoner{}
	m := model{doner: doner, list: tasklist.New([]taskwarrior.Task{{ID: 1, Description: "Buy milk"}})}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = newModel.(model)
	assert.True(t, m.completing)
	assert.Nil(t, cmd)
	assert.Zero(t, doner.call)
}

func TestModelUpdate_DKeyNoOpOnDoneTab(t *testing.T) {
	doner := &stubDoner{}
	tasks := []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}
	list := tasklist.New(tasks).NextStatus() // Todo -> Done
	m := model{doner: doner, list: list}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = newModel.(model)
	assert.False(t, m.completing)
	assert.Nil(t, cmd)
	assert.Zero(t, doner.call)
}

func TestModelUpdate_DKeyNoSelectionNoOp(t *testing.T) {
	doner := &stubDoner{}
	m := model{doner: doner, list: tasklist.New(nil)}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = newModel.(model)
	assert.False(t, m.completing)
	assert.Nil(t, cmd)
	assert.Zero(t, doner.call)
}

func TestModelUpdate_CompletingConfirmYDonesOnTodoTab(t *testing.T) {
	doner := &stubDoner{}
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}}
	m := model{reader: reader, doner: doner, list: tasklist.New(reader.tasks), completing: true}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = newModel.(model)
	assert.False(t, m.completing)
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(taskDoneMsg)
	assert.True(t, ok)
	assert.Equal(t, []string{"abc-123"}, doner.ids)

	// Regression: a successful done must trigger an automatic refresh.
	newModel, refreshCmd := m.Update(msg)
	m = newModel.(model)
	require.NotNil(t, refreshCmd)
	refreshMsgs := runBatch(refreshCmd)
	_, ok = findTasksLoaded(refreshMsgs)
	assert.True(t, ok)
	assert.Equal(t, 2, reader.calls, "expected both a task refresh and a projects refresh (so counts stay in sync)")
}

// TestModelUpdate_CompletingConfirmYRestoresThenDonesOnDeletedTab verifies
// the fix for a bug where marking a Deleted task done failed outright:
// Taskwarrior's `done` command refuses to act on a non-pending task, so
// confirming here must first restore the task to pending and only then
// mark it done.
func TestModelUpdate_CompletingConfirmYRestoresThenDonesOnDeletedTab(t *testing.T) {
	doner := &stubDoner{}
	restorer := &stubRestorer{}
	tasks := []taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}
	list := tasklist.New(tasks).NextStatus().NextStatus() // Todo -> Done -> Deleted
	m := model{doner: doner, restorer: restorer, list: list, completing: true}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = newModel.(model)
	assert.False(t, m.completing)
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(taskDoneMsg)
	assert.True(t, ok)
	assert.Equal(t, []string{"abc-123"}, restorer.ids)
	assert.Equal(t, []string{"abc-123"}, doner.ids)
}

func TestModelUpdate_CompletingConfirmYRestoreErrSkipsDone(t *testing.T) {
	doner := &stubDoner{}
	restorer := &stubRestorer{err: errors.New("restore failed")}
	tasks := []taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}
	list := tasklist.New(tasks).NextStatus().NextStatus() // Todo -> Done -> Deleted
	m := model{doner: doner, restorer: restorer, list: list, completing: true}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	_ = newModel.(model)
	require.NotNil(t, cmd)

	msg := cmd()
	errMsg, ok := msg.(taskDoneErrMsg)
	assert.True(t, ok)
	assert.EqualError(t, errMsg.err, "restore failed")
	assert.Zero(t, doner.call)
}

func TestModelUpdate_CompletingConfirmNCancels(t *testing.T) {
	doner := &stubDoner{}
	m := model{
		doner:      doner,
		list:       tasklist.New([]taskwarrior.Task{{ID: 1, Description: "Buy milk"}}),
		completing: true,
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = newModel.(model)
	assert.False(t, m.completing)
	assert.Nil(t, cmd)
	assert.Zero(t, doner.call)
}

func TestModelUpdate_CompletingConfirmEscCancels(t *testing.T) {
	doner := &stubDoner{}
	m := model{
		doner:      doner,
		list:       tasklist.New([]taskwarrior.Task{{ID: 1, Description: "Buy milk"}}),
		completing: true,
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(model)
	assert.False(t, m.completing)
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
	assert.False(t, m.purging)
	assert.Nil(t, cmd)
}

func TestModelUpdate_XKeyEntersPurgingModeOnDeletedTab(t *testing.T) {
	tasks := []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}
	list := tasklist.New(tasks).NextStatus().NextStatus() // Todo -> Done -> Deleted
	m := model{list: list}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = newModel.(model)
	assert.True(t, m.purging)
	assert.False(t, m.deleting)
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
	refreshMsgs := runBatch(refreshCmd)
	_, ok = findTasksLoaded(refreshMsgs)
	assert.True(t, ok)
}

func TestModelUpdate_ConfirmEnterConfirmsSameAsY(t *testing.T) {
	tasks := []taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}
	enterKey := tea.KeyMsg{Type: tea.KeyEnter}

	t.Run("deleting", func(t *testing.T) {
		deleter := &stubDeleter{}
		m := model{deleter: deleter, list: tasklist.New(tasks), deleting: true}
		newModel, cmd := m.Update(enterKey)
		m = newModel.(model)
		assert.False(t, m.deleting)
		require.NotNil(t, cmd)
		_, ok := cmd().(taskDeletedMsg)
		assert.True(t, ok)
		assert.Equal(t, []string{"abc-123"}, deleter.ids)
	})

	t.Run("purging", func(t *testing.T) {
		purger := &stubPurger{}
		m := model{purger: purger, list: tasklist.New(tasks), purging: true}
		newModel, cmd := m.Update(enterKey)
		m = newModel.(model)
		assert.False(t, m.purging)
		require.NotNil(t, cmd)
		_, ok := cmd().(taskPurgedMsg)
		assert.True(t, ok)
		assert.Equal(t, []string{"abc-123"}, purger.ids)
	})

	t.Run("completing", func(t *testing.T) {
		doner := &stubDoner{}
		m := model{doner: doner, list: tasklist.New(tasks), completing: true}
		newModel, cmd := m.Update(enterKey)
		m = newModel.(model)
		assert.False(t, m.completing)
		require.NotNil(t, cmd)
		_, ok := cmd().(taskDoneMsg)
		assert.True(t, ok)
		assert.Equal(t, []string{"abc-123"}, doner.ids)
	})

	t.Run("restoring", func(t *testing.T) {
		restorer := &stubRestorer{}
		m := model{restorer: restorer, list: tasklist.New(tasks).NextStatus(), restoring: true}
		newModel, cmd := m.Update(enterKey)
		m = newModel.(model)
		assert.False(t, m.restoring)
		require.NotNil(t, cmd)
		_, ok := cmd().(taskRestoredMsg)
		assert.True(t, ok)
		assert.Equal(t, []string{"abc-123"}, restorer.ids)
	})
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

func TestModelUpdate_RKeyNoOpOnTodoTab(t *testing.T) {
	restorer := &stubRestorer{}
	// tasklist.New defaults to the Todo tab.
	m := model{restorer: restorer, list: tasklist.New([]taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}})}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	_ = newModel.(model)
	assert.Nil(t, cmd)
	assert.Zero(t, restorer.call)
}

func TestModelUpdate_RKeyRestoresSelectedTaskOnDoneTab(t *testing.T) {
	restorer := &stubRestorer{}
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}}
	list := tasklist.New(reader.tasks).NextStatus() // Todo -> Done
	m := model{reader: reader, restorer: restorer, list: list}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = newModel.(model)
	assert.True(t, m.restoring)
	assert.Nil(t, cmd)
	assert.Zero(t, restorer.call)
}

func TestModelUpdate_RKeyRestoresSelectedTaskOnDeletedTab(t *testing.T) {
	restorer := &stubRestorer{}
	tasks := []taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}
	list := tasklist.New(tasks).NextStatus().NextStatus() // Todo -> Done -> Deleted
	m := model{restorer: restorer, list: list}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = newModel.(model)
	assert.True(t, m.restoring)
	assert.Nil(t, cmd)
	assert.Zero(t, restorer.call)
}

func TestModelUpdate_RKeyNoSelectionNoOp(t *testing.T) {
	restorer := &stubRestorer{}
	m := model{restorer: restorer, list: tasklist.New(nil).NextStatus()}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = newModel.(model)
	assert.False(t, m.restoring)
	assert.Nil(t, cmd)
	assert.Zero(t, restorer.call)
}

func TestModelUpdate_RestoringConfirmYRestoresOnDoneTab(t *testing.T) {
	restorer := &stubRestorer{}
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}}
	list := tasklist.New(reader.tasks).NextStatus() // Todo -> Done
	m := model{reader: reader, restorer: restorer, list: list, restoring: true}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = newModel.(model)
	assert.False(t, m.restoring)
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(taskRestoredMsg)
	assert.True(t, ok)
	assert.Equal(t, []string{"abc-123"}, restorer.ids)

	// Regression: a successful restore must trigger an automatic refresh.
	newModel, refreshCmd := m.Update(msg)
	m = newModel.(model)
	require.NotNil(t, refreshCmd)
	refreshMsgs := runBatch(refreshCmd)
	_, ok = findTasksLoaded(refreshMsgs)
	assert.True(t, ok)
	assert.Equal(t, 2, reader.calls, "expected both a task refresh and a projects refresh (so counts stay in sync)")
}

func TestModelUpdate_RestoringConfirmYRestoresOnDeletedTab(t *testing.T) {
	restorer := &stubRestorer{}
	tasks := []taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}
	list := tasklist.New(tasks).NextStatus().NextStatus() // Todo -> Done -> Deleted
	m := model{restorer: restorer, list: list, restoring: true}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = newModel.(model)
	assert.False(t, m.restoring)
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(taskRestoredMsg)
	assert.True(t, ok)
	assert.Equal(t, []string{"abc-123"}, restorer.ids)
}

func TestModelUpdate_RestoringConfirmNCancels(t *testing.T) {
	restorer := &stubRestorer{}
	m := model{
		restorer:  restorer,
		list:      tasklist.New([]taskwarrior.Task{{ID: 1, Description: "Buy milk"}}).NextStatus(),
		restoring: true,
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = newModel.(model)
	assert.False(t, m.restoring)
	assert.Nil(t, cmd)
	assert.Zero(t, restorer.call)
}

func TestModelUpdate_RestoringConfirmEscCancels(t *testing.T) {
	restorer := &stubRestorer{}
	m := model{
		restorer:  restorer,
		list:      tasklist.New([]taskwarrior.Task{{ID: 1, Description: "Buy milk"}}).NextStatus(),
		restoring: true,
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(model)
	assert.False(t, m.restoring)
	assert.Nil(t, cmd)
	assert.Zero(t, restorer.call)
}

func TestModelUpdate_TaskRestoredMsgTriggersRefresh(t *testing.T) {
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}}
	m := model{reader: reader, list: tasklist.New(nil)}

	newModel, cmd := m.Update(taskRestoredMsg{})
	_ = newModel.(model)
	require.NotNil(t, cmd)
	msgs := runBatch(cmd)
	_, sawTasks := findTasksLoaded(msgs)
	assert.True(t, sawTasks, "expected a task refresh after restore")
	var sawProjects bool
	for _, msg := range msgs {
		if _, ok := msg.(projectsLoadedMsg); ok {
			sawProjects = true
		}
	}
	assert.True(t, sawProjects, "expected a projects refresh after restore, so counts stay in sync")
}

func TestModelUpdate_TaskRestoreErrMsgSetsErr(t *testing.T) {
	m := model{list: tasklist.New(nil)}
	wantErr := errors.New("restore failed")

	newModel, cmd := m.Update(taskRestoreErrMsg{err: wantErr})
	m = newModel.(model)
	assertErrPopup(t, m, wantErr)
	assert.Nil(t, cmd)
}

func TestModelUpdate_PurgingConfirmYPurges(t *testing.T) {
	purger := &stubPurger{}
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}}
	m := model{
		reader:  reader,
		purger:  purger,
		list:    tasklist.New(reader.tasks),
		purging: true,
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = newModel.(model)
	assert.False(t, m.purging)
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(taskPurgedMsg)
	assert.True(t, ok)
	assert.Equal(t, []string{"abc-123"}, purger.ids)

	// Regression: a successful purge must trigger an automatic refresh.
	newModel, refreshCmd := m.Update(msg)
	m = newModel.(model)
	require.NotNil(t, refreshCmd)
	refreshMsgs := runBatch(refreshCmd)
	_, ok = findTasksLoaded(refreshMsgs)
	assert.True(t, ok)
}

func TestModelUpdate_PurgingConfirmNCancels(t *testing.T) {
	purger := &stubPurger{}
	m := model{
		purger:  purger,
		list:    tasklist.New([]taskwarrior.Task{{ID: 1, Description: "Buy milk"}}),
		purging: true,
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = newModel.(model)
	assert.False(t, m.purging)
	assert.Nil(t, cmd)
	assert.Zero(t, purger.call)
}

func TestModelUpdate_PurgingConfirmEscCancels(t *testing.T) {
	purger := &stubPurger{}
	m := model{
		purger:  purger,
		list:    tasklist.New([]taskwarrior.Task{{ID: 1, Description: "Buy milk"}}),
		purging: true,
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(model)
	assert.False(t, m.purging)
	assert.Nil(t, cmd)
	assert.Zero(t, purger.call)
}

func TestModelUpdate_TaskPurgedMsgTriggersRefresh(t *testing.T) {
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}}
	m := model{reader: reader, list: tasklist.New(nil)}

	newModel, cmd := m.Update(taskPurgedMsg{})
	_ = newModel.(model)
	require.NotNil(t, cmd)
	msgs := runBatch(cmd)
	_, sawTasks := findTasksLoaded(msgs)
	assert.True(t, sawTasks, "expected a task refresh after purge")
	var sawProjects bool
	for _, msg := range msgs {
		if _, ok := msg.(projectsLoadedMsg); ok {
			sawProjects = true
		}
	}
	assert.True(t, sawProjects, "expected a projects refresh after purge, so counts stay in sync")
}

func TestModelUpdate_TaskPurgeErrMsgSetsErr(t *testing.T) {
	m := model{list: tasklist.New(nil)}
	wantErr := errors.New("purge failed")

	newModel, cmd := m.Update(taskPurgeErrMsg{err: wantErr})
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

	loaded, ok := findTasksLoaded(runBatch(cmd))
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

func TestModelUpdate_HMLKeysSetPriority(t *testing.T) {
	for key, want := range map[string]string{"h": "H", "m": "M", "l": "L"} {
		t.Run(key, func(t *testing.T) {
			prioritizer := &stubPrioritizer{}
			m := model{
				prioritizer: prioritizer,
				list:        tasklist.New([]taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}),
			}

			newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
			m = newModel.(model)
			require.NotNil(t, cmd)

			msg := cmd()
			priMsg, ok := msg.(taskPrioritySetMsg)
			assert.True(t, ok)
			assert.Equal(t, 1, priMsg.id)
			assert.Equal(t, []string{"abc-123"}, prioritizer.ids)
			assert.Equal(t, []string{want}, prioritizer.priorities)
		})
	}
}

func TestModelUpdate_HMLKeysNoSelectionNoOp(t *testing.T) {
	m := model{prioritizer: &stubPrioritizer{}, list: tasklist.New(nil)}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m = newModel.(model)
	assert.Nil(t, cmd)
}

func TestModelUpdate_TaskPrioritySetMsgTriggersRefreshAndKeepsSelection(t *testing.T) {
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}}
	m := model{reader: reader, list: tasklist.New(nil)}

	newModel, cmd := m.Update(taskPrioritySetMsg{id: 1})
	m = newModel.(model)
	require.NotNil(t, cmd)
	assert.Equal(t, 1, m.pendingFocusID)

	loaded, ok := findTasksLoaded(runBatch(cmd))
	assert.True(t, ok)
	assert.Equal(t, reader.tasks, loaded.tasks)
}

func TestModelUpdate_TaskPriorityErrMsgSetsErr(t *testing.T) {
	m := model{list: tasklist.New(nil)}
	wantErr := errors.New("set priority failed")

	newModel, cmd := m.Update(taskPriorityErrMsg{err: wantErr})
	m = newModel.(model)
	assertErrPopup(t, m, wantErr)
	assert.Nil(t, cmd)
}

func TestModelUpdate_DKeyOpensDatePicker(t *testing.T) {
	m := model{
		focus:    focusTasks,
		duer:     &stubDueSetter{},
		datePick: datepick.New(),
		list:     tasklist.New([]taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}),
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	m = newModel.(model)
	assert.True(t, m.datePicking)
	assert.True(t, m.datePick.Focused())
	assert.NotNil(t, cmd)
}

func TestModelUpdate_DueDateKeyNoSelectionNoOp(t *testing.T) {
	m := model{focus: focusTasks, duer: &stubDueSetter{}, datePick: datepick.New(), list: tasklist.New(nil)}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	m = newModel.(model)
	assert.False(t, m.datePicking)
	assert.Nil(t, cmd)
}

func TestModelUpdate_DatePickingEscCancels(t *testing.T) {
	m := model{datePicking: true, datePick: datepick.New().Focus()}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(model)
	assert.False(t, m.datePicking)
	assert.False(t, m.datePick.Focused())
	assert.Nil(t, cmd)
}

func TestModelUpdate_DatePickingEnterValidCordCallsSetDue(t *testing.T) {
	duer := &stubDueSetter{}
	m := model{
		datePicking: true,
		datePick:    datepick.New().Focus(),
		duer:        duer,
		list:        tasklist.New([]taskwarrior.Task{{ID: 1, UUID: "abc-123", Description: "Buy milk"}}),
	}

	for _, r := range "2d" {
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = newModel.(model)
	}
	require.Equal(t, "2d", m.datePick.Value())

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(model)
	assert.False(t, m.datePicking)
	require.NotNil(t, cmd)

	msg := cmd()
	dueMsg, ok := msg.(taskDueSetMsg)
	require.True(t, ok)
	assert.Equal(t, 1, dueMsg.id)
	require.Len(t, duer.ids, 1)
	assert.Equal(t, "abc-123", duer.ids[0])

	got, err := time.Parse(taskDueLayout, duer.dues[0])
	require.NoError(t, err)
	want := time.Now().AddDate(0, 0, 2)
	assert.WithinDuration(t, want, got, 5*time.Second)
}

func TestModelUpdate_DatePickingEnterInvalidShowsWarningAndStaysOpen(t *testing.T) {
	m := model{
		datePicking: true,
		datePick:    datepick.New().Focus(),
		duer:        &stubDueSetter{},
		list:        tasklist.New([]taskwarrior.Task{{ID: 1, Description: "Buy milk"}}),
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(model)
	assert.True(t, m.datePicking)
	assert.Nil(t, cmd)
	msg, ok := m.popups.Current()
	require.True(t, ok)
	assert.Equal(t, popup.Warning, msg.Severity)
}

func TestModelUpdate_PKeyOpensProjectPicker(t *testing.T) {
	m := model{
		focus:    focusTasks,
		reader:   &stubReader{},
		projects: projects.New().SetProjects([]string{"work", "home"}),
	}

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	m = newModel.(model)
	assert.True(t, m.pickingProject)
	label, ok := m.projectPicker.Selected()
	require.True(t, ok)
	assert.Equal(t, projects.AllLabel, label)
}

func TestModelUpdate_ProjectPickingEscCancelsWithoutChangingFilter(t *testing.T) {
	m := model{
		pickingProject: true,
		projectPicker:  projects.New().SetProjects([]string{"work", "home"}),
		filter:         filterState{},
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(model)
	assert.False(t, m.pickingProject)
	assert.Nil(t, cmd)
	assert.Nil(t, m.filter.project)
}

func TestModelUpdate_ProjectPickingEnterAppliesSelectedFilter(t *testing.T) {
	reader := &stubReader{}
	m := model{
		pickingProject: true,
		projectPicker:  projects.New().SetProjects([]string{"work", "home"}),
		projects:       projects.New().SetProjects([]string{"work", "home"}),
		reader:         reader,
		filter:         filterState{},
	}

	// Move the picker's cursor down twice: past AllLabel, NoneLabel, to
	// "work".
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(model)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(model)

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(model)
	assert.False(t, m.pickingProject)
	require.NotNil(t, cmd)
	require.NotNil(t, m.filter.project)
	assert.Equal(t, "work", *m.filter.project)

	label, ok := m.projects.Selected()
	require.True(t, ok)
	assert.Equal(t, "work", label)
}

func TestModelUpdate_ProjectPickingEnterSameFilterNoOp(t *testing.T) {
	m := model{
		pickingProject: true,
		projectPicker:  projects.New().SetProjects([]string{"work", "home"}),
		projects:       projects.New().SetProjects([]string{"work", "home"}),
		filter:         filterState{},
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(model)
	assert.False(t, m.pickingProject)
	assert.Nil(t, cmd)
	assert.Nil(t, m.filter.project)
}

func TestModelUpdate_TaskDueSetMsgTriggersRefreshAndKeepsSelection(t *testing.T) {
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}}
	m := model{reader: reader, list: tasklist.New(nil)}

	newModel, cmd := m.Update(taskDueSetMsg{id: 1})
	m = newModel.(model)
	require.NotNil(t, cmd)
	assert.Equal(t, 1, m.pendingFocusID)

	loaded, ok := findTasksLoaded(runBatch(cmd))
	assert.True(t, ok)
	assert.Equal(t, reader.tasks, loaded.tasks)
}

func TestModelUpdate_TaskDueErrMsgSetsErr(t *testing.T) {
	m := model{list: tasklist.New(nil)}
	wantErr := errors.New("set due failed")

	newModel, cmd := m.Update(taskDueErrMsg{err: wantErr})
	m = newModel.(model)
	assertErrPopup(t, m, wantErr)
	assert.Nil(t, cmd)
}

func TestModelUpdate_CtrlJMovesTaskDownPastNeighbor(t *testing.T) {
	reranker := &stubReRanker{}
	// Sorted descending by urgency: task 2 (9.0) is selected (cursor 0),
	// task 1 (5.0) is the neighbor below it.
	m := model{
		reranker: reranker,
		list:     tasklist.New([]taskwarrior.Task{{ID: 1, UUID: "low", Urgency: 5.0}, {ID: 2, UUID: "high", Urgency: 9.0}}),
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	m = newModel.(model)
	require.NotNil(t, cmd)

	msg := cmd()
	reorderMsg, ok := msg.(taskReorderedMsg)
	require.True(t, ok)
	assert.Equal(t, 2, reorderMsg.id)
	require.Equal(t, []string{"high"}, reranker.ids)
	// New effective urgency (9.0 no longer applies; rank+urgency) should
	// land just below neighbor's effective urgency of 5.0.
	assert.InDelta(t, 5.0-reorderEpsilon-9.0, reranker.ranks[0], 1e-9)
}

func TestModelUpdate_CtrlKMovesTaskUpPastNeighbor(t *testing.T) {
	reranker := &stubReRanker{}
	// Sorted descending by urgency: task 2 (9.0), task 1 (5.0, selected at
	// cursor 1). Neighbor above is task 2.
	m := model{
		reranker: reranker,
		list:     tasklist.New([]taskwarrior.Task{{ID: 1, UUID: "low", Urgency: 5.0}, {ID: 2, UUID: "high", Urgency: 9.0}}).SelectID(1),
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = newModel.(model)
	require.NotNil(t, cmd)

	msg := cmd()
	reorderMsg, ok := msg.(taskReorderedMsg)
	require.True(t, ok)
	assert.Equal(t, 1, reorderMsg.id)
	require.Equal(t, []string{"low"}, reranker.ids)
	assert.InDelta(t, 9.0+reorderEpsilon-5.0, reranker.ranks[0], 1e-9)
}

func TestModelUpdate_CtrlJNoNeighborNoOp(t *testing.T) {
	m := model{
		reranker: &stubReRanker{},
		list:     tasklist.New([]taskwarrior.Task{{ID: 1, Description: "only"}}),
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	m = newModel.(model)
	assert.Nil(t, cmd)
}

func TestModelUpdate_TaskReorderedMsgTriggersRefreshAndKeepsSelection(t *testing.T) {
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, Description: "Buy milk"}}}
	m := model{reader: reader, list: tasklist.New(nil)}

	newModel, cmd := m.Update(taskReorderedMsg{id: 1})
	m = newModel.(model)
	require.NotNil(t, cmd)
	assert.Equal(t, 1, m.pendingFocusID)

	msg := cmd()
	loaded, ok := msg.(tasksLoadedMsg)
	require.True(t, ok)
	assert.Equal(t, reader.tasks, loaded.tasks)
}

func TestModelUpdate_TaskReorderErrMsgSetsErr(t *testing.T) {
	m := model{list: tasklist.New(nil)}
	wantErr := errors.New("reorder failed")

	newModel, cmd := m.Update(taskReorderErrMsg{err: wantErr})
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

// TestModelUpdate_ProjectsNavigationOnlyWhenFocused verifies up/down
// navigation reaches the Projects panel only while it has focus, mirroring
// the existing Tasks-panel behavior.
func TestModelUpdate_ProjectsNavigationOnlyWhenFocused(t *testing.T) {
	t.Run("forwarded when Projects panel focused", func(t *testing.T) {
		m := model{list: tasklist.New(nil), projects: projects.New().SetProjects([]string{"home", "work"}), focus: focusProjects}
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = newModel.(model)
		selected, ok := m.projects.Selected()
		require.True(t, ok)
		assert.Equal(t, projects.NoneLabel, selected, "(all) is pinned first, (none) second, ahead of real projects")
	})

	t.Run("not forwarded when another panel focused", func(t *testing.T) {
		m := model{list: tasklist.New(nil), projects: projects.New().SetProjects([]string{"home", "work"}), focus: focusStatus}
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = newModel.(model)
		selected, ok := m.projects.Selected()
		require.True(t, ok)
		assert.Equal(t, projects.AllLabel, selected)
	})
}

// TestModelUpdate_ProjectsNavigationAutoFiltersAndRefetchesTasks verifies
// that moving the Projects panel cursor immediately updates the shared
// filter state and triggers a Tasks refetch including the resulting
// project: filter, with no "enter" press required.
func TestModelUpdate_ProjectsNavigationAutoFiltersAndRefetchesTasks(t *testing.T) {
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, Description: "Buy milk", Project: "home"}}}
	m := model{
		reader:   reader,
		list:     tasklist.New(nil),
		projects: projects.New().SetProjects([]string{"home", "work"}),
		focus:    focusProjects,
	}

	// (all) -> (none): filters to tasks with no project at all.
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(model)
	require.NotNil(t, m.filter.project)
	assert.Equal(t, "", *m.filter.project)
	require.NotNil(t, cmd)

	// (none) -> home: now a project filter applies, so Tasks refetches.
	newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(model)
	require.NotNil(t, cmd)
	require.NotNil(t, m.filter.project)
	assert.Equal(t, "home", *m.filter.project)
	assert.Equal(t, []string{"status:pending", "project:home"}, m.taskFilters())

	loaded, ok := findTasksLoaded(runBatch(cmd))
	require.True(t, ok)
	assert.Equal(t, reader.tasks, loaded.tasks)
}

// TestModelUpdate_ProjectsNavigatingBackToAllClearsFilter verifies
// navigating the cursor back onto (all) clears any active project filter
// and refetches Tasks.
func TestModelUpdate_ProjectsNavigatingBackToAllClearsFilter(t *testing.T) {
	m := model{
		reader:   &stubReader{},
		list:     tasklist.New(nil),
		projects: projects.New().SetProjects([]string{"home"}),
		focus:    focusProjects,
		filter:   filterState{}.withProjectSelection("home"),
	}
	// Cursor starts on (all) by default, but the filter is already set to
	// "home" above (simulating a prior selection), so moving up (clamped,
	// staying on (all)) should re-apply and clear the filter.
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(model)
	require.NotNil(t, cmd)
	assert.Nil(t, m.filter.project)
	assert.Equal(t, []string{"status:pending"}, m.taskFilters())
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
	m := model{list: tasklist.New(nil), add: taskform.New()}
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

func TestProjectCounts_OnlyCountsPendingTasks(t *testing.T) {
	tasks := []taskwarrior.Task{
		{ID: 1, Status: "pending", Project: "chores"},
		{ID: 2, Status: "pending", Project: "chores"},
		{ID: 3, Status: "completed", Project: "chores"},
		{ID: 4, Status: "pending", Project: ""},
		{ID: 5, Status: "completed", Project: ""},
	}

	counts := projectCounts(tasks)

	assert.Equal(t, 3, counts.All, "only the 3 pending tasks should count toward (all)")
	assert.Equal(t, 1, counts.None, "only the 1 pending, project-less task should count toward (none)")
	assert.Equal(t, 2, counts.ByProject["chores"], "the completed chores task should not be counted")
}

// projectsPanelOnEntry builds a Projects-panel-focused model with the
// given project names loaded and the cursor moved down to the entry at
// index (0-based, counting the (all)/(none) special entries first).
func projectsPanelOnEntry(names []string, index int) model {
	m := model{
		reader:   &stubReader{},
		list:     tasklist.New(nil),
		projects: projects.New().SetProjects(names),
		focus:    focusProjects,
	}
	for i := 0; i < index; i++ {
		var newModel tea.Model
		newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = newModel.(model)
	}
	return m
}

func TestModelUpdate_ShiftRKeyEntersRenamingOnRealProject(t *testing.T) {
	m := projectsPanelOnEntry([]string{"chores", "home"}, 2) // (all), (none), chores

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
	m = newModel.(model)
	assert.True(t, m.renaming)
	assert.Equal(t, "chores", m.renameFrom)
	assert.Equal(t, "chores", m.renameInput.Value())
	assert.True(t, m.renameInput.Focused())
	require.NotNil(t, cmd) // textinput.Blink from Init()
}

func TestModelUpdate_ShiftRKeyOnAllOrNoneIsNoOp(t *testing.T) {
	for _, idx := range []int{0, 1} { // (all), (none)
		m := projectsPanelOnEntry([]string{"chores"}, idx)
		newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
		m = newModel.(model)
		assert.False(t, m.renaming)
		assert.Nil(t, cmd)
	}
}

func TestModelUpdate_RenamingEnterWithNewNameEntersConfirm(t *testing.T) {
	m := projectsPanelOnEntry([]string{"chores", "home"}, 2)
	m.renaming = true
	m.renameFrom = "chores"
	m.renameInput = m.renameInput.SetValue("errands")

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(model)
	assert.False(t, m.renaming)
	assert.True(t, m.renameConfirm)
	assert.False(t, m.renameMerging)
	assert.Equal(t, "errands", m.renameTo)
	assert.Nil(t, cmd)
}

func TestModelUpdate_RenamingEnterWithExistingProjectNameEntersMergeConfirm(t *testing.T) {
	m := projectsPanelOnEntry([]string{"chores", "home"}, 2)
	m.renaming = true
	m.renameFrom = "chores"
	m.renameInput = m.renameInput.SetValue("home")

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(model)
	assert.False(t, m.renaming)
	assert.True(t, m.renameConfirm)
	assert.True(t, m.renameMerging, "renaming onto an existing project name should trigger the merge warning")
	assert.Equal(t, "home", m.renameTo)
	assert.Nil(t, cmd)
}

func TestModelUpdate_RenamingEnterTrimsWhitespace(t *testing.T) {
	m := projectsPanelOnEntry([]string{"chores", "home"}, 2)
	m.renaming = true
	m.renameFrom = "chores"
	m.renameInput = m.renameInput.SetValue("  home  ")

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(model)
	assert.True(t, m.renameConfirm)
	assert.True(t, m.renameMerging)
	assert.Equal(t, "home", m.renameTo)
}

func TestModelUpdate_RenamingEnterSameNameIsNoOp(t *testing.T) {
	m := projectsPanelOnEntry([]string{"chores"}, 2)
	m.renaming = true
	m.renameFrom = "chores"
	m.renameInput = m.renameInput.SetValue("  chores  ")

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(model)
	assert.False(t, m.renaming)
	assert.False(t, m.renameConfirm)
	assert.Nil(t, cmd)
}

func TestModelUpdate_RenamingEnterEmptyIsNoOp(t *testing.T) {
	m := projectsPanelOnEntry([]string{"chores"}, 2)
	m.renaming = true
	m.renameFrom = "chores"
	m.renameInput = m.renameInput.SetValue("   ")

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(model)
	assert.False(t, m.renaming)
	assert.False(t, m.renameConfirm)
	assert.Nil(t, cmd)
}

func TestModelUpdate_RenamingEscCancels(t *testing.T) {
	m := projectsPanelOnEntry([]string{"chores"}, 2)
	m.renaming = true
	m.renameFrom = "chores"
	m.renameInput = m.renameInput.SetValue("errands")

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(model)
	assert.False(t, m.renaming)
	assert.False(t, m.renameInput.Focused())
	assert.Nil(t, cmd)
}

func TestModelUpdate_RenameConfirmYTriggersRenameProject(t *testing.T) {
	reader := &stubReader{tasks: []taskwarrior.Task{
		{ID: 1, UUID: "u1", Status: "pending", Project: "chores"},
		{ID: 2, UUID: "u2", Status: "completed", Project: "chores"},
		{ID: 3, UUID: "u3", Status: "deleted", Project: "chores"},
		{ID: 4, UUID: "u4", Status: "pending", Project: "home"},
	}}
	importer := &stubImporter{}
	m := model{
		reader:        reader,
		importer:      importer,
		renameConfirm: true,
		renameFrom:    "chores",
		renameTo:      "errands",
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = newModel.(model)
	assert.False(t, m.renameConfirm)
	require.NotNil(t, cmd)

	msg := cmd()
	renamedMsg, ok := msg.(projectRenamedMsg)
	require.True(t, ok)
	assert.Equal(t, "errands", renamedMsg.to)

	require.Len(t, importer.calls, 1)
	var updated []taskwarrior.Task
	require.NoError(t, json.Unmarshal(importer.calls[0], &updated))
	require.Len(t, updated, 3, "all 3 chores tasks (pending/completed/deleted) should be renamed, but not the home task")
	for _, task := range updated {
		assert.Equal(t, "errands", task.Project)
	}
}

func TestModelUpdate_RenameConfirmNCancels(t *testing.T) {
	importer := &stubImporter{}
	m := model{
		importer:      importer,
		renameConfirm: true,
		renameMerging: true,
		renameFrom:    "chores",
		renameTo:      "home",
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = newModel.(model)
	assert.False(t, m.renameConfirm)
	assert.False(t, m.renameMerging)
	assert.Empty(t, importer.calls)
	assert.Nil(t, cmd)
}

func TestFetchProjects_QueriesOnlyPendingTasks(t *testing.T) {
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, Project: "chores", Status: "pending"}}}
	cmd := fetchProjects(reader)
	msg := cmd()
	require.IsType(t, projectsLoadedMsg{}, msg)
	assert.Equal(t, []string{"status:pending"}, reader.lastFilters, "fetchProjects should not include completed/deleted tasks, so an all-done project doesn't reappear after restart")
}

func TestModel_KnownProjectsStickyWithinSession(t *testing.T) {
	m := model{
		reader:   &stubReader{},
		list:     tasklist.New(nil),
		projects: projects.New(),
	}

	// First fetch sees "chores" with one pending task.
	newModel, _ := m.Update(projectsLoadedMsg{
		projects: []string{"chores"},
		counts:   projects.Counts{All: 1, ByProject: map[string]int{"chores": 1}},
	})
	m = newModel.(model)
	assert.Equal(t, []string{"chores"}, m.projects.Projects())

	// The last "chores" task is deleted, so a fresh unfiltered pending
	// query no longer reports it at all. It should still show, now at 0.
	newModel, _ = m.Update(projectsLoadedMsg{
		projects: nil,
		counts:   projects.Counts{ByProject: map[string]int{}},
	})
	m = newModel.(model)
	assert.Equal(t, []string{"chores"}, m.projects.Projects(), "chores should remain visible for the rest of the session")
}

func TestModel_ProjectsLoadedMsg_SyncsPanelCursorToRestoredFilter(t *testing.T) {
	// Regression test: a project filter restored from a previous session
	// (see internal/config) should also move the Projects panel's visible
	// cursor to match, not just the underlying task filter/query.
	project := "work"
	m := model{
		reader:   &stubReader{},
		list:     tasklist.New(nil),
		projects: projects.New(),
		filter:   filterState{project: &project},
	}

	newModel, _ := m.Update(projectsLoadedMsg{
		projects: []string{"home", "work"},
		counts:   projects.Counts{All: 2, ByProject: map[string]int{"home": 1, "work": 1}},
	})
	m = newModel.(model)

	label, ok := m.projects.Selected()
	require.True(t, ok)
	assert.Equal(t, "work", label)
}

func TestModel_ProjectsLoadedMsg_ClearsStaleRestoredFilter(t *testing.T) {
	// Regression test: a project filter restored from a previous session
	// (see internal/config) may point at a project that no longer has any
	// pending tasks left (e.g. everything in it was completed/deleted
	// since the last session). That project won't appear in an
	// unfiltered pending-tasks query, so it's not a selectable Projects
	// entry. Previously SelectLabel would silently leave the panel's
	// cursor at its zero-value default (AllLabel, i.e. "(all)" shown as
	// selected) while m.filter still held the stale project, so Tasks
	// was fetched with a `project:<stale>` filter matching nothing —
	// the list appeared empty despite "(all)" looking selected. The
	// filter should be cleared to match what's actually shown.
	project := "old-project"
	reader := &stubReader{tasks: []taskwarrior.Task{
		{ID: 1, Description: "keep going", Project: "home"},
	}}
	m := model{
		reader:   reader,
		list:     tasklist.New(nil),
		projects: projects.New(),
		filter:   filterState{project: &project},
	}

	newModel, cmd := m.Update(projectsLoadedMsg{
		projects: []string{"home"},
		counts:   projects.Counts{All: 1, ByProject: map[string]int{"home": 1}},
	})
	m = newModel.(model)

	label, ok := m.projects.Selected()
	require.True(t, ok)
	assert.Equal(t, projects.AllLabel, label, "panel should visibly show (all) selected")
	assert.Nil(t, m.filter.project, "stale project filter should be cleared to match the visible (all) selection")

	require.NotNil(t, cmd)
	msgs := runBatch(cmd)
	tasksMsg, ok := findTasksLoaded(msgs)
	require.True(t, ok, "clearing the stale filter should trigger a re-fetch of tasks")
	assert.Equal(t, reader.tasks, tasksMsg.tasks)
	assert.Equal(t, []string{"status:pending"}, reader.lastFilters, "re-fetch should use the cleared (unfiltered) project filter")
}

func TestModel_MergeKnownProjects_UnionsAndSorts(t *testing.T) {
	m := model{}
	got := m.mergeKnownProjects([]string{"chores", "errands"})
	assert.Equal(t, []string{"chores", "errands"}, got)

	got = m.mergeKnownProjects([]string{"errands", "yardwork"})
	assert.Equal(t, []string{"chores", "errands", "yardwork"}, got, "previously seen projects should persist even if absent from a later fetch")
}

func TestModelUpdate_ProjectRenamedMsgUpdatesFollowingFilterAndRefreshes(t *testing.T) {
	reader := &stubReader{}
	m := model{
		reader:     reader,
		list:       tasklist.New(nil),
		renameFrom: "chores",
		filter:     filterState{}.withProjectSelection("chores"),
	}

	newModel, cmd := m.Update(projectRenamedMsg{to: "errands"})
	m = newModel.(model)
	require.NotNil(t, cmd)
	assert.Equal(t, "project:errands", m.filter.taskFilter(), "the active filter should follow the renamed project")

	msgs := runBatch(cmd)
	var sawTasks, sawProjects bool
	for _, msg := range msgs {
		switch msg.(type) {
		case tasksLoadedMsg, tasksErrMsg:
			sawTasks = true
		case projectsLoadedMsg, projectsErrMsg:
			sawProjects = true
		}
	}
	assert.True(t, sawTasks, "expected a task refresh after rename")
	assert.True(t, sawProjects, "expected a projects refresh after rename")
}

func TestModelUpdate_ProjectRenameErrMsgSetsErr(t *testing.T) {
	m := model{}
	wantErr := errors.New("rename failed")
	newModel, cmd := m.Update(projectRenameErrMsg{err: wantErr})
	m = newModel.(model)
	assertErrPopup(t, m, wantErr)
	assert.Nil(t, cmd)
}

func TestRenameProject_OnlyImportsWhenMatchesExist(t *testing.T) {
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, Project: "home"}}}
	importer := &stubImporter{}

	msg := renameProject(reader, importer, "chores", "errands")()
	renamedMsg, ok := msg.(projectRenamedMsg)
	require.True(t, ok)
	assert.Equal(t, "errands", renamedMsg.to)
	assert.Empty(t, importer.calls, "no matching tasks means Import should not be called")
}

func TestRenameProject_ExportErrSetsErrMsg(t *testing.T) {
	reader := &stubReader{err: errors.New("export failed")}
	importer := &stubImporter{}

	msg := renameProject(reader, importer, "chores", "errands")()
	errMsg, ok := msg.(projectRenameErrMsg)
	require.True(t, ok)
	assert.Contains(t, errMsg.err.Error(), "export failed")
}

func TestRenameProject_ImportErrSetsErrMsg(t *testing.T) {
	reader := &stubReader{tasks: []taskwarrior.Task{{ID: 1, Project: "chores"}}}
	importer := &stubImporter{err: errors.New("import failed")}

	msg := renameProject(reader, importer, "chores", "errands")()
	errMsg, ok := msg.(projectRenameErrMsg)
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

// withTempStateFile redirects the internal/config state file to a fresh
// per-test directory (via $TMPDIR), so tests never touch a real shared
// /tmp/lazytask/state.json.
func withTempStateFile(t *testing.T) {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
}

func TestInitialModel_RestoresPersistedProjectFilter(t *testing.T) {
	withTempStateFile(t)

	project := "work"
	require.NoError(t, config.Save(config.State{Project: &project}))

	m := initialModel()
	require.NotNil(t, m.filter.project)
	assert.Equal(t, "work", *m.filter.project)
}

func TestInitialModel_NoPersistedStateMeansNoFilter(t *testing.T) {
	withTempStateFile(t)

	m := initialModel()
	assert.Nil(t, m.filter.project)
}

func TestSaveFilter_PersistsProjectSelection(t *testing.T) {
	withTempStateFile(t)

	project := "home"
	msg := saveFilter(filterState{project: &project})()
	assert.Nil(t, msg, "saveFilter should not emit a message on success")

	got := config.Load()
	require.NotNil(t, got.Project)
	assert.Equal(t, "home", *got.Project)
}
