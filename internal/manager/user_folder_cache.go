package manager

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/database"
	"gorm.io/gorm"
)

// UserFolderCacheService manages per-user folder caching
type UserFolderCacheService struct {
	db           *gorm.DB
	caches       map[uuid.UUID]*userFolderCache
	mu           sync.RWMutex
	refreshInterval time.Duration
}

// userFolderCache represents a single user's folder cache
type userFolderCache struct {
	folders     []string
	updated     time.Time
	mu          sync.RWMutex
	refreshing  bool
}

// NewUserFolderCacheService creates a new user folder cache service
func NewUserFolderCacheService(db *gorm.DB) *UserFolderCacheService {
	refreshInterval := cacheRefreshInterval
	if refreshInterval <= 0 {
		refreshInterval = 60 * time.Minute
	}
	
	return &UserFolderCacheService{
		db:              db,
		caches:          make(map[uuid.UUID]*userFolderCache),
		refreshInterval: refreshInterval,
	}
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
		return s.refreshUserFolders(userID, userCache)
	}

	userCache.mu.RLock()
	defer userCache.mu.RUnlock()
	
	// Return a copy to avoid concurrent modifications
	result := make([]string, len(userCache.folders))
	copy(result, userCache.folders)
	return result, nil
}

// refreshUserFolders refreshes the folder cache for a specific user
func (s *UserFolderCacheService) refreshUserFolders(userID uuid.UUID, userCache *userFolderCache) ([]string, error) {
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
	// For now, we'll use the existing ListFolders function
	// In a full implementation, we'd need to configure rmapi per user
	// TODO: Implement per-user rmapi configuration
	return ListFolders()
}

// saveFolderCacheToDatabase saves the folder cache to the database
func (s *UserFolderCacheService) saveFolderCacheToDatabase(userID uuid.UUID, folders []string) error {
	// Convert folders to JSON
	foldersJSON, err := json.Marshal(folders)
	if err != nil {
		return err
	}

	// Create or update folder cache record
	folderCache := database.FolderCache{
		ID:          uuid.New(),
		UserID:      userID,
		FolderPath:  "/", // Root path for the complete folder listing
		FolderData:  string(foldersJSON),
		LastUpdated: time.Now(),
	}

	// Use GORM's native upsert: update if exists, insert if not
	return s.db.Where("user_id = ? AND folder_path = ?", userID, "/").
		Assign(map[string]interface{}{
			"folder_data":   string(foldersJSON),
			"last_updated":  time.Now(),
		}).
		FirstOrCreate(&folderCache).Error
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
		ticker := time.NewTicker(s.refreshInterval)
		defer ticker.Stop()
		
		for range ticker.C {
			s.refreshActiveUserCaches()
		}
	}()
}

// refreshActiveUserCaches refreshes caches for users who have been recently active
func (s *UserFolderCacheService) refreshActiveUserCaches() {
	s.mu.RLock()
	userIDs := make([]uuid.UUID, 0, len(s.caches))
	for userID := range s.caches {
		userIDs = append(userIDs, userID)
	}
	s.mu.RUnlock()

	// Refresh each user's cache in the background
	for _, userID := range userIDs {
		go func(uid uuid.UUID) {
			if userCache, exists := s.caches[uid]; exists {
				userCache.mu.RLock()
				needsRefresh := time.Since(userCache.updated) > s.refreshInterval
				userCache.mu.RUnlock()
				
				if needsRefresh {
					_, err := s.GetUserFolders(uid, false)
					if err != nil {
						Logf("Background refresh failed for user %s: %v", uid, err)
					}
				}
			}
		}(userID)
	}
}