// Package tasklist implements the lazygit-style bordered task list panel.
//
// This chunk renders whatever []taskwarrior.Task it is constructed with —
// it does not know how to fetch tasks itself. Live taskwarrior wiring is
// added in a later chunk (see requirements.md chunk 5).
package tasklist

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/panel"
)

const (
	// minPanelWidth is used when no tea.WindowSizeMsg has been received yet.
	minPanelWidth = 60
)

// StatusTab identifies which of the Tasks panel's status tabs
// (Todo/Done/Deleted) is currently selected.
type StatusTab int

const (
	TabTodo StatusTab = iota
	TabDone
	TabDeleted
)

// statusTabs is the fixed cycle order used by NextStatus/PrevStatus and by
// View when rendering the title's tab row.
var statusTabs = []StatusTab{TabTodo, TabDone, TabDeleted}

// Label returns the display name shown in the panel title for this tab.
func (t StatusTab) Label() string {
	switch t {
	case TabDone:
		return "Done"
	case TabDeleted:
		return "Deleted"
	default:
		return "Todo"
	}
}

// Filter returns the `task export` status filter corresponding to this tab.
func (t StatusTab) Filter() string {
	switch t {
	case TabDone:
		return "status:completed"
	case TabDeleted:
		return "status:deleted"
	default:
		return "status:pending"
	}
}

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("245"))

	// searchMatchColor is the background used to highlight the substring
	// that matched the active `/` search query within the current cursor
	// row's Description cell. Teal, kept visually distinct from
	// panel.SelectedRowBackground (blue) and the priority colors (red/
	// yellow/cyan) so it reads clearly even on the current cursor row.
	searchMatchColor = lipgloss.Color("6")
	// searchMatchColorUnfocused is used for the same search-match
	// highlight on every row other than the current cursor row, so the
	// row you're actually on (and where n/N are jumping to/from) still
	// stands out from the rest of the matches.
	searchMatchColorUnfocused = lipgloss.Color("3")
)

// priorityColor returns the row foreground color for a task's priority
// field ("H"/"M"/"L"/empty), per the palette roles defined in
// internal/ui/panel. Unrecognized values fall back to the default
// (unstyled) color, same as no priority.
func priorityColor(priority string) lipgloss.Color {
	switch priority {
	case "H":
		return panel.PriorityHighColor
	case "M":
		return panel.PriorityMediumColor
	case "L":
		return panel.PriorityLowColor
	default:
		return panel.PriorityLowDefaultColor
	}
}

// Model is a Bubble Tea model rendering a bordered, selectable list of
// taskwarrior tasks.
type Model struct {
	tasks       []taskwarrior.Task
	cursor      int
	width       int
	height      int
	focused     bool
	status      StatusTab
	searchQuery string
}

// New constructs a Model over the given tasks. The list starts with the
// first task (if any) selected.
func New(tasks []taskwarrior.Task) Model {
	return Model{tasks: sortByUrgency(tasks)}
}

// SetTasks replaces the underlying task slice, clamping the cursor so it
// remains within bounds.
func (m Model) SetTasks(tasks []taskwarrior.Task) Model {
	m.tasks = sortByUrgency(tasks)
	if m.cursor >= len(tasks) {
		m.cursor = len(tasks) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	return m
}

// sortByUrgency returns a copy of tasks ordered by taskwarrior's computed
// urgency field plus each task's manual UrgencyOffset (see
// Task.UrgencyOffset), highest first. Taskwarrior's default urgency
// coefficients already weight priority heavily, so this alone gives an
// urgency-first, priority-as-largest-factor ordering without lazytask
// needing its own weighting logic; UrgencyOffset lets Ctrl+j/Ctrl+k nudge a
// task's position within that ordering. The sort is stable so ties
// (including all-zero urgency, e.g. in tests that don't set it) keep their
// original relative order.
func sortByUrgency(tasks []taskwarrior.Task) []taskwarrior.Task {
	sorted := make([]taskwarrior.Task, len(tasks))
	copy(sorted, tasks)
	sort.SliceStable(sorted, func(i, j int) bool {
		return effectiveUrgency(sorted[i]) > effectiveUrgency(sorted[j])
	})
	return sorted
}

// effectiveUrgency is the sort key used by sortByUrgency: taskwarrior's
// computed Urgency plus the task's manual UrgencyOffset.
func effectiveUrgency(t taskwarrior.Task) float64 {
	return t.Urgency + t.UrgencyOffset
}

// Selected returns the currently selected task and true, or a zero Task and
// false if the list is empty.
func (m Model) Selected() (taskwarrior.Task, bool) {
	if len(m.tasks) == 0 {
		return taskwarrior.Task{}, false
	}
	return m.tasks[m.cursor], true
}

// SelectID moves the cursor to the task whose ID matches id, leaving the
// cursor unchanged if no task matches (e.g. it was already deleted/renumbered
// by the time the caller's refresh completed).
func (m Model) SelectID(id int) Model {
	for i, t := range m.tasks {
		if t.ID == id {
			m.cursor = i
			break
		}
	}
	return m
}

// Neighbor returns the task adjacent to the currently selected one in the
// list's current sorted order, in the direction of delta (-1 for the task
// immediately above, +1 for the task immediately below), and true if such a
// neighbor exists. Used by the Ctrl+j/Ctrl+k manual-reorder keys to find
// which task's effective urgency the selected task should move past.
func (m Model) Neighbor(delta int) (taskwarrior.Task, bool) {
	i := m.cursor + delta
	if i < 0 || i >= len(m.tasks) {
		return taskwarrior.Task{}, false
	}
	return m.tasks[i], true
}

// ClearSearch discards the active search query (if any), so the Tasks
// panel stops highlighting matches and n/N become no-ops again, bound to
// `esc` from the Tasks panel while a search is active. It never moves the
// cursor.
func (m Model) ClearSearch() Model {
	m.searchQuery = ""
	return m
}

// Search sets query as the active search (matched case-insensitively
// against each task's Description only, per requirements.md Phase 1), then
// jumps the cursor to the nearest match at or after the current position,
// wrapping around to the top of the list if needed. It returns the updated
// Model and whether any match was found; the query stays recorded either
// way so NextMatch/PrevMatch keep working (or keep silently no-op'ing) for
// repeat n/N presses, matching lazygit's file-search UX where the search
// stays "active" after a commit. The list itself is never filtered — this
// only ever moves the cursor.
func (m Model) Search(query string) (Model, bool) {
	m.searchQuery = query
	i, ok := m.findMatch(m.cursor, 1, true)
	if ok {
		m.cursor = i
	}
	return m, ok
}

// NextMatch moves the cursor to the next task (forward, wrapping) whose
// Description contains the active search query, bound to "n". It is a
// no-op (returning ok=false) if there is no active query or no task
// matches.
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

// PrevMatch moves the cursor to the previous task (backward, wrapping)
// whose Description contains the active search query, bound to "N". It is
// a no-op (returning ok=false) if there is no active query or no task
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

// findMatch scans m.tasks in direction (+1/-1) starting from start,
// wrapping around the ends of the list, and returns the index of the first
// task whose Description matches m.searchQuery (case-insensitive substring),
// and true. If includeStart is true the start index itself is included in
// the scan (used by Search's "jump to first match" semantics); otherwise
// the scan begins one step past start (used by NextMatch/PrevMatch, so
// repeated presses always advance rather than getting stuck on the current
// match). Returns (0, false) if m.tasks is empty or nothing matches.
func (m Model) findMatch(start, direction int, includeStart bool) (int, bool) {
	n := len(m.tasks)
	if n == 0 || m.searchQuery == "" {
		return 0, false
	}
	i := start
	if !includeStart {
		i += direction
	}
	i = ((i % n) + n) % n
	for step := 0; step < n; step++ {
		if matchesQuery(m.tasks[i].Description, m.searchQuery) {
			return i, true
		}
		i = ((i+direction)%n + n) % n
	}
	return 0, false
}

// matchesQuery reports whether desc contains query as a case-insensitive
// substring.
func matchesQuery(desc, query string) bool {
	return strings.Contains(strings.ToLower(desc), strings.ToLower(query))
}

// SetFocused records whether the Tasks panel currently has focus in the
// surrounding panel grid, so View can apply the shared focused-panel border
// highlight (see internal/ui/panel).
func (m Model) SetFocused(focused bool) Model {
	m.focused = focused
	return m
}

// Status returns the currently selected status tab (Todo/Done/Deleted).
func (m Model) Status() StatusTab {
	return m.status
}

// StatusFilter returns the `task export` filter for the currently selected
// status tab, for callers (main.go) that need to refetch tasks.
func (m Model) StatusFilter() string {
	return m.status.Filter()
}

// NextStatus cycles forward through the Todo/Done/Deleted tabs (bound to
// `]`), resetting the cursor since the underlying task set is about to
// change.
func (m Model) NextStatus() Model {
	return m.setStatus((int(m.status) + 1) % len(statusTabs))
}

// PrevStatus cycles backward through the Todo/Done/Deleted tabs (bound to
// `[`), resetting the cursor since the underlying task set is about to
// change.
func (m Model) PrevStatus() Model {
	return m.setStatus((int(m.status) - 1 + len(statusTabs)) % len(statusTabs))
}

func (m Model) setStatus(i int) Model {
	m.status = statusTabs[i]
	m.cursor = 0
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model, handling up/down (and vim-style j/k)
// navigation and terminal resize events.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.tasks)-1 {
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
	// Account for the left/right border columns (see panel.InnerSize).
	innerWidth := width - 2
	if innerWidth < 20 {
		innerWidth = 20
	}

	_, innerHeight := panel.InnerSize(width, m.height)
	// One row is reserved for the header, so the list body scrolls within
	// whatever remains.
	visibleRows := innerHeight - 1
	if visibleRows < 1 {
		visibleRows = 1
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render(formatRow(innerWidth, "ID", "Description", "Project", "Priority", "Due")))
	b.WriteString("\n")

	if len(m.tasks) == 0 {
		b.WriteString("(no tasks)")
	} else {
		start, end := panel.ScrollWindow(m.cursor, len(m.tasks), visibleRows)
		for i := start; i < end; i++ {
			t := m.tasks[i]
			row := renderDataRow(innerWidth, t, i == m.cursor, m.searchQuery)
			b.WriteString(row)
			if i < end-1 {
				b.WriteString("\n")
			}
		}
	}

	body := b.String()

	tabs := make([]panel.Tab, len(statusTabs))
	for i, t := range statusTabs {
		tabs[i] = panel.Tab{Label: t.Label(), Active: t == m.status}
	}

	footer := ""
	if len(m.tasks) > 0 {
		footer = fmt.Sprintf("%d of %d", m.cursor+1, len(m.tasks))
	}
	return panel.FrameTabs('2', tabs, body, innerWidth, innerHeight, m.focused, footer)
}

// formatRow lays out the fixed-width columns used by both the header and
// data rows so they stay aligned.
func formatRow(width int, id, description, project, priority, due string) string {
	idWidth, descWidth, projectWidth, priorityWidth, dueWidth := columnWidths(width)

	return fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s",
		idWidth, truncate(id, idWidth),
		descWidth, truncate(description, descWidth),
		projectWidth, truncate(project, projectWidth),
		priorityWidth, truncate(priority, priorityWidth),
		dueWidth, truncate(due, dueWidth),
	)
}

// columnWidths returns the fixed column widths shared by formatRow (header)
// and renderDataRow (data rows), so the two stay aligned.
func columnWidths(width int) (idWidth, descWidth, projectWidth, priorityWidth, dueWidth int) {
	const (
		fixedIDWidth       = 4
		fixedProjectWidth  = 12
		fixedPriorityWidth = 4
		fixedDueWidth      = 10
	)
	descWidth = width - fixedIDWidth - fixedProjectWidth - fixedPriorityWidth - fixedDueWidth - 4
	if descWidth < 8 {
		descWidth = 8
	}
	return fixedIDWidth, descWidth, fixedProjectWidth, fixedPriorityWidth, fixedDueWidth
}

// renderDataRow lays out one task's row using the same column widths as
// formatRow, but colors only the Priority cell by the task's priority
// (see priorityColor) rather than the whole row — coloring the entire row
// would compete with a possible future overdue-due-date highlight in the
// Due column. When selected is true, the row-highlight background/bold
// (matching panel.SelectedRowBackground) is applied across every cell and
// the inter-column spacing so the highlight still reads as a full-row bar.
// When query is non-empty (an active `/` search), any substring of the
// Description matching query (case-insensitively) is rendered with
// searchMatchColor on the current cursor row (selected) or
// searchMatchColorUnfocused on every other matching row, so every visible
// match is obvious while the current one still stands out.
func renderDataRow(width int, t taskwarrior.Task, selected bool, query string) string {
	idWidth, descWidth, projectWidth, priorityWidth, dueWidth := columnWidths(width)

	base := lipgloss.NewStyle()
	if selected {
		base = base.Background(panel.SelectedRowBackground).Bold(true)
	}
	priorityStyle := base.Foreground(priorityColor(t.Priority))

	pad := func(s string, w int) string {
		return fmt.Sprintf("%-*s", w, truncate(s, w))
	}

	sep := base.Render(" ")
	return strings.Join([]string{
		base.Render(pad(fmt.Sprintf("%d", t.ID), idWidth)),
		renderDescriptionCell(t.Description, descWidth, query, base, selected),
		base.Render(pad(t.Project, projectWidth)),
		priorityStyle.Render(pad(t.Priority, priorityWidth)),
		base.Render(pad(t.Due, dueWidth)),
	}, sep)
}

// renderDescriptionCell renders the Description column for one row: it
// truncates/pads the text to w exactly like the other cells (via pad in
// renderDataRow), but when query is non-empty it splits the truncated text
// around case-insensitive matches of query and renders the matched
// portions with searchMatchColor/searchMatchColorUnfocused, leaving the
// rest styled as base (so selected-row bold/background still applies to
// the whole cell).
func renderDescriptionCell(desc string, w int, query string, base lipgloss.Style, selected bool) string {
	truncated := truncate(sanitizeSingleLine(desc), w)
	pad := w - len(truncated)
	if pad < 0 {
		pad = 0
	}

	rendered := renderWithMatches(truncated, query, base, selected)
	if pad > 0 {
		rendered += base.Render(strings.Repeat(" ", pad))
	}
	return rendered
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
	matchStyle := base.Background(highlight).Foreground(lipgloss.Color("0")).Bold(false)

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

// sanitizeSingleLine collapses a Description that contains embedded
// newlines (possible today via the $EDITOR multi-line Description flow)
// or other line-breaking control characters into a single line, so a
// tasklist row can never be visually broken across multiple terminal
// lines. Each run of newlines/control characters is replaced with a
// single space; truncation happens separately afterward.
func sanitizeSingleLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || (unicode.IsControl(r) && r != '\t') {
			return ' '
		}
		return r
	}, s)
}

// truncate shortens s to fit within width, adding an ellipsis if it was cut.
func truncate(s string, width int) string {
	if len(s) <= width {
		return s
	}
	if width <= 1 {
		return s[:width]
	}
	return s[:width-1] + "…"
}
