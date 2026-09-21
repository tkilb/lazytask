package popup

import "testing"
import "strings"

func TestModelQueueOrder(t *testing.T) {
	var m Model
	if m.Active() {
		t.Fatalf("zero-value Model should not be active")
	}
	if _, ok := m.Current(); ok {
		t.Fatalf("zero-value Model should have no current message")
	}

	m = m.Push(Message{Severity: Error, Text: "first"})
	m = m.Push(Message{Severity: Warning, Text: "second"})

	if !m.Active() {
		t.Fatalf("expected Active() after Push")
	}

	got, ok := m.Current()
	if !ok || got.Text != "first" {
		t.Fatalf("expected current message %q, got %+v (ok=%v)", "first", got, ok)
	}

	m = m.Dismiss()
	got, ok = m.Current()
	if !ok || got.Text != "second" {
		t.Fatalf("expected current message %q after dismiss, got %+v (ok=%v)", "second", got, ok)
	}

	m = m.Dismiss()
	if m.Active() {
		t.Fatalf("expected queue empty after dismissing all messages")
	}
}

func TestModelDismissEmptyIsNoop(t *testing.T) {
	var m Model
	m = m.Dismiss()
	if m.Active() {
		t.Fatalf("dismissing an empty queue should stay empty")
	}
}

func TestSeverityString(t *testing.T) {
	cases := map[Severity]string{
		Info:    "Info",
		Warning: "Warning",
		Error:   "Error",
	}
	for sev, want := range cases {
		if got := sev.String(); got != want {
			t.Errorf("Severity(%d).String() = %q, want %q", sev, got, want)
		}
	}
}

func TestRenderContainsMessageText(t *testing.T) {
	msg := Message{Severity: Error, Text: "task binary not found"}
	out := Render(msg, 80, 24)
	if out == "" {
		t.Fatalf("Render returned empty string")
	}
	if !contains(out, "task binary not found") {
		t.Errorf("Render output missing message text: %q", out)
	}
	if !contains(out, "Error") {
		t.Errorf("Render output missing severity label: %q", out)
	}
}

func TestOverlayPreservesBackgroundOutsideBox(t *testing.T) {
	background := strings.Repeat("A", 20) + "\n" + strings.Repeat("B", 20)
	box := "X"

	out := Overlay(background, box, 20, 2)
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), out)
	}
	// The single-line, single-column box lands on row 0 (of 2, centered),
	// so row 0 should have its middle character replaced with "X" while
	// everything else on both rows stays the untouched background.
	if !contains(lines[0], "X") {
		t.Errorf("expected box content spliced into row 0, got %q", lines[0])
	}
	if lines[1] != strings.Repeat("B", 20) {
		t.Errorf("expected row 1 untouched, got %q", lines[1])
	}
	if got := strings.Count(lines[0], "A"); got != 19 {
		t.Errorf("expected 19 untouched background chars on row 0, got %d in %q", got, lines[0])
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
