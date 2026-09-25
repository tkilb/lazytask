// Package version holds the lazytask build version. Version defaults to
// "dev" for local `go build`/`go run` and is overridden at release-build
// time via `-ldflags "-X github.com/tkilb/lazytask/internal/version.Version=X.Y.Z"`
// (see the Makefile's `build` target, which reads the VERSION file).
package version

// Version is the lazytask release version, e.g. "0.0.1". Still pre-v1
// (see requirements.md Phase 4): bump the VERSION file's 0.0.X patch
// number for now; move to v1 once the project owner is happy.
var Version = "dev"
