package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/popup"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
)

func click(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
}

func wheel(x, y int, b tea.MouseButton) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Button: b, Action: tea.MouseActionPress}
}

func TestPanelAt_MapsCoordinatesToPanels(t *testing.T) {
	m := layoutModel(3, focusTasks, 40)
	leftWidth, _, statusHeight, tasksHeight, bottomHeight, fullHeight := m.gridDims()
	projectsWidth, _ := subColumnWidths(leftWidth)

	tests := []struct {
		name string
		x, y int
		want panelFocus
		ok   bool
	}{
		{"suggested", 2, 0, focusStatus, true},
		{"tasks top", 2, statusHeight, focusTasks, true},
		{"tasks bottom", 2, statusHeight + tasksHeight - 1, focusTasks, true},
		{"projects", 2, statusHeight + tasksHeight, focusProjects, true},
		{"tags", projectsWidth, statusHeight + tasksHeight, focusTags, true},
		{"details", leftWidth, 0, focusDetails, true},
		{"details bottom", leftWidth + 3, fullHeight - 1, focusDetails, true},
		{"below the grid", 2, statusHeight + tasksHeight + bottomHeight, 0, false},
		{"negative", -1, 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := m.panelAt(tt.x, tt.y)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPanelAt_TasklistOnlyHidesLeftPanels(t *testing.T) {
	m := layoutModel(3, focusTasks, tasklistOnlyBelow-1)
	require.True(t, m.tasklistOnly())

	got, ok := m.panelAt(2, 0)
	require.True(t, ok)
	assert.Equal(t, focusTasks, got)

	leftWidth, _, _, _, _, _ := m.gridDims()
	got, ok = m.panelAt(leftWidth, 0)
	require.True(t, ok)
	assert.Equal(t, focusDetails, got)
}

func TestMouse_LeftClickFocusesPanel(t *testing.T) {
	m := layoutModel(3, focusTasks, 40)
	leftWidth, _, _, _, _, _ := m.gridDims()

	nm, _ := m.Update(click(leftWidth+1, 1))
	assert.Equal(t, focusDetails, nm.(model).focus)

	nm, _ = nm.(model).Update(click(1, 0))
	assert.Equal(t, focusStatus, nm.(model).focus)
}

func TestMouse_ClickOutsideGridKeepsFocus(t *testing.T) {
	m := layoutModel(3, focusTasks, 40)
	nm, _ := m.Update(click(1, 39))
	assert.Equal(t, focusTasks, nm.(model).focus)
}

func TestMouse_NonPressAndOtherButtonsAreIgnored(t *testing.T) {
	m := layoutModel(3, focusTasks, 40)
	leftWidth, _, _, _, _, _ := m.gridDims()

	for _, msg := range []tea.MouseMsg{
		{X: leftWidth + 1, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease},
		{X: leftWidth + 1, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion},
		{X: leftWidth + 1, Y: 1, Button: tea.MouseButtonRight, Action: tea.MouseActionPress},
	} {
		nm, _ := m.Update(msg)
		assert.Equal(t, focusTasks, nm.(model).focus, "%v", msg)
	}
}

func TestMouse_WheelMovesSuggestionUnderPointer(t *testing.T) {
	m := layoutModel(3, focusTasks, 40)
	require.GreaterOrEqual(t, len(m.suggestions()), 2)

	nm, _ := m.Update(wheel(1, 0, tea.MouseButtonWheelDown))
	got := nm.(model)
	assert.Equal(t, focusStatus, got.focus, "wheel focuses the panel under the pointer")
	assert.Equal(t, 1, got.selectedSuggestion())

	nm, _ = got.Update(wheel(1, 0, tea.MouseButtonWheelUp))
	assert.Equal(t, 0, nm.(model).selectedSuggestion())
}

func TestMouse_WheelScrollsTasks(t *testing.T) {
	m := layoutModel(0, focusProjects, 40)
	nm, _ := m.Update(tasksLoadedMsg{tasks: []taskwarrior.Task{
		{ID: 1, UUID: "a", Description: "one", Status: "pending"},
		{ID: 2, UUID: "b", Description: "two", Status: "pending"},
	}})
	m = nm.(model)
	_, _, statusHeight, _, _, _ := m.gridDims()

	nm, _ = m.Update(wheel(1, statusHeight, tea.MouseButtonWheelDown))
	got := nm.(model)
	assert.Equal(t, focusTasks, got.focus)
	sel, ok := got.list.Selected()
	require.True(t, ok)
	assert.Equal(t, 2, sel.ID)
}

func TestMouse_IgnoredWhileModalOpen(t *testing.T) {
	m := layoutModel(3, focusTasks, 40)
	leftWidth, _, _, _, _, _ := m.gridDims()
	m.deleting = true

	nm, _ := m.Update(click(leftWidth+1, 1))
	assert.Equal(t, focusTasks, nm.(model).focus)
}

func TestMouse_IgnoredWhilePopupActive(t *testing.T) {
	m := layoutModel(3, focusTasks, 40)
	leftWidth, _, _, _, _, _ := m.gridDims()
	m.popups = m.popups.Push(popup.Message{Severity: popup.Error, Text: "boom"})
	require.True(t, m.popups.Active())

	nm, _ := m.Update(click(leftWidth+1, 1))
	assert.Equal(t, focusTasks, nm.(model).focus)
}

func clickModel(t *testing.T) model {
	t.Helper()
	m := layoutModel(0, focusTasks, 40)
	nm, _ := m.Update(tasksLoadedMsg{tasks: []taskwarrior.Task{
		{ID: 1, UUID: "a", Description: "one", Status: "pending"},
		{ID: 2, UUID: "b", Description: "two", Status: "pending"},
		{ID: 3, UUID: "c", Description: "three", Status: "pending"},
	}})
	m = nm.(model)
	nm, _ = m.Update(projectsLoadedMsg{projects: []string{"alpha", "beta"}})
	m = nm.(model)
	nm, _ = m.Update(tagsLoadedMsg{tags: []string{"x", "y"}})
	return nm.(model)
}

func TestMouse_ClickTaskRowSelectsIt(t *testing.T) {
	m := clickModel(t)
	_, _, statusHeight, _, _, _ := m.gridDims()

	// Row 0 of the panel is the border, 1 the header, 2 the first task.
	nm, _ := m.Update(click(3, statusHeight+2+2))
	sel, ok := nm.(model).list.Selected()
	require.True(t, ok)
	assert.Equal(t, 3, sel.ID)

	nm, _ = nm.(model).Update(click(3, statusHeight+1))
	sel, _ = nm.(model).list.Selected()
	assert.Equal(t, 3, sel.ID, "header click keeps selection")
}

func TestMouse_ClickTaskTabSwitchesAndRefetches(t *testing.T) {
	m := clickModel(t)
	_, _, statusHeight, _, _, _ := m.gridDims()

	// "╭──[2]-Todo - Done": Done starts at column 14.
	nm, cmd := m.Update(click(14, statusHeight))
	got := nm.(model)
	assert.Equal(t, tasklist.TabDone, got.list.Status())
	assert.NotNil(t, cmd, "tab change refetches tasks")
}

func TestMouse_ClickProjectRowAppliesFilter(t *testing.T) {
	m := clickModel(t)
	_, _, statusHeight, tasksHeight, _, _ := m.gridDims()

	// Entries: (all), (none), alpha, beta; the first sits one row below
	// the border.
	nm, cmd := m.Update(click(2, statusHeight+tasksHeight+1+3))
	got := nm.(model)
	assert.Equal(t, focusProjects, got.focus)
	label, _ := got.projects.Selected()
	assert.Equal(t, "beta", label)
	require.NotNil(t, got.filter.project)
	assert.Equal(t, "beta", *got.filter.project)
	assert.NotNil(t, cmd)
}

func TestMouse_ClickTagRowAppliesFilter(t *testing.T) {
	m := clickModel(t)
	leftWidth, _, statusHeight, tasksHeight, _, _ := m.gridDims()
	projectsWidth, _ := subColumnWidths(leftWidth)

	// Entries: (any), x, y.
	nm, _ := m.Update(click(projectsWidth+2, statusHeight+tasksHeight+1+1))
	got := nm.(model)
	assert.Equal(t, focusTags, got.focus)
	label, _ := got.tags.Selected()
	assert.Equal(t, "x", label)
}

func TestMouse_ClickSuggestionSelectsIt(t *testing.T) {
	m := layoutModel(3, focusStatus, 40)
	require.Equal(t, 0, m.selectedSuggestion())

	nm, _ := m.Update(click(2, 1+2))
	assert.Equal(t, 2, nm.(model).selectedSuggestion())
}

func TestMouse_ClickBlankRowOnlyFocuses(t *testing.T) {
	m := clickModel(t)
	_, _, statusHeight, tasksHeight, _, _ := m.gridDims()

	nm, _ := m.Update(click(2, statusHeight+tasksHeight-2))
	got := nm.(model)
	assert.Equal(t, focusTasks, got.focus)
	sel, _ := got.list.Selected()
	assert.Equal(t, 1, sel.ID)
}
