package manager

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/auth"
	"github.com/rmitchellscott/aviary/internal/database"
	"gorm.io/gorm"
)

// Global instance of user folder cache service
var userFolderCacheService *UserFolderCacheService

// isConfigFileValid checks if an rmapi config file exists and has content
func isConfigFileValid(configPath string) bool {
	// In DRY_RUN mode, always consider config as valid
	if os.Getenv("DRY_RUN") != "" {
		return true
	}
	
	info, err := os.Stat(configPath)
	return err == nil && info.Size() > 0
}


// IsSingleUserPaired checks if rmapi.conf exists for single-user mode
func IsSingleUserPaired() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	cfgPath := filepath.Join(home, ".config", "rmapi", "rmapi.conf")
	return isConfigFileValid(cfgPath)
}

// InitializeUserFolderCache initializes the user folder cache service
func InitializeUserFolderCache(db *gorm.DB) {
	if database.IsMultiUserMode() {
		userFolderCacheService = NewUserFolderCacheService(db)
		userFolderCacheService.StartBackgroundRefresh()
	}
}

// RefreshUserFolderCache manually triggers a folder cache refresh for a specific user
func RefreshUserFolderCache(userID string) error {
	if userFolderCacheService == nil {
		return nil // Not in multi-user mode or not initialized
	}
	
	uuid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	
	// Check if user is paired before attempting refresh
	if !IsUserPaired(uuid) {
		return fmt.Errorf("user %s not paired", userID)
	}
	
	_, err = userFolderCacheService.GetUserFolders(uuid, true) // force refresh
	return err
}

// ListFolders returns a slice of all folder paths on the reMarkable device.
// Paths are returned with a leading slash, e.g. "/Books/Fiction".
func ListFolders(user *database.User) ([]string, error) {
	// Check pairing status based on mode
	if database.IsMultiUserMode() {
		if user != nil && !IsUserPaired(user.ID) {
			return nil, fmt.Errorf("user %s not paired", user.ID)
		}
	} else {
		if !IsSingleUserPaired() {
			return nil, fmt.Errorf("single user not paired")
		}
	}
	
	// Include the root directory explicitly so the UI can offer it as an option
	folders := []string{"/"}

	var walk func(string) error
	walk = func(p string) error {
		args := []string{"ls"}
		if p != "" {
			args = append(args, p)
		}
		out, err := rmapiCmd(user, args...).Output()
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(bytes.NewReader(out))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "[d]") {
				name := strings.TrimSpace(strings.TrimPrefix(line, "[d]"))
				name = strings.TrimLeft(name, "\t ")

				// Skip the /trash folder entirely
				if p == "" && name == "trash" {
					continue
				}

				var child string
				if p == "" {
					child = name
				} else {
					child = path.Join(p, name)
				}
				folders = append(folders, "/"+child)
				if err := walk(child); err != nil {
					return err
				}
			}
		}
		return scanner.Err()
	}

	if err := walk(""); err != nil {
		return nil, err
	}
	return folders, nil
}

// FoldersHandler writes a JSON {"folders": ["/path", ...]}.
func FoldersHandler(c *gin.Context) {
	force := strings.EqualFold(c.Query("refresh"), "true") || c.Query("refresh") == "1"

	// In multi-user mode, use per-user caching
	if database.IsMultiUserMode() && userFolderCacheService != nil {
		user, ok := auth.RequireUser(c)
		if !ok {
			return
		}

		folders, err := userFolderCacheService.GetUserFolders(user.ID, force)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "backend.status.internal_error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"folders": folders})
		return
	}

	// Fall back to global cache for single-user mode
	useCache := cacheRefreshInterval > 0

	if useCache && !force {
		if cached, ok := cachedFolders(); ok {
			// Kick off a background refresh if the cache is old, but return immediately.
			maybeRefreshFolderCache()
			c.JSON(http.StatusOK, gin.H{"folders": cached})
			return
		}
	}

	// Either forced refresh, cache miss, or caching disabled.
	dirs, err := ListFolders(nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "backend.status.internal_error"})
		return
	}
	if useCache {
		globalFoldersCache.mu.Lock()
		globalFoldersCache.folders = dirs
		globalFoldersCache.updated = time.Now()
		globalFoldersCache.mu.Unlock()
	}
	c.JSON(http.StatusOK, gin.H{"folders": dirs})
}
