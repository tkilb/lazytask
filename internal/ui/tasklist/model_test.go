package tasklist

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tkilb/lazytask/internal/taskwarrior"
)

// sampleTasks returns fixture data used across tests; no taskwarrior
// process is ever invoked in this package.
func sampleTasks() []taskwarrior.Task {
	return []taskwarrior.Task{
		{ID: 1, Description: "Buy groceries", Project: "Home", Priority: "H", Due: "2026-09-20"},
		{ID: 2, Description: "Write report", Project: "Work", Priority: "M"},
		{ID: 3, Description: "Water plants", Project: "Home"},
	}
}

func TestNew_SelectsFirstTask(t *testing.T) {
	m := New(sampleTasks())

	selected, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, 1, selected.ID)
}

func TestNew_EmptyTasks(t *testing.T) {
	m := New(nil)

	_, ok := m.Selected()
	assert.False(t, ok)
}

func TestModel_Update_Navigation(t *testing.T) {
	tests := []struct {
		name       string
		keys       []string
		wantSelID  int
		wantCursor int
	}{
		{
			name:       "down moves to next task",
			keys:       []string{"down"},
			wantSelID:  2,
			wantCursor: 1,
		},
		{
			name:       "j moves to next task",
			keys:       []string{"j"},
			wantSelID:  2,
			wantCursor: 1,
		},
		{
			name:       "down twice then up once",
			keys:       []string{"down", "down", "up"},
			wantSelID:  2,
			wantCursor: 1,
		},
		{
			name:       "k does nothing at top",
			keys:       []string{"k"},
			wantSelID:  1,
			wantCursor: 0,
		},
		{
			name:       "down past end clamps at last task",
			keys:       []string{"down", "down", "down", "down", "down"},
			wantSelID:  3,
			wantCursor: 2,
		},
		{
			name:       "up past start clamps at first task",
			keys:       []string{"up", "up"},
			wantSelID:  1,
			wantCursor: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(sampleTasks())
			for _, k := range tt.keys {
				m, _ = m.Update(tea.KeyMsg{Type: keyTypeFor(k), Runes: []rune(k)})
			}

			assert.Equal(t, tt.wantCursor, m.cursor)
			selected, ok := m.Selected()
			require.True(t, ok)
			assert.Equal(t, tt.wantSelID, selected.ID)
		})
	}
}

// keyTypeFor maps the key strings used in table tests to the tea.KeyType
// bubbletea expects, since arrow keys are distinct types while letters are
// tea.KeyRunes.
func keyTypeFor(key string) tea.KeyType {
	switch key {
	case "up":
		return tea.KeyUp
	case "down":
		return tea.KeyDown
	default:
		return tea.KeyRunes
	}
}

func TestModel_SetTasks_ClampsCursor(t *testing.T) {
	m := New(sampleTasks())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	require.Equal(t, 2, m.cursor)

	m = m.SetTasks([]taskwarrior.Task{{ID: 9, Description: "only one left"}})
	assert.Equal(t, 0, m.cursor)

	selected, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, 9, selected.ID)
}

func TestModel_SetTasks_EmptyClampsToZero(t *testing.T) {
	m := New(sampleTasks())
	m = m.SetTasks(nil)

	_, ok := m.Selected()
	assert.False(t, ok)
}

func TestModel_SelectID_MovesCursorToMatchingTask(t *testing.T) {
	m := New(sampleTasks())

	m = m.SelectID(3)

	assert.Equal(t, 2, m.cursor)
	selected, ok := m.Selected()
	require.True(t, ok)
	assert.Equal(t, 3, selected.ID)
}

func TestModel_SelectID_NoMatchLeavesCursorUnchanged(t *testing.T) {
	m := New(sampleTasks())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	require.Equal(t, 1, m.cursor)

	m = m.SelectID(999)

	assert.Equal(t, 1, m.cursor)
}

func TestModel_View_ContainsTaskData(t *testing.T) {
	m := New(sampleTasks())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	view := m.View()

	assert.True(t, strings.Contains(view, "Buy groceries"))
	assert.True(t, strings.Contains(view, "Write report"))
	assert.True(t, strings.Contains(view, "Water plants"))
	assert.True(t, strings.Contains(view, "Todo"))
}

func TestModel_View_Empty(t *testing.T) {
	m := New(nil)
	view := m.View()

	assert.True(t, strings.Contains(view, "(no tasks)"))
}

func TestModel_SetFocused_SetsFocusedFlag(t *testing.T) {
	m := New(sampleTasks())
	assert.False(t, m.focused)

	m = m.SetFocused(true)
	assert.True(t, m.focused)

	m = m.SetFocused(false)
	assert.False(t, m.focused)
}

func TestModel_NextStatus_CyclesTodoDoneDeleted(t *testing.T) {
	m := New(sampleTasks())
	assert.Equal(t, TabTodo, m.Status())
	assert.Equal(t, "status:pending", m.StatusFilter())

	m = m.NextStatus()
	assert.Equal(t, TabDone, m.Status())
	assert.Equal(t, "status:completed", m.StatusFilter())

	m = m.NextStatus()
	assert.Equal(t, TabDeleted, m.Status())
	assert.Equal(t, "status:deleted", m.StatusFilter())

	m = m.NextStatus()
	assert.Equal(t, TabTodo, m.Status())
}

func TestModel_PrevStatus_CyclesBackward(t *testing.T) {
	m := New(sampleTasks())

	m = m.PrevStatus()
	assert.Equal(t, TabDeleted, m.Status())

	m = m.PrevStatus()
	assert.Equal(t, TabDone, m.Status())

	m = m.PrevStatus()
	assert.Equal(t, TabTodo, m.Status())
}

func TestModel_NextStatus_ResetsCursor(t *testing.T) {
	m := New(sampleTasks())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	require.Equal(t, 1, m.cursor)

	m = m.NextStatus()
	assert.Equal(t, 0, m.cursor)
}

func TestModel_View_ShowsActiveTabHighlighted(t *testing.T) {
	m := New(sampleTasks())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	view := m.View()
	assert.True(t, strings.Contains(view, "[2]-Todo - Done - Deleted"))
}
