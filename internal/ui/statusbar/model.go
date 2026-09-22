// Package statusbar renders a lazygit-style bottom help bar showing the
// keybindings active for whatever panel/mode currently has focus. It has
// no knowledge of taskwarrior or any specific panel — callers pass in the
// list of bindings relevant to their current mode.
package statusbar

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// keyStyle uses ANSI blue (SGR 34), matching lazygit's default
	// options-bar color (theme.OptionsTextColor: "blue").
	keyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("4"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	separator = "  "
)

// Binding describes a single keybinding entry shown in the status bar, e.g.
// {Key: "a", Label: "add"}.
type Binding struct {
	Key   string
	Label string
}

// Render lays out bindings as a single lazygit-style line, e.g.
// "a add  d done  x delete  q quit". Each entry pairs a highlighted key
// with a dimmed label. Returns an empty string if bindings is empty.
func Render(bindings []Binding) string {
	if len(bindings) == 0 {
		return ""
	}

	parts := make([]string, 0, len(bindings))
	for _, b := range bindings {
		parts = append(parts, keyStyle.Render(b.Key)+" "+labelStyle.Render(b.Label))
	}
	return strings.Join(parts, separator)
}
