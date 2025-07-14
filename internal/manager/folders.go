package manager

import (
	"bufio"
	"bytes"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rmitchellscott/aviary/internal/auth"
	"github.com/rmitchellscott/aviary/internal/database"
	"gorm.io/gorm"
)

// Global instance of user folder cache service
var userFolderCacheService *UserFolderCacheService

// InitializeUserFolderCache initializes the user folder cache service
func InitializeUserFolderCache(db *gorm.DB) {
	if database.IsMultiUserMode() {
		userFolderCacheService = NewUserFolderCacheService(db)
		userFolderCacheService.StartBackgroundRefresh()
	}
}

// ListFolders returns a slice of all folder paths on the reMarkable device.
// Paths are returned with a leading slash, e.g. "/Books/Fiction".
func ListFolders(user *database.User) ([]string, error) {
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
