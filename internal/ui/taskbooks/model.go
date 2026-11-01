// Package taskbooks implements the selectable list popup of distinct
// taskbook names, plus special entries for viewing all tasks or the inbox
// (unassigned tasks).
package taskbooks

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tkilb/lazytask/internal/ui/panel"
)

const (
	// AllLabel clears the taskbook filter entirely (no taskbook filter applied).
	AllLabel = "(all)"

	// minPanelWidth is used when no tea.WindowSizeMsg has been received yet.
	minPanelWidth = 20
)

var selectedRowStyle = lipgloss.NewStyle().Background(panel.SelectedRowBackground).Bold(true)

var (
	searchMatchColor          = lipgloss.Color("6")
	searchMatchColorUnfocused = lipgloss.Color("3")
	searchMatchTextColor      = lipgloss.Color("0")
)

// Counts carries the number of tasks behind each selectable entry.
type Counts struct {
	All        int
	Unscoped   int
	ByTaskbook map[string]int
}

// Model is a Bubble Tea model rendering a bordered, selectable list of
// distinct taskbook names, plus the AllLabel special entry.
type Model struct {
	taskbooks   []string
	counts      Counts
	cursor      int
	width       int
	height      int
	focused     bool
	title       string
	searchQuery string
}

const defaultTitle = "Taskbooks"

// New constructs an empty Model.
func New() Model {
	return Model{}
}

// SetTitle overrides the panel title embedded in View's top border.
func (m Model) SetTitle(title string) Model {
	m.title = title
	return m
}

// SetTaskbooks replaces the underlying distinct taskbook names, clamping the
// cursor so it remains within bounds.
func (m Model) SetTaskbooks(names []string) Model {
	m.taskbooks = names
	entries := m.entries()
	if m.cursor >= len(entries) {
		m.cursor = len(entries) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	return m
}

// SetCounts replaces the per-entry task counts shown as an "(N)" suffix.
func (m Model) SetCounts(counts Counts) Model {
	m.counts = counts
	return m
}

// SelectLabel moves the cursor to the given entry if it exists.
func (m Model) SelectLabel(label string) Model {
	for i, entry := range m.entries() {
		if entry == label {
			m.cursor = i
			break
		}
	}
	return m
}

// HasLabel reports whether label is a currently selectable entry.
func (m Model) HasLabel(label string) bool {
	for _, entry := range m.entries() {
		if entry == label {
			return true
		}
	}
	return false
}

// entries returns the full selectable list.
func (m Model) entries() []string {
	entries := make([]string, 0, len(m.taskbooks)+2)
	entries = append(entries, AllLabel)
	entries = append(entries, m.taskbooks...)
	return entries
}

// Taskbooks returns the current distinct taskbook names.
func (m Model) Taskbooks() []string {
	return append([]string(nil), m.taskbooks...)
}

// ClearSearch discards the active search query.
func (m Model) ClearSearch() Model {
	m.searchQuery = ""
	return m
}

// HasActiveSearch reports whether a search query is active.
func (m Model) HasActiveSearch() bool {
	return m.searchQuery != ""
}

// Search sets query as the active search and jumps to the first match.
func (m Model) Search(query string) (Model, bool) {
	m.searchQuery = query
	i, ok := m.findMatch(m.cursor, 1, true)
	if ok {
		m.cursor = i
	}
	return m, ok
}

// NextMatch moves the cursor to the next matching entry.
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

// PrevMatch moves the cursor to the previous matching entry.
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

func matchesQuery(name, query string) bool {
	return strings.Contains(strings.ToLower(name), strings.ToLower(query))
}

// Selected returns the entry currently under the cursor.
func (m Model) Selected() (string, bool) {
	entries := m.entries()
	if len(entries) == 0 {
		return "", false
	}
	return entries[m.cursor], true
}

// RowAt maps a row within the panel to the entry drawn there.
func (m Model) RowAt(row int) (int, bool) {
	_, innerHeight := panel.InnerSize(max(m.width, minPanelWidth), m.height)
	return panel.ListRowAt(row-1, m.cursor, len(m.entries()), max(innerHeight, 1), m.focused)
}

// SelectIndex moves the cursor to entry i.
func (m Model) SelectIndex(i int) Model {
	if i >= 0 && i < len(m.entries()) {
		m.cursor = i
	}
	return m
}

// SetFocused records whether the panel currently has focus.
func (m Model) SetFocused(focused bool) Model {
	m.focused = focused
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
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
	scrollWindow := panel.CenteredScrollWindow
	if m.focused {
		scrollWindow = panel.ScrollWindow
	}
	start, end := scrollWindow(m.cursor, len(entries), visibleRows)

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

const countFieldWidth = 6

func (m Model) displayLine(entry string, width int, query string, selected bool) string {
	base := lipgloss.NewStyle()
	if selected {
		base = selectedRowStyle
	}
	field := countField(m.count(entry))
	if width <= 0 {
		return base.Render(entry + " " + field)
	}
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

func countField(n int) string {
	s := fmt.Sprintf("(%d)", n)
	if pad := countFieldWidth - len([]rune(s)); pad > 0 {
		s = strings.Repeat(" ", pad) + s
	}
	return s
}

func (m Model) count(entry string) int {
	switch entry {
	case AllLabel:
		return m.counts.All
	default:
		return m.counts.ByTaskbook[entry] + m.counts.Unscoped
	}
}

func truncate(s string, width int) string {
	if width <= 0 || len(s) <= width {
		return s
	}
	if width <= 1 {
		return s[:width]
	}
	return s[:width-1] + "…"
}
