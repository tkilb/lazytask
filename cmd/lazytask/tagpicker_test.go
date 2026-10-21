package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tkilb/lazytask/internal/ui/tags"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
)

func tagPickerTasksModel(names ...string) model {
	return model{
		list:   tasklist.New(searchTestTasks()),
		focus:  focusTasks,
		reader: &stubReader{},
		tags:   tags.New().SetTags(names),
	}
}

func TestTKey_OpensTagPickerOnCurrentSelection(t *testing.T) {
	m := tagPickerTasksModel("work", "home")
	m.tags = m.tags.SelectLabel("home")

	m = keyOf(m, "t")
	assert.True(t, m.pickingTag)
	label, ok := m.tagPicker.Selected()
	require.True(t, ok)
	assert.Equal(t, "home", label, "opens on the tag currently filtered")
}

func TestTKey_OnlyOpensFromTasksPanel(t *testing.T) {
	m := tagPickerTasksModel("work")
	m.focus = focusProjects
	m = keyOf(m, "t")
	assert.False(t, m.pickingTag)
}

func TestTagPicker_EscCancelsWithoutChangingFilter(t *testing.T) {
	m := keyOf(tagPickerTasksModel("work", "home"), "t")
	m = keyOf(m, "j")

	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = nm.(model)
	assert.False(t, m.pickingTag)
	assert.Nil(t, cmd)
	assert.Nil(t, m.filter.tag)
}

func TestTagPicker_MovingDoesNotApplyFilterUntilEnter(t *testing.T) {
	m := keyOf(tagPickerTasksModel("work", "home"), "t")
	m = keyOf(m, "j")
	assert.Nil(t, m.filter.tag)
	label, _ := m.tagPicker.Selected()
	assert.Equal(t, "work", label)
}

func TestTagPicker_EnterAppliesFilterAndSyncsPanel(t *testing.T) {
	r := &stubReader{}
	m := tagPickerTasksModel("work", "home")
	m.reader = r
	m = keyOf(keyOf(m, "t"), "j") // (any) -> first tag (sorted by the caller: "work")

	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(model)
	assert.False(t, m.pickingTag)
	require.NotNil(t, cmd)
	require.NotNil(t, m.filter.tag)
	assert.Equal(t, "work", *m.filter.tag)
	label, _ := m.tags.Selected()
	assert.Equal(t, "work", label, "the Tags panel follows the popup's choice")
}

func TestTagPicker_EnterOnAnyClearsActiveTagFilter(t *testing.T) {
	m := tagPickerTasksModel("work")
	work := "work"
	m.filter.tag = &work
	m.tags = m.tags.SelectLabel("work")

	m = keyOf(m, "t")
	m = keyOf(m, "k") // up to (any)
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(model)
	assert.Nil(t, m.filter.tag)
	assert.NotNil(t, cmd, "tasks are refetched without the tag")
}

func TestTagPicker_EnterOnSameFilterIsNoOp(t *testing.T) {
	m := keyOf(tagPickerTasksModel("work"), "t")
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(model)
	assert.False(t, m.pickingTag)
	assert.Nil(t, cmd)
	assert.Nil(t, m.filter.tag)
}

func TestTagPicker_SearchScopedToPickerAndKeepsPopupOpen(t *testing.T) {
	m := keyOf(tagPickerTasksModel("alpha", "bravo"), "t")

	m = keyOf(m, "/")
	assert.True(t, m.searching)
	assert.Equal(t, searchScopeTagPicker, m.searchScope)

	m.searchInput.SetValue("bravo")
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(model)
	assert.False(t, m.searching)
	assert.True(t, m.pickingTag, "committing a search must not close the popup")
	assert.Nil(t, m.filter.tag, "nor apply the filter")
	label, _ := m.tagPicker.Selected()
	assert.Equal(t, "bravo", label)
}

func TestTagPicker_EscClearsSearchBeforeClosing(t *testing.T) {
	m := keyOf(tagPickerTasksModel("alpha", "bravo"), "t")
	m.tagPicker, _ = m.tagPicker.Search("bravo")

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = nm.(model)
	assert.True(t, m.pickingTag, "first esc only clears the search")
	assert.False(t, m.tagPicker.HasActiveSearch())

	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, nm.(model).pickingTag)
}

func TestTagPicker_NextPrevMatch(t *testing.T) {
	m := keyOf(tagPickerTasksModel("alpha-1", "bravo", "alpha-2"), "t")
	m.tagPicker, _ = m.tagPicker.Search("alpha")

	m = keyOf(m, "n")
	label, _ := m.tagPicker.Selected()
	assert.Equal(t, "alpha-2", label)
	m = keyOf(m, "N")
	label, _ = m.tagPicker.Selected()
	assert.Equal(t, "alpha-1", label)
}

func TestTagPicker_ViewShowsTitledPopup(t *testing.T) {
	m := keyOf(tagPickerTasksModel("work"), "t")
	assert.Contains(t, m.View(), "Tag Filter")
	m.pickingTag = false
	assert.NotContains(t, m.View(), "Tag Filter")
}

func TestHelp_TasksPanelListsTKey(t *testing.T) {
	found := false
	for _, b := range tagPickerTasksModel("work").helpLocalBindings() {
		if b.Key == "t" {
			found = true
		}
	}
	assert.True(t, found)
}
