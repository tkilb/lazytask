package main

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/projects"
	"github.com/tkilb/lazytask/internal/ui/tags"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
)

type modifyCall struct {
	id          string
	add, remove []string
}

type stubTagModifier struct {
	calls []modifyCall
	err   error
}

func (s *stubTagModifier) AddTag(context.Context, string, string) error    { return s.err }
func (s *stubTagModifier) RemoveTag(context.Context, string, string) error { return s.err }
func (s *stubTagModifier) ModifyTags(_ context.Context, id string, add, remove []string) error {
	s.calls = append(s.calls, modifyCall{id: id, add: add, remove: remove})
	return s.err
}

// assignModel returns a model on the Tasks panel whose selected task carries
// taskTags, with knownTags offered in the picker.
func assignModel(st *stubTagModifier, taskTags []string, knownTags ...string) model {
	task := taskwarrior.Task{ID: 1, UUID: "u1", Description: "Write", Status: "pending", Tags: taskTags}
	return model{
		list:     tasklist.New([]taskwarrior.Task{task}),
		focus:    focusTasks,
		reader:   &stubReader{},
		tagger:   st,
		projects: projects.New(),
		tags:     tags.New().SetTags(knownTags),
	}
}

func key(m model, k tea.KeyMsg) (model, tea.Cmd) {
	nm, cmd := m.Update(k)
	return nm.(model), cmd
}

func typeText(m model, s string) model {
	for _, r := range s {
		m, _ = key(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func TestTAssign_OnlyOpensFromTasksPanelWithSelection(t *testing.T) {
	m := assignModel(&stubTagModifier{}, nil, "a")
	m.focus = focusProjects
	assert.False(t, keyOf(m, "T").assigningTags)

	empty := assignModel(&stubTagModifier{}, nil, "a")
	empty.list = tasklist.New(nil)
	assert.False(t, keyOf(empty, "T").assigningTags, "no task selected")

	assert.True(t, keyOf(assignModel(&stubTagModifier{}, nil, "a"), "T").assigningTags)
}

func TestTAssign_OpensOnTypeEntry(t *testing.T) {
	m := keyOf(assignModel(&stubTagModifier{}, nil, "a", "b"), "T")
	label, ok := m.tagAssignPicker.Selected()
	require.True(t, ok)
	assert.Equal(t, tags.TypeLabel, label)
}

func TestTAssign_SpaceTogglesAndEnterAppendsAllChecked(t *testing.T) {
	st := &stubTagModifier{}
	m := keyOf(assignModel(st, nil, "a", "b", "c"), "T")
	m = keyOf(m, "j") // a
	m = keyOf(m, " ")
	m = keyOf(m, "j") // b
	m = keyOf(m, "j") // c
	m = keyOf(m, " ")

	m, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, m.assigningTags)
	require.NotNil(t, cmd)
	msg := cmd()
	require.IsType(t, taskTagsSetMsg{}, msg)
	assert.Equal(t, []modifyCall{{id: "u1", add: []string{"a", "c"}}}, st.calls)
}

func TestTAssign_EnterWithNothingCheckedAppendsHighlighted(t *testing.T) {
	st := &stubTagModifier{}
	m := keyOf(assignModel(st, nil, "a", "b"), "T")
	m = keyOf(keyOf(m, "j"), "j") // b

	_, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	cmd()
	assert.Equal(t, []modifyCall{{id: "u1", add: []string{"b"}}}, st.calls)
}

func TestTAssign_ExistingTagsStartChecked(t *testing.T) {
	m := keyOf(assignModel(&stubTagModifier{}, []string{"a", "c"}, "a", "b", "c"), "T")
	assert.Equal(t, []string{"a", "c"}, m.tagAssignPicker.Checked())
	assert.Contains(t, m.View(), "[x] a")
	assert.Contains(t, m.View(), "[ ] b")
}

func TestTAssign_ExistingTagNotInListIsIgnoredWhenChecking(t *testing.T) {
	m := keyOf(assignModel(&stubTagModifier{}, []string{"gone"}, "a"), "T")
	assert.Empty(t, m.tagAssignPicker.Checked())
}

func TestTAssign_CheckingANewTagKeepsExistingOnes(t *testing.T) {
	st := &stubTagModifier{}
	m := keyOf(assignModel(st, []string{"a"}, "a", "b"), "T")
	m = keyOf(keyOf(m, "j"), "j") // b
	m = keyOf(m, " ")

	_, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	cmd()
	assert.Equal(t, []modifyCall{{id: "u1", add: []string{"b"}}}, st.calls)
}

func TestTAssign_UncheckingAnExistingTagRemovesIt(t *testing.T) {
	st := &stubTagModifier{}
	m := keyOf(assignModel(st, []string{"a", "b"}, "a", "b", "c"), "T")
	m = keyOf(m, "j")             // a
	m = keyOf(m, " ")             // uncheck a
	m = keyOf(keyOf(m, "j"), "j") // c
	m = keyOf(m, " ")             // check c

	_, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	cmd()
	assert.Equal(t, []modifyCall{{id: "u1", add: []string{"c"}, remove: []string{"a"}}}, st.calls)
}

func TestTAssign_TagsNotShownInListSurviveAChange(t *testing.T) {
	st := &stubTagModifier{}
	m := keyOf(assignModel(st, []string{"gone", "a"}, "a", "b"), "T")
	m = keyOf(keyOf(m, "j"), "j") // b
	m = keyOf(m, " ")

	_, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	cmd()
	assert.Equal(t, []modifyCall{{id: "u1", add: []string{"b"}}}, st.calls, "'gone' is not removed")
}

func TestTAssign_UncheckingEverythingClearsListedTags(t *testing.T) {
	st := &stubTagModifier{}
	m := keyOf(assignModel(st, []string{"a"}, "a"), "T")
	m = keyOf(m, "j")
	m = keyOf(m, " ")

	_, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	cmd()
	assert.Equal(t, []modifyCall{{id: "u1", remove: []string{"a"}}}, st.calls)
}

func TestTAssign_NothingToggledAppendsHighlightedEvenWithExistingTags(t *testing.T) {
	st := &stubTagModifier{}
	m := keyOf(assignModel(st, []string{"a"}, "a", "b"), "T")
	m = keyOf(keyOf(m, "j"), "j") // b, nothing toggled

	_, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	cmd()
	assert.Equal(t, []modifyCall{{id: "u1", add: []string{"b"}}}, st.calls)
}

func TestTAssign_AppendingOnlyExistingTagsIsNoOp(t *testing.T) {
	st := &stubTagModifier{}
	m := keyOf(assignModel(st, []string{"a"}, "a"), "T")
	m = keyOf(m, "j") // a

	m, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, m.assigningTags)
	assert.Nil(t, cmd)
	assert.Empty(t, st.calls)
}

func TestTAssign_EscCancelsPicker(t *testing.T) {
	st := &stubTagModifier{}
	m := keyOf(keyOf(assignModel(st, nil, "a"), "T"), "j")
	m, cmd := key(m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, m.assigningTags)
	assert.Nil(t, cmd)
	assert.Empty(t, st.calls)
}

func TestTAssign_EnterOnTypeEntryOpensPrefilledPrompt(t *testing.T) {
	m := keyOf(assignModel(&stubTagModifier{}, []string{"x", "y"}, "a"), "T")
	m, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter}) // on (type tags...)
	assert.False(t, m.assigningTags)
	assert.True(t, m.tagAssignTyping)
	assert.NotNil(t, cmd)
	assert.Equal(t, "x, y", m.tagAssignInput.Value())
}

func openTypedPrompt(st *stubTagModifier, taskTags []string) model {
	m := keyOf(assignModel(st, taskTags, "a"), "T")
	m, _ = key(m, tea.KeyMsg{Type: tea.KeyEnter})
	return m
}

func clearPrompt(m model) model {
	for i := 0; i < 40; i++ {
		m, _ = key(m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	return m
}

func TestTAssign_TypedReplacesTags(t *testing.T) {
	st := &stubTagModifier{}
	m := clearPrompt(openTypedPrompt(st, []string{"x", "y"}))
	m = typeText(m, "y, +z w")

	m, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, m.tagAssignTyping)
	require.NotNil(t, cmd)
	require.IsType(t, taskTagsSetMsg{}, cmd())
	assert.Equal(t, []modifyCall{{id: "u1", add: []string{"z", "w"}, remove: []string{"x"}}}, st.calls)
}

func TestTAssign_TypedEmptyClearsAllTags(t *testing.T) {
	st := &stubTagModifier{}
	m := clearPrompt(openTypedPrompt(st, []string{"x", "y"}))

	_, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	cmd()
	assert.Equal(t, []modifyCall{{id: "u1", remove: []string{"x", "y"}}}, st.calls)
}

func TestTAssign_TypedUnchangedIsNoOp(t *testing.T) {
	st := &stubTagModifier{}
	m := openTypedPrompt(st, []string{"x", "y"})

	m, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, m.tagAssignTyping)
	assert.Nil(t, cmd)
	assert.Empty(t, st.calls)
}

func TestTAssign_TypedInvalidWarnsAndKeepsPrompt(t *testing.T) {
	st := &stubTagModifier{}
	m := clearPrompt(openTypedPrompt(st, []string{"x"}))
	m = typeText(m, "ok -bad")

	m, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, m.tagAssignTyping, "prompt stays open so the input can be fixed")
	assert.Nil(t, cmd)
	assert.Empty(t, st.calls)
	_, hasPopup := m.popups.Current()
	assert.True(t, hasPopup)
}

func TestTAssign_TypedEscCancels(t *testing.T) {
	st := &stubTagModifier{}
	m := openTypedPrompt(st, []string{"x"})
	m, cmd := key(m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, m.tagAssignTyping)
	assert.Nil(t, cmd)
	assert.Empty(t, st.calls)
}

func TestTAssign_ErrorSurfacesAsErrMsg(t *testing.T) {
	st := &stubTagModifier{err: errors.New("boom")}
	m := keyOf(assignModel(st, nil, "a"), "T")
	m = keyOf(m, "j")
	_, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.IsType(t, taskTagsErrMsg{}, cmd())
}

func TestTAssign_SetMsgRecordsUndoAndRefreshesTasksAndTags(t *testing.T) {
	st := &stubTagModifier{}
	reader := &stubReader{}
	m := assignModel(st, []string{"x"}, "a")
	m.reader = reader
	m = clearPrompt(openTypedPrompt(st, []string{"x"}))
	m.reader = reader
	m = typeText(m, "z")
	m, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()

	nm, refresh := m.Update(msg)
	m = nm.(model)
	require.NotNil(t, refresh)
	runBatch(refresh)
	assert.Equal(t, 4, reader.calls, "tasks, projects, tags and taskbooks are refetched")
	assert.Equal(t, 1, m.pendingFocusID, "cursor stays on the edited task")
	assert.True(t, m.undo.CanUndo())

	// Undo reverses the whole change in one step; redo reapplies it.
	st.calls = nil
	act, _ := m.undo.Undo()
	require.NoError(t, act.Undo())
	require.NoError(t, act.Redo())
	assert.Equal(t, []modifyCall{
		{id: "u1", add: []string{"x"}, remove: []string{"z"}},
		{id: "u1", add: []string{"z"}, remove: []string{"x"}},
	}, st.calls)
}

func TestTAssign_ErrMsgShowsPopup(t *testing.T) {
	m := assignModel(&stubTagModifier{}, nil)
	wantErr := errors.New("boom")
	nm, _ := m.Update(taskTagsErrMsg{err: wantErr})
	assertErrPopup(t, nm.(model), wantErr)
}

func TestTAssign_ViewShowsPopupTitle(t *testing.T) {
	m := keyOf(assignModel(&stubTagModifier{}, nil, "a"), "T")
	assert.Contains(t, m.View(), "Assign Tags")
	m, _ = key(m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.NotContains(t, m.View(), "Assign Tags")
}

func TestHelp_TasksPanelListsCapitalTKey(t *testing.T) {
	found := false
	for _, b := range assignModel(&stubTagModifier{}, nil).helpLocalBindings() {
		if b.Key == "T" {
			found = true
		}
	}
	assert.True(t, found)
}
