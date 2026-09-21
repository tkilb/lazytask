package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tkilb/lazytask/internal/editbuffer"
	"github.com/tkilb/lazytask/internal/editor"
	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/addform"
	"github.com/tkilb/lazytask/internal/ui/panel"
	"github.com/tkilb/lazytask/internal/ui/statusbar"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
)

// panelFocus identifies which panel in the grid currently has keyboard
// focus. focusTasks is the zero value so a model constructed without going
// through initialModel (as most tests do) keeps the pre-grid behavior of
// routing navigation keys straight to the Tasks list.
type panelFocus int

const (
	focusTasks panelFocus = iota
	focusStatus
	focusProjects
	focusTags
	focusDetails
)

// panelKeyBindings maps the number keys shown in the panel titles to the
// panel they focus.
var panelKeyBindings = map[string]panelFocus{
	"0": focusDetails,
	"1": focusStatus,
	"2": focusTasks,
	"3": focusProjects,
	"4": focusTags,
}

// focusCycle is the Tab/Shift+Tab traversal order: top-to-bottom through
// the left column, then the right column's single large panel.
var focusCycle = []panelFocus{focusStatus, focusTasks, focusProjects, focusTags, focusDetails}

// panelTitle returns the display title for a given panel, including its
// number-key hint (lazygit-style).
func panelTitle(f panelFocus) string {
	switch f {
	case focusStatus:
		return "1 Status"
	case focusProjects:
		return "3 Projects"
	case focusTags:
		return "4 Tags"
	case focusDetails:
		return "0 Details"
	default:
		return ""
	}
}

func nextFocus(f panelFocus) panelFocus {
	for i, c := range focusCycle {
		if c == f {
			return focusCycle[(i+1)%len(focusCycle)]
		}
	}
	return focusCycle[0]
}

func prevFocus(f panelFocus) panelFocus {
	for i, c := range focusCycle {
		if c == f {
			return focusCycle[(i-1+len(focusCycle))%len(focusCycle)]
		}
	}
	return focusCycle[0]
}

const (
	// defaultGridWidth/defaultGridHeight size the panel grid before the
	// first tea.WindowSizeMsg arrives (e.g. in tests that call View()
	// directly without going through a real terminal).
	defaultGridWidth  = 100
	defaultGridHeight = 30

	// gridBottomOverhead is the number of terminal rows View() renders
	// below the grid (a blank line plus the status bar's single line of
	// key hints). It must be subtracted from the real terminal height
	// before sizing the grid, otherwise the grid plus this trailing
	// content overflows the terminal and the top of the grid (the Status
	// and Details panels' top borders, both on the grid's first row)
	// scrolls off-screen.
	gridBottomOverhead = 2
)

// Keybinding sets shown in the status bar for each panel/mode. Kept in one
// place so the bar and the actual key handling in Update don't drift apart.
var (
	listBindings = []statusbar.Binding{
		{Key: "↑/k", Label: "up"},
		{Key: "↓/j", Label: "down"},
		{Key: "0-4/tab", Label: "panels"},
		{Key: "a", Label: "add"},
		{Key: "d", Label: "done"},
		{Key: "x", Label: "delete"},
		{Key: "e", Label: "edit"},
		{Key: "r", Label: "refresh"},
		{Key: "q", Label: "quit"},
	}

	addBindings = []statusbar.Binding{
		{Key: "enter", Label: "add"},
		{Key: "esc", Label: "cancel"},
	}

	deleteBindings = []statusbar.Binding{
		{Key: "y", Label: "confirm"},
		{Key: "n/esc", Label: "cancel"},
	}
)

// TaskReader is the subset of the taskwarrior client this model depends on,
// so it can be stubbed out in tests without shelling out to the real `task`
// binary.
type TaskReader interface {
	Export(ctx context.Context, filters ...string) ([]taskwarrior.Task, error)
}

// TaskAdder is the subset of the taskwarrior client needed to create new
// tasks, so it can be stubbed out in tests without shelling out to the
// real `task` binary.
type TaskAdder interface {
	Add(ctx context.Context, description string, extraArgs ...string) (int, error)
}

// TaskDoner is the subset of the taskwarrior client needed to mark a task
// done, so it can be stubbed out in tests without shelling out to the real
// `task` binary.
type TaskDoner interface {
	Done(ctx context.Context, id string) error
}

// TaskDeleter is the subset of the taskwarrior client needed to delete a
// task, so it can be stubbed out in tests without shelling out to the real
// `task` binary.
type TaskDeleter interface {
	Delete(ctx context.Context, id string) error
}

// TaskImporter is the subset of the taskwarrior client needed to re-import
// an edited task, so it can be stubbed out in tests without shelling out to
// the real `task` binary.
type TaskImporter interface {
	Import(ctx context.Context, data []byte) error
}

// tasksLoadedMsg carries the result of a successful task fetch.
type tasksLoadedMsg struct {
	tasks []taskwarrior.Task
}

// tasksErrMsg carries the error from a failed task fetch.
type tasksErrMsg struct {
	err error
}

// taskAddedMsg carries the result of a successful Add call, including the
// numeric ID Taskwarrior assigned so the list can focus it once the
// following refresh completes.
type taskAddedMsg struct {
	id int
}

// taskAddErrMsg carries the error from a failed Add call.
type taskAddErrMsg struct {
	err error
}

// taskDoneMsg carries the result of a successful Done call.
type taskDoneMsg struct{}

// taskDoneErrMsg carries the error from a failed Done call.
type taskDoneErrMsg struct {
	err error
}

// taskDeletedMsg carries the result of a successful Delete call.
type taskDeletedMsg struct{}

// taskDeleteErrMsg carries the error from a failed Delete call.
type taskDeleteErrMsg struct {
	err error
}

// taskEditedMsg carries the result of a successful edit-and-reimport.
type taskEditedMsg struct{}

// taskEditErrMsg carries the error from a failed edit-and-reimport, whether
// from the editor invocation itself, parsing its output, or re-importing.
type taskEditErrMsg struct {
	err error
}

type model struct {
	reader         TaskReader
	adder          TaskAdder
	doner          TaskDoner
	deleter        TaskDeleter
	importer       TaskImporter
	list           tasklist.Model
	add            addform.Model
	adding         bool
	deleting       bool
	err            error
	quitting       bool
	pendingFocusID int
	focus          panelFocus
	width          int
	height         int
}

func initialModel() model {
	client := taskwarrior.NewClient()
	return model{
		reader:   client,
		adder:    client,
		doner:    client,
		deleter:  client,
		importer: client,
		list:     tasklist.New(nil).SetFocused(true),
		add:      addform.New(),
	}
}

// setFocus updates which panel has focus, keeping the Tasks list's own
// focused flag (used for its border highlight) in sync.
func (m model) setFocus(f panelFocus) model {
	m.focus = f
	m.list = m.list.SetFocused(f == focusTasks)
	return m
}

// gridDims computes the panel grid's column widths and left-column row
// height from the model's last known terminal size, falling back to sane
// defaults if no tea.WindowSizeMsg has been received yet (e.g. in tests
// that call View directly).
func (m model) gridDims() (leftWidth, rightWidth, rowHeight, fullHeight int) {
	width := m.width
	if width <= 0 {
		width = defaultGridWidth
	}
	height := m.height
	if height <= 0 {
		height = defaultGridHeight
	} else if height > gridBottomOverhead {
		height -= gridBottomOverhead
	}
	leftWidth = width / 2
	rightWidth = width - leftWidth
	rowHeight = height / 4
	fullHeight = height
	return leftWidth, rightWidth, rowHeight, fullHeight
}

// fetchTasks returns a tea.Cmd that loads pending tasks via reader.
func fetchTasks(reader TaskReader) tea.Cmd {
	return func() tea.Msg {
		tasks, err := reader.Export(context.Background(), "status:pending")
		if err != nil {
			return tasksErrMsg{err: err}
		}
		return tasksLoadedMsg{tasks: tasks}
	}
}

// addTask returns a tea.Cmd that creates a new task via adder.
func addTask(adder TaskAdder, description string) tea.Cmd {
	return func() tea.Msg {
		id, err := adder.Add(context.Background(), description)
		if err != nil {
			return taskAddErrMsg{err: err}
		}
		return taskAddedMsg{id: id}
	}
}

// doneTask returns a tea.Cmd that marks the given task id as done via doner.
func doneTask(doner TaskDoner, id string) tea.Cmd {
	return func() tea.Msg {
		if err := doner.Done(context.Background(), id); err != nil {
			return taskDoneErrMsg{err: err}
		}
		return taskDoneMsg{}
	}
}

// deleteTask returns a tea.Cmd that deletes the given task id via deleter.
func deleteTask(deleter TaskDeleter, id string) tea.Cmd {
	return func() tea.Msg {
		if err := deleter.Delete(context.Background(), id); err != nil {
			return taskDeleteErrMsg{err: err}
		}
		return taskDeletedMsg{}
	}
}

// editTask opens task in the user's $EDITOR as a structured plain-text
// buffer (see internal/editbuffer), similar in spirit to `task <id> edit`,
// then re-imports the edited fields via importer once the editor exits. It
// suspends the Bubble Tea program for the duration of the editor
// invocation via tea.ExecProcess.
func editTask(importer TaskImporter, task taskwarrior.Task) tea.Cmd {
	cmd, session, err := editor.Prepare(editbuffer.Serialize(task))
	if err != nil {
		return func() tea.Msg { return taskEditErrMsg{err: err} }
	}

	return tea.ExecProcess(cmd, editTaskCallback(importer, session, task))
}

// editTaskCallback builds the tea.ExecCallback run once the editor process
// launched by editTask exits: it reads back session's temp file, parses
// its editable fields, applies them onto original (preserving read-only
// fields such as UUID untouched, regardless of what the user may have
// typed in that section of the buffer), and re-imports the result via
// importer. Split out from editTask so it can be unit-tested without going
// through tea.ExecProcess/a real editor process.
func editTaskCallback(importer TaskImporter, session *editor.Session, original taskwarrior.Task) func(error) tea.Msg {
	return func(err error) tea.Msg {
		defer session.Close()
		if err != nil {
			return taskEditErrMsg{err: fmt.Errorf("running editor: %w", err)}
		}

		edited, err := session.Read()
		if err != nil {
			return taskEditErrMsg{err: err}
		}

		fields, err := editbuffer.Parse(edited)
		if err != nil {
			return taskEditErrMsg{err: fmt.Errorf("parsing edited task: %w", err)}
		}
		if strings.TrimSpace(original.UUID) == "" {
			return taskEditErrMsg{err: fmt.Errorf("edited task is missing its uuid; not importing")}
		}
		updated := editbuffer.Apply(original, fields)

		data, err := json.Marshal(updated)
		if err != nil {
			return taskEditErrMsg{err: err}
		}
		if err := importer.Import(context.Background(), data); err != nil {
			return taskEditErrMsg{err: err}
		}
		return taskEditedMsg{}
	}
}

func (m model) Init() tea.Cmd {
	return fetchTasks(m.reader)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.adding {
		return m.updateAdding(msg)
	}
	if m.deleting {
		return m.updateDeleting(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		leftWidth, _, rowHeight, _ := m.gridDims()
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(tea.WindowSizeMsg{Width: leftWidth, Height: rowHeight})
		return m, cmd
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "0", "1", "2", "3", "4":
			m = m.setFocus(panelKeyBindings[msg.String()])
			return m, nil
		case "tab":
			m = m.setFocus(nextFocus(m.focus))
			return m, nil
		case "shift+tab":
			m = m.setFocus(prevFocus(m.focus))
			return m, nil
		case "r":
			return m, fetchTasks(m.reader)
		case "a":
			m.adding = true
			m.err = nil
			m.add = m.add.Focus()
			return m, m.add.Init()
		case "d":
			m.err = nil
			if task, ok := m.list.Selected(); ok {
				return m, doneTask(m.doner, taskID(task))
			}
			return m, nil
		case "x":
			if _, ok := m.list.Selected(); ok {
				m.err = nil
				m.deleting = true
			}
			return m, nil
		case "e":
			m.err = nil
			if task, ok := m.list.Selected(); ok {
				return m, editTask(m.importer, task)
			}
			return m, nil
		}
	case tasksLoadedMsg:
		m.err = nil
		m.list = m.list.SetTasks(msg.tasks)
		if m.pendingFocusID != 0 {
			m.list = m.list.SelectID(m.pendingFocusID)
			m.pendingFocusID = 0
		}
		return m, nil
	case tasksErrMsg:
		m.err = msg.err
		return m, nil
	case taskAddedMsg:
		m.err = nil
		m.add = m.add.Reset()
		m.pendingFocusID = msg.id
		return m, fetchTasks(m.reader)
	case taskAddErrMsg:
		m.err = msg.err
		return m, nil
	case taskDoneMsg:
		m.err = nil
		return m, fetchTasks(m.reader)
	case taskDoneErrMsg:
		m.err = msg.err
		return m, nil
	case taskDeletedMsg:
		m.err = nil
		return m, fetchTasks(m.reader)
	case taskDeleteErrMsg:
		m.err = msg.err
		return m, nil
	case taskEditedMsg:
		m.err = nil
		return m, fetchTasks(m.reader)
	case taskEditErrMsg:
		m.err = msg.err
		return m, nil
	}

	// Remaining messages (e.g. up/down/j/k navigation) only apply to the
	// Tasks panel, and only reach it while it has focus.
	if m.focus == focusTasks {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	return m, nil
}

// taskID returns the identifier used to address t in taskwarrior mutation
// commands, preferring the stable UUID over the pending numeric ID (which
// can be renumbered by taskwarrior after other tasks complete/are deleted).
func taskID(t taskwarrior.Task) string {
	if t.UUID != "" {
		return t.UUID
	}
	return fmt.Sprintf("%d", t.ID)
}

// updateDeleting handles messages while a delete confirmation is pending.
func (m model) updateDeleting(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y":
			m.deleting = false
			task, ok := m.list.Selected()
			if !ok {
				return m, nil
			}
			return m, deleteTask(m.deleter, taskID(task))
		case "n", "esc":
			m.deleting = false
			return m, nil
		}
	}
	return m, nil
}

// updateAdding handles messages while the add-task input panel is focused.
func (m model) updateAdding(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.adding = false
			m.add = m.add.Blur()
			return m, nil
		case "enter":
			description := strings.TrimSpace(m.add.Value())
			if description == "" {
				return m, nil
			}
			m.adding = false
			m.add = m.add.Blur()
			return m, addTask(m.adder, description)
		}
	}

	var cmd tea.Cmd
	m.add, cmd = m.add.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return "Exiting lazytask...\n"
	}

	if m.adding {
		view := m.add.View()
		view += "\n" + statusbar.Render(addBindings) + "\n"
		return view
	}

	view := m.renderGrid()
	if m.deleting {
		if task, ok := m.list.Selected(); ok {
			view += fmt.Sprintf("\nDelete task %d %q? (y/n)\n", task.ID, task.Description)
		}
		view += statusbar.Render(deleteBindings) + "\n"
		return view
	}
	if m.err != nil {
		view += fmt.Sprintf("\nerror: %v\n", m.err)
	}
	view += "\n" + statusbar.Render(listBindings) + "\n"
	return view
}

// renderGrid lays out the lazygit-style panel grid: a left column of 4
// stacked panels (Status, Tasks, Projects, Tags) and one large panel
// (Details) filling the right column. Only the Tasks panel (slot 2) has
// real content so far; the rest are placeholders until later chunks.
func (m model) renderGrid() string {
	leftWidth, rightWidth, rowHeight, fullHeight := m.gridDims()

	status := panel.Render(panelTitle(focusStatus), "(coming soon)", leftWidth, rowHeight, m.focus == focusStatus)
	tasksPanel := m.list.View()
	projects := panel.Render(panelTitle(focusProjects), "(coming soon)", leftWidth, rowHeight, m.focus == focusProjects)
	tags := panel.Render(panelTitle(focusTags), "(coming soon)", leftWidth, rowHeight, m.focus == focusTags)

	leftCol := lipgloss.JoinVertical(lipgloss.Left, status, tasksPanel, projects, tags)

	details := panel.Render(panelTitle(focusDetails), "(coming soon)", rightWidth, fullHeight, m.focus == focusDetails)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftCol, details)
}

func main() {
	// Use the alternate screen so the program owns the full terminal
	// buffer. Without it, resuming after an external process (e.g. the
	// editor launched by the 'e' key, via tea.ExecProcess) just repaints
	// the current view in place rather than clearing first, leaving old
	// frames stacked above the live one.
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running lazytask: %v\n", err)
		os.Exit(1)
	}
}
