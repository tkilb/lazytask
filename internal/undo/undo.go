// Package undo provides a small in-memory, multi-level undo/redo stack for
// reversible task mutations (see Phase 4 of requirements.md). It has no
// knowledge of Taskwarrior itself: callers supply Undo/Redo closures that
// perform the actual reversal, so this package only tracks ordering and
// history-truncation semantics.
package undo

// Action is a single undoable/redoable operation. Undo reverses whatever
// the original mutation did; Redo re-applies it. Both are invoked
// on-demand (not eagerly), so they may perform I/O (e.g. shelling out to
// `task`) and can fail.
type Action struct {
	// Description is a short human-readable summary of the action, shown
	// in feedback popups (e.g. "mark task 12 done").
	Description string
	Undo        func() error
	Redo        func() error
}

// Stack is a multi-level undo/redo history of Actions. The zero value is a
// ready-to-use empty stack.
//
// Entries are kept in a single slice with pos marking the boundary between
// undoable history (entries[:pos]) and redoable future (entries[pos:]),
// standard undo/redo-stack semantics: pushing a new action after undoing
// discards any redo history ahead of it, since that "future" no longer
// applies once a new mutation has happened.
type Stack struct {
	entries []Action
	pos     int
}

// Push records a newly-performed action, making it the next Undo candidate
// and discarding any previously-undone (now stale) redo history.
func (s *Stack) Push(a Action) {
	s.entries = append(s.entries[:s.pos], a)
	s.pos++
}

// CanUndo reports whether there is an action available to undo.
func (s *Stack) CanUndo() bool {
	return s.pos > 0
}

// CanRedo reports whether there is a previously-undone action available to
// redo.
func (s *Stack) CanRedo() bool {
	return s.pos < len(s.entries)
}

// Undo returns the most recent not-yet-undone action and moves the stack's
// position back one step, without invoking Action.Undo itself (the caller
// decides when/how to run it, e.g. as a tea.Cmd). Returns ok=false if
// nothing is available to undo.
func (s *Stack) Undo() (Action, bool) {
	if !s.CanUndo() {
		return Action{}, false
	}
	s.pos--
	return s.entries[s.pos], true
}

// Redo returns the next previously-undone action and moves the stack's
// position forward one step, without invoking Action.Redo itself. Returns
// ok=false if nothing is available to redo.
func (s *Stack) Redo() (Action, bool) {
	if !s.CanRedo() {
		return Action{}, false
	}
	a := s.entries[s.pos]
	s.pos++
	return a, true
}
