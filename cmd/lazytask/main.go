package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tkilb/lazytask/internal/config"
	"github.com/tkilb/lazytask/internal/editbuffer"
	"github.com/tkilb/lazytask/internal/editor"
	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/addform"
	"github.com/tkilb/lazytask/internal/ui/panel"
	"github.com/tkilb/lazytask/internal/ui/popup"
	"github.com/tkilb/lazytask/internal/ui/projects"
	"github.com/tkilb/lazytask/internal/ui/statusbar"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
)

// panelFocus identifies which panel in the grid currently has keyboard
// focus. focusTasks is the zero value so a model constructed without going
// through initialModel (as most tests do) keeps the pre-grid behavior of
// routing navigation keys straight to the Tasks list.
type panelFocus int

const (
	focusTasks panelFocus = iota
	focusStatus
	focusProjects
	focusTags
	focusDetails
)

// panelKeyBindings maps the number keys shown in the panel titles to the
// panel they focus.
var panelKeyBindings = map[string]panelFocus{
	"0": focusDetails,
	"1": focusStatus,
	"2": focusTasks,
	"3": focusProjects,
	"4": focusTags,
}

// focusCycle is the Tab/Shift+Tab traversal order: top-to-bottom through
// the left column, then the right column's single large panel.
var focusCycle = []panelFocus{focusStatus, focusTasks, focusProjects, focusTags, focusDetails}

// panelTitle returns the display title for a given panel, including its
// number-key hint (lazygit-style).
func panelTitle(f panelFocus) string {
	switch f {
	case focusStatus:
		return "1 Status"
	case focusProjects:
		return "3 Projects"
	case focusTags:
		return "4 Tags"
	case focusDetails:
		return "0 Details"
	default:
		return ""
	}
}

func nextFocus(f panelFocus) panelFocus {
	for i, c := range focusCycle {
		if c == f {
			return focusCycle[(i+1)%len(focusCycle)]
		}
	}
	return focusCycle[0]
}

func prevFocus(f panelFocus) panelFocus {
	for i, c := range focusCycle {
		if c == f {
			return focusCycle[(i-1+len(focusCycle))%len(focusCycle)]
		}
	}
	return focusCycle[0]
}

const (
	// defaultGridWidth/defaultGridHeight size the panel grid before the
	// first tea.WindowSizeMsg arrives (e.g. in tests that call View()
	// directly without going through a real terminal).
	defaultGridWidth  = 100
	defaultGridHeight = 30

	// gridBottomOverhead is the number of terminal rows View() renders
	// below the grid (a blank line plus the status bar's single line of
	// key hints). It must be subtracted from the real terminal height
	// before sizing the grid, otherwise the grid plus this trailing
	// content overflows the terminal and the top of the grid (the Status
	// and Details panels' top borders, both on the grid's first row)
	// scrolls off-screen.
	gridBottomOverhead = 2

	// statusPanelHeight is the Status panel's fixed outer height: one
	// content line (the selected task's id/status/project) plus the two
	// border rows. Unlike Tasks and the Projects/Tags row, it doesn't grow
	// with the terminal.
	statusPanelHeight = 3
)

// globalBindings are active no matter which panel currently has focus,
// mirroring lazygit's global-vs-local keybinding split: they only ever
// affect panel focus/navigation and app lifecycle, never panel-specific
// data (tasks, projects, tags, ...).
var globalBindings = []statusbar.Binding{
	{Key: "0-4/tab", Label: "panels"},
	{Key: "a", Label: "add"},
	{Key: "q", Label: "quit"},
}

// tasksLocalBindings are only active while the Tasks panel has focus. They
// are appended after globalBindings in the status bar so the hint line
// reflects exactly which keys will do something right now. "r" and the
// Deleted-tab-specific "x" (purge) label are only meaningful on certain
// tabs, so they're added/overridden separately by statusBindings rather
// than listed here. "a" (add) moved to globalBindings since it's no longer
// Tasks-panel-local.
var tasksLocalBindings = []statusbar.Binding{
	{Key: "↑/k", Label: "up"},
	{Key: "↓/j", Label: "down"},
	{Key: "[/]", Label: "tabs"},
	{Key: "d", Label: "done"},
	{Key: "x", Label: "delete"},
	{Key: "e", Label: "edit"},
}

// projectsLocalBindings are only active while the Projects panel has
// focus: navigating the list automatically updates the shared project
// filter, and "R" opens the rename-project prompt on the entry under the
// cursor (only meaningful on a real project, not the (all)/(none) special
// entries, but shown unconditionally here for simplicity, matching how
// other bindings lists don't special-case every possible selection).
var projectsLocalBindings = []statusbar.Binding{
	{Key: "↑/k", Label: "up"},
	{Key: "↓/j", Label: "down"},
	{Key: "R", Label: "rename"},
}

// statusBindings returns the keybinding hints to show in the status bar for
// the model's current focus: global bindings always apply, and Tasks-panel
// bindings are appended only while that panel is focused (local bindings
// per other panels are added here as those panels grow their own actions).
// "d" is hidden on the Done tab since a task there is already done. "r" is
// shown only on the Done/Deleted tabs (labeled "reopen" on Done, "restore"
// on Deleted), and "x" is relabeled "purge" on the Deleted tab since it
// becomes an irreversible permanent delete there, matching the actual key
// handling in Update.
func (m model) statusBindings() []statusbar.Binding {
	bindings := make([]statusbar.Binding, 0, len(globalBindings)+len(tasksLocalBindings)+1)
	bindings = append(bindings, globalBindings...)
	if m.focus == focusTasks {
		for _, b := range tasksLocalBindings {
			if b.Key == "d" && m.list.Status() == tasklist.TabDone {
				continue
			}
			if b.Key == "x" && m.list.Status() == tasklist.TabDeleted {
				b.Label = "purge"
			}
			bindings = append(bindings, b)
		}
		switch m.list.Status() {
		case tasklist.TabDone:
			bindings = append(bindings, statusbar.Binding{Key: "r", Label: "reopen"})
		case tasklist.TabDeleted:
			bindings = append(bindings, statusbar.Binding{Key: "r", Label: "restore"})
		}
	}
	if m.focus == focusProjects {
		bindings = append(bindings, projectsLocalBindings...)
	}
	return bindings
}

// TaskReader is the subset of the taskwarrior client this model depends on,
// so it can be stubbed out in tests without shelling out to the real `task`
// binary.
type TaskReader interface {
	Export(ctx context.Context, filters ...string) ([]taskwarrior.Task, error)
}

// TaskAdder is the subset of the taskwarrior client needed to create new
// tasks, so it can be stubbed out in tests without shelling out to the
// real `task` binary.
type TaskAdder interface {
	Add(ctx context.Context, description string, extraArgs ...string) (int, error)
}

// TaskDoner is the subset of the taskwarrior client needed to mark a task
// done, so it can be stubbed out in tests without shelling out to the real
// `task` binary.
type TaskDoner interface {
	Done(ctx context.Context, id string) error
}

// TaskDeleter is the subset of the taskwarrior client needed to delete a
// task, so it can be stubbed out in tests without shelling out to the real
// `task` binary.
type TaskDeleter interface {
	Delete(ctx context.Context, id string) error
}

// TaskRestorer is the subset of the taskwarrior client needed to restore a
// Done or Deleted task back to pending, so it can be stubbed out in tests
// without shelling out to the real `task` binary.
type TaskRestorer interface {
	Restore(ctx context.Context, id string) error
}

// TaskPurger is the subset of the taskwarrior client needed to permanently
// remove an already-deleted task, so it can be stubbed out in tests without
// shelling out to the real `task` binary.
type TaskPurger interface {
	Purge(ctx context.Context, id string) error
}

// TaskImporter is the subset of the taskwarrior client needed to re-import
// an edited task, so it can be stubbed out in tests without shelling out to
// the real `task` binary.
type TaskImporter interface {
	Import(ctx context.Context, data []byte) error
}

// tasksLoadedMsg carries the result of a successful task fetch.
type tasksLoadedMsg struct {
	tasks []taskwarrior.Task
}

// tasksErrMsg carries the error from a failed task fetch.
type tasksErrMsg struct {
	err error
}

// projectsLoadedMsg carries the distinct, sorted project names for the
// Projects panel, sourced from an unfiltered task query (see
// fetchProjects) rather than whatever filter the Tasks panel currently
// has applied.
type projectsLoadedMsg struct {
	projects []string
	counts   projects.Counts
}

// projectsErrMsg carries the error from a failed projects fetch.
type projectsErrMsg struct {
	err error
}

// taskAddedMsg carries the result of a successful Add call, including the
// numeric ID Taskwarrior assigned so the list can focus it once the
// following refresh completes.
type taskAddedMsg struct {
	id int
}

// taskAddErrMsg carries the error from a failed Add call.
type taskAddErrMsg struct {
	err error
}

// taskDoneMsg carries the result of a successful Done call.
type taskDoneMsg struct{}

// taskDoneErrMsg carries the error from a failed Done call.
type taskDoneErrMsg struct {
	err error
}

// taskDeletedMsg carries the result of a successful Delete call.
type taskDeletedMsg struct{}

// taskDeleteErrMsg carries the error from a failed Delete call.
type taskDeleteErrMsg struct {
	err error
}

// taskRestoredMsg carries the result of a successful Restore call.
type taskRestoredMsg struct{}

// taskRestoreErrMsg carries the error from a failed Restore call.
type taskRestoreErrMsg struct {
	err error
}

// taskPurgedMsg carries the result of a successful Purge call.
type taskPurgedMsg struct{}

// taskPurgeErrMsg carries the error from a failed Purge call.
type taskPurgeErrMsg struct {
	err error
}

// taskEditedMsg carries the result of a successful edit-and-reimport.
type taskEditedMsg struct{}

// taskEditErrMsg carries the error from a failed edit-and-reimport, whether
// from the editor invocation itself, parsing its output, or re-importing.
type taskEditErrMsg struct {
	err error
}

// projectRenamedMsg carries the result of a successful project rename,
// including the new name so the active project filter (if it pointed at
// the old name) can be updated to follow it.
type projectRenamedMsg struct {
	to string
}

// projectRenameErrMsg carries the error from a failed project rename.
type projectRenameErrMsg struct {
	err error
}

// filterSaveErrMsg carries the error from a failed attempt to persist the
// project filter to disk (see internal/config). It never blocks or
// reverts the in-memory filter change — only the on-disk copy failed to
// update.
type filterSaveErrMsg struct {
	err error
}

type model struct {
	reader         TaskReader
	adder          TaskAdder
	doner          TaskDoner
	deleter        TaskDeleter
	restorer       TaskRestorer
	purger         TaskPurger
	importer       TaskImporter
	list           tasklist.Model
	add            addform.Model
	adding         bool
	deleting       bool
	purging        bool
	completing     bool
	restoring      bool
	renaming       bool
	renameInput    addform.Model
	renameFrom     string
	renameTo       string
	renameMerging  bool
	renameConfirm  bool
	popups         popup.Model
	quitting       bool
	pendingFocusID int
	focus          panelFocus
	width          int
	height         int
	projects       projects.Model
	filter         filterState
	knownProjects  map[string]struct{}
}

// initialModel constructs the app's starting state, including restoring
// the project filter persisted by a previous session (see internal/config)
// so the app reopens filtered the way the user left it.
func initialModel() model {
	client := taskwarrior.NewClient()
	persisted := config.Load()
	return model{
		reader:        client,
		adder:         client,
		doner:         client,
		deleter:       client,
		restorer:      client,
		purger:        client,
		importer:      client,
		list:          tasklist.New(nil).SetFocused(true),
		add:           addform.New(),
		projects:      projects.New(),
		knownProjects: make(map[string]struct{}),
		filter:        filterState{project: persisted.Project},
	}
}

// setFocus updates which panel has focus, keeping the Tasks list's and
// Projects panel's own focused flags (used for their border highlight) in
// sync.
func (m model) setFocus(f panelFocus) model {
	m.focus = f
	m.list = m.list.SetFocused(f == focusTasks)
	m.projects = m.projects.SetFocused(f == focusProjects)
	return m
}

// gridDims computes the panel grid's column widths and left-column row
// heights from the model's last known terminal size, falling back to sane
// defaults if no tea.WindowSizeMsg has been received yet (e.g. in tests
// that call View directly). The left column's 3 rows use static
// proportions rather than an even split: Status is a fixed single-line
// panel, and of the remaining height Tasks gets about two-thirds (half,
// plus a third of what would otherwise go to the bottom row) since it's
// the most important panel, with the bottom Projects/Tags row getting the
// rest.
func (m model) gridDims() (leftWidth, rightWidth, statusHeight, tasksHeight, bottomHeight, fullHeight int) {
	width := m.width
	if width <= 0 {
		width = defaultGridWidth
	}
	height := m.height
	if height <= 0 {
		height = defaultGridHeight
	} else if height > gridBottomOverhead {
		height -= gridBottomOverhead
	}
	leftWidth = width / 2
	rightWidth = width - leftWidth
	fullHeight = height

	statusHeight = statusPanelHeight
	tasksHeight = height / 2
	bottomHeight = height - statusHeight - tasksHeight
	if bottomHeight < 0 {
		bottomHeight = 0
	}

	// Shift some of the Projects/Tags row's height over to Tasks, since
	// Tasks is the more important panel and benefits more from the extra
	// vertical space, while still leaving Projects/Tags a usable amount.
	shift := bottomHeight / 3
	tasksHeight += shift
	bottomHeight -= shift

	return leftWidth, rightWidth, statusHeight, tasksHeight, bottomHeight, fullHeight
}

// subColumnWidths splits a row's total width into two side-by-side
// subcolumns (used for the Projects/Tags row at the bottom of the left
// column), the second absorbing any remainder so the pair always sums to
// leftWidth.
func subColumnWidths(leftWidth int) (firstWidth, secondWidth int) {
	firstWidth = leftWidth / 2
	secondWidth = leftWidth - firstWidth
	return firstWidth, secondWidth
}

// fetchTasks returns a tea.Cmd that loads tasks matching filters (the
// currently selected status tab's filter, plus any active project filter)
// via reader.
func fetchTasks(reader TaskReader, filters ...string) tea.Cmd {
	return func() tea.Msg {
		tasks, err := reader.Export(context.Background(), filters...)
		if err != nil {
			return tasksErrMsg{err: err}
		}
		return tasksLoadedMsg{tasks: tasks}
	}
}

// taskFilters returns the full set of `task export` filter fragments for
// the model's current state: the active status tab, plus any project
// filter selected in the Projects panel.
func (m model) taskFilters() []string {
	filters := []string{m.list.StatusFilter()}
	if pf := m.filter.taskFilter(); pf != "" {
		filters = append(filters, pf)
	}
	return filters
}

// fetchProjects returns a tea.Cmd that loads the distinct project names
// across pending tasks only. This deliberately ignores whatever status tab
// or project filter the Tasks panel currently has applied: otherwise
// applying a project filter would shrink the very list used to change/clear
// that filter. Projects that have gone to zero pending tasks (all done or
// deleted) are handled by the caller merging this result into the
// session's previously-seen project set, rather than by including
// completed/deleted tasks here — that way a project a user never actually
// worked with this session (all tasks already done before launch) doesn't
// reappear just because it's still in taskwarrior's history.
func fetchProjects(reader TaskReader) tea.Cmd {
	return func() tea.Msg {
		tasks, err := reader.Export(context.Background(), "status:pending")
		if err != nil {
			return projectsErrMsg{err: err}
		}
		return projectsLoadedMsg{projects: distinctProjects(tasks), counts: projectCounts(tasks)}
	}
}

// saveFilter returns a tea.Cmd that persists f's project filter to disk
// (see internal/config) so it's restored on the next app launch. On
// failure it reports a filterSaveErrMsg rather than blocking or reverting
// the (already-applied) in-memory filter change.
func saveFilter(f filterState) tea.Cmd {
	return func() tea.Msg {
		if err := config.Save(config.State{Project: f.project}); err != nil {
			return filterSaveErrMsg{err: err}
		}
		return nil
	}
}

// projectCounts tallies pending ("todo") task counts per project, plus
// totals for the AllLabel (every pending task) and NoneLabel (no project
// set) special entries, so the Projects panel can render a "(N)" field next
// to each entry.
func projectCounts(tasks []taskwarrior.Task) projects.Counts {
	c := projects.Counts{ByProject: make(map[string]int)}
	for _, t := range tasks {
		if t.Status != "pending" {
			continue
		}
		c.All++
		if t.Project == "" {
			c.None++
			continue
		}
		c.ByProject[t.Project]++
	}
	return c
}

// mergeKnownProjects folds newly-seen project names into the session's
// running set of known projects and returns the sorted union. This makes a
// project "sticky" for the remainder of the session once it's been seen
// with at least one pending task: deleting or completing its last pending
// task (which would otherwise make it vanish from an unfiltered
// status:pending query) still leaves the project visible, now showing
// "(0)", until the app restarts. On restart the set starts empty again, so
// a project with no pending tasks left simply won't reappear.
func (m *model) mergeKnownProjects(names []string) []string {
	if m.knownProjects == nil {
		m.knownProjects = make(map[string]struct{})
	}
	for _, n := range names {
		m.knownProjects[n] = struct{}{}
	}
	merged := make([]string, 0, len(m.knownProjects))
	for n := range m.knownProjects {
		merged = append(merged, n)
	}
	sort.Strings(merged)
	return merged
}

// distinctProjects returns the sorted set of distinct, non-empty project
// names found across tasks.
func distinctProjects(tasks []taskwarrior.Task) []string {
	seen := make(map[string]struct{})
	names := make([]string, 0)
	for _, t := range tasks {
		if t.Project == "" {
			continue
		}
		if _, ok := seen[t.Project]; ok {
			continue
		}
		seen[t.Project] = struct{}{}
		names = append(names, t.Project)
	}
	sort.Strings(names)
	return names
}

// addTask returns a tea.Cmd that creates a new task via adder, passing
// along any extra `task add` argument fragments (e.g. "project:chores").
func addTask(adder TaskAdder, description string, extraArgs ...string) tea.Cmd {
	return func() tea.Msg {
		id, err := adder.Add(context.Background(), description, extraArgs...)
		if err != nil {
			return taskAddErrMsg{err: err}
		}
		return taskAddedMsg{id: id}
	}
}

// doneTask returns a tea.Cmd that marks the given task id as done via doner.
func doneTask(doner TaskDoner, id string) tea.Cmd {
	return func() tea.Msg {
		if err := doner.Done(context.Background(), id); err != nil {
			return taskDoneErrMsg{err: err}
		}
		return taskDoneMsg{}
	}
}

// restoreThenDoneTask returns a tea.Cmd that restores the given task id
// back to pending and then immediately marks it done. This is used to move
// a Deleted task straight to Done, since Taskwarrior's `done` command
// refuses to act on a task that isn't pending/waiting.
func restoreThenDoneTask(restorer TaskRestorer, doner TaskDoner, id string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := restorer.Restore(ctx, id); err != nil {
			return taskDoneErrMsg{err: err}
		}
		if err := doner.Done(ctx, id); err != nil {
			return taskDoneErrMsg{err: err}
		}
		return taskDoneMsg{}
	}
}

// deleteTask returns a tea.Cmd that deletes the given task id via deleter.
func deleteTask(deleter TaskDeleter, id string) tea.Cmd {
	return func() tea.Msg {
		if err := deleter.Delete(context.Background(), id); err != nil {
			return taskDeleteErrMsg{err: err}
		}
		return taskDeletedMsg{}
	}
}

// restoreTask returns a tea.Cmd that restores the given task id back to
// pending status via restorer.
func restoreTask(restorer TaskRestorer, id string) tea.Cmd {
	return func() tea.Msg {
		if err := restorer.Restore(context.Background(), id); err != nil {
			return taskRestoreErrMsg{err: err}
		}
		return taskRestoredMsg{}
	}
}

// purgeTask returns a tea.Cmd that permanently removes the given task id
// via purger. Unlike deleteTask, this is irreversible.
func purgeTask(purger TaskPurger, id string) tea.Cmd {
	return func() tea.Msg {
		if err := purger.Purge(context.Background(), id); err != nil {
			return taskPurgeErrMsg{err: err}
		}
		return taskPurgedMsg{}
	}
}

// editTask opens task in the user's $EDITOR as a structured plain-text
// buffer (see internal/editbuffer), similar in spirit to `task <id> edit`,
// then re-imports the edited fields via importer once the editor exits. It
// suspends the Bubble Tea program for the duration of the editor
// invocation via tea.ExecProcess.
func editTask(importer TaskImporter, task taskwarrior.Task) tea.Cmd {
	cmd, session, err := editor.Prepare(editbuffer.Serialize(task))
	if err != nil {
		return func() tea.Msg { return taskEditErrMsg{err: err} }
	}

	return tea.ExecProcess(cmd, editTaskCallback(importer, session, task))
}

// editTaskCallback builds the tea.ExecCallback run once the editor process
// launched by editTask exits: it reads back session's temp file, parses
// its editable fields, applies them onto original (preserving read-only
// fields such as UUID untouched, regardless of what the user may have
// typed in that section of the buffer), and re-imports the result via
// importer. Split out from editTask so it can be unit-tested without going
// through tea.ExecProcess/a real editor process.
func editTaskCallback(importer TaskImporter, session *editor.Session, original taskwarrior.Task) func(error) tea.Msg {
	return func(err error) tea.Msg {
		defer session.Close()
		if err != nil {
			return taskEditErrMsg{err: fmt.Errorf("running editor: %w", err)}
		}

		edited, err := session.Read()
		if err != nil {
			return taskEditErrMsg{err: err}
		}

		fields, err := editbuffer.Parse(edited)
		if err != nil {
			return taskEditErrMsg{err: fmt.Errorf("parsing edited task: %w", err)}
		}
		if strings.TrimSpace(original.UUID) == "" {
			return taskEditErrMsg{err: fmt.Errorf("edited task is missing its uuid; not importing")}
		}
		updated := editbuffer.Apply(original, fields)

		data, err := json.Marshal(updated)
		if err != nil {
			return taskEditErrMsg{err: err}
		}
		if err := importer.Import(context.Background(), data); err != nil {
			return taskEditErrMsg{err: err}
		}
		return taskEditedMsg{}
	}
}

// renameProject returns a tea.Cmd that renames a project across every task
// that belongs to it, regardless of status (pending, completed, or
// deleted): it fetches all three statuses via reader, filters to those
// whose Project field exactly matches from, rewrites that field to to on
// each, and re-imports the batch via importer (the same Export-then-Import
// round trip editTask already uses, rather than a dedicated taskwarrior CLI
// mutation, so exact-match filtering is done in Go instead of relying on
// Taskwarrior's project-hierarchy filter semantics).
func renameProject(reader TaskReader, importer TaskImporter, from, to string) tea.Cmd {
	return func() tea.Msg {
		tasks, err := reader.Export(context.Background(), "status:pending", "or", "status:completed", "or", "status:deleted")
		if err != nil {
			return projectRenameErrMsg{err: err}
		}

		var matched []taskwarrior.Task
		for _, t := range tasks {
			if t.Project != from {
				continue
			}
			t.Project = to
			matched = append(matched, t)
		}
		if len(matched) == 0 {
			return projectRenamedMsg{to: to}
		}

		data, err := json.Marshal(matched)
		if err != nil {
			return projectRenameErrMsg{err: err}
		}
		if err := importer.Import(context.Background(), data); err != nil {
			return projectRenameErrMsg{err: err}
		}
		return projectRenamedMsg{to: to}
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// A pending popup blocks all other input: any keypress dismisses it
	// rather than reaching the add/delete/panel handling below. Non-key
	// messages (e.g. window resizes, or async command results that may
	// themselves enqueue further popups) still flow through normally.
	if m.popups.Active() {
		if _, ok := msg.(tea.KeyMsg); ok {
			m.popups = m.popups.Dismiss()
			return m, nil
		}
	}
	if m.adding {
		return m.updateAdding(msg)
	}
	if m.deleting {
		return m.updateDeleting(msg)
	}
	if m.purging {
		return m.updatePurging(msg)
	}
	if m.completing {
		return m.updateCompleting(msg)
	}
	if m.restoring {
		return m.updateRestoring(msg)
	}
	if m.renaming {
		return m.updateRenaming(msg)
	}
	if m.renameConfirm {
		return m.updateRenameConfirm(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		leftWidth, _, _, tasksHeight, bottomHeight, _ := m.gridDims()
		projectsWidth, _ := subColumnWidths(leftWidth)
		var listCmd, projectsCmd tea.Cmd
		m.list, listCmd = m.list.Update(tea.WindowSizeMsg{Width: leftWidth, Height: tasksHeight})
		m.projects, projectsCmd = m.projects.Update(tea.WindowSizeMsg{Width: projectsWidth, Height: bottomHeight})
		return m, tea.Batch(listCmd, projectsCmd)
	case tea.KeyMsg:
		// Global bindings apply no matter which panel has focus: they only
		// ever touch app lifecycle or panel focus itself, never
		// panel-specific data.
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "0", "1", "2", "3", "4":
			m = m.setFocus(panelKeyBindings[msg.String()])
			return m, nil
		case "tab":
			m = m.setFocus(nextFocus(m.focus))
			return m, nil
		case "shift+tab":
			m = m.setFocus(prevFocus(m.focus))
			return m, nil
		case "a":
			m.adding = true
			m.add = m.add.Focus()
			return m, m.add.Init()
		}
		// Projects-panel-local: navigation (up/down/j/k) is applied
		// directly here, rather than being delegated to the bottom
		// routing block, because moving the cursor also needs to update
		// the shared filter and refetch Tasks immediately (no "enter"
		// required).
		if m.focus == focusProjects {
			if msg.String() == "R" {
				if label, ok := m.projects.Selected(); ok && label != projects.AllLabel && label != projects.NoneLabel {
					m.renaming = true
					m.renameFrom = label
					m.renameInput = addform.NewNamed("Rename Project", "Press <enter> to rename, <esc> to cancel", "New project name...").Focus().SetValue(label)
					return m, m.renameInput.Init()
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.projects, cmd = m.projects.Update(msg)
			if label, ok := m.projects.Selected(); ok {
				newFilter := m.filter.withProjectSelection(label)
				if !newFilter.equal(m.filter) {
					m.filter = newFilter
					return m, tea.Batch(cmd, fetchTasks(m.reader, m.taskFilters()...), saveFilter(m.filter))
				}
			}
			return m, cmd
		}
		// Everything else is local to the Tasks panel and only fires while
		// it has focus (lazygit-style global/local keybinding split).
		if m.focus != focusTasks {
			return m, nil
		}
		switch msg.String() {
		case "[":
			m.list = m.list.PrevStatus()
			return m, fetchTasks(m.reader, m.taskFilters()...)
		case "]":
			m.list = m.list.NextStatus()
			return m, fetchTasks(m.reader, m.taskFilters()...)
		case "d":
			// A Done task is already done; there's nothing to confirm.
			if m.list.Status() == tasklist.TabDone {
				return m, nil
			}
			if _, ok := m.list.Selected(); ok {
				m.completing = true
			}
			return m, nil
		case "r":
			// Only Done/Deleted tasks have anything to restore; Todo tasks
			// are already pending.
			if m.list.Status() == tasklist.TabTodo {
				return m, nil
			}
			if _, ok := m.list.Selected(); ok {
				m.restoring = true
			}
			return m, nil
		case "x":
			if _, ok := m.list.Selected(); ok {
				if m.list.Status() == tasklist.TabDeleted {
					m.purging = true
				} else {
					m.deleting = true
				}
			}
			return m, nil
		case "e":
			if task, ok := m.list.Selected(); ok {
				return m, editTask(m.importer, task)
			}
			return m, nil
		}
	case tasksLoadedMsg:
		m.list = m.list.SetTasks(msg.tasks)
		if m.pendingFocusID != 0 {
			m.list = m.list.SelectID(m.pendingFocusID)
			m.pendingFocusID = 0
		}
		return m, nil
	case tasksErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case projectsLoadedMsg:
		m.projects = m.projects.SetProjects(m.mergeKnownProjects(msg.projects)).SetCounts(msg.counts).SelectLabel(m.filter.projectLabel())
		return m, nil
	case projectsErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case taskAddedMsg:
		m.add = m.add.Reset()
		m.pendingFocusID = msg.id
		return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader))
	case taskAddErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case taskDoneMsg:
		return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader))
	case taskDoneErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case taskDeletedMsg:
		return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader))
	case taskDeleteErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case taskRestoredMsg:
		return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader))
	case taskRestoreErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case taskPurgedMsg:
		return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader))
	case taskPurgeErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case taskEditedMsg:
		return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader))
	case taskEditErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case projectRenamedMsg:
		newFilter := m.filter
		if m.filter.project != nil && *m.filter.project == m.renameFrom {
			newFilter = m.filter.withProjectSelection(msg.to)
		}
		m.filter = newFilter
		m.renameFrom = ""
		m.renameTo = ""
		return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader), saveFilter(m.filter))
	case projectRenameErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case filterSaveErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	}

	// Remaining messages (e.g. up/down/j/k navigation) only apply to the
	// Tasks or Projects panel, and only reach each while it has focus.
	if m.focus == focusTasks {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	if m.focus == focusProjects {
		var cmd tea.Cmd
		m.projects, cmd = m.projects.Update(msg)
		return m, cmd
	}
	return m, nil
}

// taskID returns the identifier used to address t in taskwarrior mutation
// commands, preferring the stable UUID over the pending numeric ID (which
// can be renumbered by taskwarrior after other tasks complete/are deleted).
func taskID(t taskwarrior.Task) string {
	if t.UUID != "" {
		return t.UUID
	}
	return fmt.Sprintf("%d", t.ID)
}

// errPopup builds an error-severity popup.Message from err.
func errPopup(err error) popup.Message {
	return popup.Message{Severity: popup.Error, Text: err.Error()}
}

// updateDeleting handles messages while a delete confirmation is pending.
func (m model) updateDeleting(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "enter":
			m.deleting = false
			task, ok := m.list.Selected()
			if !ok {
				return m, nil
			}
			return m, deleteTask(m.deleter, taskID(task))
		case "n", "esc":
			m.deleting = false
			return m, nil
		}
	}
	return m, nil
}

// updatePurging handles messages while a permanent-delete (purge)
// confirmation is pending. This is a distinct flow from updateDeleting
// because purging is irreversible and only ever offered on the Deleted
// tab, whereas "x" on Todo/Done still means the reversible soft-delete.
func (m model) updatePurging(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "enter":
			m.purging = false
			task, ok := m.list.Selected()
			if !ok {
				return m, nil
			}
			return m, purgeTask(m.purger, taskID(task))
		case "n", "esc":
			m.purging = false
			return m, nil
		}
	}
	return m, nil
}

// updateCompleting handles messages while a done confirmation is pending.
// On the Deleted tab, confirming restores the task to pending first (since
// Taskwarrior's `done` refuses non-pending tasks) and then marks it done,
// moving it straight from Deleted to Done.
func (m model) updateCompleting(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "enter":
			m.completing = false
			task, ok := m.list.Selected()
			if !ok {
				return m, nil
			}
			if m.list.Status() == tasklist.TabDeleted {
				return m, restoreThenDoneTask(m.restorer, m.doner, taskID(task))
			}
			return m, doneTask(m.doner, taskID(task))
		case "n", "esc":
			m.completing = false
			return m, nil
		}
	}
	return m, nil
}

// updateRestoring handles messages while a restore/reopen confirmation is
// pending.
func (m model) updateRestoring(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "enter":
			m.restoring = false
			task, ok := m.list.Selected()
			if !ok {
				return m, nil
			}
			return m, restoreTask(m.restorer, taskID(task))
		case "n", "esc":
			m.restoring = false
			return m, nil
		}
	}
	return m, nil
}

// updateAdding handles messages while the add-task input panel is focused.
func (m model) updateAdding(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.adding = false
			m.add = m.add.Blur()
			return m, nil
		case "enter":
			description := strings.TrimSpace(m.add.Value())
			if description == "" {
				m.popups = m.popups.Push(popup.Message{Severity: popup.Warning, Text: "Description cannot be empty."})
				return m, nil
			}
			m.adding = false
			m.add = m.add.Blur()
			var extraArgs []string
			// Auto-assign the task to whatever real project is currently
			// selected via the Projects panel filter, so "a" from any
			// panel creates tasks already scoped to that project. Only a
			// non-nil, non-empty filter.project is a real project name:
			// nil means "(all)" (no filter) and "" means "(none)"
			// (explicitly project-less), neither of which should be
			// applied to the new task.
			if m.filter.project != nil && *m.filter.project != "" {
				extraArgs = append(extraArgs, "project:"+*m.filter.project)
			}
			return m, addTask(m.adder, description, extraArgs...)
		}
	}

	var cmd tea.Cmd
	m.add, cmd = m.add.Update(msg)
	return m, cmd
}

// updateRenaming handles messages while the rename-project input panel is
// focused: entering a new name for m.renameFrom. Leading/trailing
// whitespace is trimmed before it's used for anything (comparison,
// display, or the eventual rename), so "  chores" and "chores " both
// resolve to "chores". An unchanged (or empty) name is a silent no-op back
// to the Projects panel; otherwise it proceeds to a confirmation step,
// which is the merge-warning (danger) variant if the trimmed name matches
// an existing, different project.
func (m model) updateRenaming(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.renaming = false
			m.renameInput = m.renameInput.Blur()
			return m, nil
		case "enter":
			to := strings.TrimSpace(m.renameInput.Value())
			m.renaming = false
			m.renameInput = m.renameInput.Blur()
			if to == "" || to == m.renameFrom {
				return m, nil
			}
			m.renameTo = to
			m.renameConfirm = true
			for _, p := range m.projects.Projects() {
				if p == to {
					m.renameMerging = true
					break
				}
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.renameInput, cmd = m.renameInput.Update(msg)
	return m, cmd
}

// updateRenameConfirm handles messages while a project-rename confirmation
// (either the normal or the merge-warning/danger variant) is pending.
func (m model) updateRenameConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "enter":
			m.renameConfirm = false
			m.renameMerging = false
			return m, renameProject(m.reader, m.importer, m.renameFrom, m.renameTo)
		case "n", "esc":
			m.renameConfirm = false
			m.renameMerging = false
			m.renameFrom = ""
			m.renameTo = ""
			return m, nil
		}
	}
	return m, nil
}

// screenDims returns the model's last known terminal size, falling back to
// the same defaults gridDims() uses when no tea.WindowSizeMsg has arrived
// yet (e.g. in tests calling View() directly).
func (m model) screenDims() (width, height int) {
	width = m.width
	if width <= 0 {
		width = defaultGridWidth
	}
	height = m.height
	if height <= 0 {
		height = defaultGridHeight
	}
	return width, height
}

// baseView renders the grid plus whatever status line/prompt belongs
// underneath it, ignoring the adding/popup overlays layered on top by
// View(). This is the "background" the add-task box and any popup are
// composited over, so the rest of the UI stays visible behind them instead
// of being replaced by a blank screen.
func (m model) baseView() string {
	view := m.renderGrid()
	view += "\n" + statusbar.Render(m.statusBindings()) + "\n"
	return view
}

func (m model) View() string {
	if m.quitting {
		return "Exiting lazytask...\n"
	}

	width, height := m.screenDims()
	view := m.baseView()

	if m.adding {
		view = popup.Overlay(view, m.add.View(), width, height)
	}

	if m.deleting {
		if task, ok := m.list.Selected(); ok {
			var text string
			if m.list.Status() == tasklist.TabTodo {
				text = fmt.Sprintf("Delete task %d %q?", task.ID, task.Description)
			} else {
				text = fmt.Sprintf("Delete %q?", task.Description)
			}
			view = popup.Overlay(view, popup.ConfirmBox(text, width), width, height)
		}
	}

	if m.purging {
		if task, ok := m.list.Selected(); ok {
			text := fmt.Sprintf("Permanently delete %q? This cannot be undone.", task.Description)
			view = popup.Overlay(view, popup.ConfirmBox(text, width), width, height)
		}
	}

	if m.completing {
		if task, ok := m.list.Selected(); ok {
			var text string
			if m.list.Status() == tasklist.TabTodo {
				text = fmt.Sprintf("Mark task %d %q as done?", task.ID, task.Description)
			} else {
				text = fmt.Sprintf("Mark %q as done?", task.Description)
			}
			view = popup.Overlay(view, popup.ConfirmBox(text, width), width, height)
		}
	}

	if m.restoring {
		if task, ok := m.list.Selected(); ok {
			var text string
			if m.list.Status() == tasklist.TabDone {
				text = fmt.Sprintf("Reopen %q?", task.Description)
			} else {
				text = fmt.Sprintf("Restore %q as a todo?", task.Description)
			}
			view = popup.Overlay(view, popup.ConfirmBox(text, width), width, height)
		}
	}

	if m.renaming {
		view = popup.Overlay(view, m.renameInput.View(), width, height)
	}

	if m.renameConfirm {
		if m.renameMerging {
			text := fmt.Sprintf(
				"Renaming project %q to %q will merge its tasks into the existing project %q (including done/deleted tasks). This cannot be undone. Continue?",
				m.renameFrom, m.renameTo, m.renameTo)
			view = popup.Overlay(view, popup.DangerConfirmBox(text, width), width, height)
		} else {
			text := fmt.Sprintf("Rename project %q to %q? This updates every task in this project, including done/deleted ones.", m.renameFrom, m.renameTo)
			view = popup.Overlay(view, popup.ConfirmBox(text, width), width, height)
		}
	}

	if msg, ok := m.popups.Current(); ok {
		view = popup.Overlay(view, popup.Box(msg, width), width, height)
	}

	return view
}

// statusPanelContent returns the single content line shown in the Status
// panel: the id, status, project, and tags of whichever task is currently
// selected in the Tasks list. It updates live as the Tasks cursor moves,
// since it always reads the list's current selection.
func (m model) statusPanelContent() string {
	task, ok := m.list.Selected()
	if !ok {
		return "(no task selected)"
	}
	project := task.Project
	if project == "" {
		project = "(none)"
	}
	tags := "(none)"
	if len(task.Tags) > 0 {
		tags = strings.Join(task.Tags, ",")
	}
	return fmt.Sprintf("#%d  [%s]  P:%s  T:%s", task.ID, task.Status, project, tags)
}

// renderGrid lays out the lazygit-style panel grid: a left column of 3
// stacked rows (Status, Tasks, and a bottom row splitting Projects/Tags
// into side-by-side subcolumns) and one large panel (Details) filling the
// right column. Tasks (slot 2) and Projects (slot 3) have real content;
// Tags and Details remain placeholders until later chunks.
func (m model) renderGrid() string {
	leftWidth, rightWidth, statusHeight, _, bottomHeight, fullHeight := m.gridDims()
	_, tagsWidth := subColumnWidths(leftWidth)

	status := panel.Render(panelTitle(focusStatus), m.statusPanelContent(), leftWidth, statusHeight, m.focus == focusStatus)
	tasksPanel := m.list.View()
	projectsPanel := m.projects.View()
	tags := panel.Render(panelTitle(focusTags), "(coming soon)", tagsWidth, bottomHeight, m.focus == focusTags)
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, projectsPanel, tags)

	leftCol := lipgloss.JoinVertical(lipgloss.Left, status, tasksPanel, bottomRow)

	details := panel.Render(panelTitle(focusDetails), "(coming soon)", rightWidth, fullHeight, m.focus == focusDetails)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftCol, details)
}

func main() {
	// Use the alternate screen so the program owns the full terminal
	// buffer. Without it, resuming after an external process (e.g. the
	// editor launched by the 'e' key, via tea.ExecProcess) just repaints
	// the current view in place rather than clearing first, leaving old
	// frames stacked above the live one.
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running lazytask: %v\n", err)
		os.Exit(1)
	}
}
