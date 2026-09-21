// Package addform implements the lazygit-style input panel used to prompt
// for a new task description before it is created via the taskwarrior
// client.
package addform

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	borderColor = lipgloss.Color("62")

	// bodyStyle draws the left/right/bottom border only; the top border is
	// built by hand in View() so the title and key hints can be embedded in
	// it, lazygit-style.
	bodyStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderTop(false).
			BorderForeground(borderColor)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(borderColor)

	hintStyle = lipgloss.NewStyle().
			Foreground(borderColor)
)

// minPanelWidth is used when no tea.WindowSizeMsg has been received yet.
const minPanelWidth = 80

const hintText = "Press <enter> to add, <esc> to cancel"

// Model is a Bubble Tea model rendering a single-line bordered text input
// panel for entering a new task description. This package has no
// taskwarrior dependency: the host model reads Value() when it decides the
// input has been submitted (e.g. on an enter keypress) and issues the Add
// call itself.
type Model struct {
	input textinput.Model
	width int
	title string
	hint  string
}

// New constructs a Model with an empty input, titled "Add Task" for
// prompting a new task description.
func New() Model {
	return NewNamed("Add Task", hintText, "Task description...")
}

// NewNamed constructs a Model with an empty input, using title/hint/
// placeholder in place of the "Add Task" defaults, so this same
// bordered-input-box component can be reused for other single-line
// prompts (e.g. renaming a project).
func NewNamed(title, hint, placeholder string) Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = placeholder
	ti.CharLimit = 256
	return Model{input: ti, title: title, hint: hint}
}

// Focus gives the input keyboard focus and clears any prior text.
func (m Model) Focus() Model {
	m.input.Reset()
	m.input.Focus()
	return m
}

// Blur removes keyboard focus from the input.
func (m Model) Blur() Model {
	m.input.Blur()
	return m
}

// Focused reports whether the input currently has keyboard focus.
func (m Model) Focused() bool {
	return m.input.Focused()
}

// Value returns the current (untrimmed) input text.
func (m Model) Value() string {
	return m.input.Value()
}

// SetValue replaces the input text (e.g. pre-filling it with an existing
// name to edit), moving the cursor to the end.
func (m Model) SetValue(s string) Model {
	m.input.SetValue(s)
	m.input.CursorEnd()
	return m
}

// Reset clears the input text.
func (m Model) Reset() Model {
	m.input.Reset()
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model. It only updates the underlying text input;
// it does not interpret enter/esc itself, since those keys are also
// meaningful to the host model (submit/cancel), and the host model retains
// full control over the current mode/focus.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m Model) View() string {
	width := m.width
	if width <= 0 {
		width = minPanelWidth
	}
	innerWidth := width - 2
	if innerWidth < 16 {
		innerWidth = 16
	}

	body := bodyStyle.Width(innerWidth).Render(m.input.View())
	return m.topBorder(innerWidth) + "\n" + body
}

// topBorder builds the box's top edge with the title embedded on the left
// and the key-binding hint embedded on the right, lazygit-style, e.g.
// "╭─ Add Task ─────── Press <enter> to add, <esc> to cancel ─╮".
func (m Model) topBorder(innerWidth int) string {
	border := lipgloss.RoundedBorder()

	title := " " + titleStyle.Render(m.title) + " "
	hint := " " + hintStyle.Render(m.hint) + " "

	// totalWidth is the full rendered width, including the two corner
	// runes, matching bodyStyle.Width(innerWidth)'s rendered width.
	totalWidth := innerWidth + 2
	fillWidth := totalWidth - 2 - lipgloss.Width(title) - lipgloss.Width(hint)
	if fillWidth < 1 {
		fillWidth = 1
	}

	var b strings.Builder
	b.WriteString(border.TopLeft)
	b.WriteString(title)
	b.WriteString(strings.Repeat(border.Top, fillWidth))
	b.WriteString(hint)
	b.WriteString(border.TopRight)
	return b.String()
}
