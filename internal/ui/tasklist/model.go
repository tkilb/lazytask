// Package tasklist implements the lazygit-style bordered task list panel.
//
// This chunk renders whatever []taskwarrior.Task it is constructed with —
// it does not know how to fetch tasks itself. Live taskwarrior wiring is
// added in a later chunk (see requirements.md chunk 5).
package tasklist

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/panel"
)

const (
	// minPanelWidth is used when no tea.WindowSizeMsg has been received yet.
	minPanelWidth = 60
)

// StatusTab identifies which of the Tasks panel's status tabs
// (Todo/Done/Deleted) is currently selected.
type StatusTab int

const (
	TabTodo StatusTab = iota
	TabDone
	TabDeleted
)

// statusTabs is the fixed cycle order used by NextStatus/PrevStatus and by
// View when rendering the title's tab row.
var statusTabs = []StatusTab{TabTodo, TabDone, TabDeleted}

// Label returns the display name shown in the panel title for this tab.
func (t StatusTab) Label() string {
	switch t {
	case TabDone:
		return "Done"
	case TabDeleted:
		return "Deleted"
	default:
		return "Todo"
	}
}

// Filter returns the `task export` status filter corresponding to this tab.
func (t StatusTab) Filter() string {
	switch t {
	case TabDone:
		return "status:completed"
	case TabDeleted:
		return "status:deleted"
	default:
		return "status:pending"
	}
}

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("245"))

	selectedRowStyle = lipgloss.NewStyle().
				Reverse(true)
)

// Model is a Bubble Tea model rendering a bordered, selectable list of
// taskwarrior tasks.
type Model struct {
	tasks   []taskwarrior.Task
	cursor  int
	width   int
	height  int
	focused bool
	status  StatusTab
}

// New constructs a Model over the given tasks. The list starts with the
// first task (if any) selected.
func New(tasks []taskwarrior.Task) Model {
	return Model{tasks: tasks}
}

// SetTasks replaces the underlying task slice, clamping the cursor so it
// remains within bounds.
func (m Model) SetTasks(tasks []taskwarrior.Task) Model {
	m.tasks = tasks
	if m.cursor >= len(tasks) {
		m.cursor = len(tasks) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	return m
}

// Selected returns the currently selected task and true, or a zero Task and
// false if the list is empty.
func (m Model) Selected() (taskwarrior.Task, bool) {
	if len(m.tasks) == 0 {
		return taskwarrior.Task{}, false
	}
	return m.tasks[m.cursor], true
}

// SelectID moves the cursor to the task whose ID matches id, leaving the
// cursor unchanged if no task matches (e.g. it was already deleted/renumbered
// by the time the caller's refresh completed).
func (m Model) SelectID(id int) Model {
	for i, t := range m.tasks {
		if t.ID == id {
			m.cursor = i
			break
		}
	}
	return m
}

// SetFocused records whether the Tasks panel currently has focus in the
// surrounding panel grid, so View can apply the shared focused-panel border
// highlight (see internal/ui/panel).
func (m Model) SetFocused(focused bool) Model {
	m.focused = focused
	return m
}

// Status returns the currently selected status tab (Todo/Done/Deleted).
func (m Model) Status() StatusTab {
	return m.status
}

// StatusFilter returns the `task export` filter for the currently selected
// status tab, for callers (main.go) that need to refetch tasks.
func (m Model) StatusFilter() string {
	return m.status.Filter()
}

// NextStatus cycles forward through the Todo/Done/Deleted tabs (bound to
// `]`), resetting the cursor since the underlying task set is about to
// change.
func (m Model) NextStatus() Model {
	return m.setStatus((int(m.status) + 1) % len(statusTabs))
}

// PrevStatus cycles backward through the Todo/Done/Deleted tabs (bound to
// `[`), resetting the cursor since the underlying task set is about to
// change.
func (m Model) PrevStatus() Model {
	return m.setStatus((int(m.status) - 1 + len(statusTabs)) % len(statusTabs))
}

func (m Model) setStatus(i int) Model {
	m.status = statusTabs[i]
	m.cursor = 0
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model, handling up/down (and vim-style j/k)
// navigation and terminal resize events.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.tasks)-1 {
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
	// Account for the left/right border columns (see panel.InnerSize).
	innerWidth := width - 2
	if innerWidth < 20 {
		innerWidth = 20
	}

	_, innerHeight := panel.InnerSize(width, m.height)
	// One row is reserved for the header, so the list body scrolls within
	// whatever remains.
	visibleRows := innerHeight - 1
	if visibleRows < 1 {
		visibleRows = 1
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render(formatRow(innerWidth, "ID", "Description", "Project", "Priority", "Due")))
	b.WriteString("\n")

	if len(m.tasks) == 0 {
		b.WriteString("(no tasks)")
	} else {
		start, end := panel.ScrollWindow(m.cursor, len(m.tasks), visibleRows)
		for i := start; i < end; i++ {
			t := m.tasks[i]
			row := formatRow(innerWidth,
				fmt.Sprintf("%d", t.ID),
				t.Description,
				t.Project,
				t.Priority,
				t.Due,
			)
			if i == m.cursor {
				row = selectedRowStyle.Render(row)
			}
			b.WriteString(row)
			if i < end-1 {
				b.WriteString("\n")
			}
		}
	}

	body := b.String()

	tabs := make([]panel.Tab, len(statusTabs))
	for i, t := range statusTabs {
		tabs[i] = panel.Tab{Label: t.Label(), Active: t == m.status}
	}

	footer := ""
	if len(m.tasks) > 0 {
		footer = fmt.Sprintf("%d of %d", m.cursor+1, len(m.tasks))
	}
	return panel.FrameTabs('2', tabs, body, innerWidth, innerHeight, m.focused, footer)
}

// formatRow lays out the fixed-width columns used by both the header and
// data rows so they stay aligned.
func formatRow(width int, id, description, project, priority, due string) string {
	const (
		idWidth       = 4
		projectWidth  = 12
		priorityWidth = 4
		dueWidth      = 10
	)
	descWidth := width - idWidth - projectWidth - priorityWidth - dueWidth - 4
	if descWidth < 8 {
		descWidth = 8
	}

	return fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s",
		idWidth, truncate(id, idWidth),
		descWidth, truncate(description, descWidth),
		projectWidth, truncate(project, projectWidth),
		priorityWidth, truncate(priority, priorityWidth),
		dueWidth, truncate(due, dueWidth),
	)
}

// truncate shortens s to fit within width, adding an ellipsis if it was cut.
func truncate(s string, width int) string {
	if len(s) <= width {
		return s
	}
	if width <= 1 {
		return s[:width]
	}
	return s[:width-1] + "…"
}
