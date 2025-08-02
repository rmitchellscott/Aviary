package manager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/database"
	"github.com/rmitchellscott/aviary/internal/logging"
)

// GetUserDataDir returns the data directory for a specific user
// In multi-user mode, creates /data/users/{user_id}/
// In single-user mode, returns /data/
func GetUserDataDir(userID uuid.UUID) (string, error) {
	baseDir := config.Get("DATA_DIR", "")
	if baseDir == "" {
		baseDir = "/data"
	}

	if database.IsMultiUserMode() {
		userDir := filepath.Join(baseDir, "users", userID.String())
		if err := os.MkdirAll(userDir, 0755); err != nil {
			return "", err
		}
		return userDir, nil
	}

	// Single-user mode - use base directory
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", err
	}
	return baseDir, nil
}

// GetUserPDFDir returns the PDF directory for a specific user
// In multi-user mode, creates /data/users/{user_id}/pdfs/
// In single-user mode, returns the PDF_DIR env var or /data/pdfs/
func GetUserPDFDir(userID uuid.UUID, prefix string) (string, error) {
	var baseDir string

	if database.IsMultiUserMode() {
		userDataDir, err := GetUserDataDir(userID)
		if err != nil {
			return "", err
		}
		baseDir = filepath.Join(userDataDir, "pdfs")
	} else {
		// Single-user mode - use PDF_DIR environment variable
		baseDir = config.Get("PDF_DIR", "")
		if baseDir == "" {
			baseDir = "/data/pdfs"
		}
	}

	// Add prefix subdirectory if provided
	if prefix != "" {
		baseDir = filepath.Join(baseDir, prefix)
	}

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", err
	}
	return baseDir, nil
}

// GetUserUploadDir returns the upload directory for a specific user
// In multi-user mode, creates /data/users/{user_id}/uploads/
// In single-user mode, returns ./uploads/
func GetUserUploadDir(userID uuid.UUID) (string, error) {
	if database.IsMultiUserMode() {
		userDataDir, err := GetUserDataDir(userID)
		if err != nil {
			return "", err
		}
		uploadDir := filepath.Join(userDataDir, "uploads")
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			return "", err
		}
		return uploadDir, nil
	}

	// Single-user mode - use ./uploads
	uploadDir := "./uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", err
	}
	return uploadDir, nil
}

// GetUserTempDir returns the temp directory for a specific user
// In multi-user mode, creates /data/users/{user_id}/temp/
// In single-user mode, returns system temp directory
func GetUserTempDir(userID uuid.UUID) (string, error) {
	if database.IsMultiUserMode() {
		userDataDir, err := GetUserDataDir(userID)
		if err != nil {
			return "", err
		}
		tempDir := filepath.Join(userDataDir, "temp")
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			return "", err
		}
		return tempDir, nil
	}

	// Single-user mode - use system temp directory
	return os.TempDir(), nil
}

// CleanupUserTempDir removes old temp files for a user
func CleanupUserTempDir(userID uuid.UUID) error {
	if !database.IsMultiUserMode() {
		return nil // Don't cleanup system temp directory
	}

	userDataDir, err := GetUserDataDir(userID)
	if err != nil {
		return err
	}

	tempDir := filepath.Join(userDataDir, "temp")
	if err := os.RemoveAll(tempDir); err != nil {
		return err
	}

	return os.MkdirAll(tempDir, 0755)
}

// GetUserRmapiConfigPath returns the path to the rmapi configuration file for a
// user. In multi-user mode, it first checks the database for stored config content
// and creates a temporary file if found, otherwise falls back to filesystem.
// The necessary directory structure is created if missing.
func GetUserRmapiConfigPath(userID uuid.UUID) (string, error) {
	if database.IsMultiUserMode() {
		// First try to load config from database
		configContent, err := LoadUserRmapiConfig(userID)
		if err == nil && configContent != "" {
			// Create temporary file from database content
			return createTempConfigFile(userID, configContent)
		}

		// Fall back to filesystem path for migration period
		userDataDir, err := GetUserDataDir(userID)
		if err != nil {
			return "", err
		}
		cfgDir := filepath.Join(userDataDir, "rmapi")
		if err := os.MkdirAll(cfgDir, 0755); err != nil {
			return "", err
		}
		return filepath.Join(cfgDir, "rmapi.conf"), nil
	}

	// Single-user mode - use filesystem
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cfgDir := filepath.Join(home, ".config", "rmapi")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "rmapi.conf"), nil
}

// IsUserPaired checks whether the rmapi configuration exists for the given user.
// In multi-user mode, it first checks the database, then falls back to filesystem.
func IsUserPaired(userID uuid.UUID) bool {
	logging.Logf("[DEBUG] IsUserPaired called with userID: %s", userID.String())
	
	if database.IsMultiUserMode() {
		logging.Logf("[DEBUG] In multi-user mode, checking database first")
		
		// First check database
		configContent, err := LoadUserRmapiConfig(userID)
		logging.Logf("[DEBUG] LoadUserRmapiConfig returned: err=%v, contentLength=%d", err, len(configContent))
		
		if err == nil && configContent != "" {
			// Found config in database
			logging.Logf("[DEBUG] Found valid config in database, returning true")
			return true
		}

		logging.Logf("[DEBUG] Database check failed or empty, falling back to filesystem")
		
		// If database query failed or config is empty, fall back to filesystem check
		userDataDir, err := GetUserDataDir(userID)
		if err != nil {
			logging.Logf("[DEBUG] GetUserDataDir failed: %v", err)
			return false
		}
		cfgPath := filepath.Join(userDataDir, "rmapi", "rmapi.conf")
		logging.Logf("[DEBUG] Checking filesystem config at: %s", cfgPath)
		
		info, err := os.Stat(cfgPath)
		if err != nil {
			logging.Logf("[DEBUG] Filesystem config stat failed: %v", err)
			return false
		}
		
		result := info.Size() > 0
		logging.Logf("[DEBUG] Filesystem config exists, size: %d, returning: %t", info.Size(), result)
		return result
	}

	logging.Logf("[DEBUG] In single-user mode, checking filesystem")
	
	// Single-user mode - check filesystem
	cfgPath, err := GetUserRmapiConfigPath(userID)
	if err != nil {
		logging.Logf("[DEBUG] GetUserRmapiConfigPath failed: %v", err)
		return false
	}
	info, err := os.Stat(cfgPath)
	if err != nil || info.Size() == 0 {
		logging.Logf("[DEBUG] Single-user filesystem check failed: %v", err)
		return false
	}
	
	logging.Logf("[DEBUG] Single-user filesystem config valid, returning true")
	return true
}


// LoadUserRmapiConfig loads the rmapi configuration content from the database
func LoadUserRmapiConfig(userID uuid.UUID) (string, error) {
	logging.Logf("[DEBUG] LoadUserRmapiConfig called with userID: %s", userID.String())
	
	if !database.IsMultiUserMode() {
		logging.Logf("[DEBUG] Not in multi-user mode, returning error")
		return "", fmt.Errorf("rmapi config database storage only available in multi-user mode")
	}

	var user database.User
	logging.Logf("[DEBUG] Querying database for user %s", userID.String())
	if err := database.DB.Select("rmapi_config").Where("id = ?", userID).First(&user).Error; err != nil {
		logging.Logf("[DEBUG] Database query failed for user %s: %v", userID.String(), err)
		return "", err
	}

	logging.Logf("[DEBUG] Database query succeeded for user %s, config length: %d", userID.String(), len(user.RmapiConfig))
	logging.Logf("[DEBUG] Config content preview: %s", func() string {
		if len(user.RmapiConfig) > 50 {
			return user.RmapiConfig[:50] + "..."
		}
		return user.RmapiConfig
	}())
	
	// Return the config content (could be empty string if not migrated yet)
	return user.RmapiConfig, nil
}

// createTempConfigFile creates a temporary rmapi config file from database content
func createTempConfigFile(userID uuid.UUID, configContent string) (string, error) {
	// Get user temp directory
	tempDir, err := GetUserTempDir(userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user temp directory: %w", err)
	}

	// Generate unique filename using UUID to prevent collisions
	tempFileName := fmt.Sprintf("rmapi-config-%s.conf", uuid.New().String())
	tempPath := filepath.Join(tempDir, tempFileName)

	// Write config content to temp file
	if err := os.WriteFile(tempPath, []byte(configContent), 0600); err != nil {
		return "", fmt.Errorf("failed to write temp config file: %w", err)
	}

	return tempPath, nil
}

// cleanupTempConfigFile removes a temporary config file
func cleanupTempConfigFile(tempPath string) {
	if tempPath != "" && filepath.Base(tempPath) != "rmapi.conf" { // Don't delete the actual config file
		os.Remove(tempPath)
	}
}

// DebugUserConfigState returns debug information about user's rmapi config state
func DebugUserConfigState(userID uuid.UUID) map[string]interface{} {
	debug := make(map[string]interface{})
	debug["user_id"] = userID.String()
	debug["multi_user_mode"] = database.IsMultiUserMode()
	
	// Check database state
	if database.IsMultiUserMode() {
		configContent, err := LoadUserRmapiConfig(userID)
		debug["database_query_error"] = err
		debug["database_config_length"] = len(configContent)
		debug["database_config_empty"] = configContent == ""
		
		// Check filesystem state
		userDataDir, err := GetUserDataDir(userID)
		debug["user_data_dir_error"] = err
		if err == nil {
			cfgPath := filepath.Join(userDataDir, "rmapi", "rmapi.conf")
			debug["filesystem_config_path"] = cfgPath
			info, err := os.Stat(cfgPath)
			debug["filesystem_stat_error"] = err
			if err == nil {
				debug["filesystem_config_size"] = info.Size()
				debug["filesystem_config_exists"] = true
			} else {
				debug["filesystem_config_exists"] = false
			}
		}
	}
	
	// Final paired result
	debug["is_user_paired"] = IsUserPaired(userID)
	
	return debug
}
