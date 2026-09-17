package tasklist_test

import (
	"fmt"

	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
)

// Example demonstrates constructing the task list panel with fixture data
// and reading back the initial selection. For a visual look at the
// rendered, bordered panel (including the ANSI styling, which doesn't
// render cleanly as a deterministic Example output), run:
//
//	go test -v -run TestModel_View ./internal/ui/tasklist
func Example() {
	m := tasklist.New([]taskwarrior.Task{
		{ID: 1, Description: "Buy groceries", Project: "Home", Priority: "H", Due: "2026-09-20"},
		{ID: 2, Description: "Write report", Project: "Work", Priority: "M"},
	})

	selected, _ := m.Selected()
	fmt.Println(selected.ID, selected.Description)
	// Output:
	// 1 Buy groceries
}
