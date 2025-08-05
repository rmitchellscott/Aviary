package rmapi

import (
	"os"
	"os/exec"
	"strings"

	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/database"
	"github.com/rmitchellscott/aviary/internal/logging"
)

// ExecCommand is exec.Command by default, but can be overridden in tests
var ExecCommand = exec.Command

func init() {
	if config.Get("DRY_RUN", "") != "" {
		ExecCommand = func(name string, args ...string) *exec.Cmd {
			cmdStr := name
			if len(args) > 0 {
				cmdStr += " " + strings.Join(args, " ")
			}
			logging.Logf("[DRY RUN] would run: %s", cmdStr)
			return exec.Command("true")
		}
	}
}

// NewCommand creates a new rmapi command with user-specific configuration
// Returns the command and a cleanup function that should be called after execution
func NewCommand(user *database.User, args ...string) (*exec.Cmd, func()) {
	cmd := ExecCommand("rmapi", args...)
	env := os.Environ()
	var tempConfigPath string

	if user != nil {
		// Set user-specific RMAPI_HOST
		if user.RmapiHost != "" {
			env = append(env, "RMAPI_HOST="+user.RmapiHost)
		} else {
			// Remove server-level RMAPI_HOST to use official cloud
			env = filterEnv(env, "RMAPI_HOST")
		}

		// Set user-specific config path
		if cfg, err := GetUserConfigPath(user.ID); err == nil {
			env = append(env, "RMAPI_CONFIG="+cfg)
			tempConfigPath = cfg
		}
	}

	cmd.Env = env

	// Return cleanup function
	cleanup := func() {
		CleanupTempConfigFile(tempConfigPath)
	}

	return cmd, cleanup
}

// NewSimpleCommand creates a new rmapi command without cleanup (for simple operations)
// Deprecated: Use NewCommand instead for proper temp file management
func NewSimpleCommand(user *database.User, args ...string) *exec.Cmd {
	cmd := ExecCommand("rmapi", args...)
	env := os.Environ()

	if user != nil {
		if user.RmapiHost != "" {
			env = append(env, "RMAPI_HOST="+user.RmapiHost)
		} else {
			env = filterEnv(env, "RMAPI_HOST")
		}
		if cfg, err := GetUserConfigPath(user.ID); err == nil {
			env = append(env, "RMAPI_CONFIG="+cfg)
		}
	}

	cmd.Env = env
	return cmd
}

// filterEnv removes environment variables with the given prefix from the slice
func filterEnv(env []string, prefix string) []string {
	var filtered []string
	for _, e := range env {
		if !strings.HasPrefix(e, prefix+"=") {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
