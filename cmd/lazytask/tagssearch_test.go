package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tkilb/lazytask/internal/ui/tags"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
)

func tagsFocusedModel(names ...string) model {
	m := model{list: tasklist.New(searchTestTasks()), focus: focusTags, reader: &stubReader{}}
	m.tags = tags.New().SetTags(names)
	return m
}

func TestTagsPanel_SlashStartsTagsScopedSearch(t *testing.T) {
	m := keyOf(tagsFocusedModel("a", "b"), "/")
	assert.True(t, m.searching)
	assert.Equal(t, searchScopeTags, m.searchScope)
}

func TestTagsPanel_SearchEnterJumpsToMatchAndAppliesFilter(t *testing.T) {
	m := tagsFocusedModel("alpha", "bravo", "charlie")
	m.searching = true
	m.searchScope = searchScopeTags
	m.searchInput.SetValue("bravo")

	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(model)
	assert.False(t, m.searching)
	label, ok := m.tags.Selected()
	require.True(t, ok)
	assert.Equal(t, "bravo", label)
	require.NotNil(t, m.filter.tag)
	assert.Equal(t, "bravo", *m.filter.tag)
	assert.NotNil(t, cmd, "refetches tasks for the new tag filter")
}

func TestTagsPanel_SearchNoMatchLeavesFilterAlone(t *testing.T) {
	m := tagsFocusedModel("alpha", "bravo")
	m.searching = true
	m.searchScope = searchScopeTags
	m.searchInput.SetValue("zzz")

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(model)
	assert.Nil(t, m.filter.tag)
}

func TestTagsPanel_SearchEscClearsQuery(t *testing.T) {
	m := tagsFocusedModel("alpha", "bravo")
	m.searching = true
	m.searchScope = searchScopeTags
	m.tags, _ = m.tags.Search("alp")

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, nm.(model).tags.HasActiveSearch())
}

func TestTagsPanel_NextPrevMatchMoveFilter(t *testing.T) {
	m := tagsFocusedModel("alpha-1", "bravo", "alpha-2")
	m.tags, _ = m.tags.Search("alpha")

	m = keyOf(m, "n")
	label, _ := m.tags.Selected()
	assert.Equal(t, "alpha-2", label)
	require.NotNil(t, m.filter.tag)
	assert.Equal(t, "alpha-2", *m.filter.tag)

	m = keyOf(m, "N")
	label, _ = m.tags.Selected()
	assert.Equal(t, "alpha-1", label)
	assert.Equal(t, "alpha-1", *m.filter.tag)
}

func TestTagsPanel_EscClearsSearchWhenIdle(t *testing.T) {
	m := tagsFocusedModel("alpha", "bravo")
	m.tags, _ = m.tags.Search("alp")
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, nm.(model).tags.HasActiveSearch())
}

func TestTagsPanel_SpaceJumpsToTasksWithoutChangingFilter(t *testing.T) {
	m := tagsFocusedModel("alpha", "bravo")
	m = keyOf(m, "j") // (any) -> alpha, filter follows
	before := m.filter
	m = keyOf(m, " ")
	assert.Equal(t, focusTasks, m.focus)
	assert.True(t, before.equal(m.filter))
}

func TestHelp_TagsPanelListsSpaceAndSearch(t *testing.T) {
	keys := map[string]bool{}
	for _, b := range tagsFocusedModel("a").helpLocalBindings() {
		keys[b.Key] = true
	}
	assert.True(t, keys["space"])
	assert.True(t, keys["/"])
	assert.True(t, keys["n/N"])
}
