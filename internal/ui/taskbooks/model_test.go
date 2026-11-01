package taskbooks

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelEntries(t *testing.T) {
	m := New().SetTaskbooks([]string{"personal", "work"})
	assert.True(t, m.HasLabel(AllLabel))
	assert.False(t, m.HasLabel(NoneLabel))
	assert.True(t, m.HasLabel("work"))
	assert.True(t, m.HasLabel("personal"))

	sel, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, AllLabel, sel)

	m = m.SelectLabel("work")
	sel, ok = m.Selected()
	require.True(t, ok)
	assert.Equal(t, "work", sel)
}

func TestModelReassignMode(t *testing.T) {
	m := New().SetTaskbooks([]string{"work"}).SetReassignMode(true)
	assert.False(t, m.HasLabel(AllLabel))
	assert.True(t, m.HasLabel(NoneLabel))
	assert.True(t, m.HasLabel(NewTaskbookLabel))
	assert.True(t, m.HasLabel("work"))
}

func TestModelNavigation(t *testing.T) {
	m := New().SetTaskbooks([]string{"work", "personal"})
	// Initial is AllLabel
	sel, _ := m.Selected()
	assert.Equal(t, AllLabel, sel)

	// Down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	sel, _ = m.Selected()
	assert.Equal(t, "work", sel)

	// Down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	sel, _ = m.Selected()
	assert.Equal(t, "personal", sel)

	// Up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	sel, _ = m.Selected()
	assert.Equal(t, "work", sel)
}

func TestModelCounts(t *testing.T) {
	counts := Counts{
		All:        5,
		Unscoped:   2,
		ByTaskbook: map[string]int{"work": 3},
	}
	m := New().SetTaskbooks([]string{"work"}).SetCounts(counts)

	// AllLabel shows All count (5)
	assert.Equal(t, 5, m.count(AllLabel))
	// "work" shows scoped + unscoped (3 + 2 = 5)
	assert.Equal(t, 5, m.count("work"))
	// NoneLabel shows unscoped (2)
	assert.Equal(t, 2, m.count(NoneLabel))
}

func TestModelSearch(t *testing.T) {
	m := New().SetTaskbooks([]string{"finances", "work"})
	m, found := m.Search("fin")
	require.True(t, found)
	sel, _ := m.Selected()
	assert.Equal(t, "finances", sel)

	m = m.ClearSearch()
	assert.False(t, m.HasActiveSearch())
}
