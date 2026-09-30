package tasklist

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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
		{ID: 1, Description: "Buy groceries", Project: "Home", Priority: "H", Due: "20260920T000000Z"},
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

	task := taskwarrior.Task{ID: 1, Description: "Buy groceries", Project: "Home", Priority: "H", Due: "20260920T000000Z"}

	row := renderDataRow(80, task, false, "", time.Now())

	_, _, _, priorityWidth, _ := columnWidths(80)
	priorityCell := lipgloss.NewStyle().Foreground(panel.PriorityHighColor).Render(
		fmt.Sprintf("%-*s", priorityWidth, "H"),
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

	row := renderDataRow(80, task, true, "groc", time.Now())

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

	row := renderDataRow(80, task, true, "groceries", time.Now())

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

	row := renderDataRow(80, task, false, "groc", time.Now())

	focusedStyle := lipgloss.NewStyle().Background(searchMatchColor).Foreground(lipgloss.Color("0"))
	unfocusedStyle := lipgloss.NewStyle().Background(searchMatchColorUnfocused).Foreground(lipgloss.Color("0"))
	assert.Contains(t, row, unfocusedStyle.Render("groc"))
	assert.NotContains(t, row, focusedStyle.Render("groc"))
}

func TestRenderDataRow_NoQueryLeavesDescriptionUnstyled(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	task := taskwarrior.Task{ID: 1, Description: "Buy groceries", Project: "Home"}

	row := renderDataRow(80, task, false, "", time.Now())

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

	row := renderDataRow(80, task, false, "", time.Now())

	assert.NotContains(t, row, "\n")
	assert.Contains(t, row, "Buy groceries and also milk")
}

// TestRenderDataRow_MultiByteDescriptionKeepsLaterColumnsAligned is a
// regression test: a Description containing a multi-byte UTF-8 rune (here
// a curly apostrophe, U+2019, 3 bytes/1 rune) previously threw off
// renderDescriptionCell's pad calculation, which used len() (byte count)
// instead of utf8.RuneCountInString, under-padding the Description cell
// and shifting every column after it left by the byte/rune delta. Project
// must render at exactly the same column position regardless of whether
// Description contains multi-byte characters.
func TestRenderDataRow_MultiByteDescriptionKeepsLaterColumnsAligned(t *testing.T) {
	plain := taskwarrior.Task{ID: 1, Description: "Investigate pricing and licensing", Project: "ccl"}
	multiByte := taskwarrior.Task{ID: 1, Description: "Investigate Tolge\u2019s pricing and licensing", Project: "ccl"}

	plainRow := renderDataRow(80, plain, false, "", time.Now())
	multiByteRow := renderDataRow(80, multiByte, false, "", time.Now())

	// byteIndexOf finds "ccl" and converts its byte offset to a rune
	// (display-column) offset, since a curly apostrophe earlier in the
	// row shifts the byte offset without moving the display column — the
	// bug fixed here was specifically about display-column alignment, so
	// the assertion must compare in rune terms, not raw bytes.
	byteIndexOf := func(s string, byteIdx int) int {
		require.GreaterOrEqual(t, byteIdx, 0)
		return utf8.RuneCountInString(s[:byteIdx])
	}
	plainByteIdx := strings.Index(plainRow, "ccl")
	multiByteByteIdx := strings.Index(multiByteRow, "ccl")
	require.Greater(t, plainByteIdx, 0)
	require.Greater(t, multiByteByteIdx, 0)
	plainRuneIdx := byteIndexOf(plainRow, plainByteIdx)
	multiByteRuneIdx := byteIndexOf(multiByteRow, multiByteByteIdx)
	assert.Equal(t, plainRuneIdx, multiByteRuneIdx, "Project column must start at the same display column regardless of multi-byte characters in Description")
}

// localDue builds a taskDueLayout-formatted Due string for a task due at
// local midnight on the given local calendar date, offsetDays days after
// baseLocal. Constructing both the "now" reference and Due fixtures from
// the same time.Local anchor (rather than hardcoding UTC clock times) is
// what keeps these tests deterministic regardless of the test runner's
// timezone, since dueDeltaDays compares local calendar dates.
func localDue(baseLocal time.Time, offsetDays int) string {
	return baseLocal.AddDate(0, 0, offsetDays).UTC().Format(taskDueLayout)
}

func TestDueDeltaDays(t *testing.T) {
	now := time.Date(2026, time.September, 20, 15, 0, 0, 0, time.Local)
	today := time.Date(2026, time.September, 20, 0, 0, 0, 0, time.Local)

	tests := []struct {
		name      string
		due       string
		wantDays  int
		wantFound bool
	}{
		{"empty due", "", 0, false},
		{"unparseable due", "not-a-date", 0, false},
		{"due today", localDue(today, 0), 0, true},
		{"due tomorrow", localDue(today, 1), 1, true},
		{"due in five days", localDue(today, 5), 5, true},
		{"overdue by one day", localDue(today, -1), -1, true},
		{"overdue by ten days", localDue(today, -10), -10, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			days, ok := dueDeltaDays(tc.due, now)
			assert.Equal(t, tc.wantFound, ok)
			if tc.wantFound {
				assert.Equal(t, tc.wantDays, days)
			}
		})
	}
}

func TestFormatDueCell(t *testing.T) {
	today := time.Date(2026, time.September, 20, 0, 0, 0, 0, time.Local)
	now := today

	text, ok := formatDueCell("", now)
	assert.False(t, ok)
	assert.Equal(t, "", text)

	text, ok = formatDueCell(localDue(today, 0), now)
	assert.True(t, ok)
	assert.Equal(t, "0", text)

	text, ok = formatDueCell(localDue(today, -1), now)
	assert.True(t, ok)
	assert.Equal(t, "-1", text)

	text, ok = formatDueCell(localDue(today, 2), now)
	assert.True(t, ok)
	assert.Equal(t, "2", text)
}

func TestDueColor(t *testing.T) {
	assert.Equal(t, dueOverdueColor, dueColor("-1"))
	assert.Equal(t, dueOverdueColor, dueColor("-10"))
	assert.Equal(t, dueTodayColor, dueColor("0"))
	assert.Equal(t, dueTomorrowColor, dueColor("1"))
	assert.Equal(t, dueLaterColor, dueColor("2"))
	assert.Equal(t, dueLaterColor, dueColor("30"))
}

func TestRenderDataRow_DueCellColoredByDelta(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	today := time.Date(2026, time.September, 20, 0, 0, 0, 0, time.Local)
	now := today
	_, _, _, _, dueWidth := columnWidths(80)

	overdue := taskwarrior.Task{ID: 1, Description: "Overdue task", Due: localDue(today, -1)}
	row := renderDataRow(80, overdue, false, "", now)
	wantCell := lipgloss.NewStyle().Foreground(dueOverdueColor).Render(fmt.Sprintf("%*s", dueWidth, "-1"))
	assert.Contains(t, row, wantCell)

	dueTodayTask := taskwarrior.Task{ID: 2, Description: "Due today task", Due: localDue(today, 0)}
	row = renderDataRow(80, dueTodayTask, false, "", now)
	wantCell = lipgloss.NewStyle().Foreground(dueTodayColor).Render(fmt.Sprintf("%*s", dueWidth, "0"))
	assert.Contains(t, row, wantCell)

	tomorrow := taskwarrior.Task{ID: 3, Description: "Due tomorrow task", Due: localDue(today, 1)}
	row = renderDataRow(80, tomorrow, false, "", now)
	wantCell = lipgloss.NewStyle().Foreground(dueTomorrowColor).Render(fmt.Sprintf("%*s", dueWidth, "1"))
	assert.Contains(t, row, wantCell)

	later := taskwarrior.Task{ID: 4, Description: "Due later task", Due: localDue(today, 5)}
	row = renderDataRow(80, later, false, "", now)
	wantCell = lipgloss.NewStyle().Foreground(dueLaterColor).Render(fmt.Sprintf("%*s", dueWidth, "5"))
	assert.Contains(t, row, wantCell)

	noDue := taskwarrior.Task{ID: 5, Description: "No due date task"}
	row = renderDataRow(80, noDue, false, "", now)
	assert.NotContains(t, row, "\x1b[")
}

func TestModel_WithNow_UsedByView(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	today := time.Date(2026, time.September, 20, 0, 0, 0, 0, time.Local)
	fixedNow := today
	tasks := []taskwarrior.Task{{ID: 1, Description: "Due tomorrow", Due: localDue(today, 1)}}
	m := New(tasks).WithNow(func() time.Time { return fixedNow })
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	view := m.View()
	_, _, _, _, dueWidth := columnWidths(78)
	// Cursor starts on the only (and thus selected) task, so the expected
	// style also carries the selected-row background/bold, matching how
	// renderDataRow composes dueStyle from base when selected.
	wantCell := lipgloss.NewStyle().Background(panel.SelectedRowBackground).Bold(true).
		Foreground(dueTomorrowColor).Render(fmt.Sprintf("%*s", dueWidth, "1"))
	assert.Contains(t, view, wantCell)
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
