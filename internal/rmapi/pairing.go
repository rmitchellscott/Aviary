package rmapi

import (
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/database"
)

// IsUserPaired checks if a user has valid rmapi configuration
// Handles both single-user (filesystem) and multi-user (database) modes automatically
func IsUserPaired(userID uuid.UUID) bool {
	// In DRY_RUN mode, always consider users as paired
	if config.Get("DRY_RUN", "") != "" {
		return true
	}

	if database.IsMultiUserMode() {
		return isMultiUserPaired(userID)
	}
	return isSingleUserPaired()
}

// isMultiUserPaired checks if user has rmapi config in database
func isMultiUserPaired(userID uuid.UUID) bool {
	var user database.User
	if err := database.DB.Select("rmapi_config").Where("id = ?", userID).First(&user).Error; err != nil {
		return false
	}
	return user.RmapiConfig != ""
}

// isSingleUserPaired checks if rmapi.conf exists in user's home directory
func isSingleUserPaired() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	cfgPath := filepath.Join(home, ".config", "rmapi", "rmapi.conf")
	info, err := os.Stat(cfgPath)
	return err == nil && info.Size() > 0
}