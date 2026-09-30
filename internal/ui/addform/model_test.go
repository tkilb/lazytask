package addform

import (
	"strings"
	"testing"

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

func TestFocusResetsPriorText(t *testing.T) {
	m := New().Focus()

	var cmd tea.Cmd
	for _, r := range "leftover text" {
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		_ = cmd
	}
	require.Equal(t, "leftover text", m.Value())

	m = m.Focus()
	assert.Equal(t, "", m.Value())
}

func TestReset(t *testing.T) {
	m := New().Focus()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	require.Equal(t, "hello", m.Value())

	m = m.Reset()
	assert.Equal(t, "", m.Value())
}

func TestUpdateTypesCharacters(t *testing.T) {
	m := New().Focus()

	for _, r := range "Buy milk" {
		var cmd tea.Cmd
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		_ = cmd
	}

	assert.Equal(t, "Buy milk", m.Value())
}

func TestUpdateWindowSizeResizesPanel(t *testing.T) {
	m := New()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	assert.Contains(t, m.View(), "Add Task")
}

func TestViewShowsTitleAndPlaceholder(t *testing.T) {
	m := New()
	view := m.View()
	assert.Contains(t, view, "Add Task")
	assert.Contains(t, view, "Task description...")
}

func TestViewShowsTypedValue(t *testing.T) {
	m := New().Focus()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("groceries")})
	assert.Contains(t, m.View(), "groceries")
}

func TestView_WidthCappedRegardlessOfTerminalWidth(t *testing.T) {
	m := New()
	// A wide terminal used to make the popup stretch to the full terminal
	// width; it should now stay capped at maxWidth instead.
	m, _ = m.Update(tea.WindowSizeMsg{Width: 220, Height: 40})

	lines := strings.Split(m.View(), "\n")
	require.NotEmpty(t, lines)
	got := ansi.StringWidth(lines[0])
	assert.LessOrEqual(t, got, maxWidth+2, "top border line %q is wider than the capped max", lines[0])
}

func TestView_WidthShrinksForNarrowTerminal(t *testing.T) {
	m := New()
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
	m := New()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	top := strings.Split(m.View(), "\n")[0]
	colorSeq := lipgloss.NewStyle().Foreground(borderColor).Render("x")
	prefix := colorSeq[:strings.IndexRune(colorSeq, 'x')]
	assert.True(t, strings.HasPrefix(top, prefix), "top border corner should open with the same color escape as the rest of the box")
}
