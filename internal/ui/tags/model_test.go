package tags

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModel_EntriesPinAnyAtTop(t *testing.T) {
	m := New().SetTags([]string{"home", "work"})

	label, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, AnyLabel, label, "(any) is pinned to the top of the list")
}

func TestModel_NavigationReachesAllEntriesInOrder(t *testing.T) {
	m := New().SetTags([]string{"home", "work"})

	// (any) -> home -> work
	wantOrder := []string{AnyLabel, "home", "work"}
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

func TestModel_SelectedWithNoTags(t *testing.T) {
	m := New()

	label, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, AnyLabel, label, "the special entry is always present")
}

func TestModel_ViewShowsCountSuffix(t *testing.T) {
	m := New().SetTags([]string{"errand", "work"}).SetCounts(Counts{
		All:   10,
		ByTag: map[string]int{"errand": 4, "work": 6},
	})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 10})

	view := m.View()
	for _, want := range []string{"(any)", "(10)", "errand", "(4)", "work", "(6)"} {
		assert.Contains(t, view, want)
	}
}

func TestModel_ViewAlignsCountFieldsAtFixedWidth(t *testing.T) {
	m := New().SetTags([]string{"a", "much-longer-tag-name"}).SetCounts(Counts{
		All:   10,
		ByTag: map[string]int{"a": 1, "much-longer-tag-name": 2345},
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

func TestModel_SetTagsClampsCursor(t *testing.T) {
	m := New().SetTags([]string{"a", "b", "c"})
	for i := 0; i < 4; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}
	label, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, "c", label, "cursor should land on the last real tag entry")

	// Shrinking the tag list should clamp the cursor back within range
	// rather than leaving it pointing past the end.
	m = m.SetTags([]string{"a"})
	label, ok = m.Selected()
	require.True(t, ok)
	assert.Equal(t, "a", label)
}

func TestModel_SelectLabelMovesCursorToMatchingEntry(t *testing.T) {
	m := New().SetTags([]string{"home", "work"})

	m = m.SelectLabel("work")
	label, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, "work", label)

	m = m.SelectLabel(AnyLabel)
	label, ok = m.Selected()
	require.True(t, ok)
	assert.Equal(t, AnyLabel, label)
}

func TestModel_SelectLabelLeavesCursorUnchangedWhenLabelMissing(t *testing.T) {
	m := New().SetTags([]string{"home", "work"}).SelectLabel("work")

	m = m.SelectLabel("no-such-tag")
	label, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, "work", label, "cursor should stay put when the requested label isn't a current entry")
}

func TestModel_HasLabel(t *testing.T) {
	m := New().SetTags([]string{"home", "work"})

	assert.True(t, m.HasLabel(AnyLabel))
	assert.True(t, m.HasLabel("home"))
	assert.True(t, m.HasLabel("work"))
	assert.False(t, m.HasLabel("no-such-tag"), "a stale/removed tag name should not be reported as a current entry")
}

func TestModel_View_ShowsPositionFooter(t *testing.T) {
	m := New().SetTags([]string{"home", "work"})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 10})

	view := m.View()
	// (any), home, work -> 3 entries, cursor starts at (any) = 1 of 3.
	assert.Contains(t, view, "1 of 3")
}

func manyTags(n int) []string {
	names := make([]string, n)
	for i := 0; i < n; i++ {
		names[i] = fmt.Sprintf("tag-%02d", i)
	}
	return names
}

func TestModel_View_ScrollsToKeepCursorVisible(t *testing.T) {
	// Height 8 gives 6 inner rows (InnerSize subtracts 2 for borders); with
	// 21 entries (20 tags + 1 special), only 6 are visible at once.
	m := New().SetTags(manyTags(20))
	m, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 8})

	view := m.View()
	assert.Contains(t, view, AnyLabel)
	assert.NotContains(t, view, "tag-19")

	for i := 0; i < 20; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}

	view = m.View()
	assert.Contains(t, view, "tag-19")
	assert.NotContains(t, view, AnyLabel)
	assert.Contains(t, view, "21 of 21")
}

func TestModel_View_UnfocusedCentersCursorFocusedDoesNot(t *testing.T) {
	// Height 7 gives 5 inner rows. Entry 10 is "tag-09" ((any) is entry 0).
	m := New().SetTags(manyTags(20))
	m, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 7})
	for i := 0; i < 10; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}

	unfocused := m.View()
	assert.Contains(t, unfocused, "tag-07")
	assert.Contains(t, unfocused, "tag-11", "rows below the cursor are visible")
	assert.NotContains(t, unfocused, "tag-06")
	assert.NotContains(t, unfocused, "tag-12")

	focused := m.SetFocused(true).View()
	assert.Contains(t, focused, "tag-05")
	assert.Contains(t, focused, "tag-09")
	assert.NotContains(t, focused, "tag-10", "focused keeps the cursor on the last row")
}

func TestModel_SearchJumpsToFirstMatchAtOrAfterCursor(t *testing.T) {
	m := New().SetTags([]string{"home", "work-alpha", "work-beta"})

	m, ok := m.Search("work")
	require.True(t, ok)
	label, _ := m.Selected()
	assert.Equal(t, "work-alpha", label)
	assert.True(t, m.HasActiveSearch())
}

func TestModel_SearchIsCaseInsensitiveAndMatchesSpecialEntry(t *testing.T) {
	m := New().SetTags([]string{"home", "Work"})
	m, ok := m.Search("WORK")
	require.True(t, ok)
	label, _ := m.Selected()
	assert.Equal(t, "Work", label)

	m, ok = m.Search("any")
	require.True(t, ok, "wraps around to the (any) entry")
	label, _ = m.Selected()
	assert.Equal(t, AnyLabel, label)
}

func TestModel_SearchNoMatchKeepsCursorAndRecordsQuery(t *testing.T) {
	m := New().SetTags([]string{"home", "work"})
	m, ok := m.Search("zzz")
	assert.False(t, ok)
	label, _ := m.Selected()
	assert.Equal(t, AnyLabel, label)
	assert.True(t, m.HasActiveSearch())
}

func TestModel_NextPrevMatchWrapAndNoopWithoutQuery(t *testing.T) {
	m := New().SetTags([]string{"a-1", "b", "a-2"})
	_, ok := m.NextMatch()
	assert.False(t, ok, "no active query")
	_, ok = m.PrevMatch()
	assert.False(t, ok, "no active query")

	m, _ = m.Search("a-")
	label, _ := m.Selected()
	assert.Equal(t, "a-1", label)
	m, ok = m.NextMatch()
	require.True(t, ok)
	label, _ = m.Selected()
	assert.Equal(t, "a-2", label)
	m, _ = m.NextMatch()
	label, _ = m.Selected()
	assert.Equal(t, "a-1", label, "wraps forward")
	m, _ = m.PrevMatch()
	label, _ = m.Selected()
	assert.Equal(t, "a-2", label, "wraps backward")
}

func TestModel_ClearSearchKeepsCursor(t *testing.T) {
	m := New().SetTags([]string{"a", "b"})
	m, _ = m.Search("b")
	m = m.ClearSearch()
	assert.False(t, m.HasActiveSearch())
	label, _ := m.Selected()
	assert.Equal(t, "b", label)
	_, ok := m.NextMatch()
	assert.False(t, ok)
}

func TestModel_View_HighlightsSearchMatch(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256) // tests run without a TTY, which strips all styling
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	m := New().SetTags([]string{"alpha", "beta"})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 8})
	m, _ = m.Search("alp")
	highlighted := m.View()
	plain := m.ClearSearch().View() // same cursor, no highlight
	assert.NotEqual(t, plain, highlighted, "match is styled")
	assert.Equal(t, ansi.Strip(plain), ansi.Strip(highlighted), "same text either way")
}
