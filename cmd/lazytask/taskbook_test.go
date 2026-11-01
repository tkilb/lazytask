package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/projects"
	"github.com/tkilb/lazytask/internal/ui/tags"
	"github.com/tkilb/lazytask/internal/ui/taskbooks"
	"github.com/tkilb/lazytask/internal/ui/taskform"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
)

func taskbookTestModel(tasks []taskwarrior.Task, knownBooks ...string) model {
	return model{
		list:                 tasklist.New(tasks),
		focus:                focusTasks,
		reader:               &stubReader{},
		projects:             projects.New(),
		tags:                 tags.New(),
		taskbookPicker:       taskbooks.New().SetTaskbooks(knownBooks),
	}
}

func TestTaskbook_PickerOpenAndSelect(t *testing.T) {
	m := taskbookTestModel(nil, "work", "personal")
	assert.False(t, m.pickingTaskbook)

	// Press "B" globally
	m, _ = key(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'B'}})
	assert.True(t, m.pickingTaskbook)

	// Move cursor down from (all) to "work"
	m, _ = key(m, tea.KeyMsg{Type: tea.KeyDown})

	sel, ok := m.taskbookPicker.Selected()
	require.True(t, ok)
	assert.Equal(t, "work", sel)

	// Press Enter to apply
	m, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, m.pickingTaskbook)
	require.NotNil(t, m.filter.taskbook)
	assert.Equal(t, "work", *m.filter.taskbook)
	assert.NotNil(t, cmd)
}

func TestTaskbook_AddingScopedTaskbook(t *testing.T) {
	adder := &stubAdder{}
	annotator := &stubAnnotator{}
	work := "work"
	m := model{
		adder:     adder,
		annotator: annotator,
		filter:    filterState{taskbook: &work},
		list:      tasklist.New(nil),
		add:       taskform.New().Focus(),
		adding:    true,
	}
	m = typeText(m, "New work item")

	// Press enter on add form
	m, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, m.adding)
	require.NotNil(t, cmd)

	cmd()
	require.Len(t, adder.descriptions, 1)
	require.Len(t, adder.extraArgs, 1)
	assert.Contains(t, adder.extraArgs[0], "taskbook:work")
}

func TestTaskbook_AddingAllOmitsTaskbook(t *testing.T) {
	adder := &stubAdder{}
	annotator := &stubAnnotator{}

	// When scoped to (all) (taskbook: nil)
	m := model{
		adder:     adder,
		annotator: annotator,
		filter:    filterState{taskbook: nil},
		list:      tasklist.New(nil),
		add:       taskform.New().Focus(),
		adding:    true,
	}
	m = typeText(m, "Inbox item 1")
	m, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	cmd()
	require.Len(t, adder.descriptions, 1)
	for _, arg := range adder.extraArgs[0] {
		assert.False(t, strings.HasPrefix(arg, "taskbook:"))
	}
}

func TestTaskbook_StatusBarAndDetailsOmitted(t *testing.T) {
	work := "work"
	task := taskwarrior.Task{ID: 1, UUID: "u1", Description: "Work task", Status: "pending", Taskbook: "work"}
	m := model{
		list:     tasklist.New([]taskwarrior.Task{task}),
		focus:    focusTasks,
		filter:   filterState{taskbook: &work},
		projects: projects.New(),
		tags:     tags.New(),
	}

	base := m.baseView()
	assert.Contains(t, base, "work")
	assert.NotContains(t, base, "[work]")

	details := m.detailsPanelContent()
	assert.NotContains(t, details, "Taskbook:")
}

func TestTaskbook_PickerScopesTasksProjectsTagsSuggested(t *testing.T) {
	reader := &stubReader{}
	m := model{
		reader:         reader,
		list:           tasklist.New(nil),
		projects:       projects.New(),
		tags:           tags.New(),
		taskbookPicker: taskbooks.New().SetTaskbooks([]string{"work", "personal"}),
	}

	// Open picker
	m, _ = key(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'B'}})
	// Move down to "work"
	m, _ = key(m, tea.KeyMsg{Type: tea.KeyDown})
	// Press Enter
	m, cmd := key(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, m.filter.taskbook)
	assert.Equal(t, "work", *m.filter.taskbook)
	require.NotNil(t, cmd)

	// Execute batched commands
	batchMsg := cmd()
	if batch, ok := batchMsg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c != nil {
				c()
			}
		}
	}

	// Verify that tasks, projects, and tags fetches all contained the taskbook filter
	require.NotEmpty(t, reader.allFilters)
	expectedFilter := []string{"(", "taskbook:work", "or", "taskbook.none:", ")"}
	for _, call := range reader.allFilters {
		assert.Subset(t, call, expectedFilter)
	}

	// Now send tasksLoadedMsg to verify fetchSuggestedTasks is scoped
	reader.allFilters = nil
	_, suggestCmd := m.Update(tasksLoadedMsg{tasks: nil})
	require.NotNil(t, suggestCmd)
	suggestCmd()
	require.Len(t, reader.allFilters, 1)
	assert.Subset(t, reader.allFilters[0], expectedFilter)
}

func TestTaskbook_MutationRefreshesScopeToActiveTaskbook(t *testing.T) {
	reader := &stubReader{}
	work := "work"
	m := model{
		reader:   reader,
		list:     tasklist.New(nil),
		add:      taskform.New(),
		projects: projects.New(),
		tags:     tags.New(),
		filter:   filterState{taskbook: &work},
	}

	// Trigger taskAddedMsg
	_, cmd := m.Update(taskAddedMsg{id: 42})
	require.NotNil(t, cmd)

	batchMsg := cmd()
	if batch, ok := batchMsg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c != nil {
				c()
			}
		}
	}

	expectedFilter := []string{"(", "taskbook:work", "or", "taskbook.none:", ")"}
	// We expect multiple calls (fetchTasks, fetchProjects, fetchTags, fetchTaskbooks)
	// fetchTasks, fetchProjects, fetchTags must all include the taskbook filter
	var scopedCalls int
	for _, call := range reader.allFilters {
		var hasWork bool
		for _, token := range call {
			if token == "taskbook:work" {
				hasWork = true
				break
			}
		}
		if hasWork {
			assert.Subset(t, call, expectedFilter)
			scopedCalls++
		}
	}
	// At least 3 calls should have taskbook:work (tasks, projects, tags)
	assert.GreaterOrEqual(t, scopedCalls, 3)
}

