package version

import (
	"fmt"
	"runtime"
)

// Build information. Populated at build-time via ldflags.
var (
	Version   = "dev"           // Version is the semantic version (e.g., "1.2.3")
	GitCommit = "unknown"       // GitCommit is the git commit hash
	BuildDate = "unknown"       // BuildDate is the build timestamp
	GoVersion = runtime.Version() // GoVersion is the Go version used to build
)

// Info returns version information as a struct
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
}

// Get returns the version information
func Get() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
		GoVersion: GoVersion,
	}
}

// String returns a human-readable version string
func String() string {
	if Version == "dev" {
		return fmt.Sprintf("aviary %s (commit %s, built %s with %s)", 
			Version, GitCommit[:min(7, len(GitCommit))], BuildDate, GoVersion)
	}
	return fmt.Sprintf("aviary v%s (commit %s, built %s with %s)", 
		Version, GitCommit[:min(7, len(GitCommit))], BuildDate, GoVersion)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
