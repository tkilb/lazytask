// Package config persists small pieces of UI state (currently just the
// active project filter) across lazytask sessions, so the app reopens the
// way the user left it.
//
// State is stored under os.TempDir() rather than a dotfile-style location
// (e.g. ~/.config or the home directory) by explicit user request, to avoid
// adding noise to a dotfiles repo. This means the persisted filter may not
// survive a reboot or OS temp-dir cleanup — that tradeoff is intentional.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// stateDirName and stateFileName make up the on-disk path:
// os.TempDir()/lazytask/state.json.
const (
	stateDirName  = "lazytask"
	stateFileName = "state.json"
)

// State is the set of UI state persisted between sessions.
//
// Project mirrors filterState.project's tri-state semantics: nil means no
// project filter is applied, a pointer to "" means "no project" (only
// projectless tasks), and any other pointer value is a specific project
// name.
type State struct {
	Project *string `json:"project,omitempty"`
}

// Load reads the persisted state from disk. If the file doesn't exist, or
// can't be read/parsed, Load returns a zero-value State and a nil error —
// a missing or corrupt state file is not a fatal condition, it just means
// starting with no filter applied.
func Load() State {
	p := filepath.Join(os.TempDir(), stateDirName, stateFileName)
	data, err := os.ReadFile(p)
	if err != nil {
		return State{}
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}
	}
	return s
}

// Save writes state to disk, creating the containing directory if needed.
func Save(s State) error {
	dir := filepath.Join(os.TempDir(), stateDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, stateFileName), data, 0o644)
}
