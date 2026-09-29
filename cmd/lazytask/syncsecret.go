package main

import (
	"fmt"

	"github.com/tkilb/lazytask/internal/config"
)

// isSyncSecretArg reports whether the CLI was invoked to manage the
// git-sync encryption secret (`lazytask sync-secret [rotate]`) rather than
// start the TUI.
func isSyncSecretArg(args []string) bool {
	return len(args) > 0 && args[0] == "sync-secret"
}

// runSyncSecret handles `lazytask sync-secret` and `lazytask sync-secret
// rotate`. args should be os.Args[2:] (everything after "sync-secret").
//
// Plain `sync-secret` is idempotent: it creates the secret on first use
// (mirroring what happens automatically on the first `S`-triggered sync)
// and reports its path, without ever printing the secret value itself.
// `sync-secret rotate` unconditionally replaces it with a fresh one.
func runSyncSecret(args []string) error {
	cfg, err := config.LoadGitSyncConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if len(args) > 0 && args[0] == "rotate" {
		path, err := config.SyncSecretPath(cfg.EncryptionSecretFile)
		if err != nil {
			return fmt.Errorf("resolving sync secret path: %w", err)
		}
		fmt.Println("Rotating the git-sync encryption secret makes any history already pushed")
		fmt.Println("to the sync remote under the old secret permanently undecryptable, and")
		fmt.Println("every other device sharing that sync repo must be updated with the new")
		fmt.Println("secret before it can sync again.")
		if _, err := config.RotateSyncSecret(cfg.EncryptionSecretFile); err != nil {
			return fmt.Errorf("rotating sync secret: %w", err)
		}
		fmt.Printf("Rotated git-sync encryption secret at %s\n", path)
		return nil
	}

	if len(args) > 0 {
		return fmt.Errorf("unknown sync-secret subcommand %q (expected no argument, or \"rotate\")", args[0])
	}

	if _, err := config.EnsureSyncSecret(cfg.EncryptionSecretFile); err != nil {
		return fmt.Errorf("ensuring sync secret: %w", err)
	}
	path, err := config.SyncSecretPath(cfg.EncryptionSecretFile)
	if err != nil {
		return fmt.Errorf("resolving sync secret path: %w", err)
	}
	fmt.Printf("Git-sync encryption secret ready at %s\n", path)
	return nil
}
