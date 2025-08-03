package rmapi

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/database"
)

// LoadUserConfig loads rmapi configuration for a user
// In multi-user mode: loads from database
// In single-user mode: returns path to ~/.config/rmapi/rmapi.conf
func LoadUserConfig(userID uuid.UUID) (string, error) {
	if database.IsMultiUserMode() {
		return loadMultiUserConfig(userID)
	}
	return loadSingleUserConfig()
}

// SaveUserConfig saves rmapi configuration for a user
// In multi-user mode: saves to database
// In single-user mode: not supported (configs are managed by rmapi directly)
func SaveUserConfig(userID uuid.UUID, configContent string) error {
	if !database.IsMultiUserMode() {
		return fmt.Errorf("config saving only available in multi-user mode")
	}
	return database.SaveUserRmapiConfig(userID, configContent)
}

// loadMultiUserConfig loads config content from database
func loadMultiUserConfig(userID uuid.UUID) (string, error) {
	var user database.User
	if err := database.DB.Select("rmapi_config").Where("id = ?", userID).First(&user).Error; err != nil {
		return "", fmt.Errorf("failed to load user config from database: %w", err)
	}
	return user.RmapiConfig, nil
}

// loadSingleUserConfig returns path to single-user config file
func loadSingleUserConfig() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".config", "rmapi", "rmapi.conf"), nil
}

// GetUserConfigPath returns the config path for a user
// In multi-user mode: creates temporary file from database content
// In single-user mode: returns ~/.config/rmapi/rmapi.conf
func GetUserConfigPath(userID uuid.UUID) (string, error) {
	if database.IsMultiUserMode() {
		// Load config from database and create temp file
		configContent, err := loadMultiUserConfig(userID)
		if err != nil {
			return "", err
		}
		if configContent == "" {
			return "", fmt.Errorf("user %s has no rmapi config", userID)
		}
		return createTempConfigFile(userID, configContent)
	}
	return loadSingleUserConfig()
}

// createTempConfigFile creates a temporary config file from database content
func createTempConfigFile(userID uuid.UUID, configContent string) (string, error) {
	// Create unique temp file name to avoid conflicts between concurrent users
	tempFileName := fmt.Sprintf("rmapi-%s.conf", userID.String())
	tempFilePath := filepath.Join(os.TempDir(), tempFileName)
	
	// Write config content to temp file
	if err := os.WriteFile(tempFilePath, []byte(configContent), 0600); err != nil {
		return "", fmt.Errorf("failed to create temp config file: %w", err)
	}
	
	return tempFilePath, nil
}

// CleanupTempConfigFile removes a temporary config file if it exists
func CleanupTempConfigFile(configPath string) {
	if configPath == "" {
		return
	}
	
	// Only cleanup files in temp directory with our naming pattern
	if filepath.Dir(configPath) == os.TempDir() && 
	   filepath.Base(configPath) != "rmapi.conf" { // Don't delete the standard config
		os.Remove(configPath)
	}
}

