package addform

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
