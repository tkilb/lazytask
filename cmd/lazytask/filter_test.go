package main

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tkilb/lazytask/internal/ui/projects"
	"github.com/tkilb/lazytask/internal/ui/tags"
	"github.com/tkilb/lazytask/internal/ui/tasklist"
)

func TestFilterState_TaskFilter(t *testing.T) {
	none := ""
	work := "work"

	tests := []struct {
		name string
		f    filterState
		want []string
	}{
		{name: "no filter (all)", f: filterState{project: nil}, want: nil},
		{name: "no project (none)", f: filterState{project: &none}, want: []string{"project:"}},
		{name: "specific project", f: filterState{project: &work}, want: []string{"project:work"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f.taskFilterArgs(); !slices.Equal(got, tt.want) {
				t.Errorf("taskFilterArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterState_WithProjectSelection(t *testing.T) {
	f := filterState{}

	f = f.withProjectSelection("work")
	if got := f.taskFilterArgs(); !slices.Equal(got, []string{"project:work"}) {
		t.Fatalf("after selecting 'work', taskFilterArgs() = %q", got)
	}

	f = f.withProjectSelection(projects.NoneLabel)
	if got := f.taskFilterArgs(); !slices.Equal(got, []string{"project:"}) {
		t.Fatalf("after selecting %s, taskFilterArgs() = %q", projects.NoneLabel, got)
	}

	f = f.withProjectSelection(projects.AllLabel)
	if got := f.taskFilterArgs(); len(got) != 0 {
		t.Fatalf("after selecting %s, taskFilterArgs() = %q, want empty", projects.AllLabel, got)
	}
}

func TestFilterState_Equal(t *testing.T) {
	tests := []struct {
		name string
		a, b filterState
		want bool
	}{
		{name: "both all", a: filterState{}, b: filterState{}, want: true},
		{name: "same project, different pointers", a: filterState{}.withProjectSelection("work"), b: filterState{}.withProjectSelection("work"), want: true},
		{name: "all vs none", a: filterState{}, b: filterState{}.withProjectSelection(projects.NoneLabel), want: false},
		{name: "different projects", a: filterState{}.withProjectSelection("work"), b: filterState{}.withProjectSelection("home"), want: false},
		{name: "same tag, different pointers", a: filterState{}.withTagSelection("urgent"), b: filterState{}.withTagSelection("urgent"), want: true},
		{name: "any vs specific tag", a: filterState{}, b: filterState{}.withTagSelection("urgent"), want: false},
		{name: "different tags", a: filterState{}.withTagSelection("urgent"), b: filterState{}.withTagSelection("later"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.equal(tt.b); got != tt.want {
				t.Errorf("equal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterState_WithTagSelection(t *testing.T) {
	f := filterState{}

	f = f.withTagSelection("urgent")
	if got := f.taskFilterArgs(); !slices.Equal(got, []string{"+urgent"}) {
		t.Fatalf("after selecting 'urgent', taskFilterArgs() = %q", got)
	}
	if got := f.tagLabel(); got != "urgent" {
		t.Fatalf("tagLabel() = %q, want %q", got, "urgent")
	}

	f = f.withTagSelection(tags.AnyLabel)
	if got := f.taskFilterArgs(); len(got) != 0 {
		t.Fatalf("after selecting %s, taskFilterArgs() = %q, want empty", tags.AnyLabel, got)
	}
	if got := f.tagLabel(); got != tags.AnyLabel {
		t.Fatalf("tagLabel() = %q, want %q", got, tags.AnyLabel)
	}
}

// TestFilterState_TaskFilterArgs_CombinesProjectAndTagAsSeparateArgs guards
// the bug where project + tag were joined into one "project:work +urgent"
// argument, which taskwarrior does not parse as two filters.
func TestFilterState_TaskFilterArgs_CombinesProjectAndTagAsSeparateArgs(t *testing.T) {
	f := filterState{}.withProjectSelection("work").withTagSelection("urgent")
	if got, want := f.taskFilterArgs(), []string{"project:work", "+urgent"}; !slices.Equal(got, want) {
		t.Fatalf("taskFilterArgs() = %q, want %q", got, want)
	}
}

func TestModel_TaskFilters_ProjectAndTagAreSeparateExportArgs(t *testing.T) {
	r := &stubReader{}
	m := model{list: tasklist.New(nil), reader: r}
	m.filter = filterState{}.withProjectSelection("My Project").withTagSelection("urgent")

	assert.Equal(t, []string{"status:pending", "project:My Project", "+urgent"}, m.taskFilters())
}

func TestFilterState_WithTaskbookSelection(t *testing.T) {
	f := filterState{}

	f = f.withTaskbookSelection("work")
	wantWork := []string{"(", "taskbook:work", "or", "taskbook.none:", ")"}
	if got := f.taskFilterArgs(); !slices.Equal(got, wantWork) {
		t.Fatalf("after selecting 'work', taskFilterArgs() = %q, want %q", got, wantWork)
	}
	if got := f.taskbookLabel(); got != "work" {
		t.Fatalf("taskbookLabel() = %q, want %q", got, "work")
	}

	f = f.withTaskbookSelection("(all)")
	if got := f.taskFilterArgs(); len(got) != 0 {
		t.Fatalf("after selecting (all), taskFilterArgs() = %q, want empty", got)
	}
	if got := f.taskbookLabel(); got != "(all)" {
		t.Fatalf("taskbookLabel() = %q, want %q", got, "(all)")
	}
}
