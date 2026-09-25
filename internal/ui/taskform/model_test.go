package taskform

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func typeString(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func TestModel_Focus(t *testing.T) {
	m := New().Focus()
	assert.True(t, m.Focused())
	assert.Equal(t, FieldDescription, m.FocusedField())
}

func TestModel_DescriptionTyping(t *testing.T) {
	m := New().Focus()
	m = typeString(t, m, "Buy milk")
	assert.Equal(t, "Buy milk", m.Description())
}

func TestModel_TabCyclesFields(t *testing.T) {
	m := New().Focus()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, FieldAnnotations, m.FocusedField())

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, FieldProject, m.FocusedField())

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, FieldPriority, m.FocusedField())

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, FieldDueDate, m.FocusedField())

	// Wraps back around to Description.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, FieldDescription, m.FocusedField())

	// Shift+tab wraps backwards.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	assert.Equal(t, FieldDueDate, m.FocusedField())
}

func TestModel_AnnotationsTyping(t *testing.T) {
	m := New().Focus()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, FieldAnnotations, m.FocusedField())

	m = typeString(t, m, "first note")
	assert.Equal(t, "first note", m.Annotations())
}

func TestModel_AnnotationsEnterInsertsNewline(t *testing.T) {
	m := New().Focus()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = typeString(t, m, "line one")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = typeString(t, m, "line two")

	// <enter> inserted a newline within the Annotations field rather than
	// being swallowed/ignored, and focus stayed on Annotations (the host
	// model is the one that special-cases <enter> as "submit", never
	// taskform itself).
	assert.Equal(t, "line one\nline two", m.Annotations())
	assert.Equal(t, FieldAnnotations, m.FocusedField())
}

func TestModel_TabTypingGoesToFocusedField(t *testing.T) {
	m := New().Focus()
	m = typeString(t, m, "Buy milk")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Annotations
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Project
	m = typeString(t, m, "chores")
	assert.Equal(t, "chores", m.Project())
	assert.Equal(t, "Buy milk", m.Description())

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Priority
	m = typeString(t, m, "h")
	priority, ok := m.Priority()
	assert.True(t, ok)
	assert.Equal(t, "H", priority)
}

func TestModel_SetProject(t *testing.T) {
	m := New().Focus().SetProject("chores")
	assert.Equal(t, "chores", m.Project())
}

func TestModel_Priority(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		wantOK bool
	}{
		{name: "blank is valid, empty value", input: "", want: "", wantOK: true},
		{name: "lowercase h normalizes to H", input: "h", want: "H", wantOK: true},
		{name: "uppercase M", input: "M", want: "M", wantOK: true},
		{name: "lowercase l normalizes to L", input: "l", want: "L", wantOK: true},
		{name: "invalid letter", input: "x", want: "X", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New().Focus()
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Annotations
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Project
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Priority
			m = typeString(t, m, tt.input)

			got, ok := m.Priority()
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestModel_ResolveDue(t *testing.T) {
	fixedNow := time.Date(2024, time.January, 10, 12, 0, 0, 0, time.UTC)
	m := New().WithNow(func() time.Time { return fixedNow }).Focus()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Annotations
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Project
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Priority
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Due Date
	m = typeString(t, m, "2d")

	assert.Equal(t, "2d", m.DueInput())
	resolved, ok := m.ResolveDue()
	require.True(t, ok)
	assert.Equal(t, time.Date(2024, time.January, 12, 12, 0, 0, 0, time.UTC), resolved)
}

func TestModel_ResolveDue_Invalid(t *testing.T) {
	m := New().Focus()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Annotations
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Project
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Priority
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Due Date
	m = typeString(t, m, "not-a-date")

	_, ok := m.ResolveDue()
	assert.False(t, ok)
}

func TestModel_FocusResetsPriorFields(t *testing.T) {
	m := New().Focus()
	m = typeString(t, m, "Buy milk")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Annotations
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Project
	m = typeString(t, m, "chores")

	m = m.Focus()
	assert.Empty(t, m.Description())
	assert.Empty(t, m.Project())
	assert.Equal(t, FieldDescription, m.FocusedField())
}

func TestModel_Reset(t *testing.T) {
	m := New().Focus()
	m = typeString(t, m, "Buy milk")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Annotations
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Project
	m = typeString(t, m, "chores")

	m = m.Reset()
	assert.Empty(t, m.Description())
	assert.Empty(t, m.Project())
	// Reset doesn't change focus.
	assert.Equal(t, FieldProject, m.FocusedField())
}

func TestModel_View(t *testing.T) {
	m := New().Focus()
	view := m.View()
	assert.Contains(t, view, "Description")
	assert.Contains(t, view, "Annotations")
	assert.Contains(t, view, "Project")
	assert.Contains(t, view, "Priority")
	assert.Contains(t, view, "Due Date")
	// Each field is its own bordered box (lazygit-style), so there must be
	// more than one top/bottom border pair.
	assert.GreaterOrEqual(t, strings.Count(view, "╭"), 5)
	assert.GreaterOrEqual(t, strings.Count(view, "╰"), 5)
}
