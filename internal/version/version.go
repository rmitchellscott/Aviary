package version

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
)

// Build information. Populated at build-time via ldflags.
var (
	Version   = "dev"           // Version is the semantic version (e.g., "1.2.3")
	GitCommit = "unknown"       // GitCommit is the git commit hash
	BuildDate = "unknown"       // BuildDate is the build timestamp
	GoVersion = runtime.Version() // GoVersion is the Go version used to build
)

// Regex to match semantic version pattern (X.Y.Z with optional prerelease and build metadata)
var semverRegex = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z\-\.]+)?(?:\+[0-9A-Za-z\-\.]+)?$`)

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
	var versionStr string
	
	// Handle special cases without 'v' prefix
	if Version == "dev" || Version == "main" || Version == "latest" {
		versionStr = Version
	} else if strings.HasPrefix(Version, "v") {
		// Version already has 'v' prefix (from git tags like "v1.6.0")
		versionStr = Version
	} else if semverRegex.MatchString(Version) {
		// It's a semantic version, add 'v' prefix
		versionStr = fmt.Sprintf("v%s", Version)
	} else {
		// It's something else (branch name, commit hash, etc.), use as-is
		versionStr = Version
	}
	
	return fmt.Sprintf("aviary %s (commit %s, built %s with %s)", 
		versionStr, GitCommit[:min(7, len(GitCommit))], BuildDate, GoVersion)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
