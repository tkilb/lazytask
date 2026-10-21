package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/projects"
	"github.com/tkilb/lazytask/internal/ui/tags"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
)

// stubToggler records start/stop and add/remove-tag calls as "verb:id[:tag]".
type stubToggler struct {
	calls []string
	err   error
}

func (s *stubToggler) Start(_ context.Context, id string) error {
	s.calls = append(s.calls, "start:"+id)
	return s.err
}
func (s *stubToggler) Stop(_ context.Context, id string) error {
	s.calls = append(s.calls, "stop:"+id)
	return s.err
}
func (s *stubToggler) AddTag(_ context.Context, id, tag string) error {
	s.calls = append(s.calls, "add:"+id+":"+tag)
	return s.err
}
func (s *stubToggler) RemoveTag(_ context.Context, id, tag string) error {
	s.calls = append(s.calls, "remove:"+id+":"+tag)
	return s.err
}

func (s *stubToggler) ModifyTags(_ context.Context, id string, add, remove []string) error {
	s.calls = append(s.calls, fmt.Sprintf("modify:%s:+%v:-%v", id, add, remove))
	return s.err
}

func runeKey(r string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(r)} }

func togglerModel(st *stubToggler, tasks ...taskwarrior.Task) model {
	return model{list: tasklist.New(tasks), starter: st, tagger: st, projects: projects.New(), tags: tags.New()}
}

func TestKeyS_TogglesStartStopWithUndo(t *testing.T) {
	st := &stubToggler{}
	m := togglerModel(st, taskwarrior.Task{ID: 1, UUID: "u1", Status: "pending"})

	_, cmd := m.Update(runeKey("s"))
	require.NotNil(t, cmd)
	msg, ok := cmd().(taskToggledMsg)
	require.True(t, ok)
	assert.Equal(t, []string{"start:u1"}, st.calls)
	require.NoError(t, msg.action.Undo())
	require.NoError(t, msg.action.Redo())
	assert.Equal(t, []string{"start:u1", "stop:u1", "start:u1"}, st.calls)

	st.calls = nil
	m = togglerModel(st, taskwarrior.Task{ID: 1, UUID: "u1", Status: "pending", Start: "20240101T000000Z"})
	_, cmd = m.Update(runeKey("s"))
	_, ok = cmd().(taskToggledMsg)
	require.True(t, ok)
	assert.Equal(t, []string{"stop:u1"}, st.calls)
}

func TestKeyN_TogglesNextTagUnlessSearching(t *testing.T) {
	st := &stubToggler{}
	task := taskwarrior.Task{ID: 1, UUID: "u1", Status: "pending", Description: "alpha"}
	m := togglerModel(st, task)

	_, cmd := m.Update(runeKey("n"))
	msg, ok := cmd().(taskToggledMsg)
	require.True(t, ok)
	assert.Equal(t, []string{"add:u1:next"}, st.calls)
	require.NoError(t, msg.action.Undo())
	assert.Equal(t, "remove:u1:next", st.calls[1])

	st.calls = nil
	task.Tags = []string{"next"}
	_, cmd = togglerModel(st, task).Update(runeKey("n"))
	cmd()
	assert.Equal(t, []string{"remove:u1:next"}, st.calls)

	// While a search is active, n keeps meaning "next match".
	st.calls = nil
	searching := togglerModel(st, task)
	searching.list, _ = searching.list.Search("alpha")
	_, cmd = searching.Update(runeKey("n"))
	assert.Nil(t, cmd)
	assert.Empty(t, st.calls)
}

func TestKeysSN_OnlyOnTodoTab(t *testing.T) {
	st := &stubToggler{}
	m := togglerModel(st, taskwarrior.Task{ID: 1, UUID: "u1", Status: "completed"})
	m.list = m.list.NextStatus() // Done
	for _, k := range []string{"s", "n"} {
		_, cmd := m.Update(runeKey(k))
		assert.Nil(t, cmd, k)
	}
}

func TestToggleError_ShowsPopupAndPushesNoUndo(t *testing.T) {
	st := &stubToggler{err: errors.New("boom")}
	m := togglerModel(st, taskwarrior.Task{ID: 1, UUID: "u1", Status: "pending"})
	_, cmd := m.Update(runeKey("s"))
	newModel, _ := m.Update(cmd())
	mm := newModel.(model)
	assert.True(t, mm.popups.Active())
	assert.False(t, mm.undo.CanUndo())
}

func TestToggledMsg_PushesUndoAndRefocuses(t *testing.T) {
	m := togglerModel(&stubToggler{})
	m.reader = &stubReader{}
	st := &stubToggler{}
	_, cmd := togglerModel(st, taskwarrior.Task{ID: 4, UUID: "u4", Status: "pending"}).Update(runeKey("s"))
	newModel, refresh := m.Update(cmd())
	mm := newModel.(model)
	assert.True(t, mm.undo.CanUndo())
	assert.Equal(t, 4, mm.pendingFocusID)
	assert.NotNil(t, refresh)
}

func TestKeyF_FocusesSuggestedTaskAndProject(t *testing.T) {
	reader := &stubReader{}
	m := togglerModel(&stubToggler{},
		taskwarrior.Task{ID: 1, UUID: "u1", Status: "pending", Project: "A"},
		taskwarrior.Task{ID: 2, UUID: "u2", Status: "pending", Project: "B"},
	)
	m.reader = reader
	m.focus = focusTags
	m.list = m.list.NextStatus() // Done tab
	m.suggestedTasks = []taskwarrior.Task{
		{ID: 1, Status: "pending", Project: "A", Urgency: 1},
		{ID: 2, Status: "pending", Project: "B", Start: "20240101T000000Z"},
	}

	newModel, cmd := m.Update(runeKey("f"))
	mm := newModel.(model)
	assert.Equal(t, focusTasks, mm.focus)
	assert.Equal(t, tasklist.TabTodo, mm.list.Status())
	require.NotNil(t, mm.filter.project)
	assert.Equal(t, "B", *mm.filter.project)
	assert.Equal(t, 2, mm.pendingFocusID)
	assert.NotNil(t, cmd)
}

func TestKeyF_OrphanClearsProjectFilterAndDropsHidingTag(t *testing.T) {
	proj, tag := "A", "work"
	m := togglerModel(&stubToggler{})
	m.reader = &stubReader{}
	m.filter = filterState{project: &proj, tag: &tag}
	m.suggestedTasks = []taskwarrior.Task{{ID: 9, Status: "pending"}} // orphan, no tags

	newModel, _ := m.Update(runeKey("f"))
	mm := newModel.(model)
	assert.Nil(t, mm.filter.project)
	assert.Nil(t, mm.filter.tag)
	assert.Equal(t, 9, mm.pendingFocusID)
}

func TestKeyF_NoSuggestionIsNoOp(t *testing.T) {
	m := togglerModel(&stubToggler{})
	m.focus = focusProjects
	newModel, cmd := m.Update(runeKey("f"))
	assert.Nil(t, cmd)
	assert.Equal(t, focusProjects, newModel.(model).focus)
}

func TestHelp_DocumentsSNAndF(t *testing.T) {
	m := togglerModel(&stubToggler{}, taskwarrior.Task{ID: 1, Status: "pending"})
	var keys []string
	for _, sec := range m.helpSections() {
		for _, b := range sec.Bindings {
			keys = append(keys, b.Key)
		}
	}
	for _, want := range []string{"s", "n", "f"} {
		assert.Contains(t, keys, want)
	}
}
