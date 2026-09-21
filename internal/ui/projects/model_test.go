package projects

import (
	"strings"
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

func TestModel_ViewShowsCountSuffix(t *testing.T) {
	m := New().SetProjects([]string{"chores", "work"}).SetCounts(Counts{
		All:       10,
		None:      2,
		ByProject: map[string]int{"chores": 4, "work": 6},
	})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 10})

	view := m.View()
	for _, want := range []string{"(all)", "(10)", "(none)", "(2)", "chores", "(4)", "work", "(6)"} {
		assert.Contains(t, view, want)
	}
}

func TestModel_ViewAlignsCountFieldsAtFixedWidth(t *testing.T) {
	// Entries with very different name lengths, and counts with different
	// digit counts, should still have their "(N)" fields end at the same
	// fixed-width column, so the eye can scan a straight column of counts.
	m := New().SetProjects([]string{"a", "much-longer-project-name"}).SetCounts(Counts{
		All:       10,
		None:      0,
		ByProject: map[string]int{"a": 1, "much-longer-project-name": 2345},
	})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})

	lines := strings.Split(stripANSI(m.View()), "\n")
	var ends []int
	for _, line := range lines {
		if idx := strings.LastIndex(line, ")"); idx != -1 {
			ends = append(ends, idx)
		}
	}
	require.NotEmpty(t, ends)
	for _, e := range ends[1:] {
		assert.Equal(t, ends[0], e, "all rows should end their (N) count field at the same column")
	}
}

// stripANSI removes SGR escape sequences (e.g. the selected-row reverse
// style) so string-position assertions aren't thrown off by invisible
// control characters.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
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
