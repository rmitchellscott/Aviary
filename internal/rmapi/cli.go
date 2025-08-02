package rmapi

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/rmitchellscott/aviary/internal/config"
	"golang.org/x/term"
)

// RunPair runs the interactive rmapi pairing process for single-user mode
// This is called by the CLI command `aviary pair`
func RunPair(stdout, stderr io.Writer) error {
	// 1) Check if we're in an interactive terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("no TTY detected; please run `docker run ... aviary pair` in an interactive shell")
	}

	// 2) Show welcome message
	if host := config.Get("RMAPI_HOST", ""); host != "" {
		fmt.Fprintf(stdout, "Welcome to Aviary. Let's pair with %s!\n", host)
	} else {
		fmt.Fprintln(stdout, "Welcome to Aviary. Let's pair with the reMarkable Cloud!")
	}

	// 3) Run rmapi cd command for pairing
	cmd := exec.Command("rmapi", "cd")
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// Set RMAPI_HOST if configured
	if host := config.Get("RMAPI_HOST", ""); host != "" {
		env := os.Environ()
		env = append(env, "RMAPI_HOST="+host)
		cmd.Env = env
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("`rmapi cd` failed: %w", err)
	}

	fmt.Fprintln(stdout, "Pairing successful!")
	return nil
}