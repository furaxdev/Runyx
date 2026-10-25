// Package version holds build-time version info, set via -ldflags.
package version

var (
	// Version is the Runyx release version, e.g. "0.1.0". "dev" for local builds.
	Version = "dev"
	// Commit is the git commit hash this binary was built from.
	Commit = "unknown"
	// BuildDate is the RFC3339 build timestamp.
	BuildDate = "unknown"
)

// String returns a human-readable version string.
func String() string {
	return Version + " (" + Commit + ", " + BuildDate + ")"
}
