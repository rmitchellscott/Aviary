package auth

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rmitchellscott/aviary/internal/database"
	"github.com/rmitchellscott/aviary/internal/smtp"
	"github.com/rmitchellscott/aviary/internal/storage"
)

// TestSMTPHandler tests SMTP configuration (admin only)
func TestSMTPHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "SMTP testing not available in single-user mode"})
		return
	}

	_, ok := RequireAdmin(c)
	if !ok {
		return
	}

	// Test SMTP connection
	if err := smtp.TestSMTPConnection(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "SMTP connection failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "SMTP connection successful",
	})
}

// GetSystemStatusHandler returns system status information (admin only)
func GetSystemStatusHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "System status not available in single-user mode"})
		return
	}

	_, ok := RequireAdmin(c)
	if !ok {
		return
	}

	// Get database stats
	dbStats, err := database.GetDatabaseStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get database stats"})
		return
	}

	// Check SMTP configuration
	smtpConfigured := smtp.IsSMTPConfigured()
	var smtpStatus string
	if smtpConfigured {
		if err := smtp.TestSMTPConnection(); err != nil {
			smtpStatus = "configured_but_failed"
		} else {
			smtpStatus = "configured_and_working"
		}
	} else {
		smtpStatus = "not_configured"
	}

	// Get system settings
	registrationEnabled, _ := database.GetSystemSetting("registration_enabled")
	maxAPIKeys, _ := database.GetSystemSetting("max_api_keys_per_user")
	sessionTimeout, _ := database.GetSystemSetting("session_timeout_hours")

	// Check if we're in dry run mode
	dryRunMode := os.Getenv("DRY_RUN") != ""

	c.JSON(http.StatusOK, gin.H{
		"database": dbStats,
		"smtp": gin.H{
			"configured": smtpConfigured,
			"status":     smtpStatus,
		},
		"settings": gin.H{
			"registration_enabled":  registrationEnabled,
			"max_api_keys_per_user": maxAPIKeys,
			"session_timeout_hours": sessionTimeout,
		},
		"mode":    "multi_user",
		"dry_run": dryRunMode,
	})
}

// UpdateSystemSettingHandler updates a system setting (admin only)
func UpdateSystemSettingHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "System settings not available in single-user mode"})
		return
	}

	user, ok := RequireAdmin(c)
	if !ok {
		return
	}

	var req struct {
		Key   string `json:"key" binding:"required"`
		Value string `json:"value" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Validate allowed settings
	allowedSettings := map[string]bool{
		"registration_enabled":         true,
		"max_api_keys_per_user":        true,
		"session_timeout_hours":        true,
		"password_reset_timeout_hours": true,
	}

	if !allowedSettings[req.Key] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Setting not allowed to be updated"})
		return
	}

	// Update the setting
	if err := database.SetSystemSetting(req.Key, req.Value, &user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update setting"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Setting updated successfully",
	})
}

// GetSystemSettingsHandler returns all system settings (admin only)
func GetSystemSettingsHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "System settings not available in single-user mode"})
		return
	}

	_, ok := RequireAdmin(c)
	if !ok {
		return
	}

	// Get all system settings
	var settings []database.SystemSetting
	if err := database.DB.Find(&settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve settings"})
		return
	}

	// Convert to map for easier frontend consumption
	settingsMap := make(map[string]interface{})
	for _, setting := range settings {
		settingsMap[setting.Key] = gin.H{
			"value":       setting.Value,
			"description": setting.Description,
			"updated_at":  setting.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, settingsMap)
}

// CleanupDataHandler performs database cleanup (admin only)
func CleanupDataHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Data cleanup not available in single-user mode"})
		return
	}

	_, ok := RequireAdmin(c)
	if !ok {
		return
	}

	// Perform cleanup
	if err := database.CleanupOldData(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cleanup data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Data cleanup completed successfully",
	})
}

// BackupDatabaseHandler initiates database backup (admin only)
func BackupDatabaseHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Database backup not available in single-user mode"})
		return
	}

	_, ok := RequireAdmin(c)
	if !ok {
		return
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	config := database.GetDatabaseConfig()

	var filename string
	var contentType string

	switch config.Type {
	case "sqlite":
		filename = fmt.Sprintf("aviary_backup_sqlite_%s.db", timestamp)
		contentType = "application/x-sqlite3"
	case "postgres":
		filename = fmt.Sprintf("aviary_backup_postgres_%s.sql", timestamp)
		contentType = "application/sql"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported database type for backup"})
		return
	}

	// Create temporary backup file
	tempDir := os.TempDir()
	backupPath := filepath.Join(tempDir, filename)

	// Perform backup
	if err := database.BackupDatabase(backupPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create database backup: " + err.Error(),
		})
		return
	}

	// Clean up backup file after download
	defer os.Remove(backupPath)

	// Set headers for download
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", contentType)
	c.Header("Content-Description", "Database Backup")

	// Stream file to client
	c.File(backupPath)
}

// RestoreDatabaseHandler initiates database restore (admin only)
func RestoreDatabaseHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Database restore not available in single-user mode"})
		return
	}

	_, ok := RequireAdmin(c)
	if !ok {
		return
	}

	// Parse multipart form
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse multipart form: " + err.Error()})
		return
	}

	// Get uploaded file
	file, header, err := c.Request.FormFile("backup_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No backup file provided"})
		return
	}
	defer file.Close()

	// Validate file type based on database configuration
	config := database.GetDatabaseConfig()
	validExtensions := map[string][]string{
		"sqlite":   {".db", ".sqlite", ".sqlite3"},
		"postgres": {".sql", ".dump", ".custom"},
	}

	validExts, exists := validExtensions[config.Type]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported database type for restore"})
		return
	}

	// Check file extension
	filename := header.Filename
	ext := filepath.Ext(filename)
	isValidExt := false
	for _, validExt := range validExts {
		if ext == validExt {
			isValidExt = true
			break
		}
	}

	if !isValidExt {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid file type. Expected: %v, got: %s", validExts, ext),
		})
		return
	}

	// Create temporary file for upload
	tempDir := os.TempDir()
	tempFilePath := filepath.Join(tempDir, "restore_"+filename)

	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temporary file"})
		return
	}
	defer os.Remove(tempFilePath)
	defer tempFile.Close()

	// Copy uploaded file to temp location
	if _, err := io.Copy(tempFile, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save uploaded file"})
		return
	}
	tempFile.Close()

	// Perform database restore
	if err := database.RestoreDatabase(tempFilePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to restore database: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Database restored successfully from " + filename,
	})
}

// BackupStorageHandler initiates storage directory backup (admin only)
func BackupStorageHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Storage backup not available in single-user mode"})
		return
	}

	_, ok := RequireAdmin(c)
	if !ok {
		return
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("aviary_storage_%s.tar.gz", timestamp)

	tempDir := os.TempDir()
	backupPath := filepath.Join(tempDir, filename)

	if err := storage.BackupUsersDir(backupPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create storage backup: " + err.Error()})
		return
	}

	defer os.Remove(backupPath)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/gzip")
	c.Header("Content-Description", "Storage Backup")
	c.File(backupPath)
}

// RestoreStorageHandler restores the storage directory from uploaded archive (admin only)
func RestoreStorageHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Storage restore not available in single-user mode"})
		return
	}

	_, ok := RequireAdmin(c)
	if !ok {
		return
	}

	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse multipart form: " + err.Error()})
		return
	}

	file, header, err := c.Request.FormFile("backup_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No backup file provided"})
		return
	}
	defer file.Close()

	overwrite := c.PostForm("mode") == "overwrite"

	tempDir := os.TempDir()
	tempFilePath := filepath.Join(tempDir, "restore_"+header.Filename)

	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temporary file"})
		return
	}
	defer os.Remove(tempFilePath)
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save uploaded file"})
		return
	}
	tempFile.Close()

	if err := storage.RestoreUsersDir(tempFilePath, overwrite); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restore storage: " + err.Error()})
		return
	}

	mode := "skipped conflicts"
	if overwrite {
		mode = "overwritten conflicts"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Storage restored successfully from %s (%s)", header.Filename, mode),
	})
}
