// Package helppopup renders the keybinding reference shown when the user
// presses "?" (lazygit-style "Keybindings" menu). Unlike internal/ui/popup
// (transient errors/confirmations) or internal/ui/statusbar (the
// always-visible, focus-scoped hint line), this lists every keybinding
// relevant to the current context at once, grouped under section headers,
// styled to match lazygit's own keybindings menu
// (jesseduffield/lazygit pkg/gui/controllers/options_menu_action.go,
// pkg/gui/context/menu_context.go and pkg/gui/context/list_renderer.go):
// a neutral (not accent-colored) bordered box with the title embedded in
// the top border (via internal/ui/panel.Frame, this repo's existing
// title-in-border pattern), bold-green "--- Title ---" section headers
// indented under the description column, a cyan right-aligned key
// column, and default-colored description text. Scrollable (see
// MaxScroll) since a context with many local bindings can still exceed
// one screen; the visible row budget scales with the terminal's actual
// height (see VisibleRows) rather than a fixed constant, since lazygit's
// own menu is a large, near-full-height box, not a small fixed dialog.
package helppopup

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/tkilb/lazytask/internal/ui/panel"
	"github.com/tkilb/lazytask/internal/ui/statusbar"
)

// Section is a named group of bindings shown under its own header, e.g.
// "Local", "Global", "Navigation" (lazygit's own three keybindings-menu
// section names, reused here for the same meaning: Local is whatever
// panel currently has focus, Global applies everywhere, Navigation is
// panel-switching/list-movement).
type Section struct {
	Title    string
	Bindings []statusbar.Binding
}

// headerStyle/keyStyle mirror lazygit's own styling: bold green section
// headers (list_renderer.go's formatListSectionHeader, rendered via
// style.FgGreen.SetBold()), reusing this app's established
// panel.FocusedColor (lazygit's default activeBorderColor) rather than a
// second, potentially-drifting green literal; and a bold cyan key column
// (menu_context.go's GetDisplayStrings hardcodes style.FgCyan for the key
// label regardless of theme). Description text is intentionally left
// unstyled (lazygit renders it in the terminal's plain default
// foreground, not dimmed).
var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(panel.FocusedColor)
	keyStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
)

const (
	minWidth = 40
	maxWidth = 80

	// minVisibleRows/maxVisibleRows bound the scrollable body viewport's
	// height (in lines): never shorter than minVisibleRows even on a
	// tiny terminal, never taller than maxVisibleRows even on a huge
	// one. Actual height (see VisibleRows) scales with the real
	// terminal size between those bounds, matching lazygit's own menu
	// (a large, near-full-height box, not a small fixed dialog).
	minVisibleRows = 8
	maxVisibleRows = 40

	// reservedRows is how much of the terminal's height VisibleRows
	// reserves for the box's own top/bottom border plus a small margin
	// above/below so the box never touches the screen edges.
	reservedRows = 6
)

// VisibleRows returns how many body lines Box should show given the
// terminal's actual height, scaling between minVisibleRows and
// maxVisibleRows. screenHeight <= 0 (unknown) falls back to
// maxVisibleRows, matching the fixed-size behavior before this was made
// height-aware.
func VisibleRows(screenHeight int) int {
	if screenHeight <= 0 {
		return maxVisibleRows
	}
	rows := screenHeight - reservedRows
	if rows > maxVisibleRows {
		rows = maxVisibleRows
	}
	if rows < minVisibleRows {
		rows = minVisibleRows
	}
	return rows
}

// sectionHeader formats a section title the way this popup's screenshot-
// verified visual target does: a "--- Title ---" rule, symmetric on both
// sides (lazygit's own list_renderer.go formatListSectionHeader emits a
// left-only rule, but the actual rendered "Keybindings" menu shows a
// trailing rule too).
func sectionHeader(title string) string {
	return "--- " + title + " ---"
}

// keyWidth returns the longest binding key's display width across every
// section, so Box can right-align every key to a shared column
// regardless of which keys happen to be in play for the current context.
// Uses ansi.StringWidth (display-column width) rather than len (byte
// count), since several keys (e.g. "↑/k") contain multi-byte-but-
// single-column runes — byte length would misalign those against
// same-width ASCII keys.
func keyWidth(sections []Section) int {
	width := 0
	for _, sec := range sections {
		for _, b := range sec.Bindings {
			if w := ansi.StringWidth(b.Key); w > width {
				width = w
			}
		}
	}
	return width
}

// bodyLines flattens sections into one pre-rendered line per row (a blank
// separator before every section but the first, then its header indented
// to align under the description column — matching lazygit's own menu,
// where section headers occupy the description column rather than the
// key column — then one right-aligned-key/label line per binding), the
// unit MaxScroll and Box's viewport windowing both operate on.
func bodyLines(sections []Section) []string {
	width := keyWidth(sections)
	headerIndent := strings.Repeat(" ", width+2)
	var lines []string
	for i, sec := range sections {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, headerIndent+headerStyle.Render(sectionHeader(sec.Title)))
		for _, bind := range sec.Bindings {
			key := keyStyle.Render(padLeft(bind.Key, width))
			lines = append(lines, key+"  "+bind.Label)
		}
	}
	return lines
}

// MaxScroll returns the largest valid scrollOffset for Box given sections
// and the terminal's actual height (see VisibleRows): 0 if the full
// reference already fits within that budget, otherwise the offset that
// scrolls exactly to the last line. Callers (key handling) should clamp
// their scroll position to [0, MaxScroll(sections, screenHeight)].
func MaxScroll(sections []Section, screenHeight int) int {
	total := len(bodyLines(sections))
	visible := VisibleRows(screenHeight)
	if total <= visible {
		return 0
	}
	return total - visible
}

// Box renders sections as a single bordered, scrollable "Keybindings"
// reference (title embedded in the top border, one section per header,
// right-aligned key/label pairs, and a bottom-border-embedded
// scroll-position + close hint), sized to fit within screenWidth and
// screenHeight. scrollOffset selects which VisibleRows(screenHeight)-tall
// slice of the full (possibly longer) content is visible; it's clamped
// internally to [0, MaxScroll(sections, screenHeight)], so out-of-range
// values are safe to pass. Combine with popup.Overlay to composite it
// centered over an existing view.
func Box(sections []Section, screenWidth, screenHeight, scrollOffset int) string {
	boxWidth := maxWidth
	if screenWidth > 0 && screenWidth-4 < boxWidth {
		boxWidth = screenWidth - 4
	}
	if boxWidth < minWidth {
		boxWidth = minWidth
	}
	// panel.Frame's width/height describe the content area inside the
	// border, so the outer box (what screenWidth/boxWidth describe) is
	// 2 columns wider than what's passed to Frame.
	contentWidth := boxWidth - 2

	lines := bodyLines(sections)
	visibleRows := VisibleRows(screenHeight)
	maxScroll := MaxScroll(sections, screenHeight)
	if scrollOffset < 0 {
		scrollOffset = 0
	}
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}
	end := scrollOffset + visibleRows
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[scrollOffset:end]

	footer := "esc/? close"
	if maxScroll > 0 {
		footer = fmt.Sprintf("%d-%d/%d  \u2191/\u2193 scroll  ", scrollOffset+1, end, len(lines)) + footer
	}

	body := strings.Join(visible, "\n")
	// focused=true: the help popup is the actively focused panel while
	// open (it owns input), so its border uses the app's standard
	// focused-panel green, same convention as every other panel here.
	return panel.Frame("Keybindings", body, contentWidth, len(visible), true, footer)
}

// padLeft pads s with leading spaces to at least width display columns
// (right-aligning s within that width), leaving s unchanged if it's
// already that wide or wider. Uses ansi.StringWidth rather than len, for
// the same multi-byte-single-column-rune reason as keyWidth.
func padLeft(s string, width int) string {
	w := ansi.StringWidth(s)
	if w >= width {
		return s
	}
	return strings.Repeat(" ", width-w) + s
}
