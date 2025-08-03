package database

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/logging"
	"github.com/rmitchellscott/aviary/internal/storage"
)

// MigrateToMultiUser handles the migration from single-user to multi-user mode
func MigrateToMultiUser() error {
	if !IsMultiUserMode() {
		return nil // Nothing to do
	}

	userService := NewUserService(DB)

	// Check if any users exist
	var userCount int64
	if err := DB.Model(&User{}).Count(&userCount).Error; err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	if userCount > 0 {
		logging.Logf("[STARTUP] Users already exist, skipping user creation migration")
		// Still run schema migrations even if users exist
		return RunMigrations("STARTUP")
	}

	// Create admin user from environment variables
	username := config.Get("AUTH_USERNAME", "")
	password := config.Get("AUTH_PASSWORD", "")
	email := config.Get("ADMIN_EMAIL", "")

	if username == "" || password == "" {
		logging.Logf("[STARTUP] No admin user configured - navigate to /register to create the first admin account")
		return nil
	}

	if email == "" {
		email = username + "@localhost" // Default email if not provided
	}

	adminUser, err := userService.CreateUser(username, email, password, true)
	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	logging.Logf("[STARTUP] Admin user created: %s (ID: %s)", adminUser.Username, adminUser.ID)

	// Set default rmapi settings if available
	if rmapiHost := config.Get("RMAPI_HOST", ""); rmapiHost != "" {
		err = userService.UpdateUserSettings(adminUser.ID, map[string]interface{}{
			"rmapi_host": rmapiHost,
		})
		if err != nil {
			logging.Logf("[WARNING] failed to set RMAPI_HOST for admin user: %v", err)
		}
	}

	if rmTargetDir := config.Get("RM_TARGET_DIR", ""); rmTargetDir != "" {
		err = userService.UpdateUserSettings(adminUser.ID, map[string]interface{}{
			"default_rmdir": rmTargetDir,
		})
		if err != nil {
			logging.Logf("[WARNING] failed to set default_rmdir for admin user: %v", err)
		}
	}

	// Set coverpage setting based on RMAPI_COVERPAGE environment variable
	coverpageSetting := "current" // default value
	if config.Get("RMAPI_COVERPAGE", "") == "first" {
		coverpageSetting = "first"
	}
	err = userService.UpdateUserSettings(adminUser.ID, map[string]interface{}{
		"coverpage_setting": coverpageSetting,
	})
	if err != nil {
		logging.Logf("[WARNING] failed to set coverpage_setting for admin user: %v", err)
	}

	// Migrate API key from environment to database
	if envApiKey := config.Get("API_KEY", ""); envApiKey != "" {
		logging.Logf("[STARTUP] Migrating API_KEY environment variable to database for admin user")
		apiKeyService := NewAPIKeyService(DB)
		_, err := apiKeyService.CreateAPIKeyFromValue(adminUser.ID, "Migrated from API_KEY env var", envApiKey, nil)
		if err != nil {
			logging.Logf("[WARNING] failed to migrate API_KEY to database: %v", err)
		} else {
			logging.Logf("[STARTUP] Successfully migrated API_KEY to database with never-expiring key")
		}
	}

	// Migrate single-user data (rmapi config and files) synchronously to avoid database locking
	if err := MigrateSingleUserData(adminUser.ID); err != nil {
		logging.Logf("[WARNING] failed to migrate single-user data during startup: %v", err)
	}

	// Ensure all existing users have coverpage setting set
	if err := ensureUsersHaveCoverpageSetting(); err != nil {
		logging.Logf("[WARNING] failed to set coverpage setting for existing users: %v", err)
	}

	// Run schema migrations after user creation
	return RunMigrations("STARTUP")
}

// MigrateSingleUserData migrates rmapi config and archived files to first admin user
func MigrateSingleUserData(adminUserID uuid.UUID) error {
	// Copy rmapi config from root to user directory
	if err := migrateRmapiConfig(adminUserID); err != nil {
		logging.Logf("[WARNING] failed to migrate rmapi config: %v", err)
	}

	// Migrate archived files to user directory
	if err := migrateArchivedFiles(adminUserID); err != nil {
		logging.Logf("[WARNING] failed to migrate archived files: %v", err)
	}

	return nil
}

// CreateDefaultAdminUser creates a default admin user if none exists
func CreateDefaultAdminUser(username, email, password string) (*User, error) {
	userService := NewUserService(DB)

	// Check if any admin users exist
	var adminCount int64
	if err := DB.Model(&User{}).Where("is_admin = ?", true).Count(&adminCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count admin users: %w", err)
	}

	if adminCount > 0 {
		return nil, fmt.Errorf("admin user already exists")
	}

	return userService.CreateUser(username, email, password, true)
}


// CleanupOldData removes old data based on retention policies
func CleanupOldData() error {
	userService := NewUserService(DB)
	apiKeyService := NewAPIKeyService(DB)

	// Clean up expired sessions
	if err := userService.CleanupExpiredSessions(); err != nil {
		logging.Logf("[WARNING] failed to cleanup expired sessions: %v", err)
	}

	// Clean up expired reset tokens
	if err := userService.CleanupExpiredResetTokens(); err != nil {
		logging.Logf("[WARNING] failed to cleanup expired reset tokens: %v", err)
	}

	// Clean up expired API keys
	if err := apiKeyService.CleanupExpiredAPIKeys(); err != nil {
		logging.Logf("[WARNING] failed to cleanup expired API keys: %v", err)
	}

	// Clean up old login attempts (older than 30 days)
	if err := DB.Where("attempted_at < ?", time.Now().AddDate(0, 0, -30)).Delete(&LoginAttempt{}).Error; err != nil {
		logging.Logf("[WARNING] failed to cleanup old login attempts: %v", err)
	}

	return nil
}

// GetDatabaseStats returns database statistics
func GetDatabaseStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// User counts
	var userCount, activeUserCount, adminCount int64
	if err := DB.Model(&User{}).Count(&userCount).Error; err != nil {
		return nil, err
	}
	stats["total_users"] = userCount

	if err := DB.Model(&User{}).Where("is_active = ?", true).Count(&activeUserCount).Error; err != nil {
		return nil, err
	}
	stats["active_users"] = activeUserCount

	if err := DB.Model(&User{}).Where("is_admin = ?", true).Count(&adminCount).Error; err != nil {
		return nil, err
	}
	stats["admin_users"] = adminCount

	// API key stats
	apiKeyService := NewAPIKeyService(DB)
	apiKeyStats, err := apiKeyService.GetAPIKeyStats()
	if err != nil {
		return nil, err
	}
	stats["api_keys"] = apiKeyStats

	// Document counts
	var documentCount int64
	if err := DB.Model(&Document{}).Count(&documentCount).Error; err != nil {
		return nil, err
	}
	stats["documents"] = documentCount

	// Session counts
	var sessionCount int64
	if err := DB.Model(&UserSession{}).Where("expires_at > ?", time.Now()).Count(&sessionCount).Error; err != nil {
		return nil, err
	}
	stats["active_sessions"] = sessionCount

	return stats, nil
}

func isDocumentFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".pdf" || ext == ".epub"
}

func getSingleUserStorageDir() string {
	if pdfDir := config.Get("PDF_DIR", ""); pdfDir != "" {
		return pdfDir
	}
	return "pdfs"
}

// migrateRmapiConfig migrates the root rmapi.conf file to the admin user's database
func migrateRmapiConfig(adminUserID uuid.UUID) error {
	// Check if single-user rmapi.conf exists at /root/.config/rmapi/rmapi.conf
	rootRmapiPath := "/root/.config/rmapi/rmapi.conf"
	if _, err := os.Stat(rootRmapiPath); os.IsNotExist(err) {
		logging.Logf("[STARTUP] No single-user rmapi.conf found to migrate at %s", rootRmapiPath)
		return nil
	}

	// Check if user already has rmapi config in database
	var user User
	if err := DB.Select("rmapi_config").Where("id = ?", adminUserID).First(&user).Error; err != nil {
		return fmt.Errorf("failed to check existing rmapi config: %w", err)
	}
	
	if user.RmapiConfig != "" {
		logging.Logf("[STARTUP] User already has rmapi config in database, skipping migration")
		return nil
	}

	// Read config content directly from source
	configContent, err := os.ReadFile(rootRmapiPath)
	if err != nil {
		return fmt.Errorf("failed to read rmapi config from %s: %w", rootRmapiPath, err)
	}

	// Save config content to database
	if err := SaveUserRmapiConfig(adminUserID, string(configContent)); err != nil {
		return fmt.Errorf("failed to save rmapi config to database during migration: %w", err)
	}

	logging.Logf("[STARTUP] Successfully migrated rmapi.conf from %s to database for admin user", rootRmapiPath)
	return nil
}

// migrateArchivedFiles copies archived files from single-user storage to multi-user storage
func migrateArchivedFiles(adminUserID uuid.UUID) error {
	ctx := context.Background()
	backend := storage.GetStorageBackend()
	migratedCount := 0

	var singleUserFiles []string
	var err error
	
	if storage.GetStorageType() == "s3" {
		singleUserFiles, err = backend.List(ctx, "pdfs/")
		if err != nil {
			logging.Logf("[STARTUP] No single-user files found to migrate: %v", err)
			return nil
		}
	} else {
		singleUserStorageDir := getSingleUserStorageDir()
		
		// Check if directory exists
		if _, err := os.Stat(singleUserStorageDir); os.IsNotExist(err) {
			logging.Logf("[STARTUP] No single-user PDF directory found at %s", singleUserStorageDir)
			return nil
		}
		
		// Walk directory recursively
		err := filepath.Walk(singleUserStorageDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Continue walking despite errors
			}
			
			// Calculate relative path from base directory
			relPath, err := filepath.Rel(singleUserStorageDir, path)
			if err != nil {
				return nil
			}
			
			if !info.IsDir() && isDocumentFile(info.Name()) {
				singleUserFiles = append(singleUserFiles, relPath)
			}
			
			return nil
		})
		
		if err != nil {
			logging.Logf("[STARTUP] Failed to walk single-user PDF directory %s: %v", singleUserStorageDir, err)
			return nil
		}
	}

	if len(singleUserFiles) == 0 {
		logging.Logf("[STARTUP] No single-user archived files found to migrate")
		return nil
	}

	logging.Logf("[STARTUP] Found %d single-user files to migrate", len(singleUserFiles))

	// Migrate each file from single-user to multi-user storage
	for _, filename := range singleUserFiles {
		var reader io.ReadCloser
		var oldKey string
		
		if storage.GetStorageType() == "s3" {
			if strings.HasSuffix(filename, "/") {
				continue
			}
			relPath := strings.TrimPrefix(filename, "pdfs/")
			if relPath == filename {
				continue
			}
			oldKey = filename
			
			r, err := backend.Get(ctx, oldKey)
			if err != nil {
				logging.Logf("[WARNING] Failed to read file %s during migration: %v", oldKey, err)
				continue
			}
			reader = r
		} else {
			singleUserStorageDir := getSingleUserStorageDir()
			oldPath := filepath.Join(singleUserStorageDir, filename)
			
			file, err := os.Open(oldPath)
			if err != nil {
				logging.Logf("[WARNING] Failed to read file %s during migration: %v", oldPath, err)
				continue
			}
			
			reader = file
			oldKey = filename
		}

		// Generate new multi-user key
		var fileRelPath string
		if storage.GetStorageType() == "s3" {
			fileRelPath = strings.TrimPrefix(filename, "pdfs/")
		} else {
			fileRelPath = filename
		}
		
		// Check if file is in a subdirectory
		dir := filepath.Dir(fileRelPath)
		base := filepath.Base(fileRelPath)
		
		var newKey string
		if dir != "." && dir != "" {
			newKey = storage.GenerateUserDocumentKey(adminUserID, dir, base, true)
		} else {
			newKey = storage.GenerateUserDocumentKey(adminUserID, "", base, true)
		}

		// Write to new location
		if err := backend.Put(ctx, newKey, reader); err != nil {
			reader.Close()
			logging.Logf("[WARNING] Failed to write file %s during migration: %v", newKey, err)
			continue
		}
		reader.Close()

		// Delete old file
		if storage.GetStorageType() == "s3" {
			if err := backend.Delete(ctx, oldKey); err != nil {
				logging.Logf("[WARNING] Failed to delete old file %s after migration: %v", oldKey, err)
			}
		} else {
			singleUserStorageDir := getSingleUserStorageDir()
			oldPath := filepath.Join(singleUserStorageDir, filename)
			if err := os.Remove(oldPath); err != nil {
				logging.Logf("[WARNING] Failed to delete old file %s after migration: %v", oldPath, err)
			}
		}

		migratedCount++
	}

	logging.Logf("[STARTUP] Successfully migrated %d archived files from single-user to multi-user storage", migratedCount)
	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Ensure destination directory exists
	destDir := filepath.Dir(dst)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Copy file permissions
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, sourceInfo.Mode())
}

// ensureUsersHaveCoverpageSetting sets coverpage setting for users who don't have it set
func ensureUsersHaveCoverpageSetting() error {
	// Set coverpage setting based on server's RMAPI_COVERPAGE environment variable
	coverpageSetting := "current" // default value
	if config.Get("RMAPI_COVERPAGE", "") == "first" {
		coverpageSetting = "first"
	}

	// Update users who have empty or null coverpage_setting
	result := DB.Model(&User{}).Where("coverpage_setting = ? OR coverpage_setting IS NULL", "").Update("coverpage_setting", coverpageSetting)
	if result.Error != nil {
		return fmt.Errorf("failed to update coverpage setting for existing users: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		logging.Logf("[STARTUP] Updated coverpage setting to '%s' for %d existing users", coverpageSetting, result.RowsAffected)
	}

	return nil
}
