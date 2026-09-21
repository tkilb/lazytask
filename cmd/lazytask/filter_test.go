package main

import (
	"testing"

	"github.com/tkilb/lazytask/internal/ui/projects"
)

func TestFilterState_TaskFilter(t *testing.T) {
	none := ""
	work := "work"

	tests := []struct {
		name string
		f    filterState
		want string
	}{
		{name: "no filter (all)", f: filterState{project: nil}, want: ""},
		{name: "no project (none)", f: filterState{project: &none}, want: "project:"},
		{name: "specific project", f: filterState{project: &work}, want: "project:work"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f.taskFilter(); got != tt.want {
				t.Errorf("taskFilter() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterState_WithProjectSelection(t *testing.T) {
	f := filterState{}

	f = f.withProjectSelection("work")
	if got := f.taskFilter(); got != "project:work" {
		t.Fatalf("after selecting 'work', taskFilter() = %q", got)
	}

	f = f.withProjectSelection(projects.NoneLabel)
	if got := f.taskFilter(); got != "project:" {
		t.Fatalf("after selecting %s, taskFilter() = %q", projects.NoneLabel, got)
	}

	f = f.withProjectSelection(projects.AllLabel)
	if got := f.taskFilter(); got != "" {
		t.Fatalf("after selecting %s, taskFilter() = %q, want empty", projects.AllLabel, got)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.equal(tt.b); got != tt.want {
				t.Errorf("equal() = %v, want %v", got, tt.want)
			}
		})
	}
}
