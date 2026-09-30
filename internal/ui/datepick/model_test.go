package datepick

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	m := New()
	assert.Equal(t, "", m.Value())
	assert.False(t, m.Focused())
}

func TestFocusBlur(t *testing.T) {
	m := New()

	m = m.Focus()
	assert.True(t, m.Focused())

	m = m.Blur()
	assert.False(t, m.Focused())
}

func typeString(t *testing.T, m Model, s string) Model {
	t.Helper()
	var cmd tea.Cmd
	for _, r := range s {
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		_ = cmd
	}
	return m
}

func TestResolvedEmptyInput(t *testing.T) {
	m := New().Focus()
	_, ok := m.Resolved()
	assert.False(t, ok)
}

func TestResolvedValidCord(t *testing.T) {
	fixedNow := time.Date(2024, time.January, 10, 9, 0, 0, 0, time.UTC)
	m := New().WithNow(func() time.Time { return fixedNow }).Focus()
	m = typeString(t, m, "2d")

	got, ok := m.Resolved()
	assert.True(t, ok)
	assert.True(t, got.Equal(fixedNow.AddDate(0, 0, 2)))
}

func TestResolvedZeroMeansToday(t *testing.T) {
	fixedNow := time.Date(2024, time.January, 10, 9, 0, 0, 0, time.UTC)

	for _, in := range []string{"0", "0d"} {
		m := New().WithNow(func() time.Time { return fixedNow }).Focus()
		m = typeString(t, m, in)

		got, ok := m.Resolved()
		assert.True(t, ok, "input %q should resolve", in)
		assert.True(t, got.Equal(fixedNow), "input %q should resolve to today, got %v", in, got)
	}
}

func TestResolvedInvalidCord(t *testing.T) {
	m := New().Focus()
	m = typeString(t, m, "bogus")

	_, ok := m.Resolved()
	assert.False(t, ok)
}

func TestResolvedAbsoluteDate(t *testing.T) {
	m := New().Focus()
	m = typeString(t, m, "2024-01-15")

	got, ok := m.Resolved()
	assert.True(t, ok)
	want := time.Date(2024, time.January, 15, 0, 0, 0, 0, time.Local)
	assert.True(t, got.Equal(want))
}

func TestResolvedTwoPartShorthandRollsToNextYear(t *testing.T) {
	// From Dec 30 2026, "1.5" (Jan 5, already passed for this year) rolls
	// forward to Jan 5 2027, matching the US month-day convention.
	fixedNow := time.Date(2026, time.December, 30, 9, 0, 0, 0, time.Local)
	m := New().WithNow(func() time.Time { return fixedNow }).Focus()
	m = typeString(t, m, "1.5")

	got, ok := m.Resolved()
	assert.True(t, ok)
	want := time.Date(2027, time.January, 5, 0, 0, 0, 0, time.Local)
	assert.True(t, got.Equal(want), "got %v, want %v", got, want)
}

func TestResolvedTwoPartShorthandVariousSeparatorsAndLeadingZeros(t *testing.T) {
	fixedNow := time.Date(2026, time.January, 1, 9, 0, 0, 0, time.Local)
	want := time.Date(2026, time.March, 14, 0, 0, 0, 0, time.Local)

	for _, in := range []string{"3-14", "3.14", "3/14", "03-14", "03.14", "03/14"} {
		t.Run(in, func(t *testing.T) {
			m := New().WithNow(func() time.Time { return fixedNow }).Focus()
			m = typeString(t, m, in)

			got, ok := m.Resolved()
			assert.True(t, ok)
			assert.True(t, got.Equal(want), "got %v, want %v", got, want)
		})
	}
}

func TestResolvedTwoPartShorthandStaysCurrentYearWhenNotPast(t *testing.T) {
	fixedNow := time.Date(2026, time.December, 30, 9, 0, 0, 0, time.Local)
	m := New().WithNow(func() time.Time { return fixedNow }).Focus()
	m = typeString(t, m, "12-31")

	got, ok := m.Resolved()
	assert.True(t, ok)
	want := time.Date(2026, time.December, 31, 0, 0, 0, 0, time.Local)
	assert.True(t, got.Equal(want), "got %v, want %v", got, want)
}

func TestResolvedThreePartUSOrderFourDigitYear(t *testing.T) {
	m := New().Focus()
	m = typeString(t, m, "3-14-2026")

	got, ok := m.Resolved()
	assert.True(t, ok)
	want := time.Date(2026, time.March, 14, 0, 0, 0, 0, time.Local)
	assert.True(t, got.Equal(want))
}

func TestResolvedThreePartUSOrderTwoDigitYear(t *testing.T) {
	m := New().Focus()
	m = typeString(t, m, "3.14.26")

	got, ok := m.Resolved()
	assert.True(t, ok)
	want := time.Date(2026, time.March, 14, 0, 0, 0, 0, time.Local)
	assert.True(t, got.Equal(want))
}

func TestResolvedThreePartISOOrder(t *testing.T) {
	m := New().Focus()
	m = typeString(t, m, "2026/3/14")

	got, ok := m.Resolved()
	assert.True(t, ok)
	want := time.Date(2026, time.March, 14, 0, 0, 0, 0, time.Local)
	assert.True(t, got.Equal(want))
}

func TestResolvedInvalidCalendarDate(t *testing.T) {
	for _, in := range []string{"2-30", "13-1", "2-30-2026"} {
		t.Run(in, func(t *testing.T) {
			m := New().Focus()
			m = typeString(t, m, in)

			_, ok := m.Resolved()
			assert.False(t, ok)
		})
	}
}

func TestFocusResetsPriorText(t *testing.T) {
	m := New().Focus()
	m = typeString(t, m, "2d")
	assert.Equal(t, "2d", m.Value())

	m = m.Focus()
	assert.Equal(t, "", m.Value())
}

func TestViewShowsPreview(t *testing.T) {
	fixedNow := time.Date(2024, time.January, 10, 9, 0, 0, 0, time.UTC)
	m := New().WithNow(func() time.Time { return fixedNow }).Focus()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = typeString(t, m, "2d")

	view := m.View()
	assert.Contains(t, view, "Due Date")
	assert.Contains(t, view, "Jan 12 2024")
}

func TestViewShowsErrorHint(t *testing.T) {
	m := New().Focus()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = typeString(t, m, "bogus")

	view := m.View()
	assert.Contains(t, view, "Invalid input")
}

func TestView_WidthCappedRegardlessOfTerminalWidth(t *testing.T) {
	m := New().Focus()
	// A wide terminal used to make the popup stretch to the full terminal
	// width; it should now stay capped at maxWidth instead.
	m, _ = m.Update(tea.WindowSizeMsg{Width: 220, Height: 40})

	lines := strings.Split(m.View(), "\n")
	require.NotEmpty(t, lines)
	got := ansi.StringWidth(lines[0])
	assert.LessOrEqual(t, got, maxWidth+2, "top border line %q is wider than the capped max", lines[0])
}

func TestView_WidthShrinksForNarrowTerminal(t *testing.T) {
	m := New().Focus()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 40})

	lines := strings.Split(m.View(), "\n")
	require.NotEmpty(t, lines)
	got := ansi.StringWidth(lines[0])
	assert.LessOrEqual(t, got, minWidth+2)
}

// TestTopBorder_FullyColored guards against the top border's corner/fill
// characters being left unstyled while the rest of the box (sides,
// bottom, title, hint) is colored — previously this made the popup render
// as "half" one color/"half" default terminal foreground.
func TestTopBorder_FullyColored(t *testing.T) {
	m := New().Focus()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	top := strings.Split(m.View(), "\n")[0]
	colorSeq := lipgloss.NewStyle().Foreground(borderColor).Render("x")
	prefix := colorSeq[:strings.IndexRune(colorSeq, 'x')]
	assert.True(t, strings.HasPrefix(top, prefix), "top border corner should open with the same color escape as the rest of the box")
}
