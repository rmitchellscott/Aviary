package manager

import (
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/database"
	"gorm.io/gorm"
)

// UserFolderCacheService manages per-user folder caching
type UserFolderCacheService struct {
	db              *gorm.DB
	caches          map[uuid.UUID]*userFolderCache
	mu              sync.RWMutex
	refreshInterval time.Duration
	rateLimitDelay  time.Duration
}

// userFolderCache represents a single user's folder cache
type userFolderCache struct {
	folders    []string
	updated    time.Time
	mu         sync.RWMutex
	refreshing bool
}

// NewUserFolderCacheService creates a new user folder cache service
func NewUserFolderCacheService(db *gorm.DB) *UserFolderCacheService {
	refreshInterval := cacheRefreshInterval
	// If caching is disabled (interval <= 0), keep it disabled for background jobs

	// Get rate limit from environment variable (refreshes per second)
	rateLimitDelay := 5 * time.Second // Default: 0.2 refreshes per second (one every 5 seconds)
	if rpsStr := os.Getenv("FOLDER_REFRESH_RATE"); rpsStr != "" {
		if rps, err := strconv.ParseFloat(rpsStr, 64); err == nil && rps > 0 {
			rateLimitDelay = time.Duration(float64(time.Second) / rps)
		}
	}

	return &UserFolderCacheService{
		db:              db,
		caches:          make(map[uuid.UUID]*userFolderCache),
		refreshInterval: refreshInterval,
		rateLimitDelay:  rateLimitDelay,
	}
}

// getUserNextRefresh calculates when a user should next refresh based on their percentage
func (s *UserFolderCacheService) getUserNextRefresh(user *database.User) time.Time {
	now := time.Now()
	intervalStart := now.Truncate(s.refreshInterval)
	
	// Calculate offset within interval based on user's percentage
	offsetDuration := time.Duration(float64(s.refreshInterval) * float64(user.FolderRefreshPercent) / 100.0)
	
	nextRefresh := intervalStart.Add(offsetDuration)
	
	// If we've passed this interval's refresh time, schedule for next interval
	if nextRefresh.Before(now) {
		nextRefresh = nextRefresh.Add(s.refreshInterval)
	}
	
	return nextRefresh
}

// GetUserFolders returns the cached folders for a user, refreshing if necessary
func (s *UserFolderCacheService) GetUserFolders(userID uuid.UUID, force bool) ([]string, error) {
	// Get or create user cache
	s.mu.Lock()
	userCache, exists := s.caches[userID]
	if !exists {
		userCache = &userFolderCache{}
		s.caches[userID] = userCache

		// Try to load from database
		if folders, err := s.LoadFolderCacheFromDatabase(userID); err == nil && folders != nil {
			userCache.folders = folders
			userCache.updated = time.Now() // Treat as fresh for now
		}
	}
	s.mu.Unlock()

	userCache.mu.RLock()
	needsRefresh := len(userCache.folders) == 0 ||
		time.Since(userCache.updated) > s.refreshInterval ||
		force
	userCache.mu.RUnlock()

	if needsRefresh {
		return s.refreshUserFolders(userID, userCache, false) // UI refresh - no rate limiting
	}

	userCache.mu.RLock()
	defer userCache.mu.RUnlock()

	// Return a copy to avoid concurrent modifications
	result := make([]string, len(userCache.folders))
	copy(result, userCache.folders)
	return result, nil
}

// refreshUserFolders refreshes the folder cache for a specific user
func (s *UserFolderCacheService) refreshUserFolders(userID uuid.UUID, userCache *userFolderCache, rateLimited bool) ([]string, error) {
	userCache.mu.Lock()
	defer userCache.mu.Unlock()

	// Check if another goroutine is already refreshing
	if userCache.refreshing {
		// Wait for the refresh to complete and return current folders
		for userCache.refreshing {
			userCache.mu.Unlock()
			time.Sleep(100 * time.Millisecond)
			userCache.mu.Lock()
		}
		result := make([]string, len(userCache.folders))
		copy(result, userCache.folders)
		return result, nil
	}

	userCache.refreshing = true
	defer func() { userCache.refreshing = false }()

	// Get user information
	user, err := database.NewUserService(s.db).GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	// Apply rate limiting for background refreshes
	if rateLimited {
		time.Sleep(s.rateLimitDelay)
	}

	// Get folders using user-specific configuration
	folders, err := s.listUserFolders(user)
	if err != nil {
		return nil, err
	}

	// Update cache
	userCache.folders = folders
	userCache.updated = time.Now()

	// Save to database
	if err := s.saveFolderCacheToDatabase(userID, folders); err != nil {
		Logf("Failed to save folder cache to database for user %s: %v", userID, err)
		// Continue anyway, we have the data in memory
	}

	// Return a copy
	result := make([]string, len(folders))
	copy(result, folders)
	return result, nil
}

// listUserFolders lists folders for a specific user using their rmapi configuration
func (s *UserFolderCacheService) listUserFolders(user *database.User) ([]string, error) {
	return ListFolders(user)
}

// saveFolderCacheToDatabase saves the folder cache to the database
func (s *UserFolderCacheService) saveFolderCacheToDatabase(userID uuid.UUID, folders []string) error {
	// Convert folders to JSON
	foldersJSON, err := json.Marshal(folders)
	if err != nil {
		return err
	}

	// Use SQLite's native UPSERT for atomic operation
	// This works with both the old mattn/go-sqlite3 and new glebarez/sqlite drivers
	query := `
		INSERT INTO user_folders_cache (id, user_id, folder_path, folder_data, last_updated)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id, folder_path) DO UPDATE SET
			folder_data = EXCLUDED.folder_data,
			last_updated = EXCLUDED.last_updated
	`
	
	return s.db.Exec(query, uuid.New(), userID, "/", string(foldersJSON), time.Now()).Error
}

// LoadFolderCacheFromDatabase loads cached folders from the database
func (s *UserFolderCacheService) LoadFolderCacheFromDatabase(userID uuid.UUID) ([]string, error) {
	var folderCache database.FolderCache
	err := s.db.Where("user_id = ? AND folder_path = ?", userID, "/").First(&folderCache).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No cache found
		}
		return nil, err
	}

	// Parse JSON data
	var folders []string
	if err := json.Unmarshal([]byte(folderCache.FolderData), &folders); err != nil {
		return nil, err
	}

	return folders, nil
}

// InvalidateUserCache invalidates the cache for a specific user
func (s *UserFolderCacheService) InvalidateUserCache(userID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userCache, exists := s.caches[userID]; exists {
		userCache.mu.Lock()
		userCache.folders = nil
		userCache.updated = time.Time{}
		userCache.mu.Unlock()
	}
}

// StartBackgroundRefresh starts a background goroutine that periodically refreshes user caches
func (s *UserFolderCacheService) StartBackgroundRefresh() {
	if s.refreshInterval <= 0 {
		return
	}

	go func() {
		// Check every minute for users due for refresh
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			s.refreshActiveUserCaches()
		}
	}()
}

// refreshActiveUserCaches refreshes caches for users who have database cache entries
func (s *UserFolderCacheService) refreshActiveUserCaches() {
	// Get all users who have folder cache entries in database
	var folderCaches []database.FolderCache
	if err := s.db.Select("DISTINCT user_id").Find(&folderCaches).Error; err != nil {
		Logf("Failed to get users with folder cache: %v", err)
		return
	}

	userIDs := make([]uuid.UUID, 0, len(folderCaches))
	for _, cache := range folderCaches {
		userIDs = append(userIDs, cache.UserID)
	}

	// Check which users are due for refresh based on their percentage timing
	for i, userID := range userIDs {
		go func(uid uuid.UUID, delay int) {
			// Rate limit: stagger refreshes 
			time.Sleep(time.Duration(delay) * s.rateLimitDelay)
			
			userService := database.NewUserService(s.db)
			user, err := userService.GetUserByID(uid)
			if err != nil {
				Logf("Failed to get user %s for background refresh: %v", uid, err)
				return
			}

			// Check if user is due for refresh based on percentage timing
			nextRefresh := s.getUserNextRefresh(user)
			now := time.Now()
			
			if now.After(nextRefresh.Add(-time.Minute)) && now.Before(nextRefresh.Add(time.Minute)) {
				// User is due for refresh (within 1 minute window)
				s.mu.Lock()
				userCache, exists := s.caches[uid]
				if !exists {
					// Create cache entry for this user
					userCache = &userFolderCache{}
					s.caches[uid] = userCache
					
					// Try to load existing data from database
					if folders, err := s.LoadFolderCacheFromDatabase(uid); err == nil && folders != nil {
						userCache.folders = folders
						userCache.updated = time.Now()
					}
				}
				s.mu.Unlock()
				
				_, err := s.refreshUserFolders(uid, userCache, true) // Background refresh - rate limited
				if err != nil {
					Logf("Background refresh failed for user %s: %v", uid, err)
				}
			}
		}(userID, i)
	}
}
