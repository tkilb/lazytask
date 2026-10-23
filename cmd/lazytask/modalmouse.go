package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tkilb/lazytask/internal/ui/popup"
	"github.com/tkilb/lazytask/internal/ui/tags"
)

var (
	keyEsc   = tea.KeyMsg{Type: tea.KeyEsc}
	keyEnter = tea.KeyMsg{Type: tea.KeyEnter}
	keySpace = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	keyUp    = tea.KeyMsg{Type: tea.KeyUp}
	keyDown  = tea.KeyMsg{Type: tea.KeyDown}
	keyYes   = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	keyNo    = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
)

// updateModalMouse handles the mouse while a popup or modal is open. It
// reports handled=false when neither is, so the grid handler takes over.
// Every click is translated into the key the modal already understands, so
// the mouse never has a behavior the keyboard lacks.
//
// A pending popup swallows everything but a left click, which dismisses it.
// Beyond that: clicking outside a confirm, picker, or help box cancels it;
// clicking a confirm's hint line picks that choice; clicking a picker row
// chooses it (toggles it, in the tag-assign list); the wheel scrolls
// pickers and help; clicking a form field focuses it. Text-input modals
// ignore the mouse, so a stray click never discards typed text.
func (m model) updateModalMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd, bool) {
	if m.popups.Active() {
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			m.popups = m.popups.Dismiss()
		}
		return m, nil, true
	}
	width, height := m.screenDims()
	box, kind, ok := m.modalBox(width, height)
	if !ok {
		return m, nil, false
	}
	if m.searching || msg.Action != tea.MouseActionPress {
		return m, nil, true
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return m.modalKey(kind, keyUp)
	case tea.MouseButtonWheelDown:
		return m.modalKey(kind, keyDown)
	case tea.MouseButtonLeft:
		boxW, boxH := lipgloss.Size(box)
		x := msg.X - max((width-boxW)/2, 0)
		y := msg.Y - max((height-boxH)/2, 0)
		if x < 0 || y < 0 || x >= boxW || y >= boxH {
			return m.modalKey(kind, keyEsc)
		}
		return m.clickModal(kind, x, y, boxH)
	}
	return m, nil, true
}

// modalKey forwards key to the modal, but only for kinds that map the wheel
// and outside clicks to keys (not forms or text inputs).
func (m model) modalKey(kind modalKind, key tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	if kind == modalForm || kind == modalInput {
		return m, nil, true
	}
	nm, cmd := m.Update(key)
	return nm, cmd, true
}

// clickModal handles a left click at (x, y) inside the modal box, measured
// from its top-left corner.
func (m model) clickModal(kind modalKind, x, y, boxH int) (tea.Model, tea.Cmd, bool) {
	switch kind {
	case modalConfirm:
		// The hint line is the last row inside the bottom border.
		if y == boxH-2 {
			if confirm, ok := popup.ConfirmChoiceAt(x); ok {
				key := keyNo
				if confirm {
					key = keyYes
				}
				nm, cmd := m.Update(key)
				return nm, cmd, true
			}
		}
	case modalPicker, modalAssign:
		return m.clickPickerRow(kind, y)
	case modalForm:
		if form, ok := m.add.FocusFieldAtRow(y); ok {
			m.add = form
			return m, m.add.Init(), true
		}
	}
	return m, nil, true
}

// clickPickerRow moves the open picker's cursor to the clicked row and
// then confirms it with enter, or toggles it with space in the tag-assign
// list (which confirms separately, like the keyboard). The "type tags"
// row of that list opens its prompt instead, as enter does.
func (m model) clickPickerRow(kind modalKind, y int) (tea.Model, tea.Cmd, bool) {
	picked, key := false, keyEnter
	switch {
	case m.pickingProject:
		if i, ok := m.projectPicker.RowAt(y); ok {
			m.projectPicker, picked = m.projectPicker.SelectIndex(i), true
		}
	case m.pickingTag:
		if i, ok := m.tagPicker.RowAt(y); ok {
			m.tagPicker, picked = m.tagPicker.SelectIndex(i), true
		}
	case m.reassigning:
		if i, ok := m.reassignPicker.RowAt(y); ok {
			m.reassignPicker, picked = m.reassignPicker.SelectIndex(i), true
		}
	case m.assigningTags:
		if i, ok := m.tagAssignPicker.RowAt(y); ok {
			m.tagAssignPicker, picked = m.tagAssignPicker.SelectIndex(i), true
			if label, _ := m.tagAssignPicker.Selected(); label != tags.TypeLabel {
				key = keySpace
			}
		}
	}
	if !picked {
		return m, nil, true
	}
	nm, cmd := m.Update(key)
	return nm, cmd, true
}
