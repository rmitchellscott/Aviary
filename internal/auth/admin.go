package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rmitchellscott/aviary/internal/database"
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

	c.JSON(http.StatusOK, gin.H{
		"database": dbStats,
		"smtp": gin.H{
			"configured": smtpConfigured,
			"status":     smtpStatus,
		},
		"settings": gin.H{
			"registration_enabled":    registrationEnabled,
			"max_api_keys_per_user":   maxAPIKeys,
			"session_timeout_hours":   sessionTimeout,
		},
		"mode": "multi_user",
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
		"registration_enabled":           true,
		"max_api_keys_per_user":          true,
		"session_timeout_hours":          true,
		"password_reset_timeout_hours":   true,
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

	// For now, just return a TODO message
	// This would need to be implemented based on the specific backup requirements
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Database backup functionality is not yet implemented",
		"todo":    "Implement backup functionality based on requirements",
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

	// For now, just return a TODO message
	// This would need to be implemented based on the specific restore requirements
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Database restore functionality is not yet implemented",
		"todo":    "Implement restore functionality based on requirements",
	})
}