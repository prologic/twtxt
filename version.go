package twtxt

import (
	"fmt"
)

var (
	// Version release version
	Version = "0.0.2"

	// Commit will be overwritten automatically by the build system
	Commit = "HEAD"
)

// FullVersion display the full version and build
func FullVersion() string {
	return fmt.Sprintf("%s@%s", Version, Commit)
}
