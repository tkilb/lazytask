package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tkilb/lazytask/internal/config"
	"github.com/tkilb/lazytask/internal/editbuffer"
	"github.com/tkilb/lazytask/internal/editor"
	"github.com/tkilb/lazytask/internal/taskwarrior"
	"github.com/tkilb/lazytask/internal/ui/addform"
	"github.com/tkilb/lazytask/internal/ui/datepick"
	"github.com/tkilb/lazytask/internal/ui/panel"
	"github.com/tkilb/lazytask/internal/ui/popup"
	"github.com/tkilb/lazytask/internal/ui/projects"
	"github.com/tkilb/lazytask/internal/ui/statusbar"
	"github.com/tkilb/lazytask/internal/ui/tags"
	"github.com/tkilb/lazytask/internal/ui/taskform"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
	"github.com/tkilb/lazytask/internal/undo"
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

// searchScope identifies which list a "/" search prompt (see m.searching)
// currently applies to, since the prompt/input state is shared across the
// Tasks panel, the Projects panel, and the "p" quick project-filter
// popup (requirements.md Phase 1's continuation into Projects).
type searchScope int

const (
	searchScopeTasks searchScope = iota
	searchScopeProjects
	searchScopeProjectPicker
)

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
	{Key: "u", Label: "undo"},
	{Key: "ctrl+r", Label: "redo"},
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
	{Key: "h/m/l", Label: "priority"},
	{Key: "D", Label: "due date"},
	{Key: "p", Label: "project filter"},
	{Key: "ctrl+j/k", Label: "reorder"},
	{Key: "/", Label: "search"},
	{Key: "n/N", Label: "next/prev match"},
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

// tagsLocalBindings are only active while the Tags panel has focus:
// navigating the list automatically updates the shared tag filter.
var tagsLocalBindings = []statusbar.Binding{
	{Key: "↑/k", Label: "up"},
	{Key: "↓/j", Label: "down"},
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
	if m.focus == focusTags {
		bindings = append(bindings, tagsLocalBindings...)
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

// TaskPrioritizer is the subset of the taskwarrior client needed to set a
// task's priority, so it can be stubbed out in tests without shelling out
// to the real `task` binary.
type TaskPrioritizer interface {
	SetPriority(ctx context.Context, id, priority string) error
}

// TaskReRanker is the subset of the taskwarrior client needed to set a
// task's manual reorder rank (see taskwarrior.Task.UrgencyOffset), so it can be
// stubbed out in tests without shelling out to the real `task` binary.
type TaskReRanker interface {
	SetUrgencyOffset(ctx context.Context, id string, rank float64) error
}

// TaskDueSetter is the subset of the taskwarrior client needed to set a
// task's due date, so it can be stubbed out in tests without shelling out
// to the real `task` binary.
type TaskDueSetter interface {
	SetDue(ctx context.Context, id, due string) error
}

// TaskProjectSetter is the subset of the taskwarrior client needed to
// reassign a task's project, so it can be stubbed out in tests without
// shelling out to the real `task` binary.
type TaskProjectSetter interface {
	SetProject(ctx context.Context, id, project string) error
}

// TaskAnnotator is the subset of the taskwarrior client needed to add an
// annotation to a task, so it can be stubbed out in tests without
// shelling out to the real `task` binary.
type TaskAnnotator interface {
	Annotate(ctx context.Context, id, text string) error
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

// tagsLoadedMsg carries the distinct, sorted tag names for the Tags
// panel, sourced from the same unfiltered task query as fetchProjects
// rather than whatever filter the Tasks panel currently has applied.
type tagsLoadedMsg struct {
	tags   []string
	counts tags.Counts
}

// tagsErrMsg carries the error from a failed tags fetch.
type tagsErrMsg struct {
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

// taskDoneMsg carries the result of a successful Done call, plus the
// undo.Action that reverses it (see doneTask).
type taskDoneMsg struct {
	action undo.Action
}

// taskDoneErrMsg carries the error from a failed Done call.
type taskDoneErrMsg struct {
	err error
}

// taskDeletedMsg carries the result of a successful Delete call, plus the
// undo.Action that reverses it (see deleteTask).
type taskDeletedMsg struct {
	action undo.Action
}

// taskDeleteErrMsg carries the error from a failed Delete call.
type taskDeleteErrMsg struct {
	err error
}

// taskRestoredMsg carries the result of a successful Restore call, plus the
// undo.Action that reverses it (see restoreTask).
type taskRestoredMsg struct {
	action undo.Action
}

// taskRestoreErrMsg carries the error from a failed Restore call.
type taskRestoreErrMsg struct {
	err error
}

// taskPurgedMsg carries the result of a successful Purge call, plus the
// undo.Action that reverses it by re-importing the pre-purge snapshot (see
// purgeTask).
type taskPurgedMsg struct {
	action undo.Action
}

// taskPurgeErrMsg carries the error from a failed Purge call.
type taskPurgeErrMsg struct {
	err error
}

// taskPrioritySetMsg carries the result of a successful SetPriority call,
// plus the undo.Action that reverses it (see setPriorityTask) and the id of
// the task whose priority changed, so the cursor can be kept on it after
// the following refresh re-sorts the list by urgency.
type taskPrioritySetMsg struct {
	id     int
	action undo.Action
}

// taskPriorityErrMsg carries the error from a failed SetPriority call.
type taskPriorityErrMsg struct {
	err error
}

// taskDueSetMsg carries the result of a successful SetDue call, plus the
// undo.Action that reverses it (see setDueTask) and the id of the task
// whose due date changed, so the cursor can be kept on it after the
// following refresh re-sorts the list by urgency.
type taskDueSetMsg struct {
	id     int
	action undo.Action
}

// taskDueErrMsg carries the error from a failed SetDue call.
type taskDueErrMsg struct {
	err error
}

// taskProjectSetMsg carries the result of a successful SetProject call,
// plus the undo.Action that reverses it (see setProjectTask) and the id of
// the task whose project changed, so the cursor can be kept on it after
// the following refresh.
type taskProjectSetMsg struct {
	id     int
	action undo.Action
}

// taskProjectErrMsg carries the error from a failed SetProject call.
type taskProjectErrMsg struct {
	err error
}

// taskReorderedMsg carries the result of a successful manual-reorder
// SetUrgencyOffset call, plus the undo.Action that reverses it (see
// reorderTask) and the id of the task whose rank changed, so the cursor
// can be kept on it after the following refresh re-sorts the list.
type taskReorderedMsg struct {
	id     int
	action undo.Action
}

// taskReorderErrMsg carries the error from a failed SetUrgencyOffset call.
type taskReorderErrMsg struct {
	err error
}

// undoAppliedMsg carries the description of the undo.Action whose Undo
// func just ran successfully via the "u" key.
type undoAppliedMsg struct {
	description string
}

// undoErrMsg carries the error from a failed undo.Action.Undo invocation.
// The stack has already advanced past this entry (see runUndo); this is an
// intentional simplification for this first undo/redo chunk, matching how
// the rest of the app's mutation errors aren't automatically retried
// either.
type undoErrMsg struct {
	err error
}

// redoAppliedMsg carries the description of the undo.Action whose Redo
// func just ran successfully via the "ctrl+r" key.
type redoAppliedMsg struct {
	description string
}

// redoErrMsg carries the error from a failed undo.Action.Redo invocation.
type redoErrMsg struct {
	err error
}

// taskEditedMsg carries the result of a successful edit-and-reimport, plus
// the undo.Action that reverses it by re-importing the pre-edit snapshot
// (see editTaskCallback).
type taskEditedMsg struct {
	action undo.Action
}

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
	prioritizer    TaskPrioritizer
	reranker       TaskReRanker
	duer           TaskDueSetter
	projecter      TaskProjectSetter
	annotator      TaskAnnotator
	list           tasklist.Model
	add            taskform.Model
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
	datePicking    bool
	datePick       datepick.Model
	pickingProject bool
	projectPicker  projects.Model
	reassigning    bool
	reassignPicker projects.Model
	reassignTyping bool
	reassignInput  addform.Model
	searching      bool
	searchInput    textinput.Model
	searchScope    searchScope
	popups         popup.Model
	quitting       bool
	pendingFocusID int
	focus          panelFocus
	width          int
	height         int
	projects       projects.Model
	filter         filterState
	knownProjects  map[string]struct{}
	tags           tags.Model
	knownTags      map[string]struct{}
	undo           undo.Stack
}

// initialModel constructs the app's starting state, including restoring
// the project filter persisted by a previous session (see internal/config)
// so the app reopens filtered the way the user left it.
func initialModel() model {
	client := taskwarrior.NewClient()
	// Best-effort, idempotent: registers the "urgencyoffset" UDA (see
	// taskwarrior.Task.UrgencyOffset) so manual reordering has somewhere to
	// persist its per-task rank offset. A failure here (e.g. `task` not
	// installed) doesn't block startup — Ctrl+j/Ctrl+k just won't persist
	// correctly, same as any other taskwarrior mutation would fail.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := client.EnsureUDA(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not register urgencyoffset UDA: %v\n", err)
	}
	cancel()
	persisted := config.Load()
	return model{
		reader:         client,
		adder:          client,
		doner:          client,
		deleter:        client,
		restorer:       client,
		purger:         client,
		importer:       client,
		prioritizer:    client,
		reranker:       client,
		duer:           client,
		projecter:      client,
		annotator:      client,
		list:           tasklist.New(nil).SetFocused(true),
		add:            taskform.New(),
		datePick:       datepick.New(),
		projects:       projects.New(),
		projectPicker:  projects.New(),
		reassignPicker: projects.New(),
		knownProjects:  make(map[string]struct{}),
		tags:           tags.New(),
		knownTags:      make(map[string]struct{}),
		filter:         filterState{project: persisted.Project, tag: persisted.Tag},
	}
}

// setFocus updates which panel has focus, keeping the Tasks list's and
// Projects/Tags panels' own focused flags (used for their border
// highlight) in sync.
func (m model) setFocus(f panelFocus) model {
	m.focus = f
	m.list = m.list.SetFocused(f == focusTasks)
	m.projects = m.projects.SetFocused(f == focusProjects)
	m.tags = m.tags.SetFocused(f == focusTags)
	return m
}

// applyProjectSelection updates m.filter from a Projects-panel entry label
// (a project name, or projects.NoneLabel/projects.AllLabel), applying two
// rules: the project component updates as usual, and — only when the
// project actually changes, not merely when this is called again with the
// same already-selected label (e.g. a "k" keypress that doesn't move the
// cursor past the top of the list) — the tag filter resets back to
// tags.AnyLabel and the Tags panel's own visible cursor follows suit. This
// keeps the Tags panel's data (scoped to the current project, see
// fetchTags) and its selection from silently pointing at a tag that may
// not even exist under the newly selected project.
//
// It also clears m.knownTags, the Tags-panel "sticky within this session"
// cache (see mergeKnownTags): that cache is only valid for tags seen
// *under the current project scope* — carrying tags forward from a
// previously selected project would otherwise leave them visible in the
// Tags panel even though the newly selected project has none of its own
// pending tasks with that tag.
//
// It returns the updated model, a tea.Cmd re-fetching Tasks/Tags and
// persisting the change (nil if nothing changed), and whether the filter
// actually changed.
func (m model) applyProjectSelection(label string) (model, tea.Cmd, bool) {
	newFilter := m.filter.withProjectSelection(label)
	if !stringPtrEqual(newFilter.project, m.filter.project) {
		newFilter.tag = nil
		m.knownTags = nil
	}
	if newFilter.equal(m.filter) {
		return m, nil, false
	}
	m.filter = newFilter
	m.tags = m.tags.SelectLabel(tags.AnyLabel)
	cmd := tea.Batch(
		fetchTasks(m.reader, m.taskFilters()...),
		fetchTags(m.reader, m.filter.projectOnlyFilter()),
		saveFilter(m.filter),
	)
	return m, cmd, true
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

// fetchTags returns a tea.Cmd that loads the distinct tags across pending
// tasks scoped to projectFilter (a "project:<name>" fragment, or "" for no
// project scoping — see filterState.projectOnlyFilter). It deliberately
// ignores any currently selected tag filter, for the same reason
// fetchProjects ignores the project filter: applying a tag filter must not
// shrink the very list used to change/clear that filter. Scoping to the
// active project, however, is intentional — the Tags panel is meant to
// show only the tags relevant to whatever project is currently selected.
func fetchTags(reader TaskReader, projectFilter string) tea.Cmd {
	return func() tea.Msg {
		filters := []string{"status:pending"}
		if projectFilter != "" {
			filters = append(filters, projectFilter)
		}
		exported, err := reader.Export(context.Background(), filters...)
		if err != nil {
			return tagsErrMsg{err: err}
		}
		return tagsLoadedMsg{tags: distinctTags(exported), counts: tagCounts(exported)}
	}
}

// saveFilter returns a tea.Cmd that persists f's project and tag filter to
// disk (see internal/config) so it's restored on the next app launch. On
// failure it reports a filterSaveErrMsg rather than blocking or reverting
// the (already-applied) in-memory filter change.
func saveFilter(f filterState) tea.Cmd {
	return func() tea.Msg {
		if err := config.Save(config.State{Project: f.project, Tag: f.tag}); err != nil {
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

// tagCounts tallies pending ("todo") task counts per tag, plus the total
// for the AnyLabel special entry (every pending task), so the Tags panel
// can render a "(N)" field next to each entry. A task with multiple tags
// contributes to each of its tags' counts.
func tagCounts(tasks []taskwarrior.Task) tags.Counts {
	c := tags.Counts{ByTag: make(map[string]int)}
	for _, t := range tasks {
		if t.Status != "pending" {
			continue
		}
		c.All++
		for _, tag := range t.Tags {
			c.ByTag[tag]++
		}
	}
	return c
}

// mergeKnownTags folds newly-seen tag names into the session's running set
// of known tags and returns the sorted union, making a tag "sticky" for the
// remainder of the session once seen with at least one pending task — the
// same rationale as mergeKnownProjects.
func (m *model) mergeKnownTags(names []string) []string {
	if m.knownTags == nil {
		m.knownTags = make(map[string]struct{})
	}
	for _, n := range names {
		m.knownTags[n] = struct{}{}
	}
	merged := make([]string, 0, len(m.knownTags))
	for n := range m.knownTags {
		merged = append(merged, n)
	}
	sort.Strings(merged)
	return merged
}

// distinctTags returns the sorted set of distinct tags found across tasks.
func distinctTags(tasks []taskwarrior.Task) []string {
	seen := make(map[string]struct{})
	names := make([]string, 0)
	for _, t := range tasks {
		for _, tag := range t.Tags {
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			names = append(names, tag)
		}
	}
	sort.Strings(names)
	return names
}

// addTask returns a tea.Cmd that creates a new task via adder, passing
// along any extra `task add` argument fragments (e.g. "project:chores"),
// then—if annotations is non-empty—adds one Taskwarrior annotation per
// non-empty line of it via annotator (see splitAnnotationLines). If the
// Add itself succeeds but a subsequent Annotate call fails, the task
// still exists (it is not rolled back); the error is surfaced so the user
// knows the annotation(s) didn't fully land, but treated the same as any
// other post-add failure for undo/refresh purposes.
func addTask(adder TaskAdder, annotator TaskAnnotator, description, annotations string, extraArgs ...string) tea.Cmd {
	return func() tea.Msg {
		id, err := adder.Add(context.Background(), description, extraArgs...)
		if err != nil {
			return taskAddErrMsg{err: err}
		}
		for _, line := range splitAnnotationLines(annotations) {
			if err := annotator.Annotate(context.Background(), strconv.Itoa(id), line); err != nil {
				return taskAddErrMsg{err: fmt.Errorf("task added but annotation failed: %w", err)}
			}
		}
		return taskAddedMsg{id: id}
	}
}

// splitAnnotationLines splits a (possibly multi-line) Annotations field
// value into individual annotation texts, one per non-blank line, with
// leading/trailing whitespace trimmed from each. Blank lines are dropped
// rather than becoming empty annotations.
func splitAnnotationLines(value string) []string {
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

// doneTask returns a tea.Cmd that marks the given task id as done via
// doner. If wasDeleted is true, the task is restored to pending first
// (Taskwarrior's `done` refuses to act on a non-pending task), moving a
// Deleted task straight to Done; the resulting undo.Action reverses either
// case: back to pending (via restorer) or back to deleted (via deleter).
func doneTask(doner TaskDoner, restorer TaskRestorer, deleter TaskDeleter, id string, wasDeleted bool) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if wasDeleted {
			if err := restorer.Restore(ctx, id); err != nil {
				return taskDoneErrMsg{err: err}
			}
		}
		if err := doner.Done(ctx, id); err != nil {
			return taskDoneErrMsg{err: err}
		}
		return taskDoneMsg{action: undo.Action{
			Description: fmt.Sprintf("mark task %s done", id),
			Undo: func() error {
				if wasDeleted {
					return deleter.Delete(context.Background(), id)
				}
				return restorer.Restore(context.Background(), id)
			},
			Redo: func() error {
				ctx := context.Background()
				if wasDeleted {
					if err := restorer.Restore(ctx, id); err != nil {
						return err
					}
				}
				return doner.Done(ctx, id)
			},
		}}
	}
}

// deleteTask returns a tea.Cmd that deletes the given task id via deleter,
// returning an undo.Action that reverses it via restorer.
// deleteTask returns a tea.Cmd that deletes the given task id via deleter.
// from records which tab (Todo or Done) the task was deleted from, so the
// resulting undo.Action's Undo can send it back to that same status: a
// plain restore-to-pending if it was deleted from Todo, or a
// restore-then-done if it was deleted from Done (mirroring doneTask's
// wasDeleted handling, since Taskwarrior's `done` refuses a non-pending
// task).
func deleteTask(deleter TaskDeleter, restorer TaskRestorer, doner TaskDoner, id string, from tasklist.StatusTab) tea.Cmd {
	return func() tea.Msg {
		if err := deleter.Delete(context.Background(), id); err != nil {
			return taskDeleteErrMsg{err: err}
		}
		return taskDeletedMsg{action: undo.Action{
			Description: fmt.Sprintf("delete task %s", id),
			Undo: func() error {
				ctx := context.Background()
				if err := restorer.Restore(ctx, id); err != nil {
					return err
				}
				if from == tasklist.TabDone {
					return doner.Done(ctx, id)
				}
				return nil
			},
			Redo: func() error { return deleter.Delete(context.Background(), id) },
		}}
	}
}

// restoreTask returns a tea.Cmd that restores the given task id back to
// pending status via restorer. from records which tab (Done or Deleted)
// the task was restored from, so the resulting undo.Action's Undo can send
// it back to that same status.
func restoreTask(restorer TaskRestorer, doner TaskDoner, deleter TaskDeleter, id string, from tasklist.StatusTab) tea.Cmd {
	return func() tea.Msg {
		if err := restorer.Restore(context.Background(), id); err != nil {
			return taskRestoreErrMsg{err: err}
		}
		return taskRestoredMsg{action: undo.Action{
			Description: fmt.Sprintf("restore task %s", id),
			Undo: func() error {
				ctx := context.Background()
				switch from {
				case tasklist.TabDone:
					return doner.Done(ctx, id)
				case tasklist.TabDeleted:
					return deleter.Delete(ctx, id)
				}
				return nil
			},
			Redo: func() error { return restorer.Restore(context.Background(), id) },
		}}
	}
}

// purgeTask returns a tea.Cmd that permanently removes task via purger.
// Unlike Taskwarrior's own purge, this is undoable within lazytask: task's
// full field set is captured as a JSON snapshot before purging, and the
// resulting undo.Action's Undo re-creates the task via importer.Import
// (same UUID, so it slots back in as the same task) rather than relying on
// Taskwarrior itself, which offers no way to recover a purged task.
func purgeTask(purger TaskPurger, importer TaskImporter, task taskwarrior.Task) tea.Cmd {
	return func() tea.Msg {
		id := taskID(task)
		snapshot, err := json.Marshal(task)
		if err != nil {
			return taskPurgeErrMsg{err: err}
		}
		if err := purger.Purge(context.Background(), id); err != nil {
			return taskPurgeErrMsg{err: err}
		}
		return taskPurgedMsg{action: undo.Action{
			Description: fmt.Sprintf("purge task %s", id),
			Undo:        func() error { return importer.Import(context.Background(), snapshot) },
			Redo:        func() error { return purger.Purge(context.Background(), id) },
		}}
	}
}

// setPriorityTask returns a tea.Cmd that sets task's priority to priority
// via prioritizer, returning an undo.Action that reverses it back to the
// task's prior priority.
func setPriorityTask(prioritizer TaskPrioritizer, task taskwarrior.Task, priority string) tea.Cmd {
	return func() tea.Msg {
		id := taskID(task)
		prior := task.Priority
		if err := prioritizer.SetPriority(context.Background(), id, priority); err != nil {
			return taskPriorityErrMsg{err: err}
		}
		return taskPrioritySetMsg{id: task.ID, action: undo.Action{
			Description: fmt.Sprintf("set task %s priority to %s", id, priority),
			Undo: func() error {
				return prioritizer.SetPriority(context.Background(), id, prior)
			},
			Redo: func() error {
				return prioritizer.SetPriority(context.Background(), id, priority)
			},
		}}
	}
}

// taskDueLayout is Taskwarrior's combined UTC export/import format for
// date attributes (e.g. "20240115T140000Z"), as confirmed by `task
// export`; using this rather than a locale-dependent format means SetDue
// is interpreted the same way regardless of the user's configured
// dateformat.
const taskDueLayout = "20060102T150405Z"

// resolveEditedDue resolves the Due field's text as read back from the
// $EDITOR edit buffer (see internal/editbuffer) into the taskDueLayout
// string taskwarrior expects, or reports it as invalid. An empty/blank
// input clears the due date. A value already in taskDueLayout (i.e.
// unchanged from what Serialize wrote out) passes through unchanged.
// Otherwise it's resolved via datepick.ResolveInput relative to now —
// the same cord ("2d", "1w", "2b") and flexible-absolute-date parsing the
// add form's Due Date field uses — so the edit flow no longer requires a
// literal taskwarrior-format date.
func resolveEditedDue(input string, now time.Time) (due string, ok bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", true
	}
	if _, err := time.Parse(taskDueLayout, trimmed); err == nil {
		return trimmed, true
	}
	if resolved, ok := datepick.ResolveInput(trimmed, now); ok {
		return resolved.UTC().Format(taskDueLayout), true
	}
	return "", false
}

// setDueTask returns a tea.Cmd that sets task's due date to due via duer,
// returning an undo.Action that reverses it back to the task's prior due
// date (which may be empty, clearing it back to "no due date").
func setDueTask(duer TaskDueSetter, task taskwarrior.Task, due time.Time) tea.Cmd {
	return func() tea.Msg {
		id := taskID(task)
		prior := task.Due
		formatted := due.UTC().Format(taskDueLayout)
		if err := duer.SetDue(context.Background(), id, formatted); err != nil {
			return taskDueErrMsg{err: err}
		}
		return taskDueSetMsg{id: task.ID, action: undo.Action{
			Description: fmt.Sprintf("set task %s due date to %s", id, formatted),
			Undo: func() error {
				return duer.SetDue(context.Background(), id, prior)
			},
			Redo: func() error {
				return duer.SetDue(context.Background(), id, formatted)
			},
		}}
	}
}

// setProjectTask returns a tea.Cmd that reassigns task's project to
// project (a real project name, or "" to clear it back to project-less)
// via projecter, returning an undo.Action that reverses it back to the
// task's prior project.
func setProjectTask(projecter TaskProjectSetter, task taskwarrior.Task, project string) tea.Cmd {
	return func() tea.Msg {
		id := taskID(task)
		prior := task.Project
		if err := projecter.SetProject(context.Background(), id, project); err != nil {
			return taskProjectErrMsg{err: err}
		}
		return taskProjectSetMsg{id: task.ID, action: undo.Action{
			Description: fmt.Sprintf("set task %s project to %q", id, project),
			Undo: func() error {
				return projecter.SetProject(context.Background(), id, prior)
			},
			Redo: func() error {
				return projecter.SetProject(context.Background(), id, project)
			},
		}}
	}
}

// reorderEpsilon is the small offset added past a neighbor's effective
// urgency (Urgency+UrgencyOffset) when computing a moved task's new UrgencyOffset, so
// it sorts just past that neighbor without otherwise disturbing the list.
const reorderEpsilon = 0.0001

// reorderTask returns a tea.Cmd that manually reorders task past neighbor
// in the Tasks panel's urgency-sorted list (Ctrl+j/Ctrl+k), by setting
// task's UrgencyOffset UDA via reranker so its effective urgency
// (Urgency+UrgencyOffset) lands just past neighbor's, leaving neighbor's own
// rank untouched. down is true when moving task later in the list (past
// the neighbor below it), false when moving it earlier (past the neighbor
// above it). Returns an undo.Action that restores task's prior UrgencyOffset.
func reorderTask(reranker TaskReRanker, task, neighbor taskwarrior.Task, down bool) tea.Cmd {
	return func() tea.Msg {
		id := taskID(task)
		prior := task.UrgencyOffset
		neighborUrgency := neighbor.Urgency + neighbor.UrgencyOffset
		var newRank float64
		if down {
			newRank = neighborUrgency - reorderEpsilon - task.Urgency
		} else {
			newRank = neighborUrgency + reorderEpsilon - task.Urgency
		}
		if err := reranker.SetUrgencyOffset(context.Background(), id, newRank); err != nil {
			return taskReorderErrMsg{err: err}
		}
		return taskReorderedMsg{id: task.ID, action: undo.Action{
			Description: fmt.Sprintf("reorder task %s", id),
			Undo: func() error {
				return reranker.SetUrgencyOffset(context.Background(), id, prior)
			},
			Redo: func() error {
				return reranker.SetUrgencyOffset(context.Background(), id, newRank)
			},
		}}
	}
}

// runUndo returns a tea.Cmd that invokes a previously-pushed undo.Action's
// Undo func (see the "u" key binding in Update).
func runUndo(a undo.Action) tea.Cmd {
	return func() tea.Msg {
		if err := a.Undo(); err != nil {
			return undoErrMsg{err: err}
		}
		return undoAppliedMsg{description: a.Description}
	}
}

// runRedo returns a tea.Cmd that invokes a previously-undone undo.Action's
// Redo func (see the "ctrl+r" key binding in Update).
func runRedo(a undo.Action) tea.Cmd {
	return func() tea.Msg {
		if err := a.Redo(); err != nil {
			return redoErrMsg{err: err}
		}
		return redoAppliedMsg{description: a.Description}
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

	return tea.ExecProcess(cmd, editTaskCallback(importer, session, task, time.Now))
}

// editTaskCallback builds the tea.ExecCallback run once the editor process
// launched by editTask exits: it reads back session's temp file, parses
// its editable fields, resolves the Due field's shorthand (see
// resolveEditedDue) exactly like the add form's Due Date field does,
// applies the result onto original (preserving read-only fields such as
// UUID untouched, regardless of what the user may have typed in that
// section of the buffer), and re-imports the result via importer. now is
// the reference instant the Due field's cord is resolved relative to
// (time.Now in production, fixed in tests). Split out from editTask so it
// can be unit-tested without going through tea.ExecProcess/a real editor
// process.
func editTaskCallback(importer TaskImporter, session *editor.Session, original taskwarrior.Task, now func() time.Time) func(error) tea.Msg {
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

		due, ok := resolveEditedDue(fields.Due, now())
		if !ok {
			return taskEditErrMsg{err: fmt.Errorf("invalid due date %q", fields.Due)}
		}
		fields.Due = due

		updated := editbuffer.Apply(original, fields)

		originalSnapshot, err := json.Marshal(original)
		if err != nil {
			return taskEditErrMsg{err: err}
		}
		updatedSnapshot, err := json.Marshal(updated)
		if err != nil {
			return taskEditErrMsg{err: err}
		}
		if err := importer.Import(context.Background(), updatedSnapshot); err != nil {
			return taskEditErrMsg{err: err}
		}
		return taskEditedMsg{action: undo.Action{
			Description: fmt.Sprintf("edit task %s", taskID(original)),
			Undo:        func() error { return importer.Import(context.Background(), originalSnapshot) },
			Redo:        func() error { return importer.Import(context.Background(), updatedSnapshot) },
		}}
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
	return tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader), fetchTags(m.reader, m.filter.projectOnlyFilter()))
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
	if m.datePicking {
		return m.updateDatePicking(msg)
	}
	if m.pickingProject {
		return m.updateProjectPicking(msg)
	}
	if m.reassigning {
		return m.updateReassigning(msg)
	}
	if m.reassignTyping {
		return m.updateReassignTyping(msg)
	}
	if m.searching {
		return m.updateSearching(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		leftWidth, _, _, tasksHeight, bottomHeight, _ := m.gridDims()
		projectsWidth, tagsWidth := subColumnWidths(leftWidth)
		var listCmd, projectsCmd, tagsCmd tea.Cmd
		m.list, listCmd = m.list.Update(tea.WindowSizeMsg{Width: leftWidth, Height: tasksHeight})
		m.projects, projectsCmd = m.projects.Update(tea.WindowSizeMsg{Width: projectsWidth, Height: bottomHeight})
		m.tags, tagsCmd = m.tags.Update(tea.WindowSizeMsg{Width: tagsWidth, Height: bottomHeight})
		return m, tea.Batch(listCmd, projectsCmd, tagsCmd)
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
			// Pre-fill the Project field with whatever real project is
			// currently selected via the Projects panel filter, so "a"
			// from any panel starts already scoped to that project (the
			// user can still edit/clear it). Only a non-nil, non-empty
			// filter.project is a real project name: nil means "(all)"
			// (no filter) and "" means "(none)" (explicitly project-less),
			// neither of which should pre-fill the field.
			if m.filter.project != nil && *m.filter.project != "" {
				m.add = m.add.SetProject(*m.filter.project)
			}
			return m, m.add.Init()
		case "u":
			act, ok := m.undo.Undo()
			if !ok {
				m.popups = m.popups.Push(popup.Message{Severity: popup.Info, Text: "Nothing to undo"})
				return m, nil
			}
			return m, runUndo(act)
		case "ctrl+r":
			act, ok := m.undo.Redo()
			if !ok {
				m.popups = m.popups.Push(popup.Message{Severity: popup.Info, Text: "Nothing to redo"})
				return m, nil
			}
			return m, runRedo(act)
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
			switch msg.String() {
			case "/":
				var cmd tea.Cmd
				m, cmd = m.startSearch("search projects...", searchScopeProjects)
				return m, cmd
			case "n":
				if newProjects, ok := m.projects.NextMatch(); ok {
					m.projects = newProjects
					return m.afterProjectsCursorMove()
				}
				return m, nil
			case "N":
				if newProjects, ok := m.projects.PrevMatch(); ok {
					m.projects = newProjects
					return m.afterProjectsCursorMove()
				}
				return m, nil
			case "esc":
				m.projects = m.projects.ClearSearch()
				return m, nil
			}
			var cmd tea.Cmd
			m.projects, cmd = m.projects.Update(msg)
			if label, ok := m.projects.Selected(); ok {
				if newM, refreshCmd, changed := m.applyProjectSelection(label); changed {
					return newM, tea.Batch(cmd, refreshCmd)
				}
			}
			return m, cmd
		}
		// Tags-panel-local: navigation (up/down/j/k) is applied directly
		// here for the same reason as the Projects panel above — moving
		// the cursor needs to update the shared filter and refetch Tasks
		// immediately (no "enter" required).
		if m.focus == focusTags {
			var cmd tea.Cmd
			m.tags, cmd = m.tags.Update(msg)
			if label, ok := m.tags.Selected(); ok {
				newFilter := m.filter.withTagSelection(label)
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
		case "h", "m", "l":
			if task, ok := m.list.Selected(); ok {
				return m, setPriorityTask(m.prioritizer, task, strings.ToUpper(msg.String()))
			}
			return m, nil
		case "D":
			if _, ok := m.list.Selected(); ok {
				m.datePicking = true
				m.datePick = m.datePick.Focus()
				return m, m.datePick.Init()
			}
			return m, nil
		case "p":
			w, h := projectPickerSize(len(m.projects.Projects()) + 2)
			m.projectPicker = m.projects.SetFocused(true).SetTitle("Project Filter")
			var cmd tea.Cmd
			m.projectPicker, cmd = m.projectPicker.Update(tea.WindowSizeMsg{Width: w, Height: h})
			m.pickingProject = true
			return m, cmd
		case "P":
			if _, ok := m.list.Selected(); ok {
				w, h := projectPickerSize(len(m.projects.Projects()) + 2)
				m.reassignPicker = m.projects.SetFocused(true).SetTitle("Reassign Project").SetReassignMode(true)
				var cmd tea.Cmd
				m.reassignPicker, cmd = m.reassignPicker.Update(tea.WindowSizeMsg{Width: w, Height: h})
				m.reassigning = true
				return m, cmd
			}
			return m, nil
		case "ctrl+j":
			if task, ok := m.list.Selected(); ok {
				if neighbor, ok := m.list.Neighbor(1); ok {
					return m, reorderTask(m.reranker, task, neighbor, true)
				}
			}
			return m, nil
		case "ctrl+k":
			if task, ok := m.list.Selected(); ok {
				if neighbor, ok := m.list.Neighbor(-1); ok {
					return m, reorderTask(m.reranker, task, neighbor, false)
				}
			}
			return m, nil
		case "/":
			var cmd tea.Cmd
			m, cmd = m.startSearch("search description...", searchScopeTasks)
			return m, cmd
		case "n":
			if newList, ok := m.list.NextMatch(); ok {
				m.list = newList
			}
			return m, nil
		case "N":
			if newList, ok := m.list.PrevMatch(); ok {
				m.list = newList
			}
			return m, nil
		case "esc":
			m.list = m.list.ClearSearch()
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
		m.projects = m.projects.SetProjects(m.mergeKnownProjects(msg.projects)).SetCounts(msg.counts)
		label := m.filter.projectLabel()
		if !m.projects.HasLabel(label) {
			// The active filter (e.g. restored from a previous session's
			// persisted state) refers to a project that no longer has any
			// pending tasks, so it doesn't appear as a selectable entry
			// here. SelectLabel would silently leave the panel's cursor
			// wherever it already was (defaulting to AllLabel), which
			// would show "(all)" selected while m.filter still applied
			// the stale project filter to Tasks, making the list appear
			// empty. Clear the filter (via applyProjectSelection, which
			// also resets any tag filter/scoping to match the now-cleared
			// project) so the visible selection and the actual task query
			// agree.
			m.projects = m.projects.SelectLabel(projects.AllLabel)
			if newM, cmd, changed := m.applyProjectSelection(projects.AllLabel); changed {
				return newM, cmd
			}
			return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), saveFilter(m.filter))
		}
		m.projects = m.projects.SelectLabel(label)
		return m, nil
	case projectsErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case tagsLoadedMsg:
		m.tags = m.tags.SetTags(m.mergeKnownTags(msg.tags)).SetCounts(msg.counts)
		label := m.filter.tagLabel()
		if !m.tags.HasLabel(label) {
			// Same rationale as the projectsLoadedMsg case above: a
			// persisted tag filter that no longer has any pending tasks
			// must be cleared so the visible selection and the actual
			// task query agree.
			m.filter.tag = nil
			m.tags = m.tags.SelectLabel(tags.AnyLabel)
			return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), saveFilter(m.filter))
		}
		m.tags = m.tags.SelectLabel(label)
		return m, nil
	case tagsErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case taskAddedMsg:
		m.add = m.add.Reset()
		m.pendingFocusID = msg.id
		return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader), fetchTags(m.reader, m.filter.projectOnlyFilter()))
	case taskAddErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case taskDoneMsg:
		m.undo.Push(msg.action)
		return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader), fetchTags(m.reader, m.filter.projectOnlyFilter()))
	case taskDoneErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case taskDeletedMsg:
		m.undo.Push(msg.action)
		return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader), fetchTags(m.reader, m.filter.projectOnlyFilter()))
	case taskDeleteErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case taskRestoredMsg:
		m.undo.Push(msg.action)
		return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader), fetchTags(m.reader, m.filter.projectOnlyFilter()))
	case taskRestoreErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case taskPurgedMsg:
		m.undo.Push(msg.action)
		return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader), fetchTags(m.reader, m.filter.projectOnlyFilter()))
	case taskPurgeErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case taskPrioritySetMsg:
		m.undo.Push(msg.action)
		m.pendingFocusID = msg.id
		return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader), fetchTags(m.reader, m.filter.projectOnlyFilter()))
	case taskPriorityErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case taskDueSetMsg:
		m.undo.Push(msg.action)
		m.pendingFocusID = msg.id
		return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader), fetchTags(m.reader, m.filter.projectOnlyFilter()))
	case taskDueErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case taskProjectSetMsg:
		m.undo.Push(msg.action)
		m.pendingFocusID = msg.id
		return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader), fetchTags(m.reader, m.filter.projectOnlyFilter()))
	case taskProjectErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case taskReorderedMsg:
		m.undo.Push(msg.action)
		m.pendingFocusID = msg.id
		return m, fetchTasks(m.reader, m.taskFilters()...)
	case taskReorderErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case undoAppliedMsg:
		m.popups = m.popups.Push(popup.Message{Severity: popup.Info, Text: "Undo: " + msg.description})
		return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader), fetchTags(m.reader, m.filter.projectOnlyFilter()))
	case undoErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case redoAppliedMsg:
		m.popups = m.popups.Push(popup.Message{Severity: popup.Info, Text: "Redo: " + msg.description})
		return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader), fetchTags(m.reader, m.filter.projectOnlyFilter()))
	case redoErrMsg:
		m.popups = m.popups.Push(errPopup(msg.err))
		return m, nil
	case taskEditedMsg:
		m.undo.Push(msg.action)
		return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader), fetchTags(m.reader, m.filter.projectOnlyFilter()))
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
		return m, tea.Batch(fetchTasks(m.reader, m.taskFilters()...), fetchProjects(m.reader), fetchTags(m.reader, m.filter.projectOnlyFilter()), saveFilter(m.filter))
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
			return m, deleteTask(m.deleter, m.restorer, m.doner, taskID(task), m.list.Status())
		case "n", "esc":
			m.deleting = false
			return m, nil
		}
	}
	return m, nil
}

// updatePurging handles messages while a permanent-delete (purge)
// confirmation is pending. This is a distinct flow from updateDeleting
// because purging is a much more destructive operation (Taskwarrior itself
// offers no way to recover a purged task) and is only ever offered on the
// Deleted tab, whereas "x" on Todo/Done still means the reversible
// soft-delete. lazytask's own undo stack (see purgeTask) makes it
// undoable within the running session, but only via "u" here, not via
// Taskwarrior's own tooling.
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
			return m, purgeTask(m.purger, m.importer, task)
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
				return m, doneTask(m.doner, m.restorer, m.deleter, taskID(task), true)
			}
			return m, doneTask(m.doner, m.restorer, m.deleter, taskID(task), false)
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
			return m, restoreTask(m.restorer, m.doner, m.deleter, taskID(task), m.list.Status())
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
			// While the Annotations field has focus, <enter> means "insert
			// a newline" (bubbles' textarea's own default binding), not
			// "submit the form" — fall through to the generic forwarding
			// below instead of treating it as submit.
			if m.add.FocusedField() == taskform.FieldAnnotations {
				break
			}
			description := strings.TrimSpace(m.add.Description())
			if description == "" {
				m.popups = m.popups.Push(popup.Message{Severity: popup.Warning, Text: "Description cannot be empty."})
				return m, nil
			}
			priority, ok := m.add.Priority()
			if !ok {
				m.popups = m.popups.Push(popup.Message{Severity: popup.Warning, Text: "Priority must be H, M, L, or blank."})
				return m, nil
			}
			// A blank Priority field defaults to M rather than leaving the
			// new task with no priority at all.
			if priority == "" {
				priority = "M"
			}
			var extraArgs []string
			if project := strings.TrimSpace(m.add.Project()); project != "" {
				extraArgs = append(extraArgs, "project:"+project)
			}
			extraArgs = append(extraArgs, "priority:"+priority)
			if dueInput := strings.TrimSpace(m.add.DueInput()); dueInput != "" {
				due, ok := m.add.ResolveDue()
				if !ok {
					m.popups = m.popups.Push(popup.Message{Severity: popup.Warning, Text: "Invalid due date."})
					return m, nil
				}
				extraArgs = append(extraArgs, "due:"+due.UTC().Format(taskDueLayout))
			}
			m.adding = false
			m.add = m.add.Blur()
			return m, addTask(m.adder, m.annotator, description, m.add.Annotations(), extraArgs...)
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

// updateDatePicking handles messages while the due-date quick-pick popup is
// focused: entering a cord (e.g. "2d") or an absolute date for the
// selected task's new due date. An empty or unparseable input surfaces a
// warning popup and leaves the picker open rather than silently doing
// nothing, so the user knows why enter didn't submit.
func (m model) updateDatePicking(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.datePicking = false
			m.datePick = m.datePick.Blur()
			return m, nil
		case "enter":
			due, ok := m.datePick.Resolved()
			if !ok {
				m.popups = m.popups.Push(popup.Message{Severity: popup.Warning, Text: "Enter a valid cord (2d, 1w, 2b) or date (YYYY-MM-DD)."})
				return m, nil
			}
			task, ok := m.list.Selected()
			m.datePicking = false
			m.datePick = m.datePick.Blur()
			if !ok {
				return m, nil
			}
			return m, setDueTask(m.duer, task, due)
		}
	}

	var cmd tea.Cmd
	m.datePick, cmd = m.datePick.Update(msg)
	return m, cmd
}

// projectPickerWidth is the fixed outer width of the quick project-filter
// popup (see updateProjectPicking); unlike the Projects panel in the grid,
// it doesn't need to track the terminal's left-column width.
const projectPickerWidth = 40

// projectPickerSize returns the outer width/height to size the quick
// project-filter popup's projects.Model at, given entryCount selectable
// entries (project names plus the AllLabel/NoneLabel special entries): tall
// enough to show up to 15 entries at once without scrolling, but no
// smaller than 3, plus the 2 border rows projects.Model.View() expects the
// outer height to include.
func projectPickerSize(entryCount int) (width, height int) {
	rows := entryCount
	if rows > 15 {
		rows = 15
	}
	if rows < 3 {
		rows = 3
	}
	return projectPickerWidth, rows + 2
}

// updateProjectPicking handles messages while the quick project-filter
// popup (opened via "p" from the Tasks panel) is active. Unlike the
// Projects panel itself, moving the cursor here doesn't touch the shared
// filter live — it's only applied on "enter" — so "esc" cleanly discards
// an in-progress selection without side effects (or, while a "/" search is
// active, clears the search first, only closing the popup on a second
// "esc" — matching lazygit's "innermost thing first" escape convention).
func (m model) updateProjectPicking(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.searching {
		return m.updateSearching(msg)
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "/":
			var cmd tea.Cmd
			m, cmd = m.startSearch("search projects...", searchScopeProjectPicker)
			return m, cmd
		case "n":
			if newPicker, ok := m.projectPicker.NextMatch(); ok {
				m.projectPicker = newPicker
			}
			return m, nil
		case "N":
			if newPicker, ok := m.projectPicker.PrevMatch(); ok {
				m.projectPicker = newPicker
			}
			return m, nil
		case "esc":
			if m.projectPicker.HasActiveSearch() {
				m.projectPicker = m.projectPicker.ClearSearch()
				return m, nil
			}
			m.pickingProject = false
			return m, nil
		case "enter":
			m.pickingProject = false
			label, ok := m.projectPicker.Selected()
			if !ok {
				return m, nil
			}
			m.projects = m.projects.SelectLabel(label)
			if newM, cmd, changed := m.applyProjectSelection(label); changed {
				return newM, cmd
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.projectPicker, cmd = m.projectPicker.Update(msg)
	return m, cmd
}

// updateReassigning handles messages while the task project-reassign popup
// (opened via "P" from the Tasks panel) is active: navigating/selecting an
// existing project (or NoneLabel, to clear the task's project) applies
// immediately on "enter", while selecting NewProjectLabel instead
// transitions to updateReassignTyping to key in a brand new project name.
func (m model) updateReassigning(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.reassigning = false
			return m, nil
		case "enter":
			label, ok := m.reassignPicker.Selected()
			if !ok {
				return m, nil
			}
			if label == projects.NewProjectLabel {
				m.reassigning = false
				m.reassignTyping = true
				m.reassignInput = addform.NewNamed("Reassign Project", "Press <enter> to assign, <esc> to cancel", "New project name...").Focus()
				return m, m.reassignInput.Init()
			}
			m.reassigning = false
			task, ok := m.list.Selected()
			if !ok {
				return m, nil
			}
			project := label
			if label == projects.NoneLabel {
				project = ""
			}
			return m, setProjectTask(m.projecter, task, project)
		}
	}

	var cmd tea.Cmd
	m.reassignPicker, cmd = m.reassignPicker.Update(msg)
	return m, cmd
}

// updateReassignTyping handles messages while the "key in a new project
// name" input (reached via NewProjectLabel in updateReassigning) is
// focused. Leading/trailing whitespace is trimmed before use; an empty
// name is a silent no-op back to the Tasks panel.
func (m model) updateReassignTyping(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.reassignTyping = false
			m.reassignInput = m.reassignInput.Blur()
			return m, nil
		case "enter":
			project := strings.TrimSpace(m.reassignInput.Value())
			m.reassignTyping = false
			m.reassignInput = m.reassignInput.Blur()
			if project == "" {
				return m, nil
			}
			task, ok := m.list.Selected()
			if !ok {
				return m, nil
			}
			return m, setProjectTask(m.projecter, task, project)
		}
	}

	var cmd tea.Cmd
	m.reassignInput, cmd = m.reassignInput.Update(msg)
	return m, cmd
}

// startSearch begins a "/" search prompt targeting scope (the Tasks panel,
// Projects panel, or "p" quick project-filter popup — see searchScope),
// focusing a fresh textinput.Model with placeholder as its hint text. The
// caller is responsible for returning the resulting tea.Cmd so the input
// actually receives focus.
func (m model) startSearch(placeholder string, scope searchScope) (model, tea.Cmd) {
	ti := textinput.New()
	ti.Prompt = "/"
	ti.Placeholder = placeholder
	ti.CharLimit = 256
	cmd := ti.Focus()
	m.searchInput = ti
	m.searching = true
	m.searchScope = scope
	return m, cmd
}

// updateSearching handles messages while the "/" search prompt (opened from
// the Tasks panel) is focused. Per requirements.md Phase 1, the list is
// never live-filtered while typing; "enter" is what commits the query,
// jumping to the first match and leaving the query active so "n"/"N" keep
// navigating afterward. An empty query is a silent no-op (no search is
// started); a committed query with no match is also a silent no-op — the
// cursor simply stays put, with no popup interrupting the flow. "esc"
// cancels the prompt and also clears any active search query (committed or
// still being typed), so match highlighting disappears immediately,
// matching lazygit. The prompt itself renders inline at the very bottom of
// the screen (see baseView), temporarily replacing the status bar's key
// hints while typing, lazygit-style. Which underlying list "enter"/"esc"
// apply to is determined by m.searchScope, set when the prompt was opened
// (see startSearch), so the same prompt/input state can be reused for the
// Tasks panel, the Projects panel, and the "p" quick project-filter popup.
func (m model) updateSearching(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.searching = false
			m.searchInput.Blur()
			m.clearSearch()
			return m, nil
		case "enter":
			query := strings.TrimSpace(m.searchInput.Value())
			m.searching = false
			m.searchInput.Blur()
			if query == "" {
				return m, nil
			}
			m.applySearch(query)
			if m.searchScope == searchScopeProjects {
				return m.afterProjectsCursorMove()
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

// afterProjectsCursorMove re-applies the Projects panel's
// filter-follows-cursor behavior (see applyProjectSelection) after a
// cursor jump that isn't routed through projects.Model.Update itself —
// namely a "/" search commit or an "n"/"N" match jump — so the shared
// task/tag filter stays in sync with the newly selected entry exactly as
// it would after a plain up/down keypress.
func (m model) afterProjectsCursorMove() (model, tea.Cmd) {
	if label, ok := m.projects.Selected(); ok {
		if newM, cmd, changed := m.applyProjectSelection(label); changed {
			return newM, cmd
		}
	}
	return m, nil
}

// applySearch commits query as the active search against whichever list
// m.searchScope points at.
func (m *model) applySearch(query string) {
	switch m.searchScope {
	case searchScopeProjects:
		m.projects, _ = m.projects.Search(query)
	case searchScopeProjectPicker:
		m.projectPicker, _ = m.projectPicker.Search(query)
	default:
		m.list, _ = m.list.Search(query)
	}
}

// clearSearch discards the active search query on whichever list
// m.searchScope points at.
func (m *model) clearSearch() {
	switch m.searchScope {
	case searchScopeProjects:
		m.projects = m.projects.ClearSearch()
	case searchScopeProjectPicker:
		m.projectPicker = m.projectPicker.ClearSearch()
	default:
		m.list = m.list.ClearSearch()
	}
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
// of being replaced by a blank screen. While a "/" search is being typed,
// the bottom line temporarily shows the search prompt in place of the
// status bar's key hints (lazygit's file-search UX), restoring the hints
// once the search is committed or cancelled.
func (m model) baseView() string {
	view := m.renderGrid()
	if m.searching {
		view += "\n" + m.searchInput.View() + "\n"
	} else {
		view += "\n" + statusbar.Render(m.statusBindings()) + "\n"
	}
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
			view = popup.Overlay(view, popup.DangerConfirmBox(text, width), width, height)
		}
	}

	if m.purging {
		if task, ok := m.list.Selected(); ok {
			text := fmt.Sprintf("Permanently delete %q? This cannot be undone.", task.Description)
			view = popup.Overlay(view, popup.DangerConfirmBox(text, width), width, height)
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
			view = popup.Overlay(view, popup.SuccessConfirmBox(text, width), width, height)
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

	if m.datePicking {
		view = popup.Overlay(view, m.datePick.View(), width, height)
	}

	if m.pickingProject {
		view = popup.Overlay(view, m.projectPicker.View(), width, height)
	}

	if m.reassigning {
		view = popup.Overlay(view, m.reassignPicker.View(), width, height)
	}

	if m.reassignTyping {
		view = popup.Overlay(view, m.reassignInput.View(), width, height)
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

// detailsPanelContent returns the full-detail, read-only, multi-line view
// shown in the Details panel for whatever task is currently selected in
// the Tasks list. It updates live as the Tasks cursor moves, since it
// always reads the list's current selection (same approach as
// statusPanelContent).
func (m model) detailsPanelContent() string {
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
	priority := task.Priority
	if priority == "" {
		priority = "(none)"
	}
	due := task.Due
	if due == "" {
		due = "(none)"
	} else {
		due = formatDetailsDate(due)
	}

	lines := []string{
		fmt.Sprintf("ID:          %d", task.ID),
		fmt.Sprintf("UUID:        %s", task.UUID),
		fmt.Sprintf("Description: %s", task.Description),
	}
	if len(task.Annotations) > 0 {
		lines = append(lines, "Annotations:")
		for _, a := range task.Annotations {
			lines = append(lines, renderAnnotationLines(a)...)
		}
	}
	lines = append(lines,
		fmt.Sprintf("Project:     %s", project),
		fmt.Sprintf("Tags:        %s", tags),
		fmt.Sprintf("Priority:    %s", priority),
		fmt.Sprintf("Urgency:     %.2f", task.Urgency),
		fmt.Sprintf("Urg.Offset:  %.4f", task.UrgencyOffset),
		fmt.Sprintf("Status:      %s", task.Status),
		fmt.Sprintf("Due:         %s", due),
		fmt.Sprintf("Entry:       %s", formatDetailsDate(task.Entry)),
		fmt.Sprintf("Modified:    %s", formatDetailsDate(task.Modified)),
	)
	if task.End != "" {
		lines = append(lines, fmt.Sprintf("End:         %s", formatDetailsDate(task.End)))
	}
	return strings.Join(lines, "\n")
}

// formatDetailsDate reformats a Taskwarrior timestamp (e.g. "20250101T000000Z",
// per taskDueLayout) as "YYYY-MM-DD" for display in the Details panel. If the
// value doesn't parse as a Taskwarrior timestamp, it's returned unchanged
// rather than dropped, so unexpected formats still show something useful.
func formatDetailsDate(value string) string {
	t, err := time.Parse(taskDueLayout, value)
	if err != nil {
		return value
	}
	return t.Format("2006-01-02")
}

// renderAnnotationLines renders one annotation as one or more Details-panel
// lines: "  <first line of Description>", with any subsequent lines of a
// multi-line Description (possible via `task annotate` with embedded
// newlines, or the $EDITOR buffer's escaped-newline round trip) indented to
// align under the first line's text rather than starting back at the left
// margin, so a multi-line annotation still reads as one annotation rather
// than looking like several unrelated ones. The Entry timestamp is
// intentionally omitted; it adds clutter without much value here.
func renderAnnotationLines(a taskwarrior.Annotation) []string {
	const prefix = "  "
	indent := strings.Repeat(" ", len(prefix))

	descLines := strings.Split(a.Description, "\n")
	rendered := make([]string, len(descLines))
	for i, line := range descLines {
		if i == 0 {
			rendered[i] = prefix + line
		} else {
			rendered[i] = indent + line
		}
	}
	return rendered
}

// renderGrid lays out the lazygit-style panel grid: a left column of 3
// stacked rows (Status, Tasks, and a bottom row splitting Projects/Tags
// into side-by-side subcolumns) and one large panel (Details) filling the
// right column. Tasks (slot 2), Projects (slot 3), Tags (slot 4), and
// Details (slot 0) all have real content.
func (m model) renderGrid() string {
	leftWidth, rightWidth, statusHeight, _, _, fullHeight := m.gridDims()

	status := panel.Render(panelTitle(focusStatus), m.statusPanelContent(), leftWidth, statusHeight, m.focus == focusStatus)
	tasksPanel := m.list.View()
	projectsPanel := m.projects.View()
	tagsPanel := m.tags.View()
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, projectsPanel, tagsPanel)

	leftCol := lipgloss.JoinVertical(lipgloss.Left, status, tasksPanel, bottomRow)

	details := panel.Render(panelTitle(focusDetails), m.detailsPanelContent(), rightWidth, fullHeight, m.focus == focusDetails)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftCol, details)
}

func main() {
	if isVersionArg(os.Args[1:]) {
		fmt.Println(versionString())
		return
	}

	if isUpdateArg(os.Args[1:]) {
		if err := runUpdate(); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating lazytask: %v\n", err)
			os.Exit(1)
		}
		return
	}

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
