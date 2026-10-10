package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/tkilb/lazytask/internal/config"
	"github.com/tkilb/lazytask/internal/taskwarrior"
)

// isSyncResetArg reports whether the CLI was invoked to reset local state
// (`lazytask sync-reset [-y|--yes]`) rather than start the TUI.
func isSyncResetArg(args []string) bool {
	return len(args) > 0 && args[0] == "sync-reset"
}

// runSyncReset handles `lazytask sync-reset [-y|--yes]`.
// It wipes local Taskwarrior SQLite database files and the local git-sync
// clone cache, then triggers a fresh `task sync` using the configured
// encryption secret so state is cleanly pulled from the remote.
func runSyncReset(in io.Reader, out io.Writer, client *taskwarrior.Client, args []string) error {
	autoConfirm := false
	for _, arg := range args {
		switch arg {
		case "-y", "--yes":
			autoConfirm = true
		default:
			return fmt.Errorf("unknown sync-reset flag %q (expected -y or --yes)", arg)
		}
	}

	if !autoConfirm {
		fmt.Fprint(out, "This will delete your local Taskwarrior replica and local sync clone,\nand pull down fresh task state from your remote git repo.\nAre you sure you want to proceed? [y/N]: ")
		reader := bufio.NewReader(in)
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return fmt.Errorf("reading confirmation: %w", err)
		}
		answer := strings.TrimSpace(strings.ToLower(line))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(out, "Aborted.")
			return nil
		}
	}

	cfg, err := config.LoadGitSyncConfig()
	if err != nil {
		return fmt.Errorf("loading git-sync config: %w", err)
	}

	secret, err := config.EnsureSyncSecret(cfg.EncryptionSecretFile)
	if err != nil {
		return fmt.Errorf("ensuring sync secret: %w", err)
	}

	ctx := context.Background()

	// 1. Apply any git sync config to Taskwarrior if configured
	if cfg.LocalPath != "" || cfg.Branch != "" || cfg.Remote != "" {
		gitCfg := taskwarrior.GitSyncConfig{
			LocalPath: cfg.LocalPath,
			Branch:    cfg.Branch,
			Remote:    cfg.Remote,
		}
		if err := client.ApplyGitSyncConfig(ctx, gitCfg); err != nil {
			return fmt.Errorf("applying git-sync config: %w", err)
		}
	}

	// 2. Reset local state (SQLite database and local git sync clone)
	fmt.Fprintln(out, "Resetting local Taskwarrior database and sync clone...")
	if err := client.ResetLocalState(ctx, cfg.LocalPath); err != nil {
		return fmt.Errorf("resetting local state: %w", err)
	}

	// 3. Trigger fresh sync to pull down remote tasks
	fmt.Fprintln(out, "Syncing fresh from remote...")
	if err := client.Sync(ctx, secret); err != nil {
		return fmt.Errorf("syncing from remote: %w", err)
	}

	fmt.Fprintln(out, "Successfully reset local state and synced from remote.")
	return nil
}
