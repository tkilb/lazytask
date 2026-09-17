package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
)

// TaskReader is the subset of the taskwarrior client this model depends on,
// so it can be stubbed out in tests without shelling out to the real `task`
// binary.
type TaskReader interface {
	Export(ctx context.Context, filters ...string) ([]taskwarrior.Task, error)
}

// tasksLoadedMsg carries the result of a successful task fetch.
type tasksLoadedMsg struct {
	tasks []taskwarrior.Task
}

// tasksErrMsg carries the error from a failed task fetch.
type tasksErrMsg struct {
	err error
}

type model struct {
	reader   TaskReader
	list     tasklist.Model
	err      error
	quitting bool
}

func initialModel() model {
	return model{
		reader: taskwarrior.NewClient(),
		list:   tasklist.New(nil),
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

func (m model) Init() tea.Cmd {
	return fetchTasks(m.reader)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "r":
			return m, fetchTasks(m.reader)
		}
	case tasksLoadedMsg:
		m.err = nil
		m.list = m.list.SetTasks(msg.tasks)
		return m, nil
	case tasksErrMsg:
		m.err = msg.err
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return "Exiting lazytask...\n"
	}

	view := m.list.View()
	if m.err != nil {
		view += fmt.Sprintf("\nerror loading tasks: %v\n", m.err)
	}
	view += "\n(r) refresh  (q) quit\n"
	return view
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running lazytask: %v\n", err)
		os.Exit(1)
	}
}
