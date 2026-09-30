// Package tasklist implements the lazygit-style bordered task list panel.
//
// This chunk renders whatever []taskwarrior.Task it is constructed with —
// it does not know how to fetch tasks itself. Live taskwarrior wiring is
// added in a later chunk (see requirements.md chunk 5).
package tasklist

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/panel"
)

// taskDueLayout is Taskwarrior's combined UTC export/import format for the
// Due field (e.g. "20240115T140000Z"), duplicated here rather than shared
// across packages per this codebase's existing convention (see
// internal/editbuffer's own copy of the same constant).
const taskDueLayout = "20060102T150405Z"

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

	// Due-column colors, per requirements.md Phase 6. Chosen to avoid
	// colliding with priorityColor's red/yellow/cyan or the search-match
	// colors above: overdue is red (distinct from priority-high's red
	// only in that it's never shown on the same cell), due-today is a
	// bright 256-color orange (not in the base 16-color palette used
	// elsewhere, since every base color was already spoken for), due-in-
	// 1-day is yellow, and due-in-2-or-more-days is green.
	dueOverdueColor  = lipgloss.Color("1")
	dueTodayColor    = lipgloss.Color("208")
	dueTomorrowColor = lipgloss.Color("3")
	dueLaterColor    = lipgloss.Color("2")
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
	// now returns the reference instant the Due column's day-delta is
	// computed relative to. Defaults to time.Now; overridable via WithNow
	// so tests get deterministic deltas regardless of wall-clock time.
	now func() time.Time
}

// New constructs a Model over the given tasks. The list starts with the
// first task (if any) selected.
func New(tasks []taskwarrior.Task) Model {
	return Model{tasks: sortByUrgency(tasks), now: time.Now}
}

// WithNow returns a copy of m using now in place of time.Now for resolving
// the Due column's day-delta, for deterministic tests (see
// internal/ui/datepick.Model.WithNow for the same pattern).
func (m Model) WithNow(now func() time.Time) Model {
	m.now = now
	return m
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
	b.WriteString(headerStyle.Render(formatRow(innerWidth, "ID", "Description", "Project", "Pr.", "Due")))
	b.WriteString("\n")

	nowFn := m.now
	if nowFn == nil {
		nowFn = time.Now
	}

	if len(m.tasks) == 0 {
		b.WriteString("(no tasks)")
	} else {
		start, end := panel.ScrollWindow(m.cursor, len(m.tasks), visibleRows)
		for i := start; i < end; i++ {
			t := m.tasks[i]
			row := renderDataRow(innerWidth, t, i == m.cursor, m.searchQuery, nowFn())
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
// data rows so they stay aligned. ID/Description/Project/Priority are
// left-aligned; Due is right-aligned (rightPad) per requirements.md
// Phase 6, so its short delta value sits flush against the right edge of
// its column. Priority is left-aligned (not right, like Due) since its
// column is sized exactly to its content ("H"/"M"/"L"), so right vs. left
// alignment only matters when reading the header ("Pr.") against the
// value below it — left keeps both flush on their shared left edge.
// Project stays left-aligned as well (rather than right-aligned like
// Due) since project names vary enough in length that right-justifying
// them produced a ragged, harder-to-scan left edge — left-aligned reads
// naturally while the Project/Pr./Due block as a whole is still pushed
// to the panel's far right (Description absorbs the flexible space
// before it). A single trailing space (accounted for in columnWidths'
// -5, not -4) keeps Due from touching the panel's right border.
func formatRow(width int, id, description, project, priority, due string) string {
	idWidth, descWidth, projectWidth, priorityWidth, dueWidth := columnWidths(width)

	return fmt.Sprintf("%-*s %-*s  %-*s %-*s %s ",
		idWidth, truncate(id, idWidth),
		descWidth, truncate(description, descWidth),
		projectWidth, truncate(project, projectWidth),
		priorityWidth, truncate(priority, priorityWidth),
		rightPad(due, dueWidth),
	)
}

// columnWidths returns the fixed column widths shared by formatRow (header)
// and renderDataRow (data rows), so the two stay aligned. Priority and Due
// are sized to their actual content (not a generic fixed width) since
// both are short, right-aligned columns and an oversized width just
// leaves a wall of wasted blank space to their left: Priority only ever
// holds "H"/"M"/"L" (1 char) and its header is "Pr." (3 chars), so 3 is
// exact; Due holds a signed day-delta (rarely more than 3-4 digits) with
// a "Due" (3-char) header, so 5 comfortably fits e.g. "-999" with a
// leading space to spare.
func columnWidths(width int) (idWidth, descWidth, projectWidth, priorityWidth, dueWidth int) {
	const (
		fixedIDWidth       = 4
		fixedProjectWidth  = 12
		fixedPriorityWidth = 3
		fixedDueWidth      = 3
	)
	// -6, not -5: four single-space separators between the five columns,
	// plus one extra space between Description and Project (for visual
	// breathing room at that boundary), plus one trailing margin space
	// after Due, so its right-aligned value never touches the panel's
	// right border.
	descWidth = width - fixedIDWidth - fixedProjectWidth - fixedPriorityWidth - fixedDueWidth - 6
	if descWidth < 8 {
		descWidth = 8
	}
	return fixedIDWidth, descWidth, fixedProjectWidth, fixedPriorityWidth, fixedDueWidth
}

// rightPad truncates/right-aligns s within width w (e.g. "-10" in a
// 10-wide column becomes "       -10"), the mirror image of the
// left-aligned pad used for ID/Description/Project.
func rightPad(s string, w int) string {
	return fmt.Sprintf("%*s", w, truncate(s, w))
}

// renderDataRow lays out one task's row using the same column widths as
// formatRow, but colors only the Priority cell by the task's priority
// (see priorityColor) rather than the whole row — coloring the entire row
// would compete with a possible future overdue-due-date highlight in the
// Due column. When selected is true, the row-highlight background/bold
// (matching panel.SelectedRowBackground) is applied across every cell and
// the inter-column spacing (including the trailing margin space after
// Due) so the highlight still reads as a full-row bar. When query is
// non-empty (an active `/` search), any substring of the Description
// matching query (case-insensitively) is rendered with searchMatchColor
// on the current cursor row (selected) or searchMatchColorUnfocused on
// every other matching row, so every visible match is obvious while the
// current one still stands out. The Due cell itself is colored by
// dueColor/formatDueCell (see those for the delta/color rules), computed
// relative to now. Due is right-aligned (rightPad); Priority and Project
// stay left-aligned (see formatRow for why), matching the header.
func renderDataRow(width int, t taskwarrior.Task, selected bool, query string, now time.Time) string {
	idWidth, descWidth, projectWidth, priorityWidth, dueWidth := columnWidths(width)

	base := lipgloss.NewStyle()
	if selected {
		base = base.Background(panel.SelectedRowBackground).Bold(true)
	}
	priorityStyle := base.Foreground(priorityColor(t.Priority))

	pad := func(s string, w int) string {
		return fmt.Sprintf("%-*s", w, truncate(s, w))
	}

	dueText, dueHasDelta := formatDueCell(t.Due, now)
	dueStyle := base
	if dueHasDelta {
		dueStyle = base.Foreground(dueColor(dueText))
	}

	sep := base.Render(" ")
	return strings.Join([]string{
		base.Render(pad(fmt.Sprintf("%d", t.ID), idWidth)),
		renderDescriptionCell(t.Description, descWidth, query, base, selected),
		sep + base.Render(pad(t.Project, projectWidth)),
		priorityStyle.Render(pad(t.Priority, priorityWidth)),
		dueStyle.Render(rightPad(dueText, dueWidth)),
	}, sep) + sep
}

// dueDeltaDays parses a task's raw Due value (taskDueLayout, e.g.
// "20240115T140000Z") and returns the whole number of calendar days
// between now's local date and the due date's local date (negative if
// overdue, 0 if due today), and true. An empty due (no due date) or a
// value that doesn't parse as taskDueLayout returns (0, false).
func dueDeltaDays(due string, now time.Time) (int, bool) {
	if due == "" {
		return 0, false
	}
	t, err := time.Parse(taskDueLayout, due)
	if err != nil {
		return 0, false
	}
	// Both sides must be converted to the same (local) zone before their
	// date components are compared: due is a UTC instant that may land on
	// a different local calendar date than its UTC date, and now (e.g. as
	// injected via WithNow in tests) may not already be in Local.
	dueLocal := t.Local()
	nowLocal := now.Local()
	dueDate := time.Date(dueLocal.Year(), dueLocal.Month(), dueLocal.Day(), 0, 0, 0, 0, dueLocal.Location())
	nowDate := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, nowLocal.Location())
	days := int(dueDate.Sub(nowDate).Hours() / 24)
	return days, true
}

// formatDueCell renders a task's Due column: a bare signed day-delta (e.g.
// "2", "0" for due today, "-1" for overdue by a day), per requirements.md
// Phase 6. A task with no due date renders as an empty string. The second
// return value mirrors dueDeltaDays' ok, so callers know whether to apply
// dueColor.
func formatDueCell(due string, now time.Time) (string, bool) {
	days, ok := dueDeltaDays(due, now)
	if !ok {
		return "", false
	}
	return strconv.Itoa(days), true
}

// dueColor returns the Due cell's foreground color for an already-formatted
// day-delta string (as produced by formatDueCell): red if overdue
// (negative), dueTodayColor for "0", yellow for due tomorrow ("1"), and
// green for two or more days out.
func dueColor(deltaText string) lipgloss.Color {
	days, err := strconv.Atoi(deltaText)
	if err != nil {
		return dueLaterColor
	}
	switch {
	case days < 0:
		return dueOverdueColor
	case days == 0:
		return dueTodayColor
	case days == 1:
		return dueTomorrowColor
	default:
		return dueLaterColor
	}
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
	// utf8.RuneCountInString, not len(truncated): len() is byte length,
	// which overcounts any multi-byte UTF-8 rune (e.g. a curly quote is 3
	// bytes but 1 display rune) and under-pads the cell by the
	// difference, visibly shifting every column after Description to the
	// left on any row whose Description contains such a character.
	pad := w - utf8.RuneCountInString(truncated)
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

// truncate shortens s to fit within width **runes** (not bytes), adding an
// ellipsis if it was cut. Operating on runes (rather than len(s)/byte
// slicing) is required for any Description containing multi-byte UTF-8
// characters (curly quotes, accents, emoji, etc., all common in real
// task descriptions): byte-based slicing can both split a multi-byte
// sequence mid-rune (corrupting the string) and miscount how many
// characters are actually present, which in turn threw off later
// columns' alignment (see renderDescriptionCell's pad calculation, fixed
// alongside this for the same reason).
func truncate(s string, width int) string {
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width <= 1 {
		return string(r[:width])
	}
	return string(r[:width-1]) + "…"
}
