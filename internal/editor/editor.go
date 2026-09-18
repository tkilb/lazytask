// Package editor provides a small helper for round-tripping text through an
// external editor process (the user's $EDITOR, or vi as a fallback) via a
// temporary file, similar in spirit to `task <id> edit`.
package editor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// DefaultEditor is used when no editor is configured via WithEditor and no
// $EDITOR environment variable is present.
const DefaultEditor = "vi"

// Option configures a call to Edit.
type Option func(*config)

type config struct {
	editor string
	env    []string
}

// WithEditor overrides the editor command to launch, bypassing $EDITOR
// resolution entirely. The value may include arguments (e.g. "code --wait"),
// which are split on whitespace.
func WithEditor(editor string) Option {
	return func(c *config) { c.editor = editor }
}

// WithEnviron overrides the environment consulted for $EDITOR. Primarily
// intended for tests; production callers can omit this to use the process
// environment.
func WithEnviron(env []string) Option {
	return func(c *config) { c.env = env }
}

// Edit writes content to a temporary file, launches an editor (resolved via
// resolveEditor) attached to the current process's stdin/stdout/stderr with
// the temp file path as its sole positional argument, waits for it to exit,
// and returns the file's contents afterward.
//
// The temporary file is removed before Edit returns, regardless of outcome.
func Edit(content string, opts ...Option) (string, error) {
	cmd, session, err := Prepare(content, opts...)
	if err != nil {
		return "", err
	}
	defer session.Close()

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("running editor %q: %w", strings.Join(cmd.Args, " "), err)
	}

	return session.Read()
}

// Session represents an in-progress external editor invocation whose temp
// file has been written but whose editor command has not necessarily been
// run yet. It lets callers that must run the *exec.Cmd themselves (for
// example a Bubble Tea program via tea.ExecProcess, which needs to suspend
// its own terminal handling around the process) read back the result and
// clean up afterward.
type Session struct {
	path string
}

// Read returns the current contents of the session's temp file. It is
// intended to be called after the *exec.Cmd returned alongside this Session
// by Prepare has finished running.
func (s *Session) Read() (string, error) {
	out, err := os.ReadFile(s.path)
	if err != nil {
		return "", fmt.Errorf("reading temp file: %w", err)
	}
	return string(out), nil
}

// Close removes the session's temp file. Safe to call even if the file was
// already removed.
func (s *Session) Close() error {
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing temp file: %w", err)
	}
	return nil
}

// Prepare writes content to a fresh temporary file and returns an *exec.Cmd
// configured to open it in the resolved editor (see resolveEditor), along
// with a Session for reading back the result once that command has been
// run to completion. The caller owns running the returned *exec.Cmd and
// must call Session.Close afterward to remove the temp file; Edit does all
// of this itself for simpler, non-interactive callers.
func Prepare(content string, opts ...Option) (*exec.Cmd, *Session, error) {
	cfg := config{env: os.Environ()}
	for _, opt := range opts {
		opt(&cfg)
	}

	editorCmd, err := resolveEditor(cfg)
	if err != nil {
		return nil, nil, err
	}

	f, err := os.CreateTemp("", "lazytask-edit-*.txt")
	if err != nil {
		return nil, nil, fmt.Errorf("creating temp file: %w", err)
	}
	path := f.Name()

	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(path)
		return nil, nil, fmt.Errorf("writing temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return nil, nil, fmt.Errorf("closing temp file: %w", err)
	}

	args := append(append([]string{}, editorCmd[1:]...), path)
	cmd := exec.Command(editorCmd[0], args...)

	return cmd, &Session{path: path}, nil
}

// resolveEditor determines the editor command (and its arguments) to run,
// preferring an explicit WithEditor option, then $EDITOR from cfg.env, then
// DefaultEditor.
func resolveEditor(cfg config) ([]string, error) {
	editor := cfg.editor
	if editor == "" {
		editor = envValue(cfg.env, "EDITOR")
	}
	if editor == "" {
		editor = DefaultEditor
	}

	fields := strings.Fields(editor)
	if len(fields) == 0 {
		return nil, fmt.Errorf("editor command is empty")
	}
	return fields, nil
}

// envValue looks up key in env (a slice of "KEY=VALUE" strings, as returned
// by os.Environ), returning "" if not present.
func envValue(env []string, key string) string {
	prefix := key + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return strings.TrimPrefix(kv, prefix)
		}
	}
	return ""
}
