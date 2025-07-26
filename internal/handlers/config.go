package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rmitchellscott/aviary/internal/auth"
	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/database"
	"github.com/rmitchellscott/aviary/internal/manager"
	"github.com/rmitchellscott/aviary/internal/smtp"
)

// ConfigHandler returns application configuration information
func ConfigHandler(c *gin.Context) {
	var authEnabled bool
	var apiKeyEnabled bool
	var multiUserMode = database.IsMultiUserMode()
	var defaultRmDir string
	var rmapiHost string

	if multiUserMode {
		// In multi-user mode, auth is always enabled
		authEnabled = true
		apiKeyEnabled = true

		// Get user-specific settings if authenticated
		if user, exists := c.Get("user"); exists {
			if dbUser, ok := user.(*database.User); ok {
				// Use user-specific defaultrmdir if set, otherwise use global default
				if dbUser.DefaultRmdir != "" {
					defaultRmDir = dbUser.DefaultRmdir
				} else {
					defaultRmDir = manager.DefaultRmDir()
				}

				// Use user-specific rmapi host if set, empty string for official cloud
				rmapiHost = dbUser.RmapiHost
			} else {
				// Fallback to global defaults
				defaultRmDir = manager.DefaultRmDir()
				rmapiHost = config.Get("RMAPI_HOST", "")
			}
		} else {
			// Not authenticated, use global defaults
			defaultRmDir = manager.DefaultRmDir()
			rmapiHost = config.Get("RMAPI_HOST", "")
		}
	} else {
		// Single-user mode - check traditional auth methods only
		envUsername := config.Get("AUTH_USERNAME", "")
		envPassword := config.Get("AUTH_PASSWORD", "")
		envApiKey := config.Get("API_KEY", "")

		// Auth is enabled if traditional methods are configured
		authEnabled = (envUsername != "" && envPassword != "")

		apiKeyEnabled = envApiKey != ""
		defaultRmDir = manager.DefaultRmDir()
		rmapiHost = config.Get("RMAPI_HOST", "")
	}

	// Check SMTP configuration (only in multi-user mode)
	smtpConfigured := false
	if multiUserMode {
		smtpConfigured = smtp.IsSMTPConfigured()
	}

	// Check rmapi pairing status
	rmapiPaired := false
	if multiUserMode {
		// In multi-user mode, pairing status is per-user and handled by /api/auth/check
		// We don't include it here as it requires user context
	} else {
		// In single-user mode, check the global rmapi.conf file
		rmapiPaired = auth.CheckSingleUserPaired()
	}

	// Check authentication methods (multi-user mode only)
	oidcEnabled := false
	proxyAuthEnabled := false
	if multiUserMode {
		oidcEnabled = auth.IsOIDCEnabled()
		proxyAuthEnabled = auth.IsProxyAuthEnabled()
	}

	response := gin.H{
		"apiUrl":           "/api/",
		"authEnabled":      authEnabled,
		"apiKeyEnabled":    apiKeyEnabled,
		"multiUserMode":    multiUserMode,
		"defaultRmDir":     defaultRmDir,
		"rmapi_host":       rmapiHost,
		"smtpConfigured":   smtpConfigured,
		"oidcEnabled":      oidcEnabled,
		"proxyAuthEnabled": proxyAuthEnabled,
	}

	// Add rmapi_paired for single-user mode only
	if !multiUserMode {
		response["rmapi_paired"] = rmapiPaired
	}

	c.JSON(http.StatusOK, response)
}
