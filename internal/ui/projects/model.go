// Package projects implements the lazygit-style bordered Projects panel: a
// selectable list of the distinct project names currently known to
// taskwarrior, plus two special entries pinned to the top for clearing
// the project filter or explicitly selecting tasks with no project.
package projects

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tkilb/lazytask/internal/ui/panel"
)

const (
	// NoneLabel selects tasks that have no project set (`project:`).
	NoneLabel = "(none)"
	// AllLabel clears the project filter entirely (no `project:` filter
	// applied at all).
	AllLabel = "(all)"

	// minPanelWidth is used when no tea.WindowSizeMsg has been received yet.
	minPanelWidth = 20
)

var selectedRowStyle = lipgloss.NewStyle().Reverse(true)

// Model is a Bubble Tea model rendering a bordered, selectable list of
// distinct project names, plus the NoneLabel/AllLabel special entries
// pinned to the top.
//
// This panel does not fetch or dedupe tasks itself: the caller (main.go)
// is expected to source the project names from an unfiltered task query
// (so applying a project filter doesn't shrink the very list used to
// change/clear that filter) and pass the already-deduplicated, sorted
// names to SetProjects.
type Model struct {
	projects []string
	cursor   int
	width    int
	height   int
	focused  bool
}

// New constructs an empty Model.
func New() Model {
	return Model{}
}

// SetProjects replaces the underlying distinct project names (expected
// already deduplicated/sorted by the caller), clamping the cursor so it
// remains within bounds of the resulting entry list (projects plus the two
// special entries).
func (m Model) SetProjects(projectNames []string) Model {
	m.projects = projectNames
	entries := m.entries()
	if m.cursor >= len(entries) {
		m.cursor = len(entries) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	return m
}

// entries returns the full selectable list: the AllLabel/NoneLabel special
// entries pinned to the top, followed by the real project names.
func (m Model) entries() []string {
	entries := make([]string, 0, len(m.projects)+2)
	entries = append(entries, AllLabel, NoneLabel)
	entries = append(entries, m.projects...)
	return entries
}

// Selected returns the entry currently under the cursor (a project name, or
// NoneLabel/AllLabel) and true, or "" and false if there are no entries
// (which should not normally happen, since the two special entries are
// always present).
func (m Model) Selected() (string, bool) {
	entries := m.entries()
	if len(entries) == 0 {
		return "", false
	}
	return entries[m.cursor], true
}

// SetFocused records whether the Projects panel currently has focus in the
// surrounding panel grid, so View can apply the shared focused-panel border
// highlight (see internal/ui/panel).
func (m Model) SetFocused(focused bool) Model {
	m.focused = focused
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model, handling up/down (and vim-style j/k)
// navigation and terminal resize events. Selection (enter) is handled by
// the caller, since it also needs to update the shared filter state.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		entries := m.entries()
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(entries)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	width := m.width
	if width <= 0 {
		width = minPanelWidth
	}
	innerWidth, innerHeight := panel.InnerSize(width, m.height)

	entries := m.entries()
	lines := make([]string, len(entries))
	for i, e := range entries {
		line := truncate(e, innerWidth)
		if i == m.cursor {
			line = selectedRowStyle.Render(line)
		}
		lines[i] = line
	}
	body := strings.Join(lines, "\n")

	return panel.Frame("3 Projects", body, innerWidth, innerHeight, m.focused)
}

// truncate shortens s to fit within width, adding an ellipsis if it was cut.
func truncate(s string, width int) string {
	if width <= 0 || len(s) <= width {
		return s
	}
	if width <= 1 {
		return s[:width]
	}
	return s[:width-1] + "…"
}
