package taskwarrior

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// TaskMutator provides write access to Taskwarrior tasks.
type TaskMutator interface {
	Add(ctx context.Context, description string, extraArgs ...string) (int, error)
	Done(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	Import(ctx context.Context, data []byte) error
}

// createdTaskRE matches Taskwarrior's "Created task <id>." confirmation line.
var createdTaskRE = regexp.MustCompile(`Created task (\d+)\.`)

// run executes `task` with the given arguments, using the client's
// environment overrides, and returns combined stdout/stderr.
func (c *Client) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.binary, args...)
	cmd.Env = c.buildEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrMsg := strings.TrimSpace(stderr.String())
		if stderrMsg != "" {
			return "", fmt.Errorf("task %s failed (%w): %s", strings.Join(args, " "), err, stderrMsg)
		}
		return "", fmt.Errorf("task %s failed: %w", strings.Join(args, " "), err)
	}

	return stdout.String(), nil
}

// Add runs `task add <description> [extraArgs...]` and returns the numeric
// ID of the newly created task, as reported by Taskwarrior's confirmation
// output. extraArgs may include attributes such as "project:Home" or
// "priority:H".
func (c *Client) Add(ctx context.Context, description string, extraArgs ...string) (int, error) {
	args := make([]string, 0, len(extraArgs)+3)
	args = append(args, "rc.confirmation=off", "add", description)
	args = append(args, extraArgs...)

	out, err := c.run(ctx, args...)
	if err != nil {
		return 0, err
	}

	match := createdTaskRE.FindStringSubmatch(out)
	if match == nil {
		return 0, fmt.Errorf("could not determine created task id from output: %q", strings.TrimSpace(out))
	}

	id, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, fmt.Errorf("parsing created task id %q: %w", match[1], err)
	}

	return id, nil
}

// Done marks the task identified by id (a Taskwarrior ID or UUID) as completed.
func (c *Client) Done(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("id must not be empty")
	}
	_, err := c.run(ctx, "rc.confirmation=off", id, "done")
	return err
}

// Delete removes the task identified by id (a Taskwarrior ID or UUID).
func (c *Client) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("id must not be empty")
	}
	_, err := c.run(ctx, "rc.confirmation=off", id, "delete")
	return err
}

// Import runs `task import` with data (Taskwarrior export-format JSON,
// either a single task object or an array of them) piped to stdin. A task
// whose "uuid" field matches an existing task updates that task's fields in
// place; this is how edited tasks are written back after a round trip
// through an external editor.
func (c *Client) Import(ctx context.Context, data []byte) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return fmt.Errorf("data must not be empty")
	}

	cmd := exec.CommandContext(ctx, c.binary, "rc.confirmation=off", "import", "-")
	cmd.Env = c.buildEnv()
	cmd.Stdin = bytes.NewReader(data)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrMsg := strings.TrimSpace(stderr.String())
		if stderrMsg != "" {
			return fmt.Errorf("task import failed (%w): %s", err, stderrMsg)
		}
		return fmt.Errorf("task import failed: %w", err)
	}

	return nil
}
