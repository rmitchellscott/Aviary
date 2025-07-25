package auth

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/database"
	"github.com/rmitchellscott/aviary/internal/export"
	"github.com/rmitchellscott/aviary/internal/smtp"
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

	// Check SMTP configuration (without testing connection)
	smtpConfigured := smtp.IsSMTPConfigured()
	var smtpStatus string
	if smtpConfigured {
		smtpStatus = "configured"
	} else {
		smtpStatus = "not_configured"
	}

	// Get system settings
	registrationEnabled, _ := database.GetSystemSetting("registration_enabled")
	maxAPIKeys, _ := database.GetSystemSetting("max_api_keys_per_user")
	sessionTimeout, _ := database.GetSystemSetting("session_timeout_hours")

	// Check if we're in dry run mode
	dryRunMode := config.Get("DRY_RUN", "") != ""

	// Check authentication methods
	oidcEnabled := IsOIDCEnabled()
	proxyAuthEnabled := IsProxyAuthEnabled()

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
		"auth": gin.H{
			"oidc_enabled":       oidcEnabled,
			"proxy_auth_enabled": proxyAuthEnabled,
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

	// Parse query parameters for export options
	includeFiles := c.DefaultQuery("include_files", "true") == "true"
	includeConfigs := c.DefaultQuery("include_configs", "true") == "true"
	userIDsParam := c.Query("user_ids") // Comma-separated list of UUIDs

	// Parse user IDs if specified
	var userIDs []uuid.UUID
	if userIDsParam != "" {
		for _, idStr := range strings.Split(userIDsParam, ",") {
			if id, err := uuid.Parse(strings.TrimSpace(idStr)); err == nil {
				userIDs = append(userIDs, id)
			}
		}
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("aviary_backup_%s.tar.gz", timestamp)

	// Create temporary backup file
	tempDir := os.TempDir()
	backupPath := filepath.Join(tempDir, filename)

	// Create exporter
	dataDir := config.Get("DATA_DIR", "")
	if dataDir == "" {
		dataDir = "/data"
	}
	exporter := export.NewExporter(database.DB, dataDir)

	// Configure export options
	exportOptions := export.ExportOptions{
		IncludeDatabase: true,
		IncludeFiles:    includeFiles,
		IncludeConfigs:  includeConfigs,
		UserIDs:         userIDs,
	}

	// Perform export
	if err := exporter.Export(backupPath, exportOptions); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create backup: " + err.Error(),
		})
		return
	}

	// Clean up backup file after download
	defer os.Remove(backupPath)

	// Set headers for download
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/gzip")
	c.Header("Content-Description", "Aviary Backup")

	// Stream file to client
	c.File(backupPath)
}

// AnalyzeBackupHandler analyzes a backup file and returns metadata without restoring (admin only)
func AnalyzeBackupHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Backup analysis not available in single-user mode"})
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

	// Validate file type
	filename := header.Filename
	if !strings.HasSuffix(filename, ".tar.gz") && !strings.HasSuffix(filename, ".tgz") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file type. Expected .tar.gz or .tgz file",
			"valid": false,
		})
		return
	}

	// Create temporary file for upload
	tempDir := os.TempDir()
	tempFilePath := filepath.Join(tempDir, "analyze_"+filename)

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

	// Create analyzer
	dataDir := config.Get("DATA_DIR", "")
	if dataDir == "" {
		dataDir = "/data"
	}
	analyzer := export.NewAnalyzer(database.DB, dataDir)

	// Analyze backup
	analysis, err := analyzer.AnalyzeBackup(tempFilePath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to analyze backup: " + err.Error(),
			"valid": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":    true,
		"metadata": analysis,
	})
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

	// Parse import options
	overwriteFiles := c.PostForm("overwrite_files") == "true"
	overwriteDatabase := c.PostForm("overwrite_database") == "true"
	userIDsParam := c.PostForm("user_ids") // Comma-separated list of UUIDs

	// Parse user IDs if specified
	var userIDs []uuid.UUID
	if userIDsParam != "" {
		for _, idStr := range strings.Split(userIDsParam, ",") {
			if id, err := uuid.Parse(strings.TrimSpace(idStr)); err == nil {
				userIDs = append(userIDs, id)
			}
		}
	}

	// Validate file type
	filename := header.Filename
	if !strings.HasSuffix(filename, ".tar.gz") && !strings.HasSuffix(filename, ".tgz") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file type. Expected .tar.gz or .tgz file",
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

	// Create importer
	dataDir := config.Get("DATA_DIR", "")
	if dataDir == "" {
		dataDir = "/data"
	}
	importer := export.NewImporter(database.DB, dataDir)

	// Configure import options
	importOptions := export.ImportOptions{
		OverwriteFiles:    overwriteFiles,
		OverwriteDatabase: overwriteDatabase,
		UserIDs:           userIDs,
	}

	// Perform import
	metadata, err := importer.Import(tempFilePath, importOptions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to restore backup: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Backup restored successfully from %s", filename),
		"metadata": gin.H{
			"aviary_version": metadata.AviaryVersion,
			"database_type":  metadata.DatabaseType,
			"users_restored": len(metadata.UsersExported),
			"export_date":    metadata.ExportTimestamp.Format("2006-01-02 15:04:05"),
		},
	})
}
