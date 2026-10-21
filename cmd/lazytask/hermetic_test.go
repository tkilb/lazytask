package main

import (
	"os"
	"strings"
	"testing"
)

// TestMain points $TMPDIR at a throwaway directory for the whole package.
// internal/config persists the project/tag filter under os.TempDir(), and
// many tests run the saveFilter command, so without this a test run would
// overwrite the developer's real saved filter.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "lazytask-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("TMPDIR", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestMain_StateFileIsRedirectedAwayFromRealTempDir(t *testing.T) {
	if !strings.Contains(os.TempDir(), "lazytask-test-") {
		t.Fatalf("os.TempDir() = %q; tests must not share the real state file", os.TempDir())
	}
}
