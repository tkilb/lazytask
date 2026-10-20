package suggest

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tkilb/lazytask/internal/taskwarrior"
)

func task(desc string, mod func(*taskwarrior.Task)) taskwarrior.Task {
	t := taskwarrior.Task{Description: desc, Status: "pending", Project: "P"}
	if mod != nil {
		mod(&t)
	}
	return t
}

func TestPick(t *testing.T) {
	started := func(ts string) func(*taskwarrior.Task) { return func(t *taskwarrior.Task) { t.Start = ts } }
	next := func(t *taskwarrior.Task) { t.Tags = []string{"x", "next"} }
	orphan := func(t *taskwarrior.Task) { t.Project = "" }
	urg := func(u, off float64) func(*taskwarrior.Task) {
		return func(t *taskwarrior.Task) { t.Urgency, t.UrgencyOffset = u, off }
	}

	tests := []struct {
		name       string
		tasks      []taskwarrior.Task
		wantIdx    int
		wantReason Reason
	}{
		{"empty", nil, -1, ReasonNone},
		{"only non-pending", []taskwarrior.Task{task("a", func(t *taskwarrior.Task) { t.Status = "completed" })}, -1, ReasonNone},
		{"started beats next, orphan, urgent", []taskwarrior.Task{
			task("urgent", urg(99, 0)), task("orphan", orphan), task("next", next), task("started", started("20240101T000000Z")),
		}, 3, ReasonStarted},
		{"most recently started wins", []taskwarrior.Task{
			task("old", started("20240101T000000Z")), task("new", started("20240201T000000Z")),
		}, 1, ReasonStarted},
		{"next beats orphan and urgent", []taskwarrior.Task{
			task("urgent", urg(99, 0)), task("orphan", orphan), task("next", next),
		}, 2, ReasonNext},
		{"orphan beats urgent", []taskwarrior.Task{
			task("urgent", urg(99, 0)), task("orphan", orphan),
		}, 1, ReasonOrphan},
		{"urgency includes offset", []taskwarrior.Task{
			task("a", urg(5, 0)), task("b", urg(3, 4)),
		}, 1, ReasonUrgent},
		{"tie goes to earliest", []taskwarrior.Task{
			task("a", urg(5, 0)), task("b", urg(5, 0)),
		}, 0, ReasonUrgent},
		{"non-pending started task ignored", []taskwarrior.Task{
			task("done", func(t *taskwarrior.Task) { t.Status = "completed"; t.Start = "20240101T000000Z" }), task("a", nil),
		}, 1, ReasonUrgent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, reason := Pick(tt.tasks)
			assert.Equal(t, tt.wantIdx, idx)
			assert.Equal(t, tt.wantReason, reason)
		})
	}
}

func TestReasonLabel(t *testing.T) {
	assert.Equal(t, "started", ReasonStarted.Label())
	assert.Equal(t, "next", ReasonNext.Label())
	assert.Equal(t, "no project", ReasonOrphan.Label())
	assert.Equal(t, "urgent", ReasonUrgent.Label())
	assert.Equal(t, "", ReasonNone.Label())
}

func TestRank(t *testing.T) {
	urg := func(u float64) func(*taskwarrior.Task) { return func(t *taskwarrior.Task) { t.Urgency = u } }
	tasks := []taskwarrior.Task{
		task("urgent-low", urg(1)),
		task("urgent-high", urg(9)),
		task("done", func(t *taskwarrior.Task) { t.Status = "completed" }),
		task("next", func(t *taskwarrior.Task) { t.Tags = []string{"next"} }),
		task("started", func(t *taskwarrior.Task) { t.Start = "20240101T000000Z" }),
		task("orphan", func(t *taskwarrior.Task) { t.Project = "" }),
	}

	got := Rank(tasks, 3)
	assert.Equal(t, []Suggestion{
		{Index: 4, Reason: ReasonStarted},
		{Index: 3, Reason: ReasonNext},
		{Index: 5, Reason: ReasonOrphan},
	}, got)

	all := Rank(tasks, 10)
	assert.Len(t, all, 5, "non-pending tasks are excluded")
	assert.Equal(t, 1, all[3].Index)
	assert.Equal(t, 0, all[4].Index)

	assert.Empty(t, Rank(tasks, 0))
	assert.Empty(t, Rank(tasks, -1))
	assert.Empty(t, Rank(nil, 3))
}
