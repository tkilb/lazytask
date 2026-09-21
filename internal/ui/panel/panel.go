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

const (
	// UnfocusedColor is the border/title color for a panel that does not
	// currently have focus.
	UnfocusedColor = lipgloss.Color("62")
	// FocusedColor is the border/title color for the currently focused
	// panel.
	FocusedColor = lipgloss.Color("212")

	// minWidth/minHeight guard against nonsensical (zero or negative)
	// panel sizes, e.g. before the first tea.WindowSizeMsg arrives.
	minWidth  = 10
	minHeight = 3
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
func Frame(title, body string, width, height int, focused bool) string {
	color := borderColor(focused)
	lineStyle := lipgloss.NewStyle().Foreground(color)

	top := topBorder(title, width, focused, lineStyle)
	bottom := lineStyle.Render("╰" + strings.Repeat("─", width) + "╯")

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

// titleLabel formats a panel title for the border line. Titles of the form
// "<digit> <name>" (the number-key hint, e.g. "1 Status") become
// "[1]-Status"; any other title is used as-is.
func titleLabel(title string) string {
	if len(title) > 2 && title[1] == ' ' && title[0] >= '0' && title[0] <= '9' {
		return "[" + string(title[0]) + "]-" + title[2:]
	}
	return title
}
