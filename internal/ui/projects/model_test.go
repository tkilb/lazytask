package projects

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
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

func TestModel_ReassignModeSwapsSpecialEntries(t *testing.T) {
	m := New().SetProjects([]string{"home", "work"}).SetReassignMode(true)

	wantOrder := []string{NoneLabel, NewProjectLabel, "home", "work"}
	for i, want := range wantOrder {
		label, ok := m.Selected()
		require.True(t, ok)
		assert.Equal(t, want, label, "entry %d", i)
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}
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

func TestModel_SelectLabelMovesCursorToMatchingEntry(t *testing.T) {
	m := New().SetProjects([]string{"home", "work"})

	m = m.SelectLabel("work")
	label, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, "work", label)

	m = m.SelectLabel(NoneLabel)
	label, ok = m.Selected()
	require.True(t, ok)
	assert.Equal(t, NoneLabel, label)
}

func TestModel_SelectLabelLeavesCursorUnchangedWhenLabelMissing(t *testing.T) {
	m := New().SetProjects([]string{"home", "work"}).SelectLabel("work")

	m = m.SelectLabel("no-such-project")
	label, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, "work", label, "cursor should stay put when the requested label isn't a current entry")
}

func TestModel_HasLabel(t *testing.T) {
	m := New().SetProjects([]string{"home", "work"})

	assert.True(t, m.HasLabel(AllLabel))
	assert.True(t, m.HasLabel(NoneLabel))
	assert.True(t, m.HasLabel("home"))
	assert.True(t, m.HasLabel("work"))
	assert.False(t, m.HasLabel("no-such-project"), "a stale/removed project name should not be reported as a current entry")
}

func TestModel_View_ShowsPositionFooter(t *testing.T) {
	m := New().SetProjects([]string{"home", "work"})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 10})

	view := m.View()
	// (all), (none), home, work -> 4 entries, cursor starts at (all) = 1 of 4.
	assert.Contains(t, view, "1 of 4")
}

func manyProjects(n int) []string {
	names := make([]string, n)
	for i := 0; i < n; i++ {
		names[i] = fmt.Sprintf("project-%02d", i)
	}
	return names
}

func TestModel_View_ScrollsToKeepCursorVisible(t *testing.T) {
	// Height 8 gives 6 inner rows (InnerSize subtracts 2 for borders); with
	// 22 entries (20 projects + 2 special), only 6 are visible at once.
	m := New().SetProjects(manyProjects(20))
	m, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 8})

	view := m.View()
	assert.Contains(t, view, AllLabel)
	assert.NotContains(t, view, "project-19")

	for i := 0; i < 21; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}

	view = m.View()
	assert.Contains(t, view, "project-19")
	assert.NotContains(t, view, AllLabel)
	assert.Contains(t, view, "22 of 22")
}

func TestModel_SearchJumpsToFirstMatch(t *testing.T) {
	m := New().SetProjects([]string{"home", "work-alpha", "work-beta"})

	m, ok := m.Search("work")
	require.True(t, ok)
	label, _ := m.Selected()
	assert.Equal(t, "work-alpha", label)
	assert.True(t, m.HasActiveSearch())
}

func TestModel_SearchNoMatchIsNoOpButRecordsQuery(t *testing.T) {
	m := New().SetProjects([]string{"home", "work"})

	m, ok := m.Search("nonexistent")
	assert.False(t, ok)
	label, _ := m.Selected()
	assert.Equal(t, AllLabel, label, "cursor stays put on a no-match search")
	assert.True(t, m.HasActiveSearch())
}

func TestModel_NextMatchAndPrevMatchWrap(t *testing.T) {
	m := New().SetProjects([]string{"alpha-1", "beta", "alpha-2"})
	m, _ = m.Search("alpha")

	label, _ := m.Selected()
	assert.Equal(t, "alpha-1", label)

	m, ok := m.NextMatch()
	require.True(t, ok)
	label, _ = m.Selected()
	assert.Equal(t, "alpha-2", label)

	// Wraps back around to the first match.
	m, ok = m.NextMatch()
	require.True(t, ok)
	label, _ = m.Selected()
	assert.Equal(t, "alpha-1", label)

	m, ok = m.PrevMatch()
	require.True(t, ok)
	label, _ = m.Selected()
	assert.Equal(t, "alpha-2", label)
}

func TestModel_NextPrevMatchNoOpWithoutActiveSearch(t *testing.T) {
	m := New().SetProjects([]string{"home", "work"})

	_, ok := m.NextMatch()
	assert.False(t, ok)
	_, ok = m.PrevMatch()
	assert.False(t, ok)
}

func TestModel_ClearSearchStopsHighlightingAndMatching(t *testing.T) {
	m := New().SetProjects([]string{"home", "work"})
	m, _ = m.Search("work")
	require.True(t, m.HasActiveSearch())

	m = m.ClearSearch()
	assert.False(t, m.HasActiveSearch())
	_, ok := m.NextMatch()
	assert.False(t, ok)
}

func TestModel_View_HighlightsSearchMatch(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := New().SetProjects([]string{"home", "work"})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 8})
	m, _ = m.Search("work")

	view := m.View()
	matchStyle := lipgloss.NewStyle().Background(searchMatchColor).Foreground(searchMatchTextColor).Bold(false)
	assert.Contains(t, view, matchStyle.Render("work"))
}
