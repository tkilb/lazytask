package taskwarrior

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// TaskReader provides read-only access to Taskwarrior tasks.
type TaskReader interface {
	Export(ctx context.Context, filters ...string) ([]Task, error)
}

// Client wraps the Taskwarrior CLI binary for executing operations.
type Client struct {
	binary   string
	taskData string
	taskRC   string
	environ  []string
}

// ClientOption configures a Client instance.
type ClientOption func(*Client)

// WithBinary overrides the Taskwarrior binary name or path.
func WithBinary(bin string) ClientOption {
	return func(c *Client) {
		c.binary = bin
	}
}

// WithTaskData sets the TASKDATA directory override.
func WithTaskData(dir string) ClientOption {
	return func(c *Client) {
		c.taskData = dir
	}
}

// WithTaskRC sets the TASKRC configuration file override.
func WithTaskRC(rcPath string) ClientOption {
	return func(c *Client) {
		c.taskRC = rcPath
	}
}

// WithEnviron sets an explicit environment for the CLI invocations.
func WithEnviron(env []string) ClientOption {
	return func(c *Client) {
		c.environ = env
	}
}

// NewClient returns a new Client with the default or configured options.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		binary: "task",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// buildEnv creates the process environment, applying TASKDATA and TASKRC overrides.
func (c *Client) buildEnv() []string {
	base := c.environ
	if base == nil {
		base = os.Environ()
	}

	env := make([]string, 0, len(base)+2)
	for _, kv := range base {
		if (c.taskData != "" && strings.HasPrefix(kv, "TASKDATA=")) ||
			(c.taskRC != "" && strings.HasPrefix(kv, "TASKRC=")) {
			continue
		}
		env = append(env, kv)
	}

	if c.taskData != "" {
		env = append(env, "TASKDATA="+c.taskData)
	}
	if c.taskRC != "" {
		env = append(env, "TASKRC="+c.taskRC)
	}

	return env
}

// Export runs `task [filters...] export` and decodes the resulting JSON tasks.
func (c *Client) Export(ctx context.Context, filters ...string) ([]Task, error) {
	args := make([]string, 0, len(filters)+1)
	args = append(args, filters...)
	args = append(args, "export")

	cmd := exec.CommandContext(ctx, c.binary, args...)
	cmd.Env = c.buildEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrMsg := strings.TrimSpace(stderr.String())
		if stderrMsg != "" {
			return nil, fmt.Errorf("task export failed (%w): %s", err, stderrMsg)
		}
		return nil, fmt.Errorf("task export failed: %w", err)
	}

	tasks, err := ParseTasks(stdout.Bytes())
	if err != nil {
		return nil, fmt.Errorf("decoding export output: %w", err)
	}

	return tasks, nil
}
