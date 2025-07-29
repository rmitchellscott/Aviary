package manager

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/database"
)

// ExecCommand is exec.Command by default, but can be overridden in tests.
var ExecCommand = exec.Command

func init() {
	if config.Get("DRY_RUN", "") != "" {
		ExecCommand = func(name string, args ...string) *exec.Cmd {
			cmdStr := name
			if len(args) > 0 {
				cmdStr += " " + strings.Join(args, " ")
			}
			Logf("[dry-run] would run: %s", cmdStr)
			return exec.Command("true")
		}
	}
}

func DefaultRmDir() string {
	d := config.Get("RM_TARGET_DIR", "")
	if d == "" {
		d = "/"
	}
	return d
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

// rmapiCmd builds an exec.Cmd to run rmapi with user-specific configuration
// if provided.
func rmapiCmd(user *database.User, args ...string) *exec.Cmd {
	cmd := ExecCommand("rmapi", args...)
	env := os.Environ()
	if user != nil {
		if user.RmapiHost != "" {
			env = append(env, "RMAPI_HOST="+user.RmapiHost)
		} else {
			// Remove server-level RMAPI_HOST to use official cloud
			env = filterEnv(env, "RMAPI_HOST")
		}
		if cfg, err := GetUserRmapiConfigPath(user.ID); err == nil {
			env = append(env, "RMAPI_CONFIG="+cfg)
		}
	}
	cmd.Env = env
	return cmd
}

func Logf(format string, v ...interface{}) {
	fmt.Printf("[%s] "+format+"\n", append([]interface{}{time.Now().Format(time.RFC3339)}, v...)...)
}

// LogfWithUser logs a message with username prefix in multi-user mode
func LogfWithUser(user *database.User, format string, v ...interface{}) {
	if database.IsMultiUserMode() && user != nil {
		format = "[" + user.Username + "] " + format
	}
	Logf(format, v...)
}

// SanitizePrefix ensures prefix is a simple directory name with no path
// separators, leading slashes, or parent directory components.
func SanitizePrefix(p string) (string, error) {
	if p == "" {
		return "", nil
	}
	if filepath.IsAbs(p) || strings.Contains(p, "..") || strings.ContainsAny(p, "/\\") {
		return "", fmt.Errorf("invalid prefix %q", p)
	}
	return p, nil
}

func RenameLocalNoYear(src, prefix string) (string, error) {
	dir := filepath.Dir(src)
	today := time.Now()
	month, day := today.Format("January"), today.Day()

	name := fmt.Sprintf("%s %s %d.pdf", prefix, month, day)
	if prefix == "" {
		name = fmt.Sprintf("%s %d.pdf", month, day)
	}
	dest := filepath.Join(dir, name)
	if err := moveFile(src, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func AppendYearLocal(noYearPath string) (string, error) {
	dir := filepath.Dir(noYearPath)
	today := time.Now()
	year := today.Year()

	base := strings.TrimSuffix(filepath.Base(noYearPath), ".pdf")
	name := fmt.Sprintf("%s %d.pdf", base, year)
	dest := filepath.Join(dir, name)
	if err := moveFile(noYearPath, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func moveFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	// ensure data flushed
	if err := out.Sync(); err != nil {
		return err
	}
	// remove src
	return os.Remove(src)
}

// SimpleUpload calls `rmapi put` and returns the uploaded filename or a detailed error.
func SimpleUpload(path, rmDir string, user *database.User) (string, error) {
	args := []string{"put"}

	// Use user's coverpage setting if provided, otherwise fall back to environment variable or default
	coverpageSetting := ""
	if user != nil {
		coverpageSetting = user.CoverpageSetting
	}
	if coverpageSetting == "" {
		if config.Get("RMAPI_COVERPAGE", "") == "first" {
			coverpageSetting = "first"
		} else {
			coverpageSetting = "current"
		}
	}

	if coverpageSetting == "first" {
		args = append(args, "--coverpage=1")
	}

	// Use user's conflict resolution setting if provided, otherwise fall back to environment variable or default
	conflictResolution := ""
	if user != nil {
		conflictResolution = user.ConflictResolution
	}
	if conflictResolution == "" {
		conflictResolution = config.Get("RMAPI_CONFLICT_RESOLUTION", "abort")
	}

	// Determine effective conflict resolution based on file type
	// content_only only works with PDF files, so fallback to abort for other file types
	effectiveConflictResolution := conflictResolution
	if conflictResolution == "content_only" {
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".pdf" {
			effectiveConflictResolution = "abort"
		}
	}

	switch effectiveConflictResolution {
	case "overwrite":
		args = append(args, "--force")
	case "content_only":
		args = append(args, "--content-only") // Only reached for PDFs
	}

	args = append(args, path, rmDir)
	cmd := rmapiCmd(user, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		raw := strings.TrimSpace(string(out))
		// find the first "Error: " in the output
		if idx := strings.Index(raw, "Error:"); idx != -1 {
			// return just "Error: entry already exists" (or whatever follows)
			return "", fmt.Errorf(raw[idx:])
		}
		// fallback if we didn't find it
		return "", fmt.Errorf("rmapi put failed: %s", raw)
	}
	remoteName := filepath.Base(path)
	return remoteName, nil
}

func RenameLocal(path, prefix string) (string, error) {
	dir := filepath.Dir(path)
	today := time.Now()
	month, day, year := today.Format("January"), today.Day(), today.Year()

	// 1) rename to no‐year
	noYear := fmt.Sprintf("%s %s %d.pdf", prefix, month, day)
	if prefix == "" {
		noYear = fmt.Sprintf("%s %d.pdf", month, day)
	}
	noYearPath := filepath.Join(dir, noYear)
	if err := moveFile(path, noYearPath); err != nil {
		return "", err
	}

	// 2) rename to include year
	withYear := strings.TrimSuffix(noYear, ".pdf") + fmt.Sprintf(" %d.pdf", year)
	withYearPath := filepath.Join(dir, withYear)
	if err := moveFile(noYearPath, withYearPath); err != nil {
		return "", err
	}

	return withYearPath, nil
}

// RenameAndUpload renames locally, uploads via rmapi, and returns the name on the device.
func RenameAndUpload(path, prefix, rmDir string, user *database.User) (string, error) {
	// Validate prefix early
	var err error
	prefix, err = SanitizePrefix(prefix)
	if err != nil {
		return "", err
	}

	// Build target dir - use user-specific directory in multi-user mode
	var pdfDir string
	if database.IsMultiUserMode() && user != nil {
		pdfDir, err = GetUserPDFDir(user.ID, prefix)
		if err != nil {
			return "", err
		}
	} else {
		// Single-user mode - use existing logic
		pdfDir = config.Get("PDF_DIR", "")
		if pdfDir == "" {
			pdfDir = "/app/pdfs"
		}
		if prefix != "" {
			pdfDir = filepath.Join(pdfDir, prefix)
			if err := os.MkdirAll(pdfDir, 0755); err != nil {
				return "", err
			}
		}
	}

	// Date strings
	today := time.Now()
	month, day, year := today.Format("January"), today.Day(), today.Year()

	// Move into target as "<prefix> Month D.pdf"
	noYear := fmt.Sprintf("%s %s %d.pdf", prefix, month, day)
	if prefix == "" {
		noYear = fmt.Sprintf("%s %d.pdf", month, day)
	}
	noYearPath := filepath.Join(pdfDir, noYear)
	if err := moveFile(path, noYearPath); err != nil {
		return "", err
	}

	args := []string{"put"}

	// Use user's coverpage setting, fallback to environment variable, then default to "current"
	coverpageSetting := ""
	if user != nil {
		coverpageSetting = user.CoverpageSetting
	}
	if coverpageSetting == "" {
		if config.Get("RMAPI_COVERPAGE", "") == "first" {
			coverpageSetting = "first"
		} else {
			coverpageSetting = "current"
		}
	}

	if coverpageSetting == "first" {
		args = append(args, "--coverpage=1")
	}

	// Use user's conflict resolution setting if provided, otherwise fall back to environment variable or default
	conflictResolution := ""
	if user != nil {
		conflictResolution = user.ConflictResolution
	}
	if conflictResolution == "" {
		conflictResolution = config.Get("RMAPI_CONFLICT_RESOLUTION", "abort")
	}

	// Determine effective conflict resolution based on file type
	// content_only only works with PDF files, so fallback to abort for other file types
	effectiveConflictResolution := conflictResolution
	if conflictResolution == "content_only" {
		ext := strings.ToLower(filepath.Ext(noYearPath))
		if ext != ".pdf" {
			effectiveConflictResolution = "abort"
		}
	}

	switch effectiveConflictResolution {
	case "overwrite":
		args = append(args, "--force")
	case "content_only":
		args = append(args, "--content-only") // Only reached for PDFs
	}

	args = append(args, noYearPath, rmDir)
	cmd := rmapiCmd(user, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		raw := strings.TrimSpace(string(out))
		// find the first "Error: " in the output
		if idx := strings.Index(raw, "Error:"); idx != -1 {
			// return just "Error: entry already exists" (or whatever follows)
			return "", fmt.Errorf(raw[idx:])
		}
		// fallback if we didn't find it
		return "", fmt.Errorf("rmapi put failed: %s", raw)
	}

	// Rename local file to include year (for local storage)
	withYear := strings.TrimSuffix(noYear, ".pdf") + fmt.Sprintf(" %d.pdf", year)
	withYearPath := filepath.Join(pdfDir, withYear)
	if err := moveFile(noYearPath, withYearPath); err != nil {
		return "", err
	}

	// Return the name that ended up on the device
	return noYear, nil
}

func CleanupOld(prefix, rmDir string, retentionDays int, user *database.User) error {
	today := time.Now()
	cutoff := today.AddDate(0, 0, -retentionDays)

	// In multi-user mode, add user context to cleanup logs
	if database.IsMultiUserMode() && user != nil {
		Logf("[cleanup] user=%s, today=%s, cutoff=%s",
			user.ID.String(), today.Format("2006-01-02"), cutoff.Format("2006-01-02"))
	} else {
		Logf("[cleanup] today=%s, cutoff=%s",
			today.Format("2006-01-02"), cutoff.Format("2006-01-02"))
	}

	// 1) List remote files
	proc := rmapiCmd(user, "ls", rmDir)
	out, err := proc.Output()
	if err != nil {
		return err
	}

	// 2) Compile regexes
	// match any file entry (we'll strip .pdf later if present)
	lineRe := regexp.MustCompile(`^\[f\]\s+(.*)$`)

	// date pattern: optional prefix, Month, Day, with or without ".pdf"
	var dateRe *regexp.Regexp
	if prefix != "" {
		// e.g. "DUMMY May 7" or "DUMMY May 7.pdf"
		dateRe = regexp.MustCompile(
			`^` + regexp.QuoteMeta(prefix+` `) + `([A-Za-z]+)\s+(\d+)(?:\.pdf)?$`,
		)
	} else {
		dateRe = regexp.MustCompile(
			`^([A-Za-z]+)\s+(\d+)(?:\.pdf)?$`,
		)
	}

	// 3) Scan lines
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()

		// grab the name field (might end in ".pdf" or not)
		m := lineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		fname := strings.TrimSpace(m[1])

		// match date portion
		md := dateRe.FindStringSubmatch(fname)
		if md == nil {
			continue
		}
		monthStr, dayStr := md[1], md[2]

		// parse month and day into a time.Time
		t, err := time.Parse("January 2", monthStr+" "+dayStr)
		if err != nil {
			Logf("  ↳ parse error: %v", err)
			continue
		}
		fileDate := time.Date(today.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
		if fileDate.After(today) {
			fileDate = fileDate.AddDate(-1, 0, 0)
		}

		// decide whether to remove
		if fileDate.Before(cutoff) {
			Logf("Removing %s (dated %s < %s)",
				fname, fileDate.Format("2006-01-02"), cutoff.Format("2006-01-02"))
			rmapiCmd(user, "rm", filepath.Join(rmDir, fname)).Run()
		}
	}

	return nil
}
