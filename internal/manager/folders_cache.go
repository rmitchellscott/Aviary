package manager

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/rmapi"
)

// folderCache stores the cached folder listing
// along with the time it was last refreshed.
type folderCache struct {
	mu      sync.RWMutex
	folders []string
	updated time.Time
}

var globalFoldersCache = &folderCache{}

// used to ensure only one background refresh runs at a time
var refreshRunning int32

// cacheRefreshInterval controls how often the cache is refreshed.
// It can be overridden via the FOLDER_CACHE_INTERVAL
// environment variable. Set to 0 to disable caching entirely.
var cacheRefreshInterval = 60 * time.Minute

func init() {
	if v := config.Get("FOLDER_CACHE_INTERVAL", ""); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cacheRefreshInterval = d
		}
	}
}

// StartFolderCache begins a goroutine that periodically refreshes
// the cached folder listing. It also performs an initial refresh so
// the first user request can be served quickly.
func StartFolderCache() {
	if cacheRefreshInterval <= 0 {
		// caching disabled
		return
	}
	// initial warm
	if err := refreshFolderCache(); err != nil {
		Logf("initial folder cache refresh failed: %v", err)
	}

	go func() {
		ticker := time.NewTicker(cacheRefreshInterval)
		defer ticker.Stop()
		for range ticker.C {
			if err := refreshFolderCache(); err != nil {
				Logf("folder cache refresh failed: %v", err)
			}
		}
	}()
}

// RefreshFolderCache manually triggers a folder cache refresh (single-user mode)
func RefreshFolderCache() error {
	// Check if single user is paired before attempting refresh
	if !rmapi.IsUserPaired(uuid.Nil) {
		return fmt.Errorf("single user not paired")
	}
	return refreshFolderCache()
}

// refreshFolderCache fetches folders from the device and stores them
// in the global cache.
func refreshFolderCache() error {
	// Check if single user is paired before attempting refresh
	if !rmapi.IsUserPaired(uuid.Nil) {
		return fmt.Errorf("single user not paired")
	}

	dirs, err := ListFolders(nil)
	if err != nil {
		return err
	}
	globalFoldersCache.mu.Lock()
	globalFoldersCache.folders = dirs
	globalFoldersCache.updated = time.Now()
	globalFoldersCache.mu.Unlock()
	return nil
}

// cachedFolders returns the currently cached folder list if available.
// The returned slice is a copy and safe for callers to modify.
func cachedFolders() ([]string, bool) {
	globalFoldersCache.mu.RLock()
	defer globalFoldersCache.mu.RUnlock()
	if len(globalFoldersCache.folders) == 0 {
		return nil, false
	}
	cp := make([]string, len(globalFoldersCache.folders))
	copy(cp, globalFoldersCache.folders)
	return cp, true
}

// maybeRefreshFolderCache triggers a background refresh if the cached listing
// is older than the refresh interval. It ensures only one refresh runs at a
// time to avoid hammering the device when bots hit the endpoint.
func maybeRefreshFolderCache() {
	if cacheRefreshInterval <= 0 {
		return
	}

	globalFoldersCache.mu.RLock()
	age := time.Since(globalFoldersCache.updated)
	globalFoldersCache.mu.RUnlock()
	if age < cacheRefreshInterval {
		return
	}

	if !atomic.CompareAndSwapInt32(&refreshRunning, 0, 1) {
		return
	}

	go func() {
		defer atomic.StoreInt32(&refreshRunning, 0)
		if err := refreshFolderCache(); err != nil {
			Logf("folder cache refresh failed: %v", err)
		}
	}()
}
