package main

import (
	"fmt"

	"github.com/tkilb/lazytask/internal/ui/helppopup"
	"github.com/tkilb/lazytask/internal/ui/popup"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
)

// modalKind says how a modal reacts to the mouse (see updateModalMouse).
type modalKind int

const (
	modalConfirm modalKind = iota // yes/no box: click a choice, or outside to cancel
	modalPicker                   // list popup: click a row to choose it
	modalAssign                   // checkbox list popup: click a row to toggle it
	modalHelp                     // scrollable help: wheel scrolls, outside closes
	modalForm                     // multi-field form: click a field to focus it
	modalInput                    // single input: clicks are ignored
)

// modalBox returns the box of the open modal, if any, for the given screen
// size, along with how it reacts to the mouse. View overlays it centered
// on the grid; updateModalMouse hit-tests clicks against the same box.
func (m model) modalBox(width, height int) (string, modalKind, bool) {
	if m.adding {
		return m.add.View(), modalForm, true
	}

	if m.deleting {
		if task, ok := m.list.Selected(); ok {
			var text string
			if m.list.Status() == tasklist.TabTodo {
				text = fmt.Sprintf("Delete task %d %q?", task.ID, task.Description)
			} else {
				text = fmt.Sprintf("Delete %q?", task.Description)
			}
			return popup.DangerConfirmBox(text, width), modalConfirm, true
		}
	}

	if m.purging {
		if task, ok := m.list.Selected(); ok {
			text := fmt.Sprintf("Permanently delete %q? This cannot be undone.", task.Description)
			return popup.DangerConfirmBox(text, width), modalConfirm, true
		}
	}

	if m.purgingAll {
		n := len(m.list.Tasks())
		noun := "tasks"
		if n == 1 {
			noun = "task"
		}
		text := fmt.Sprintf("Permanently delete all %d deleted %s? This cannot be undone.", n, noun)
		return popup.DangerConfirmBox(text, width), modalConfirm, true
	}

	if m.completing {
		if task, ok := m.list.Selected(); ok {
			var text string
			if m.list.Status() == tasklist.TabTodo {
				text = fmt.Sprintf("Mark task %d %q as done?", task.ID, task.Description)
			} else {
				text = fmt.Sprintf("Mark %q as done?", task.Description)
			}
			return popup.SuccessConfirmBox(text, width), modalConfirm, true
		}
	}

	if m.restoring {
		if task, ok := m.list.Selected(); ok {
			var text string
			if m.list.Status() == tasklist.TabDone {
				text = fmt.Sprintf("Reopen %q?", task.Description)
			} else {
				text = fmt.Sprintf("Restore %q as a todo?", task.Description)
			}
			return popup.ConfirmBox(text, width), modalConfirm, true
		}
	}

	if m.renaming {
		return m.renameInput.View(), modalInput, true
	}

	if m.renameConfirm {
		if m.renameMerging {
			text := fmt.Sprintf(
				"Renaming project %q to %q will merge its tasks into the existing project %q (including done/deleted tasks). This cannot be undone. Continue?",
				m.renameFrom, m.renameTo, m.renameTo)
			return popup.DangerConfirmBox(text, width), modalConfirm, true
		} else {
			text := fmt.Sprintf("Rename project %q to %q? This updates every task in this project, including done/deleted ones.", m.renameFrom, m.renameTo)
			return popup.ConfirmBox(text, width), modalConfirm, true
		}
	}

	if m.datePicking {
		return m.datePick.View(), modalInput, true
	}

	if m.pickingProject {
		return m.projectPicker.View(), modalPicker, true
	}

	if m.pickingTag {
		return m.tagPicker.View(), modalPicker, true
	}

	if m.assigningTags {
		return m.tagAssignPicker.View(), modalAssign, true
	}

	if m.tagAssignTyping {
		return m.tagAssignInput.View(), modalInput, true
	}

	if m.reassigning {
		return m.reassignPicker.View(), modalPicker, true
	}

	if m.reassignTyping {
		return m.reassignInput.View(), modalInput, true
	}

	if m.showingHelp {
		return helppopup.Box(m.helpSections(), width, height, m.helpScroll), modalHelp, true
	}
	return "", 0, false
}
