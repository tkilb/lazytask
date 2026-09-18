package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/addform"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
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

// tasksLoadedMsg carries the result of a successful task fetch.
type tasksLoadedMsg struct {
	tasks []taskwarrior.Task
}

// tasksErrMsg carries the error from a failed task fetch.
type tasksErrMsg struct {
	err error
}

// taskAddedMsg carries the result of a successful Add call.
type taskAddedMsg struct{}

// taskAddErrMsg carries the error from a failed Add call.
type taskAddErrMsg struct {
	err error
}

type model struct {
	reader   TaskReader
	adder    TaskAdder
	list     tasklist.Model
	add      addform.Model
	adding   bool
	err      error
	quitting bool
}

func initialModel() model {
	client := taskwarrior.NewClient()
	return model{
		reader: client,
		adder:  client,
		list:   tasklist.New(nil),
		add:    addform.New(),
	}
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
		if _, err := adder.Add(context.Background(), description); err != nil {
			return taskAddErrMsg{err: err}
		}
		return taskAddedMsg{}
	}
}

func (m model) Init() tea.Cmd {
	return fetchTasks(m.reader)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.adding {
		return m.updateAdding(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "r":
			return m, fetchTasks(m.reader)
		case "a":
			m.adding = true
			m.err = nil
			m.add = m.add.Focus()
			return m, m.add.Init()
		}
	case tasksLoadedMsg:
		m.err = nil
		m.list = m.list.SetTasks(msg.tasks)
		return m, nil
	case tasksErrMsg:
		m.err = msg.err
		return m, nil
	case taskAddedMsg:
		m.err = nil
		m.add = m.add.Reset()
		return m, fetchTasks(m.reader)
	case taskAddErrMsg:
		m.err = msg.err
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
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
		view += "\n(enter) add  (esc) cancel\n"
		return view
	}

	view := m.list.View()
	if m.err != nil {
		view += fmt.Sprintf("\nerror: %v\n", m.err)
	}
	view += "\n(a) add  (r) refresh  (q) quit\n"
	return view
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running lazytask: %v\n", err)
		os.Exit(1)
	}
}
