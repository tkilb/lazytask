package main

import (
	"github.com/tkilb/lazytask/internal/ui/projects"
	"github.com/tkilb/lazytask/internal/ui/tags"
	"github.com/tkilb/lazytask/internal/ui/taskbooks"
)

// filterState holds the cross-panel task filter selected via the Projects,
// Tags, and Taskbooks selectors. It is deliberately kept separate from
// tasklist.Model's status-tab state, since filtering is orthogonal to
// which status tab is active.
//
// taskbook is a tri-state: nil means no taskbook filter is applied
// (taskbooks.AllLabel), a pointer to "" means only inbox tasks with no taskbook
// (taskbooks.InboxLabel), and any other pointer value filters to that
// specific taskbook name.
//
// project is a tri-state: nil means no project filter is applied
// (projects.AllLabel), a pointer to "" means only tasks with no project at
// all (projects.NoneLabel), and any other pointer value filters to that
// specific project name.
//
// tag is simpler: nil means no tag filter is applied (tags.AnyLabel), any
// other pointer value filters to that specific tag. Unlike project, there
// is no "(none)" equivalent — the requirement only calls for an (any)
// entry plus the real tags.
type filterState struct {
	project  *string
	tag      *string
	taskbook *string
}

// taskbookFilterArgs returns the `task export` filter fragments for the
// currently selected taskbook, including non-scoped tasks (taskbook.none:),
// or nil if no taskbook filter is active.
func (f filterState) taskbookFilterArgs() []string {
	if f.taskbook != nil && *f.taskbook != "" {
		return []string{"(", "taskbook:" + *f.taskbook, "or", "taskbook.none:", ")"}
	}
	return nil
}

// taskFilterArgs returns the `task export` filter fragments for the
// currently selected taskbook, project, and tag. When scoped to a specific
// taskbook, non-scoped tasks (taskbook.none:) are included so they show for all
// books.
func (f filterState) taskFilterArgs() []string {
	var args []string
	args = append(args, f.taskbookFilterArgs()...)
	if f.project != nil {
		args = append(args, "project:"+*f.project)
	}
	if f.tag != nil {
		args = append(args, "+"+*f.tag)
	}
	return args
}

// projectOnlyFilter returns the `task export` filter fragment for just
// f's project component (ignoring any tag filter), or "" if no project
// filter is applied. Used to scope the Tags panel's own data fetch to the
// currently selected project without also narrowing it by whatever tag is
// currently selected — mirroring how fetchProjects/fetchTags both ignore
// the tag/project filter they themselves control, so applying either
// filter doesn't shrink the very list used to change/clear it.
func (f filterState) projectOnlyFilter() string {
	if f.project == nil {
		return ""
	}
	return "project:" + *f.project
}

// withProjectSelection returns a copy of f updated from whatever entry
// label was selected in the Projects panel (a project name, or the
// projects.NoneLabel/projects.AllLabel special entries).
func (f filterState) withProjectSelection(label string) filterState {
	switch label {
	case projects.AllLabel:
		f.project = nil
	case projects.NoneLabel:
		empty := ""
		f.project = &empty
	default:
		project := label
		f.project = &project
	}
	return f
}

// projectLabel returns the Projects-panel entry label (a project name, or
// projects.NoneLabel/projects.AllLabel) corresponding to f's current
// project filter. It is the inverse of withProjectSelection, used to keep
// the Projects panel's visible cursor in sync with the shared filter (e.g.
// after restoring a persisted filter from a previous session).
func (f filterState) projectLabel() string {
	if f.project == nil {
		return projects.AllLabel
	}
	if *f.project == "" {
		return projects.NoneLabel
	}
	return *f.project
}

// withTagSelection returns a copy of f updated from whatever entry label
// was selected in the Tags panel (a tag name, or the tags.AnyLabel special
// entry).
func (f filterState) withTagSelection(label string) filterState {
	if label == tags.AnyLabel {
		f.tag = nil
		return f
	}
	tag := label
	f.tag = &tag
	return f
}

// tagLabel returns the Tags-panel entry label (a tag name, or
// tags.AnyLabel) corresponding to f's current tag filter. It is the
// inverse of withTagSelection, used to keep the Tags panel's visible
// cursor in sync with the shared filter (e.g. after restoring a persisted
// filter from a previous session).
func (f filterState) tagLabel() string {
	if f.tag == nil {
		return tags.AnyLabel
	}
	return *f.tag
}

// withTaskbookSelection returns a copy of f updated from whatever entry
// label was selected (a taskbook name, or taskbooks.AllLabel).
func (f filterState) withTaskbookSelection(label string) filterState {
	if label == taskbooks.AllLabel {
		f.taskbook = nil
		return f
	}
	tb := label
	f.taskbook = &tb
	return f
}

// taskbookLabel returns the taskbook entry label corresponding to f's current
// taskbook filter.
func (f filterState) taskbookLabel() string {
	if f.taskbook == nil || *f.taskbook == "" {
		return taskbooks.AllLabel
	}
	return *f.taskbook
}

// equal reports whether f and other select the same taskbook, project, and tag
// filter.
func (f filterState) equal(other filterState) bool {
	return stringPtrEqual(f.project, other.project) &&
		stringPtrEqual(f.tag, other.tag) &&
		stringPtrEqual(f.taskbook, other.taskbook)
}

// stringPtrEqual reports whether a and b are both nil, or both non-nil and
// point to equal strings.
func stringPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
