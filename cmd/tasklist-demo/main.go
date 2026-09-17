// Command tasklist-demo is a throwaway manual-QA harness for chunk 4
// (internal/ui/tasklist). It runs the tasklist panel standalone against
// fixture data so a human can visually verify rendering and navigation
// without any live taskwarrior wiring (that's chunk 5).
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
)

type demoModel struct {
	list tasklist.Model
}

func (m demoModel) Init() tea.Cmd {
	return nil
}

func (m demoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	m.list, _ = m.list.Update(msg)
	return m, nil
}

func (m demoModel) View() string {
	return m.list.View() + "\n\nup/k, down/j to move; q or ctrl+c to quit\n"
}

func main() {
	fixtures := []taskwarrior.Task{
		{ID: 1, Description: "Buy groceries", Project: "Home", Priority: "H", Due: "2026-09-20"},
		{ID: 2, Description: "Write quarterly report", Project: "Work", Priority: "M"},
		{ID: 3, Description: "Water plants", Project: "Home"},
		{ID: 4, Description: "Fix leaky faucet", Project: "Home", Priority: "L", Due: "2026-10-01"},
	}

	m := demoModel{list: tasklist.New(fixtures)}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
