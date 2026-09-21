package projects

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModel_EntriesPinSpecialsAtTop(t *testing.T) {
	m := New().SetProjects([]string{"home", "work"})

	label, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, AllLabel, label, "(all)/(none) are pinned to the top of the list, (all) first")
}

func TestModel_NavigationReachesAllEntriesInOrder(t *testing.T) {
	m := New().SetProjects([]string{"home", "work"})

	// * (all) -> * (none) -> home -> work
	wantOrder := []string{AllLabel, NoneLabel, "home", "work"}
	for i, want := range wantOrder {
		label, ok := m.Selected()
		require.True(t, ok)
		assert.Equal(t, want, label, "entry %d", i)
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}

	// Cursor should not move past the last entry.
	label, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, "work", label)
}

func TestModel_SelectedWithNoProjects(t *testing.T) {
	m := New()

	label, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, AllLabel, label, "the special entries are always present")
}

func TestModel_SetProjectsClampsCursor(t *testing.T) {
	m := New().SetProjects([]string{"a", "b", "c"})
	for i := 0; i < 4; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}
	label, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, "c", label, "cursor should land on the last real project entry")

	// Shrinking the project list should clamp the cursor back within range
	// rather than leaving it pointing past the end.
	m = m.SetProjects([]string{"a"})
	label, ok = m.Selected()
	require.True(t, ok)
	assert.Equal(t, "a", label)
}
