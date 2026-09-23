// Package tags implements the lazygit-style bordered Tags panel: a
// selectable list of the distinct tags currently known to taskwarrior,
// plus an AnyLabel entry pinned to the top to clear the tag filter.
package tags

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tkilb/lazytask/internal/ui/panel"
)

// AnyLabel clears the tag filter entirely (no `+tag` filter applied at
// all). It is wrapped in `*` rather than parens (unlike projects.AllLabel)
// to match the "(any)" wording of the requirement this panel implements.
const AnyLabel = "(any)"

// minPanelWidth is used when no tea.WindowSizeMsg has been received yet.
const minPanelWidth = 20

var selectedRowStyle = lipgloss.NewStyle().Background(panel.SelectedRowBackground).Bold(true)

// Model is a Bubble Tea model rendering a bordered, selectable list of
// distinct tags, plus the AnyLabel special entry pinned to the top.
//
// This panel does not fetch or dedupe tasks itself: the caller (main.go)
// is expected to source the tag names from an unfiltered task query (so
// applying a tag filter doesn't shrink the very list used to change/clear
// that filter) and pass the already-deduplicated, sorted names to SetTags.
type Model struct {
	tags    []string
	counts  Counts
	cursor  int
	width   int
	height  int
	focused bool
	title   string
}

// Counts carries the number of tasks behind each selectable entry, so the
// panel can render an "[N]" suffix next to each tag name (and the AnyLabel
// special entry). It is expected to be sourced from the same task set used
// to derive the distinct tag names, so the counts stay consistent with
// what's actually selectable/filterable here.
type Counts struct {
	// All is the total task count across every tag (what AnyLabel
	// represents).
	All int
	// ByTag maps a tag name to its task count.
	ByTag map[string]int
}

// defaultTitle is the panel title shown in the grid, with the "4" number
// key hint lazygit-style panels embed in their border. SetTitle overrides
// this for callers rendering the same Model as a standalone popup instead
// (e.g. the "t" quick tag-filter popup), where the number key hint isn't
// meaningful.
const defaultTitle = "4 Tags"

// New constructs an empty Model.
func New() Model {
	return Model{}
}

// SetTitle overrides the panel title embedded in View's top border,
// replacing the default "4 Tags" grid-panel title. Used by callers
// rendering this Model standalone (e.g. as a popup) where the "4" number
// key hint doesn't apply.
func (m Model) SetTitle(title string) Model {
	m.title = title
	return m
}

// SetTags replaces the underlying distinct tag names (expected already
// deduplicated/sorted by the caller), clamping the cursor so it remains
// within bounds of the resulting entry list (tags plus AnyLabel).
func (m Model) SetTags(tagNames []string) Model {
	m.tags = tagNames
	entries := m.entries()
	if m.cursor >= len(entries) {
		m.cursor = len(entries) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	return m
}

// SetCounts replaces the per-entry task counts shown as an "[N]" suffix
// next to each entry in View. It does not affect Selected/navigation,
// which continue to operate on the raw entry names.
func (m Model) SetCounts(counts Counts) Model {
	m.counts = counts
	return m
}

// SelectLabel moves the cursor to the given entry (a tag name, or the
// AnyLabel special entry) if it exists in the current entry list, leaving
// the cursor unchanged otherwise (e.g. the label no longer exists as a
// tag). Used to keep the panel's visible selection in sync with a filter
// applied/restored from elsewhere (e.g. main.go's shared filterState,
// including a tag filter persisted from a prior session).
func (m Model) SelectLabel(label string) Model {
	for i, entry := range m.entries() {
		if entry == label {
			m.cursor = i
			break
		}
	}
	return m
}

// HasLabel reports whether label is a currently selectable entry (a known
// tag name, or the AnyLabel special entry). Callers use this to detect a
// stale filter — e.g. a tag filter restored from a previous session whose
// tag no longer has any pending tasks, and so no longer appears in the
// panel — since SelectLabel silently leaves the cursor unchanged in that
// case rather than reporting the mismatch.
func (m Model) HasLabel(label string) bool {
	for _, entry := range m.entries() {
		if entry == label {
			return true
		}
	}
	return false
}

// entries returns the full selectable list: AnyLabel pinned to the top,
// followed by the real tag names.
func (m Model) entries() []string {
	entries := make([]string, 0, len(m.tags)+1)
	entries = append(entries, AnyLabel)
	entries = append(entries, m.tags...)
	return entries
}

// Tags returns the current distinct tag names (as passed to the last
// SetTags call), excluding the AnyLabel special entry.
func (m Model) Tags() []string {
	return append([]string(nil), m.tags...)
}

// Selected returns the entry currently under the cursor (a tag name, or
// AnyLabel) and true, or "" and false if there are no entries (which
// should not normally happen, since AnyLabel is always present).
func (m Model) Selected() (string, bool) {
	entries := m.entries()
	if len(entries) == 0 {
		return "", false
	}
	return entries[m.cursor], true
}

// SetFocused records whether the Tags panel currently has focus in the
// surrounding panel grid, so View can apply the shared focused-panel
// border highlight (see internal/ui/panel).
func (m Model) SetFocused(focused bool) Model {
	m.focused = focused
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model, handling up/down (and vim-style j/k)
// navigation and terminal resize events. Selection (enter) is handled by
// the caller, since it also needs to update the shared filter state.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		entries := m.entries()
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(entries)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	width := m.width
	if width <= 0 {
		width = minPanelWidth
	}
	innerWidth, innerHeight := panel.InnerSize(width, m.height)

	entries := m.entries()
	visibleRows := innerHeight
	if visibleRows < 1 {
		visibleRows = 1
	}
	start, end := panel.ScrollWindow(m.cursor, len(entries), visibleRows)

	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		line := m.displayLine(entries[i], innerWidth)
		if i == m.cursor {
			line = selectedRowStyle.Render(line)
		}
		lines = append(lines, line)
	}
	body := strings.Join(lines, "\n")

	footer := ""
	if len(entries) > 0 {
		footer = fmt.Sprintf("%d of %d", m.cursor+1, len(entries))
	}
	title := m.title
	if title == "" {
		title = defaultTitle
	}
	return panel.Frame(title, body, innerWidth, innerHeight, m.focused, footer)
}

// countFieldWidth is the fixed width reserved for the "(N)" count field
// (right-aligned within it), so every row's count lines up in the same
// column regardless of name length or digit count — e.g. "(4)" and
// "(1234)" both occupy the same field width, padded with leading spaces.
const countFieldWidth = 6 // fits up to "(9999)"

// displayLine renders entry's full row: its name, then a fixed-width
// "(N)" task-count field right-aligned flush against the panel's right
// edge, so counts form a straight column down the panel.
func (m Model) displayLine(entry string, width int) string {
	field := countField(m.count(entry))
	if width <= 0 {
		return entry + " " + field
	}
	// Reserve a space between name and count field; truncate the name if
	// the combination doesn't fit the available width.
	nameWidth := width - countFieldWidth - 1
	if nameWidth < 0 {
		nameWidth = 0
	}
	name := truncate(entry, nameWidth)
	pad := width - len([]rune(name)) - countFieldWidth
	if pad < 1 {
		pad = 1
	}
	return name + strings.Repeat(" ", pad) + field
}

// countField renders n as "(n)", right-aligned/padded to countFieldWidth.
func countField(n int) string {
	s := fmt.Sprintf("(%d)", n)
	if pad := countFieldWidth - len([]rune(s)); pad > 0 {
		s = strings.Repeat(" ", pad) + s
	}
	return s
}

// count returns the task count backing entry (AnyLabel/a tag name),
// sourced from m.counts.
func (m Model) count(entry string) int {
	if entry == AnyLabel {
		return m.counts.All
	}
	return m.counts.ByTag[entry]
}

// truncate shortens s to fit within width, adding an ellipsis if it was cut.
func truncate(s string, width int) string {
	if width <= 0 || len(s) <= width {
		return s
	}
	if width <= 1 {
		return s[:width]
	}
	return s[:width-1] + "…"
}
