package statusbar

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/tkilb/lazytask/internal/ui/panel"
)

var (
	// suggestedTitleStyle uses ANSI cyan (teal) rather than blue so the reason
	// label stays legible on the blue selected-row background.
	suggestedTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	suggestedDescStyle  = lipgloss.NewStyle().Bold(true)
)

// RenderSuggested lays out one Suggested row in exactly width columns when
// selected, e.g. "started: Write report                      [Work]": the
// reason and description on the left, the project right-aligned in
// brackets. The left part is truncated with an ellipsis so it never crowds
// the project (one column of gap is always kept). An empty description
// renders the nothing-to-suggest state; an empty project omits the
// bracketed part. A project too wide to share the row is dropped rather
// than truncated. When selected, every cell gets the shared
// selected-row background and the row is padded to the full width so it
// reads as a highlight bar.
func RenderSuggested(reason, description, project string, width int, selected bool) string {
	style := func(st lipgloss.Style) lipgloss.Style {
		if selected {
			return st.Background(panel.SelectedRowBackground)
		}
		return st
	}
	pad := func(s string) string {
		if gap := width - lipgloss.Width(s); selected && gap > 0 {
			return s + style(lipgloss.NewStyle()).Render(strings.Repeat(" ", gap))
		}
		return s
	}

	if description == "" {
		return pad(ansi.Truncate(style(labelStyle).Render("nothing pending"), width, "…"))
	}
	var left string
	if reason != "" {
		left = style(suggestedTitleStyle).Render(reason) + style(lipgloss.NewStyle()).Render(": ")
	}
	left += style(suggestedDescStyle).Render(description)

	right := ""
	if project != "" {
		right = style(labelStyle).Render("[" + project + "]")
	}
	rightWidth := lipgloss.Width(right)
	if rightWidth == 0 || rightWidth+2 > width {
		return pad(ansi.Truncate(left, width, "…"))
	}
	left = ansi.Truncate(left, width-rightWidth-1, "…")
	gap := width - lipgloss.Width(left) - rightWidth
	return left + style(lipgloss.NewStyle()).Render(strings.Repeat(" ", gap)) + right
}
