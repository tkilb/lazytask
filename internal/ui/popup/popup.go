// Package popup provides a small blocking-modal component for surfacing
// errors, warnings, and informational messages to the user: failed
// taskwarrior CLI invocations, app-level warnings (e.g. the task binary
// not being found), and form validation errors. Messages queue up (shown
// one at a time, oldest first) and each is dismissed by any keypress.
package popup

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Severity controls a popup's border/title color and label.
type Severity int

const (
	Info Severity = iota
	Warning
	Error
)

// String returns the label shown in the popup's title bar.
func (s Severity) String() string {
	switch s {
	case Warning:
		return "Warning"
	case Error:
		return "Error"
	default:
		return "Info"
	}
}

func (s Severity) color() lipgloss.Color {
	switch s {
	case Warning:
		return lipgloss.Color("214") // orange
	case Error:
		return lipgloss.Color("196") // red
	default:
		return lipgloss.Color("39") // blue
	}
}

// Message is a single queued popup.
type Message struct {
	Severity Severity
	Text     string
}

// Model holds a FIFO queue of pending popup messages. The zero value is a
// ready-to-use empty queue.
type Model struct {
	queue []Message
}

// Push enqueues msg, returning the updated model.
func (m Model) Push(msg Message) Model {
	m.queue = append(append([]Message{}, m.queue...), msg)
	return m
}

// Active reports whether there is a popup currently pending display.
func (m Model) Active() bool {
	return len(m.queue) > 0
}

// Current returns the front-of-queue message, if any.
func (m Model) Current() (Message, bool) {
	if len(m.queue) == 0 {
		return Message{}, false
	}
	return m.queue[0], true
}

// Dismiss removes the front-of-queue message, revealing the next one (if
// any) on the following render.
func (m Model) Dismiss() Model {
	if len(m.queue) == 0 {
		return m
	}
	m.queue = m.queue[1:]
	return m
}

const (
	minBoxWidth = 30
	maxBoxWidth = 60
)

// Render draws msg as a bordered, centered modal box occupying the full
// screenWidth x screenHeight terminal area (lazygit-style popup): border
// and title colored per severity, with a dismiss hint at the bottom. The
// area outside the box is left blank; use Overlay instead to composite the
// box on top of an existing background view.
func Render(msg Message, screenWidth, screenHeight int) string {
	box := Box(msg, screenWidth)
	if screenWidth <= 0 || screenHeight <= 0 {
		return box
	}
	return lipgloss.Place(screenWidth, screenHeight, lipgloss.Center, lipgloss.Center, box)
}

// Box draws msg as a bordered modal box (border/title colored per
// severity, with a dismiss hint at the bottom) sized to fit within
// screenWidth, without placing it on any particular background. Combine
// with Overlay to composite it centered over an existing view.
func Box(msg Message, screenWidth int) string {
	return renderBox(msg.Severity.String(), msg.Severity.color(), "press any key to dismiss", msg.Text, screenWidth)
}

// confirmColor is the border/title color used by ConfirmBox, matching the
// "x" delete keybinding's destructive intent.
var confirmColor = lipgloss.Color("214") // orange, same as Warning

// ConfirmBox draws a yes/no confirmation modal box (bordered, titled
// "Confirm", with a y/n hint) sized to fit within screenWidth, without
// placing it on any particular background. Combine with Overlay to
// composite it centered over an existing view, mirroring Box's
// severity-styled popups but for blocking confirmation prompts.
func ConfirmBox(text string, screenWidth int) string {
	return renderBox("Confirm", confirmColor, "(y/enter) confirm   (n/esc) cancel", text, screenWidth)
}

// renderBox is the shared layout used by Box and ConfirmBox: a bordered box
// with a colored title, body text, and a hint line at the bottom.
func renderBox(titleText string, color lipgloss.Color, hintText, bodyText string, screenWidth int) string {
	boxWidth := maxBoxWidth
	if screenWidth > 0 && screenWidth-4 < boxWidth {
		boxWidth = screenWidth - 4
	}
	if boxWidth < minBoxWidth {
		boxWidth = minBoxWidth
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(color).Render(titleText)
	hint := lipgloss.NewStyle().Faint(true).Render(hintText)

	body := lipgloss.JoinVertical(lipgloss.Left, title, "", bodyText, "", hint)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Padding(0, 1).
		Width(boxWidth).
		Render(body)
}

// Overlay composites box on top of background, centered within a
// screenWidth x screenHeight terminal area, preserving whatever of
// background falls outside the box's bounds (unlike Render/lipgloss.Place,
// which replace the whole area). Both background and box may contain ANSI
// styling; splicing is done ANSI- and wide-rune-aware via
// charmbracelet/x/ansi so escape sequences and multi-cell runes aren't torn
// mid-sequence/mid-rune.
func Overlay(background, box string, screenWidth, screenHeight int) string {
	if screenWidth <= 0 || screenHeight <= 0 {
		return box
	}

	bgLines := strings.Split(background, "\n")
	for len(bgLines) < screenHeight {
		bgLines = append(bgLines, "")
	}

	boxLines := strings.Split(box, "\n")
	boxWidth := 0
	for _, l := range boxLines {
		if w := ansi.StringWidth(l); w > boxWidth {
			boxWidth = w
		}
	}
	boxHeight := len(boxLines)

	startRow := (screenHeight - boxHeight) / 2
	startCol := (screenWidth - boxWidth) / 2
	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	out := make([]string, len(bgLines))
	copy(out, bgLines)

	for i, boxLine := range boxLines {
		row := startRow + i
		if row < 0 || row >= len(out) {
			continue
		}
		bgLine := out[row]
		if w := ansi.StringWidth(bgLine); w < screenWidth {
			bgLine += strings.Repeat(" ", screenWidth-w)
		}

		if w := ansi.StringWidth(boxLine); w < boxWidth {
			boxLine += strings.Repeat(" ", boxWidth-w)
		}

		left := ansi.Cut(bgLine, 0, startCol)
		right := ansi.Cut(bgLine, startCol+boxWidth, screenWidth)
		out[row] = left + boxLine + right
	}

	return strings.Join(out, "\n")
}

