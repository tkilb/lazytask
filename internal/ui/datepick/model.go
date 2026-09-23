// Package datepick implements a small bordered input component for
// quickly picking a due date via lazytask's cord shorthand (see
// internal/dateparse): short strings like "2d", "1w", or "2b", or a typed
// absolute date — US/international, with flexible "-", "/", "."
// separators and optional leading zeros, e.g. "3/14" (MM-DD shorthand,
// current year, rolling to next year if already past), "3-14-2026", or
// "2026-03-14" — that the component live-previews as a resolved calendar
// date/time as the user types. This package has no taskwarrior
// dependency; it's wired into the 'D' due-date reassign popup, and will
// also back the Add form's due-date field in a later chunk.
package datepick

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tkilb/lazytask/internal/dateparse"
)

var (
	borderColor = lipgloss.Color("62")

	bodyStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderTop(false).
			BorderForeground(borderColor)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(borderColor)

	hintStyle = lipgloss.NewStyle().
			Foreground(borderColor)

	previewStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	previewErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196"))
)

const minPanelWidth = 80

const (
	title    = "Due Date"
	hintText = "Press <enter> to confirm, <esc> to cancel"

	// previewLayout is used to render a resolved cord as a human-readable
	// date in the preview line, e.g. "Wed Jan 10 2024". Due dates never
	// carry a time-of-day, so no time component is shown.
	previewLayout = "Mon Jan 2 2006"
)

// Model is a Bubble Tea model rendering a single-line bordered text input
// for a due-date cord (e.g. "2d", "1w", "2b"), with a live preview line
// underneath showing the resolved date/time or a parse-error hint.
type Model struct {
	input textinput.Model
	width int

	// now returns the reference instant cords are resolved relative to.
	// Defaults to time.Now but is overridable so tests get deterministic
	// output.
	now func() time.Time
}

// New constructs a Model with an empty input.
func New() Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "e.g. 2d, 1w, 2b, 3/14, or 2026-03-14"
	ti.CharLimit = 32
	return Model{input: ti, now: time.Now}
}

// WithNow returns a copy of m using now in place of time.Now for resolving
// cords, so callers (tests, or a host model wanting a fixed "current time"
// for the whole session) get deterministic previews.
func (m Model) WithNow(now func() time.Time) Model {
	m.now = now
	return m
}

// Focus gives the input keyboard focus and clears any prior text.
func (m Model) Focus() Model {
	m.input.Reset()
	m.input.Focus()
	return m
}

// Blur removes keyboard focus from the input.
func (m Model) Blur() Model {
	m.input.Blur()
	return m
}

// Focused reports whether the input currently has keyboard focus.
func (m Model) Focused() bool {
	return m.input.Focused()
}

// Value returns the current (untrimmed) input text.
func (m Model) Value() string {
	return m.input.Value()
}

// Resolved parses and resolves the current input, first as a cord relative
// to now(), then (if that fails) as an absolute date typed in directly via
// parseFlexibleAbsoluteDate (a looser US/international date, no
// time-of-day) — so a user can either type a quick cord or key in a
// specific date. ok is false if neither parses (including an empty
// input).
func (m Model) Resolved() (t time.Time, ok bool) {
	value := m.input.Value()

	nowFn := m.now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn()
	if resolved, err := dateparse.ResolveString(value, now); err == nil {
		return resolved, true
	}

	trimmed := strings.TrimSpace(value)
	if parsed, ok := parseFlexibleAbsoluteDate(trimmed, now); ok {
		return parsed, true
	}
	return time.Time{}, false
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model. It only updates the underlying text input;
// it does not interpret enter/esc itself, since those keys are also
// meaningful to the host model (confirm/cancel), which retains full
// control over the current mode/focus.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m Model) View() string {
	width := m.width
	if width <= 0 {
		width = minPanelWidth
	}
	innerWidth := width - 2
	if innerWidth < 16 {
		innerWidth = 16
	}

	preview := m.previewLine()
	body := bodyStyle.Width(innerWidth).Render(m.input.View() + "\n" + preview)
	return m.topBorder(innerWidth) + "\n" + body
}

// previewLine renders the live preview shown under the input: the resolved
// date/time for a valid cord, a neutral hint for an empty input, or an
// error hint for text that doesn't parse as a cord.
func (m Model) previewLine() string {
	if m.input.Value() == "" {
		return previewStyle.Render("Enter shorthand (2d, 2b, 1w) or a date")
	}
	if resolved, ok := m.Resolved(); ok {
		return previewStyle.Render(resolved.Format(previewLayout))
	}
	return previewErrorStyle.Render("Invalid input (expected <N>d/w/b, M-D, or a full date)")
}

// topBorder builds the box's top edge with the title embedded on the left
// and the key-binding hint embedded on the right, lazygit-style, matching
// the addform package's box styling.
func (m Model) topBorder(innerWidth int) string {
	border := lipgloss.RoundedBorder()

	titleRendered := " " + titleStyle.Render(title) + " "
	hint := " " + hintStyle.Render(hintText) + " "

	totalWidth := innerWidth + 2
	fillWidth := totalWidth - 2 - lipgloss.Width(titleRendered) - lipgloss.Width(hint)
	if fillWidth < 1 {
		fillWidth = 1
	}

	return border.TopLeft + titleRendered + strings.Repeat(border.Top, fillWidth) + hint + border.TopRight
}

// flexibleDateSeparators are the separators accepted between the
// components of a typed absolute date, in addition to dashes: "3/14",
// "3.14", and "3-14" are all treated the same way.
var flexibleDateSeparators = strings.NewReplacer("/", "-", ".", "-")

// parseFlexibleAbsoluteDate parses value as a loosely-formatted absolute
// date, accepting "-", "/", or "." as separators and optional leading
// zeros on every component:
//
//   - Two components ("M-D", e.g. "3-14", "3.14", "3/14", "1.5"): US-style
//     month-day shorthand with no year. Resolves to that month/day in
//     now's year, rolling forward to next year if that date has already
//     passed relative to now (future-only), e.g. from Dec 30 2026, "1.5"
//     resolves to Jan 5 2027, but "12.31" still resolves to Dec 31 2026.
//   - Three components, first component 4 digits ("YYYY-M-D", e.g.
//     "2026-3-14"): international/ISO order, year first.
//   - Three components, otherwise ("M-D-YY" or "M-D-YYYY", e.g.
//     "3-14-2026", "3.14.26"): US order, year last. A 2-digit year is
//     interpreted as 2000+YY.
//
// ok is false if value doesn't match any of these shapes, or resolves to
// an invalid calendar date (e.g. "2-30").
func parseFlexibleAbsoluteDate(value string, now time.Time) (time.Time, bool) {
	normalized := flexibleDateSeparators.Replace(value)
	parts := strings.Split(normalized, "-")

	switch len(parts) {
	case 2:
		month, day, ok := parseInts(parts[0], parts[1])
		if !ok {
			return time.Time{}, false
		}
		year := now.Year()
		candidate, ok := buildDate(year, month, day, now.Location())
		if !ok {
			return time.Time{}, false
		}
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		if candidate.Before(today) {
			candidate, ok = buildDate(year+1, month, day, now.Location())
			if !ok {
				return time.Time{}, false
			}
		}
		return candidate, true

	case 3:
		if len(strings.TrimSpace(parts[0])) == 4 {
			year, month, day, ok := parseInts3(parts[0], parts[1], parts[2])
			if !ok {
				return time.Time{}, false
			}
			return buildDate(year, month, day, now.Location())
		}

		month, day, yearRaw, ok := parseInts3(parts[0], parts[1], parts[2])
		if !ok {
			return time.Time{}, false
		}
		year := normalizeYear(yearRaw, len(strings.TrimSpace(parts[2])))
		return buildDate(year, month, day, now.Location())

	default:
		return time.Time{}, false
	}
}

// parseInts parses two components as plain (optionally zero-padded)
// non-negative integers.
func parseInts(a, b string) (x, y int, ok bool) {
	x, err1 := strconv.Atoi(strings.TrimSpace(a))
	y, err2 := strconv.Atoi(strings.TrimSpace(b))
	return x, y, err1 == nil && err2 == nil
}

// parseInts3 parses three components as plain (optionally zero-padded)
// non-negative integers.
func parseInts3(a, b, c string) (x, y, z int, ok bool) {
	x, err1 := strconv.Atoi(strings.TrimSpace(a))
	y, err2 := strconv.Atoi(strings.TrimSpace(b))
	z, err3 := strconv.Atoi(strings.TrimSpace(c))
	return x, y, z, err1 == nil && err2 == nil && err3 == nil
}

// normalizeYear expands a 2-digit year (as commonly typed, e.g. "26") to
// 2000+YY. digits is the original component's character length, so a
// zero-padded 4-digit year like "0026" isn't mistaken for a 2-digit one.
func normalizeYear(year, digits int) int {
	if digits <= 2 && year < 100 {
		return 2000 + year
	}
	return year
}

// buildDate constructs a midnight local date from year/month/day,
// rejecting out-of-range values and dates that time.Date would silently
// normalize (e.g. month 13, or day 30 in February).
func buildDate(year, month, day int, loc *time.Location) (time.Time, bool) {
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}, false
	}
	candidate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, loc)
	if int(candidate.Month()) != month || candidate.Day() != day {
		return time.Time{}, false
	}
	return candidate, true
}
