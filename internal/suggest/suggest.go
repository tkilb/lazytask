// Package suggest picks the single task the user should look at next.
package suggest

import (
	"sort"

	"github.com/tkilb/lazytask/internal/taskwarrior"
)

// Reason says why a task was suggested, in descending priority order.
type Reason int

const (
	// ReasonNone means there was no pending task to suggest.
	ReasonNone Reason = iota
	// ReasonStarted means the task has Taskwarrior's "start" attribute set.
	ReasonStarted
	// ReasonNext means the task carries the "next" tag.
	ReasonNext
	// ReasonOrphan means the task has no project and needs a home.
	ReasonOrphan
	// ReasonUrgent is the fallback: the highest effective urgency.
	ReasonUrgent
)

// NextTag is the native Taskwarrior tag marking a task as queued up next.
const NextTag = "next"

const statusPending = "pending"

// Suggestion is one ranked pick: an index into the slice passed to Rank,
// and why it was chosen.
type Suggestion struct {
	Index  int
	Reason Reason
}

// Rank returns up to n suggested pending tasks, best first. Priority:
// started, then +next, then orphan (no project), then highest urgency.
// Ties within a rank go to the most recently started task (rank 1) or the
// highest effective urgency (Urgency+UrgencyOffset), with the earliest
// list position winning any remaining tie. Returns an empty slice when n
// <= 0 or no task is pending.
func Rank(tasks []taskwarrior.Task, n int) []Suggestion {
	var picks []Suggestion
	for i, t := range tasks {
		if t.Status == statusPending {
			picks = append(picks, Suggestion{Index: i, Reason: reasonFor(t)})
		}
	}
	sort.SliceStable(picks, func(a, b int) bool {
		return better(tasks[picks[a].Index], picks[a].Reason, tasks[picks[b].Index], picks[b].Reason)
	})
	if n < 0 {
		n = 0
	}
	if len(picks) > n {
		picks = picks[:n]
	}
	return picks
}

// Pick returns the index into tasks of the single best suggestion and why
// (see Rank), or (-1, ReasonNone) when no task is pending.
func Pick(tasks []taskwarrior.Task) (int, Reason) {
	top := Rank(tasks, 1)
	if len(top) == 0 {
		return -1, ReasonNone
	}
	return top[0].Index, top[0].Reason
}

func reasonFor(t taskwarrior.Task) Reason {
	switch {
	case t.Start != "":
		return ReasonStarted
	case hasTag(t.Tags, NextTag):
		return ReasonNext
	case t.Project == "":
		return ReasonOrphan
	default:
		return ReasonUrgent
	}
}

// better reports whether candidate (rank cr) strictly beats current (rank
// xr). Lower Reason values outrank higher ones.
func better(c taskwarrior.Task, cr Reason, x taskwarrior.Task, xr Reason) bool {
	if cr != xr {
		return cr < xr
	}
	if cr == ReasonStarted {
		// Taskwarrior's UTC timestamps sort lexically in time order.
		return c.Start > x.Start
	}
	return c.Urgency+c.UrgencyOffset > x.Urgency+x.UrgencyOffset
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// Label is the short human-readable name for r shown in the Suggested bar.
func (r Reason) Label() string {
	switch r {
	case ReasonStarted:
		return "started"
	case ReasonNext:
		return "next"
	case ReasonOrphan:
		return "no project"
	case ReasonUrgent:
		return "urgent"
	default:
		return ""
	}
}
