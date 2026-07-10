// Package version holds build metadata for the slickcode binary. Values are
// overridden at build time via linker flags (see the Makefile and
// .goreleaser.yaml); the defaults below apply to unreleased, locally built
// binaries.
package version

import "fmt"

// Build metadata, injected via -ldflags at build time.
var (
	// Version is the released semantic version, e.g. "1.2.0".
	Version = "dev"

	// Commit is the git commit SHA the binary was built from.
	Commit = "none"

	// Date is the RFC 3339 build timestamp.
	Date = "unknown"
)

// String returns a human-readable summary of the build metadata, suitable
// for display in `slickcode version`.
func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, Date)
}
