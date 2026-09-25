package main

import "testing"

func TestIsUpdateArg(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"no args", nil, false},
		{"update", []string{"update"}, true},
		{"update with extra args", []string{"update", "--force"}, true},
		{"version", []string{"version"}, false},
		{"other", []string{"foo"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isUpdateArg(c.args); got != c.want {
				t.Errorf("isUpdateArg(%v) = %v, want %v", c.args, got, c.want)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in      string
		want    [3]int
		wantErr bool
	}{
		{"0.0.1", [3]int{0, 0, 1}, false},
		{"v1.2.3", [3]int{1, 2, 3}, false},
		{"10.20.30", [3]int{10, 20, 30}, false},
		{"dev", [3]int{}, true},
		{"1.2", [3]int{}, true},
		{"1.2.3-rc1", [3]int{}, true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := parseVersion(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("parseVersion(%q) expected error, got %v", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseVersion(%q) unexpected error: %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("parseVersion(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.0.1", "0.0.1", 0},
		{"0.0.1", "0.0.2", -1},
		{"0.0.2", "0.0.1", 1},
		{"0.1.0", "0.0.9", 1},
		{"1.0.0", "0.9.9", 1},
		{"v0.1.0", "0.1.0", 0},
	}
	for _, c := range cases {
		t.Run(c.a+"_vs_"+c.b, func(t *testing.T) {
			got, err := compareVersions(c.a, c.b)
			if err != nil {
				t.Fatalf("compareVersions(%q, %q) unexpected error: %v", c.a, c.b, err)
			}
			if got != c.want {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
			}
		})
	}
}
