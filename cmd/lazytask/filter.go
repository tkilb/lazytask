package main

import "github.com/tkilb/lazytask/internal/ui/projects"

// filterState holds the cross-panel task filter selected via the
// Projects (and, in a later chunk, Tags) panel. It is deliberately kept
// separate from tasklist.Model's status-tab state, since project (and
// eventually tag) filtering is orthogonal to which status tab is active.
//
// project is a tri-state: nil means no project filter is applied
// (projects.AllLabel), a pointer to "" means only tasks with no project at
// all (projects.NoneLabel), and any other pointer value filters to that
// specific project name.
type filterState struct {
	project *string
}

// taskFilter returns the `task export` filter fragment for the currently
// selected project, or "" if no project filter is applied.
func (f filterState) taskFilter() string {
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

// equal reports whether f and other select the same project filter. It
// exists because filterState.project is a *string, so comparing
// filterState values with == compares pointer identity rather than the
// underlying filter, which would spuriously treat every navigation step as
// a change.
func (f filterState) equal(other filterState) bool {
	if f.project == nil || other.project == nil {
		return f.project == other.project
	}
	return *f.project == *other.project
}
