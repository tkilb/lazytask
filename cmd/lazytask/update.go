package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/tkilb/lazytask/internal/version"
)

const updateRepo = "tkilb/lazytask"

// isUpdateArg reports whether the CLI was invoked to self-update
// (`lazytask update`) rather than start the TUI.
func isUpdateArg(args []string) bool {
	return len(args) > 0 && args[0] == "update"
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

// runUpdate checks GitHub Releases for a newer lazytask build than the
// one currently running and, if found, downloads and replaces the
// running binary in place. Linux and macOS only — see requirements.md
// Phase 4; Windows was never, and will never be, supported here.
func runUpdate() error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("lazytask does not support Windows, and it never will; grab a real terminal on Linux or macOS")
	}

	client := &http.Client{Timeout: 30 * time.Second}

	latestTag, err := latestReleaseTag(client)
	if err != nil {
		return fmt.Errorf("checking latest release: %w", err)
	}
	latestVersion := strings.TrimPrefix(latestTag, "v")

	if version.Version != "dev" {
		cmp, err := compareVersions(version.Version, latestVersion)
		if err != nil {
			return fmt.Errorf("comparing versions: %w", err)
		}
		if cmp >= 0 {
			fmt.Printf("lazytask %s is already up to date.\n", version.Version)
			return nil
		}
	}

	fmt.Printf("Updating to %s...\n", latestVersion)

	wantPath := fmt.Sprintf("lazytask_%s_%s_%s/lazytask", latestVersion, runtime.GOOS, runtime.GOARCH)
	archiveName := fmt.Sprintf("lazytask_%s_%s_%s.tar.gz", latestVersion, runtime.GOOS, runtime.GOARCH)
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", updateRepo, latestTag, archiveName)

	binaryData, err := downloadBinary(client, url, wantPath)
	if err != nil {
		return fmt.Errorf("downloading release: %w", err)
	}

	if err := replaceRunningBinary(binaryData); err != nil {
		return fmt.Errorf("installing update: %w", err)
	}

	fmt.Printf("Updated lazytask to %s.\n", latestVersion)
	return nil
}

func latestReleaseTag(client *http.Client) (string, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", updateRepo), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %s", resp.Status)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", fmt.Errorf("release response missing tag_name")
	}
	return release.TagName, nil
}

// downloadBinary fetches the release tarball at url and returns the
// bytes of the entry at wantPath within it.
func downloadBinary(client *http.Client, url, wantPath string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s fetching %s", resp.Status, url)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Name == wantPath {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("archive did not contain %s", wantPath)
}

// replaceRunningBinary writes newBinary to a temp file alongside the
// currently running executable and atomically renames it into place,
// preserving execute permissions. Renaming over a running binary is
// safe on Linux/macOS: the old inode stays valid for this process
// while any new invocation picks up the replacement.
func replaceRunningBinary(newBinary []byte) error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return err
	}

	dir := filepath.Dir(execPath)
	tmp, err := os.CreateTemp(dir, ".lazytask-update-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once successfully renamed away

	if _, err := tmp.Write(newBinary); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return err
	}
	return os.Rename(tmpPath, execPath)
}

// compareVersions compares two "MAJOR.MINOR.PATCH" strings, returning
// -1, 0, or 1 as a is less than, equal to, or greater than b.
func compareVersions(a, b string) (int, error) {
	pa, err := parseVersion(a)
	if err != nil {
		return 0, err
	}
	pb, err := parseVersion(b)
	if err != nil {
		return 0, err
	}
	for i := range pa {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1, nil
			}
			return 1, nil
		}
	}
	return 0, nil
}

func parseVersion(v string) ([3]int, error) {
	var out [3]int
	parts := strings.SplitN(strings.TrimPrefix(v, "v"), ".", 3)
	if len(parts) != 3 {
		return out, fmt.Errorf("%q is not a MAJOR.MINOR.PATCH version", v)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, fmt.Errorf("%q is not a MAJOR.MINOR.PATCH version", v)
		}
		out[i] = n
	}
	return out, nil
}
