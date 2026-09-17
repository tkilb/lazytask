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
)

const (
	// minPanelWidth is used when no tea.WindowSizeMsg has been received yet.
	minPanelWidth = 60
)

var (
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("62"))

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("245"))

	selectedRowStyle = lipgloss.NewStyle().
				Reverse(true)
)

// Model is a Bubble Tea model rendering a bordered, selectable list of
// taskwarrior tasks.
type Model struct {
	tasks  []taskwarrior.Task
	cursor int
	width  int
	height int
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
	// Account for border + padding consumed by borderStyle.
	innerWidth := width - 4
	if innerWidth < 20 {
		innerWidth = 20
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Tasks"))
	b.WriteString("\n")
	b.WriteString(headerStyle.Render(formatRow(innerWidth, "ID", "Description", "Project", "Priority", "Due")))
	b.WriteString("\n")

	if len(m.tasks) == 0 {
		b.WriteString("(no tasks)")
	} else {
		for i, t := range m.tasks {
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
			if i < len(m.tasks)-1 {
				b.WriteString("\n")
			}
		}
	}

	return borderStyle.Width(innerWidth).Render(b.String())
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
