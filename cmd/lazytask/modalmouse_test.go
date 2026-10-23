package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tkilb/lazytask/internal/ui/addform"
	"github.com/tkilb/lazytask/internal/ui/popup"
	"github.com/tkilb/lazytask/internal/ui/taskform"
)

func runes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

// press feeds a key and returns the updated model.
func press(t *testing.T, m model, key tea.KeyMsg) model {
	t.Helper()
	nm, _ := m.Update(key)
	return nm.(model)
}

// clickInBox clicks (x, y) relative to the open modal's top-left corner.
func clickInBox(t *testing.T, m model, x, y int) (model, tea.Cmd) {
	t.Helper()
	w, h := m.screenDims()
	box, _, ok := m.modalBox(w, h)
	require.True(t, ok, "a modal must be open")
	bw, bh := lipgloss.Size(box)
	nm, cmd := m.Update(click((w-bw)/2+x, (h-bh)/2+y))
	return nm.(model), cmd
}

func boxSize(t *testing.T, m model) (int, int) {
	t.Helper()
	w, h := m.screenDims()
	box, _, ok := m.modalBox(w, h)
	require.True(t, ok)
	return lipgloss.Size(box)
}

func TestModalMouse_ConfirmChoices(t *testing.T) {
	m := press(t, clickModel(t), runes("d"))
	require.True(t, m.completing)
	_, bh := boxSize(t, m)

	// "(y/enter) confirm" starts two columns in; cancel follows the gap.
	got, cmd := clickInBox(t, m, 3, bh-2)
	assert.False(t, got.completing, "confirm closes the prompt")
	assert.NotNil(t, cmd, "confirm runs the action")

	got, cmd = clickInBox(t, m, 2+len("(y/enter) confirm")+3+1, bh-2)
	assert.False(t, got.completing, "cancel closes the prompt")
	assert.Nil(t, cmd, "cancel runs nothing")
}

func TestModalMouse_ConfirmIgnoresBodyClicksAndCancelsOutside(t *testing.T) {
	m := press(t, clickModel(t), runes("d"))
	require.True(t, m.completing)

	got, _ := clickInBox(t, m, 5, 3)
	assert.True(t, got.completing, "body click keeps it open")

	nm, _ := m.Update(click(0, 0))
	assert.False(t, nm.(model).completing, "outside click cancels")
}

func TestModalMouse_PickerRowChoosesAndAppliesFilter(t *testing.T) {
	m := press(t, clickModel(t), runes("p"))
	require.True(t, m.pickingProject)

	// Row 0 is the border; entries are (all), (none), alpha, beta.
	got, cmd := clickInBox(t, m, 3, 1+3)
	assert.False(t, got.pickingProject)
	require.NotNil(t, got.filter.project)
	assert.Equal(t, "beta", *got.filter.project)
	assert.NotNil(t, cmd)
}

func TestModalMouse_PickerOutsideClickCancels(t *testing.T) {
	m := press(t, clickModel(t), runes("t"))
	require.True(t, m.pickingTag)

	nm, _ := m.Update(click(0, 0))
	got := nm.(model)
	assert.False(t, got.pickingTag)
	assert.Nil(t, got.filter.tag, "nothing applied")
}

func TestModalMouse_PickerWheelMovesCursor(t *testing.T) {
	m := press(t, clickModel(t), runes("p"))
	before, _ := m.projectPicker.Selected()

	nm, _ := m.Update(wheel(0, 0, tea.MouseButtonWheelDown))
	got := nm.(model)
	assert.True(t, got.pickingProject)
	after, _ := got.projectPicker.Selected()
	assert.NotEqual(t, before, after)
}

func TestModalMouse_AssignRowTogglesCheckbox(t *testing.T) {
	m := press(t, clickModel(t), runes("T"))
	require.True(t, m.assigningTags)
	require.Empty(t, m.tagAssignPicker.Checked())

	// Row 1 is "(type tags...)", row 2 the first tag.
	got, _ := clickInBox(t, m, 3, 2)
	assert.True(t, got.assigningTags, "stays open to toggle more")
	assert.Len(t, got.tagAssignPicker.Checked(), 1)

	got, _ = clickInBox(t, m, 3, 1)
	assert.True(t, got.tagAssignTyping, "the type row opens the prompt")
}

func TestModalMouse_ReassignRowChoosesProject(t *testing.T) {
	m := press(t, clickModel(t), runes("P"))
	require.True(t, m.reassigning)

	got, cmd := clickInBox(t, m, 3, 1) // (none)
	assert.False(t, got.reassigning)
	assert.NotNil(t, cmd)
}

func TestModalMouse_HelpOutsideClosesAndWheelScrolls(t *testing.T) {
	m := clickModel(t)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 14})
	m = press(t, nm.(model), runes("?"))
	require.True(t, m.showingHelp)

	nm, _ = m.Update(wheel(0, 0, tea.MouseButtonWheelDown))
	assert.True(t, nm.(model).showingHelp)
	assert.Greater(t, nm.(model).helpScroll, 0)

	nm, _ = m.Update(click(0, 0))
	assert.False(t, nm.(model).showingHelp)
}

func TestModalMouse_FormClickFocusesFieldAndOutsideKeepsIt(t *testing.T) {
	m := clickModel(t)
	m.add = taskform.New()
	m = press(t, m, runes("a"))
	require.True(t, m.adding)
	require.Equal(t, taskform.FieldDescription, m.add.FocusedField())

	// Rows: Description 0-2, Annotations 3-7, Project 8-10, Tags 11-13.
	got, _ := clickInBox(t, m, 5, 12)
	assert.Equal(t, taskform.FieldTags, got.add.FocusedField())
	got, _ = clickInBox(t, got, 5, 4)
	assert.Equal(t, taskform.FieldAnnotations, got.add.FocusedField())

	nm, _ := got.Update(click(0, 0))
	assert.True(t, nm.(model).adding, "outside click never discards the form")
}

func TestModalMouse_PopupDismissedByClickOnly(t *testing.T) {
	m := layoutModel(3, focusTasks, 40)
	m.popups = m.popups.Push(popup.Message{Severity: popup.Error, Text: "boom"})

	nm, _ := m.Update(wheel(0, 0, tea.MouseButtonWheelDown))
	assert.True(t, nm.(model).popups.Active(), "wheel does not dismiss")

	nm, _ = m.Update(click(0, 0))
	assert.False(t, nm.(model).popups.Active())
}

func TestModalMouse_InputModalsIgnoreMouse(t *testing.T) {
	m := clickModel(t)
	m.reassignTyping = true
	m.reassignInput = addform.NewNamed("Reassign Project", "", "").Focus()

	nm, _ := m.Update(click(0, 0))
	assert.True(t, nm.(model).reassignTyping)
}

func TestModalMouse_NoModalFallsThroughToGrid(t *testing.T) {
	m := clickModel(t)
	leftWidth, _, _, _, _, _ := m.gridDims()
	nm, _ := m.Update(click(leftWidth+1, 1))
	assert.Equal(t, focusDetails, nm.(model).focus)
}
