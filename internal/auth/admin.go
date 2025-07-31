package auth

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/backup"
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
		c.JSON(http.StatusBadRequest, gin.H{"error_type": "invalid_request"})
		return
	}

	// Validate allowed settings
	allowedSettings := map[string]bool{
		"registration_enabled":         true,
		"max_api_keys_per_user":        true,
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
		c.JSON(http.StatusBadRequest, gin.H{"error_type": "parse_form_failed"})
		return
	}

	// Get uploaded file
	file, header, err := c.Request.FormFile("backup_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error_type": "no_backup_file"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error_type": "create_temp_file_failed"})
		return
	}
	defer os.Remove(tempFilePath)
	defer tempFile.Close()

	// Copy uploaded file to temp location
	if _, err := io.Copy(tempFile, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error_type": "save_uploaded_file_failed"})
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
			"error": err.Error(),
			"valid": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":    true,
		"metadata": analysis,
	})
}

// AnalyzeRestoreUploadHandler analyzes an already uploaded restore file by ID (admin only)
func AnalyzeRestoreUploadHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Backup analysis not available in single-user mode"})
		return
	}

	user, ok := RequireAdmin(c)
	if !ok {
		return
	}

	// Get upload ID from URL parameter
	uploadIDStr := c.Param("id")
	uploadID, err := uuid.Parse(uploadIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error_type": "invalid_upload_id"})
		return
	}

	// Find uploaded file in database
	var restoreUpload database.RestoreUpload
	if err := database.DB.Where("id = ? AND admin_user_id = ? AND status = ?", uploadID, user.ID, "uploaded").First(&restoreUpload).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error_type": "upload_not_found"})
		return
	}

	// Check if file still exists
	if _, err := os.Stat(restoreUpload.FilePath); os.IsNotExist(err) {
		// Clean up database record
		database.DB.Delete(&restoreUpload)
		c.JSON(http.StatusNotFound, gin.H{"error_type": "upload_not_found"})
		return
	}

	// Create analyzer
	dataDir := config.Get("DATA_DIR", "")
	if dataDir == "" {
		dataDir = "/data"
	}
	analyzer := export.NewAnalyzer(database.DB, dataDir)

	// Analyze backup
	analysis, err := analyzer.AnalyzeBackup(restoreUpload.FilePath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
			"valid": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":    true,
		"metadata": analysis,
	})
}

// UploadRestoreFileHandler uploads a restore file and returns upload ID (admin only)
func UploadRestoreFileHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Database restore not available in single-user mode"})
		return
	}

	user, ok := RequireAdmin(c)
	if !ok {
		return
	}

	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error_type": "parse_form_failed"})
		return
	}

	file, header, err := c.Request.FormFile("backup_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error_type": "no_backup_file"})
		return
	}
	defer file.Close()

	filename := header.Filename
	if !strings.HasSuffix(filename, ".tar.gz") && !strings.HasSuffix(filename, ".tgz") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file type. Expected .tar.gz or .tgz file",
		})
		return
	}

	// Create database record
	uploadID := uuid.New()
	tempDir := os.TempDir()
	tempFilePath := filepath.Join(tempDir, "restore_"+uploadID.String()+"_"+filename)

	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error_type": "create_temp_file_failed"})
		return
	}
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error_type": "save_uploaded_file_failed"})
		return
	}

	// Get file size
	fileInfo, err := tempFile.Stat()
	if err != nil {
		os.Remove(tempFilePath)
		c.JSON(http.StatusInternalServerError, gin.H{"error_type": "get_file_info_failed"})
		return
	}

	// Save to database
	restoreUpload := database.RestoreUpload{
		ID:          uploadID,
		AdminUserID: user.ID,
		Filename:    filename,
		FilePath:    tempFilePath,
		FileSize:    fileInfo.Size(),
		Status:      "uploaded",
		ExpiresAt:   time.Now().Add(24 * time.Hour), // Expire after 24 hours
	}

	if err := database.DB.Create(&restoreUpload).Error; err != nil {
		os.Remove(tempFilePath)
		c.JSON(http.StatusInternalServerError, gin.H{"error_type": "save_upload_record_failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"upload_id": uploadID,
		"filename":  filename,
		"status":    "uploaded",
		"file_size": fileInfo.Size(),
		"expires_at": restoreUpload.ExpiresAt,
		"message":   "File uploaded successfully. Ready for restore.",
	})
}

// RestoreDatabaseHandler initiates database restore from uploaded file (admin only)
func RestoreDatabaseHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Database restore not available in single-user mode"})
		return
	}

	user, ok := RequireAdmin(c)
	if !ok {
		return
	}

	var req struct {
		UploadID          string   `json:"upload_id" binding:"required"`
		OverwriteFiles    bool     `json:"overwrite_files"`
		OverwriteDatabase bool     `json:"overwrite_database"`
		UserIDs           []string `json:"user_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error_type": "invalid_request"})
		return
	}

	// Find uploaded file in database
	uploadID, err := uuid.Parse(req.UploadID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error_type": "invalid_upload_id"})
		return
	}

	var restoreUpload database.RestoreUpload
	if err := database.DB.Where("id = ? AND admin_user_id = ? AND status = ?", uploadID, user.ID, "uploaded").First(&restoreUpload).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error_type": "upload_not_found"})
		return
	}

	// Check if file still exists
	if _, err := os.Stat(restoreUpload.FilePath); os.IsNotExist(err) {
		// Clean up database record
		database.DB.Delete(&restoreUpload)
		c.JSON(http.StatusNotFound, gin.H{"error_type": "upload_not_found"})
		return
	}

	// Parse user IDs
	var userIDs []uuid.UUID
	for _, idStr := range req.UserIDs {
		if id, err := uuid.Parse(strings.TrimSpace(idStr)); err == nil {
			userIDs = append(userIDs, id)
		}
	}

	dataDir := config.Get("DATA_DIR", "")
	if dataDir == "" {
		dataDir = "/data"
	}
	importer := export.NewImporter(database.DB, dataDir)

	importOptions := export.ImportOptions{
		OverwriteFiles:    req.OverwriteFiles,
		OverwriteDatabase: req.OverwriteDatabase,
		UserIDs:           userIDs,
	}

	_, err = importer.Import(restoreUpload.FilePath, importOptions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_type": "restore_import_failed",
		})
		return
	}

	// Run database migrations after successful restore
	log.Printf("[RESTORE] Running database migrations after restore...")
	
	// Run GORM auto-migrations to update schema
	if err := database.RunAutoMigrations("RESTORE"); err != nil {
		log.Printf("Warning: GORM auto-migration failed after restore: %v", err)
		// Don't fail the restore - migrations can be run manually if needed
	} else {
		log.Printf("[RESTORE] GORM auto-migration completed successfully")
	}

	// Run custom migrations
	if err := database.RunMigrations("RESTORE"); err != nil {
		log.Printf("Warning: Custom migrations failed after restore: %v", err)
		// Don't fail the restore - migrations can be run manually if needed
	} else {
		log.Printf("[RESTORE] Custom migrations completed successfully")
	}

	// Clean up the uploaded file and database record
	os.Remove(restoreUpload.FilePath)
	database.DB.Delete(&restoreUpload)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// GetRestoreUploadsHandler returns pending restore uploads for the admin user
func GetRestoreUploadsHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Database restore not available in single-user mode"})
		return
	}

	user, ok := RequireAdmin(c)
	if !ok {
		return
	}

	var uploads []database.RestoreUpload
	if err := database.DB.Where("admin_user_id = ? AND status = ? AND expires_at > ?", user.ID, "uploaded", time.Now()).Find(&uploads).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get restore uploads: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"uploads": uploads,
	})
}

// DeleteRestoreUploadHandler deletes a restore upload and its file
func DeleteRestoreUploadHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Database restore not available in single-user mode"})
		return
	}

	user, ok := RequireAdmin(c)
	if !ok {
		return
	}

	uploadIDStr := c.Param("id")
	uploadID, err := uuid.Parse(uploadIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error_type": "invalid_upload_id"})
		return
	}

	var restoreUpload database.RestoreUpload
	if err := database.DB.Where("id = ? AND admin_user_id = ?", uploadID, user.ID).First(&restoreUpload).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error_type": "upload_not_found"})
		return
	}

	// Delete the file
	if restoreUpload.FilePath != "" {
		os.Remove(restoreUpload.FilePath)
	}

	// Delete the database record
	if err := database.DB.Delete(&restoreUpload).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_type": "restore_delete_record_failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// CreateBackupJobHandler creates a background backup job (admin only)
func CreateBackupJobHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Background backup not available in single-user mode"})
		return
	}

	user, ok := RequireAdmin(c)
	if !ok {
		return
	}

	// Parse query parameters for export options
	includeFiles := c.DefaultQuery("include_files", "true") == "true"
	includeConfigs := c.DefaultQuery("include_configs", "true") == "true"
	userIDsParam := c.Query("user_ids")

	// Parse user IDs if specified
	var userIDs []uuid.UUID
	if userIDsParam != "" {
		for _, idStr := range strings.Split(userIDsParam, ",") {
			if id, err := uuid.Parse(strings.TrimSpace(idStr)); err == nil {
				userIDs = append(userIDs, id)
			}
		}
	}

	// Create backup job
	job, err := backup.CreateBackupJob(database.DB, user.ID, includeFiles, includeConfigs, userIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_type": "create_backup_job_failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"job_id":  job.ID,
		"message": "Backup job created successfully",
	})
}

// GetBackupJobsHandler returns backup jobs for the admin user
func GetBackupJobsHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Background backup not available in single-user mode"})
		return
	}

	user, ok := RequireAdmin(c)
	if !ok {
		return
	}

	jobs, err := backup.GetBackupJobs(database.DB, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_type": "get_backup_jobs_failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"jobs": jobs,
	})
}

// GetBackupJobHandler returns a specific backup job
func GetBackupJobHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Background backup not available in single-user mode"})
		return
	}

	user, ok := RequireAdmin(c)
	if !ok {
		return
	}

	jobIDStr := c.Param("id")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error_type": "invalid_job_id"})
		return
	}

	job, err := backup.GetBackupJob(database.DB, jobID, user.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error_type": "backup_job_not_found"})
		return
	}

	c.JSON(http.StatusOK, job)
}

// DownloadBackupHandler downloads a completed backup file
func DownloadBackupHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Background backup not available in single-user mode"})
		return
	}

	user, ok := RequireAdmin(c)
	if !ok {
		return
	}

	jobIDStr := c.Param("id")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error_type": "invalid_job_id"})
		return
	}

	job, err := backup.GetBackupJob(database.DB, jobID, user.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error_type": "backup_job_not_found"})
		return
	}

	if job.Status != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error_type": "backup_not_ready"})
		return
	}

	if job.FilePath == "" || job.Filename == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error_type": "backup_file_unavailable"})
		return
	}

	// Check if file exists
	if _, err := os.Stat(job.FilePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error_type": "backup_not_found"})
		return
	}

	// Set headers for download
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", job.Filename))
	c.Header("Content-Type", "application/gzip")
	c.Header("Content-Description", "Aviary Backup")

	// Stream file to client
	c.File(job.FilePath)
}

// DeleteBackupJobHandler deletes a backup job and its file
func DeleteBackupJobHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Background backup not available in single-user mode"})
		return
	}

	user, ok := RequireAdmin(c)
	if !ok {
		return
	}

	jobIDStr := c.Param("id")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error_type": "invalid_job_id"})
		return
	}

	if err := backup.DeleteBackupJob(database.DB, jobID, user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_type": "delete_backup_job_failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Backup job deleted successfully",
	})
}

