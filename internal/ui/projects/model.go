// Package projects implements the lazygit-style bordered Projects panel: a
// selectable list of the distinct project names currently known to
// taskwarrior, plus two special entries pinned to the top for clearing
// the project filter or explicitly selecting tasks with no project.
package projects

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tkilb/lazytask/internal/ui/panel"
)

const (
	// NoneLabel selects tasks that have no project set (`project:`).
	NoneLabel = "(none)"
	// AllLabel clears the project filter entirely (no `project:` filter
	// applied at all).
	AllLabel = "(all)"
	// NewProjectLabel, in reassign mode (see SetReassignMode), selects the
	// "key in a new project name" entry in place of AllLabel (which has no
	// meaning when reassigning a single task's project).
	NewProjectLabel = "+ New Project"

	// minPanelWidth is used when no tea.WindowSizeMsg has been received yet.
	minPanelWidth = 20
)

var selectedRowStyle = lipgloss.NewStyle().Background(panel.SelectedRowBackground).Bold(true)

// searchMatchColor/searchMatchColorUnfocused highlight the substring
// matching an active `/` search query, mirroring the Tasks panel's search
// highlighting (see internal/ui/tasklist): teal for the current cursor
// row's match, yellow for every other matching row, so the row n/N are
// jumping to/from still stands out from the rest of the matches.
var (
	searchMatchColor          = lipgloss.Color("6")
	searchMatchColorUnfocused = lipgloss.Color("3")
	searchMatchTextColor      = lipgloss.Color("0")
)

// Model is a Bubble Tea model rendering a bordered, selectable list of
// distinct project names, plus the NoneLabel/AllLabel special entries
// pinned to the top.
//
// This panel does not fetch or dedupe tasks itself: the caller (main.go)
// is expected to source the project names from an unfiltered task query
// (so applying a project filter doesn't shrink the very list used to
// change/clear that filter) and pass the already-deduplicated, sorted
// names to SetProjects.
type Model struct {
	projects     []string
	counts       Counts
	cursor       int
	width        int
	height       int
	focused      bool
	title        string
	reassignMode bool
	searchQuery  string
}

// Counts carries the number of tasks behind each selectable entry, so the
// panel can render an "[N]" suffix next to each project name (and the
// AllLabel/NoneLabel special entries). It is expected to be sourced from
// the same task set used to derive the distinct project names, so the
// counts stay consistent with what's actually selectable/filterable here.
type Counts struct {
	// All is the total task count across every project (what AllLabel
	// represents).
	All int
	// None is the count of tasks with no project set (what NoneLabel
	// represents).
	None int
	// ByProject maps a project name to its task count.
	ByProject map[string]int
}

// defaultTitle is the panel title shown in the grid, with the "3" number
// key hint lazygit-style panels embed in their border. SetTitle overrides
// this for callers rendering the same Model as a standalone popup instead
// (e.g. the "p" quick project-filter popup), where the number key hint
// isn't meaningful.
const defaultTitle = "3 Projects"

// New constructs an empty Model.
func New() Model {
	return Model{}
}

// SetTitle overrides the panel title embedded in View's top border,
// replacing the default "3 Projects" grid-panel title. Used by callers
// rendering this Model standalone (e.g. as a popup) where the "3" number
// key hint doesn't apply.
func (m Model) SetTitle(title string) Model {
	m.title = title
	return m
}

// SetReassignMode switches the panel between its default entry set (the
// AllLabel/NoneLabel special entries pinned to the top, used by the
// Projects grid panel and the "p" quick project-filter popup) and a
// reassign-picker entry set (NoneLabel + NewProjectLabel), used when this
// same Model is reused as the "P" task project-reassign popup: there is no
// "(all)" project to reassign a single task to, but there does need to be
// a way to key in a brand new project name rather than picking an existing
// one. The cursor is clamped to stay within bounds of the resulting entry
// list, same as SetProjects.
func (m Model) SetReassignMode(reassign bool) Model {
	m.reassignMode = reassign
	entries := m.entries()
	if m.cursor >= len(entries) {
		m.cursor = len(entries) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	return m
}

// SetProjects replaces the underlying distinct project names (expected
// already deduplicated/sorted by the caller), clamping the cursor so it
// remains within bounds of the resulting entry list (projects plus the two
// special entries).
func (m Model) SetProjects(projectNames []string) Model {
	m.projects = projectNames
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

// SelectLabel moves the cursor to the given entry (a project name, or the
// NoneLabel/AllLabel special entries) if it exists in the current entry
// list, leaving the cursor unchanged otherwise (e.g. the label no longer
// exists as a project). Used to keep the panel's visible selection in sync
// with a filter applied/restored from elsewhere (e.g. main.go's shared
// filterState, including a project filter persisted from a prior session).
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
// project name, or the AllLabel/NoneLabel special entries). Callers use
// this to detect a stale filter — e.g. a project filter restored from a
// previous session whose project no longer has any pending tasks, and so
// no longer appears in the panel — since SelectLabel silently leaves the
// cursor unchanged in that case rather than reporting the mismatch.
func (m Model) HasLabel(label string) bool {
	for _, entry := range m.entries() {
		if entry == label {
			return true
		}
	}
	return false
}

// entries returns the full selectable list: the two special entries
// pinned to the top (AllLabel/NoneLabel normally, or NoneLabel/
// NewProjectLabel in reassign mode — see SetReassignMode), followed by the
// real project names.
func (m Model) entries() []string {
	entries := make([]string, 0, len(m.projects)+2)
	if m.reassignMode {
		entries = append(entries, NoneLabel, NewProjectLabel)
	} else {
		entries = append(entries, AllLabel, NoneLabel)
	}
	entries = append(entries, m.projects...)
	return entries
}

// Projects returns the current distinct project names (as passed to the
// last SetProjects call), excluding the AllLabel/NoneLabel special entries.
// Callers use this to check a candidate name against the existing set
// (e.g. detecting a rename that would merge into another project).
func (m Model) Projects() []string {
	return append([]string(nil), m.projects...)
}

// ClearSearch discards the active search query (if any), so the panel
// stops highlighting matches and NextMatch/PrevMatch become no-ops again.
// Bound to `esc` from the Projects panel (and, in the "p" quick
// project-filter popup, only once no search is active does `esc` fall
// through to closing the popup). It never moves the cursor.
func (m Model) ClearSearch() Model {
	m.searchQuery = ""
	return m
}

// HasActiveSearch reports whether a "/" search query is currently active,
// used by callers (e.g. the "p" quick project-filter popup) that need to
// clear an in-progress search on a first "esc" before falling through to
// their own "esc" semantics (e.g. closing the popup) on a second press.
func (m Model) HasActiveSearch() bool {
	return m.searchQuery != ""
}

// Search sets query as the active search (matched case-insensitively
// against each entry's name, per requirements.md Phase 1's continuation
// into the Projects panel/quick-filter popup), then jumps the cursor to
// the nearest match at or after the current position, wrapping around to
// the top of the list if needed. It returns the updated Model and whether
// any match was found; the query stays recorded either way so
// NextMatch/PrevMatch keep working (or keep silently no-op'ing) for repeat
// n/N presses. The entry list itself is never filtered — this only ever
// moves the cursor.
func (m Model) Search(query string) (Model, bool) {
	m.searchQuery = query
	i, ok := m.findMatch(m.cursor, 1, true)
	if ok {
		m.cursor = i
	}
	return m, ok
}

// NextMatch moves the cursor to the next entry (forward, wrapping) whose
// name contains the active search query, bound to "n". It is a no-op
// (returning ok=false) if there is no active query or no entry matches.
func (m Model) NextMatch() (Model, bool) {
	if m.searchQuery == "" {
		return m, false
	}
	i, ok := m.findMatch(m.cursor, 1, false)
	if ok {
		m.cursor = i
	}
	return m, ok
}

// PrevMatch moves the cursor to the previous entry (backward, wrapping)
// whose name contains the active search query, bound to "N". It is a
// no-op (returning ok=false) if there is no active query or no entry
// matches.
func (m Model) PrevMatch() (Model, bool) {
	if m.searchQuery == "" {
		return m, false
	}
	i, ok := m.findMatch(m.cursor, -1, false)
	if ok {
		m.cursor = i
	}
	return m, ok
}

// findMatch scans the current entry list in direction (+1/-1) starting
// from start, wrapping around the ends of the list, and returns the index
// of the first entry whose name matches m.searchQuery (case-insensitive
// substring), and true. If includeStart is true the start index itself is
// included in the scan (used by Search's "jump to first match" semantics);
// otherwise the scan begins one step past start (used by
// NextMatch/PrevMatch, so repeated presses always advance rather than
// getting stuck on the current match). Returns (0, false) if there are no
// entries or nothing matches.
func (m Model) findMatch(start, direction int, includeStart bool) (int, bool) {
	entries := m.entries()
	n := len(entries)
	if n == 0 || m.searchQuery == "" {
		return 0, false
	}
	i := start
	if !includeStart {
		i += direction
	}
	i = ((i % n) + n) % n
	for step := 0; step < n; step++ {
		if matchesQuery(entries[i], m.searchQuery) {
			return i, true
		}
		i = ((i+direction)%n + n) % n
	}
	return 0, false
}

// matchesQuery reports whether name contains query as a case-insensitive
// substring.
func matchesQuery(name, query string) bool {
	return strings.Contains(strings.ToLower(name), strings.ToLower(query))
}

// Selected returns the entry currently under the cursor (a project name, or
// NoneLabel/AllLabel) and true, or "" and false if there are no entries
// (which should not normally happen, since the two special entries are
// always present).
func (m Model) Selected() (string, bool) {
	entries := m.entries()
	if len(entries) == 0 {
		return "", false
	}
	return entries[m.cursor], true
}

// SetFocused records whether the Projects panel currently has focus in the
// surrounding panel grid, so View can apply the shared focused-panel border
// highlight (see internal/ui/panel).
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
		lines = append(lines, m.displayLine(entries[i], innerWidth, m.searchQuery, i == m.cursor))
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
// edge, so counts form a straight column down the panel. When selected is
// true, the row-highlight background/bold (matching selectedRowStyle) is
// applied across the whole row. When query is non-empty (an active `/`
// search), any substring of entry's name matching query
// (case-insensitively) is rendered with searchMatchColor on the current
// cursor row (selected) or searchMatchColorUnfocused on every other
// matching row, mirroring the Tasks panel's search highlighting.
func (m Model) displayLine(entry string, width int, query string, selected bool) string {
	base := lipgloss.NewStyle()
	if selected {
		base = selectedRowStyle
	}
	field := countField(m.count(entry))
	if width <= 0 {
		return base.Render(entry + " " + field)
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
	return renderWithMatches(name, query, base, selected) +
		base.Render(strings.Repeat(" ", pad)+field)
}

// renderWithMatches renders s split around every non-overlapping,
// case-insensitive occurrence of query: matched substrings use base with
// searchMatchColor (on the current cursor row, selected) or
// searchMatchColorUnfocused (every other row) as the background, never
// bold (even on the selected row, whose base style is otherwise bold) so
// the highlighted text always reads the same weight; everything else uses
// base as-is. If query is empty (no active search) the whole string is
// rendered with base, unchanged.
func renderWithMatches(s, query string, base lipgloss.Style, selected bool) string {
	if query == "" {
		return base.Render(s)
	}
	highlight := searchMatchColorUnfocused
	if selected {
		highlight = searchMatchColor
	}
	matchStyle := base.Background(highlight).Foreground(searchMatchTextColor).Bold(false)

	lowerS := strings.ToLower(s)
	lowerQ := strings.ToLower(query)

	var b strings.Builder
	i := 0
	for i < len(s) {
		idx := strings.Index(lowerS[i:], lowerQ)
		if idx < 0 {
			b.WriteString(base.Render(s[i:]))
			break
		}
		idx += i
		if idx > i {
			b.WriteString(base.Render(s[i:idx]))
		}
		matchEnd := idx + len(query)
		b.WriteString(matchStyle.Render(s[idx:matchEnd]))
		i = matchEnd
	}
	return b.String()
}

// countField renders n as "(n)", right-aligned/padded to countFieldWidth.
func countField(n int) string {
	s := fmt.Sprintf("(%d)", n)
	if pad := countFieldWidth - len([]rune(s)); pad > 0 {
		s = strings.Repeat(" ", pad) + s
	}
	return s
}

// count returns the task count backing entry (AllLabel/NoneLabel/a project
// name), sourced from m.counts.
func (m Model) count(entry string) int {
	switch entry {
	case AllLabel:
		return m.counts.All
	case NoneLabel:
		return m.counts.None
	default:
		return m.counts.ByProject[entry]
	}
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
