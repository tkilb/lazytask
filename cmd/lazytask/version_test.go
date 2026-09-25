package main

import "testing"

func TestIsVersionArg(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"no args", nil, false},
		{"empty slice", []string{}, false},
		{"version word", []string{"version"}, true},
		{"long flag", []string{"--version"}, true},
		{"short flag", []string{"-v"}, true},
		{"unrelated arg", []string{"--help"}, false},
		{"version not first", []string{"foo", "version"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isVersionArg(tc.args); got != tc.want {
				t.Errorf("isVersionArg(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestVersionString(t *testing.T) {
	got := versionString()
	if got == "" {
		t.Fatal("versionString() returned empty string")
	}
	const want = "lazytask "
	if len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("versionString() = %q, want prefix %q", got, want)
	}
}
