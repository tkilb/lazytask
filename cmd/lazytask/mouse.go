package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tkilb/lazytask/internal/config"
)

// programOptions returns the tea.ProgramOptions lazytask starts with: the
// alternate screen (see main), plus cell-motion mouse reporting unless
// ui.mouse is false in the app config. A broken config keeps the default
// (mouse on) and warns, matching how the git-sync config is handled.
func programOptions() []tea.ProgramOption {
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	uiCfg, err := config.LoadUIConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load ui config: %v\n", err)
	}
	if uiCfg.MouseEnabled() {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	return opts
}

// mouseHit is a terminal cell resolved to a panel and the cell's position
// within that panel's outer frame (0,0 is its top-left border corner).
type mouseHit struct {
	panel panelFocus
	x, y  int
}

// hitAt maps a terminal cell to the panel drawn there, mirroring the
// layout renderGrid builds from gridDims. It reports false for cells
// outside the grid (e.g. the status bar row).
func (m model) hitAt(x, y int) (mouseHit, bool) {
	leftWidth, _, statusHeight, tasksHeight, bottomHeight, fullHeight := m.gridDims()
	if x < 0 || y < 0 || y >= fullHeight {
		return mouseHit{}, false
	}
	if x >= leftWidth {
		return mouseHit{focusDetails, x - leftWidth, y}, true
	}
	if m.tasklistOnly() {
		return mouseHit{focusTasks, x, y}, true
	}
	switch {
	case y < statusHeight:
		return mouseHit{focusStatus, x, y}, true
	case y < statusHeight+tasksHeight:
		return mouseHit{focusTasks, x, y - statusHeight}, true
	case y < statusHeight+tasksHeight+bottomHeight:
		row := y - statusHeight - tasksHeight
		projectsWidth, _ := subColumnWidths(leftWidth)
		if x < projectsWidth {
			return mouseHit{focusProjects, x, row}, true
		}
		return mouseHit{focusTags, x - projectsWidth, row}, true
	}
	return mouseHit{}, false
}

// panelAt is hitAt without the in-panel position.
func (m model) panelAt(x, y int) (panelFocus, bool) {
	hit, ok := m.hitAt(x, y)
	return hit.panel, ok
}

// clickPanel focuses the clicked panel and then, for the list panels,
// selects the row or tab under the pointer. The row is resolved against
// the layout the user saw (before the focus change resizes panels), then
// applied afterwards, running the same follow-up as an up/down keypress so
// Projects and Tags still apply their filters and Tasks tabs refetch.
func (m model) clickPanel(hit mouseHit) (model, tea.Cmd) {
	switch hit.panel {
	case focusTasks:
		if tab, ok := m.list.TabAt(hit.x); ok && hit.y == 0 {
			m = m.setFocus(focusTasks)
			m.list = m.list.SelectTab(tab)
			return m, fetchTasks(m.reader, m.taskFilters()...)
		}
		i, ok := m.list.RowAt(hit.y)
		m = m.setFocus(focusTasks)
		if ok {
			m.list = m.list.SelectIndex(i)
		}
	case focusProjects:
		i, ok := m.projects.RowAt(hit.y)
		m = m.setFocus(focusProjects)
		if ok {
			m.projects = m.projects.SelectIndex(i)
			return m.afterProjectsCursorMove()
		}
	case focusTags:
		i, ok := m.tags.RowAt(hit.y)
		m = m.setFocus(focusTags)
		if ok {
			m.tags = m.tags.SelectIndex(i)
			return m.afterTagsCursorMove()
		}
	case focusStatus:
		row := hit.y - 1
		picks := m.suggestions()
		m = m.setFocus(focusStatus)
		if row >= 0 && row < min(len(picks), suggestedRows(m.focus, len(picks))) {
			m.suggestSel = suggestionKey(m.suggestedTasks[picks[row].Index])
		}
	default:
		m = m.setFocus(hit.panel)
	}
	return m, nil
}

// updateMouse handles mouse input on the main screen: a left click focuses
// the panel under the pointer and selects the row or Tasks tab clicked, and the wheel focuses it then moves its
// selection, reusing the up/down key handling so the Projects and Tags
// panels still apply their filters. Modal states never reach here (Update
// routes them first), and a pending popup blocks the mouse like any input.
func (m model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.popups.Active() || msg.Action != tea.MouseActionPress {
		return m, nil
	}
	hit, ok := m.hitAt(msg.X, msg.Y)
	if !ok {
		return m, nil
	}
	target := hit.panel

	switch msg.Button {
	case tea.MouseButtonLeft:
		return m.clickPanel(hit)
	case tea.MouseButtonWheelUp:
		return m.setFocus(target).Update(tea.KeyMsg{Type: tea.KeyUp})
	case tea.MouseButtonWheelDown:
		return m.setFocus(target).Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	return m, nil
}
