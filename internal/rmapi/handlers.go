package rmapi

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/database"
)

// PostPairingCallback is called after successful pairing
type PostPairingCallback func(userID string, singleUserMode bool)

// Global callback for post-pairing actions
var postPairingCallback PostPairingCallback

// requireUser extracts the authenticated user from the gin context
// This avoids import cycle with auth package
func requireUser(c *gin.Context) (*database.User, bool) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return nil, false
	}
	
	dbUser, ok := user.(*database.User)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user context"})
		return nil, false
	}
	
	return dbUser, true
}

// SetPostPairingCallback sets the callback to be called after successful pairing
func SetPostPairingCallback(callback PostPairingCallback) {
	postPairingCallback = callback
}

// GetPostPairingCallback returns the current post-pairing callback
func GetPostPairingCallback() PostPairingCallback {
	return postPairingCallback
}

// HandlePairRequest handles pairing requests for both single-user and multi-user modes
func HandlePairRequest(c *gin.Context) {
	if database.IsMultiUserMode() {
		// In multi-user mode, delegate to the multi-user handler
		PairHandler(c)
		return
	}
	
	// Single-user mode pairing
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	
	// Ensure the rmapi config directory exists
	home, err := os.UserHomeDir()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to determine home directory"})
		return
	}
	cfgDir := filepath.Join(home, ".config", "rmapi")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create config directory"})
		return
	}
	cfgPath := filepath.Join(cfgDir, "rmapi.conf")
	
	// Run rmapi cd command with the provided code
	cmd := exec.Command("rmapi", "cd")
	cmd.Stdin = strings.NewReader(req.Code + "\n")
	
	// Set environment variables
	env := os.Environ()
	env = append(env, "RMAPI_CONFIG="+cfgPath)
	if host := config.Get("RMAPI_HOST", ""); host != "" {
		env = append(env, "RMAPI_HOST="+host)
	}
	cmd.Env = env
	
	if err := cmd.Run(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Pairing failed"})
		return
	}
	
	// Call post-pairing callback if set (async for folder cache refresh)
	if postPairingCallback != nil {
		go postPairingCallback("single-user", true) // true = single-user mode
	}
	
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// PairHandler handles rmapi pairing for multi-user mode
func PairHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not available in single-user mode"})
		return
	}

	// Mock successful pairing in DRY_RUN mode
	if config.Get("DRY_RUN", "") != "" {
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	user, ok := requireUser(c)
	if !ok {
		return
	}

	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Create temporary directory for rmapi pairing process
	tempDir, err := os.MkdirTemp("", "rmapi-pair-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp directory"})
		return
	}
	defer os.RemoveAll(tempDir)
	
	cfgPath := filepath.Join(tempDir, "rmapi.conf")

	// Run rmapi cd command with user-specific configuration
	cmd := exec.Command("rmapi", "cd")
	cmd.Stdin = strings.NewReader(req.Code + "\n")
	
	env := os.Environ()
	env = append(env, "RMAPI_CONFIG="+cfgPath)
	if user.RmapiHost != "" {
		env = append(env, "RMAPI_HOST="+user.RmapiHost)
	} else {
		// Remove server-level RMAPI_HOST to use official cloud
		env = filterEnv(env, "RMAPI_HOST")
	}
	cmd.Env = env
	
	if err := cmd.Run(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Pairing failed"})
		return
	}

	// After successful pairing, save config to database
	if configContent, err := os.ReadFile(cfgPath); err == nil {
		SaveUserConfig(user.ID, string(configContent))
	}

	// Call post-pairing callback if set (async for folder cache refresh)
	if postPairingCallback != nil {
		go postPairingCallback(user.ID.String(), false) // false = multi-user mode
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// UnpairHandler removes the rmapi configuration for the current user
func UnpairHandler(c *gin.Context) {
	if !database.IsMultiUserMode() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not available in single-user mode"})
		return
	}

	user, ok := requireUser(c)
	if !ok {
		return
	}

	// Clear database config (no filesystem operations needed)
	SaveUserConfig(user.ID, "")

	// Cleanup user cache to ensure fresh state on next pairing
	cachePath := GetUserCachePath(user.ID)
	CleanupUserCache(cachePath)

	c.JSON(http.StatusOK, gin.H{"success": true})
}