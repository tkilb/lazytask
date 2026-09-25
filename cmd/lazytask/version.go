package main

import (
	"fmt"

	"github.com/tkilb/lazytask/internal/version"
)

// isVersionArg reports whether the CLI was invoked asking to print its
// version (any of: `version`, `--version`, `-v`) rather than start the TUI.
// It only looks at the first argument (args should be os.Args[1:]) since
// lazytask takes no other positional arguments today.
func isVersionArg(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "version", "--version", "-v":
		return true
	default:
		return false
	}
}

// versionString formats the version banner printed for isVersionArg.
func versionString() string {
	return fmt.Sprintf("lazytask %s", version.Version)
}
