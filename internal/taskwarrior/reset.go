package taskwarrior

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DataLocation returns the resolved path to Taskwarrior's data directory.
// It checks the client's explicit taskData override, then the TASKDATA
// environment variable, and falls back to querying `task _get rc.data.location`
// or `~/.task`.
func (c *Client) DataLocation(ctx context.Context) (string, error) {
	if c.taskData != "" {
		return c.taskData, nil
	}
	for _, kv := range c.environ {
		if strings.HasPrefix(kv, "TASKDATA=") {
			return strings.TrimPrefix(kv, "TASKDATA="), nil
		}
	}
	if v := os.Getenv("TASKDATA"); v != "" {
		return v, nil
	}

	out, err := c.run(ctx, "_get", "rc.data.location")
	if err == nil && strings.TrimSpace(out) != "" {
		return strings.TrimSpace(out), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".task"), nil
}

// ResetLocalState removes local Taskwarrior replica database files
// (taskchampion.sqlite3*) and the local git-sync clone directory, so the
// next sync pulls fresh from the remote without pushing local history.
func (c *Client) ResetLocalState(ctx context.Context, localSyncPath string) error {
	c.preemptBackgroundSync()
	c.mu.Lock()
	defer c.mu.Unlock()

	dataDir, err := c.DataLocation(ctx)
	if err != nil {
		return fmt.Errorf("locating task data directory: %w", err)
	}

	// Remove taskchampion SQLite replica files if the directory exists.
	matches, err := filepath.Glob(filepath.Join(dataDir, "taskchampion.sqlite3*"))
	if err == nil {
		for _, match := range matches {
			if err := os.Remove(match); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing local task db %s: %w", match, err)
			}
		}
	}

	// Remove local git-sync clone directory if provided.
	if localSyncPath != "" {
		if err := os.RemoveAll(localSyncPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing local sync directory %s: %w", localSyncPath, err)
		}
	}

	return nil
}
