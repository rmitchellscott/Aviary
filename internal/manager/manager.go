package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/database"
	"github.com/rmitchellscott/aviary/internal/rmapi"
	"github.com/rmitchellscott/aviary/internal/storage"
)

// UploadOptions bundles all rmapi put flags for SimpleUpload and RenameAndUpload.
type UploadOptions struct {
	ConflictResolution string
	Coverpage          string
	Contrast           string
	CurrentPage        string
}

// ExecCommand is exec.Command by default, but can be overridden in tests.
var ExecCommand = exec.Command

func init() {
	if config.Get("DRY_RUN", "") != "" {
		ExecCommand = func(name string, args ...string) *exec.Cmd {
			cmdStr := name
			if len(args) > 0 {
				cmdStr += " " + strings.Join(args, " ")
			}
			Logf("[DRY RUN] would run: %s", cmdStr)
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

// Storage-based rename functions that work with storage keys instead of file paths

// RenameStorageNoYear renames a file in storage to include month and day but no year
func RenameStorageNoYear(ctx context.Context, srcKey, prefix string, userID uuid.UUID) (string, error) {
	today := time.Now()
	month, day := today.Format("January"), today.Day()

	ext := filepath.Ext(srcKey)
	filename := fmt.Sprintf("%s %s %d%s", prefix, month, day, ext)
	if prefix == "" {
		filename = fmt.Sprintf("%s %d%s", month, day, ext)
	}

	// Generate destination key
	multiUserMode := database.IsMultiUserMode()
	dstKey := storage.GenerateUserDocumentKey(userID, prefix, filename, multiUserMode)

	// Move in storage
	if err := storage.MoveInStorage(ctx, srcKey, dstKey); err != nil {
		return "", err
	}

	return dstKey, nil
}

// AppendYearStorage renames a file in storage to append the current year
func AppendYearStorage(ctx context.Context, noYearKey string, userID uuid.UUID) (string, error) {
	today := time.Now()
	year := today.Year()

	// Extract filename from storage key
	parts := strings.Split(noYearKey, "/")
	filename := parts[len(parts)-1]
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	newFilename := fmt.Sprintf("%s %d%s", base, year, ext)

	// Extract prefix from storage key structure
	var prefix string
	if database.IsMultiUserMode() && userID != uuid.Nil {
		// Multi-user: users/{userID}/pdfs/{prefix}/{filename}
		if len(parts) >= 5 {
			prefix = parts[3] // The prefix is the 4th part (0-indexed)
		}
	} else {
		// Single-user: pdfs/{prefix}/{filename}
		if len(parts) >= 3 {
			prefix = parts[1] // The prefix is the 2nd part (0-indexed)
		}
	}

	// Generate destination key
	multiUserMode := database.IsMultiUserMode()
	dstKey := storage.GenerateUserDocumentKey(userID, prefix, newFilename, multiUserMode)

	// Move in storage
	if err := storage.MoveInStorage(ctx, noYearKey, dstKey); err != nil {
		return "", err
	}

	return dstKey, nil
}

// RenameStorage performs both operations: rename to no-year, then append year
func RenameStorage(ctx context.Context, srcKey, prefix string, userID uuid.UUID) (string, error) {
	// First rename to no-year format
	noYearKey, err := RenameStorageNoYear(ctx, srcKey, prefix, userID)
	if err != nil {
		return "", err
	}

	// Then append year
	withYearKey, err := AppendYearStorage(ctx, noYearKey, userID)
	if err != nil {
		return "", err
	}

	return withYearKey, nil
}

// buildPutArgs constructs the rmapi put argument list from user settings and request options.
func buildPutArgs(path string, user *database.User, opts UploadOptions) []string {
	args := []string{"put"}
	ext := strings.ToLower(filepath.Ext(path))

	coverpageSetting := opts.Coverpage
	if coverpageSetting == "" && user != nil {
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

	conflictResolution := opts.ConflictResolution
	if conflictResolution == "" && user != nil {
		conflictResolution = user.ConflictResolution
	}
	if conflictResolution == "" {
		conflictResolution = config.Get("RMAPI_CONFLICT_RESOLUTION", "abort")
	}
	effectiveConflictResolution := conflictResolution
	if conflictResolution == "content_only" && ext != ".pdf" {
		effectiveConflictResolution = "abort"
	}
	switch effectiveConflictResolution {
	case "overwrite":
		args = append(args, "--force")
	case "content_only":
		args = append(args, "--content-only")
	}

	if ext == ".pdf" || ext == ".epub" {
		contrastSetting := opts.Contrast
		if contrastSetting == "" && user != nil {
			contrastSetting = user.ContrastSetting
		}
		if contrastSetting == "" {
			contrastSetting = config.Get("RMAPI_CONTRAST", "")
		}
		if contrastSetting != "" && contrastSetting != "none" {
			args = append(args, "--contrast="+contrastSetting)
		}
	}

	if opts.CurrentPage != "" && ext == ".pdf" {
		args = append(args, "--currentpage="+opts.CurrentPage)
	}

	return args
}

// runPutCommand executes rmapi put and returns the parsed result.
func runPutCommand(path, rmDir string, user *database.User, args []string) (string, error) {
	args = append(args, path, rmDir)
	cmd, cleanup := rmapi.NewCommand(user, args...)
	defer cleanup()
	out, err := cmd.CombinedOutput()
	if err != nil {
		raw := strings.TrimSpace(string(out))
		if idx := strings.Index(raw, "Error:"); idx != -1 {
			return "", fmt.Errorf("%s", raw[idx:])
		}
		return "", fmt.Errorf("rmapi put failed: %s", raw)
	}
	remoteName := filepath.Base(path)
	return remoteName, nil
}

// SimpleUpload calls `rmapi put` and returns the uploaded filename or a detailed error.
func SimpleUpload(path, rmDir string, user *database.User, opts UploadOptions) (string, error) {
	args := buildPutArgs(path, user, opts)
	return runPutCommand(path, rmDir, user, args)
}

// RenameAndUpload takes a storage key, renames it in storage, uploads via rmapi, and creates archival copy
func RenameAndUpload(storageKey, prefix, rmDir string, user *database.User, opts UploadOptions) (string, error) {
	// Validate prefix early
	var err error
	prefix, err = SanitizePrefix(prefix)
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	var userID uuid.UUID
	if user != nil {
		userID = user.ID
	}

	// Input is already a storage key, rename it directly to no-year format
	noYearKey, err := RenameStorageNoYear(ctx, storageKey, prefix, userID)
	if err != nil {
		return "", err
	}

	// Create temp file for rmapi upload
	ext := filepath.Ext(storageKey)
	tempFile, err := ioutil.TempFile("", "aviary-rmapi-*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tempFilePath := tempFile.Name()
	tempFile.Close()
	defer os.Remove(tempFilePath)

	// Download from storage to temp file
	if err := storage.CopyFileFromStorage(ctx, noYearKey, tempFilePath); err != nil {
		return "", fmt.Errorf("failed to download from storage: %w", err)
	}

	// Upload via rmapi
	args := buildPutArgs(tempFilePath, user, opts)
	_, err = runPutCommand(tempFilePath, rmDir, user, args)
	if err != nil {
		return "", err
	}

	// Append year in storage for archival
	Logf("[ARCHIVE] Starting archival storage process")
	archiveKey, err := AppendYearStorage(ctx, noYearKey, userID)
	if err != nil {
		return "", fmt.Errorf("failed to create archival copy: %w", err)
	}

	Logf("[ARCHIVE] Archival copy stored with key: %s", archiveKey)

	// Extract filename from no-year key for return value
	parts := strings.Split(noYearKey, "/")
	noYearFilename := parts[len(parts)-1]

	// Return the name that ended up on the device
	return noYearFilename, nil
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

	// 1) List remote files using JSON output for reliable parsing
	proc, cleanup := rmapi.NewCommand(user, "ls", "--json", rmDir)
	defer cleanup()
	out, err := proc.Output()
	if err != nil {
		return err
	}

	type lsEntry struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}

	var entries []lsEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return fmt.Errorf("failed to parse rmapi ls --json output: %w", err)
	}

	// 2) Compile date regex
	var dateRe *regexp.Regexp
	if prefix != "" {
		dateRe = regexp.MustCompile(
			`^` + regexp.QuoteMeta(prefix+` `) + `([A-Za-z]+)\s+(\d+)(?:\.\w+)?$`,
		)
	} else {
		dateRe = regexp.MustCompile(
			`^([A-Za-z]+)\s+(\d+)(?:\.\w+)?$`,
		)
	}

	// 3) Check each file entry
	for _, entry := range entries {
		if entry.Type != "DocumentType" {
			continue
		}

		md := dateRe.FindStringSubmatch(entry.Name)
		if md == nil {
			continue
		}
		monthStr, dayStr := md[1], md[2]

		t, err := time.Parse("January 2", monthStr+" "+dayStr)
		if err != nil {
			Logf("  ↳ parse error: %v", err)
			continue
		}
		fileDate := time.Date(today.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
		if fileDate.After(today) {
			fileDate = fileDate.AddDate(-1, 0, 0)
		}

		if fileDate.Before(cutoff) {
			Logf("Removing %s (dated %s < %s)",
				entry.Name, fileDate.Format("2006-01-02"), cutoff.Format("2006-01-02"))
			rmCmd, rmCleanup := rmapi.NewCommand(user, "rm", filepath.Join(rmDir, entry.Name))
			rmCmd.Run()
			rmCleanup()
		}
	}

	return nil
}
