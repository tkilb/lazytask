// Package config persists small pieces of UI state (currently just the
// active project filter) across lazytask sessions, so the app reopens the
// way the user left it. It also loads lazytask's user-editable app config
// (currently just Taskwarrior git-sync settings; see LoadGitSyncConfig).
//
// State is stored under os.TempDir() rather than a dotfile-style location
// (e.g. ~/.config or the home directory) by explicit user request, to avoid
// adding noise to a dotfiles repo. This means the persisted filter may not
// survive a reboot or OS temp-dir cleanup — that tradeoff is intentional.
// The app config below is the exception: it's meant to be hand-edited by
// the user, so it lives at a fixed ~/.config/lazytask path on every OS
// (by explicit user request), rather than following os.UserConfigDir()'s
// per-OS convention (which would put it under "Library/Application
// Support" on macOS).
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// stateDirName and stateFileName make up the on-disk path:
// os.TempDir()/lazytask/state.json.
const (
	stateDirName  = "lazytask"
	stateFileName = "state.json"
)

// appConfigDirName and appConfigFileName make up the on-disk path for the
// user-editable app config (as opposed to the ephemeral UI state above):
// ~/.config/lazytask/config.yaml, on every OS — see appConfigPath.
const (
	appConfigDirName  = "lazytask"
	appConfigFileName = "config.yaml"
)

// GitSyncConfig holds Taskwarrior's git-sync backend settings
// (sync.git.local_path / sync.git.branch / sync.git.remote; see `task`
// 3.5.0+, PR #4111), as read from lazytask's own YAML config file. Any
// field left empty means "don't configure this key" — see
// taskwarrior.Client.ApplyGitSyncConfig.
//
// Deliberately excluded: sync.git.encryption_secret. Unlike the other
// three keys, the secret should never be persisted into Taskwarrior's own
// .taskrc, and shouldn't need to live in this hand-edited, potentially
// dotfile-managed file at all — see EncryptionSecretFile and
// EnsureSyncSecret.
type GitSyncConfig struct {
	LocalPath string `yaml:"local_path"`
	Branch    string `yaml:"branch"`
	Remote    string `yaml:"remote"`

	// EncryptionSecretFile optionally overrides where the auto-managed
	// encryption secret lives on disk (see EnsureSyncSecret). It's a path,
	// not the secret itself, so it's safe to keep in this dotfile-managed
	// config file — e.g. pointing at a location your dotfile manager
	// deliberately excludes. Supports a leading "~/" for the home
	// directory. Empty means "use the default path".
	EncryptionSecretFile string `yaml:"encryption_secret_file"`
}

// appConfig mirrors the on-disk YAML shape:
//
//	sync:
//	  git:
//	    local_path: /path/to/local/clone
//	    branch: main
//	    remote: git@github.com:me/tasks-sync.git
//	    encryption_secret_file: ~/.secrets/lazytask-sync-secret
type appConfig struct {
	Sync struct {
		Git GitSyncConfig `yaml:"git"`
	} `yaml:"sync"`
}

// State is the set of UI state persisted between sessions.
//
// Project mirrors filterState.project's tri-state semantics: nil means no
// project filter is applied, a pointer to "" means "no project" (only
// projectless tasks), and any other pointer value is a specific project
// name.
//
// Tag mirrors filterState.tag: nil means no tag filter is applied, and any
// other pointer value is a specific tag name.
type State struct {
	Project *string `json:"project,omitempty"`
	Tag     *string `json:"tag,omitempty"`
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

// appConfigPath returns ~/.config/lazytask/config.yaml, using
// os.UserHomeDir() (which honors $HOME) rather than os.UserConfigDir(),
// so this fixed layout applies uniformly across macOS/Linux/etc. — see
// the package doc comment for why.
func appConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home dir: %w", err)
	}
	return filepath.Join(home, ".config", appConfigDirName, appConfigFileName), nil
}

// LoadGitSyncConfig reads sync.git.* settings from lazytask's YAML app
// config file (~/.config/lazytask/config.yaml). A missing file is not an
// error — it returns a zero-value GitSyncConfig, meaning "git-sync is not
// configured" — but a malformed file does return an error, since a broken
// config is more likely a typo the user should know about than a normal
// "not set up yet" state.
func LoadGitSyncConfig() (GitSyncConfig, error) {
	path, err := appConfigPath()
	if err != nil {
		return GitSyncConfig{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return GitSyncConfig{}, nil
		}
		return GitSyncConfig{}, fmt.Errorf("reading %s: %w", path, err)
	}

	var cfg appConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return GitSyncConfig{}, fmt.Errorf("parsing %s: %w", path, err)
	}

	// Expand a leading "~/" ourselves rather than relying on Taskwarrior's
	// own expansion of sync.git.local_path: on at least some platforms
	// (observed on macOS) it resolves "~/foo" to "/home/<empty>/foo"
	// instead of the real home directory, sending git operations into a
	// bogus path and producing a cryptic "Operation not supported" error
	// instead of the intended clone location.
	if cfg.Sync.Git.LocalPath != "" {
		expanded, err := expandHome(cfg.Sync.Git.LocalPath)
		if err != nil {
			return GitSyncConfig{}, err
		}
		cfg.Sync.Git.LocalPath = expanded
	}

	return cfg.Sync.Git, nil
}

// defaultSyncSecretRelPath is where the auto-managed git-sync encryption
// secret lives when GitSyncConfig.EncryptionSecretFile isn't set:
// ~/.local/share/lazytask/sync-secret. Deliberately outside ~/.config, so
// a dotfile manager tracking ~/.config/lazytask/config.yaml won't also
// pick up the secret file sitting next to it.
const defaultSyncSecretRelPath = ".local/share/lazytask/sync-secret"

// EnsureSyncSecret returns the git-sync encryption secret, generating and
// persisting a fresh random one (0600, owner-only) the first time it's
// needed. path overrides where it's stored (see
// GitSyncConfig.EncryptionSecretFile, including "~/" expansion); an empty
// path falls back to defaultSyncSecretRelPath under the user's home
// directory. This is the only place lazytask ever handles the secret's
// value — it's never written to Taskwarrior's own .taskrc (see
// taskwarrior.Client.Sync), only ever passed as a one-off override at
// sync time.
func EnsureSyncSecret(path string) (string, error) {
	resolved, err := resolveSyncSecretPath(path)
	if err != nil {
		return "", err
	}

	if data, err := os.ReadFile(resolved); err == nil {
		if secret := strings.TrimSpace(string(data)); secret != "" {
			return secret, nil
		}
		// Empty file: fall through and regenerate rather than syncing
		// with a blank secret.
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("reading sync secret file %s: %w", resolved, err)
	}

	secret, err := generateSyncSecret()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(resolved), 0o700); err != nil {
		return "", fmt.Errorf("creating sync secret dir: %w", err)
	}
	if err := os.WriteFile(resolved, []byte(secret+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("writing sync secret file %s: %w", resolved, err)
	}
	return secret, nil
}

// RotateSyncSecret generates a brand-new secret and overwrites whatever is
// currently stored at path (see EnsureSyncSecret for path resolution),
// unconditionally — unlike EnsureSyncSecret, it does not reuse an existing
// secret. Callers must warn users that rotating invalidates the ability to
// decrypt any history already pushed to the sync remote under the old
// secret, and that every other device sharing this sync repo needs to be
// updated with the new secret too.
func RotateSyncSecret(path string) (string, error) {
	resolved, err := resolveSyncSecretPath(path)
	if err != nil {
		return "", err
	}

	secret, err := generateSyncSecret()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(resolved), 0o700); err != nil {
		return "", fmt.Errorf("creating sync secret dir: %w", err)
	}
	if err := os.WriteFile(resolved, []byte(secret+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("writing sync secret file %s: %w", resolved, err)
	}
	return secret, nil
}

// SyncSecretPath returns the resolved on-disk path of the git-sync
// encryption secret for the given GitSyncConfig.EncryptionSecretFile
// override (empty meaning "use the default path"), without reading or
// creating it. Useful for CLI output that shouldn't print the secret
// itself, only where it lives.
func SyncSecretPath(path string) (string, error) {
	return resolveSyncSecretPath(path)
}

// resolveSyncSecretPath expands a leading "~/" and falls back to
// defaultSyncSecretRelPath under the home directory when path is empty.
func resolveSyncSecretPath(path string) (string, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("determining home dir: %w", err)
		}
		return filepath.Join(home, defaultSyncSecretRelPath), nil
	}
	return expandHome(path)
}

// expandHome expands a leading "~/" to the user's home directory (the
// only shorthand supported, matching how the rest of lazytask's paths are
// plain strings, not full shell expansion). Paths without that prefix are
// returned unchanged.
func expandHome(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home dir: %w", err)
	}
	return filepath.Join(home, path[len("~/"):]), nil
}

// generateSyncSecret returns a fresh 256-bit secret, hex-encoded.
func generateSyncSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating sync secret: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
