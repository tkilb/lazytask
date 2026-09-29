package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tkilb/lazytask/internal/config"
)

func TestIsSyncSecretArg(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"no args", nil, false},
		{"empty slice", []string{}, false},
		{"sync-secret", []string{"sync-secret"}, true},
		{"sync-secret rotate", []string{"sync-secret", "rotate"}, true},
		{"unrelated arg", []string{"version"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isSyncSecretArg(tc.args); got != tc.want {
				t.Errorf("isSyncSecretArg(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

// withIsolatedHome points $HOME at a fresh temp dir so
// config.LoadGitSyncConfig/EnsureSyncSecret/RotateSyncSecret never touch
// the developer's real home directory.
func withIsolatedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func TestRunSyncSecret_CreatesOnFirstUse(t *testing.T) {
	home := withIsolatedHome(t)

	if err := runSyncSecret(nil); err != nil {
		t.Fatalf("runSyncSecret(nil) returned error: %v", err)
	}

	path := filepath.Join(home, ".local", "share", "lazytask", "sync-secret")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected secret file at %s: %v", path, err)
	}
}

func TestRunSyncSecret_RotateChangesSecret(t *testing.T) {
	withIsolatedHome(t)

	before, err := config.EnsureSyncSecret("")
	if err != nil {
		t.Fatalf("EnsureSyncSecret() returned error: %v", err)
	}

	if err := runSyncSecret([]string{"rotate"}); err != nil {
		t.Fatalf("runSyncSecret([rotate]) returned error: %v", err)
	}

	after, err := config.EnsureSyncSecret("")
	if err != nil {
		t.Fatalf("EnsureSyncSecret() returned error: %v", err)
	}
	if before == after {
		t.Fatalf("expected rotate to change the secret, got the same value")
	}
}

func TestRunSyncSecret_UnknownSubcommand(t *testing.T) {
	withIsolatedHome(t)

	if err := runSyncSecret([]string{"bogus"}); err == nil {
		t.Fatal("expected an error for an unknown sync-secret subcommand")
	}
}
