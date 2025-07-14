package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
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
		log.Printf("Users already exist, skipping migration")
		return nil
	}
	
	// Create admin user from environment variables
	username := os.Getenv("AUTH_USERNAME")
	password := os.Getenv("AUTH_PASSWORD")
	email := os.Getenv("ADMIN_EMAIL")
	
	if username == "" || password == "" {
		return fmt.Errorf("AUTH_USERNAME and AUTH_PASSWORD must be set when enabling multi-user mode")
	}
	
	if email == "" {
		email = username + "@localhost" // Default email if not provided
	}
	
	adminUser, err := userService.CreateUser(username, email, password, true)
	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}
	
	log.Printf("Admin user created: %s (ID: %s)", adminUser.Username, adminUser.ID)
	
	// Set default rmapi settings if available
	if rmapiHost := os.Getenv("RMAPI_HOST"); rmapiHost != "" {
		err = userService.UpdateUserSettings(adminUser.ID, map[string]interface{}{
			"rmapi_host": rmapiHost,
		})
		if err != nil {
			log.Printf("Warning: failed to set RMAPI_HOST for admin user: %v", err)
		}
	}
	
	if rmTargetDir := os.Getenv("RM_TARGET_DIR"); rmTargetDir != "" {
		err = userService.UpdateUserSettings(adminUser.ID, map[string]interface{}{
			"default_rmdir": rmTargetDir,
		})
		if err != nil {
			log.Printf("Warning: failed to set default_rmdir for admin user: %v", err)
		}
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

// MigrateUserDocuments migrates existing documents to the new user structure
func MigrateUserDocuments(userID uuid.UUID) error {
	// This would scan the existing PDF directory and create document records
	// for files that already exist on disk
	
	pdfDir := os.Getenv("PDF_DIR")
	if pdfDir == "" {
		pdfDir = "/app/pdfs"
	}
	
	// For now, just log that this would need to be implemented
	log.Printf("TODO: Implement document migration for user %s from directory %s", userID, pdfDir)
	
	return nil
}

// BackupDatabase creates a backup of the database
func BackupDatabase(backupPath string) error {
	config := GetDatabaseConfig()
	
	switch config.Type {
	case "sqlite":
		return backupSQLiteDatabase(backupPath)
	case "postgres":
		return backupPostgresDatabase(backupPath, config)
	default:
		return fmt.Errorf("backup not implemented for database type: %s", config.Type)
	}
}

// backupSQLiteDatabase creates a backup of SQLite database
func backupSQLiteDatabase(backupPath string) error {
	// For SQLite, we can just copy the database file
	config := GetDatabaseConfig()
	dbPath := fmt.Sprintf("%s/aviary.db", config.DataDir)
	
	// Use OS commands to copy the file
	// This is a simplified implementation
	log.Printf("TODO: Implement SQLite backup from %s to %s", dbPath, backupPath)
	return nil
}

// backupPostgresDatabase creates a backup of PostgreSQL database
func backupPostgresDatabase(backupPath string, config *DatabaseConfig) error {
	// Use pg_dump command
	log.Printf("TODO: Implement PostgreSQL backup to %s", backupPath)
	return nil
}

// RestoreDatabase restores a database from backup
func RestoreDatabase(backupPath string) error {
	config := GetDatabaseConfig()
	
	switch config.Type {
	case "sqlite":
		return restoreSQLiteDatabase(backupPath)
	case "postgres":
		return restorePostgresDatabase(backupPath, config)
	default:
		return fmt.Errorf("restore not implemented for database type: %s", config.Type)
	}
}

// restoreSQLiteDatabase restores SQLite database from backup
func restoreSQLiteDatabase(backupPath string) error {
	log.Printf("TODO: Implement SQLite restore from %s", backupPath)
	return nil
}

// restorePostgresDatabase restores PostgreSQL database from backup
func restorePostgresDatabase(backupPath string, config *DatabaseConfig) error {
	log.Printf("TODO: Implement PostgreSQL restore from %s", backupPath)
	return nil
}

// CleanupOldData removes old data based on retention policies
func CleanupOldData() error {
	userService := NewUserService(DB)
	apiKeyService := NewAPIKeyService(DB)
	
	// Clean up expired sessions
	if err := userService.CleanupExpiredSessions(); err != nil {
		log.Printf("Warning: failed to cleanup expired sessions: %v", err)
	}
	
	// Clean up expired reset tokens
	if err := userService.CleanupExpiredResetTokens(); err != nil {
		log.Printf("Warning: failed to cleanup expired reset tokens: %v", err)
	}
	
	// Clean up expired API keys
	if err := apiKeyService.CleanupExpiredAPIKeys(); err != nil {
		log.Printf("Warning: failed to cleanup expired API keys: %v", err)
	}
	
	// Clean up old login attempts (older than 30 days)
	if err := DB.Where("attempted_at < ?", time.Now().AddDate(0, 0, -30)).Delete(&LoginAttempt{}).Error; err != nil {
		log.Printf("Warning: failed to cleanup old login attempts: %v", err)
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