// Package panel provides the shared lazygit-style bordered-panel styling
// used across the panel grid: a rounded border and bold title, both
// switching to a highlight color when the panel is focused. Individual
// panel implementations (tasklist, and the placeholders wired up in
// cmd/lazytask) build on these helpers so the focused-panel treatment stays
// visually consistent as more panels gain real content in later chunks.
package panel

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// palette is the single place ANSI color values are defined for this
// package. Roles below (UnfocusedColor, FocusedColor, etc.) reference these
// named entries instead of embedding raw color literals, so retuning the
// look later is a one-line edit here rather than a hunt through the
// package. Values are chosen to match lazygit's default theme colors
// (github.com/jesseduffield/lazygit's default UserConfig) for cross-tool
// visual consistency, not picked arbitrarily.
const (
	// paletteDefault leaves the terminal's own foreground color in place
	// (no escape codes emitted), matching lazygit's "default" theme color.
	paletteDefault = lipgloss.Color("")
	// paletteGreen matches lazygit's default theme.activeBorderColor.
	paletteGreen = lipgloss.Color("2")
	// paletteBlue matches lazygit's default theme.optionsTextColor (also
	// its selectedLineBgColor, reused here for the same row-highlight
	// role in list panels).
	paletteBlue = lipgloss.Color("4")
)

const (
	// UnfocusedColor is the border/title color for a panel that does not
	// currently have focus.
	UnfocusedColor = paletteDefault
	// FocusedColor is the border/title color for the currently focused
	// panel.
	FocusedColor = paletteGreen

	// activeTabColor highlights the selected tab in a FrameTabs title, kept
	// deliberately distinct from FocusedColor so the active tab stays
	// visually identifiable even when the panel itself is focused.
	activeTabColor = paletteBlue

	// SelectedRowBackground is the background color for the
	// currently-selected row in a list panel (Tasks, Projects), matching
	// lazygit's default theme.selectedLineBgColor (blue). Exported so
	// list-owning packages outside panel can share it rather than each
	// picking their own row-highlight color.
	SelectedRowBackground = paletteBlue
	// inactiveTabColor is used for the non-selected tabs in a FrameTabs
	// title, deliberately independent of panel focus so it doesn't compete
	// with activeTabColor.
	inactiveTabColor = paletteDefault

	// minWidth/minHeight guard against nonsensical (zero or negative)
	// panel sizes, e.g. before the first tea.WindowSizeMsg arrives. minHeight
	// is deliberately just 1 content row (rather than enough for a header
	// plus rows) so panels that intentionally want a single-line body, like
	// the Status panel's fixed height, aren't forced taller than requested.
	minWidth  = 10
	minHeight = 1
)

// BorderStyle returns the rounded-border style for a panel, colored
// according to whether it currently has focus.
func BorderStyle(focused bool) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor(focused))
}

// TitleStyle returns the bold title style for a panel, colored according to
// whether it currently has focus.
func TitleStyle(focused bool) lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(borderColor(focused))
}

func borderColor(focused bool) lipgloss.Color {
	if focused {
		return FocusedColor
	}
	return UnfocusedColor
}

// Render draws a titled, bordered panel with the given body text, sized to
// occupy outerWidth x outerHeight (the panel's total footprint, border
// included) once placed in the grid. The title is embedded directly in the
// top border line (lazygit-style, e.g. "╭─[1] Status─────╮") instead of
// consuming a content row, so the full inner height is available to body.
func Render(title, body string, outerWidth, outerHeight int, focused bool) string {
	width, height := InnerSize(outerWidth, outerHeight)
	return Frame(title, body, width, height, focused)
}

// ScrollWindow returns the [start, end) slice bounds of a scrollable list of
// totalItems entries, sized so exactly visibleRows are shown, such that
// cursor always falls within [start, end). It is a pure function of
// (cursor, totalItems, visibleRows) — callers don't need to persist a
// scroll offset across renders. If totalItems fits within visibleRows,
// start is always 0 and end is totalItems.
func ScrollWindow(cursor, totalItems, visibleRows int) (start, end int) {
	if visibleRows < 1 {
		visibleRows = 1
	}
	if totalItems <= visibleRows {
		return 0, totalItems
	}

	maxStart := totalItems - visibleRows
	start = cursor - visibleRows + 1
	if start < 0 {
		start = 0
	}
	if start > maxStart {
		start = maxStart
	}
	end = start + visibleRows
	return start, end
}

// InnerSize converts a panel's total outer footprint (border included) into
// the content width/height available inside the border, clamped to sane
// minimums. Since the title now lives in the top border line rather than a
// content row, callers get the full outerHeight-2 rows for body content.
func InnerSize(outerWidth, outerHeight int) (width, height int) {
	width = outerWidth - 2 // left/right border columns
	if width < minWidth {
		width = minWidth
	}
	height = outerHeight - 2 // top/bottom border rows
	if height < minHeight {
		height = minHeight
	}
	return width, height
}

// Frame wraps body (already sized to width columns and up to height rows)
// in a rounded border of exactly width+2 x height+2, with title embedded in
// the top border line rather than as a separate content row.
//
// An optional footer (e.g. a lazygit-style "3/10" scroll position
// indicator) may be passed, which is embedded right-aligned in the bottom
// border line instead of consuming a content row. Only the first footer
// argument is used; it exists as a variadic parameter purely so existing
// callers that don't need a footer aren't required to pass "".
func Frame(title, body string, width, height int, focused bool, footer ...string) string {
	lineStyle := lipgloss.NewStyle().Foreground(borderColor(focused))
	top := topBorder(title, width, focused, lineStyle)
	return frameBody(top, firstFooter(footer), body, width, height, lineStyle)
}

// firstFooter returns the first element of footer, or "" if it's empty, so
// Frame/FrameTabs can accept an optional trailing footer argument.
func firstFooter(footer []string) string {
	if len(footer) == 0 {
		return ""
	}
	return footer[0]
}

// Tab describes one segment of a multi-tab panel title (see FrameTabs),
// e.g. the Tasks panel's Todo/Done/Deleted status tabs.
type Tab struct {
	Label  string
	Active bool
}

// FrameTabs is like Frame, but embeds a row of tabs in the title instead of
// a single label (e.g. "╭─[2]-Todo - Done - Deleted─╮"), styling the
// active tab distinctly (inverted) from the others so the current tab is
// visually obvious. Like Frame, an optional footer may be passed to embed
// a right-aligned scroll position indicator in the bottom border.
func FrameTabs(number rune, tabs []Tab, body string, width, height int, focused bool, footer ...string) string {
	lineStyle := lipgloss.NewStyle().Foreground(borderColor(focused))
	top := topBorderTabs(number, tabs, width, focused, lineStyle)
	return frameBody(top, firstFooter(footer), body, width, height, lineStyle)
}

// frameBody renders the shared border/content assembly used by both Frame
// and FrameTabs, given an already-built top border line and an optional
// footer to embed in the bottom border line.
func frameBody(top, footer, body string, width, height int, lineStyle lipgloss.Style) string {
	bottom := bottomBorder(footer, width, lineStyle)

	// Height/Width here size only the content area; the border characters
	// are added separately above/below/around it.
	content := lipgloss.NewStyle().Width(width).Height(height).Render(body)

	var b strings.Builder
	b.WriteString(top)
	b.WriteString("\n")
	side := lineStyle.Render("│")
	for _, line := range strings.Split(content, "\n") {
		b.WriteString(side)
		b.WriteString(line)
		b.WriteString(side)
		b.WriteString("\n")
	}
	b.WriteString(bottom)
	return b.String()
}

// topBorder builds the top border line with the title embedded, e.g.
// "╭──[1]-Status────────╮". If the title (plus decoration) doesn't fit
// within width, it's truncated to avoid overflowing the panel.
func topBorder(title string, width int, focused bool, lineStyle lipgloss.Style) string {
	label := titleLabel(title)
	if len(label) > width {
		label = label[:width]
	}

	const leadDashes = 2
	fill := width - leadDashes - len(label)
	if fill < 0 {
		fill = 0
	}

	var b strings.Builder
	b.WriteString(lineStyle.Render("╭" + strings.Repeat("─", leadDashes)))
	b.WriteString(TitleStyle(focused).Render(label))
	b.WriteString(lineStyle.Render(strings.Repeat("─", fill) + "╮"))
	return b.String()
}

// bottomBorder builds the bottom border line, optionally embedding footer
// right-aligned near the corner (lazygit-style, e.g.
// "╰──────────────────3/10─╮"). If footer is empty, a plain unbroken
// border is returned. If footer doesn't fit within width, it's dropped
// entirely rather than truncated (a garbled "3/1…" position indicator is
// worse than no indicator at all).
func bottomBorder(footer string, width int, lineStyle lipgloss.Style) string {
	if footer == "" || len(footer) > width {
		return lineStyle.Render("╰" + strings.Repeat("─", width) + "╯")
	}

	const trailDashes = 1
	fill := width - trailDashes - len(footer)
	if fill < 0 {
		fill = 0
	}

	var b strings.Builder
	b.WriteString(lineStyle.Render("╰" + strings.Repeat("─", fill)))
	b.WriteString(footerStyle().Render(footer))
	b.WriteString(lineStyle.Render(strings.Repeat("─", trailDashes) + "╯"))
	return b.String()
}

// footerStyle renders a Frame/FrameTabs footer (e.g. a scroll position
// indicator) in a dim, unobtrusive color independent of panel focus, since
// it's informational rather than a focus/selection cue.
func footerStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(inactiveTabColor)
}

// activeTabStyle highlights whichever tab is currently selected within a
// FrameTabs title using activeTabColor, distinct from both the panel's
// focus-dependent border/prefix color and inactiveTabStyle, so the active
// tab is identifiable regardless of whether the panel itself has focus.
func activeTabStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(activeTabColor)
}

// inactiveTabStyle is used for the non-selected tabs in a FrameTabs title.
// It deliberately does not vary with panel focus (unlike TitleStyle), so it
// never collides with activeTabColor when the panel is focused.
func inactiveTabStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(inactiveTabColor)
}

// topBorderTabs builds the top border line for FrameTabs, e.g.
// "╭──[2]-Todo - Done - Deleted────╮", with the active tab styled
// distinctly from the others. If the full tab row doesn't fit within
// width, it falls back to a single truncated, unstyled label (via
// topBorder) rather than trying to partially render styled tabs.
func topBorderTabs(number rune, tabs []Tab, width int, focused bool, lineStyle lipgloss.Style) string {
	prefix := "[" + string(number) + "]-"

	labels := make([]string, len(tabs))
	for i, t := range tabs {
		labels[i] = t.Label
	}
	plain := prefix + strings.Join(labels, " - ")

	const leadDashes = 2
	if len(plain) > width {
		return topBorder(plain, width, focused, lineStyle)
	}

	var b strings.Builder
	b.WriteString(lineStyle.Render("╭" + strings.Repeat("─", leadDashes)))
	b.WriteString(TitleStyle(focused).Render(prefix))
	for i, t := range tabs {
		if i > 0 {
			b.WriteString(inactiveTabStyle().Render(" - "))
		}
		style := inactiveTabStyle()
		if t.Active {
			style = activeTabStyle()
		}
		b.WriteString(style.Render(t.Label))
	}
	fill := width - leadDashes - len(plain)
	if fill < 0 {
		fill = 0
	}
	b.WriteString(lineStyle.Render(strings.Repeat("─", fill) + "╮"))
	return b.String()
}

// titleLabel formats a panel title for the border line. Titles of the form
// "<digit> <name>" (the number-key hint, e.g. "1 Status") become
// "[1]-Status"; any other title is used as-is.
func titleLabel(title string) string {
	if len(title) > 2 && title[1] == ' ' && title[0] >= '0' && title[0] <= '9' {
		return "[" + string(title[0]) + "]-" + title[2:]
	}
	return title
}
