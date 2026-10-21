package main

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/projects"
	"github.com/tkilb/lazytask/internal/ui/tags"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
)

func layoutModel(n int, focus panelFocus, height int) model {
	m := model{list: tasklist.New(nil), projects: projects.New(), tags: tags.New(), reader: &stubReader{}, focus: focus}
	for i := 0; i < n; i++ {
		m.suggestedTasks = append(m.suggestedTasks, taskwarrior.Task{
			ID: i + 1, UUID: fmt.Sprintf("u%d", i+1), Description: fmt.Sprintf("task%d", i), Status: "pending",
			Project: "P", Urgency: float64(100 - i),
		})
	}
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: height})
	return nm.(model)
}

func TestLeftHeights_SumToTotalAndRespectFloors(t *testing.T) {
	for _, focus := range []panelFocus{focusStatus, focusTasks, focusProjects, focusTags, focusDetails} {
		for total := tasklistOnlyBelow - gridBottomOverhead; total <= 80; total++ {
			s, ts, b := leftColumnHeights(total, focus, 5)
			assert.Equal(t, total, s+ts+b, "focus=%d total=%d", focus, total)
			assert.GreaterOrEqual(t, b, minListPanelHeight, "focus=%d total=%d", focus, total)
			assert.GreaterOrEqual(t, ts, tasksMinHeight(total), "focus=%d total=%d", focus, total)
		}
	}
}

func TestLeftHeights_SuggestedIsOneRowUnlessFocused(t *testing.T) {
	for _, focus := range []panelFocus{focusTasks, focusProjects, focusTags, focusDetails} {
		s, _, _ := leftColumnHeights(40, focus, 5)
		assert.Equal(t, 3, s, "focus=%d", focus)
	}
	s, _, _ := leftColumnHeights(40, focusStatus, 5)
	assert.Equal(t, 7, s, "5 picks plus borders")
	s, _, _ = leftColumnHeights(40, focusStatus, 8)
	assert.Equal(t, 7, s, "capped at 5 rows")
	s, _, _ = leftColumnHeights(40, focusStatus, 2)
	assert.Equal(t, 4, s)
	s, _, _ = leftColumnHeights(40, focusStatus, 0)
	assert.Equal(t, 3, s, "empty-state message still needs a row")
}

func TestLeftHeights_FocusedPanelGetsExtraSpace(t *testing.T) {
	_, tasksF, bottomF := leftColumnHeights(40, focusTasks, 3)
	_, tasksP, bottomP := leftColumnHeights(40, focusProjects, 3)
	_, tasksT, bottomT := leftColumnHeights(40, focusTags, 3)
	assert.Greater(t, tasksF, tasksP, "Tasks is larger while focused")
	assert.Greater(t, bottomP, bottomF, "Projects/Tags row is larger while focused")
	assert.Equal(t, bottomP, bottomT, "Projects and Tags share the row, so grow together")
	assert.Equal(t, tasksP, tasksT)
}

func TestLeftHeights_ProjectsAndTagsFloorIsFiveContentRows(t *testing.T) {
	assert.Equal(t, 7, minListPanelHeight)
	_, _, b := leftColumnHeights(40, focusTasks, 3)
	assert.Equal(t, 7, b)
}

func TestLeftHeights_DetailsFocusMatchesTasksFocus(t *testing.T) {
	s1, t1, b1 := leftColumnHeights(40, focusTasks, 3)
	s2, t2, b2 := leftColumnHeights(40, focusDetails, 3)
	assert.Equal(t, [3]int{s1, t1, b1}, [3]int{s2, t2, b2})
}

func TestLeftHeights_UnfocusedTasksKeepsSensibleMinimum(t *testing.T) {
	assert.Equal(t, 10, tasksMinHeight(23), "8 content rows plus borders beats 40%")
	assert.Equal(t, 20, tasksMinHeight(50), "40% beats 8 rows on tall terminals")
	_, ts, _ := leftColumnHeights(50, focusProjects, 3)
	assert.GreaterOrEqual(t, ts, 20)
}

func TestShortTerminal_TasklistOnly(t *testing.T) {
	m := layoutModel(3, focusTasks, tasklistOnlyBelow-1)
	assert.True(t, m.tasklistOnly())
	view := m.View()
	assert.NotContains(t, view, "Suggested")
	assert.NotContains(t, view, "Projects")
	assert.NotContains(t, view, "Tags")
	assert.Len(t, strings.Split(view, "\n"), tasklistOnlyBelow-1)

	tall := layoutModel(3, focusTasks, tasklistOnlyBelow)
	assert.False(t, tall.tasklistOnly())
	assert.Contains(t, tall.View(), "Suggested")
}

func TestShortTerminal_PanelKeysIgnoredAndTabSkipsHidden(t *testing.T) {
	m := layoutModel(3, focusTasks, tasklistOnlyBelow-1)
	for _, k := range []string{"1", "3", "4"} {
		assert.Equal(t, focusTasks, keyOf(m, k).focus, "key %s", k)
	}
	assert.Equal(t, focusDetails, keyOf(m, "0").focus)
	assert.Equal(t, focusDetails, keyOf(m, "tab").focus)
	d := keyOf(m, "tab")
	assert.Equal(t, focusTasks, keyOf(d, "tab").focus)
	assert.Equal(t, focusTasks, keyOf(d, "shift+tab").focus)
}

func TestShortTerminal_FocusMovesToTasksWhenPanelHides(t *testing.T) {
	m := layoutModel(3, focusProjects, 40)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: tasklistOnlyBelow - 1})
	assert.Equal(t, focusTasks, nm.(model).focus)

	m = layoutModel(3, focusDetails, 40)
	nm, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: tasklistOnlyBelow - 1})
	assert.Equal(t, focusDetails, nm.(model).focus, "Details stays visible")
}

func TestView_SuggestedShowsOnePickUnfocusedAndFiveFocused(t *testing.T) {
	m := layoutModel(8, focusTasks, 40)
	view := m.View()
	assert.Contains(t, view, "task0")
	assert.NotContains(t, view, "task1")

	m = keyOf(m, "1")
	view = m.View()
	for i := 0; i < 5; i++ {
		assert.Contains(t, view, fmt.Sprintf("task%d", i))
	}
	assert.NotContains(t, view, "task5")
	assert.Len(t, strings.Split(view, "\n"), 40, "grid still fills the terminal exactly")
}

func TestView_FocusChangeResizesPanelsAndFillsTerminal(t *testing.T) {
	m := layoutModel(5, focusTasks, 40)
	for _, k := range []string{"1", "3", "4", "2", "0", "tab", "shift+tab"} {
		m = keyOf(m, k)
		assert.Len(t, strings.Split(m.View(), "\n"), 40, "after key %s", k)
	}
}

func TestView_SpaceFromSuggestedReturnsToTasksLayout(t *testing.T) {
	m := layoutModel(5, focusStatus, 40)
	m = keyOf(m, " ")
	assert.Equal(t, focusTasks, m.focus)
	assert.Len(t, strings.Split(m.View(), "\n"), 40)
	assert.NotContains(t, m.View(), "task4", "Suggested collapsed back to one row")
}

func TestHelp_LayoutUsesUnderlyingFocus(t *testing.T) {
	m := layoutModel(5, focusProjects, 40)
	before := m.View()
	m = keyOf(m, "?")
	assert.True(t, m.showingHelp)
	_, _, s1, t1, b1, _ := m.gridDims()
	m.showingHelp = false
	_, _, s2, t2, b2, _ := m.gridDims()
	assert.Equal(t, [3]int{s2, t2, b2}, [3]int{s1, t1, b1})
	assert.Equal(t, before, m.View(), "closing help leaves the layout unchanged")
}

func TestProjectsPanel_SpaceJumpsToTasks(t *testing.T) {
	m := layoutModel(3, focusProjects, 40)
	m = keyOf(m, " ")
	assert.Equal(t, focusTasks, m.focus)
	assert.Len(t, strings.Split(m.View(), "\n"), 40, "layout follows the focus change")
}

func TestProjectsPanel_SpaceDoesNotChangeFilter(t *testing.T) {
	m := layoutModel(3, focusProjects, 40)
	before := m.filter
	m = keyOf(m, " ")
	assert.True(t, before.equal(m.filter))
}

func TestHelp_ProjectsPanelListsSpace(t *testing.T) {
	m := layoutModel(3, focusProjects, 40)
	found := false
	for _, b := range m.helpLocalBindings() {
		if b.Key == "space" {
			found = true
		}
	}
	assert.True(t, found, "Projects local help lists space")
}
