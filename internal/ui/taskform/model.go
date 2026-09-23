// Package taskform implements the lazygit-style multi-field "Add Task"
// panel: Description, Project, Priority, and Due Date fields, each its own
// bordered box (matching the look of the Tasks/Projects grid panels), with
// the currently focused field's border highlighted, tab-navigable, and
// submitted as one unit rather than as a sequence of separate popups. This
// package has no taskwarrior dependency: the host model reads the resolved
// field values when it decides the form has been submitted (e.g. on an
// enter keypress) and issues the Add call itself.
package taskform

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tkilb/lazytask/internal/ui/datepick"
	"github.com/tkilb/lazytask/internal/ui/panel"
)

// Field identifies one of the form's inputs.
type Field int

const (
	FieldDescription Field = iota
	FieldProject
	FieldPriority
	FieldDueDate

	fieldCount
)

// fieldTitles are the border titles for each field's box, in Field order.
var fieldTitles = [fieldCount]string{
	FieldDescription: "Description",
	FieldProject:     "Project",
	FieldPriority:    "Priority",
	FieldDueDate:     "Due Date",
}

var (
	previewStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	previewErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196"))

	// flatStyle replaces textarea's default focused-line highlight so the
	// Description box's styling stays consistent with the other (plain,
	// unhighlighted) fields.
	flatStyle = lipgloss.NewStyle()
)

// minPanelWidth is used when no tea.WindowSizeMsg has been received yet.
const minPanelWidth = 80

// descriptionRows is the number of visible rows in the Description field's
// textarea: long descriptions wrap and scroll within this fixed-height
// viewport rather than growing the box.
const descriptionRows = 2

// hintText is embedded as a footer in the last (Due Date) field's box,
// lazygit-style, so the key-binding hint doesn't need its own row.
const hintText = "<tab> next field, <enter> to add, <esc> cancel"

// previewLayout mirrors internal/ui/datepick's preview format for the Due
// Date field's live-resolved-date line.
const previewLayout = "Mon Jan 2 2006"

// Model is a Bubble Tea model rendering the Add Task form as a stack of
// individually bordered field boxes.
type Model struct {
	description textarea.Model
	project     textinput.Model
	priority    textinput.Model
	due         textinput.Model
	focused     Field
	width       int

	// now returns the reference instant the Due Date field's cord is
	// resolved relative to. Defaults to time.Now but is overridable so
	// tests get deterministic output.
	now func() time.Time
}

// New constructs a Model with all fields empty, focused on Description.
func New() Model {
	m := Model{now: time.Now}

	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.Placeholder = "Task description..."
	ta.CharLimit = 256
	ta.SetHeight(descriptionRows)
	ta.FocusedStyle.CursorLine = flatStyle
	ta.FocusedStyle.CursorLineNumber = flatStyle
	m.description = ta
	// Give the textarea a real width up front (matching the minPanelWidth
	// fallback View() uses before any tea.WindowSizeMsg arrives), so its
	// internal cursor-follow scrolling works correctly even if View() is
	// called before the first Update (see the comment in Update).
	m.description.SetWidth(m.descriptionInnerWidth())

	projectInput := textinput.New()
	projectInput.Prompt = ""
	projectInput.Placeholder = "(none)"
	projectInput.CharLimit = 256
	m.project = projectInput

	priorityInput := textinput.New()
	priorityInput.Prompt = ""
	priorityInput.Placeholder = "H/M/L, blank = M"
	priorityInput.CharLimit = 1
	m.priority = priorityInput

	dueInput := textinput.New()
	dueInput.Prompt = ""
	dueInput.Placeholder = "e.g. 2d, 1w, 2b, 3/14, or 2026-03-14"
	dueInput.CharLimit = 32
	m.due = dueInput

	return m
}

// WithNow returns a copy of m using now in place of time.Now for resolving
// the Due Date field's cord, so callers (tests, or a host model wanting a
// fixed "current time" for the whole session) get deterministic previews.
func (m Model) WithNow(now func() time.Time) Model {
	m.now = now
	return m
}

// Focus clears every field and gives keyboard focus to Description, ready
// for a brand new task.
func (m Model) Focus() Model {
	m.description.Reset()
	m.description.Blur()
	m.project.Reset()
	m.project.Blur()
	m.priority.Reset()
	m.priority.Blur()
	m.due.Reset()
	m.due.Blur()

	m.focused = FieldDescription
	m.description.Focus()
	return m
}

// Reset clears every field's text without changing focus state.
func (m Model) Reset() Model {
	m.description.Reset()
	m.project.Reset()
	m.priority.Reset()
	m.due.Reset()
	return m
}

// Blur removes keyboard focus from every field.
func (m Model) Blur() Model {
	m.description.Blur()
	m.project.Blur()
	m.priority.Blur()
	m.due.Blur()
	return m
}

// Focused reports whether the form currently has keyboard focus.
func (m Model) Focused() bool {
	switch m.focused {
	case FieldDescription:
		return m.description.Focused()
	case FieldProject:
		return m.project.Focused()
	case FieldPriority:
		return m.priority.Focused()
	default:
		return m.due.Focused()
	}
}

// FocusedField reports which field currently has keyboard focus.
func (m Model) FocusedField() Field {
	return m.focused
}

// SetProject replaces the Project field's text (e.g. pre-filling it with
// the currently active project filter when the form is opened), moving
// the cursor to the end.
func (m Model) SetProject(v string) Model {
	m.project.SetValue(v)
	m.project.CursorEnd()
	return m
}

// Description returns the current (untrimmed) Description field text.
func (m Model) Description() string {
	return m.description.Value()
}

// Project returns the current (untrimmed) Project field text.
func (m Model) Project() string {
	return m.project.Value()
}

// Priority returns the trimmed, upper-cased Priority field text and
// whether it's a valid value: empty (caller decides the default), "H",
// "M", or "L".
func (m Model) Priority() (value string, ok bool) {
	value = strings.ToUpper(strings.TrimSpace(m.priority.Value()))
	switch value {
	case "", "H", "M", "L":
		return value, true
	default:
		return value, false
	}
}

// DueInput returns the current (untrimmed) Due Date field text.
func (m Model) DueInput() string {
	return m.due.Value()
}

// ResolveDue parses and resolves the Due Date field's current text via
// datepick.ResolveInput (the same cord/absolute-date parsing the 'D'
// due-date reassign popup uses). ok is false if it doesn't parse
// (including an empty input) — callers should check DueInput for
// emptiness first if they want to distinguish "no due date entered" from
// "invalid due date".
func (m Model) ResolveDue() (time.Time, bool) {
	nowFn := m.now
	if nowFn == nil {
		nowFn = time.Now
	}
	return datepick.ResolveInput(m.due.Value(), nowFn())
}

// next/prev wrap around the fixed set of fields.
func (f Field) next() Field { return (f + 1) % fieldCount }
func (f Field) prev() Field { return (f - 1 + fieldCount) % fieldCount }

// focusField moves keyboard focus to f, blurring whichever field
// previously had it.
func (m Model) focusField(f Field) Model {
	m.description.Blur()
	m.project.Blur()
	m.priority.Blur()
	m.due.Blur()

	m.focused = f
	switch f {
	case FieldDescription:
		m.description.Focus()
	case FieldProject:
		m.project.Focus()
	case FieldPriority:
		m.priority.Focus()
	case FieldDueDate:
		m.due.Focus()
	}
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model. Tab/shift+tab cycle focus between fields
// (deliberately not up/down, since the Description field is a real
// multi-line textarea that needs up/down for its own cursor movement);
// every other message (including plain text entry) is forwarded to the
// currently focused field. It does not interpret enter/esc itself, since
// those keys are also meaningful to the host model (submit/cancel), which
// retains full control over the current mode/focus.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m = m.focusField(m.focused.next())
			return m, textinput.Blink
		case "shift+tab":
			m = m.focusField(m.focused.prev())
			return m, textinput.Blink
		}
	}

	// The Description textarea's own cursor-follow scrolling (used once
	// content wraps past the visible height) relies on its internal width
	// matching what View() actually renders at. View() only ever sets the
	// width on a throwaway render-time copy, so it must be kept in sync
	// here too, or the textarea scrolls itself using a stale (zero) width
	// and ends up showing blank lines instead of the wrapped content.
	m.description.SetWidth(m.descriptionInnerWidth())

	var cmd tea.Cmd
	switch m.focused {
	case FieldDescription:
		m.description, cmd = m.description.Update(msg)
	case FieldProject:
		m.project, cmd = m.project.Update(msg)
	case FieldPriority:
		m.priority, cmd = m.priority.Update(msg)
	case FieldDueDate:
		m.due, cmd = m.due.Update(msg)
	}
	return m, cmd
}

// View implements tea.Model.
func (m Model) View() string {
	width := m.width
	if width <= 0 {
		width = minPanelWidth
	}

	boxes := []string{
		m.descriptionBox(width),
		m.simpleFieldBox(FieldProject, m.project.View(), width),
		m.simpleFieldBox(FieldPriority, m.priority.View(), width),
		m.dueDateBox(width),
	}
	return strings.Join(boxes, "\n")
}

// descriptionInnerWidth returns the Description textarea's current inner
// (content) width, mirroring the outer-to-inner sizing descriptionBox uses
// for rendering, so Update can keep the textarea's own width in sync (see
// the comment in Update for why this matters).
func (m Model) descriptionInnerWidth() int {
	width := m.width
	if width <= 0 {
		width = minPanelWidth
	}
	innerWidth, _ := panel.InnerSize(width, descriptionRows+2)
	return innerWidth
}

// descriptionBox renders the Description field as its own lazygit-style
// bordered panel.Frame box, descriptionRows tall, so long descriptions
// wrap/scroll within a fixed-height viewport rather than growing the box.
func (m Model) descriptionBox(outerWidth int) string {
	innerWidth, innerHeight := panel.InnerSize(outerWidth, descriptionRows+2)
	return panel.Frame(fieldTitles[FieldDescription], m.description.View(), innerWidth, innerHeight, m.focused == FieldDescription, "")
}

// simpleFieldBox renders a single-line textinput-backed field (Project,
// Priority) as its own lazygit-style bordered panel.Frame box.
func (m Model) simpleFieldBox(f Field, body string, outerWidth int) string {
	innerWidth, innerHeight := panel.InnerSize(outerWidth, 1+2)
	return panel.Frame(fieldTitles[f], body, innerWidth, innerHeight, f == m.focused, "")
}

// dueDateBox renders the Due Date field as its own lazygit-style bordered
// panel.Frame box, with the live-resolved-date preview line beneath the
// input, and the form's key-binding hint embedded as this (last) box's
// footer.
func (m Model) dueDateBox(outerWidth int) string {
	innerWidth, innerHeight := panel.InnerSize(outerWidth, 2+2)
	body := m.due.View() + "\n" + m.duePreviewLine()
	return panel.Frame(fieldTitles[FieldDueDate], body, innerWidth, innerHeight, m.focused == FieldDueDate, hintText)
}

// duePreviewLine renders the Due Date field's live-resolved-date preview,
// matching internal/ui/datepick's preview line.
func (m Model) duePreviewLine() string {
	value := m.due.Value()
	if value == "" {
		return previewStyle.Render("Enter shorthand (2d, 2b, 1w) or a date, or leave blank")
	}
	if resolved, ok := m.ResolveDue(); ok {
		return previewStyle.Render(resolved.Format(previewLayout))
	}
	return previewErrorStyle.Render("Invalid input (expected <N>d/w/b, M-D, or a full date)")
}
