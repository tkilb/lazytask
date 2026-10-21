package main

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

	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/projects"
	"github.com/tkilb/lazytask/internal/ui/tags"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
)

func TestSuggestedLine_EmptyAndPicked(t *testing.T) {
	m := model{}
	assert.Contains(t, m.suggestedContent(80), "nothing pending")

	m.suggestedTasks = []taskwarrior.Task{
		{Description: "Low", Status: "pending", Project: "A", Urgency: 1},
		{Description: "Doing it", Status: "pending", Project: "B", Start: "20240101T000000Z"},
	}
	line := m.suggestedContent(80)
	assert.Contains(t, line, "started")
	assert.Contains(t, line, "Doing it")
	assert.Contains(t, line, "[B]")
}

func TestView_SuggestedPanelReplacesStatusPanel(t *testing.T) {
	m := model{
		list:           tasklist.New(nil),
		suggestedTasks: []taskwarrior.Task{{Description: "Orphan one", Status: "pending"}},
	}
	view := m.View()
	assert.Contains(t, view, "[1]-Suggested")
	assert.Contains(t, view, "Orphan one")
	assert.NotContains(t, view, "Status─")
}

func TestUpdate_SuggestedTasksLoadedStoresTasks(t *testing.T) {
	m := model{}
	newModel, cmd := m.Update(suggestedTasksLoadedMsg{tasks: []taskwarrior.Task{{Description: "x"}}})
	assert.Nil(t, cmd)
	assert.Len(t, newModel.(model).suggestedTasks, 1)
}

func TestFetchSuggestedTasks(t *testing.T) {
	assert.Nil(t, fetchSuggestedTasks(nil))

	r := &stubReader{tasks: []taskwarrior.Task{{Description: "p"}}}
	msg := fetchSuggestedTasks(r)()
	loaded, ok := msg.(suggestedTasksLoadedMsg)
	require.True(t, ok)
	assert.Len(t, loaded.tasks, 1)
	assert.Equal(t, []string{"status:pending"}, r.lastFilters)

	r.err = assert.AnError
	assert.Nil(t, fetchSuggestedTasks(r)())
}

// TestView_FillsTerminalHeightExactly guards against a trailing blank row
// below the key-hint bar: the view must be exactly as tall as the terminal,
// with the hint line as its last row.
func TestView_FillsTerminalHeightExactly(t *testing.T) {
	nm, _ := model{list: tasklist.New(nil)}.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	lines := strings.Split(nm.(model).View(), "\n")
	assert.Len(t, lines, 40)
	assert.Contains(t, lines[len(lines)-1], "quit")
}

func TestSuggestedContent_ListsUpToFiveWhenFocusedAndSizesPanel(t *testing.T) {
	mk := func(n int) model {
		m := model{focus: focusStatus}
		for i := 0; i < n; i++ {
			m.suggestedTasks = append(m.suggestedTasks, taskwarrior.Task{
				Description: fmt.Sprintf("task%d", i), Status: "pending", Project: "P", Urgency: float64(10 - i),
			})
		}
		return m
	}
	for n, wantRows := range map[int]int{0: 1, 1: 1, 2: 2, 3: 3, 5: 5, 8: 5} {
		m := mk(n)
		assert.Len(t, strings.Split(m.suggestedContent(80), "\n"), wantRows, "n=%d", n)
		assert.Equal(t, wantRows+2, m.suggestedPanelHeight(), "n=%d", n)
	}
	assert.Contains(t, mk(8).suggestedContent(80), "task4")
	assert.NotContains(t, mk(8).suggestedContent(80), "task5")

	unfocused := mk(8)
	unfocused.focus = focusTasks
	assert.Len(t, strings.Split(unfocused.suggestedContent(80), "\n"), 1, "collapses to one row unfocused")
	assert.Equal(t, 3, unfocused.suggestedPanelHeight())
	assert.Contains(t, unfocused.suggestedContent(80), "task0")
}

func TestView_SuggestedPanelGrowsWithSuggestions(t *testing.T) {
	m := model{list: tasklist.New(nil), focus: focusStatus, suggestedTasks: []taskwarrior.Task{
		{Description: "one", Status: "pending", Project: "P"},
		{Description: "two", Status: "pending", Project: "P"},
	}}
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := nm.(model).View()
	assert.Contains(t, view, "one")
	assert.Contains(t, view, "two")
	assert.Len(t, strings.Split(view, "\n"), 40)
}

// TestView_LongSuggestionsDoNotOverflowTerminal guards the regression where
// a long description wrapped inside panel 1, making the grid taller than
// the terminal and scrolling its top border off-screen.
func TestView_LongSuggestionsDoNotOverflowTerminal(t *testing.T) {
	long := strings.Repeat("a long description ", 10) + "line\nbreak"
	m := model{list: tasklist.New(nil)}
	for i := 0; i < 3; i++ {
		m.suggestedTasks = append(m.suggestedTasks, taskwarrior.Task{Description: long, Status: "pending", Project: "Project"})
	}
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	lines := strings.Split(nm.(model).View(), "\n")
	assert.Len(t, lines, 30)
	assert.Contains(t, lines[0], "[1]-Suggested")
}

// TestView_PanelsResizeWhenSuggestionsLoadAfterWindowSize guards the launch
// bug: suggestions arrive after the first WindowSizeMsg, changing panel 1's
// height, and the other panels must be re-sized or the grid overflows the
// terminal until the user resizes.
func TestView_PanelsResizeWhenSuggestionsLoadAfterWindowSize(t *testing.T) {
	m := model{list: tasklist.New(nil), projects: projects.New(), tags: tags.New()}
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	nm, _ = nm.Update(suggestedTasksLoadedMsg{tasks: []taskwarrior.Task{
		{Description: "one", Status: "pending", Project: "P"},
		{Description: "two", Status: "pending", Project: "P"},
		{Description: "three", Status: "pending", Project: "P"},
	}})
	lines := strings.Split(nm.(model).View(), "\n")
	assert.Len(t, lines, 30)
	assert.Contains(t, lines[0], "[1]-Suggested")
}

func suggestPanelModel(descs ...string) model {
	m := model{list: tasklist.New(nil), projects: projects.New(), tags: tags.New(), reader: &stubReader{}, focus: focusStatus}
	for i, d := range descs {
		m.suggestedTasks = append(m.suggestedTasks, taskwarrior.Task{
			ID: i + 1, UUID: fmt.Sprintf("u%d", i+1), Description: d, Status: "pending", Project: "P",
			Urgency: float64(100 - i), // list order == rank order
		})
	}
	return m
}

func keyOf(m model, s string) model {
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	return nm.(model)
}

func TestSuggestedPanel_UpDownMovesHighlightClamped(t *testing.T) {
	m := suggestPanelModel("a", "b", "c", "d", "e", "f")
	assert.Equal(t, 0, m.selectedSuggestion())

	m = keyOf(m, "j")
	assert.Equal(t, 1, m.selectedSuggestion())
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = nm.(model)
	assert.Equal(t, 2, m.selectedSuggestion())
	m = keyOf(keyOf(m, "j"), "j")
	assert.Equal(t, 4, m.selectedSuggestion())
	m = keyOf(m, "j") // only 5 rows are listed
	assert.Equal(t, 4, m.selectedSuggestion())

	m = keyOf(keyOf(keyOf(m, "k"), "k"), "k")
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = nm.(model)
	assert.Equal(t, 0, m.selectedSuggestion())
	m = keyOf(m, "k")
	assert.Equal(t, 0, m.selectedSuggestion())
}

func TestSuggestedPanel_MovingWithNoSuggestionsIsSafe(t *testing.T) {
	m := keyOf(suggestPanelModel(), "j")
	assert.Equal(t, 0, m.selectedSuggestion())
}

func TestSuggestedPanel_SpaceJumpsToHighlightedTask(t *testing.T) {
	m := suggestPanelModel("a", "b", "c")
	m = keyOf(m, "j")
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	mm := nm.(model)
	assert.Equal(t, focusTasks, mm.focus)
	assert.Equal(t, 2, mm.pendingFocusID, "second suggestion, ID 2")
	assert.NotNil(t, cmd)
}

func TestSuggestedPanel_SpaceWithNothingIsNoOp(t *testing.T) {
	nm, cmd := suggestPanelModel().Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	assert.Nil(t, cmd)
	assert.Equal(t, focusStatus, nm.(model).focus)
}

func TestSuggestedPanel_SelectionSticksToTaskAcrossRefresh(t *testing.T) {
	m := keyOf(suggestPanelModel("a", "b", "c"), "j") // highlight "b" (u2)

	// A new, more urgent task pushes b down one row, but it is still in the top 5.
	refreshed := append([]taskwarrior.Task{{ID: 9, UUID: "u9", Description: "new", Status: "pending", Project: "P", Urgency: 500}}, m.suggestedTasks...)
	nm, _ := m.Update(suggestedTasksLoadedMsg{tasks: refreshed})
	mm := nm.(model)
	assert.Equal(t, 2, mm.selectedSuggestion(), "still on b, now row 3")

	// b drops out of the top 5 entirely: reset to row 1, and forget it.
	more := append([]taskwarrior.Task{
		{ID: 10, UUID: "u10", Description: "x", Status: "pending", Project: "P", Urgency: 900},
		{ID: 11, UUID: "u11", Description: "y", Status: "pending", Project: "P", Urgency: 800},
		{ID: 12, UUID: "u12", Description: "z", Status: "pending", Project: "P", Urgency: 700},
		{ID: 13, UUID: "u13", Description: "w", Status: "pending", Project: "P", Urgency: 600},
	}, refreshed...)
	nm, _ = mm.Update(suggestedTasksLoadedMsg{tasks: more})
	mm = nm.(model)
	assert.Equal(t, 0, mm.selectedSuggestion())
	assert.Equal(t, "", mm.suggestSel)
}

func TestSuggestedPanel_HighlightOnlyWhileFocused(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256) // tests run without a TTY, which strips all styling
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	m := suggestPanelModel("a") // one pick: unfocused shows only one row, so the text matches
	focused := m.suggestedContent(60)
	m.focus = focusTasks
	unfocused := m.suggestedContent(60)
	assert.NotEqual(t, focused, unfocused, "only the focused panel shows the highlight")
	assert.Equal(t, ansi.Strip(unfocused), ansi.Strip(focused), "same text either way")

	m.focus = focusStatus
	m.showingHelp = true
	assert.Equal(t, unfocused, m.suggestedContent(60), "no highlight while the help popup is open")
}

func TestHelp_SuggestedPanelHasLocalSection(t *testing.T) {
	m := suggestPanelModel("a")
	var keys []string
	for _, b := range m.helpLocalBindings() {
		keys = append(keys, b.Key)
	}
	assert.Contains(t, keys, "space")
}
