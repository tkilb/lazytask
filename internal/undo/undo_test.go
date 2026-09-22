package undo

import "testing"

func TestStackUndoRedo(t *testing.T) {
	var s Stack
	if s.CanUndo() || s.CanRedo() {
		t.Fatalf("zero-value Stack should have nothing to undo/redo")
	}

	var log []string
	push := func(name string) {
		s.Push(Action{
			Description: name,
			Undo:        func() error { log = append(log, "undo:"+name); return nil },
			Redo:        func() error { log = append(log, "redo:"+name); return nil },
		})
	}

	push("a")
	push("b")

	if !s.CanUndo() || s.CanRedo() {
		t.Fatalf("after two pushes: want CanUndo=true CanRedo=false, got CanUndo=%v CanRedo=%v", s.CanUndo(), s.CanRedo())
	}

	act, ok := s.Undo()
	if !ok || act.Description != "b" {
		t.Fatalf("expected to undo %q, got %+v (ok=%v)", "b", act, ok)
	}
	if !s.CanUndo() || !s.CanRedo() {
		t.Fatalf("after one undo of two: want both true, got CanUndo=%v CanRedo=%v", s.CanUndo(), s.CanRedo())
	}

	act, ok = s.Undo()
	if !ok || act.Description != "a" {
		t.Fatalf("expected to undo %q, got %+v (ok=%v)", "a", act, ok)
	}
	if s.CanUndo() {
		t.Fatalf("expected nothing left to undo")
	}
	if _, ok := s.Undo(); ok {
		t.Fatalf("expected Undo on empty history to report ok=false")
	}

	act, ok = s.Redo()
	if !ok || act.Description != "a" {
		t.Fatalf("expected to redo %q, got %+v (ok=%v)", "a", act, ok)
	}
	act, ok = s.Redo()
	if !ok || act.Description != "b" {
		t.Fatalf("expected to redo %q, got %+v (ok=%v)", "b", act, ok)
	}
	if s.CanRedo() {
		t.Fatalf("expected nothing left to redo")
	}
	if _, ok := s.Redo(); ok {
		t.Fatalf("expected Redo on empty future to report ok=false")
	}
}

func TestStackPushTruncatesRedoHistory(t *testing.T) {
	var s Stack
	s.Push(Action{Description: "a"})
	s.Push(Action{Description: "b"})
	s.Undo() // back to just "a" undoable, "b" redoable

	s.Push(Action{Description: "c"})

	if s.CanRedo() {
		t.Fatalf("pushing a new action should discard stale redo history (%q)", "b")
	}
	act, ok := s.Undo()
	if !ok || act.Description != "c" {
		t.Fatalf("expected to undo %q, got %+v (ok=%v)", "c", act, ok)
	}
	act, ok = s.Undo()
	if !ok || act.Description != "a" {
		t.Fatalf("expected to undo %q, got %+v (ok=%v)", "a", act, ok)
	}
}
