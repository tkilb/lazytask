package tasklist

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/panel"
)

// sampleTasks returns fixture data used across tests; no taskwarrior
// process is ever invoked in this package.
func sampleTasks() []taskwarrior.Task {
	return []taskwarrior.Task{
		{ID: 1, Description: "Buy groceries", Project: "Home", Priority: "H", Due: "2026-09-20"},
		{ID: 2, Description: "Write report", Project: "Work", Priority: "M"},
		{ID: 3, Description: "Water plants", Project: "Home"},
	}
}

func TestNew_SelectsFirstTask(t *testing.T) {
	m := New(sampleTasks())

	selected, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, 1, selected.ID)
}

func TestNew_EmptyTasks(t *testing.T) {
	m := New(nil)

	_, ok := m.Selected()
	assert.False(t, ok)
}

func TestModel_Update_Navigation(t *testing.T) {
	tests := []struct {
		name       string
		keys       []string
		wantSelID  int
		wantCursor int
	}{
		{
			name:       "down moves to next task",
			keys:       []string{"down"},
			wantSelID:  2,
			wantCursor: 1,
		},
		{
			name:       "j moves to next task",
			keys:       []string{"j"},
			wantSelID:  2,
			wantCursor: 1,
		},
		{
			name:       "down twice then up once",
			keys:       []string{"down", "down", "up"},
			wantSelID:  2,
			wantCursor: 1,
		},
		{
			name:       "k does nothing at top",
			keys:       []string{"k"},
			wantSelID:  1,
			wantCursor: 0,
		},
		{
			name:       "down past end clamps at last task",
			keys:       []string{"down", "down", "down", "down", "down"},
			wantSelID:  3,
			wantCursor: 2,
		},
		{
			name:       "up past start clamps at first task",
			keys:       []string{"up", "up"},
			wantSelID:  1,
			wantCursor: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(sampleTasks())
			for _, k := range tt.keys {
				m, _ = m.Update(tea.KeyMsg{Type: keyTypeFor(k), Runes: []rune(k)})
			}

			assert.Equal(t, tt.wantCursor, m.cursor)
			selected, ok := m.Selected()
			require.True(t, ok)
			assert.Equal(t, tt.wantSelID, selected.ID)
		})
	}
}

// keyTypeFor maps the key strings used in table tests to the tea.KeyType
// bubbletea expects, since arrow keys are distinct types while letters are
// tea.KeyRunes.
func keyTypeFor(key string) tea.KeyType {
	switch key {
	case "up":
		return tea.KeyUp
	case "down":
		return tea.KeyDown
	default:
		return tea.KeyRunes
	}
}

func TestModel_SetTasks_ClampsCursor(t *testing.T) {
	m := New(sampleTasks())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	require.Equal(t, 2, m.cursor)

	m = m.SetTasks([]taskwarrior.Task{{ID: 9, Description: "only one left"}})
	assert.Equal(t, 0, m.cursor)

	selected, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, 9, selected.ID)
}

func TestModel_SetTasks_EmptyClampsToZero(t *testing.T) {
	m := New(sampleTasks())
	m = m.SetTasks(nil)

	_, ok := m.Selected()
	assert.False(t, ok)
}

func TestModel_SelectID_MovesCursorToMatchingTask(t *testing.T) {
	m := New(sampleTasks())

	m = m.SelectID(3)

	assert.Equal(t, 2, m.cursor)
	selected, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, 3, selected.ID)
}

func TestModel_SelectID_NoMatchLeavesCursorUnchanged(t *testing.T) {
	m := New(sampleTasks())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	require.Equal(t, 1, m.cursor)

	m = m.SelectID(999)

	assert.Equal(t, 1, m.cursor)
}

func TestModel_View_ContainsTaskData(t *testing.T) {
	m := New(sampleTasks())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	view := m.View()

	assert.True(t, strings.Contains(view, "Buy groceries"))
	assert.True(t, strings.Contains(view, "Write report"))
	assert.True(t, strings.Contains(view, "Water plants"))
	assert.True(t, strings.Contains(view, "Todo"))
}

func TestModel_View_Empty(t *testing.T) {
	m := New(nil)
	view := m.View()

	assert.True(t, strings.Contains(view, "(no tasks)"))
}

func TestModel_SetFocused_SetsFocusedFlag(t *testing.T) {
	m := New(sampleTasks())
	assert.False(t, m.focused)

	m = m.SetFocused(true)
	assert.True(t, m.focused)

	m = m.SetFocused(false)
	assert.False(t, m.focused)
}

func TestModel_NextStatus_CyclesTodoDoneDeleted(t *testing.T) {
	m := New(sampleTasks())
	assert.Equal(t, TabTodo, m.Status())
	assert.Equal(t, "status:pending", m.StatusFilter())

	m = m.NextStatus()
	assert.Equal(t, TabDone, m.Status())
	assert.Equal(t, "status:completed", m.StatusFilter())

	m = m.NextStatus()
	assert.Equal(t, TabDeleted, m.Status())
	assert.Equal(t, "status:deleted", m.StatusFilter())

	m = m.NextStatus()
	assert.Equal(t, TabTodo, m.Status())
}

func TestModel_PrevStatus_CyclesBackward(t *testing.T) {
	m := New(sampleTasks())

	m = m.PrevStatus()
	assert.Equal(t, TabDeleted, m.Status())

	m = m.PrevStatus()
	assert.Equal(t, TabDone, m.Status())

	m = m.PrevStatus()
	assert.Equal(t, TabTodo, m.Status())
}

func TestModel_NextStatus_ResetsCursor(t *testing.T) {
	m := New(sampleTasks())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	require.Equal(t, 1, m.cursor)

	m = m.NextStatus()
	assert.Equal(t, 0, m.cursor)
}

func TestModel_View_ShowsActiveTabHighlighted(t *testing.T) {
	m := New(sampleTasks())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	view := m.View()
	assert.True(t, strings.Contains(view, "[2]-Todo - Done - Deleted"))
}

func TestModel_View_ShowsPositionFooter(t *testing.T) {
	m := New(sampleTasks())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	view := m.View()
	assert.Contains(t, view, "2 of 3")
}

func TestModel_View_EmptyOmitsPositionFooter(t *testing.T) {
	m := New(nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	view := m.View()
	assert.NotContains(t, view, "0 of 0")
}

func manyTasks(n int) []taskwarrior.Task {
	tasks := make([]taskwarrior.Task, n)
	for i := 0; i < n; i++ {
		tasks[i] = taskwarrior.Task{ID: i + 1, Description: fmt.Sprintf("task %d", i+1)}
	}
	return tasks
}

func TestModel_View_ScrollsToKeepCursorVisible(t *testing.T) {
	// Height 8 gives 6 inner rows (InnerSize subtracts 2 for borders), minus
	// 1 for the header, so only 5 task rows are visible at once.
	m := New(manyTasks(20))
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})

	// Still at the top: task 1 visible, task 20 (off-screen) is not.
	view := m.View()
	assert.Contains(t, view, "task 1")
	assert.NotContains(t, view, "task 20")

	for i := 0; i < 19; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}

	// Cursor is now on the last task; the window should have scrolled so
	// it's visible, and the first task has scrolled out of view.
	view = m.View()
	assert.Contains(t, view, "task 20")
	assert.NotContains(t, view, "task 1 ")
	assert.Contains(t, view, "20 of 20")
}

func TestPriorityColor(t *testing.T) {
	cases := []struct {
		name     string
		priority string
		want     lipgloss.Color
	}{
		{"high", "H", panel.PriorityHighColor},
		{"medium", "M", panel.PriorityMediumColor},
		{"low", "L", panel.PriorityLowColor},
		{"none", "", panel.PriorityLowDefaultColor},
		{"unrecognized", "X", panel.PriorityLowDefaultColor},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, priorityColor(c.priority))
		})
	}
}

func TestRenderDataRow_ColorsOnlyPriorityCell(t *testing.T) {
	// Tests run without a TTY, so lipgloss auto-detects "no color" and
	// styles become no-ops; force a color profile so the ANSI codes this
	// test asserts on actually get emitted (matches a real terminal run).
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	task := taskwarrior.Task{ID: 1, Description: "Buy groceries", Project: "Home", Priority: "H", Due: "2026-09-20"}

	row := renderDataRow(80, task, false, "")

	priorityCell := lipgloss.NewStyle().Foreground(panel.PriorityHighColor).Render(
		fmt.Sprintf("%-*s", 4, "H"),
	)
	assert.Contains(t, row, priorityCell)

	// The description cell must not carry the priority foreground color:
	// rendering it plain and re-wrapping in the priority color should not
	// match what's actually in the row.
	_, descWidth, _, _, _ := columnWidths(80)
	plainDescCell := fmt.Sprintf("%-*s", descWidth, "Buy groceries")
	coloredDescCell := lipgloss.NewStyle().Foreground(panel.PriorityHighColor).Render(plainDescCell)
	assert.NotContains(t, row, coloredDescCell)
}

func TestRenderDataRow_HighlightsSearchMatchSubstringOnCursorRow(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	task := taskwarrior.Task{ID: 1, Description: "Buy groceries", Project: "Home"}

	row := renderDataRow(80, task, true, "groc")

	// Even on the selected (bold) cursor row, the matched substring itself
	// must never be bold.
	matchStyle := lipgloss.NewStyle().Background(panel.SelectedRowBackground).Bold(true).
		Background(searchMatchColor).Foreground(lipgloss.Color("0")).Bold(false)
	assert.Contains(t, row, matchStyle.Render("groc"))
}

func TestRenderDataRow_HighlightIsCaseInsensitive(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	task := taskwarrior.Task{ID: 1, Description: "Buy Groceries", Project: "Home"}

	row := renderDataRow(80, task, true, "groceries")

	matchStyle := lipgloss.NewStyle().Background(panel.SelectedRowBackground).Bold(true).
		Background(searchMatchColor).Foreground(lipgloss.Color("0")).Bold(false)
	assert.Contains(t, row, matchStyle.Render("Groceries"))
}

func TestRenderDataRow_HighlightUsesUnfocusedColorOnNonCursorRow(t *testing.T) {
	// A match on a row other than the one the cursor is on (selected=false)
	// uses searchMatchColorUnfocused (yellow) instead of searchMatchColor
	// (teal), so the current cursor row still stands out from the rest of
	// the matches.
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	task := taskwarrior.Task{ID: 1, Description: "Buy groceries", Project: "Home"}

	row := renderDataRow(80, task, false, "groc")

	focusedStyle := lipgloss.NewStyle().Background(searchMatchColor).Foreground(lipgloss.Color("0"))
	unfocusedStyle := lipgloss.NewStyle().Background(searchMatchColorUnfocused).Foreground(lipgloss.Color("0"))
	assert.Contains(t, row, unfocusedStyle.Render("groc"))
	assert.NotContains(t, row, focusedStyle.Render("groc"))
}

func TestRenderDataRow_NoQueryLeavesDescriptionUnstyled(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	task := taskwarrior.Task{ID: 1, Description: "Buy groceries", Project: "Home"}

	row := renderDataRow(80, task, false, "")

	// With no active search and no priority/selection styling, the row is
	// plain text with no ANSI escape codes at all.
	assert.NotContains(t, row, "\x1b[")
}

func TestRenderDataRow_CollapsesEmbeddedNewlinesInDescription(t *testing.T) {
	// A Description containing embedded newlines is possible today via
	// the $EDITOR multi-line Description flow; the tasklist row must stay
	// strictly single-line, never wrapped, so those newlines must be
	// collapsed away rather than leaking a raw "\n" into the rendered row.
	task := taskwarrior.Task{ID: 1, Description: "Buy groceries\nand also milk", Project: "Home"}

	row := renderDataRow(80, task, false, "")

	assert.NotContains(t, row, "\n")
	assert.Contains(t, row, "Buy groceries and also milk")
}

func TestSanitizeSingleLine_CollapsesNewlinesAndControlChars(t *testing.T) {
	got := sanitizeSingleLine("line one\nline two\r\nline three\x07end")
	assert.Equal(t, "line one line two  line three end", got)
	assert.NotContains(t, got, "\n")
	assert.NotContains(t, got, "\r")
}

func TestSetTasks_SortsByUrgencyDescending(t *testing.T) {
	tasks := []taskwarrior.Task{
		{ID: 1, Description: "low urgency", Urgency: 1.2},
		{ID: 2, Description: "high urgency", Urgency: 9.5},
		{ID: 3, Description: "mid urgency", Urgency: 4.0},
	}

	m := New(nil).SetTasks(tasks)

	got := make([]int, len(m.tasks))
	for i, task := range m.tasks {
		got[i] = task.ID
	}
	assert.Equal(t, []int{2, 3, 1}, got)
}

func TestNew_SortsByUrgencyDescending(t *testing.T) {
	tasks := []taskwarrior.Task{
		{ID: 1, Description: "low urgency", Urgency: 1.2},
		{ID: 2, Description: "high urgency", Urgency: 9.5},
	}

	m := New(tasks)

	assert.Equal(t, 2, m.tasks[0].ID)
	assert.Equal(t, 1, m.tasks[1].ID)
}

func TestSetTasks_SortsByEffectiveUrgencyIncludingUrgencyOffset(t *testing.T) {
	tasks := []taskwarrior.Task{
		{ID: 1, Description: "boosted by urgencyoffset", Urgency: 1.0, UrgencyOffset: 10.0},
		{ID: 2, Description: "high urgency, no urgencyoffset", Urgency: 9.5},
	}

	m := New(nil).SetTasks(tasks)

	got := make([]int, len(m.tasks))
	for i, task := range m.tasks {
		got[i] = task.ID
	}
	assert.Equal(t, []int{1, 2}, got)
}

func TestNeighbor_ReturnsAdjacentTaskInSortedOrder(t *testing.T) {
	tasks := []taskwarrior.Task{
		{ID: 1, Description: "low", Urgency: 1.0},
		{ID: 2, Description: "mid", Urgency: 5.0},
		{ID: 3, Description: "high", Urgency: 9.0},
	}
	// Sorted order (descending urgency): 3, 2, 1. Cursor defaults to 0 (task 3).
	m := New(tasks)

	below, ok := m.Neighbor(1)
	require.True(t, ok)
	assert.Equal(t, 2, below.ID)

	_, ok = m.Neighbor(-1)
	assert.False(t, ok)
}

func TestNeighbor_NoNeighborPastListBounds(t *testing.T) {
	tasks := []taskwarrior.Task{
		{ID: 1, Description: "only", Urgency: 1.0},
	}
	m := New(tasks)

	_, ok := m.Neighbor(1)
	assert.False(t, ok)
	_, ok = m.Neighbor(-1)
	assert.False(t, ok)
}

func TestSearch_JumpsToFirstMatchAtOrAfterCursor(t *testing.T) {
	m := New(sampleTasks())

	m, found := m.Search("water")
	require.True(t, found)
	selected, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, 3, selected.ID)
}

func TestSearch_CaseInsensitive(t *testing.T) {
	m := New(sampleTasks())

	m, found := m.Search("REPORT")
	require.True(t, found)
	selected, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, 2, selected.ID)
}

func TestSearch_NoMatch_ReturnsFalseAndLeavesCursor(t *testing.T) {
	m := New(sampleTasks())

	m, found := m.Search("nonexistent")
	assert.False(t, found)
	selected, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, 1, selected.ID, "cursor should stay put when nothing matches")
}

func TestSearch_MatchesDescriptionOnly_NotProject(t *testing.T) {
	m := New(sampleTasks())

	// "Home" is a project on tasks 1 and 3, but no description contains it.
	m, found := m.Search("home")
	assert.False(t, found)
	_ = m
}

func TestNextMatch_AdvancesPastCurrentAndWraps(t *testing.T) {
	tasks := []taskwarrior.Task{
		{ID: 1, Description: "buy milk"},
		{ID: 2, Description: "buy eggs"},
		{ID: 3, Description: "clean house"},
	}
	m := New(tasks)

	m, found := m.Search("buy")
	require.True(t, found)
	selected, _ := m.Selected()
	assert.Equal(t, 1, selected.ID)

	m, found = m.NextMatch()
	require.True(t, found)
	selected, _ = m.Selected()
	assert.Equal(t, 2, selected.ID, "n should advance to the next match, not stay put")

	// Wraps back around to the first match since task 3 doesn't match.
	m, found = m.NextMatch()
	require.True(t, found)
	selected, _ = m.Selected()
	assert.Equal(t, 1, selected.ID)
}

func TestPrevMatch_WrapsBackward(t *testing.T) {
	tasks := []taskwarrior.Task{
		{ID: 1, Description: "buy milk"},
		{ID: 2, Description: "buy eggs"},
		{ID: 3, Description: "clean house"},
	}
	m := New(tasks)

	m, found := m.Search("buy")
	require.True(t, found)

	m, found = m.PrevMatch()
	require.True(t, found)
	selected, _ := m.Selected()
	assert.Equal(t, 2, selected.ID, "N should wrap backward to the previous match")
}

func TestNextMatch_NoActiveQuery_IsNoOp(t *testing.T) {
	m := New(sampleTasks())

	_, found := m.NextMatch()
	assert.False(t, found)
}

func TestPrevMatch_NoActiveQuery_IsNoOp(t *testing.T) {
	m := New(sampleTasks())

	_, found := m.PrevMatch()
	assert.False(t, found)
}
